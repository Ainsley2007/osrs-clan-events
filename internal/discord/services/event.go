package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"osrs-events/internal/database"
)

type EventService struct {
	store           EventStore
	snapshotService SnapshotManager
	configProvider  EventConfigProvider
}

func NewEventService(store EventStore, snapshotService SnapshotManager, configProvider EventConfigProvider) *EventService {
	return &EventService{
		store:           store,
		snapshotService: snapshotService,
		configProvider:  configProvider,
	}
}

type StartEventResult struct {
	Event          *database.Event
	MetricName     string
	SnapshotResult *InitialSnapshotResult // nil if snapshots weren't created (event starts in future)
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

	previousBoss := ""
	if events, err := s.store.GetAllEventsByGuildAndType(ctx, guildID, "botw"); err == nil && len(events) > 0 {
		previousBoss = events[0].MetricJsonID
	}
	bossConfig, err := s.configProvider.GetRandomBoss(ctx, previousBoss)
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

	snapshotResult, err := s.createSnapshotsIfStarted(ctx, event, startTime, bossConfig.Name, "boss")
	if err != nil {
		return nil, err
	}

	return &StartEventResult{
		Event:          event,
		MetricName:     bossConfig.Name,
		SnapshotResult: snapshotResult,
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

	previousSkill := ""
	if events, err := s.store.GetAllEventsByGuildAndType(ctx, guildID, "sotw"); err == nil && len(events) > 0 {
		previousSkill = events[0].MetricJsonID
	}
	skillConfig, err := s.configProvider.GetRandomSkill(ctx, previousSkill)
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

	snapshotResult, err := s.createSnapshotsIfStarted(ctx, event, startTime, skillConfig.Name, "skill")
	if err != nil {
		return nil, err
	}

	return &StartEventResult{
		Event:          event,
		MetricName:     skillConfig.Name,
		SnapshotResult: snapshotResult,
	}, nil
}

// createSnapshotsIfStarted creates initial snapshots when the event's start time is now or in the past.
// During rollover the new event starts at the old event's end time, so this fires immediately.
func (s *EventService) createSnapshotsIfStarted(ctx context.Context, event *database.Event, startTime time.Time, metricName, metricType string) (*InitialSnapshotResult, error) {
	if startTime.UTC().After(time.Now().UTC()) {
		return nil, nil
	}
	result, err := s.snapshotService.CreateInitialSnapshotsWithResult(ctx, event.ID, event.GuildID, metricName, metricType)
	if err != nil {
		return nil, fmt.Errorf("failed to create initial snapshots: %w", err)
	}
	return result, nil
}

func (s *EventService) IsEventRunning(ctx context.Context, guildID, eventType string) (bool, error) {
	event, err := s.store.GetActiveEvent(ctx, guildID, eventType)
	if err != nil {
		if errors.Is(err, database.ErrNoActiveEvent) {
			return false, nil
		}
		return false, err
	}

	return time.Now().UTC().Before(event.EndTime.UTC()), nil
}

func (s *EventService) CreateEvent(ctx context.Context, event *database.Event) error {
	// Events run for exactly 7 days (± a few seconds for processing)
	event.EndTime = event.StartTime.Add(7 * 24 * time.Hour)
	event.IsActive = true
	return s.store.CreateEvent(ctx, event)
}

func (s *EventService) GetActiveEvent(ctx context.Context, guildID, eventType string) (*database.Event, error) {
	return s.store.GetActiveEvent(ctx, guildID, eventType)
}

func (s *EventService) GetNextWeekNumber(ctx context.Context, guildID, eventType string) (int, error) {
	events, err := s.store.GetAllEventsByGuildAndType(ctx, guildID, eventType)
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
	_, err := s.snapshotService.UpdateSnapshotsForEvent(ctx, event)
	if err != nil {
		return fmt.Errorf("failed to update final snapshots: %w", err)
	}

	return s.CompleteEventWithoutSnapshotUpdate(ctx, event)
}

// CompleteEventWithoutSnapshotUpdate completes an event without updating snapshots (assumes they're already updated)
func (s *EventService) CompleteEventWithoutSnapshotUpdate(ctx context.Context, event *database.Event) error {
	if err := s.snapshotService.CalculateAndAwardPoints(ctx, event); err != nil {
		return fmt.Errorf("failed to calculate points: %w", err)
	}

	if err := s.store.DeactivateEvent(ctx, event.ID); err != nil {
		return fmt.Errorf("failed to deactivate event: %w", err)
	}

	return nil
}

// StartNewEvent creates a new event after the old one has been completed
// This is used during rollover to ensure old event is fully processed before new one starts
func (s *EventService) StartNewEvent(ctx context.Context, guildID string, eventType string, startTime time.Time) (*StartEventResult, error) {
	if eventType == "botw" {
		return s.StartBotw(ctx, guildID, startTime)
	}
	return s.StartSotw(ctx, guildID, startTime)
}
