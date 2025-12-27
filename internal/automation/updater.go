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
	// Check if device exists in database
	device, err := dbConn.GetDeviceByID(ctx, deviceID)
	if err != nil {
		println("error fetching device:", err.Error())
		// Device doesn't exist, add it with accepted=false
		log.Printf("AUTOMATION: Device %s not found in database, adding with accepted=false", deviceID)
		newStateRaw, _ := json.Marshal(newState)
		mqttTopic := fmt.Sprintf("devices/%s/state", deviceID)
		if err := dbConn.InsertDevice(ctx, deviceID, deviceID, "unknown", mqttTopic, newStateRaw); err != nil {
			log.Printf("AUTOMATION: Failed to insert new device %s: %v", deviceID, err)
		}
		// Don't process rules for non-accepted devices
		return nil, nil
	}

	// Don't process rules for non-accepted devices
	if !device.Accepted {
		log.Printf("AUTOMATION: Device %s is not accepted, skipping rule processing", deviceID)
		return nil, nil
	}

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
