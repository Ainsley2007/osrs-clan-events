package discord

import (
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
)

func respondError(s *discordgo.Session, i *discordgo.Interaction, err error) {
	log.Printf("Error handling interaction: %v", err)
	if respondErr := s.InteractionRespond(i, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("❌ An error occurred: %v", err),
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	}); respondErr != nil {
		log.Printf("Failed to send error response: %v", respondErr)
	}
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

// respondDeferred sends a deferred response (use within 3s) so the work can continue async.
// Call editDeferredContent afterwards to update with the final result.
func respondDeferred(s *discordgo.Session, i *discordgo.Interaction, loadingMessage string) error {
	return s.InteractionRespond(i, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: loadingMessage,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}

func editDeferredContent(s *discordgo.Session, i *discordgo.Interaction, content string) {
	if _, err := s.InteractionResponseEdit(i, &discordgo.WebhookEdit{Content: &content}); err != nil {
		log.Printf("Failed to edit deferred response: %v", err)
	}
}
