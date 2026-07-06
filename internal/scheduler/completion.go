package scheduler

import (
	"context"
	"errors"
	"log"
	"sort"
	"strings"
	"time"

	"osrs-events/internal/database"
	"osrs-events/internal/discord"
	"osrs-events/internal/discord/services"
	"osrs-events/internal/osrs"
)

func (s *Scheduler) runCompletionCheck() {
	ticker := s.clock.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C():
			s.processEventCompletions()
			s.processPendingEventStarts()
		case <-s.stopCompletion:
			log.Println("Completion checker stopped")
			return
		}
	}
}

func (s *Scheduler) processPendingEventStarts() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	events, err := s.store.GetPendingStartEvents(ctx)
	if err != nil {
		log.Printf("Error getting pending start events: %v", err)
		return
	}

	for _, event := range events {
		// Check if snapshots already exist for this event
		snapshots, err := s.store.GetSnapshotsByEvent(ctx, event.ID)
		if err != nil {
			log.Printf("Error checking snapshots for event %d: %v", event.ID, err)
			continue
		}

		// If snapshots don't exist, create them now
		if len(snapshots) == 0 {
			log.Printf("Creating initial snapshots for event %d (%s) that just started", event.ID, event.Type)
			metricName := event.MetricJsonID

			if _, err := s.snapshotService.CreateInitialSnapshots(ctx, event.ID, event.GuildID, metricName, event.Type); err != nil {
				log.Printf("Failed to create initial snapshots for event %d: %v", event.ID, err)
				continue
			}

			// Update weekly leaderboard after creating snapshots
			if err := s.leaderboardService.UpdateWeeklyLeaderboard(ctx, event.GuildID, event.Type); err != nil {
				log.Printf("Failed to update weekly leaderboard after snapshot creation: %v", err)
			}
		}
	}
}

func (s *Scheduler) processEventCompletions() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// First, clean up any stale active events (safety mechanism)
	s.cleanupStaleActiveEvents(ctx)

	events, err := s.store.GetExpiredActiveEvents(ctx)
	if err != nil {
		log.Printf("Error getting expired active events: %v", err)
		return
	}

	s.processEventCompletionsForEvents(events)
}

// cleanupStaleActiveEvents is a safety mechanism to handle cases where multiple active events
// of the same type exist for a guild (should be prevented by unique index, but this handles legacy data)
func (s *Scheduler) cleanupStaleActiveEvents(ctx context.Context) {
	// Get all active events grouped by guild and type
	allActiveEvents, err := s.store.GetAllActiveEvents(ctx)
	if err != nil {
		log.Printf("Error getting active events for cleanup: %v", err)
		return
	}

	// Group by guild_id + type
	type eventKey struct {
		guildID   string
		eventType string
	}
	eventsByKey := make(map[eventKey][]*database.Event)
	for _, event := range allActiveEvents {
		key := eventKey{guildID: event.GuildID, eventType: event.Type}
		eventsByKey[key] = append(eventsByKey[key], event)
	}

	// Check for duplicates and deactivate older ones
	for key, events := range eventsByKey {
		if len(events) <= 1 {
			continue
		}

		log.Printf("WARNING: Found %d active %s events for guild %s - cleaning up", len(events), key.eventType, key.guildID)

		// Sort by start time descending (newest first)
		sort.Slice(events, func(i, j int) bool {
			return events[i].StartTime.After(events[j].StartTime)
		})

		// Keep the newest, properly complete all others (calculate points, then deactivate)
		// Do NOT start new events - if duplicates exist, a newer event already exists
		for i := 1; i < len(events); i++ {
			oldEvent := events[i]
			log.Printf("  Completing stale active event ID %d (week %d, started %s)",
				oldEvent.ID, oldEvent.WeekNumber, oldEvent.StartTime.Format("2006-01-02 15:04"))

			// Calculate and award points (do NOT start a new event)
			if err := s.snapshotService.CalculateAndAwardPoints(ctx, oldEvent); err != nil {
				log.Printf("  Failed to calculate points for stale event %d: %v", oldEvent.ID, err)
				continue
			}

			// Update overall leaderboard
			if err := s.leaderboardService.UpdateOverallLeaderboard(ctx, oldEvent.GuildID, oldEvent.Type); err != nil {
				log.Printf("  WARNING: Failed to update overall leaderboard: %v", err)
			}

			// Deactivate the event
			if err := s.store.DeactivateEvent(ctx, oldEvent.ID); err != nil {
				log.Printf("  Failed to deactivate stale event %d: %v", oldEvent.ID, err)
			} else {
				log.Printf("  Successfully completed stale event %d (points awarded, deactivated)", oldEvent.ID)
			}
		}
	}
}

