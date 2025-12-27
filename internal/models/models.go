package models

import "encoding/json"

// Device represents a device model
type Device struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Type      string          `json:"type"`
	State     json.RawMessage `json:"state"`
	MQTTTopic string          `json:"mqtt_topic"`
	Accepted  bool            `json:"accepted"`
	OwnerID   *string         `json:"owner_id"`
}

// Condition represents a condition in a rule
type Condition struct {
	Type      string          `json:"type"`       // "sensor", "device", "time"
	DeviceID  string          `json:"device_id"`  // For sensor/device conditions
	Key       string          `json:"key"`        // e.g., "temperature", "on"
	Op        string          `json:"op"`         // ">", "<", "==", "!="
	Value     json.RawMessage `json:"value"`      // e.g., 22.5, true, "18:00"
	MinChange float64         `json:"min_change"` // Minimum change to trigger (e.g., 0.1 for temperature)
	Operator  string          `json:"operator"`   // "AND", "OR" for nested conditions
	Children  []Condition     `json:"children"`   // For nested AND/OR logic
}

// Action represents an action in a rule
type Action struct {
	DeviceID string          `json:"device_id"` // Target device (empty for non-device actions)
	Action   string          `json:"action"`    // e.g., "set_state", "send_email"
	Params   json.RawMessage `json:"params"`    // e.g., {"on": false}, {"message": "Alert"}
}

// Rule represents a rule model
type Rule struct {
	ID         string          `json:"id"`
	Name       string          `json:"name"`
	Conditions json.RawMessage `json:"conditions"`
	Actions    json.RawMessage `json:"actions"`
	Enabled    bool            `json:"enabled"`
	OwnerID    string          `json:"owner_id"`
}

// Schedule represents a schedule model
type Schedule struct {
	ID             string `json:"id"`
	RuleID         string `json:"rule_id"`
	CronExpression string `json:"cron_expression"`
	Enabled        bool   `json:"enabled"`
}

// DeviceStateHistory for logging
type DeviceStateHistory struct {
	ID        string          `json:"id"`
	DeviceID  string          `json:"device_id"`
	Timestamp string          `json:"timestamp"`
	State     json.RawMessage `json:"state"`
}

// Expand with more models as needed
