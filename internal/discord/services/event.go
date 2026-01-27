package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"osrs-events/internal/database"
	"osrs-events/internal/firebase"
)

type EventService struct {
	store           EventStore
	snapshotService *SnapshotService
	firebaseClient  *firebase.RemoteConfigClient
}

func NewEventService(store EventStore, snapshotService *SnapshotService, firebaseClient *firebase.RemoteConfigClient) *EventService {
	return &EventService{
		store:           store,
		snapshotService: snapshotService,
		firebaseClient:  firebaseClient,
	}
}

type StartEventResult struct {
	Event      *database.Event
	MetricName string
}

func (s *EventService) StartBotw(ctx context.Context, guildID string, startTime time.Time) (*StartEventResult, error) {
	isRunning, err := s.IsEventRunning(ctx, guildID, "botw")
	if err != nil {
		return nil, fmt.Errorf("failed to check event status: %w", err)
	}

	if isRunning {
		activeEvent, _ := s.GetActiveEvent(ctx, guildID, "botw")
		if activeEvent != nil {
			return nil, fmt.Errorf("⏰ A BOTW competition is already running! Ends: %s", activeEvent.EndTime.Format("2006-01-02 15:04"))
		}
		return nil, fmt.Errorf("⏰ A BOTW competition is already running!")
	}

	bossConfig, err := s.firebaseClient.GetRandomBoss(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch random boss: %w", err)
	}

	bossesToTrackJSON, err := json.Marshal(bossConfig.BossesToTrack)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal bosses to track: %w", err)
	}

	weekNumber, err := s.GetNextWeekNumber(ctx, guildID, "botw")
	if err != nil {
		return nil, fmt.Errorf("failed to get week number: %w", err)
	}

	event := &database.Event{
		GuildID:       guildID,
		Type:          "botw",
		WeekNumber:    weekNumber,
		MetricJsonID:  bossConfig.Name,
		BossesToTrack: string(bossesToTrackJSON),
		StartTime:     startTime,
		PointsPerKC:   bossConfig.PointsPerKC,
		ThresholdKC:   bossConfig.ThresholdKC,
	}

	if err := s.CreateEvent(ctx, event); err != nil {
		return nil, fmt.Errorf("failed to create event: %w", err)
	}

	// Only create snapshots if start time is now or in the past
	// If start time is in the future (during rollover), snapshots will be created by scheduler when event actually starts
	nowUTC := time.Now().UTC()
	startTimeUTC := startTime.UTC()
	if startTimeUTC.Before(nowUTC) || startTimeUTC.Equal(nowUTC) {
		if _, err := s.snapshotService.CreateInitialSnapshots(ctx, event.ID, guildID, bossConfig.Name, "boss"); err != nil {
			return nil, fmt.Errorf("failed to create initial snapshots: %w", err)
		}
	}

	return &StartEventResult{
		Event:      event,
		MetricName: bossConfig.Name,
	}, nil
}

func (s *EventService) StartSotw(ctx context.Context, guildID string, startTime time.Time) (*StartEventResult, error) {
	isRunning, err := s.IsEventRunning(ctx, guildID, "sotw")
	if err != nil {
		return nil, fmt.Errorf("failed to check event status: %w", err)
	}

	if isRunning {
		activeEvent, _ := s.GetActiveEvent(ctx, guildID, "sotw")
		if activeEvent != nil {
			return nil, fmt.Errorf("⏰ A SOTW competition is already running! Ends: %s", activeEvent.EndTime.Format("2006-01-02 15:04"))
		}
		return nil, fmt.Errorf("⏰ A SOTW competition is already running!")
	}

	skillConfig, err := s.firebaseClient.GetRandomSkill(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch random skill: %w", err)
	}

	weekNumber, err := s.GetNextWeekNumber(ctx, guildID, "sotw")
	if err != nil {
		return nil, fmt.Errorf("failed to get week number: %w", err)
	}

	event := &database.Event{
		GuildID:      guildID,
		Type:         "sotw",
		WeekNumber:   weekNumber,
		MetricJsonID: skillConfig.Name,
		StartTime:    startTime,
		PointsPerXP:  skillConfig.PointsPerXP,
		XPThreshold:  skillConfig.XPThreshold,
	}

	if err := s.CreateEvent(ctx, event); err != nil {
		return nil, fmt.Errorf("failed to create event: %w", err)
	}

	// Only create snapshots if start time is now or in the past
	// If start time is in the future (during rollover), snapshots will be created by scheduler when event actually starts
	nowUTC := time.Now().UTC()
	startTimeUTC := startTime.UTC()
	if startTimeUTC.Before(nowUTC) || startTimeUTC.Equal(nowUTC) {
		if _, err := s.snapshotService.CreateInitialSnapshots(ctx, event.ID, guildID, skillConfig.Name, "skill"); err != nil {
			return nil, fmt.Errorf("failed to create initial snapshots: %w", err)
		}
	}

	return &StartEventResult{
		Event:      event,
		MetricName: skillConfig.Name,
	}, nil
}

func (s *EventService) IsEventRunning(ctx context.Context, guildID, eventType string) (bool, error) {
	event, err := s.store.GetActiveEvent(ctx, guildID, eventType)
	if err != nil {
		if err.Error() == "no active event found" {
			return false, nil
		}
		return false, err
	}

	return time.Now().UTC().Before(event.EndTime.UTC()), nil
}

func (s *EventService) CreateEvent(ctx context.Context, event *database.Event) error {
	event.EndTime = event.StartTime.Add(7 * 24 * time.Hour)
	event.IsActive = true
	return s.store.CreateEvent(ctx, event)
}

func (s *EventService) GetActiveEvent(ctx context.Context, guildID, eventType string) (*database.Event, error) {
	return s.store.GetActiveEvent(ctx, guildID, eventType)
}

func (s *EventService) GetNextWeekNumber(ctx context.Context, guildID, eventType string) (int, error) {
	events, err := s.store.GetActiveEvents(ctx, guildID, eventType)
	if err != nil {
		return 1, nil
	}

	maxWeek := 0
	for _, event := range events {
		if event.WeekNumber > maxWeek {
			maxWeek = event.WeekNumber
		}
	}

	return maxWeek + 1, nil
}

func (s *EventService) CompleteEvent(ctx context.Context, event *database.Event) error {
	// Final snapshot update (10 minutes before end, handled by GetExpiringEvents query)
	_, err := s.snapshotService.UpdateSnapshotsForEvent(ctx, event)
	if err != nil {
		return fmt.Errorf("failed to update final snapshots: %w", err)
	}

	if err := s.snapshotService.CalculateAndAwardPoints(ctx, event); err != nil {
		return fmt.Errorf("failed to calculate points: %w", err)
	}

	if err := s.store.DeactivateEvent(ctx, event.ID); err != nil {
		return fmt.Errorf("failed to deactivate event: %w", err)
	}

	return nil
}

func (s *EventService) AutoRollover(ctx context.Context, guildID string, eventType string, startTime time.Time) (*StartEventResult, error) {
	if eventType == "botw" {
		result, err := s.StartBotw(ctx, guildID, startTime)
		return result, err
	}

	result, err := s.StartSotw(ctx, guildID, startTime)
	return result, err
}
