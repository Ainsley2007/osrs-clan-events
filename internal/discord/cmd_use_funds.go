package discord

import (
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
					Type:        discordgo.ApplicationCommandOptionNumber,
					Name:        "amount",
					Description: "Amount to spend in millions (e.g. 1.5 = 1.5m, 100 = 100m)",
					Required:    true,
					MinValue:    ptr(1.0),
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

	ctx, cancel := cmdContext()
	defer cancel()
	data := i.ApplicationCommandData()

	var amountM float64
	var description string
	for _, opt := range data.Options {
		switch opt.Name {
		case "amount":
			amountM = opt.FloatValue()
		case "description":
			description = opt.StringValue()
		}
	}

	amountGP := int64(amountM * 1_000_000)
	if amountGP <= 0 {
		respondError(s, i.Interaction, errors.New("amount must be positive"))
		return
	}

	guild, err := b.guildService.GetGuild(ctx, i.GuildID)
	if err != nil || guild.DonationChannelID == "" {
		respondError(s, i.Interaction, errors.New("donation channel not configured. Use /setup-donation-channel first"))
		return
	}

	err = b.donationService.UseFunds(ctx, i.GuildID, amountGP, description, i.Member.User.ID)
	if err != nil {
		respondError(s, i.Interaction, err)
		return
	}

	descText := ""
	if description != "" {
		descText = fmt.Sprintf(": %s", description)
	}
	respondSuccess(s, i.Interaction, fmt.Sprintf("✅ Used `%s` from clan fund%s.", formatAmountM(amountGP), descText))

	if err := b.donationService.UpdateLeaderboard(ctx, i.GuildID); err != nil {
		// Log error but don't fail the command
		b.logger.Printf("Failed to update donation leaderboard: %v", err)
	}

	logMsg := fmt.Sprintf("➖ <@%s> used `%s` from clan fund%s.", i.Member.User.ID, formatAmountM(amountGP), descText)
	b.logAction(ctx, i.GuildID, logMsg)
}
