package discord

import (
	"fmt"
	"log"
	"slices"
	"strings"

	"osrs-events/internal/database"
	"osrs-events/internal/discord/services"

	"github.com/bwmarrin/discordgo"
)

const maxPBCategoryAutocompleteChoices = 25

func (b *Bot) submitPBCommand() Command {
	return Command{
		Definition: &discordgo.ApplicationCommand{
			Name:        "submit-pb",
			Description: "Submit a personal best proof for leaderboard review",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:         discordgo.ApplicationCommandOptionString,
					Name:         "category",
					Description:  "PB category",
					Required:     true,
					Autocomplete: true,
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
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "teammates",
					Description: "Optional: @ tag up to 4 clan teammates on this PB",
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
	if categoryOpt == nil || attachmentOpt == nil || timeOpt == nil {
		editDeferredWithError(s, i.Interaction, fmt.Errorf("missing required options"))
		return
	}

	attachment, err := getAttachmentOptionValue(data, attachmentOpt)
	if err != nil {
		editDeferredWithError(s, i.Interaction, err)
		return
	}

	timeValue := strings.TrimSpace(timeOpt.StringValue())
	if timeValue == "" {
		editDeferredWithError(s, i.Interaction, fmt.Errorf("time is required"))
		return
	}

	userID, displayName := interactionUserAndDisplayName(i)

	teammatesRaw := ""
	if teammatesOpt := data.GetOption("teammates"); teammatesOpt != nil {
		teammatesRaw = teammatesOpt.StringValue()
	}

	ctx, cancel := cmdContext()
	defer cancel()
	submission, category, err := b.pbService.SubmitPB(ctx, &services.PBSubmissionInput{
		GuildID:       i.GuildID,
		CategorySlug:  categoryOpt.StringValue(),
		DiscordUserID: userID,
		DisplayName:   displayName,
		TeammatesRaw:  teammatesRaw,
		TimeText:      &timeValue,
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

func (b *Bot) handlePBCategoryAutocomplete(s *discordgo.Session, i *discordgo.InteractionCreate) {
	ctx, cancel := cmdContext()
	defer cancel()

	categories, err := b.pbService.GetActivePBCategories(ctx)
	if err != nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionApplicationCommandAutocompleteResult,
			Data: &discordgo.InteractionResponseData{Choices: []*discordgo.ApplicationCommandOptionChoice{}},
		})
		return
	}

	query := pbCategoryAutocompleteQuery(i.ApplicationCommandData().Options)
	choices := pbCategoryAutocompleteChoices(categories, query)

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{Choices: choices},
	})
}

func pbCategoryAutocompleteQuery(options []*discordgo.ApplicationCommandInteractionDataOption) string {
	for _, opt := range options {
		if opt == nil || !opt.Focused || opt.Name != "category" {
			continue
		}
		if opt.Type != discordgo.ApplicationCommandOptionString {
			return ""
		}
		return opt.StringValue()
	}
	return ""
}

func pbCategoryAutocompleteChoices(categories []*database.PBCategory, query string) []*discordgo.ApplicationCommandOptionChoice {
	query = strings.ToLower(strings.TrimSpace(query))

	matches := make([]*database.PBCategory, 0, len(categories))
	for _, category := range categories {
		if category == nil || !category.IsActive {
			continue
		}
		if query != "" && !strings.HasPrefix(strings.ToLower(category.DisplayName), query) {
			continue
		}
		matches = append(matches, category)
	}

	slices.SortFunc(matches, func(a, b *database.PBCategory) int {
		return strings.Compare(strings.ToLower(a.DisplayName), strings.ToLower(b.DisplayName))
	})

	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0, min(len(matches), maxPBCategoryAutocompleteChoices))
	for _, category := range matches {
		if len(choices) >= maxPBCategoryAutocompleteChoices {
			break
		}
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
			Name:  category.DisplayName,
			Value: category.Slug,
		})
	}
	return choices
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
