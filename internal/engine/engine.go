package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"smarthome/internal/automation"
	"smarthome/internal/db"
	"smarthome/internal/models"
	"smarthome/internal/scheduler"
	"smarthome/internal/taskqueue"
	"smarthome/internal/utils"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/redis/go-redis/v9"
)

// Engine is the core control engine
type Engine struct {
	mqttClient  mqtt.Client
	redisClient *redis.Client
	db          *db.DB
	scheduler   *scheduler.Scheduler
	// Add channels or interfaces for expansion (e.g., event bus)
}

// NewEngine creates a new engine instance
func NewEngine(mqttClient mqtt.Client, redisClient *redis.Client, dbConn *db.DB, sched *scheduler.Scheduler) *Engine {
	return &Engine{
		mqttClient:  mqttClient,
		redisClient: redisClient,
		db:          dbConn,
		scheduler:   sched,
	}
}

// Start starts the engine
func (e *Engine) Start() error {
	// Setup MQTT handlers
	log.Println("Subscribing to MQTT topic: devices/+/state")
	e.mqttClient.Subscribe("devices/+/state", 1, e.onDeviceUpdate)

	// Load all schedules using the scheduler's LoadSchedules method
	log.Println("Loading schedules from database via scheduler")
	if err := e.scheduler.LoadSchedules(); err != nil {
		log.Printf("Error loading schedules: %v", err)
		return err
	}

	// Populate device-rule associations in Redis
	log.Println("Populating device-rule associations")
	if err := e.populateDeviceRuleAssociations(); err != nil {
		log.Printf("Error populating device-rule associations: %v", err)
		return err
	}

	log.Println("Engine started")
	return nil
}

// Stop stops the engine
func (e *Engine) Stop() {
	e.mqttClient.Disconnect(250)
	// Add cleanup for Redis, etc.
	log.Println("Engine stopped")
}

// onDeviceUpdate handles MQTT device updates
func (e *Engine) onDeviceUpdate(client mqtt.Client, msg mqtt.Message) {
	log.Printf("Device update received: %s", msg.Topic())
	deviceID := utils.ParseDeviceID(msg.Topic())
	var state utils.DeviceState
	log.Printf("Payload: %s", msg.Payload())
	if err := json.Unmarshal(msg.Payload(), &state); err != nil {
		log.Printf("Error unmarshaling state: %v", err)
		return
	}

	// Enqueue device update task for async processing
	log.Printf("Enqueuing device update for %s", deviceID)
	if err := taskqueue.EnqueueDeviceUpdate(deviceID, state); err != nil {
		log.Printf("Error enqueuing device update: %v", err)
	}
}

// populateDeviceRuleAssociations populates Redis with device-rule associations
func (e *Engine) populateDeviceRuleAssociations() error {
	// Get all rules from database
	rules, err := e.db.GetAllRules(context.Background())
	if err != nil {
		return err
	}

	log.Printf("Found %d rules to process for associations", len(rules))

	// Clear existing associations
	keys, err := e.redisClient.Keys(context.Background(), "device:*:rules").Result()
	if err != nil {
		return err
	}
	for _, key := range keys {
		e.redisClient.Del(context.Background(), key)
	}

	// Process each rule to find device associations
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}

		// Parse conditions to find referenced devices
		var condition models.Condition
		if err := json.Unmarshal(rule.Conditions, &condition); err != nil {
			log.Printf("Error parsing conditions for rule %s: %v", rule.ID, err)
			continue
		}

		// Extract device IDs from the condition tree
		deviceIDs := e.extractDeviceIDsFromConditionTree(condition)

		// Add rule to each device's rule set
		for _, deviceID := range deviceIDs {
			key := fmt.Sprintf("device:%s:rules", deviceID)
			e.redisClient.SAdd(context.Background(), key, rule.ID)
			log.Printf("Associated rule %s with device %s", rule.ID, deviceID)
		}
	}

	return nil
}

