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

	"github.com/bwmarrin/discordgo"
)

type Scheduler struct {
	store              database.Store
	eventService       *services.EventService
	snapshotService    *services.SnapshotService
	leaderboardService *services.LeaderboardService
	session            *discordgo.Session
	stopCompletion     chan struct{}
	stopHourly         chan struct{}
}

func New(store database.Store, eventService *services.EventService, snapshotService *services.SnapshotService, leaderboardService *services.LeaderboardService, session *discordgo.Session) *Scheduler {
	return &Scheduler{
		store:              store,
		eventService:       eventService,
		snapshotService:    snapshotService,
		leaderboardService: leaderboardService,
		session:            session,
		stopCompletion:     make(chan struct{}),
		stopHourly:         make(chan struct{}),
	}
}

func (s *Scheduler) Start() {
	log.Println("Starting scheduler...")

	// Process stale events synchronously on startup (events that ended while bot was offline)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	staleEvents, err := s.store.GetStaleEvents(ctx)
	cancel()
	if err != nil {
		log.Printf("Error getting stale events on startup: %v", err)
	} else if len(staleEvents) > 0 {
		log.Printf("Found %d stale events to process on startup", len(staleEvents))
		s.processEventCompletionsForEvents(staleEvents)
	}

	// Take initial snapshot update for all active events asynchronously (don't block startup)
	go func() {
		log.Println("Taking initial snapshot update for active events...")
		s.updateActiveSnapshots()
	}()

	go s.runCompletionCheck()
	go s.runHourlyUpdates()
}

func (s *Scheduler) Stop() {
	log.Println("Stopping scheduler...")
	close(s.stopCompletion)
	close(s.stopHourly)
}

func (s *Scheduler) runCompletionCheck() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
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
			var metricName string
			if event.Type == "botw" {
				metricName = event.MetricJsonID
			} else {
				metricName = event.MetricJsonID
			}

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

	for _, event := range events {
		log.Printf("Processing event completion: %s (ID: %d, Guild: %s)", event.Type, event.ID, event.GuildID)

		guild, err := s.store.GetGuild(ctx, event.GuildID)
		if err == nil {
			// Update weekly leaderboard BEFORE completing event (while it's still active)
			if err := s.leaderboardService.UpdateWeeklyLeaderboard(ctx, event.GuildID, event.Type); err != nil {
				log.Printf("Failed to update weekly leaderboard before completion: %v", err)
			}
		}

		if err := s.eventService.CompleteEvent(ctx, event); err != nil {
			log.Printf("Failed to complete event %d: %v", event.ID, err)
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

		// Use UTC for rollover to maintain consistency
		rolloverResult, err := s.eventService.AutoRollover(ctx, event.GuildID, event.Type, event.EndTime)
		if err != nil {
			log.Printf("Failed to auto-rollover event %d: %v", event.ID, err)
			continue
		}

		// Update weekly leaderboard for new event
		if rolloverResult != nil {
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

func (s *Scheduler) runHourlyUpdates() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
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

func getEventDisplayName(eventType string) string {
	if eventType == "botw" {
		return "Boss of the Week"
	}
	return "Skill of the Week"
}

func getMetricLabel(eventType string) string {
	if eventType == "botw" {
		return "Boss"
	}
	return "Skill"
}

func getEventColor(eventType string) int {
	if eventType == "botw" {
		return 0x00FF00
	}
	return 0x0099FF
}
