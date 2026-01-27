package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"osrs-events/internal/config"
	"osrs-events/internal/database"
	"osrs-events/internal/discord"
	"osrs-events/internal/firebase"
	"osrs-events/internal/osrs"
	"osrs-events/internal/scheduler"
)

func main() {
	// 1. Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 2. Initialize dependencies
	// Database
	// Using SQLite for persistence
	dbPath := os.Getenv("DATABASE_PATH")
	if dbPath == "" {
		dbPath = "osrs_events.db"
	}
	db, err := database.NewSQLiteStore(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Firebase
	fbClient, err := firebase.New(context.Background(), cfg.GoogleApplicationCredentials)
	if err != nil {
		log.Fatalf("Failed to initialize Firebase: %v", err)
	}
	log.Println("Firebase initialized successfully")

	remoteConfigClient, err := firebase.NewRemoteConfigClient(context.Background(), fbClient.App)
	if err != nil {
		log.Fatalf("Failed to initialize Remote Config client: %v", err)
	}
	log.Println("Firebase Remote Config initialized successfully")

	// OSRS API Client
	osrsClient := osrs.NewClient()
	log.Println("OSRS API client initialized successfully")

	// 3. Initialize Discord Bot
	bot, err := discord.New(cfg.DiscordToken, db, osrsClient, remoteConfigClient)
	if err != nil {
		log.Fatalf("Failed to create Discord bot: %v", err)
	}

	if err := bot.Start(); err != nil {
		log.Fatalf("Failed to start Discord bot: %v", err)
	}
	defer bot.Stop()

	log.Println("Discord bot is running...")

	// Register commands (Global or Guild-specific)
	// Passing "" for global commands. Updates might take up to an hour for global.
	// Pass a specific Guild ID for instant updates during development.
	if err := bot.RegisterCommands(""); err != nil {
		log.Printf("Failed to register commands: %v", err)
	}

	// 4. Start Scheduler
	sched := scheduler.New(db, bot.EventService, bot.SnapshotService, bot.LeaderboardService, bot.InitializerService, bot.Session)
	sched.Start()
	defer sched.Stop()

	log.Println("Scheduler started successfully")

	// 5. Wait for signal
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("Shutting down...")
}
