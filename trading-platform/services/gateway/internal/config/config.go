package config

import (
	"os"
)

type AppConfig struct {
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	RedisURL   string
	JWTSecret  string
	LogLevel   string
}

func Load() *AppConfig {
	return &AppConfig{
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "trading"),
		DBPassword: getEnv("DB_PASSWORD", "changeme_in_production"),
		DBName:     getEnv("DB_NAME", "trading_platform"),
		RedisURL:   getEnv("REDIS_URL", "redis://localhost:6379"),
		JWTSecret:  getEnv("JWT_SECRET", "generate_a_real_secret"),
		LogLevel:   getEnv("LOG_LEVEL", "info"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
