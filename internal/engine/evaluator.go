package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"smarthome/internal/models"
	"smarthome/internal/utils"
)

// EvaluateConditions evaluates rule conditions recursively
func (e *Engine) EvaluateConditions(cond models.Condition) bool {
	if cond.Operator == "" {
		switch cond.Type {
		case "sensor", "device":
			stateRaw, _ := e.redisClient.Get(context.Background(), fmt.Sprintf("device:%s", cond.DeviceID)).Result()
			var state utils.DeviceState
			json.Unmarshal([]byte(stateRaw), &state)
			return utils.Compare(state[cond.Key], cond.Op, cond.Value)
		case "time":
			cacheKey := fmt.Sprintf("time:%s:%v", cond.Op, cond.Value)
			cached, _ := e.redisClient.Get(context.Background(), cacheKey).Result()
			if cached != "" {
				return cached == "true"
			}
			result := utils.Compare(utils.GetCurrentTime(), cond.Op, cond.Value)
			e.redisClient.Set(context.Background(), cacheKey, fmt.Sprintf("%t", result), 60*time.Second)
			return result
		}
		return false
	}

	for _, child := range cond.Children {
		if cond.Operator == "AND" && !e.EvaluateConditions(child) {
			return false
		}
		if cond.Operator == "OR" && e.EvaluateConditions(child) {
			return true
		}
	}
	return cond.Operator == "AND"
}

// Expand with support for more condition types (e.g., ML-based)
