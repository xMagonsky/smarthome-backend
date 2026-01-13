package taskqueue

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"smarthome/internal/automation"
	"smarthome/internal/db"
	"smarthome/internal/utils"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
)

var (
	dbConn      *db.DB
	redisClient *redis.Client
	mqttClient  mqtt.Client
)

func SetGlobalInstances(database *db.DB, redis *redis.Client, mqtt mqtt.Client) {
	dbConn = database
	redisClient = redis
	mqttClient = mqtt
}

type DeviceUpdateTaskPayload struct {
	DeviceID string
	State    utils.DeviceState
}

type EvaluationTaskPayload struct {
	RuleID          string
	UpdatedDeviceID string
}

type PendingAction struct {
	RuleID   string
	DeviceID string
	Params   map[string]interface{}
}

type ActionTarget struct {
	DeviceID string
	Key      string
}

func EnqueueDeviceUpdate(deviceID string, state utils.DeviceState) error {
	payload, _ := json.Marshal(DeviceUpdateTaskPayload{DeviceID: deviceID, State: state})
	task := asynq.NewTask("device_update", payload)
	info, err := asynqClient.Enqueue(task, asynq.MaxRetry(3), asynq.Timeout(10*time.Second))
	if err != nil {
		log.Printf("TASKQUEUE: Failed to enqueue device update for %s: %v", deviceID, err)
		return err
	}
	log.Printf("TASKQUEUE: Device update successfully enqueued task %s for device %s", info.ID, deviceID)
	return nil
}

func EnqueueEvaluation(ruleID, updatedDeviceID string) error {
	payload, _ := json.Marshal(EvaluationTaskPayload{RuleID: ruleID, UpdatedDeviceID: updatedDeviceID})
	task := asynq.NewTask("evaluate_rule", payload)
	info, err := asynqClient.Enqueue(task, asynq.MaxRetry(3), asynq.Timeout(10*time.Second))
	if err != nil {
		log.Printf("TASKQUEUE: Failed to enqueue rule %s: %v", ruleID, err)
		return err
	}
	log.Printf("TASKQUEUE: Evaluation successfully enqueued task %s for rule %s", info.ID, ruleID)
	return nil
}

func processDeviceUpdateTask(ctx context.Context, t *asynq.Task) error {
	var payload DeviceUpdateTaskPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return err
	}

	ruleIDs, err := automation.ProcessDeviceUpdate(ctx, redisClient, dbConn, payload.DeviceID, payload.State)
	if err != nil {
		return err
	}

	for _, ruleID := range ruleIDs {
		EnqueueEvaluation(ruleID, payload.DeviceID)
	}

	return nil
}

func areAllActionsRedundant(ctx context.Context, actionsJSON json.RawMessage) bool {
	var actions []map[string]interface{}
	if err := json.Unmarshal(actionsJSON, &actions); err != nil {
		return false
	}

	for _, action := range actions {
		if action["type"] != "device" {
			return false
		}

		deviceIDFloat, ok := action["device_id"].(float64)
		if !ok {
			return false
		}
		deviceID := int64(deviceIDFloat)

		targetState, ok := action["state"].(map[string]interface{})
		if !ok || len(targetState) == 0 {
			return false
		}

		stateKey := fmt.Sprintf("device:%d:state", deviceID)
		currentStateJSON, err := redisClient.Get(ctx, stateKey).Result()
		if err != nil {
			return false
		}

		var currentState map[string]interface{}
		if err := json.Unmarshal([]byte(currentStateJSON), &currentState); err != nil {
			return false
		}

		for field, targetValue := range targetState {
			if currentValue, exists := currentState[field]; !exists || currentValue != targetValue {
				return false
			}
		}
	}
	return true
}

// collectPendingActions extracts pending actions from a rule's action JSON
func collectPendingActions(ruleID string, actionsJSON json.RawMessage) []PendingAction {
	var actions []struct {
		DeviceID string          `json:"device_id"`
		Action   string          `json:"action"`
		Params   json.RawMessage `json:"params"`
	}

	if err := json.Unmarshal(actionsJSON, &actions); err != nil {
		log.Printf("TASKQUEUE: Failed to unmarshal actions for rule %s: %v", ruleID, err)
		return nil
	}

	pendingActions := []PendingAction{}
	for _, action := range actions {
		if action.DeviceID == "" {
			// Skip non-device actions (e.g., notifications)
			continue
		}

		var params map[string]interface{}
		if err := json.Unmarshal(action.Params, &params); err != nil {
			log.Printf("TASKQUEUE: Failed to unmarshal params for rule %s: %v", ruleID, err)
			continue
		}

		pendingActions = append(pendingActions, PendingAction{
			RuleID:   ruleID,
			DeviceID: action.DeviceID,
			Params:   params,
		})
	}

	return pendingActions
}

