package taskqueue

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"smarthome/internal/automation"
	"smarthome/internal/db"
	"smarthome/internal/models"
	"smarthome/internal/utils"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
)

// Global instances - these should be initialized by the main application
var (
	dbConn      *db.DB
	redisClient *redis.Client
	mqttClient  mqtt.Client
)

// SetGlobalInstances sets the global database, Redis, and MQTT instances
func SetGlobalInstances(database *db.DB, redis *redis.Client, mqtt mqtt.Client) {
	dbConn = database
	redisClient = redis
	mqttClient = mqtt
}

// GetRule fetches a rule by ID
func GetRule(ruleID string) (*models.Rule, error) {
	if dbConn == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}
	return dbConn.GetRuleByID(context.Background(), ruleID)
}

// evaluateConditions evaluates rule conditions using the automation package
func evaluateConditions(conditionsRaw json.RawMessage) bool {
	return automation.EvaluateConditions(redisClient, conditionsRaw)
}

// executeActions executes rule actions using the automation package
func executeActions(actionsRaw json.RawMessage) {
	automation.ExecuteActions(mqttClient, actionsRaw)
}

// DeviceUpdateTaskPayload for device state update tasks
type DeviceUpdateTaskPayload struct {
	DeviceID string
	State    utils.DeviceState
}

// EvaluationTaskPayload for tasks
type EvaluationTaskPayload struct {
	RuleID          string
	UpdatedDeviceID string
}

// EnqueueDeviceUpdate enqueues a device state update task
func EnqueueDeviceUpdate(deviceID string, state utils.DeviceState) error {
	log.Printf("TASKQUEUE: Enqueuing device update for device %s", deviceID)
	payload, _ := json.Marshal(DeviceUpdateTaskPayload{DeviceID: deviceID, State: state})
	task := asynq.NewTask("device_update", payload)
	info, err := asynqClient.Enqueue(task, asynq.MaxRetry(3), asynq.Timeout(10*time.Second))
	if err != nil {
		log.Printf("TASKQUEUE: Failed to enqueue device update for %s: %v", deviceID, err)
		return err
	}
	log.Printf("TASKQUEUE: Successfully enqueued device update task %s for device %s", info.ID, deviceID)
	return nil
}

// EnqueueEvaluation enqueues a rule evaluation task
func EnqueueEvaluation(ruleID, updatedDeviceID string) error {
	log.Printf("TASKQUEUE: Enqueuing evaluation for rule %s (device: %s)", ruleID, updatedDeviceID)
	payload, _ := json.Marshal(EvaluationTaskPayload{RuleID: ruleID, UpdatedDeviceID: updatedDeviceID})
	task := asynq.NewTask("evaluate_rule", payload)
	info, err := asynqClient.Enqueue(task, asynq.MaxRetry(3), asynq.Timeout(10*time.Second))
	if err != nil {
		log.Printf("TASKQUEUE: Failed to enqueue task for rule %s: %v", ruleID, err)
		return err
	}
	log.Printf("TASKQUEUE: Successfully enqueued task %s for rule %s", info.ID, ruleID)
	return nil
}

// processDeviceUpdateTask handles device state update tasks
func processDeviceUpdateTask(ctx context.Context, t *asynq.Task) error {
	log.Printf("TASKQUEUE: Processing device update task %s", t.Type())
	var payload DeviceUpdateTaskPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		log.Printf("TASKQUEUE: Failed to unmarshal device update payload: %v", err)
		return err
	}

	// Process device update using automation package
	ruleIDs, err := automation.ProcessDeviceUpdate(ctx, redisClient, dbConn, payload.DeviceID, payload.State)
	if err != nil {
		log.Printf("TASKQUEUE: Failed to process device update: %v", err)
		return err
	}

	// Enqueue evaluation for each affected rule
	for _, ruleID := range ruleIDs {
		log.Printf("TASKQUEUE: Enqueuing evaluation for rule %s and device %s", ruleID, payload.DeviceID)
		EnqueueEvaluation(ruleID, payload.DeviceID)
	}

	return nil
}

// evaluateAndExecuteTask handles the task
func evaluateAndExecuteTask(ctx context.Context, t *asynq.Task) error {
	log.Printf("TASKQUEUE: Processing task %s", t.Type())
	var payload EvaluationTaskPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		log.Printf("TASKQUEUE: Failed to unmarshal task payload: %v", err)
		return err
	}
	log.Printf("TASKQUEUE: Evaluating rule %s for device %s", payload.RuleID, payload.UpdatedDeviceID)

	// Fetch rule (from cache or DB)
	rule, err := GetRule(payload.RuleID)
	if err != nil {
		log.Printf("TASKQUEUE: Failed to fetch rule %s: %v", payload.RuleID, err)
		return err
	}
	log.Printf("TASKQUEUE: Fetched rule %s (%s), enabled: %t", rule.ID, rule.Name, rule.Enabled)

	if !rule.Enabled {
		log.Printf("TASKQUEUE: Rule %s is disabled, skipping", payload.RuleID)
		return nil
	}

	// Pre-evaluation check: if the target device is already in the desired state, skip evaluation.
	var actions []models.Action
	if err := json.Unmarshal(rule.Actions, &actions); err != nil {
		log.Printf("TASKQUEUE: Failed to unmarshal actions for pre-check: %v", err)
	} else {
		allActionsRedundant := true
		for _, action := range actions {
			if action.DeviceID != "" {
				var desiredState map[string]interface{}
				if err := json.Unmarshal(action.Params, &desiredState); err != nil {
					allActionsRedundant = false // Cannot determine state, so must evaluate
					break
				}

				stateRaw, err := redisClient.Get(context.Background(), fmt.Sprintf("device:%s", action.DeviceID)).Result()
				if err != nil && err != redis.Nil {
					allActionsRedundant = false // Error getting state, so must evaluate
					break
				}

				var currentState utils.DeviceState
				if stateRaw != "" {
					json.Unmarshal([]byte(stateRaw), &currentState)
				}

				actionIsRedundant := true
				if currentState == nil {
					actionIsRedundant = false
				} else {
					for key, desiredValue := range desiredState {
						if currentValue, ok := currentState[key]; !ok || !utils.Compare(currentValue, "==", desiredValue) {
							actionIsRedundant = false
							break
						}
					}
				}

				if !actionIsRedundant {
					allActionsRedundant = false
					break
				}
			} else {
				// For non-device actions, we can't know if they are redundant, so we must evaluate.
				allActionsRedundant = false
				break
			}
		}

		if allActionsRedundant && len(actions) > 0 {
			log.Printf("TASKQUEUE: All actions for rule %s are redundant. Skipping evaluation.", payload.RuleID)
			return nil
		}
	}

	// Evaluate conditions
	log.Printf("TASKQUEUE: Evaluating conditions for rule %s", payload.RuleID)
	result := evaluateConditions(rule.Conditions)
	log.Printf("TASKQUEUE: Rule %s condition evaluation result: %t", payload.RuleID, result)

	if result {
		log.Printf("TASKQUEUE: Conditions met, executing actions for rule %s", payload.RuleID)
		executeActions(rule.Actions)
		log.Printf("TASKQUEUE: Completed execution for rule %s", payload.RuleID)
	} else {
		log.Printf("TASKQUEUE: Conditions not met for rule %s, skipping actions", payload.RuleID)
	}

	return nil
}

// Expand with more task types (e.g., batch processing)
