package config

import (
	"os"

	"osrs-events/internal/proofstorage"

	"github.com/joho/godotenv"
)

type Config struct {
	DiscordToken                 string
	DatabaseURL                  string
	GoogleApplicationCredentials string
	R2                           proofstorage.R2Config
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	return &Config{
		DiscordToken:                 os.Getenv("DISCORD_TOKEN"),
		DatabaseURL:                  os.Getenv("DATABASE_URL"),
		GoogleApplicationCredentials: os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"),
		R2: proofstorage.R2Config{
			AccountID:       os.Getenv("R2_ACCOUNT_ID"),
			AccessKeyID:     os.Getenv("R2_ACCESS_KEY_ID"),
			SecretAccessKey: os.Getenv("R2_SECRET_ACCESS_KEY"),
			Bucket:          envOrDefault("R2_BUCKET", "pb-challenge"),
			PublicBaseURL:   os.Getenv("R2_PUBLIC_BASE_URL"),
		},
	}, nil
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
