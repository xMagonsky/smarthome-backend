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
	_, err := d.pool.Exec(ctx, "UPDATE devices SET state = $1 WHERE id = $2", state, id)
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
