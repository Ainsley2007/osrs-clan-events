package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"osrs-events/internal/database"
	"osrs-events/internal/osrs"
)

type SnapshotService struct {
	store      SnapshotStore
	osrsClient *osrs.Client
	logger     Logger
}

func NewSnapshotService(store SnapshotStore, osrsClient *osrs.Client, logger Logger) *SnapshotService {
	return &SnapshotService{
		store:      store,
		osrsClient: osrsClient,
		logger:     logger,
	}
}

type InitialSnapshotResult struct {
	SuccessCount int
	FailedRSNs   []string
	Duration     time.Duration
}

func (s *SnapshotService) CreateInitialSnapshots(ctx context.Context, eventID int64, guildID, metricName, metricType string) (int, error) {
	result, err := s.CreateInitialSnapshotsWithResult(ctx, eventID, guildID, metricName, metricType)
	if err != nil {
		return 0, err
	}
	return result.SuccessCount, nil
}

func (s *SnapshotService) CreateInitialSnapshotsWithResult(ctx context.Context, eventID int64, guildID, metricName, metricType string) (*InitialSnapshotResult, error) {
	startTime := time.Now()
	accounts, err := s.store.GetAccountsByGuild(ctx, guildID)
	if err != nil {
		return nil, fmt.Errorf("failed to get accounts: %w", err)
	}

	if len(accounts) == 0 {
		return nil, fmt.Errorf("no participants found")
	}

	event, err := s.store.GetEvent(ctx, eventID)
	if err != nil {
		return nil, fmt.Errorf("failed to get event: %w", err)
	}

	var failedRSNs []string
	successCount := 0

	for _, account := range accounts {
		stats, err := s.osrsClient.GetPlayerStats(ctx, account.RSN)
		if err != nil {
			failedRSNs = append(failedRSNs, account.RSN)
			continue
		}

		value, err := s.extractMetricValueFromStats(stats, event)
		if err != nil {
			failedRSNs = append(failedRSNs, account.RSN)
			continue
		}

		snapshot := &database.Snapshot{
			EventID:      eventID,
			AccountID:    account.ID,
			StartValue:   value,
			CurrentValue: value,
		}

		if err := s.store.CreateSnapshot(ctx, snapshot); err != nil {
			failedRSNs = append(failedRSNs, account.RSN)
			continue
		}

		successCount++
	}

	if s.logger != nil {
		s.logger.Printf("OSRS API initial snapshots: %d ok, %d failed (%s)", successCount, len(failedRSNs), strings.Join(failedRSNs, ", "))
	}

	return &InitialSnapshotResult{
		SuccessCount: successCount,
		FailedRSNs:   failedRSNs,
		Duration:     time.Since(startTime),
	}, nil
}

