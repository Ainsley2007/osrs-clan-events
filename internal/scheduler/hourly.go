package scheduler

import (
	"context"
	"errors"
	"log"
	"time"

	"osrs-events/internal/discord"
	"osrs-events/internal/osrs"
)

func (s *Scheduler) runHourlyUpdates() {
	ticker := s.clock.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C():
			s.updateActiveSnapshots()
		case <-s.stopHourly:
			log.Println("Hourly updater stopped")
			return
		}
	}
}

func (s *Scheduler) updateActiveSnapshots() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	events, err := s.store.GetAllActiveEvents(ctx)
	if err != nil {
		log.Printf("Error getting active events: %v", err)
		return
	}

	log.Printf("Updating snapshots for %d active events", len(events))

	for _, event := range events {
		guild, err := s.store.GetGuild(ctx, event.GuildID)
		if err != nil {
			log.Printf("Failed to get guild for event %d: %v", event.ID, err)
			continue
		}

		failedUpdates, err := s.snapshotService.UpdateSnapshotsForEvent(ctx, event)
		if err != nil {
			log.Printf("Failed to update snapshots for event %d: %v", event.ID, err)
			continue
		}

		// Log 404 errors to log channel
		if guild.LogChannelID != "" && len(failedUpdates) > 0 {
			for _, failed := range failedUpdates {
				var notFoundErr *osrs.PlayerNotFoundError
				if errors.As(failed.Error, &notFoundErr) {
					discord.SendAccountNotFoundLog(s.session, guild.LogChannelID, failed.RSN)
				}
			}
		}

		// Update weekly leaderboard after snapshot update
		if err := s.leaderboardService.UpdateWeeklyLeaderboard(ctx, event.GuildID, event.Type); err != nil {
			log.Printf("Failed to update weekly leaderboard for event %d: %v", event.ID, err)
		}

		log.Printf("Updated snapshots and leaderboard for event %d (%s)", event.ID, event.Type)
	}

	log.Println("Hourly snapshot update completed")
}
