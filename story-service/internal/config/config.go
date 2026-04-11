package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port         string
	GeminiAPIKey string
	GeminiModel  string
}

func Load() *Config {
	_ = godotenv.Load()
	key := os.Getenv("GEMINI_API_KEY")
	if key == "" {
		log.Fatal("GEMINI_API_KEY is required")
	}
	return &Config{
		Port:         getEnv("PORT", ":8090"),
		GeminiAPIKey: key,
		GeminiModel:  getEnv("GEMINI_MODEL", "gemini-2.5-flash"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
