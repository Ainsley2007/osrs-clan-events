package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"osrs-events/internal/database"
	"osrs-events/internal/osrs"
)

type SnapshotService struct {
	store      SnapshotStore
	osrsClient *osrs.Client
}

func NewSnapshotService(store SnapshotStore, osrsClient *osrs.Client) *SnapshotService {
	return &SnapshotService{
		store:      store,
		osrsClient: osrsClient,
	}
}

func (s *SnapshotService) CreateInitialSnapshots(ctx context.Context, eventID int64, guildID, metricName, metricType string) (int, error) {
	accounts, err := s.store.GetAccountsByGuild(ctx, guildID)
	if err != nil {
		return 0, fmt.Errorf("failed to get accounts: %w", err)
	}

	if len(accounts) == 0 {
		return 0, fmt.Errorf("no participants found")
	}

	event, err := s.store.GetEvent(ctx, eventID)
	if err != nil {
		return 0, fmt.Errorf("failed to get event: %w", err)
	}

	for _, account := range accounts {
		value, err := s.fetchMetricValueForEvent(ctx, account.RSN, event)
		if err != nil {
			return 0, fmt.Errorf("failed to fetch stats for %s: %w", account.RSN, err)
		}

		snapshot := &database.Snapshot{
			EventID:      eventID,
			AccountID:    account.ID,
			StartValue:   value,
			CurrentValue: value,
		}

		if err := s.store.CreateSnapshot(ctx, snapshot); err != nil {
			return 0, fmt.Errorf("failed to create snapshot for %s: %w", account.RSN, err)
		}
	}

	return len(accounts), nil
}

func (s *SnapshotService) CreateSnapshotForAccount(ctx context.Context, event *database.Event, account *database.Account) error {
	// Check if snapshot already exists for this account and event
	existingSnapshot, err := s.store.GetSnapshot(ctx, event.ID, account.ID)
	if err == nil && existingSnapshot != nil {
		// Snapshot already exists, update it with new RSN's stats
		return s.UpdateSnapshotForAccount(ctx, event, account, existingSnapshot)
	}

	// Fetch current metric value for this account
	value, err := s.fetchMetricValueForEvent(ctx, account.RSN, event)
	if err != nil {
		return fmt.Errorf("failed to fetch stats for %s: %w", account.RSN, err)
	}

	snapshot := &database.Snapshot{
		EventID:      event.ID,
		AccountID:    account.ID,
		StartValue:   value,
		CurrentValue: value,
	}

	if err := s.store.CreateSnapshot(ctx, snapshot); err != nil {
		return fmt.Errorf("failed to create snapshot for %s: %w", account.RSN, err)
	}

	return nil
}

func (s *SnapshotService) UpdateSnapshotForAccount(ctx context.Context, event *database.Event, account *database.Account, snapshot *database.Snapshot) error {
	// Fetch current metric value with the new RSN
	currentValue, err := s.fetchMetricValueForEvent(ctx, account.RSN, event)
	if err != nil {
		return fmt.Errorf("failed to fetch stats for %s: %w", account.RSN, err)
	}

	// Update the snapshot's current value
	if err := s.store.UpdateSnapshotCurrentValue(ctx, snapshot.ID, currentValue); err != nil {
		return fmt.Errorf("failed to update snapshot for %s: %w", account.RSN, err)
	}

	return nil
}

func (s *SnapshotService) fetchMetricValueForEvent(ctx context.Context, rsn string, event *database.Event) (int64, error) {
	stats, err := s.osrsClient.GetPlayerStats(ctx, rsn)
	if err != nil {
		return 0, err
	}

	if event.Type == "sotw" {
		metricNameLower := strings.ToLower(event.MetricJsonID)
		for _, skill := range stats.Skills {
			if strings.ToLower(skill.Name) == metricNameLower {
				return int64(skill.XP), nil
			}
		}
		return 0, fmt.Errorf("skill %s not found", event.MetricJsonID)
	}

	if event.Type == "botw" {
		var bossesToTrack []string
		if err := json.Unmarshal([]byte(event.BossesToTrack), &bossesToTrack); err != nil {
			return 0, fmt.Errorf("failed to unmarshal bosses to track: %w", err)
		}

		var totalKC int64
		for _, bossName := range bossesToTrack {
			bossNameLower := strings.ToLower(bossName)
			for _, activity := range stats.Activities {
				if strings.ToLower(activity.Name) == bossNameLower {
					totalKC += int64(activity.Score)
					break
				}
			}
		}

		return totalKC, nil
	}

	return 0, fmt.Errorf("invalid event type: %s", event.Type)
}

type FailedAccountUpdate struct {
	RSN   string
	Error error
}

func (s *SnapshotService) UpdateSnapshotsForEvent(ctx context.Context, event *database.Event) ([]FailedAccountUpdate, error) {
	snapshots, err := s.store.GetSnapshotsByEvent(ctx, event.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get snapshots: %w", err)
	}

	var failedUpdates []FailedAccountUpdate

	for _, snapshot := range snapshots {
		account, err := s.store.GetAccount(ctx, snapshot.AccountID)
		if err != nil {
			continue
		}

		currentValue, err := s.fetchMetricValueForEvent(ctx, account.RSN, event)
		if err != nil {
			failedUpdates = append(failedUpdates, FailedAccountUpdate{
				RSN:   account.RSN,
				Error: err,
			})
			continue
		}

		if err := s.store.UpdateSnapshotCurrentValue(ctx, snapshot.ID, currentValue); err != nil {
			return failedUpdates, fmt.Errorf("failed to update snapshot for account %d: %w", account.ID, err)
		}
	}

	return failedUpdates, nil
}

func (s *SnapshotService) CalculateAndAwardPoints(ctx context.Context, event *database.Event) error {
	snapshotsWithAccounts, err := s.store.GetSnapshotsWithAccounts(ctx, event.ID)
	if err != nil {
		return fmt.Errorf("failed to get snapshots with accounts: %w", err)
	}

	pointUpdates := make(map[string]*database.ParticipantPointUpdate)

	for _, swa := range snapshotsWithAccounts {
		gain := swa.Snapshot.CurrentValue - swa.Snapshot.StartValue
		if gain < 0 {
			gain = 0
		}

		var points int
		if event.Type == "botw" {
			points = int(float64(gain) * event.PointsPerKC)
		} else {
			points = int(float64(gain) * event.PointsPerXP)
		}

		key := swa.Account.DiscordUserID + ":" + event.GuildID
		if _, exists := pointUpdates[key]; !exists {
			pointUpdates[key] = &database.ParticipantPointUpdate{
				DiscordUserID: swa.Account.DiscordUserID,
				GuildID:       event.GuildID,
				BotwPoints:    0,
				SotwPoints:    0,
			}
		}

		if event.Type == "botw" {
			pointUpdates[key].BotwPoints += points
		} else {
			pointUpdates[key].SotwPoints += points
		}
	}

	var updates []*database.ParticipantPointUpdate
	for _, update := range pointUpdates {
		updates = append(updates, update)
	}

	if len(updates) > 0 {
		if err := s.store.UpdateParticipantPoints(ctx, updates); err != nil {
			return fmt.Errorf("failed to update participant points: %w", err)
		}
	}

	return nil
}
