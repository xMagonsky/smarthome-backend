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
		log.Printf("TASKQUEUE: Executing actions for rule %s (%s)", payload.RuleID, rule.Name)
		automation.ExecuteActions(mqttClient, rule.Actions)
	}

	return nil
}
