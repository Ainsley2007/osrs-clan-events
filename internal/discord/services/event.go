package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"slices"
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
	logger          Logger
}

func NewEventService(store EventStore, snapshotService SnapshotManager, configProvider OSRSConfigProvider, logger Logger) *EventService {
	return &EventService{
		store:           store,
		snapshotService: snapshotService,
		configProvider:  configProvider,
		logger:          logger,
	}
}

const recentEventsWeightWindow = 52 // 52 weeks in a year

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
	weights, totalWeight := metricPickWeights(config.Bosses, recentBosses, func(b firebase.BossConfig) string { return b.Name })
	bossConfig, roll := weightedPickFromWeights(config.Bosses, weights, totalWeight)
	bossesToTrackJSON, err := json.Marshal(bossConfig.BossesToTrack)
	if err != nil {
		return nil, "", fmt.Errorf("failed to marshal bosses to track: %w", err)
	}
	weekNumber, err := s.GetNextWeekNumber(ctx, guildID, "botw")
	if err != nil {
		return nil, "", fmt.Errorf("failed to get week number: %w", err)
	}
	s.logMetricSelection(guildID, "botw", bossConfig.Name, weekNumber, len(config.Bosses), weights, totalWeight, recentBosses, roll)
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
	weights, totalWeight := metricPickWeights(config.Skills, recentSkills, func(sk firebase.SkillConfig) string { return sk.Name })
	skillConfig, roll := weightedPickFromWeights(config.Skills, weights, totalWeight)
	weekNumber, err := s.GetNextWeekNumber(ctx, guildID, "sotw")
	if err != nil {
		return nil, "", fmt.Errorf("failed to get week number: %w", err)
	}
	s.logMetricSelection(guildID, "sotw", skillConfig.Name, weekNumber, len(config.Skills), weights, totalWeight, recentSkills, roll)
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

