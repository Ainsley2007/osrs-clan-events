package firebase

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/joho/godotenv"
)

func init() {
	_ = godotenv.Load(filepath.Join("..", "..", ".env"))
}

func TestFetchOSRSConfig(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	credPath := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	if credPath == "" {
		t.Skip("GOOGLE_APPLICATION_CREDENTIALS not set, skipping integration test")
	}
	if _, err := os.Stat(credPath); err != nil {
		t.Skipf("credentials file not found at %s, skipping integration test", credPath)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fbClient, err := New(ctx, credPath)
	if err != nil {
		t.Fatalf("failed to initialize Firebase client: %v", err)
	}

	rcClient, err := fbClient.RemoteConfig(ctx)
	if err != nil {
		t.Fatalf("failed to initialize Remote Config client: %v", err)
	}

	config, err := rcClient.FetchOSRSConfig(ctx)
	if err != nil {
		t.Fatalf("failed to fetch OSRS config: %v", err)
	}

	if config == nil {
		t.Fatal("expected config to be non-nil")
	}

	if len(config.Bosses) == 0 {
		t.Error("expected at least one boss in config")
	}

	if len(config.Skills) == 0 {
		t.Error("expected at least one skill in config")
	}

	for i, boss := range config.Bosses {
		if boss.Name == "" {
			t.Errorf("boss[%d] has empty name", i)
		}
		if boss.PointsPerKC <= 0 {
			t.Errorf("boss[%d] (%s) has invalid points per KC: %f", i, boss.Name, boss.PointsPerKC)
		}
		if boss.ThresholdKC < 0 {
			t.Errorf("boss[%d] (%s) has invalid threshold KC: %d", i, boss.Name, boss.ThresholdKC)
		}
		if len(boss.BossesToTrack) == 0 {
			t.Errorf("boss[%d] (%s) has no bosses to track", i, boss.Name)
		}
	}

	for i, skill := range config.Skills {
		if skill.Name == "" {
			t.Errorf("skill[%d] has empty name", i)
		}
		if skill.PointsPerXP <= 0 {
			t.Errorf("skill[%d] (%s) has invalid points per XP: %f", i, skill.Name, skill.PointsPerXP)
		}
		if skill.XPThreshold < 0 {
			t.Errorf("skill[%d] (%s) has invalid XP threshold: %d", i, skill.Name, skill.XPThreshold)
		}
	}

	t.Logf("Successfully fetched config: %d bosses, %d skills", len(config.Bosses), len(config.Skills))
	if len(config.Bosses) > 0 {
		t.Logf("Sample boss: %s (%.2f points/KC, threshold: %d KC)",
			config.Bosses[0].Name, config.Bosses[0].PointsPerKC, config.Bosses[0].ThresholdKC)
	}
	if len(config.Skills) > 0 {
		t.Logf("Sample skill: %s (%.5f points/XP, threshold: %d XP)",
			config.Skills[0].Name, config.Skills[0].PointsPerXP, config.Skills[0].XPThreshold)
	}
}

