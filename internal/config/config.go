package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DiscordToken        string
	DatabaseURL         string
	FirebaseCredentials string
}

func Load() (*Config, error) {
	// Load .env file if it exists, but don't fail if it doesn't (production might use real env vars)
	_ = godotenv.Load()

	return &Config{
		DiscordToken:        os.Getenv("DISCORD_TOKEN"),
		DatabaseURL:         os.Getenv("DATABASE_URL"),
		FirebaseCredentials: os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"),
	}, nil
}
