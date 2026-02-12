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
			Name:                     "start",
			Description:              "Start weekly BOTW and SOTW competitions",
			DefaultMemberPermissions: ptr(int64(discordgo.PermissionAdministrator)),
		},
		Handler: b.handleStart,
	}
}

func (b *Bot) handleStart(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !hasAdminPermission(s, i.GuildID, i.Member.User.ID) {
		respondError(s, i.Interaction, errors.New("you must be an administrator to use this command"))
		return
	}

	// Defer immediately so Discord gets a response within the 3s window; all other work runs in the goroutine.
	if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "⏳ Starting BOTW and SOTW...",
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	}); err != nil {
		log.Printf("Failed to defer start interaction: %v", err)
		return
	}

	go b.runStartAndEditReply(s, i)
}

func (b *Bot) runStartAndEditReply(s *discordgo.Session, i *discordgo.InteractionCreate) {
	ctx := context.Background()

	activeBotwEvents, err := b.Store.GetActiveEvents(ctx, i.GuildID, "botw")
	if err == nil && len(activeBotwEvents) > 0 {
		editDeferredWithError(s, i.Interaction, errors.New("⏰ BOTW competition is already running! Use /stop first"))
		return
	}
	activeSotwEvents, err := b.Store.GetActiveEvents(ctx, i.GuildID, "sotw")
	if err == nil && len(activeSotwEvents) > 0 {
		editDeferredWithError(s, i.Interaction, errors.New("⏰ SOTW competition is already running! Use /stop first"))
		return
	}

	startTime := time.Now().UTC()

	botwResult, err := b.EventService.StartBotw(ctx, i.GuildID, startTime)
	if err != nil {
		editDeferredWithError(s, i.Interaction, fmt.Errorf("failed to start BOTW: %w", err))
		return
	}

	sotwResult, err := b.EventService.StartSotw(ctx, i.GuildID, startTime)
	if err != nil {
		editDeferredWithError(s, i.Interaction, fmt.Errorf("failed to start SOTW: %w", err))
		return
	}

	// Update weekly and overall leaderboards (leaderboard service logs failures)
	b.LeaderboardService.UpdateWeeklyLeaderboard(ctx, i.GuildID, "botw")
	b.LeaderboardService.UpdateWeeklyLeaderboard(ctx, i.GuildID, "sotw")
	b.LeaderboardService.UpdateOverallLeaderboard(ctx, i.GuildID, "botw")
	b.LeaderboardService.UpdateOverallLeaderboard(ctx, i.GuildID, "sotw")

	guild, err := b.Store.GetGuild(ctx, i.GuildID)
	if err == nil {
		if err := b.InitializerService.RenameCategoryForEvent(ctx, guild, "botw", botwResult.Event); err != nil {
			log.Printf("Failed to rename BOTW category: %v", err)
		}
		if err := b.InitializerService.RenameCategoryForEvent(ctx, guild, "sotw", sotwResult.Event); err != nil {
			log.Printf("Failed to rename SOTW category: %v", err)
		}
		if guild.LogChannelID != "" {
			SendCompetitionStartedLog(s, guild.LogChannelID, botwResult.MetricName, sotwResult.MetricName,
				botwResult.Event.WeekNumber, sotwResult.Event.WeekNumber, i.Member.User.ID)
		}

	}

	// Edit deferred reply with simple success message
	content := "✅ Weekly competitions started successfully!"
	if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{Content: &content}); err != nil {
		log.Printf("Failed to edit deferred start response: %v", err)
	}
}

func editDeferredWithError(s *discordgo.Session, i *discordgo.Interaction, err error) {
	log.Printf("Error handling start interaction: %v", err)
	content := fmt.Sprintf("❌ An error occurred: %v", err)
	if _, e := s.InteractionResponseEdit(i, &discordgo.WebhookEdit{Content: &content}); e != nil {
		log.Printf("Failed to edit deferred start error response: %v", e)
	}
}
