package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	OpenAIAPIKey    string
	OpenAIModel     string
	ServerPort      string
	DBPath          string
	MaxMemoryTokens int
}

// Load reads configuration from a .env file and environment variables.
func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("[config] No .env file found, reading from environment")
	}

	return &Config{
		OpenAIAPIKey:    mustGet("OPENAI_API_KEY"),
		OpenAIModel:     getOrDefault("OPENAI_MODEL", "gpt-4o"),
		ServerPort:      getOrDefault("SERVER_PORT", "8080"),
		DBPath:          getOrDefault("DB_PATH", "./kai.db"),
		MaxMemoryTokens: 8000,
	}
}

func mustGet(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("[config] required env var %q is not set", key)
	}
	return v
}

func getOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
