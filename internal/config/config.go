package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DiscordToken                 string
	DatabaseURL                  string
	GoogleApplicationCredentials string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	return &Config{
		DiscordToken:                 os.Getenv("DISCORD_TOKEN"),
		DatabaseURL:                  os.Getenv("DATABASE_URL"),
		GoogleApplicationCredentials: os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"),
	}, nil
}
