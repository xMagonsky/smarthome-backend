package automation

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"smarthome/internal/db"
	"smarthome/internal/utils"

	"github.com/redis/go-redis/v9"
)

// ProcessDeviceUpdate handles device state updates and returns rule IDs to evaluate
func ProcessDeviceUpdate(ctx context.Context, redisClient *redis.Client, dbConn *db.DB, deviceID string, newState utils.DeviceState) ([]string, error) {
	log.Printf("AUTOMATION: Processing device update for %s", deviceID)

	// Get last state from Redis
	lastStateRaw, _ := redisClient.Get(ctx, fmt.Sprintf("device:%s", deviceID)).Result()
	var lastState utils.DeviceState
	if lastStateRaw != "" {
		json.Unmarshal([]byte(lastStateRaw), &lastState)
	}
	log.Printf("AUTOMATION: Last state for device %s: %+v", deviceID, lastState)

	// Check if change is significant
	if !utils.IsSignificantChange(redisClient, dbConn, deviceID, newState, lastState) {
		log.Printf("AUTOMATION: No significant change for device %s, skipping", deviceID)
		return nil, nil
	}

	log.Printf("AUTOMATION: Significant change detected for device %s", deviceID)

	// Update state in Redis
	newStateRaw, _ := json.Marshal(newState)
	redisClient.Set(ctx, fmt.Sprintf("device:%s", deviceID), newStateRaw, time.Hour)

	// Update state in database
	go dbConn.UpdateDeviceState(ctx, deviceID, newStateRaw)

	// Get associated rules
	ruleIDs, _ := redisClient.SMembers(ctx, fmt.Sprintf("device:%s:rules", deviceID)).Result()
	log.Printf("AUTOMATION: Found %d rules for device %s: %v", len(ruleIDs), deviceID, ruleIDs)

	log.Printf("AUTOMATION: Device update processing completed for %s", deviceID)
	return ruleIDs, nil
}
