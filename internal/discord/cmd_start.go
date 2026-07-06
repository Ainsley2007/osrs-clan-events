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
	if _, ok := requireAdmin(s, i); !ok {
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

	goSafe("start", func() { b.runStartAndEditReply(s, i) })
}

func (b *Bot) runStartAndEditReply(s *discordgo.Session, i *discordgo.InteractionCreate) {
	ctx, cancel := cmdContext()
	defer cancel()

	activeBotwEvents, err := b.eventService.GetActiveEvents(ctx, i.GuildID, "botw")
	if err != nil {
		editDeferredWithError(s, i.Interaction, fmt.Errorf("failed to check BOTW status: %w", err))
		return
	}
	if len(activeBotwEvents) > 0 {
		editDeferredWithError(s, i.Interaction, errors.New("⏰ BOTW competition is already running! Use /stop first"))
		return
	}
	activeSotwEvents, err := b.eventService.GetActiveEvents(ctx, i.GuildID, "sotw")
	if err != nil {
		editDeferredWithError(s, i.Interaction, fmt.Errorf("failed to check SOTW status: %w", err))
		return
	}
	if len(activeSotwEvents) > 0 {
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
		if abortErr := b.eventService.AbortActiveEventIfPresent(ctx, i.GuildID, "sotw"); abortErr != nil {
			log.Printf("Failed to roll back partial SOTW after start failed: %v", abortErr)
		}
		editDeferredWithError(s, i.Interaction, fmt.Errorf("failed to start SOTW: %w", err))
		return
	}

	b.leaderboardService.RefreshLeaderboards(ctx, i.GuildID)

	guild, err := b.guildService.GetGuild(ctx, i.GuildID)
	if err == nil {
		if err := b.initializerService.RenameCategoryForEvent(ctx, guild, "botw", botwResult.Event); err != nil {
			log.Printf("Failed to rename BOTW category: %v", err)
		}
		if err := b.initializerService.RenameCategoryForEvent(ctx, guild, "sotw", sotwResult.Event); err != nil {
			log.Printf("Failed to rename SOTW category: %v", err)
		}
		if guild.LogChannelID != "" {
			startedBy := ""
			if actor, ok := interactionActor(i); ok {
				startedBy = actor.ID
			}
			sendCompetitionStartedLog(s, guild.LogChannelID, botwResult.MetricName, sotwResult.MetricName,
				botwResult.Event.WeekNumber, sotwResult.Event.WeekNumber, startedBy)
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
