package config

import (
	"log"
	"os"
	"strconv"
	"time"
)

type PostgresConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
}

type Config struct {
	Postgres    *PostgresConfig
	LogFilePath string
	TgBotToken  string
	AppPort     string
}

// Load reads configuration from environment variables
func Load() *Config {
	pgPort, err := strconv.Atoi(getEnv("POSTGRES_PORT", "5432"))
	if err != nil {
		log.Fatalf("invalid POSTGRES_PORT: %v", err)
	}

	return &Config{
		Postgres: &PostgresConfig{
			Host:     getEnv("POSTGRES_HOST", "94.250.203.149"),
			Port:     pgPort,
			User:     getEnv("POSTGRES_USER", "leetcode_rankings_user"),
			Password: getEnv("POSTGRES_PASSWORD", "leetcode_rankings_pwd"),
			DBName:   getEnv("POSTGRES_DB", "leetcode_rankings"),
			SSLMode:  getEnv("POSTGRES_SSLMODE", "disable"),
		},

		LogFilePath: getEnv("LOG_FILE_PATH", "app.log"),
		TgBotToken:  getEnv("TG_BOT_TOKEN", "8256069245:AAG9R6mTbOd3K_IGCaGeCSEBB-FZSE4cWVA"),
		AppPort:     getEnv("APP_PORT", "8888"),
	}
}

// Helper to get env var or default
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func getTimeEnv(key string, defaultValue int, duration time.Duration) time.Duration {
	if value, exists := os.LookupEnv(key); exists {
		valueInt, _ := strconv.Atoi(value)
		return duration * time.Duration(valueInt)
	}

	return time.Duration(defaultValue) * duration
}
