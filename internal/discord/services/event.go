package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"osrs-events/internal/database"
	"osrs-events/internal/firebase"
	"osrs-events/internal/osrs"
)

type EventService struct {
	store           EventStore
	snapshotService SnapshotManager
	configProvider  OSRSConfigProvider
}

func NewEventService(store EventStore, snapshotService SnapshotManager, configProvider OSRSConfigProvider) *EventService {
	return &EventService{
		store:           store,
		snapshotService: snapshotService,
		configProvider:  configProvider,
	}
}

const recentEventsWeightWindow = 10

type StartEventResult struct {
	Event          *database.Event
	MetricName     string
	SnapshotResult *InitialSnapshotResult
}

func (s *EventService) StartBotw(ctx context.Context, guildID string, startTime time.Time) (*StartEventResult, error) {
	event, metricName, err := s.prepareBotwEvent(ctx, guildID, startTime)
	if err != nil {
		return nil, err
	}
	if err := s.CreateEvent(ctx, event); err != nil {
		return nil, fmt.Errorf("failed to create event: %w", err)
	}
	snapshotResult, err := s.createSnapshotsIfStarted(ctx, event, startTime, metricName, "boss")
	if err != nil {
		return nil, err
	}
	return &StartEventResult{
		Event:          event,
		MetricName:     metricName,
		SnapshotResult: snapshotResult,
	}, nil
}

