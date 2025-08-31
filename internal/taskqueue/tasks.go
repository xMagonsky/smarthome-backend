package taskqueue

import (
	"context"
	"encoding/json"
	"fmt"
	"smarthome/internal/db"
	"smarthome/internal/models"
	"smarthome/internal/utils"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/go-redis/redis/v8"
	"github.com/hibiken/asynq"
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
	var condition models.Condition
	if err := json.Unmarshal(conditionsRaw, &condition); err != nil {
		return false
	}
	return evaluateCondition(condition)
}

// evaluateCondition evaluates a single condition recursively
func evaluateCondition(cond models.Condition) bool {
	if cond.Operator == "" {
		switch cond.Type {
		case "sensor", "device":
			if redisClient == nil {
				return false
			}
			stateRaw, _ := redisClient.Get(context.Background(), fmt.Sprintf("device:%s", cond.DeviceID)).Result()
			var state utils.DeviceState
			json.Unmarshal([]byte(stateRaw), &state)
			return utils.Compare(state[cond.Key], cond.Op, cond.Value)
		case "time":
			if redisClient == nil {
				return utils.Compare(utils.GetCurrentTime(), cond.Op, cond.Value)
			}
			cacheKey := fmt.Sprintf("time:%s:%v", cond.Op, cond.Value)
			cached, _ := redisClient.Get(context.Background(), cacheKey).Result()
			if cached != "" {
				return cached == "true"
			}
			result := utils.Compare(utils.GetCurrentTime(), cond.Op, cond.Value)
			redisClient.Set(context.Background(), cacheKey, fmt.Sprintf("%t", result), 60*time.Second)
			return result
		}
		return false
	}

	for _, child := range cond.Children {
		if cond.Operator == "AND" && !evaluateCondition(child) {
			return false
		}
		if cond.Operator == "OR" && evaluateCondition(child) {
			return true
		}
	}
	return cond.Operator == "AND"
}

// ExecuteActions executes rule actions (copied from engine functionality)
func ExecuteActions(actionsRaw json.RawMessage) {
	var actions []models.Action
	if err := json.Unmarshal(actionsRaw, &actions); err != nil {
		return
	}

	for _, action := range actions {
		if action.DeviceID != "" && mqttClient != nil {
			payload, _ := json.Marshal(action.Params)
			mqttClient.Publish(fmt.Sprintf("devices/%s/commands", action.DeviceID), 1, false, payload)
		}
		if action.Action == "send_email" {
			var paramsMap map[string]interface{}
			if err := json.Unmarshal(action.Params, &paramsMap); err == nil {
				if msg, ok := paramsMap["message"].(string); ok {
					go utils.SendNotification(msg)
				}
			}
		}
	}
	// Log action if database is available
	if dbConn != nil {
		go dbConn.LogAction(context.Background(), "", "", nil)
	}
}

// EvaluationTaskPayload for tasks
type EvaluationTaskPayload struct {
	RuleID          string
	UpdatedDeviceID string
}

// EnqueueEvaluation enqueues a rule evaluation task
func EnqueueEvaluation(ruleID, updatedDeviceID string) error {
	payload, _ := json.Marshal(EvaluationTaskPayload{RuleID: ruleID, UpdatedDeviceID: updatedDeviceID})
	task := asynq.NewTask("evaluate_rule", payload)
	_, err := asynqClient.Enqueue(task, asynq.MaxRetry(3), asynq.Timeout(10*time.Second))
	return err
}

// evaluateAndExecuteTask handles the task
func evaluateAndExecuteTask(ctx context.Context, t *asynq.Task) error {
	var payload EvaluationTaskPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return err
	}

	// Fetch rule (from cache or DB)
	rule, err := GetRule(payload.RuleID)
	if err != nil {
		return err
	}

	if !rule.Enabled {
		return nil
	}

	// Evaluate conditions
	result := EvaluateConditions(rule.Conditions)

	if result {
		ExecuteActions(rule.Actions)
		utils.LogAction(payload.RuleID, payload.UpdatedDeviceID, rule) // Placeholder
	}

	return nil
}

// Expand with more task types (e.g., batch processing)
