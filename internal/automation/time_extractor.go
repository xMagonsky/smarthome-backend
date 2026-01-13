package automation

import (
	"encoding/json"
	"fmt"
	"log"

	"smarthome/internal/models"
)

// TimeCondition represents a time-based condition extracted from a rule
type TimeCondition struct {
	Hour     int
	Minute   int
	Operator string
}

// ExtractTimeConditions recursively extracts all time-based conditions from a rule's conditions
func ExtractTimeConditions(conditionsRaw json.RawMessage) []TimeCondition {
	var condition models.Condition
	if err := json.Unmarshal(conditionsRaw, &condition); err != nil {
		log.Printf("TIME_EXTRACTOR: Failed to unmarshal conditions: %v", err)
		return nil
	}

	var timeConditions []TimeCondition
	extractTimeConditionsRecursive(condition, &timeConditions)
	return timeConditions
}

// extractTimeConditionsRecursive recursively processes conditions to find time-based ones
func extractTimeConditionsRecursive(cond models.Condition, timeConditions *[]TimeCondition) {
	// Check if this is a time condition with supported operators
	if cond.Type == "time" && (cond.Op == "==" || cond.Op == "<" || cond.Op == ">") {
		// Parse the time value (e.g., "18:00")
		var timeValue string
		if err := json.Unmarshal(cond.Value, &timeValue); err != nil {
			log.Printf("TIME_EXTRACTOR: Failed to parse time value: %v", err)
			return
		}

		// Parse hour and minute
		var hour, minute int
		if _, err := fmt.Sscanf(timeValue, "%d:%d", &hour, &minute); err != nil {
			log.Printf("TIME_EXTRACTOR: Failed to parse time string '%s': %v", timeValue, err)
			return
		}

		if hour < 0 || hour > 23 || minute < 0 || minute > 59 {
			log.Printf("TIME_EXTRACTOR: Invalid time values: hour=%d, minute=%d", hour, minute)
			return
		}

		*timeConditions = append(*timeConditions, TimeCondition{
			Hour:     hour,
			Minute:   minute,
			Operator: cond.Op,
		})

		log.Printf("TIME_EXTRACTOR: Found time condition: %02d:%02d (op: %s)", hour, minute, cond.Op)
	}

	// Recursively check children
	for _, child := range cond.Children {
		extractTimeConditionsRecursive(child, timeConditions)
	}
}

// ConvertToCronExpression converts a time condition to a cron expression
// Returns a cron expression that triggers at the specified time
// For '<' and '>' operators, it creates a schedule at the boundary time for evaluation
func ConvertToCronExpression(tc TimeCondition) string {
	// Cron format: minute hour day month weekday
	// For all operators (==, <, >), we trigger at the specified time
	// The actual condition evaluation will happen in the rule evaluator
	cronExpr := fmt.Sprintf("%d %d * * *", tc.Minute, tc.Hour)

	switch tc.Operator {
	case "==":
		log.Printf("TIME_EXTRACTOR: Converted time %02d:%02d (==) to cron: %s", tc.Hour, tc.Minute, cronExpr)
	case "<":
		log.Printf("TIME_EXTRACTOR: Converted time %02d:%02d (<) to cron: %s (triggers at boundary for evaluation)", tc.Hour, tc.Minute, cronExpr)
	case ">":
		log.Printf("TIME_EXTRACTOR: Converted time %02d:%02d (>) to cron: %s (triggers at boundary for evaluation)", tc.Hour, tc.Minute, cronExpr)
	}

	return cronExpr
}

// HasTimeConditions checks if a rule contains any time-based conditions
func HasTimeConditions(conditionsRaw json.RawMessage) bool {
	timeConditions := ExtractTimeConditions(conditionsRaw)
	return len(timeConditions) > 0
}
