package scheduler

import (
	"context"
	"errors"
	"log"
	"time"

	"osrs-events/internal/database"
	"osrs-events/internal/discord"
	"osrs-events/internal/discord/services"
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

	guildsMap := make(map[string]*database.Guild)
	var eventsToProcess []*database.Event
	for _, event := range events {
		if guildsMap[event.GuildID] == nil {
			guild, err := s.store.GetGuild(ctx, event.GuildID)
			if err != nil {
				log.Printf("Guild %s not found for event %d — skipping event", event.GuildID, event.ID)
				continue
			}
			guildsMap[event.GuildID] = guild
		}
		eventsToProcess = append(eventsToProcess, event)
	}
	if len(eventsToProcess) == 0 {
		log.Printf("Hourly snapshot update: no events with valid guild rows (skipped %d orphan event(s))", len(events))
		return
	}
	events = eventsToProcess

	// Update all snapshots efficiently (fetch stats once per account, update all events)
	result, err := s.snapshotService.UpdateSnapshotsForEventsWithResult(ctx, events)
	if err != nil {
		log.Printf("Failed to update snapshots: %v", err)
		return
	}

	// One global summary (TotalAccounts is unique accounts across all guilds, not per-guild)
	log.Printf("Hourly snapshot update: %d unique accounts, %d events across %d guilds, completed in %s",
		result.TotalAccounts, len(events), len(guildsMap), result.Duration.Round(time.Millisecond))

	// Per-guild only when there are failures
	for guildID := range guildsMap {
		var failedRSNs []string
		for _, failed := range result.FailedUpdates {
			if failed.GuildID == guildID {
				failedRSNs = append(failedRSNs, failed.RSN)
			}
		}
		if len(failedRSNs) > 0 {
			log.Printf("[Guild %s] %d accounts failed: %v", guildID, len(failedRSNs), failedRSNs)
		}
	}

	now := time.Now().UTC()
	s.processMissingAccountNotifications(ctx, now, result)

	for guildID := range guildsMap {
		s.leaderboardService.UpdateWeeklyLeaderboard(ctx, guildID, "botw")
		s.leaderboardService.UpdateWeeklyLeaderboard(ctx, guildID, "sotw")
	}
}

func (s *Scheduler) processMissingAccountNotifications(ctx context.Context, now time.Time, result *services.UpdateSnapshotsForEventsResult) {
	s.missingAccountMu.Lock()
	defer s.missingAccountMu.Unlock()
	for _, failed := range result.FailedUpdates {
		var notFoundErr *osrs.PlayerNotFoundError
		if !errors.As(failed.Error, &notFoundErr) {
			continue
		}
		if failed.AccountID == 0 || failed.GuildID == "" {
			continue
		}

		notification := &database.MissingAccountNotification{
			AccountID:     failed.AccountID,
			DiscordUserID: failed.DiscordUserID,
			GuildID:       failed.GuildID,
			RSN:           failed.RSN,
			FirstFailedAt: now,
			LastFailedAt:  now,
		}
		if err := s.store.UpsertMissingAccountNotificationFailure(ctx, notification); err != nil {
			log.Printf("Failed to upsert missing account notification for account %d guild %s: %v", failed.AccountID, failed.GuildID, err)
		}
	}

	pendingDMs, err := s.store.GetPendingMissingAccountNotifications(ctx)
	if err != nil {
		log.Printf("Failed to load pending missing account DMs: %v", err)
	} else {
		for _, pending := range pendingDMs {
			if err := discord.SendAccountNotFoundDM(s.session, pending.DiscordUserID, pending.GuildID, pending.RSN); err != nil {
				log.Printf("Failed to send missing account DM for account %d (%s): %v", pending.AccountID, pending.RSN, err)
				continue
			}
			if err := s.store.MarkMissingAccountNotificationDMSent(ctx, pending.ID, now); err != nil {
				log.Printf("Failed to mark missing account DM as sent for notification %d: %v", pending.ID, err)
			}
		}
	}

	for _, success := range result.SuccessfulPairs {
		if success.AccountID == 0 || success.GuildID == "" {
			continue
		}
		if err := s.store.ResolveMissingAccountNotification(ctx, success.AccountID, success.GuildID, now); err != nil {
			log.Printf("Failed to resolve missing account notification for account %d guild %s: %v", success.AccountID, success.GuildID, err)
		}
	}
}
