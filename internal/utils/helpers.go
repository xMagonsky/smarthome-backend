package utils

import (
	"log"
	"math"
	"strings"
	"time"

	"smarthome/internal/db"

	"github.com/redis/go-redis/v9"
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
	// Handle different type combinations
	switch a := actual.(type) {
	case float64:
		if e, ok := expected.(float64); ok {
			switch op {
			case ">":
				return a > e
			case "<":
				return a < e
			case "==":
				return a == e
			case "!=":
				return a != e
			}
		}
	case string:
		if e, ok := expected.(string); ok {
			switch op {
			case "==":
				return a == e
			case "!=":
				return a != e
			}
		}
	case bool:
		if e, ok := expected.(bool); ok {
			switch op {
			case "==":
				return a == e
			case "!=":
				return a != e
			}
		}
	case time.Time:
		if e, ok := expected.(string); ok {
			// Parse expected time string (e.g., "18:00")
			expectedTime, err := time.Parse("15:04", e)
			if err != nil {
				log.Printf("UTILS: Failed to parse time string %s: %v", e, err)
				return false
			}
			// Compare only hours and minutes
			actualTime := time.Date(0, 1, 1, a.Hour(), a.Minute(), 0, 0, time.UTC)
			expectedTime = time.Date(0, 1, 1, expectedTime.Hour(), expectedTime.Minute(), 0, 0, time.UTC)

			switch op {
			case ">":
				return actualTime.After(expectedTime)
			case "<":
				return actualTime.Before(expectedTime)
			case "==":
				return actualTime.Equal(expectedTime)
			case "!=":
				return !actualTime.Equal(expectedTime)
			}
		}
	}

	log.Printf("UTILS: Unsupported comparison: %T %s %T", actual, op, expected)
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
