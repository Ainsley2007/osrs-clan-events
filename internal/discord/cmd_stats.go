package discord

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

func (b *Bot) statsCommand() Command {
	return Command{
		Definition: &discordgo.ApplicationCommand{
			Name:        "stats",
			Description: "View your account progress in all events",
		},
		Handler: b.handleStats,
	}
}

func (b *Bot) handleStats(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Defer immediately
	if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	}); err != nil {
		log.Printf("Failed to defer stats interaction: %v", err)
		return
	}

	go b.runStatsAndEditReply(s, i)
}

func (b *Bot) runStatsAndEditReply(s *discordgo.Session, i *discordgo.InteractionCreate) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	userID := i.Member.User.ID
	guildID := i.GuildID

	// Get stats from service
	botwStats, sotwStats, err := b.StatsService.GetUserEventStats(ctx, userID, guildID)
	if err != nil {
		editDeferredWithError(s, i.Interaction, err)
		return
	}

	// Build message
	var message strings.Builder
	message.WriteString("📊 **Your Event Statistics**\n\n")

	if len(botwStats) > 0 {
		message.WriteString("**BOTW Events**\n")
		for _, eventStat := range botwStats {
			message.WriteString(fmt.Sprintf("• %s (Week %d):\n", eventStat.MetricName, eventStat.WeekNumber))
			for _, accountStat := range eventStat.AccountStats {
				message.WriteString(fmt.Sprintf("  - `%s`: %s KC\n", accountStat.RSN, formatNumber(accountStat.Gain)))
			}
			message.WriteString(fmt.Sprintf("  **Points**: %d\n", eventStat.Points))
		}
		message.WriteString("\n")
	}

	if len(sotwStats) > 0 {
		message.WriteString("**SOTW Events**\n")
		for _, eventStat := range sotwStats {
			message.WriteString(fmt.Sprintf("• %s (Week %d):\n", eventStat.MetricName, eventStat.WeekNumber))
			for _, accountStat := range eventStat.AccountStats {
				message.WriteString(fmt.Sprintf("  - `%s`: %s XP\n", accountStat.RSN, formatNumber(accountStat.Gain)))
			}
			message.WriteString(fmt.Sprintf("  **Points**: %d\n", eventStat.Points))
		}
	}

	if len(botwStats) == 0 && len(sotwStats) == 0 {
		message.WriteString("No events found for this guild or no progress yet.")
	}

	content := message.String()
	if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: &content,
	}); err != nil {
		log.Printf("Failed to edit stats response: %v", err)
	}
}