// prepareBotwEvent builds a BOTW event (not persisted). Caller must CreateEvent and then create initial snapshots.
func (s *EventService) prepareBotwEvent(ctx context.Context, guildID string, startTime time.Time) (*database.Event, string, error) {
	isRunning, err := s.IsEventRunning(ctx, guildID, "botw")
	if err != nil {
		return nil, "", fmt.Errorf("failed to check event status: %w", err)
	}
	if isRunning {
		activeEvent, _ := s.GetActiveEvent(ctx, guildID, "botw")
		if activeEvent != nil {
			return nil, "", fmt.Errorf("⏰ A BOTW competition is already running! Ends: %s", activeEvent.EndTime.Format("2006-01-02 15:04"))
		}
		return nil, "", fmt.Errorf("⏰ A BOTW competition is already running!")
	}
	config, err := s.configProvider.FetchOSRSConfig(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("failed to fetch OSRS config: %w", err)
	}
	if len(config.Bosses) == 0 {
		return nil, "", fmt.Errorf("no bosses configured")
	}
	var recentBosses []string
	if events, err := s.store.GetAllEventsByGuildAndType(ctx, guildID, "botw"); err == nil {
		recentBosses = lastMetricNames(events, recentEventsWeightWindow)
	}
	bossConfig := weightedPickBoss(config.Bosses, recentBosses)
	bossesToTrackJSON, err := json.Marshal(bossConfig.BossesToTrack)
	if err != nil {
		return nil, "", fmt.Errorf("failed to marshal bosses to track: %w", err)
	}
	weekNumber, err := s.GetNextWeekNumber(ctx, guildID, "botw")
	if err != nil {
		return nil, "", fmt.Errorf("failed to get week number: %w", err)
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
	return event, bossConfig.Name, nil
}

func (s *EventService) StartSotw(ctx context.Context, guildID string, startTime time.Time) (*StartEventResult, error) {
	event, metricName, err := s.prepareSotwEvent(ctx, guildID, startTime)
	if err != nil {
		return nil, err
	}
	if err := s.CreateEvent(ctx, event); err != nil {
		return nil, fmt.Errorf("failed to create event: %w", err)
	}
	snapshotResult, err := s.createSnapshotsIfStarted(ctx, event, startTime, metricName, "skill")
	if err != nil {
		return nil, err
	}
	return &StartEventResult{
		Event:          event,
		MetricName:     metricName,
		SnapshotResult: snapshotResult,
	}, nil
}

// prepareSotwEvent builds a SOTW event (not persisted). Caller must CreateEvent and then create initial snapshots.
func (s *EventService) prepareSotwEvent(ctx context.Context, guildID string, startTime time.Time) (*database.Event, string, error) {
	isRunning, err := s.IsEventRunning(ctx, guildID, "sotw")
	if err != nil {
		return nil, "", fmt.Errorf("failed to check event status: %w", err)
	}
	if isRunning {
		activeEvent, _ := s.GetActiveEvent(ctx, guildID, "sotw")
		if activeEvent != nil {
			return nil, "", fmt.Errorf("⏰ A SOTW competition is already running! Ends: %s", activeEvent.EndTime.Format("2006-01-02 15:04"))
		}
		return nil, "", fmt.Errorf("⏰ A SOTW competition is already running!")
	}
	config, err := s.configProvider.FetchOSRSConfig(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("failed to fetch OSRS config: %w", err)
	}
	if len(config.Skills) == 0 {
		return nil, "", fmt.Errorf("no skills configured")
	}
	var recentSkills []string
	if events, err := s.store.GetAllEventsByGuildAndType(ctx, guildID, "sotw"); err == nil {
		recentSkills = lastMetricNames(events, recentEventsWeightWindow)
	}
	skillConfig := weightedPickSkill(config.Skills, recentSkills)
	weekNumber, err := s.GetNextWeekNumber(ctx, guildID, "sotw")
	if err != nil {
		return nil, "", fmt.Errorf("failed to get week number: %w", err)
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
	return event, skillConfig.Name, nil
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

// StartNewEvent creates a new event after the old one has been completed (e.g. manual /start).
// Uses the API to create initial snapshots.
func (s *EventService) StartNewEvent(ctx context.Context, guildID string, eventType string, startTime time.Time) (*StartEventResult, error) {
	if eventType == "botw" {
		return s.StartBotw(ctx, guildID, startTime)
	}
	return s.StartSotw(ctx, guildID, startTime)
}

// StartNewEventFromRollover creates a new event and seeds initial snapshots from pre-fetched stats.
// Used by the scheduler so one API fetch per account serves both final snapshots and initial snapshots.
func (s *EventService) StartNewEventFromRollover(ctx context.Context, guildID string, eventType string, startTime time.Time, statsByAccountID map[int64]*osrs.PlayerStats) (*StartEventResult, error) {
	var event *database.Event
	var metricName string
	var err error
	if eventType == "botw" {
		event, metricName, err = s.prepareBotwEvent(ctx, guildID, startTime)
	} else {
		event, metricName, err = s.prepareSotwEvent(ctx, guildID, startTime)
	}
	if err != nil {
		return nil, err
	}
	if err := s.CreateEvent(ctx, event); err != nil {
		return nil, fmt.Errorf("failed to create event: %w", err)
	}
	if err := s.snapshotService.CreateInitialSnapshotsForEventsFromStats(ctx, []*database.Event{event}, statsByAccountID); err != nil {
		return nil, fmt.Errorf("failed to create initial snapshots from cache: %w", err)
	}
	return &StartEventResult{
		Event:          event,
		MetricName:     metricName,
		SnapshotResult: nil,
	}, nil
}

func lastMetricNames(events []*database.Event, n int) []string {
	if n <= 0 || len(events) == 0 {
		return nil
	}
	if len(events) < n {
		n = len(events)
	}
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		if m := events[i].MetricJsonID; m != "" {
			out = append(out, m)
		}
	}
	return out
}

func countOccurrences(names []string) map[string]int {
	counts := make(map[string]int)
	for _, n := range names {
		if n == "" {
			continue
		}
		counts[strings.ToLower(n)]++
	}
	return counts
}

func weightedPickBoss(bosses []firebase.BossConfig, recentNames []string) *firebase.BossConfig {
	return weightedPick(bosses, recentNames, func(b firebase.BossConfig) string { return b.Name })
}

func weightedPickSkill(skills []firebase.SkillConfig, recentNames []string) *firebase.SkillConfig {
	return weightedPick(skills, recentNames, func(s firebase.SkillConfig) string { return s.Name })
}

func weightedPick[T any](items []T, recentNames []string, nameOf func(T) string) *T {
	counts := countOccurrences(recentNames)
	var totalWeight float64
	weights := make([]float64, len(items))
	for i, item := range items {
		name := strings.ToLower(nameOf(item))
		count := counts[name]
		w := 1.0 / float64(1+count)
		weights[i] = w
		totalWeight += w
	}
	if totalWeight <= 0 {
		i := rand.Intn(len(items))
		return &items[i]
	}
	r := rand.Float64() * totalWeight
	for i, w := range weights {
		r -= w
		if r <= 0 {
			return &items[i]
		}
	}
	return &items[len(items)-1]
}
