package scheduler

import (
	"context"
	"log"
	"sort"
	"time"

	"osrs-events/internal/database"
	"osrs-events/internal/discord"
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

	events, err := s.store.GetExpiringEvents(ctx)
	if err != nil {
		log.Printf("Error getting expiring events: %v", err)
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

		log.Printf("⚠️  WARNING: Found %d active %s events for guild %s - cleaning up", len(events), key.eventType, key.guildID)

		// Sort by start time descending (newest first)
		sort.Slice(events, func(i, j int) bool {
			return events[i].StartTime.After(events[j].StartTime)
		})

		// Keep the newest, properly complete all others (calculate points, then deactivate)
		// Do NOT start new events - if duplicates exist, a newer event already exists
		for i := 1; i < len(events); i++ {
			oldEvent := events[i]
			log.Printf("  ⚠️  Completing stale active event ID %d (week %d, started %s)", 
				oldEvent.ID, oldEvent.WeekNumber, oldEvent.StartTime.Format("2006-01-02 15:04"))
			
			// Calculate and award points (do NOT start a new event)
			if err := s.snapshotService.CalculateAndAwardPoints(ctx, oldEvent); err != nil {
				log.Printf("  ❌ Failed to calculate points for stale event %d: %v", oldEvent.ID, err)
				continue
			}
			
			// Update overall leaderboard
			if err := s.leaderboardService.UpdateOverallLeaderboard(ctx, oldEvent.GuildID, oldEvent.Type); err != nil {
				log.Printf("  ⚠️  Failed to update overall leaderboard: %v", err)
			}
			
			// Deactivate the event
			if err := s.store.DeactivateEvent(ctx, oldEvent.ID); err != nil {
				log.Printf("  ❌ Failed to deactivate stale event %d: %v", oldEvent.ID, err)
			} else {
				log.Printf("  ✅ Successfully completed stale event %d (points awarded, deactivated)", oldEvent.ID)
			}
		}
	}
}

func (s *Scheduler) processEventCompletionsForEvents(events []*database.Event) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Group events by guild to send consolidated log messages
	guildRollovers := make(map[string]struct {
		guild           *database.Guild
		completedEvents []discord.RolloverResult
		newEvents       []discord.RolloverResult
	})

	// Take final snapshot for all events that have ended (fetch stats once per account)
	// Events are processed when end_time <= now, ensuring each week is exactly 7 days ± a few seconds
	if len(events) > 0 {
		if _, err := s.snapshotService.UpdateSnapshotsForEvents(ctx, events); err != nil {
			log.Printf("Failed to batch update final snapshots before completion: %v", err)
		}
	}

	for _, event := range events {
		log.Printf("Processing event completion: %s (ID: %d, Guild: %s)", event.Type, event.ID, event.GuildID)

		guild, err := s.store.GetGuild(ctx, event.GuildID)
		if err == nil {
			// Update weekly leaderboard BEFORE completing event (while it's still active)
			if err := s.leaderboardService.UpdateWeeklyLeaderboard(ctx, event.GuildID, event.Type); err != nil {
				log.Printf("Failed to update weekly leaderboard before completion: %v", err)
			}
		}

		// STEP 1: Fully complete the old event (points + overall leaderboard + deactivate)
		// This ensures all points are calculated BEFORE the new event is created
		if err := s.snapshotService.CalculateAndAwardPoints(ctx, event); err != nil {
			log.Printf("CRITICAL: Failed to calculate points for event %d: %v - skipping rollover", event.ID, err)
			continue // Don't create new event if we can't complete old one
		}

		// Update overall leaderboard with completed event points
		if err := s.leaderboardService.UpdateOverallLeaderboard(ctx, event.GuildID, event.Type); err != nil {
			log.Printf("Failed to update overall leaderboard for completed event: %v", err)
			// Continue anyway - points are calculated
		}

		// Deactivate the old event
		if err := s.store.DeactivateEvent(ctx, event.ID); err != nil {
			log.Printf("CRITICAL: Failed to deactivate event %d: %v - skipping rollover", event.ID, err)
			continue // Don't create new event if we can't deactivate old one
		}

		log.Printf("✅ Event %d fully completed (points awarded, leaderboard updated, deactivated)", event.ID)

		// STEP 2: Now create the new event (old one is fully completed and deactivated)
		rolloverResult, err := s.eventService.StartNewEvent(ctx, event.GuildID, event.Type, event.EndTime)
		if err != nil {
			log.Printf("CRITICAL: Failed to start new %s event for Guild %s: %v - will retry on next check", event.Type, event.GuildID, err)
			continue
		}

		// Track completed event for consolidated log
		if guild != nil {
			rollover, exists := guildRollovers[event.GuildID]
			if !exists {
				rollover = struct {
					guild           *database.Guild
					completedEvents []discord.RolloverResult
					newEvents       []discord.RolloverResult
				}{
					guild:           guild,
					completedEvents: []discord.RolloverResult{},
					newEvents:       []discord.RolloverResult{},
				}
			}
			rollover.completedEvents = append(rollover.completedEvents, discord.RolloverResult{
				EventType:  event.Type,
				MetricName: event.MetricJsonID,
				WeekNumber: event.WeekNumber,
			})
			guildRollovers[event.GuildID] = rollover

			// Update overall leaderboard after completion
			if err := s.leaderboardService.UpdateOverallLeaderboard(ctx, event.GuildID, event.Type); err != nil {
				log.Printf("Failed to update overall leaderboard for %s: %v", event.Type, err)
			}
		}

		// Rename category with new event name
		if guild != nil {
			if err := s.initializerService.RenameCategoryForEvent(ctx, guild, event.Type, rolloverResult.Event); err != nil {
				log.Printf("Failed to rename category for %s: %v", event.Type, err)
			}
		}

		if err := s.leaderboardService.UpdateWeeklyLeaderboard(ctx, event.GuildID, event.Type); err != nil {
			log.Printf("Failed to update weekly leaderboard after rollover: %v", err)
		}

		// Update overall leaderboard for new event
		if err := s.leaderboardService.UpdateOverallLeaderboard(ctx, event.GuildID, event.Type); err != nil {
			log.Printf("Failed to update overall leaderboard after rollover: %v", err)
		}

		// Track new event for consolidated log
		if guild != nil {
			rollover := guildRollovers[event.GuildID]
			rollover.newEvents = append(rollover.newEvents, discord.RolloverResult{
				EventType:  event.Type,
				MetricName: rolloverResult.MetricName,
				WeekNumber: rolloverResult.Event.WeekNumber,
			})
			guildRollovers[event.GuildID] = rollover
		}

		log.Printf("Event %d completed and rolled over successfully", event.ID)
	}

	// Send consolidated log messages per guild
	for _, rollover := range guildRollovers {
		if rollover.guild != nil && rollover.guild.LogChannelID != "" && (len(rollover.completedEvents) > 0 || len(rollover.newEvents) > 0) {
			discord.SendRolloverCompleteLog(s.session, rollover.guild.LogChannelID, rollover.completedEvents, rollover.newEvents)
		}
	}
}
