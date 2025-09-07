package models

import "encoding/json"

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type RegisterRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Email    string `json:"email"`
}

type AddRuleRequest struct {
	Name       string          `json:"name"`
	Conditions json.RawMessage `json:"conditions"`
	Actions    json.RawMessage `json:"actions"`
	Enabled    bool            `json:"enabled"`
}

type UpdateRuleRequest struct {
	Name       *string          `json:"name,omitempty"`
	Conditions *json.RawMessage `json:"conditions,omitempty"`
	Actions    *json.RawMessage `json:"actions,omitempty"`
	Enabled    *bool            `json:"enabled,omitempty"`
}

type User struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}