func (s *EventService) GetActiveEvents(ctx context.Context, guildID, eventType string) ([]*database.Event, error) {
	return s.store.GetActiveEvents(ctx, guildID, eventType)
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

// AbortStartedEvent deactivates a freshly created event when a paired start fails.
// It does not award points or update snapshots.
func (s *EventService) AbortStartedEvent(ctx context.Context, event *database.Event) error {
	if event == nil {
		return nil
	}
	return s.store.DeactivateEvent(ctx, event.ID)
}

// StartNewEvent creates a new event after the old one has been completed (e.g. manual /start).
// Uses the API to create initial snapshots.
func (s *EventService) StartNewEvent(ctx context.Context, guildID string, eventType string, startTime time.Time) (*StartEventResult, error) {
	if eventType == "botw" {
		return s.StartBotw(ctx, guildID, startTime)
	}
	return s.StartSotw(ctx, guildID, startTime)
}

// PreparedRolloverEvent holds a validated but not-yet-persisted next-week event.
// Produced by PrepareRolloverEvent and consumed by CommitRolloverEvent.
type PreparedRolloverEvent struct {
	Event      *database.Event
	MetricName string
}

// PrepareRolloverEvent selects the next metric and builds the new event without persisting anything.
// It may be called while the old event is still active in the DB (rollover knows it is replacing it).
// Any config-fetch or metric-selection failure is surfaced here before any mutations occur.
func (s *EventService) PrepareRolloverEvent(ctx context.Context, guildID, eventType string, startTime time.Time) (*PreparedRolloverEvent, error) {
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
	return &PreparedRolloverEvent{Event: event, MetricName: metricName}, nil
}

// CommitRolloverEvent persists a PreparedRolloverEvent and seeds its initial snapshots from
// pre-fetched stats so no extra hiscores API calls are needed.
func (s *EventService) CommitRolloverEvent(ctx context.Context, prepared *PreparedRolloverEvent, statsByAccountID map[int64]*osrs.PlayerStats) error {
	if err := s.CreateEvent(ctx, prepared.Event); err != nil {
		return fmt.Errorf("failed to create event: %w", err)
	}
	if err := s.snapshotService.CreateInitialSnapshotsForEventsFromStats(ctx, []*database.Event{prepared.Event}, statsByAccountID); err != nil {
		return fmt.Errorf("failed to create initial snapshots from cache: %w", err)
	}
	return nil
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

type metricPickWeight struct {
	Name   string
	Count  int
	Weight float64
}

type metricPickRoll struct {
	Value      float64 // random value in [0, totalWeight); -1 when uniform fallback was used
	UniformIdx int     // index from rand.Intn when uniform fallback was used; -1 otherwise
	PickedIdx  int     // index into the candidate slice; -1 when not set
}

func weightedPickBoss(bosses []firebase.BossConfig, recentNames []string) (*firebase.BossConfig, metricPickRoll) {
	weights, totalWeight := metricPickWeights(bosses, recentNames, func(b firebase.BossConfig) string { return b.Name })
	return weightedPickFromWeights(bosses, weights, totalWeight)
}

func weightedPickSkill(skills []firebase.SkillConfig, recentNames []string) (*firebase.SkillConfig, metricPickRoll) {
	weights, totalWeight := metricPickWeights(skills, recentNames, func(s firebase.SkillConfig) string { return s.Name })
	return weightedPickFromWeights(skills, weights, totalWeight)
}

func metricPickWeights[T any](items []T, recentNames []string, nameOf func(T) string) ([]metricPickWeight, float64) {
	counts := countOccurrences(recentNames)
	weights := make([]metricPickWeight, len(items))
	var totalWeight float64
	for i, item := range items {
		name := nameOf(item)
		count := counts[strings.ToLower(name)]
		w := 1.0 / float64(1+count)
		weights[i] = metricPickWeight{Name: name, Count: count, Weight: w}
		totalWeight += w
	}
	return weights, totalWeight
}

func weightedPickFromWeights[T any](items []T, weights []metricPickWeight, totalWeight float64) (*T, metricPickRoll) {
	if len(items) == 0 {
		return nil, metricPickRoll{}
	}
	if totalWeight <= 0 {
		i := rand.Intn(len(items))
		return &items[i], metricPickRoll{UniformIdx: i, PickedIdx: i}
	}
	roll := rand.Float64() * totalWeight
	r := roll
	for i, w := range weights {
		r -= w.Weight
		if r <= 0 {
			return &items[i], metricPickRoll{Value: roll, UniformIdx: -1, PickedIdx: i}
		}
	}
	last := len(items) - 1
	return &items[last], metricPickRoll{Value: roll, UniformIdx: -1, PickedIdx: last}
}

func (s *EventService) logMetricSelection(guildID, eventType, selected string, weekNumber, candidateCount int, weights []metricPickWeight, totalWeight float64, recentNames []string, roll metricPickRoll) {
	if s.logger == nil {
		return
	}
	s.logger.Printf("%s", formatMetricSelectionLog(guildID, eventType, selected, weekNumber, candidateCount, weights, totalWeight, recentNames, roll))
}

func formatMetricSelectionLog(guildID, eventType, selected string, weekNumber, candidateCount int, weights []metricPickWeight, totalWeight float64, recentNames []string, roll metricPickRoll) string {
	label := strings.ToUpper(eventType)
	var b strings.Builder

	fmt.Fprintf(&b, "[Guild %s] %s selection\n", guildID, label)
	fmt.Fprintf(&b, "  pool:    %d candidates | %d events in %d-week window\n",
		candidateCount, len(recentNames), recentEventsWeightWindow)

	counts := countOccurrences(recentNames)
	if len(counts) > 0 {
		b.WriteString("  recent:\n")
		for _, line := range formatSortedOccurrenceLines(counts) {
			fmt.Fprintf(&b, "    %s\n", line)
		}
	}

	fmt.Fprintf(&b, "  weights (total %.6f):\n", totalWeight)
	var low float64
	pickedIdx := roll.PickedIdx
	if roll.UniformIdx >= 0 {
		pickedIdx = roll.UniformIdx
	}
	for i, w := range weights {
		high := low + w.Weight
		marker := ""
		if i == pickedIdx {
			marker = "  <- picked"
		}
		fmt.Fprintf(&b, "    [%2d] %-24s recent=%d  weight=%8.6f  range=[%8.6f, %8.6f)%s\n",
			i, w.Name, w.Count, w.Weight, low, high, marker)
		low = high
	}

	fmt.Fprintf(&b, "  roll: %s (week %d)", formatMetricRoll(roll, totalWeight, weights, selected), weekNumber)
	return b.String()
}

func formatSortedOccurrenceLines(counts map[string]int) []string {
	names := make([]string, 0, len(counts))
	for name := range counts {
		names = append(names, name)
	}
	slices.Sort(names)
	lines := make([]string, len(names))
	for i, name := range names {
		lines[i] = fmt.Sprintf("%s ×%d", name, counts[name])
	}
	return lines
}

func formatMetricRoll(roll metricPickRoll, totalWeight float64, weights []metricPickWeight, selected string) string {
	if roll.UniformIdx >= 0 {
		return fmt.Sprintf("uniform index %d (zero total weight) -> %q", roll.UniformIdx, selected)
	}
	var low float64
	for i, w := range weights {
		high := low + w.Weight
		if i == roll.PickedIdx {
			return fmt.Sprintf("%.6f in [%.6f, %.6f) -> %q", roll.Value, low, high, selected)
		}
		low = high
	}
	return fmt.Sprintf("%.6f of %.6f -> %q", roll.Value, totalWeight, selected)
}

func formatOccurrenceCounts(counts map[string]int) string {
	if len(counts) == 0 {
		return "none"
	}
	return strings.Join(formatSortedOccurrenceLines(counts), ", ")
}
