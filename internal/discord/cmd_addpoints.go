package discord

import (
	"errors"
	"fmt"
	"strconv"

	"osrs-events/internal/discord/services"

	"github.com/bwmarrin/discordgo"
)

func (b *Bot) addPointsCommand() Command {
	return Command{
		Definition: &discordgo.ApplicationCommand{
			Name:                    "addpoints",
			Description:             "Add BOTW or SOTW total points to a participant (admin only)",
			DefaultMemberPermissions: ptr(int64(discordgo.PermissionAdministrator)),
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionUser,
					Name:        "user",
					Description: "The user to add points to",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "type",
					Description: "BOTW or SOTW",
					Required:    true,
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{Name: "BOTW", Value: "botw"},
						{Name: "SOTW", Value: "sotw"},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionInteger,
					Name:        "amount",
					Description: "Points to add (negative to subtract)",
					Required:    true,
				},
			},
		},
		Handler: b.handleAddPoints,
	}
}

func (b *Bot) handleAddPoints(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.GuildID == "" {
		respondError(s, i.Interaction, errors.New("this command can only be used in a server"))
		return
	}
	actor, ok := requireAdmin(s, i)
	if !ok {
		return
	}

	data := i.ApplicationCommandData()
	var targetUserID, eventType string
	var amount int
	for _, opt := range data.Options {
		switch opt.Name {
		case "user":
			targetUserID = opt.UserValue(s).ID
		case "type":
			eventType = opt.StringValue()
		case "amount":
			amount = int(opt.IntValue())
		}
	}
	if targetUserID == "" || eventType == "" {
		respondError(s, i.Interaction, errors.New("user and type are required"))
		return
	}

	ctx, cancel := cmdContext()
	defer cancel()
	err := b.participantService.AddPoints(ctx, i.GuildID, targetUserID, eventType, amount)
	if err != nil {
		if errors.Is(err, services.ErrParticipantNotFound) {
			respondError(s, i.Interaction, errors.New("that user is not a participant in this server"))
			return
		}
		respondError(s, i.Interaction, err)
		return
	}

	label := "BOTW"
	if eventType == "sotw" {
		label = "SOTW"
	}
	respondSuccess(s, i.Interaction, fmt.Sprintf("✅ Added %s %s points to <@%s>.", formatPoints(int64(amount)), label, targetUserID))

	b.leaderboardService.RefreshLeaderboards(ctx, i.GuildID)

	logMsg := fmt.Sprintf("➕ <@%s> added %s %s points to <@%s>.", actor.ID, formatPoints(int64(amount)), label, targetUserID)
	b.logAction(ctx, i.GuildID, logMsg)
}

func formatPoints(n int64) string {
	return strconv.FormatInt(n, 10)
}