// extractDeviceIDsFromConditionTree extracts device IDs from a condition tree
func (e *Engine) extractDeviceIDsFromConditionTree(condition models.Condition) []string {
	deviceIDs := make(map[string]bool)

	// Check the root condition
	if condition.DeviceID != "" {
		deviceIDs[condition.DeviceID] = true
	}

	// Recursively check children
	if len(condition.Children) > 0 {
		childDeviceIDs := e.extractDeviceIDsFromConditions(condition.Children)
		for _, id := range childDeviceIDs {
			deviceIDs[id] = true
		}
	}

	// Convert map to slice
	var result []string
	for id := range deviceIDs {
		result = append(result, id)
	}

	return result
}

// extractDeviceIDsFromConditions extracts device IDs from rule conditions
func (e *Engine) extractDeviceIDsFromConditions(conditions []models.Condition) []string {
	deviceIDs := make(map[string]bool)

	for _, condition := range conditions {
		if condition.DeviceID != "" {
			deviceIDs[condition.DeviceID] = true
		}

		// Recursively check nested conditions
		if len(condition.Children) > 0 {
			childDeviceIDs := e.extractDeviceIDsFromConditions(condition.Children)
			for _, id := range childDeviceIDs {
				deviceIDs[id] = true
			}
		}
	}

	// Convert map to slice
	var result []string
	for id := range deviceIDs {
		result = append(result, id)
	}

	return result
}

// RefreshRuleAssociations refreshes device-rule associations for a specific rule
func (e *Engine) RefreshRuleAssociations(ruleID string) error {
	log.Printf("Refreshing associations for rule %s", ruleID)

	// Get the rule from database
	rule, err := e.db.GetRuleByID(context.Background(), ruleID)
	if err != nil {
		log.Printf("Error fetching rule %s for refresh: %v", ruleID, err)
		return err
	}

	// Remove this rule from all existing device associations
	keys, err := e.redisClient.Keys(context.Background(), "device:*:rules").Result()
	if err != nil {
		log.Printf("Error getting device rule keys: %v", err)
		return err
	}

	for _, key := range keys {
		e.redisClient.SRem(context.Background(), key, ruleID)
	}

	// If rule is enabled, add new associations
	if rule.Enabled {
		// Parse conditions to find referenced devices
		var condition models.Condition
		if err := json.Unmarshal(rule.Conditions, &condition); err != nil {
			log.Printf("Error parsing conditions for rule %s: %v", rule.ID, err)
			return err
		}

		// Extract device IDs from the condition tree
		deviceIDs := e.extractDeviceIDsFromConditionTree(condition)

		// Add rule to each device's rule set
		for _, deviceID := range deviceIDs {
			key := fmt.Sprintf("device:%s:rules", deviceID)
			e.redisClient.SAdd(context.Background(), key, rule.ID)
			log.Printf("Associated rule %s with device %s", rule.ID, deviceID)
		}

		// Refresh schedules for this rule
		e.refreshSchedulesForRule(ruleID)
	}

	log.Printf("Successfully refreshed associations for rule %s", ruleID)
	return nil
}

// createSchedulesFromTimeConditions creates schedule records for time-based conditions in a rule
func (e *Engine) createSchedulesFromTimeConditions(ruleID string, conditionsRaw json.RawMessage, enabled bool) {
	timeConditions := automation.ExtractTimeConditions(conditionsRaw)

	if len(timeConditions) == 0 {
		log.Printf("No time conditions found for rule %s", ruleID)
		return
	}

	log.Printf("Creating %d schedules from time conditions for rule %s", len(timeConditions), ruleID)

	for _, tc := range timeConditions {
		// Generate cron expression
		cronExpr := automation.ConvertToCronExpression(tc)

		// Check if schedule already exists for this rule and cron expression
		existingSchedule, err := e.db.GetScheduleByRuleAndCron(context.Background(), ruleID, cronExpr)

		var scheduleID string
		if err != nil || existingSchedule == nil {
			// Schedule doesn't exist, create it
			scheduleID, err = e.db.CreateSchedule(context.Background(), ruleID, cronExpr, enabled)
			if err != nil {
				log.Printf("Failed to create schedule for rule %s: %v", ruleID, err)
				continue
			}
			log.Printf("Created schedule %s for rule %s: %s", scheduleID, ruleID, cronExpr)
		} else {
			// Schedule exists, update it
			scheduleID = existingSchedule.ID
			if err := e.db.UpdateSchedule(context.Background(), scheduleID, cronExpr, enabled); err != nil {
				log.Printf("Failed to update schedule %s for rule %s: %v", scheduleID, ruleID, err)
				continue
			}
			log.Printf("Updated schedule %s for rule %s: %s", scheduleID, ruleID, cronExpr)
		}

		// Add schedule to the scheduler
		if err := e.scheduler.AddOrUpdateSchedule(scheduleID, ruleID, cronExpr, enabled); err != nil {
			log.Printf("Failed to add schedule %s to scheduler: %v", scheduleID, err)
		}
	}
}

