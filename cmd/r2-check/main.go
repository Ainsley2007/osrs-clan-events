package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"osrs-events/internal/config"
	"osrs-events/internal/proofstorage"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	if missing := cfg.R2.MissingEnvVars(); len(missing) > 0 {
		fmt.Fprintf(os.Stderr, "R2 is not fully configured. Missing or empty: %s\n", strings.Join(missing, ", "))
		os.Exit(1)
	}

	store, err := proofstorage.NewR2Store(cfg.R2)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create R2 store: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	sampleURL, err := store.HealthCheck(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "R2 health check failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("R2 check passed")
	fmt.Printf("  bucket: %s\n", cfg.R2.Bucket)
	fmt.Printf("  public base: %s\n", cfg.R2.PublicBaseURL)
	fmt.Printf("  sample proof URL shape: %s\n", sampleURL)
	fmt.Printf("  account ID set: %t\n", strings.TrimSpace(cfg.R2.AccountID) != "")
}
