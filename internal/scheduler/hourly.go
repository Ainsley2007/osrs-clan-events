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
	result, err := s.snapshotService.UpdateSnapshotsForEventsWithResult(ctx, events)
	if err != nil {
		log.Printf("Failed to update snapshots: %v", err)
		return
	}

	// Log snapshot update summary locally (grouped by guild)
	for guildID := range guildsMap {
		var failedRSNs []string
		for _, failed := range result.FailedUpdates {
			if failed.GuildID == guildID {
				failedRSNs = append(failedRSNs, failed.RSN)
			}
		}
		if len(failedRSNs) > 0 {
			log.Printf("[Guild %s] Hourly snapshot update: %d accounts processed in %s, %d failed: %v", guildID, result.TotalAccounts, result.Duration.Round(time.Millisecond), len(failedRSNs), failedRSNs)
		} else {
			log.Printf("[Guild %s] Hourly snapshot update: %d accounts processed in %s", guildID, result.TotalAccounts, result.Duration.Round(time.Millisecond))
		}
	}

	// Log 404 errors to the log channel of the guild the account belongs to only
	for _, failed := range result.FailedUpdates {
		var notFoundErr *osrs.PlayerNotFoundError
		if !errors.As(failed.Error, &notFoundErr) {
			continue
		}
		guild := guildsMap[failed.GuildID]
		if guild == nil || guild.LogChannelID == "" {
			continue
		}
		discord.SendAccountNotFoundLog(s.session, guild.LogChannelID, failed.RSN)
	}

	for guildID := range guildsMap {
		s.leaderboardService.UpdateWeeklyLeaderboard(ctx, guildID, "botw")
		s.leaderboardService.UpdateWeeklyLeaderboard(ctx, guildID, "sotw")
	}

	log.Printf("Hourly snapshot update completed: %d events across %d guilds", len(events), len(guildsMap))
}
