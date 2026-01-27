package scheduler

import (
	"context"
	"errors"
	"log"
	"time"

	"osrs-events/internal/database"
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

	if len(events) == 0 {
		return
	}

	log.Printf("Updating snapshots for %d active events", len(events))

	// Group events by guild to handle logging per guild
	guildsMap := make(map[string]*database.Guild)
	for _, event := range events {
		if guildsMap[event.GuildID] == nil {
			guild, err := s.store.GetGuild(ctx, event.GuildID)
			if err != nil {
				log.Printf("Failed to get guild for event %d: %v", event.ID, err)
				continue
			}
			guildsMap[event.GuildID] = guild
		}
	}

	// Update all snapshots efficiently (fetch stats once per account, update all events)
	failedUpdates, err := s.snapshotService.UpdateSnapshotsForEvents(ctx, events)
	if err != nil {
		log.Printf("Failed to update snapshots: %v", err)
		return
	}

	// Log 404 errors to log channel per guild
	for guildID, guild := range guildsMap {
		if guild.LogChannelID != "" {
			for _, failed := range failedUpdates {
				var notFoundErr *osrs.PlayerNotFoundError
				if errors.As(failed.Error, &notFoundErr) {
					discord.SendAccountNotFoundLog(s.session, guild.LogChannelID, failed.RSN)
				}
			}
		}

		// Update weekly leaderboards for this guild
		if err := s.leaderboardService.UpdateWeeklyLeaderboard(ctx, guildID, "botw"); err != nil {
			log.Printf("Failed to update BOTW weekly leaderboard for guild %s: %v", guildID, err)
		}
		if err := s.leaderboardService.UpdateWeeklyLeaderboard(ctx, guildID, "sotw"); err != nil {
			log.Printf("Failed to update SOTW weekly leaderboard for guild %s: %v", guildID, err)
		}
	}

	log.Printf("Updated snapshots for %d events across %d guilds", len(events), len(guildsMap))
	log.Println("Hourly snapshot update completed")
}
