package discord

import (
	"fmt"
	"log"
	"time"

	"osrs-events/internal/discord/services"

	"github.com/bwmarrin/discordgo"
)

func (b *Bot) messageReactionAdd(s *discordgo.Session, event *discordgo.MessageReactionAdd) {
	if event == nil || event.GuildID == "" {
		return
	}
	if s.State != nil && s.State.User != nil && event.UserID == s.State.User.ID {
		return
	}

	emoji := event.Emoji.Name
	if emoji != services.PBApproveEmoji && emoji != services.PBRejectEmoji {
		return
	}
	if !hasAdminPermission(s, event.GuildID, event.UserID) {
		return
	}

	ctx, cancel := cmdContext()
	defer cancel()

	if emoji == services.PBRejectEmoji {
		submission, err := b.PBService.HandleRejection(ctx, event.GuildID, event.MessageID, event.UserID)
		if err != nil {
			return
		}
		if submission.ProofMessageID != nil {
			if err := s.ChannelMessageDelete(event.ChannelID, *submission.ProofMessageID); err != nil {
				log.Printf("failed to delete rejected pb proof message %s: %v", *submission.ProofMessageID, err)
			}
		}
		return
	}

	result, err := b.PBService.HandleApproval(ctx, event.GuildID, event.MessageID, event.UserID)
	if err != nil {
		_, _ = s.ChannelMessageSend(event.ChannelID, fmt.Sprintf("⚠️ Could not approve PB submission: %v", err))
		return
	}
	if err := b.PBService.MarkProofSubmissionAccepted(event.ChannelID, result, event.UserID, time.Now().UTC()); err != nil {
		log.Printf("failed to mark accepted proof message reviewed: %v", err)
	}

	if result.ImprovedPersonalBest {
		_, _ = s.ChannelMessageSend(event.ChannelID, fmt.Sprintf("✅ Approved **%s** for %s. Leaderboard updated.", result.Submission.DisplayName, result.Category.DisplayName))
		return
	}
	_, _ = s.ChannelMessageSend(event.ChannelID, fmt.Sprintf("✅ Approved **%s** for %s. Existing PB is already faster.", result.Submission.DisplayName, result.Category.DisplayName))
}