// extractActionTargets identifies all device-attribute pairs affected by pending actions
func extractActionTargets(actions []PendingAction) []ActionTarget {
	targetMap := make(map[ActionTarget]bool)

	for _, action := range actions {
		for key := range action.Params {
			target := ActionTarget{
				DeviceID: action.DeviceID,
				Key:      key,
			}
			targetMap[target] = true
		}
	}

	targets := []ActionTarget{}
	for target := range targetMap {
		targets = append(targets, target)
	}

	return targets
}

// resolveConflicts selects final values for conflicting actions based on rule ID priority
func resolveConflicts(pendingActions []PendingAction, targets []ActionTarget) map[string]map[string]interface{} {
	// Map of deviceID -> attribute -> (ruleID, value)
	conflictMap := make(map[string]map[string]struct {
		ruleID string
		value  interface{}
	})

	// Process each pending action
	for _, action := range pendingActions {
		if conflictMap[action.DeviceID] == nil {
			conflictMap[action.DeviceID] = make(map[string]struct {
				ruleID string
				value  interface{}
			})
		}

		for key, value := range action.Params {
			existing, exists := conflictMap[action.DeviceID][key]

			if !exists {
				// First action for this device-attribute
				conflictMap[action.DeviceID][key] = struct {
					ruleID string
					value  interface{}
				}{
					ruleID: action.RuleID,
					value:  value,
				}
				log.Printf("TASKQUEUE: Device %s, attr %s: Initial value from rule %s", action.DeviceID, key, action.RuleID)
			} else if action.RuleID < existing.ruleID {
				// Conflict detected - resolve by selecting lower rule ID (priority)
				log.Printf("TASKQUEUE: Conflict detected for device %s, attr %s: rule %s vs %s - selecting %s",
					action.DeviceID, key, action.RuleID, existing.ruleID, action.RuleID)
				conflictMap[action.DeviceID][key] = struct {
					ruleID string
					value  interface{}
				}{
					ruleID: action.RuleID,
					value:  value,
				}
			} else {
				log.Printf("TASKQUEUE: Conflict detected for device %s, attr %s: rule %s vs %s - keeping %s",
					action.DeviceID, key, action.RuleID, existing.ruleID, existing.ruleID)
			}
		}
	}

	// Build resolved actions map: deviceID -> params
	resolvedActions := make(map[string]map[string]interface{})
	for deviceID, attrs := range conflictMap {
		resolvedActions[deviceID] = make(map[string]interface{})
		for key, entry := range attrs {
			resolvedActions[deviceID][key] = entry.value
		}
	}

	return resolvedActions
}

func evaluateAndExecuteTask(ctx context.Context, t *asynq.Task) error {
	var payload EvaluationTaskPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return err
	}

	rule, err := dbConn.GetRuleByID(ctx, payload.RuleID)
	if err != nil {
		log.Printf("TASKQUEUE: Failed to fetch rule %s: %v", payload.RuleID, err)
		return err
	}

	if !rule.Enabled {
		return nil
	}

	if areAllActionsRedundant(ctx, rule.Actions) {
		return nil
	}

	result := automation.EvaluateConditions(redisClient, rule.Conditions)

	if result {
		log.Printf("TASKQUEUE: Rule %s (%s) conditions met, collecting pending actions", payload.RuleID, rule.Name)

		// Collect pending actions from this rule
		pendingActions := collectPendingActions(rule.ID, rule.Actions)

		// Find all affected device-attribute targets
		affectedTargets := extractActionTargets(pendingActions)

		// For each target, evaluate ALL active automations that could affect it
		allRules, err := dbConn.GetAllRules(ctx)
		if err != nil {
			log.Printf("TASKQUEUE: Failed to fetch all rules: %v", err)
			return err
		}

		// Collect pending actions from all active rules affecting the same targets
		allPendingActions := []PendingAction{}
		for _, r := range allRules {
			if !r.Enabled || r.ID == rule.ID {
				// Skip disabled rules and the current rule (already added)
				continue
			}

			// Evaluate this rule's conditions
			if automation.EvaluateConditions(redisClient, r.Conditions) {
				log.Printf("TASKQUEUE: Rule %s (%s) also triggered", r.ID, r.Name)
				ruleActions := collectPendingActions(r.ID, r.Actions)

				// Only include actions that affect the same targets
				for _, action := range ruleActions {
					for _, target := range affectedTargets {
						if action.DeviceID == target.DeviceID {
							// Check if this action affects the same attribute
							if _, hasKey := action.Params[target.Key]; hasKey {
								allPendingActions = append(allPendingActions, action)
								break
							}
						}
					}
				}
			}
		}

		// Add this rule's actions to the collection
		allPendingActions = append(allPendingActions, pendingActions...)

		// Resolve conflicts and execute final actions
		resolvedActions := resolveConflicts(allPendingActions, affectedTargets)
		if len(resolvedActions) > 0 {
			log.Printf("TASKQUEUE: Executing %d resolved actions after conflict resolution", len(resolvedActions))
			automation.ExecuteResolvedActions(mqttClient, resolvedActions)
		}
	}

	return nil
}
