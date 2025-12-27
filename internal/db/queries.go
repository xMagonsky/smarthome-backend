package db

import (
	"context"
	"encoding/json"

	"smarthome/internal/models"
)

// GetAllSchedules fetches all schedules
func (d *DB) GetAllSchedules(ctx context.Context) ([]models.Schedule, error) {
	rows, err := d.pool.Query(ctx, "SELECT id, rule_id, cron_expression, enabled FROM schedules")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var schedules []models.Schedule
	for rows.Next() {
		var s models.Schedule
		if err := rows.Scan(&s.ID, &s.RuleID, &s.CronExpression, &s.Enabled); err != nil {
			return nil, err
		}
		schedules = append(schedules, s)
	}
	return schedules, nil
}

// GetRuleByID fetches a rule
func (d *DB) GetRuleByID(ctx context.Context, id string) (*models.Rule, error) {
	var r models.Rule
	err := d.pool.QueryRow(ctx, "SELECT id, name, conditions, actions, enabled FROM rules WHERE id = $1", id).
		Scan(&r.ID, &r.Name, &r.Conditions, &r.Actions, &r.Enabled)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// UpdateDeviceState updates device state
func (d *DB) UpdateDeviceState(ctx context.Context, id string, state json.RawMessage) error {
	_, err := d.pool.Exec(ctx, "UPDATE devices SET state = $1 WHERE device_id = $2", state, id)
	return err
}

// LogAction logs to history
func (d *DB) LogAction(ctx context.Context, ruleID, deviceID string, state json.RawMessage) error {
	_, err := d.pool.Exec(ctx, "INSERT INTO device_states_history (rule_id, device_id, timestamp, state) VALUES ($1, $2, NOW(), $3)", ruleID, deviceID, state)
	return err
}

// GetAllRules fetches all rules
func (d *DB) GetAllRules(ctx context.Context) ([]models.Rule, error) {
	rows, err := d.pool.Query(ctx, "SELECT id, name, conditions, actions, enabled FROM rules")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []models.Rule
	for rows.Next() {
		var r models.Rule
		if err := rows.Scan(&r.ID, &r.Name, &r.Conditions, &r.Actions, &r.Enabled); err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}
	return rules, nil
}

// GetSchedulesByRuleID fetches schedules for a specific rule
func (d *DB) GetSchedulesByRuleID(ctx context.Context, ruleID string) ([]models.Schedule, error) {
	rows, err := d.pool.Query(ctx, "SELECT id, rule_id, cron_expression, enabled FROM schedules WHERE rule_id = $1", ruleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var schedules []models.Schedule
	for rows.Next() {
		var s models.Schedule
		if err := rows.Scan(&s.ID, &s.RuleID, &s.CronExpression, &s.Enabled); err != nil {
			return nil, err
		}
		schedules = append(schedules, s)
	}
	return schedules, nil
}

// GetDeviceByID fetches a device by ID
func (d *DB) GetDeviceByID(ctx context.Context, id string) (*models.Device, error) {
	var device models.Device
	err := d.pool.QueryRow(ctx, "SELECT device_id, name, type, state, mqtt_topic, accepted, owner_id FROM devices WHERE device_id = $1", id).
		Scan(&device.ID, &device.Name, &device.Type, &device.State, &device.MQTTTopic, &device.Accepted, &device.OwnerID)
	if err != nil {
		return nil, err
	}
	return &device, nil
}

// InsertDevice creates a new device with accepted=false
func (d *DB) InsertDevice(ctx context.Context, id, name, deviceType, mqttTopic string, state json.RawMessage) error {
	_, err := d.pool.Exec(ctx,
		"INSERT INTO devices (device_id, name, type, mqtt_topic, state, accepted) VALUES ($1, $2, $3, $4, $5, false)",
		id, name, deviceType, mqttTopic, state)
	return err
}

// GetPendingDevices fetches all devices with accepted=false
func (d *DB) GetPendingDevices(ctx context.Context) ([]models.Device, error) {
	rows, err := d.pool.Query(ctx, "SELECT device_id, name, type, state, mqtt_topic, accepted, owner_id FROM devices WHERE accepted = false")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var devices []models.Device
	for rows.Next() {
		var device models.Device
		if err := rows.Scan(&device.ID, &device.Name, &device.Type, &device.State, &device.MQTTTopic, &device.Accepted, &device.OwnerID); err != nil {
			return nil, err
		}
		devices = append(devices, device)
	}
	return devices, nil
}

// AcceptDevice updates a device's accepted status to true and optionally sets the owner
func (d *DB) AcceptDevice(ctx context.Context, id, ownerID string) error {
	if ownerID != "" {
		_, err := d.pool.Exec(ctx, "UPDATE devices SET accepted = true, owner_id = $1 WHERE device_id = $2", ownerID, id)
		return err
	}
	_, err := d.pool.Exec(ctx, "UPDATE devices SET accepted = true WHERE device_id = $1", id)
	return err
}

// DeleteDevice removes a device from the database
func (d *DB) DeleteDevice(ctx context.Context, id string) error {
	_, err := d.pool.Exec(ctx, "DELETE FROM devices WHERE device_id = $1", id)
	return err
}
