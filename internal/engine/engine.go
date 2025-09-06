package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

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

	// Load and schedule all schedules
	log.Println("Loading schedules from database")
	schedules, err := e.db.GetAllSchedules(context.Background())
	if err != nil {
		log.Printf("Error loading schedules: %v", err)
		return err
	}
	log.Printf("Found %d schedules", len(schedules))
	for _, s := range schedules {
		if s.Enabled {
			ruleID := s.RuleID // capture loop variable
			log.Printf("Scheduling rule %s with cron %s", ruleID, s.CronExpression)
			e.scheduler.AddJob(s.CronExpression, func() {
				go e.handleScheduleTrigger(ruleID)
			})
		}
	}

	// Populate device-rule associations in Redis
	log.Println("Populating device-rule associations")
	if err := e.populateDeviceRuleAssociations(); err != nil {
		log.Printf("Error populating device-rule associations: %v", err)
		return err
	}

	// Start processing Redis streams
	log.Println("Starting Redis stream processing")
	go e.processStreams()

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

	// Add to Redis Stream for debouncing
	streamKey := fmt.Sprintf("stream:device:%s", deviceID)
	log.Printf("Adding to stream %s", streamKey)
	e.redisClient.XAdd(context.Background(), &redis.XAddArgs{
		Stream: streamKey,
		MaxLen: int64(utils.StreamMaxLen),
		Values: map[string]interface{}{
			"state":     string(msg.Payload()),
			"timestamp": time.Now().UnixNano(),
		},
	})
	log.Printf("Added message to stream %s", streamKey)
}

// processStreams processes debounced updates
func (e *Engine) processStreams() {
	log.Println("Starting to process device streams")
	for {
		keys, err := e.redisClient.Keys(context.Background(), "stream:device:*").Result()
		if err != nil {
			log.Printf("Error getting streams: %v", err)
			continue
		}

		// log.Printf("Found %d device streams: %v", len(keys), keys)

		if len(keys) == 0 {
			// log.Println("No streams found, sleeping...")
			time.Sleep(utils.DebounceWindow)
			continue
		}

		ids := make([]string, len(keys))
		for i, key := range keys {
			lastID, err := e.redisClient.Get(context.Background(), "last_read:"+key).Result()
			if err != nil {
				lastID = "0-0"
				log.Printf("No last read ID for %s, using 0-0", key)
			} else {
				log.Printf("Last read ID for %s: %s", key, lastID)
			}
			ids[i] = lastID
		}
		args := redis.XReadArgs{
			Streams: append(keys, ids...),
			Block:   utils.DebounceWindow,
		}
		log.Printf("Reading streams with args: %v", args.Streams)
		streams, err := e.redisClient.XRead(context.Background(), &args).Result()
		if err != nil && err != redis.Nil {
			log.Printf("Error reading streams: %v", err)
			continue
		}

		log.Printf("Read %d streams", len(streams))

		for _, stream := range streams {
			deviceID := strings.TrimPrefix(stream.Stream, "stream:device:")
			log.Printf("Processing stream for device %s with %d messages", deviceID, len(stream.Messages))
			if len(stream.Messages) > 0 {
				latest := stream.Messages[len(stream.Messages)-1]
				var state utils.DeviceState
				json.Unmarshal([]byte(latest.Values["state"].(string)), &state)
				log.Printf("Latest message ID: %s, State: %+v", latest.ID, state)
				e.processBufferedUpdate(deviceID, state)
				// Update last read ID
				e.redisClient.Set(context.Background(), "last_read:"+stream.Stream, latest.ID, 0)
				log.Printf("Updated last read ID for %s to %s", stream.Stream, latest.ID)
			}

			// delete device streams
			// ----
			if len(stream.Messages) > 0 {
				var ids []string
				for _, msg := range stream.Messages {
					ids = append(ids, msg.ID)
				}
				e.redisClient.XDel(context.Background(), stream.Stream, ids...)
				log.Printf("Deleted %d messages from stream %s", len(ids), stream.Stream)
				e.redisClient.Do(context.Background(), "XTRIM", stream.Stream, "MAXLEN", 0)
				log.Printf("Trimmed stream %s to 0 messages", stream.Stream)
				len, err := e.redisClient.XLen(context.Background(), stream.Stream).Result()
				log.Printf("Stream %s length after trim: %d (err: %v)", stream.Stream, len, err)
				if err == nil && len == 0 {
					e.redisClient.Del(context.Background(), stream.Stream)
					log.Printf("Deleted empty stream %s", stream.Stream)
				}
			}
			// ----
		}
	}
}

