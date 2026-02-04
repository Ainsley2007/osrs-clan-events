package discord

import (
	"context"
	"errors"
	"fmt"

	"github.com/bwmarrin/discordgo"
)

func (b *Bot) useFundsCommand() Command {
	return Command{
		Definition: &discordgo.ApplicationCommand{
			Name:                    "use-funds",
			Description:             "Record spending from donation funds (admin only)",
			DefaultMemberPermissions: ptr(int64(discordgo.PermissionAdministrator)),
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionInteger,
					Name:        "amount",
					Description: "Amount to spend",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "description",
					Description: "Reason/description for spending",
					Required:    false,
				},
			},
		},
		Handler: b.handleUseFunds,
	}
}

func (b *Bot) handleUseFunds(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.GuildID == "" {
		respondError(s, i.Interaction, errors.New("this command can only be used in a server"))
		return
	}
	if !hasAdminPermission(s, i.GuildID, i.Member.User.ID) {
		respondError(s, i.Interaction, errors.New("you must be an administrator to use this command"))
		return
	}

	ctx := context.Background()
	data := i.ApplicationCommandData()

	var amount int
	var description string
	for _, opt := range data.Options {
		switch opt.Name {
		case "amount":
			amount = int(opt.IntValue())
		case "description":
			description = opt.StringValue()
		}
	}

	if amount <= 0 {
		respondError(s, i.Interaction, errors.New("amount must be positive"))
		return
	}

	// Check if donation channel is set
	guild, err := b.Store.GetGuild(ctx, i.GuildID)
	if err != nil || guild.DonationChannelID == "" {
		respondError(s, i.Interaction, errors.New("donation channel not configured. Use /setup-donation-channel first"))
		return
	}

	err = b.DonationService.UseFunds(ctx, i.GuildID, amount, description, i.Member.User.ID)
	if err != nil {
		respondError(s, i.Interaction, err)
		return
	}

	descText := ""
	if description != "" {
		descText = fmt.Sprintf(": %s", description)
	}
	respondSuccess(s, i.Interaction, fmt.Sprintf("✅ Used `%s` from clan fund%s.", formatNumber(int64(amount)), descText))

	// Update leaderboard
	if err := b.DonationService.UpdateLeaderboard(ctx, i.GuildID); err != nil {
		// Log error but don't fail the command
		b.logger.Printf("Failed to update donation leaderboard: %v", err)
	}

	// Log to logging channel
	logMsg := fmt.Sprintf("➖ <@%s> used `%s` from clan fund%s.", i.Member.User.ID, formatNumber(int64(amount)), descText)
	b.logAction(ctx, i.GuildID, logMsg)
}
