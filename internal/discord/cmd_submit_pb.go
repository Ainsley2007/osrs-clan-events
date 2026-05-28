package discord

import (
	"fmt"
	"log"
	"strings"

	"osrs-events/internal/discord/services"

	"github.com/bwmarrin/discordgo"
)

func (b *Bot) submitPBCommand() Command {
	return Command{
		Definition: &discordgo.ApplicationCommand{
			Name:        "submit-pb",
			Description: "Submit a personal best proof for leaderboard review",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "category",
					Description: "PB category",
					Required:    true,
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{Name: "Inferno", Value: "inferno"},
						{Name: "Vorkath", Value: "vorkath"},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionAttachment,
					Name:        "attachment",
					Description: "Proof image attachment",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "time",
					Description: "In-game time format (MM:SS.xx or H:MM:SS.xx)",
					Required:    false,
				},
			},
		},
		Handler: b.handleSubmitPB,
	}
}

func (b *Bot) handleSubmitPB(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.GuildID == "" {
		respondError(s, i.Interaction, fmt.Errorf("this command can only be used in a server"))
		return
	}

	if err := respondDeferred(s, i.Interaction, "⏳ Submitting PB proof..."); err != nil {
		log.Printf("Failed to defer submit-pb interaction: %v", err)
		return
	}

	data := i.ApplicationCommandData()
	categoryOpt := data.GetOption("category")
	attachmentOpt := data.GetOption("attachment")
	timeOpt := data.GetOption("time")
	if categoryOpt == nil || attachmentOpt == nil {
		editDeferredWithError(s, i.Interaction, fmt.Errorf("missing required options"))
		return
	}

	attachment, err := getAttachmentOptionValue(data, attachmentOpt)
	if err != nil {
		editDeferredWithError(s, i.Interaction, err)
		return
	}

	userID, displayName := interactionUserAndDisplayName(i)

	var timeText *string
	if timeOpt != nil {
		value := strings.TrimSpace(timeOpt.StringValue())
		if value != "" {
			timeText = &value
		}
	}

	ctx, cancel := cmdContext()
	defer cancel()
	submission, category, err := b.PBService.SubmitPB(ctx, &services.PBSubmissionInput{
		GuildID:       i.GuildID,
		CategorySlug:  categoryOpt.StringValue(),
		DiscordUserID: userID,
		DisplayName:   displayName,
		TimeText:      timeText,
		ProofURL:      attachment.URL,
	})
	if err != nil {
		editDeferredWithError(s, i.Interaction, err)
		return
	}

	content := fmt.Sprintf(
		"✅ PB submission queued for review.\nCategory: **%s**\nSubmission ID: `%d`",
		category.DisplayName,
		submission.ID,
	)
	editDeferredContent(s, i.Interaction, content)
}

func getAttachmentOptionValue(data discordgo.ApplicationCommandInteractionData, option *discordgo.ApplicationCommandInteractionDataOption) (*discordgo.MessageAttachment, error) {
	if option == nil {
		return nil, fmt.Errorf("attachment option missing")
	}

	attachmentID := fmt.Sprintf("%v", option.Value)
	if data.Resolved == nil || data.Resolved.Attachments == nil {
		return nil, fmt.Errorf("failed to resolve attachment")
	}
	attachment, ok := data.Resolved.Attachments[attachmentID]
	if !ok || attachment == nil {
		return nil, fmt.Errorf("failed to resolve attachment")
	}
	return attachment, nil
}

func interactionUserAndDisplayName(i *discordgo.InteractionCreate) (string, string) {
	if i.Member != nil && i.Member.User != nil {
		userID := i.Member.User.ID
		if strings.TrimSpace(i.Member.Nick) != "" {
			return userID, i.Member.Nick
		}
		if strings.TrimSpace(i.Member.User.GlobalName) != "" {
			return userID, i.Member.User.GlobalName
		}
		return userID, i.Member.User.Username
	}
	if i.User != nil {
		if strings.TrimSpace(i.User.GlobalName) != "" {
			return i.User.ID, i.User.GlobalName
		}
		return i.User.ID, i.User.Username
	}
	return "", "Unknown User"
}
