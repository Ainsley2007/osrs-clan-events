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

	if len(botwStats) == 0 && len(sotwStats) == 0 {
		content := "No events found for this guild or no progress yet."
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &content,
		})
		return
	}

	// Build embeds
	var embeds []*discordgo.MessageEmbed

	// BOTW Events embed
	if len(botwStats) > 0 {
		for _, eventStat := range botwStats {
			var description strings.Builder
			description.WriteString("```\n")
			for _, accountStat := range eventStat.AccountStats {
				description.WriteString(fmt.Sprintf("%-20s %10s KC\n", accountStat.RSN, formatNumber(accountStat.Gain)))
			}
			description.WriteString("```")

			embed := &discordgo.MessageEmbed{
				Title:       fmt.Sprintf("🗡️ BOTW - %s (Week %d)", eventStat.MetricName, eventStat.WeekNumber),
				Description: description.String(),
				Color:       0xFF6B6B, // Red
				Footer: &discordgo.MessageEmbedFooter{
					Text: fmt.Sprintf("Points: %d", eventStat.Points),
				},
			}
			embeds = append(embeds, embed)
		}
	}

	// SOTW Events embed
	if len(sotwStats) > 0 {
		for _, eventStat := range sotwStats {
			var description strings.Builder
			description.WriteString("```\n")
			for _, accountStat := range eventStat.AccountStats {
				description.WriteString(fmt.Sprintf("%-20s %10s XP\n", accountStat.RSN, formatNumber(accountStat.Gain)))
			}
			description.WriteString("```")

			embed := &discordgo.MessageEmbed{
				Title:       fmt.Sprintf("⚔️ SOTW - %s (Week %d)", eventStat.MetricName, eventStat.WeekNumber),
				Description: description.String(),
				Color:       0x4ECDC4, // Teal
				Footer: &discordgo.MessageEmbedFooter{
					Text: fmt.Sprintf("Points: %d", eventStat.Points),
				},
			}
			embeds = append(embeds, embed)
		}
	}

	if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Embeds: &embeds,
	}); err != nil {
		log.Printf("Failed to edit stats response: %v", err)
	}
}