func (s *Scheduler) processEventCompletionsForEvents(events []*database.Event) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	var snapshotResult *services.UpdateSnapshotsForEventsResult
	if len(events) > 0 {
		var err error
		snapshotResult, err = s.snapshotService.UpdateSnapshotsForEventsWithResult(ctx, events)
		if err != nil {
			log.Printf("Failed to batch update final snapshots before completion: %v - skipping rollover", err)
			return
		}
	}

	fetchedStats := make(map[int64]*osrs.PlayerStats)
	if snapshotResult != nil && snapshotResult.FetchedStats != nil {
		fetchedStats = snapshotResult.FetchedStats
	}

	if snapshotResult != nil {
		s.processMissingAccountNotifications(ctx, time.Now().UTC(), snapshotResult)
	}

	decisions := classifyFailuresByGuild(snapshotResult)

	for guildID, guildEvents := range groupEventsByGuild(events) {
		switch decisions[guildID] {
		case guildSkipTransient:
			log.Printf("[Guild %s] Deferring rollover: transient snapshot failures (429/5xx)", guildID)
			continue
		case guildSkipAll404:
			log.Printf("[Guild %s] Deferring rollover: too few successful hiscores fetches (likely API issue)", guildID)
			continue
		}
		sort.Slice(guildEvents, func(i, j int) bool {
			return guildEvents[i].Type < guildEvents[j].Type
		})
		s.rolloverGuild(ctx, guildID, guildEvents, fetchedStats)
	}
}

func (s *Scheduler) rolloverGuild(ctx context.Context, guildID string, events []*database.Event, fetchedStats map[int64]*osrs.PlayerStats) {
	eventTypes := make([]string, len(events))
	for i, e := range events {
		eventTypes[i] = e.Type
	}
	log.Printf("[Guild %s] Starting rollover (%d events: %s)", guildID, len(events), strings.Join(eventTypes, ", "))

	guild, err := s.store.GetGuild(ctx, guildID)
	if err != nil {
		log.Printf("[Guild %s] Failed to load guild for rollover: %v", guildID, err)
	}

	prepared := make([]*services.PreparedRolloverEvent, len(events))
	for i, event := range events {
		p, err := s.eventService.PrepareRolloverEvent(ctx, event.GuildID, event.Type, event.EndTime)
		if err != nil {
			log.Printf("[Guild %s] CRITICAL: rollover preparation failed for %s: %v — deferring entire guild", guildID, event.Type, err)
			return
		}
		prepared[i] = p
	}

	if err := s.snapshotService.ValidateRolloverCommitReadiness(ctx, events, guildID); err != nil {
		log.Printf("[Guild %s] CRITICAL: rollover commit readiness check failed: %v — deferring entire guild", guildID, err)
		return
	}

	var completedEvents, newEvents []discord.RolloverResult
	for i, event := range events {
		if err := s.snapshotService.CalculateAndAwardPoints(ctx, event); err != nil {
			log.Printf("[Guild %s] CRITICAL: failed to award points for event %d (%s): %v — aborting rollover", guildID, event.ID, event.Type, err)
			return
		}

		if err := s.store.DeactivateEvent(ctx, event.ID); err != nil {
			log.Printf("[Guild %s] CRITICAL: failed to deactivate event %d (%s): %v — aborting rollover", guildID, event.ID, event.Type, err)
			return
		}

		if err := s.eventService.CommitRolloverEvent(ctx, prepared[i], fetchedStats); err != nil {
			log.Printf("[Guild %s] CRITICAL: failed to commit new %s event: %v — aborting rollover", guildID, event.Type, err)
			return
		}

		completedEvents = append(completedEvents, discord.RolloverResult{
			EventType:  event.Type,
			MetricName: event.MetricJsonID,
			WeekNumber: event.WeekNumber,
		})
		newEvents = append(newEvents, discord.RolloverResult{
			EventType:  event.Type,
			MetricName: prepared[i].MetricName,
			WeekNumber: prepared[i].Event.WeekNumber,
		})

		if err := s.leaderboardService.UpdateWeeklyLeaderboard(ctx, event.GuildID, event.Type); err != nil {
			log.Printf("[Guild %s] Failed to update weekly leaderboard for %s: %v", guildID, event.Type, err)
		}
		if err := s.leaderboardService.UpdateOverallLeaderboard(ctx, event.GuildID, event.Type); err != nil {
			log.Printf("[Guild %s] Failed to update overall leaderboard for %s: %v", guildID, event.Type, err)
		}
		if guild != nil {
			if err := s.initializerService.RenameCategoryForEvent(ctx, guild, event.Type, prepared[i].Event); err != nil {
				log.Printf("[Guild %s] Failed to rename category for %s: %v", guildID, event.Type, err)
			}
		}
	}

	if len(completedEvents) != len(events) {
		log.Printf("[Guild %s] CRITICAL: rollover finished with %d/%d events committed — skipping Rollover Log", guildID, len(completedEvents), len(events))
		return
	}

	log.Printf("[Guild %s] Rollover commit complete (%d events)", guildID, len(completedEvents))

	if guild == nil || guild.LogChannelID == "" {
		log.Printf("[Guild %s] Rollover complete but no log channel configured — skipping Rollover Log", guildID)
		return
	}

	unresolvedRSNs := s.unresolvedMissingAccountRSNs(ctx, guildID)
	discord.SendRolloverCompleteLog(s.session, guild.LogChannelID, completedEvents, newEvents, unresolvedRSNs)
	if len(unresolvedRSNs) > 0 {
		log.Printf("[Guild %s] Rollover Log sent (%d unresolved missing account(s))", guildID, len(unresolvedRSNs))
	} else {
		log.Printf("[Guild %s] Rollover Log sent", guildID)
	}
}

