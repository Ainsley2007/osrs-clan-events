package discord

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/bwmarrin/discordgo"
)

func (b *Bot) stopCommand() Command {
	return Command{
		Definition: &discordgo.ApplicationCommand{
			Name:        "stop",
			Description: "Stop all active competitions and award points",
		},
		Handler: b.handleStop,
	}
}

func (b *Bot) handleStop(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !hasAdminPermission(s, i.GuildID, i.Member.User.ID) {
		respondError(s, i.Interaction, errors.New("you must be an administrator to use this command"))
		return
	}

	ctx := context.Background()

	activeBotwEvents, err := b.Store.GetActiveEvents(ctx, i.GuildID, "botw")
	if err != nil {
		respondError(s, i.Interaction, fmt.Errorf("failed to get BOTW events: %w", err))
		return
	}

	activeSotwEvents, err := b.Store.GetActiveEvents(ctx, i.GuildID, "sotw")
	if err != nil {
		respondError(s, i.Interaction, fmt.Errorf("failed to get SOTW events: %w", err))
		return
	}

	if len(activeBotwEvents) == 0 && len(activeSotwEvents) == 0 {
		respondError(s, i.Interaction, errors.New("no active competitions to stop"))
		return
	}

	// Build response message immediately
	var stoppedEvents []string
	for _, event := range activeBotwEvents {
		stoppedEvents = append(stoppedEvents, fmt.Sprintf("**BOTW:** %s (Week %d)", event.MetricJsonID, event.WeekNumber))
	}
	for _, event := range activeSotwEvents {
		stoppedEvents = append(stoppedEvents, fmt.Sprintf("**SOTW:** %s (Week %d)", event.MetricJsonID, event.WeekNumber))
	}

	description := "**Stopped:**\n"
	for _, eventDesc := range stoppedEvents {
		description += eventDesc + "\n"
	}
	description += "\n✅ Stopping competitions and calculating points...\n💡 Use `/start` to begin new competitions"

	responseEmbed := &discordgo.MessageEmbed{
		Title:       "🛑 Competitions Stopped",
		Description: description,
		Color:       0xFF6600,
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{responseEmbed},
		},
	})

	// Do heavy work asynchronously (completing events, leaderboard updates, logging)
	go func() {
		ctx := context.Background()
		var botwPointsAwarded, sotwPointsAwarded int

		for _, event := range activeBotwEvents {
			if err := b.EventService.CompleteEvent(ctx, event); err != nil {
				log.Printf("Failed to complete BOTW event: %v", err)
				continue
			}
			snapshots, _ := b.Store.GetSnapshotsWithAccounts(ctx, event.ID)
			botwPointsAwarded = len(snapshots)
		}

		for _, event := range activeSotwEvents {
			if err := b.EventService.CompleteEvent(ctx, event); err != nil {
				log.Printf("Failed to complete SOTW event: %v", err)
				continue
			}
			snapshots, _ := b.Store.GetSnapshotsWithAccounts(ctx, event.ID)
			sotwPointsAwarded = len(snapshots)
		}

		// Update weekly leaderboards (will show final state)
		if len(activeBotwEvents) > 0 {
			if err := b.LeaderboardService.UpdateWeeklyLeaderboard(ctx, i.GuildID, "botw"); err != nil {
				log.Printf("Failed to update BOTW weekly leaderboard: %v", err)
			}
		}
		if len(activeSotwEvents) > 0 {
			if err := b.LeaderboardService.UpdateWeeklyLeaderboard(ctx, i.GuildID, "sotw"); err != nil {
				log.Printf("Failed to update SOTW weekly leaderboard: %v", err)
			}
		}

		// Update overall leaderboards
		if len(activeBotwEvents) > 0 {
			if err := b.LeaderboardService.UpdateOverallLeaderboard(ctx, i.GuildID, "botw"); err != nil {
				log.Printf("Failed to update BOTW overall leaderboard: %v", err)
			}
		}
		if len(activeSotwEvents) > 0 {
			if err := b.LeaderboardService.UpdateOverallLeaderboard(ctx, i.GuildID, "sotw"); err != nil {
				log.Printf("Failed to update SOTW overall leaderboard: %v", err)
			}
		}

		guild, err := b.Store.GetGuild(ctx, i.GuildID)
		if err == nil && guild.LogChannelID != "" {
			SendCompetitionStoppedLog(s, guild.LogChannelID, stoppedEvents, botwPointsAwarded, sotwPointsAwarded, i.Member.User.ID)
		}
	}()
}
