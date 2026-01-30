package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
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

	// Fetch stats once per account and create snapshot
	for _, account := range accounts {
		// Fetch player stats once for this account
		stats, err := s.osrsClient.GetPlayerStats(ctx, account.RSN)
		if err != nil {
			return 0, fmt.Errorf("failed to fetch stats for %s: %w", account.RSN, err)
		}

		// Extract metric value from cached stats
		value, err := s.extractMetricValueFromStats(stats, event)
		if err != nil {
			return 0, fmt.Errorf("failed to extract metric for %s: %w", account.RSN, err)
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

// CreateSnapshotsForAccount creates snapshots for multiple events efficiently by fetching stats once
func (s *SnapshotService) CreateSnapshotsForAccount(ctx context.Context, events []*database.Event, account *database.Account) error {
	if len(events) == 0 {
		return nil
	}

	// Fetch player stats once for this account
	stats, err := s.osrsClient.GetPlayerStats(ctx, account.RSN)
	if err != nil {
		return fmt.Errorf("failed to fetch stats for %s: %w", account.RSN, err)
	}

	// Create snapshots for all events using the cached stats
	for _, event := range events {
		// Check if snapshot already exists
		existingSnapshot, err := s.store.GetSnapshot(ctx, event.ID, account.ID)
		if err == nil && existingSnapshot != nil {
			// Snapshot exists, update it
			currentValue, err := s.extractMetricValueFromStats(stats, event)
			if err != nil {
				return fmt.Errorf("failed to extract metric for event %d: %w", event.ID, err)
			}
			if err := s.store.UpdateSnapshotCurrentValue(ctx, existingSnapshot.ID, currentValue); err != nil {
				return fmt.Errorf("failed to update snapshot for %s: %w", account.RSN, err)
			}
			continue
		}

		// Extract metric value from cached stats
		value, err := s.extractMetricValueFromStats(stats, event)
		if err != nil {
			return fmt.Errorf("failed to extract metric for event %d: %w", event.ID, err)
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

// UpdateSnapshotsForAccount updates snapshots for multiple events for a single account efficiently
func (s *SnapshotService) UpdateSnapshotsForAccount(ctx context.Context, events []*database.Event, account *database.Account) error {
	if len(events) == 0 {
		return nil
	}

	// Fetch player stats once for this account
	stats, err := s.osrsClient.GetPlayerStats(ctx, account.RSN)
	if err != nil {
		return fmt.Errorf("failed to fetch stats for %s: %w", account.RSN, err)
	}

	// Update all snapshots using the cached stats
	for _, event := range events {
		// Get existing snapshot
		snapshot, err := s.store.GetSnapshot(ctx, event.ID, account.ID)
		if err != nil {
			// Snapshot doesn't exist, create it
			value, err := s.extractMetricValueFromStats(stats, event)
			if err != nil {
				return fmt.Errorf("failed to extract metric for event %d: %w", event.ID, err)
			}

			newSnapshot := &database.Snapshot{
				EventID:      event.ID,
				AccountID:    account.ID,
				StartValue:   value,
				CurrentValue: value,
			}

			if err := s.store.CreateSnapshot(ctx, newSnapshot); err != nil {
				return fmt.Errorf("failed to create snapshot for %s: %w", account.RSN, err)
			}
			continue
		}

		// Extract metric value from cached stats
		currentValue, err := s.extractMetricValueFromStats(stats, event)
		if err != nil {
			return fmt.Errorf("failed to extract metric for event %d: %w", event.ID, err)
		}

		// Update the snapshot's current value
		if err := s.store.UpdateSnapshotCurrentValue(ctx, snapshot.ID, currentValue); err != nil {
			return fmt.Errorf("failed to update snapshot for %s: %w", account.RSN, err)
		}
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
	RSN     string
	GuildID string // guild the account/participant belongs to
	Error   error
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
				RSN:     account.RSN,
				GuildID: event.GuildID,
				Error:   err,
			})
			continue
		}

		if err := s.store.UpdateSnapshotCurrentValue(ctx, snapshot.ID, currentValue); err != nil {
			return failedUpdates, fmt.Errorf("failed to update snapshot for account %d: %w", account.ID, err)
		}
	}

	return failedUpdates, nil
}

// snapData pairs an event with an existing snapshot (or nil if a new snapshot should be created).
type snapData struct {
	snapshot *database.Snapshot // nil means create new entry
	event    *database.Event
}

// UpdateSnapshotsForEvents updates snapshots for multiple events efficiently by fetching player stats once per account.
// For accounts that have no snapshot for an active event yet (e.g. added after event start or after startup),
// a new snapshot entry is created with current value as both start and current.
func (s *SnapshotService) UpdateSnapshotsForEvents(ctx context.Context, events []*database.Event) ([]FailedAccountUpdate, error) {
	if len(events) == 0 {
		return nil, nil
	}

	type accountSnapshot struct {
		account   *database.Account
		snapshots []snapData
	}

	accountSnapshotsMap := make(map[int64]*accountSnapshot)
	var failedUpdates []FailedAccountUpdate

	// Set of (eventID, accountID) that already have a snapshot (from existing rows).
	hasSnapshot := make(map[int64]map[int64]struct{})

	// 1) Collect all existing snapshots
	for _, event := range events {
		snapshots, err := s.store.GetSnapshotsByEvent(ctx, event.ID)
		if err != nil {
			log.Printf("Failed to get snapshots for event %d: %v", event.ID, err)
			continue
		}
		if hasSnapshot[event.ID] == nil {
			hasSnapshot[event.ID] = make(map[int64]struct{})
		}
		for _, snapshot := range snapshots {
			hasSnapshot[event.ID][snapshot.AccountID] = struct{}{}
			if accountSnapshotsMap[snapshot.AccountID] == nil {
				account, err := s.store.GetAccount(ctx, snapshot.AccountID)
				if err != nil {
					continue
				}
				accountSnapshotsMap[snapshot.AccountID] = &accountSnapshot{
					account:   account,
					snapshots: nil,
				}
			}
			accountSnapshotsMap[snapshot.AccountID].snapshots = append(accountSnapshotsMap[snapshot.AccountID].snapshots, snapData{snapshot: snapshot, event: event})
		}
	}

	// 2) For each event, add guild accounts that don't have a snapshot yet (create-if-missing)
	for _, event := range events {
		accounts, err := s.store.GetAccountsByGuild(ctx, event.GuildID)
		if err != nil {
			log.Printf("Failed to get accounts for guild %s (event %d): %v", event.GuildID, event.ID, err)
			continue
		}
		eventHas := hasSnapshot[event.ID]
		for _, account := range accounts {
			if _, ok := eventHas[account.ID]; ok {
				continue
			}
			if accountSnapshotsMap[account.ID] == nil {
				accountSnapshotsMap[account.ID] = &accountSnapshot{
					account:   account,
					snapshots: nil,
				}
			}
			accountSnapshotsMap[account.ID].snapshots = append(accountSnapshotsMap[account.ID].snapshots, snapData{snapshot: nil, event: event})
		}
	}

	// 3) Fetch stats once per account; update existing snapshots or create new ones
	for accountID, accSnap := range accountSnapshotsMap {
		stats, err := s.osrsClient.GetPlayerStats(ctx, accSnap.account.RSN)
		if err != nil {
			// Same account can be on multiple servers; one FailedAccountUpdate per guild (from this batch's events)
			seen := make(map[string]struct{})
			for _, sd := range accSnap.snapshots {
				gid := sd.event.GuildID
				if _, ok := seen[gid]; !ok {
					seen[gid] = struct{}{}
					failedUpdates = append(failedUpdates, FailedAccountUpdate{
						RSN:     accSnap.account.RSN,
						GuildID: gid,
						Error:   err,
					})
				}
			}
			continue
		}

		for _, sd := range accSnap.snapshots {
			value, err := s.extractMetricValueFromStats(stats, sd.event)
			if err != nil {
				failedUpdates = append(failedUpdates, FailedAccountUpdate{
					RSN:     accSnap.account.RSN,
					GuildID: sd.event.GuildID,
					Error:   fmt.Errorf("failed to extract metric for event %d: %w", sd.event.ID, err),
				})
				continue
			}

			if sd.snapshot != nil {
				if err := s.store.UpdateSnapshotCurrentValue(ctx, sd.snapshot.ID, value); err != nil {
					return failedUpdates, fmt.Errorf("failed to update snapshot for account %d: %w", accountID, err)
				}
			} else {
				snapshot := &database.Snapshot{
					EventID:      sd.event.ID,
					AccountID:    accSnap.account.ID,
					StartValue:   value,
					CurrentValue: value,
				}
				if err := s.store.CreateSnapshot(ctx, snapshot); err != nil {
					return failedUpdates, fmt.Errorf("failed to create snapshot for account %d, event %d: %w", accountID, sd.event.ID, err)
				}
			}
		}
	}

	return failedUpdates, nil
}

// extractMetricValueFromStats extracts the metric value from already-fetched PlayerStats
func (s *SnapshotService) extractMetricValueFromStats(stats *osrs.PlayerStats, event *database.Event) (int64, error) {
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

func (s *SnapshotService) CalculateAndAwardPoints(ctx context.Context, event *database.Event) error {
	snapshotsWithAccounts, err := s.store.GetSnapshotsWithAccounts(ctx, event.ID)
	if err != nil {
		return fmt.Errorf("failed to get snapshots with accounts: %w", err)
	}

	// Aggregate total gain per participant (Discord user + guild); one user can have multiple accounts.
	type participantKey struct {
		discordUserID string
		guildID       string
	}
	totalGainByParticipant := make(map[participantKey]int64)
	for _, swa := range snapshotsWithAccounts {
		gain := swa.Snapshot.CurrentValue - swa.Snapshot.StartValue
		if gain < 0 {
			gain = 0
		}
		key := participantKey{swa.Account.DiscordUserID, event.GuildID}
		totalGainByParticipant[key] += gain
	}

	var threshold int64
	if event.Type == "botw" {
		threshold = int64(event.ThresholdKC)
	} else {
		threshold = int64(event.XPThreshold)
	}

	pointUpdates := make(map[string]*database.ParticipantPointUpdate)
	for key, totalGain := range totalGainByParticipant {
		if totalGain < threshold {
			continue
		}
		var points int
		if event.Type == "botw" {
			points = int(float64(totalGain) * event.PointsPerKC)
		} else {
			points = int(float64(totalGain) * event.PointsPerXP)
		}
		updateKey := key.discordUserID + ":" + key.guildID
		pointUpdates[updateKey] = &database.ParticipantPointUpdate{
			DiscordUserID: key.discordUserID,
			GuildID:       key.guildID,
			BotwPoints:    0,
			SotwPoints:    0,
		}
		if event.Type == "botw" {
			pointUpdates[updateKey].BotwPoints = points
		} else {
			pointUpdates[updateKey].SotwPoints = points
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
