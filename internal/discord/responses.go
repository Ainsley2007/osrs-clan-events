package discord

import (
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
)

func respondError(s *discordgo.Session, i *discordgo.Interaction, err error) {
	log.Printf("Error handling interaction: %v", err)
	s.InteractionRespond(i, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("❌ An error occurred: %v", err),
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}

func respondSuccess(s *discordgo.Session, i *discordgo.Interaction, message string) {
	s.InteractionRespond(i, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: message,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}