// processBufferedUpdate handles debounced update
func (e *Engine) processBufferedUpdate(deviceID string, newState utils.DeviceState) {
	log.Printf("Processing buffered update for device %s with state: %+v", deviceID, newState)
	lastStateRaw, _ := e.redisClient.Get(context.Background(), fmt.Sprintf("device:%s", deviceID)).Result()
	var lastState utils.DeviceState
	json.Unmarshal([]byte(lastStateRaw), &lastState)
	log.Printf("Last state for device %s: %+v", deviceID, lastState)

	if !e.isSignificantChange(deviceID, newState, lastState) {
		log.Printf("No significant change for device %s, skipping", deviceID)
		return
	}

	log.Printf("Significant change detected for device %s", deviceID)
	newStateRaw, _ := json.Marshal(newState)
	e.redisClient.Set(context.Background(), fmt.Sprintf("device:%s", deviceID), newStateRaw, time.Hour)

	ruleIDs, _ := e.redisClient.SMembers(context.Background(), fmt.Sprintf("device:%s:rules", deviceID)).Result()
	log.Printf("Found %d rules for device %s: %v", len(ruleIDs), deviceID, ruleIDs)
	for _, ruleID := range ruleIDs {
		log.Printf("Enqueuing evaluation for rule %s and device %s", ruleID, deviceID)
		taskqueue.EnqueueEvaluation(ruleID, deviceID)
	}

	go e.db.UpdateDeviceState(context.Background(), deviceID, newStateRaw)
}

// isSignificantChange checks for significant changes
func (e *Engine) isSignificantChange(deviceID string, newState, lastState utils.DeviceState) bool {
	// Implementation as in previous pseudocode
	// Fetch rules, check min_change, etc.
	// For brevity, assume utils.IsSignificantChange(e.redisClient, e.db, deviceID, newState, lastState)
	return utils.IsSignificantChange(e.redisClient, e.db, deviceID, newState, lastState)
}

// handleScheduleTrigger is called when a schedule is triggered
func (e *Engine) handleScheduleTrigger(ruleID string) {
	// Move the EnqueueEvaluation logic here to avoid import cycle
	taskqueue.EnqueueEvaluation(ruleID, "")
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

	// Remove schedules for this rule
	e.removeSchedulesForRule(ruleID)

	log.Printf("Successfully removed associations for rule %s", ruleID)
	return nil
}

// refreshSchedulesForRule refreshes schedules for a specific rule
func (e *Engine) refreshSchedulesForRule(ruleID string) {
	log.Printf("Refreshing schedules for rule %s", ruleID)

	// Remove existing schedules for this rule (simplified - in a real implementation you'd want more sophisticated schedule management)
	e.removeSchedulesForRule(ruleID)

	// Get schedules for this rule
	schedules, err := e.db.GetSchedulesByRuleID(context.Background(), ruleID)
	if err != nil {
		log.Printf("Error getting schedules for rule %s: %v", ruleID, err)
		return
	}

	// Add enabled schedules
	for _, s := range schedules {
		if s.Enabled {
			capturedRuleID := s.RuleID // capture loop variable
			log.Printf("Adding schedule for rule %s with cron %s", capturedRuleID, s.CronExpression)
			e.scheduler.AddJob(s.CronExpression, func() {
				go e.handleScheduleTrigger(capturedRuleID)
			})
		}
	}
}

// removeSchedulesForRule removes schedules for a specific rule
func (e *Engine) removeSchedulesForRule(ruleID string) {
	// Note: This is a simplified implementation. In a real system, you'd want to maintain
	// a mapping of rule IDs to schedule job IDs to properly remove them from the scheduler
	log.Printf("Removing schedules for rule %s (simplified implementation)", ruleID)
	// For now, this is a placeholder - you'd need to enhance the scheduler
	// to support removing specific jobs by rule ID
}

// RefreshAllRuleAssociations refreshes all device-rule associations
func (e *Engine) RefreshAllRuleAssociations() error {
	log.Println("Refreshing all rule associations")
	return e.populateDeviceRuleAssociations()
}

// TriggerRuleEvaluation triggers immediate evaluation of a rule
func (e *Engine) TriggerRuleEvaluation(ruleID string) {
	log.Printf("Triggering immediate evaluation for rule %s", ruleID)
	taskqueue.EnqueueEvaluation(ruleID, "")
}
