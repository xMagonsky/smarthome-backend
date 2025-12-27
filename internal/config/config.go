package config

import (
	"bufio"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

// Config holds application configuration
type Config struct {
	Database     DatabaseConfig
	Redis        RedisConfig
	MQTT         MQTTConfig
	JWT          JWTConfig
	App          AppConfig
	RemoteAccess RemoteAccess
	MDNS         MDNSConfig
}

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	URL string
}

// RedisConfig holds Redis configuration
type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

// MQTTConfig holds MQTT broker configuration
type MQTTConfig struct {
	Broker   string
	ClientID string
	Username string
	Password string
}

// JWTConfig holds JWT configuration
type JWTConfig struct {
	Secret string
}

// AppConfig holds application-level configuration
type AppConfig struct {
	Port    int
	AgentID string
}

// BridgeConfig holds internet bridge configuration
type RemoteAccess struct {
	Enabled  bool
	PublicWS string
}

// MDNSConfig holds mDNS server configuration
type MDNSConfig struct {
	LocalName string
}

// LoadConfig reads configuration from .env file and environment variables
func LoadConfig() (*Config, error) {
	// Load .env file if it exists (silent fail is OK)
	_ = godotenv.Load()

	cfg := &Config{
		Database: DatabaseConfig{
			URL: getEnv("DATABASE_URL", getEnv("DB_URL", "postgres://postgres:pass@localhost:5432/smarthome?sslmode=disable")),
		},
		Redis: RedisConfig{
			Addr:     getEnv("REDIS_ADDR", "localhost:6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvInt("REDIS_DB", 0),
		},
		MQTT: MQTTConfig{
			Broker:   getEnv("MQTT_BROKER", "tcp://localhost:1883"),
			ClientID: getEnv("MQTT_CLIENT_ID", "smarthome-engine"),
			Username: getEnv("MQTT_USERNAME", ""),
			Password: getEnv("MQTT_PASSWORD", ""),
		},
		JWT: JWTConfig{
			Secret: getEnv("JWT_SECRET", ""),
		},
		App: AppConfig{
			AgentID: getEnv("AGENT_ID", ""),
			Port:    getEnvInt("SERVER_PORT", 5069),
		},
		RemoteAccess: RemoteAccess{
			Enabled:  getEnvBool("REMOTE_ACCESS_ENABLED", true),
			PublicWS: getEnv("REMOTE_ACCESS_WS_URL", "ws://magonsky.scay.net:5069/agent"),
		},
		MDNS: MDNSConfig{
			LocalName: getEnv("MDNS_URL", "smarthome.local"),
		},
	}

	// Generate secrets if not provided
	if err := generateSecrets(cfg); err != nil {
		return nil, fmt.Errorf("error generating secrets: %w", err)
	}

	return cfg, nil
}

// getEnv gets an environment variable with a default fallback
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt gets an integer environment variable with a default fallback
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var intVal int
		if _, err := fmt.Sscanf(value, "%d", &intVal); err == nil {
			return intVal
		}
	}
	return defaultValue
}

// getEnvBool gets a boolean environment variable with a default fallback
func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		value = strings.ToLower(value)
		return value == "true" || value == "1" || value == "yes" || value == "on"
	}
	return defaultValue
}

// generateSecrets generates missing secrets and saves them to .env file
func generateSecrets(cfg *Config) error {
	envFile := ".env"
	modified := false

	// Generate JWT secret if not provided
	if cfg.JWT.Secret == "" {
		secret, err := generateRandomSecret(32)
		if err != nil {
			return fmt.Errorf("failed to generate JWT secret: %w", err)
		}
		cfg.JWT.Secret = secret
		modified = true
		log.Println("Generated new JWT secret")
	}

	// Generate Agent ID if not provided
	if cfg.App.AgentID == "" {
		agentID, err := generateRandomSecret(16)
		if err != nil {
			return fmt.Errorf("failed to generate agent ID: %w", err)
		}
		cfg.App.AgentID = agentID
		modified = true
		log.Println("Generated new Agent ID")
	}

	// Save to .env file if secrets were generated
	if modified {
		if err := saveToEnvFile(envFile, cfg); err != nil {
			log.Printf("Warning: Could not save generated secrets to %s: %v", envFile, err)
			log.Println("Please save these values manually:")
			log.Printf("JWT_SECRET=%s", cfg.JWT.Secret)
			log.Printf("AGENT_ID=%s", cfg.App.AgentID)
		} else {
			log.Printf("Saved generated secrets to %s", envFile)
		}
	}

	return nil
}

// generateRandomSecret generates a random base64-encoded secret
func generateRandomSecret(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// saveToEnvFile updates or appends secrets to .env file
func saveToEnvFile(filename string, cfg *Config) error {
	// Read existing .env file
	envVars := make(map[string]string)
	if file, err := os.Open(filename); err == nil {
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				envVars[parts[0]] = parts[1]
			}
		}
	}

	// Update with new secrets
	envVars["JWT_SECRET"] = cfg.JWT.Secret
	envVars["AGENT_ID"] = cfg.App.AgentID

	// Write back to file
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create .env file: %w", err)
	}
	defer file.Close()

	writer := bufio.NewWriter(file)

	// Write header comment
	fmt.Fprintln(writer, "# SmartHome Backend Environment Configuration")
	fmt.Fprintln(writer, "# Auto-generated secrets - DO NOT COMMIT TO VERSION CONTROL")
	fmt.Fprintln(writer)

	// Write all variables
	for key, value := range envVars {
		fmt.Fprintf(writer, "%s=%s\n", key, value)
	}

	return writer.Flush()
}

// Legacy accessor methods for backward compatibility
func (c *Config) GetDBURL() string        { return c.Database.URL }
func (c *Config) GetRedisAddr() string    { return c.Redis.Addr }
func (c *Config) GetMQTTBroker() string   { return c.MQTT.Broker }
func (c *Config) GetMQTTClientID() string { return c.MQTT.ClientID }
func (c *Config) GetJWTSecret() string    { return c.JWT.Secret }
func (c *Config) GetAgentID() string      { return c.App.AgentID }
