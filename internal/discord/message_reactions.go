package discord

import (
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

	guildID := event.GuildID
	channelID := event.ChannelID
	messageID := event.MessageID
	userID := event.UserID

	if emoji == services.PBRejectEmoji {
		go b.runPBRejectionFollowUp(s, guildID, channelID, messageID, userID)
		return
	}

	go b.runPBApprovalFollowUp(guildID, channelID, messageID, userID)
}

func (b *Bot) runPBApprovalFollowUp(guildID, channelID, messageID, userID string) {
	ctx, cancel := cmdContext()
	defer cancel()

	if _, ok := b.pbService.PendingProofSubmissionForReaction(ctx, guildID, channelID, messageID); !ok {
		return
	}

	result, err := b.pbService.HandleApproval(ctx, guildID, messageID, userID)
	if err != nil {
		if services.IsPBSubmissionAlreadyReviewed(err) {
			return
		}
		log.Printf("pb approval failed for message %s in guild %s: %v", messageID, guildID, err)
		return
	}

	b.pbService.RunApprovalFollowUp(channelID, result, userID, time.Now().UTC())
}

func (b *Bot) runPBRejectionFollowUp(s *discordgo.Session, guildID, channelID, messageID, userID string) {
	ctx, cancel := cmdContext()
	defer cancel()

	if _, ok := b.pbService.PendingProofSubmissionForReaction(ctx, guildID, channelID, messageID); !ok {
		return
	}

	submission, err := b.pbService.HandleRejection(ctx, guildID, messageID, userID)
	if err != nil {
		if services.IsPBSubmissionAlreadyReviewed(err) {
			return
		}
		log.Printf("pb rejection failed for message %s in guild %s: %v", messageID, guildID, err)
		return
	}

	if submission.ProofMessageID != nil {
		if err := s.ChannelMessageDelete(channelID, *submission.ProofMessageID); err != nil {
			log.Printf("failed to delete rejected pb proof message %s: %v", *submission.ProofMessageID, err)
		}
	}
}
