package discord

import (
	"context"
	"errors"
	"fmt"

	"github.com/bwmarrin/discordgo"
)

func (b *Bot) addDonationCommand() Command {
	return Command{
		Definition: &discordgo.ApplicationCommand{
			Name:                    "add-donation",
			Description:             "Add a donation entry (admin only)",
			DefaultMemberPermissions: ptr(int64(discordgo.PermissionAdministrator)),
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionUser,
					Name:        "user",
					Description: "The user who made the donation",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionNumber,
					Name:        "amount",
					Description: "Donation amount in millions (e.g. 1.5 = 1.5m, 100 = 100m)",
					Required:    true,
					MinValue:    ptr(1.0),
				},
			},
		},
		Handler: b.handleAddDonation,
	}
}

func (b *Bot) handleAddDonation(s *discordgo.Session, i *discordgo.InteractionCreate) {
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

	var targetUserID string
	var amountM float64
	for _, opt := range data.Options {
		switch opt.Name {
		case "user":
			targetUserID = opt.UserValue(s).ID
		case "amount":
			amountM = opt.FloatValue()
		}
	}

	if targetUserID == "" {
		respondError(s, i.Interaction, errors.New("user is required"))
		return
	}

	// Check if donation channel is set (hidden error if not)
	guild, err := b.Store.GetGuild(ctx, i.GuildID)
	if err != nil || guild.DonationChannelID == "" {
		respondError(s, i.Interaction, errors.New("donation channel not configured. Use /setup-donation-channel first"))
		return
	}

	amountGP := int64(amountM * 1_000_000)
	if amountGP <= 0 {
		respondError(s, i.Interaction, errors.New("donation amount must be positive"))
		return
	}

	err = b.DonationService.AddDonation(ctx, i.GuildID, targetUserID, amountGP, i.Member.User.ID)
	if err != nil {
		respondError(s, i.Interaction, fmt.Errorf("failed to add donation: %w", err))
		return
	}

	respondSuccess(s, i.Interaction, fmt.Sprintf("✅ Added `%s` to clan fund from <@%s>.", formatAmountM(amountGP), targetUserID))

	// Update leaderboard
	if err := b.DonationService.UpdateLeaderboard(ctx, i.GuildID); err != nil {
		// Log error but don't fail the command
		b.logger.Printf("Failed to update donation leaderboard: %v", err)
	}

	// Log to logging channel
	logMsg := fmt.Sprintf("➕ <@%s> added `%s` to clan fund from <@%s>.", i.Member.User.ID, formatAmountM(amountGP), targetUserID)
	b.logAction(ctx, i.GuildID, logMsg)
}