func (s *SnapshotService) CreateSnapshotForAccount(ctx context.Context, event *database.Event, account *database.Account) error {
	existingSnapshot, err := s.store.GetSnapshot(ctx, event.ID, account.ID)
	if err == nil && existingSnapshot != nil {
		return s.UpdateSnapshotForAccount(ctx, event, account, existingSnapshot)
	}

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

// CreateSnapshotsForAccount creates snapshots for multiple events by fetching stats once.
func (s *SnapshotService) CreateSnapshotsForAccount(ctx context.Context, events []*database.Event, account *database.Account) error {
	if len(events) == 0 {
		return nil
	}

	stats, err := s.osrsClient.GetPlayerStats(ctx, account.RSN)
	if err != nil {
		return fmt.Errorf("failed to fetch stats for %s: %w", account.RSN, err)
	}

	for _, event := range events {
		existingSnapshot, err := s.store.GetSnapshot(ctx, event.ID, account.ID)
		if err == nil && existingSnapshot != nil {
			currentValue, err := s.extractMetricValueFromStats(stats, event)
			if err != nil {
				return fmt.Errorf("failed to extract metric for event %d: %w", event.ID, err)
			}
			if err := s.store.UpdateSnapshotCurrentValue(ctx, existingSnapshot.ID, currentValue); err != nil {
				return fmt.Errorf("failed to update snapshot for %s: %w", account.RSN, err)
			}
			continue
		}

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
	currentValue, err := s.fetchMetricValueForEvent(ctx, account.RSN, event)
	if err != nil {
		return fmt.Errorf("failed to fetch stats for %s: %w", account.RSN, err)
	}

	if err := s.store.UpdateSnapshotCurrentValue(ctx, snapshot.ID, currentValue); err != nil {
		return fmt.Errorf("failed to update snapshot for %s: %w", account.RSN, err)
	}

	return nil
}

// UpdateSnapshotsForAccount updates snapshots for multiple events for a single account by fetching stats once.
func (s *SnapshotService) UpdateSnapshotsForAccount(ctx context.Context, events []*database.Event, account *database.Account) error {
	if len(events) == 0 {
		return nil
	}

	stats, err := s.osrsClient.GetPlayerStats(ctx, account.RSN)
	if err != nil {
		return fmt.Errorf("failed to fetch stats for %s: %w", account.RSN, err)
	}

	for _, event := range events {
		snapshot, err := s.store.GetSnapshot(ctx, event.ID, account.ID)
		if err != nil {
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

		currentValue, err := s.extractMetricValueFromStats(stats, event)
		if err != nil {
			return fmt.Errorf("failed to extract metric for event %d: %w", event.ID, err)
		}

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
	AccountID     int64
	DiscordUserID string
	RSN           string
	GuildID       string // guild the account/participant belongs to
	Error         error
}

type SuccessfulAccountUpdate struct {
	AccountID     int64
	DiscordUserID string
	RSN           string
	GuildID       string
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
				AccountID:     account.ID,
				DiscordUserID: account.DiscordUserID,
				RSN:           account.RSN,
				GuildID:       event.GuildID,
				Error:         err,
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

// UpdateSnapshotsForEventsResult contains the result of updating snapshots with timing information.
// FetchedStats is populated so rollover can reuse the same stats for initial snapshots of new events (1 API call per account).
type UpdateSnapshotsForEventsResult struct {
	FailedUpdates   []FailedAccountUpdate
	SuccessfulPairs []SuccessfulAccountUpdate
	TotalAccounts   int
	Duration        time.Duration
	FetchedStats    map[int64]*osrs.PlayerStats // key = account ID
}

// UpdateSnapshotsForEvents updates snapshots for multiple events efficiently by fetching player stats once per account.
// For accounts that have no snapshot for an active event yet (e.g. added after event start or after startup),
// a new snapshot entry is created with current value as both start and current.
func (s *SnapshotService) UpdateSnapshotsForEvents(ctx context.Context, events []*database.Event) ([]FailedAccountUpdate, error) {
	result, err := s.UpdateSnapshotsForEventsWithResult(ctx, events)
	if err != nil {
		return nil, err
	}
	return result.FailedUpdates, nil
}

func (s *SnapshotService) UpdateSnapshotsForEventsWithResult(ctx context.Context, events []*database.Event) (*UpdateSnapshotsForEventsResult, error) {
	startTime := time.Now()
	if len(events) == 0 {
		return &UpdateSnapshotsForEventsResult{
			FailedUpdates:   nil,
			SuccessfulPairs: nil,
			TotalAccounts:   0,
			Duration:        time.Since(startTime),
			FetchedStats:    nil,
		}, nil
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
			if s.logger != nil {
				s.logger.Printf("Failed to get snapshots for event %d: %v", event.ID, err)
			}
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
			if s.logger != nil {
				s.logger.Printf("Failed to get accounts for guild %s (event %d): %v", event.GuildID, event.ID, err)
			}
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

	// 3) Fetch stats once per account; update existing snapshots or create new ones; collect stats for rollover reuse
	fetchedStats := make(map[int64]*osrs.PlayerStats, len(accountSnapshotsMap))
	successfulPairs := make([]SuccessfulAccountUpdate, 0, len(accountSnapshotsMap))
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
						AccountID:     accSnap.account.ID,
						DiscordUserID: accSnap.account.DiscordUserID,
						RSN:           accSnap.account.RSN,
						GuildID:       gid,
						Error:         err,
					})
				}
			}
			continue
		}
		fetchedStats[accountID] = stats
		seenSuccessGuild := make(map[string]struct{})
		for _, sd := range accSnap.snapshots {
			gid := sd.event.GuildID
			if _, ok := seenSuccessGuild[gid]; ok {
				continue
			}
			seenSuccessGuild[gid] = struct{}{}
			successfulPairs = append(successfulPairs, SuccessfulAccountUpdate{
				AccountID:     accSnap.account.ID,
				DiscordUserID: accSnap.account.DiscordUserID,
				RSN:           accSnap.account.RSN,
				GuildID:       gid,
			})
		}

		for _, sd := range accSnap.snapshots {
			value, err := s.extractMetricValueFromStats(stats, sd.event)
			if err != nil {
				failedUpdates = append(failedUpdates, FailedAccountUpdate{
					AccountID:     accSnap.account.ID,
					DiscordUserID: accSnap.account.DiscordUserID,
					RSN:           accSnap.account.RSN,
					GuildID:       sd.event.GuildID,
					Error:         fmt.Errorf("failed to extract metric for event %d: %w", sd.event.ID, err),
				})
				continue
			}

			if sd.snapshot != nil {
				if err := s.store.UpdateSnapshotCurrentValue(ctx, sd.snapshot.ID, value); err != nil {
					return &UpdateSnapshotsForEventsResult{
						FailedUpdates:   failedUpdates,
						SuccessfulPairs: successfulPairs,
						TotalAccounts:   len(accountSnapshotsMap),
						Duration:        time.Since(startTime),
					}, fmt.Errorf("failed to update snapshot for account %d: %w", accountID, err)
				}
			} else {
				snapshot := &database.Snapshot{
					EventID:      sd.event.ID,
					AccountID:    accSnap.account.ID,
					StartValue:   value,
					CurrentValue: value,
				}
				if err := s.store.CreateSnapshot(ctx, snapshot); err != nil {
					return &UpdateSnapshotsForEventsResult{
						FailedUpdates:   failedUpdates,
						SuccessfulPairs: successfulPairs,
						TotalAccounts:   len(accountSnapshotsMap),
						Duration:        time.Since(startTime),
					}, fmt.Errorf("failed to create snapshot for account %d, event %d: %w", accountID, sd.event.ID, err)
				}
			}
		}
	}

	if s.logger != nil {
		total := len(accountSnapshotsMap)
		failedRSNSet := make(map[string]struct{})
		for _, f := range failedUpdates {
			failedRSNSet[f.RSN] = struct{}{}
		}
		failedCount := len(failedRSNSet)
		okCount := total - failedCount
		failedRSNs := make([]string, 0, failedCount)
		for rsn := range failedRSNSet {
			failedRSNs = append(failedRSNs, rsn)
		}
		s.logger.Printf("OSRS API: %d fetches OK, %d failed (%s)", okCount, failedCount, strings.Join(failedRSNs, ", "))
	}

	return &UpdateSnapshotsForEventsResult{
		FailedUpdates:   failedUpdates,
		SuccessfulPairs: successfulPairs,
		TotalAccounts:   len(accountSnapshotsMap),
		Duration:        time.Since(startTime),
		FetchedStats:    fetchedStats,
	}, nil
}