// RemoveRuleAssociations removes all associations for a rule
func (e *Engine) RemoveRuleAssociations(ruleID string) error {
	log.Printf("Removing associations for rule %s", ruleID)

	// Remove this rule from all device associations
	keys, err := e.redisClient.Keys(context.Background(), "device:*:rules").Result()
	if err != nil {
		log.Printf("Error getting device rule keys: %v", err)
		return err
	}

	for _, key := range keys {
		e.redisClient.SRem(context.Background(), key, ruleID)
	}

	// Remove schedules for this rule (including auto-generated ones)
	e.removeSchedulesForRule(ruleID)

	log.Printf("Successfully removed associations for rule %s", ruleID)
	return nil
}

// refreshSchedulesForRule refreshes schedules for a specific rule
// Also automatically creates schedules for time-based conditions
func (e *Engine) refreshSchedulesForRule(ruleID string) {
	log.Printf("Refreshing schedules for rule %s", ruleID)

	// Get the rule to check for time conditions
	rule, err := e.db.GetRuleByID(context.Background(), ruleID)
	if err != nil {
		log.Printf("Error getting rule %s: %v", ruleID, err)
		return
	}

	// Check if rule has time conditions and auto-create schedules
	if automation.HasTimeConditions(rule.Conditions) {
		log.Printf("Rule %s has time conditions, auto-creating schedules", ruleID)
		e.createSchedulesFromTimeConditions(ruleID, rule.Conditions, rule.Enabled)
	}

	// Get existing schedules for this rule
	schedules, err := e.db.GetSchedulesByRuleID(context.Background(), ruleID)
	if err != nil {
		log.Printf("Error getting schedules for rule %s: %v", ruleID, err)
		return
	}

	// Add or update each schedule in the scheduler
	for _, s := range schedules {
		if err := e.scheduler.AddOrUpdateSchedule(s.ID, s.RuleID, s.CronExpression, s.Enabled); err != nil {
			log.Printf("Error adding/updating schedule %s for rule %s: %v", s.ID, ruleID, err)
		}
	}

	log.Printf("Refreshed %d schedules for rule %s", len(schedules), ruleID)
}

// removeSchedulesForRule removes schedules for a specific rule from both scheduler and database
func (e *Engine) removeSchedulesForRule(ruleID string) {
	log.Printf("Removing schedules for rule %s", ruleID)

	// Get schedules for this rule
	schedules, err := e.db.GetSchedulesByRuleID(context.Background(), ruleID)
	if err != nil {
		log.Printf("Error getting schedules for rule %s: %v", ruleID, err)
		return
	}

	// Remove each schedule from scheduler and database
	for _, s := range schedules {
		// Remove from scheduler
		e.scheduler.RemoveSchedule(s.ID)

		// Remove from database
		if err := e.db.DeleteSchedule(context.Background(), s.ID); err != nil {
			log.Printf("Error deleting schedule %s from database: %v", s.ID, err)
		}
	}

	log.Printf("Removed %d schedules for rule %s", len(schedules), ruleID)
}

// RefreshAllRuleAssociations refreshes all device-rule associations
func (e *Engine) RefreshAllRuleAssociations() error {
	log.Println("Refreshing all rule associations")
	return e.populateDeviceRuleAssociations()
}

// ReloadAllSchedules reloads all schedules from the database
func (e *Engine) ReloadAllSchedules() error {
	log.Println("Reloading all schedules")
	return e.scheduler.ReloadSchedules()
}

// TriggerRuleEvaluation triggers immediate evaluation of a rule
func (e *Engine) TriggerRuleEvaluation(ruleID string) {
	log.Printf("Triggering immediate evaluation for rule %s", ruleID)
	taskqueue.EnqueueEvaluation(ruleID, "")
}
