package main

import (
	"context"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"osrs-events/internal/config"
	"osrs-events/internal/database"
	"osrs-events/internal/discord"
	"osrs-events/internal/firebase"
	"osrs-events/internal/osrs"
	"osrs-events/internal/scheduler"

	"gopkg.in/natefinch/lumberjack.v2"
)

func main() {
	// ─── Configuration ─────────────────────────────────────────────
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// ─── Logging (stdout + rotating file for persistence) ─────────
	logPath := os.Getenv("LOG_FILE")
	if logPath == "" {
		logPath = "logs/app.log"
	}
	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
		log.Printf("Warning: could not create log directory: %v", err)
	} else {
		log.SetOutput(io.MultiWriter(os.Stdout, &lumberjack.Logger{
			Filename:   logPath,
			MaxSize:    50, // MB
			MaxAge:     24, // hours
			MaxBackups: 2,
		}))
	}

	// ─── Database ─────────────────────────────────────────────────
	dbPath := os.Getenv("DATABASE_PATH")
	if dbPath == "" {
		dbPath = "osrs_events.db"
	}
	db, err := database.NewSQLiteStore(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// ─── Firebase (Remote Config for event definitions) ─────────────
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

	// ─── OSRS API client ───────────────────────────────────────────
	osrsClient := osrs.NewClient()
	log.Println("OSRS API client initialized successfully")

	// ─── Discord bot ──────────────────────────────────────────────
	bot, err := discord.New(cfg.DiscordToken, db, osrsClient, remoteConfigClient)
	if err != nil {
		log.Fatalf("Failed to create Discord bot: %v", err)
	}
	if err := bot.Start(); err != nil {
		log.Fatalf("Failed to start Discord bot: %v", err)
	}
	defer bot.Stop()
	log.Println("Discord bot is running...")

	// Global commands (""). Use a guild ID for instant updates during development.
	if err := bot.RegisterCommands(""); err != nil {
		log.Printf("Failed to register commands: %v", err)
	}

	// ─── Scheduler (rollover, hourly snapshots, pending starts) ───
	sched := scheduler.New(db, bot.EventService, bot.SnapshotService, bot.LeaderboardService, bot.InitializerService, bot.Session)
	sched.Start()
	defer sched.Stop()
	log.Println("Scheduler started successfully")

	// ─── Run until shutdown signal ─────────────────────────────────
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	log.Println("Shutting down...")
}