// ValidateRolloverCommitReadiness checks that snapshot and account data needed for Rollover Commit
// is readable before any competition state changes.
func (s *SnapshotService) ValidateRolloverCommitReadiness(ctx context.Context, expiringEvents []*database.Event, guildID string) error {
	for _, event := range expiringEvents {
		if _, err := s.store.GetSnapshotsWithAccounts(ctx, event.ID); err != nil {
			return fmt.Errorf("event %d (%s): %w", event.ID, event.Type, err)
		}
	}
	if _, err := s.store.GetAccountsByGuild(ctx, guildID); err != nil {
		return fmt.Errorf("guild %s accounts: %w", guildID, err)
	}
	return nil
}

// CreateInitialSnapshotsForEventsFromStats creates initial snapshot rows for the given events using
// pre-fetched stats (e.g. from UpdateSnapshotsForEventsWithResult). No API calls are made.
// Used at rollover so the same fetch that updated final snapshots also seeds the new events.
func (s *SnapshotService) CreateInitialSnapshotsForEventsFromStats(ctx context.Context, events []*database.Event, statsByAccountID map[int64]*osrs.PlayerStats) error {
	for _, event := range events {
		accounts, err := s.store.GetAccountsByGuild(ctx, event.GuildID)
		if err != nil {
			return fmt.Errorf("failed to get accounts for guild %s: %w", event.GuildID, err)
		}
		for _, account := range accounts {
			stats, ok := statsByAccountID[account.ID]
			if !ok {
				continue
			}
			value, err := s.extractMetricValueFromStats(stats, event)
			if err != nil {
				continue
			}
			snapshot := &database.Snapshot{
				EventID:      event.ID,
				AccountID:    account.ID,
				StartValue:   value,
				CurrentValue: value,
			}
			if err := s.store.CreateSnapshot(ctx, snapshot); err != nil {
				return fmt.Errorf("failed to create snapshot for account %d, event %d: %w", account.ID, event.ID, err)
			}
		}
	}
	return nil
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

	updates := make([]*database.ParticipantPointUpdate, 0, len(pointUpdates))
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