func (s *Scheduler) unresolvedMissingAccountRSNs(ctx context.Context, guildID string) []string {
	unresolved, err := s.store.GetUnresolvedMissingAccountNotificationsByGuild(ctx, guildID)
	if err != nil {
		log.Printf("Failed to load unresolved missing account notifications for guild %s: %v", guildID, err)
		return nil
	}
	rsns := make([]string, 0, len(unresolved))
	for _, notification := range unresolved {
		rsns = append(rsns, notification.RSN)
	}
	return rsns
}

type guildRolloverDecision int

const (
	guildProceed      guildRolloverDecision = iota
	guildSkipTransient
	guildSkipAll404
)

// minSuccessFraction is the minimum share of accounts that must fetch successfully
// before rollover proceeds when some accounts returned 404. Below this threshold,
// a handful of successes amid mass 404s is treated as a flaky API, not real renames.
const minSuccessFraction = 0.5

// classifyFailuresByGuild assigns a rollover decision to each guild based on its snapshot failures.
// Guilds with transient errors (429/5xx/other non-404) are deferred for the entire tick.
// Guilds where too few accounts fetched successfully (including all-404) are deferred.
// Guilds with no failures, or 404s among a majority of successes, proceed to rollover.
func classifyFailuresByGuild(result *services.UpdateSnapshotsForEventsResult) map[string]guildRolloverDecision {
	decisions := make(map[string]guildRolloverDecision)
	if result == nil || len(result.FailedUpdates) == 0 {
		return decisions
	}

	successCount := countUniqueAccountsByGuild(result.SuccessfulPairs)
	notFoundCount := make(map[string]int)
	seenNotFound := make(map[string]map[int64]struct{})

	for _, failed := range result.FailedUpdates {
		if decisions[failed.GuildID] == guildSkipTransient {
			continue
		}
		var notFoundErr *osrs.PlayerNotFoundError
		if !errors.As(failed.Error, &notFoundErr) {
			decisions[failed.GuildID] = guildSkipTransient
			continue
		}
		if failed.AccountID == 0 {
			continue
		}
		if seenNotFound[failed.GuildID] == nil {
			seenNotFound[failed.GuildID] = make(map[int64]struct{})
		}
		if _, ok := seenNotFound[failed.GuildID][failed.AccountID]; ok {
			continue
		}
		seenNotFound[failed.GuildID][failed.AccountID] = struct{}{}
		notFoundCount[failed.GuildID]++
	}

	for guildID, nf := range notFoundCount {
		if decisions[guildID] == guildSkipTransient {
			continue
		}
		sc := successCount[guildID]
		total := sc + nf
		if total == 0 || float64(sc)/float64(total) <= minSuccessFraction {
			decisions[guildID] = guildSkipAll404
		}
	}

	return decisions
}

func countUniqueAccountsByGuild(pairs []services.SuccessfulAccountUpdate) map[string]int {
	counts := make(map[string]int)
	seen := make(map[string]map[int64]struct{})
	for _, p := range pairs {
		if p.AccountID == 0 {
			continue
		}
		if seen[p.GuildID] == nil {
			seen[p.GuildID] = make(map[int64]struct{})
		}
		if _, ok := seen[p.GuildID][p.AccountID]; ok {
			continue
		}
		seen[p.GuildID][p.AccountID] = struct{}{}
		counts[p.GuildID]++
	}
	return counts
}

func groupEventsByGuild(events []*database.Event) map[string][]*database.Event {
	byGuild := make(map[string][]*database.Event, len(events))
	for _, event := range events {
		byGuild[event.GuildID] = append(byGuild[event.GuildID], event)
	}
	return byGuild
}

