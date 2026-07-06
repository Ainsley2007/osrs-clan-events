package discord

import (
	"context"
	"fmt"
	"log"
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
	if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	}); err != nil {
		log.Printf("Failed to defer stats interaction: %v", err)
		return
	}

	goSafe("stats", func() { b.runStatsAndEditReply(s, i) })
}

func (b *Bot) runStatsAndEditReply(s *discordgo.Session, i *discordgo.InteractionCreate) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	userID, ok := interactionActorID(i)
	if !ok {
		editDeferredWithError(s, i.Interaction, fmt.Errorf("could not resolve command user"))
		return
	}
	guildID := i.GuildID

	botwStats, sotwStats, err := b.statsService.GetUserEventStats(ctx, userID, guildID)
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

	var embeds []*discordgo.MessageEmbed

	embeds = buildStatsEmbeds(botwStats, sotwStats)

	if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Embeds: &embeds,
	}); err != nil {
		log.Printf("Failed to edit stats response: %v", err)
	}
}

