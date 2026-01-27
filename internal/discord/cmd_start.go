package discord

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/bwmarrin/discordgo"
)

func (b *Bot) startCommand() Command {
	return Command{
		Definition: &discordgo.ApplicationCommand{
			Name:        "start",
			Description: "Start weekly BOTW and SOTW competitions",
		},
		Handler: b.handleStart,
	}
}

func (b *Bot) handleStart(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !hasAdminPermission(s, i.GuildID, i.Member.User.ID) {
		respondError(s, i.Interaction, errors.New("you must be an administrator to use this command"))
		return
	}

	ctx := context.Background()

	activeBotwEvents, err := b.Store.GetActiveEvents(ctx, i.GuildID, "botw")
	if err == nil && len(activeBotwEvents) > 0 {
		respondError(s, i.Interaction, errors.New("⏰ BOTW competition is already running! Use /stop first"))
		return
	}

	activeSotwEvents, err := b.Store.GetActiveEvents(ctx, i.GuildID, "sotw")
	if err == nil && len(activeSotwEvents) > 0 {
		respondError(s, i.Interaction, errors.New("⏰ SOTW competition is already running! Use /stop first"))
		return
	}

	startTime := time.Now().UTC()

	botwResult, err := b.EventService.StartBotw(ctx, i.GuildID, startTime)
	if err != nil {
		respondError(s, i.Interaction, fmt.Errorf("failed to start BOTW: %w", err))
		return
	}

	sotwResult, err := b.EventService.StartSotw(ctx, i.GuildID, startTime)
	if err != nil {
		respondError(s, i.Interaction, fmt.Errorf("failed to start SOTW: %w", err))
		return
	}

	// Respond immediately to avoid timeout, then do heavy work asynchronously
	responseEmbed := &discordgo.MessageEmbed{
		Title: "✅ Weekly Competitions Started!",
		Description: fmt.Sprintf("**BOTW:** %s\n**SOTW:** %s\n**Start:** %s\n**End:** %s\n\nInitial snapshots and leaderboards are being updated...",
			botwResult.MetricName,
			sotwResult.MetricName,
			startTime.Format("Jan 02, 2006 15:04"),
			botwResult.Event.EndTime.Format("Jan 02, 2006 15:04")),
		Color:     0x00AA00,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{responseEmbed},
		},
	})

	// Do heavy work asynchronously after responding
	go func() {
		ctx := context.Background()

		// Update weekly leaderboards
		if err := b.LeaderboardService.UpdateWeeklyLeaderboard(ctx, i.GuildID, "botw"); err != nil {
			log.Printf("Failed to update BOTW weekly leaderboard: %v", err)
		}
		if err := b.LeaderboardService.UpdateWeeklyLeaderboard(ctx, i.GuildID, "sotw"); err != nil {
			log.Printf("Failed to update SOTW weekly leaderboard: %v", err)
		}

		// Update overall leaderboards
		if err := b.LeaderboardService.UpdateOverallLeaderboard(ctx, i.GuildID, "botw"); err != nil {
			log.Printf("Failed to update BOTW overall leaderboard: %v", err)
		}
		if err := b.LeaderboardService.UpdateOverallLeaderboard(ctx, i.GuildID, "sotw"); err != nil {
			log.Printf("Failed to update SOTW overall leaderboard: %v", err)
		}

		guild, err := b.Store.GetGuild(ctx, i.GuildID)
		if err == nil && guild.LogChannelID != "" {
			SendCompetitionStartedLog(s, guild.LogChannelID, botwResult.MetricName, sotwResult.MetricName,
				botwResult.Event.WeekNumber, sotwResult.Event.WeekNumber, i.Member.User.ID)
		}
	}()
}
