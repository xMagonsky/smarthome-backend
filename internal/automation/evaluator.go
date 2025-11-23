package automation

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"smarthome/internal/models"
	"smarthome/internal/utils"

	"github.com/redis/go-redis/v9"
)

// EvaluateConditions evaluates rule conditions
func EvaluateConditions(redisClient *redis.Client, conditionsRaw json.RawMessage) bool {
	var condition models.Condition
	if err := json.Unmarshal(conditionsRaw, &condition); err != nil {
		log.Printf("AUTOMATION: Failed to unmarshal conditions: %v", err)
		return false
	}
	result := evaluateCondition(redisClient, condition)
	log.Printf("AUTOMATION: Condition evaluation completed, result: %t", result)
	return result
}

// evaluateCondition evaluates a single condition recursively
func evaluateCondition(redisClient *redis.Client, cond models.Condition) bool {
	if cond.Operator == "" {
		log.Printf("AUTOMATION: Evaluating leaf condition - Type: %s, Device: %s, Key: %s, Op: %s",
			cond.Type, cond.DeviceID, cond.Key, cond.Op)
		switch cond.Type {
		case "sensor", "device":
			if redisClient == nil {
				log.Printf("AUTOMATION: Redis client not available for device condition")
				return false
			}
			stateRaw, _ := redisClient.Get(context.Background(), fmt.Sprintf("device:%s", cond.DeviceID)).Result()
			var state utils.DeviceState
			json.Unmarshal([]byte(stateRaw), &state)

			// Parse the expected value from JSON
			var expectedValue interface{}
			if err := json.Unmarshal(cond.Value, &expectedValue); err != nil {
				log.Printf("AUTOMATION: Failed to parse condition value: %v", err)
				return false
			}

			actualValue := state[cond.Key]
			result := utils.Compare(actualValue, cond.Op, expectedValue)
			log.Printf("AUTOMATION: Device condition result: %t (%v %s %v)", result, actualValue, cond.Op, expectedValue)
			return result
		case "time":
			if redisClient == nil {
				// Parse the expected value from JSON
				var expectedValue interface{}
				if err := json.Unmarshal(cond.Value, &expectedValue); err != nil {
					log.Printf("AUTOMATION: Failed to parse time condition value: %v", err)
					return false
				}
				result := utils.Compare(utils.GetCurrentTime(), cond.Op, expectedValue)
				log.Printf("AUTOMATION: Time condition result: %t", result)
				return result
			}
			cacheKey := fmt.Sprintf("time:%s:%v", cond.Op, cond.Value)
			cached, _ := redisClient.Get(context.Background(), cacheKey).Result()
			if cached != "" {
				result := cached == "true"
				log.Printf("AUTOMATION: Time condition result: %t", result)
				return result
			}
			// Parse the expected value from JSON
			var expectedValue interface{}
			if err := json.Unmarshal(cond.Value, &expectedValue); err != nil {
				log.Printf("AUTOMATION: Failed to parse time condition value: %v", err)
				return false
			}
			result := utils.Compare(utils.GetCurrentTime(), cond.Op, expectedValue)
			redisClient.Set(context.Background(), cacheKey, fmt.Sprintf("%t", result), 60*time.Second)
			log.Printf("AUTOMATION: Time condition result: %t", result)
			return result
		}
		log.Printf("AUTOMATION: Unknown condition type: %s", cond.Type)
		return false
	}

	log.Printf("AUTOMATION: Evaluating compound condition with operator: %s, %d children", cond.Operator, len(cond.Children))
	for _, child := range cond.Children {
		childResult := evaluateCondition(redisClient, child)
		if cond.Operator == "AND" && !childResult {
			return false
		}
		if cond.Operator == "OR" && childResult {
			return true
		}
	}
	finalResult := cond.Operator == "AND"
	log.Printf("AUTOMATION: Compound condition final result: %t", finalResult)
	return finalResult
}
