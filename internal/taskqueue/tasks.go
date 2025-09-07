package taskqueue

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"smarthome/internal/db"
	"smarthome/internal/models"
	"smarthome/internal/utils"
	"time"

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

// GetRule fetches a rule by ID (copied from engine functionality)
func GetRule(ruleID string) (*models.Rule, error) {
	if dbConn == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}
	return dbConn.GetRuleByID(context.Background(), ruleID)
}

// EvaluateConditions evaluates rule conditions (simplified version)
func EvaluateConditions(conditionsRaw json.RawMessage) bool {
	log.Printf("TASKQUEUE: Starting condition evaluation")
	var condition models.Condition
	if err := json.Unmarshal(conditionsRaw, &condition); err != nil {
		log.Printf("TASKQUEUE: Failed to unmarshal conditions: %v", err)
		return false
	}
	result := evaluateCondition(condition)
	log.Printf("TASKQUEUE: Condition evaluation completed, result: %t", result)
	return result
}

// evaluateCondition evaluates a single condition recursively
func evaluateCondition(cond models.Condition) bool {
	if cond.Operator == "" {
		log.Printf("TASKQUEUE: Evaluating leaf condition - Type: %s, Device: %s, Key: %s, Op: %s",
			cond.Type, cond.DeviceID, cond.Key, cond.Op)
		switch cond.Type {
		case "sensor", "device":
			if redisClient == nil {
				log.Printf("TASKQUEUE: Redis client not available for device condition")
				return false
			}
			stateRaw, _ := redisClient.Get(context.Background(), fmt.Sprintf("device:%s", cond.DeviceID)).Result()
			var state utils.DeviceState
			json.Unmarshal([]byte(stateRaw), &state)

			// Parse the expected value from JSON
			var expectedValue interface{}
			if err := json.Unmarshal(cond.Value, &expectedValue); err != nil {
				log.Printf("TASKQUEUE: Failed to parse condition value: %v", err)
				return false
			}

			actualValue := state[cond.Key]
			result := utils.Compare(actualValue, cond.Op, expectedValue)
			log.Printf("TASKQUEUE: Device condition result: %t (%v %s %v)", result, actualValue, cond.Op, expectedValue)
			return result
		case "time":
			if redisClient == nil {
				// Parse the expected value from JSON
				var expectedValue interface{}
				if err := json.Unmarshal(cond.Value, &expectedValue); err != nil {
					log.Printf("TASKQUEUE: Failed to parse time condition value: %v", err)
					return false
				}
				result := utils.Compare(utils.GetCurrentTime(), cond.Op, expectedValue)
				log.Printf("TASKQUEUE: Time condition result: %t", result)
				return result
			}
			cacheKey := fmt.Sprintf("time:%s:%v", cond.Op, cond.Value)
			cached, _ := redisClient.Get(context.Background(), cacheKey).Result()
			if cached != "" {
				result := cached == "true"
				log.Printf("TASKQUEUE: Time condition result: %t", result)
				return result
			}
			// Parse the expected value from JSON
			var expectedValue interface{}
			if err := json.Unmarshal(cond.Value, &expectedValue); err != nil {
				log.Printf("TASKQUEUE: Failed to parse time condition value: %v", err)
				return false
			}
			result := utils.Compare(utils.GetCurrentTime(), cond.Op, expectedValue)
			redisClient.Set(context.Background(), cacheKey, fmt.Sprintf("%t", result), 60*time.Second)
			log.Printf("TASKQUEUE: Time condition result: %t", result)
			return result
		}
		log.Printf("TASKQUEUE: Unknown condition type: %s", cond.Type)
		return false
	}

	log.Printf("TASKQUEUE: Evaluating compound condition with operator: %s, %d children", cond.Operator, len(cond.Children))
	for _, child := range cond.Children {
		childResult := evaluateCondition(child)
		if cond.Operator == "AND" && !childResult {
			return false
		}
		if cond.Operator == "OR" && childResult {
			return true
		}
	}
	finalResult := cond.Operator == "AND"
	log.Printf("TASKQUEUE: Compound condition final result: %t", finalResult)
	return finalResult
}

// ExecuteActions executes rule actions (copied from engine functionality)
func ExecuteActions(actionsRaw json.RawMessage) {
	log.Printf("TASKQUEUE: Starting action execution")
	var actions []models.Action
	if err := json.Unmarshal(actionsRaw, &actions); err != nil {
		log.Printf("TASKQUEUE: Failed to unmarshal actions: %v", err)
		return
	}
	log.Printf("TASKQUEUE: Executing %d actions", len(actions))

	for _, action := range actions {
		if action.DeviceID != "" && mqttClient != nil {
			payload, _ := json.Marshal(action.Params)
			topic := fmt.Sprintf("devices/%s/commands", action.DeviceID)
			log.Printf("TASKQUEUE: Publishing MQTT command to %s: %s", topic, string(payload))
			mqttClient.Publish(topic, 1, false, payload)
		}
		if action.Action == "send_email" {
			var paramsMap map[string]interface{}
			if err := json.Unmarshal(action.Params, &paramsMap); err == nil {
				if msg, ok := paramsMap["message"].(string); ok {
					log.Printf("TASKQUEUE: Sending notification: %s", msg)
					// Removed utils.SendNotification
				}
			}
		}
	}
	// Log action if database is available
	if dbConn != nil {
		//go dbConn.LogAction(context.Background(), "", "", nil)
	}
	log.Printf("TASKQUEUE: Action execution completed")
}

// EvaluationTaskPayload for tasks
type EvaluationTaskPayload struct {
	RuleID          string
	UpdatedDeviceID string
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
	result := EvaluateConditions(rule.Conditions)
	log.Printf("TASKQUEUE: Rule %s condition evaluation result: %t", payload.RuleID, result)

	if result {
		log.Printf("TASKQUEUE: Conditions met, executing actions for rule %s", payload.RuleID)
		ExecuteActions(rule.Actions)
		log.Printf("TASKQUEUE: Completed execution for rule %s", payload.RuleID)
	} else {
		log.Printf("TASKQUEUE: Conditions not met for rule %s, skipping actions", payload.RuleID)
	}

	return nil
}

// Expand with more task types (e.g., batch processing)
