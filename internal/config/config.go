package config

import (
	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

// Config holds application configuration
type Config struct {
	DBURL        string `mapstructure:"DB_URL"`
	RedisAddr    string `mapstructure:"REDIS_ADDR"`
	MQTTBroker   string `mapstructure:"MQTT_BROKER"`
	MQTTClientID string `mapstructure:"MQTT_CLIENT_ID"`
	LogLevel     string `mapstructure:"LOG_LEVEL"`
	JWTSecret    string `mapstructure:"JWT_SECRET"`
	AgentID      string `mapstructure:"AGENT_ID"`
}

// LoadConfig reads configuration from file, .env, or env vars
func LoadConfig() (*Config, error) {
	// Print all environment variables for debugging
	if err := godotenv.Load(); err != nil {
		println("Error loading .env file: ", err)
	}

	viper.AutomaticEnv()

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")

	cfg := &Config{
		DBURL:        viper.GetString("DB_URL"),
		RedisAddr:    viper.GetString("REDIS_ADDR"),
		MQTTBroker:   viper.GetString("MQTT_BROKER"),
		MQTTClientID: viper.GetString("MQTT_CLIENT_ID"),
		LogLevel:     viper.GetString("LOG_LEVEL"),
		JWTSecret:    viper.GetString("JWT_SECRET"),
		AgentID:      viper.GetString("AGENT_ID"),
	}
	return cfg, nil
}
