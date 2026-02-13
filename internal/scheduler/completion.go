package scheduler

import (
	"context"
	"log"
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

	events, err := s.store.GetExpiringEvents(ctx)
	if err != nil {
		log.Printf("Error getting expiring events: %v", err)
		return
	}

	s.processEventCompletionsForEvents(events)
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

		// Start new event BEFORE completing the old one
		// This ensures if rollover fails, the old event stays active and will be retried
		rolloverResult, err := s.eventService.AutoRollover(ctx, event.GuildID, event.Type, event.EndTime)
		if err != nil {
			log.Printf("CRITICAL: Failed to auto-rollover %s event %d (Guild: %s): %v - will retry on next check", event.Type, event.ID, event.GuildID, err)
			continue
		}

		// Complete event only after rollover succeeds (snapshots already updated, just calculate points and deactivate)
		if err := s.eventService.CompleteEventWithoutSnapshotUpdate(ctx, event); err != nil {
			log.Printf("Failed to complete event %d after successful rollover: %v - old event remains active", event.ID, err)
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
