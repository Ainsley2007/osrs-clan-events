package discord

import (
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
	ctx, cancel := cmdContext()
	defer cancel()

	botwRunning, err := b.eventService.IsEventRunning(ctx, i.GuildID, "botw")
	if err != nil {
		editDeferredWithError(s, i.Interaction, fmt.Errorf("failed to check BOTW status: %w", err))
		return
	}
	if botwRunning {
		editDeferredWithError(s, i.Interaction, errors.New("⏰ BOTW competition is already running! Use /stop first"))
		return
	}
	sotwRunning, err := b.eventService.IsEventRunning(ctx, i.GuildID, "sotw")
	if err != nil {
		editDeferredWithError(s, i.Interaction, fmt.Errorf("failed to check SOTW status: %w", err))
		return
	}
	if sotwRunning {
		editDeferredWithError(s, i.Interaction, errors.New("⏰ SOTW competition is already running! Use /stop first"))
		return
	}

	startTime := time.Now().UTC()

	botwResult, err := b.eventService.StartBotw(ctx, i.GuildID, startTime)
	if err != nil {
		editDeferredWithError(s, i.Interaction, fmt.Errorf("failed to start BOTW: %w", err))
		return
	}

	sotwResult, err := b.eventService.StartSotw(ctx, i.GuildID, startTime)
	if err != nil {
		if abortErr := b.eventService.AbortStartedEvent(ctx, botwResult.Event); abortErr != nil {
			log.Printf("Failed to roll back BOTW after SOTW start failed: %v", abortErr)
		}
		editDeferredWithError(s, i.Interaction, fmt.Errorf("failed to start SOTW: %w", err))
		return
	}

	// Update weekly and overall leaderboards (leaderboard service logs failures)
	b.leaderboardService.UpdateWeeklyLeaderboard(ctx, i.GuildID, "botw")
	b.leaderboardService.UpdateWeeklyLeaderboard(ctx, i.GuildID, "sotw")
	b.leaderboardService.UpdateOverallLeaderboard(ctx, i.GuildID, "botw")
	b.leaderboardService.UpdateOverallLeaderboard(ctx, i.GuildID, "sotw")

	guild, err := b.guildService.GetGuild(ctx, i.GuildID)
	if err == nil {
		if err := b.initializerService.RenameCategoryForEvent(ctx, guild, "botw", botwResult.Event); err != nil {
			log.Printf("Failed to rename BOTW category: %v", err)
		}
		if err := b.initializerService.RenameCategoryForEvent(ctx, guild, "sotw", sotwResult.Event); err != nil {
			log.Printf("Failed to rename SOTW category: %v", err)
		}
		if guild.LogChannelID != "" {
			sendCompetitionStartedLog(s, guild.LogChannelID, botwResult.MetricName, sotwResult.MetricName,
				botwResult.Event.WeekNumber, sotwResult.Event.WeekNumber, i.Member.User.ID)
		}

	}

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
