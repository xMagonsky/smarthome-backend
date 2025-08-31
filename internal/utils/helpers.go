package utils

import (
	"math"
	"strings"
	"time"

	"smarthome/internal/db"

	"github.com/go-redis/redis/v8"
)

// DeviceState type alias
type DeviceState map[string]interface{}

// ParseDeviceID parses topic
func ParseDeviceID(topic string) string {
	parts := strings.Split(topic, "/")
	if len(parts) > 1 {
		return parts[1]
	}
	return ""
}

// Compare compares values
func Compare(actual interface{}, op string, expected interface{}) bool {
	// Handle float64, time.Time, etc.
	aFloat, aOk := actual.(float64)
	eFloat, eOk := expected.(float64)
	if aOk && eOk {
		switch op {
		case ">":
			return aFloat > eFloat
		case "<":
			return aFloat < eFloat
		case "==":
			return aFloat == eFloat
		case "!=":
			return aFloat != eFloat
		}
	}
	// Add more types
	return false
}

// Abs absolute value
func Abs(x float64) float64 {
	return math.Abs(x)
}

// GetCurrentTime gets current time
func GetCurrentTime() time.Time {
	return time.Now()
}

// IsSignificantChange checks changes
func IsSignificantChange(redisClient *redis.Client, db *db.DB, deviceID string, newState, lastState DeviceState) bool {
	// Implementation: Fetch rules, check min_change
	return true // Placeholder
}

// Expand with more helpers (e.g., JSON utils)
