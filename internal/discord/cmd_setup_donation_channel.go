package discord

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
)

func (b *Bot) setupDonationChannelCommand() Command {
	return Command{
		Definition: &discordgo.ApplicationCommand{
			Name:                    "setup-donation-channel",
			Description:             "Set the channel where donation leaderboard will be displayed",
			DefaultMemberPermissions: ptr(int64(discordgo.PermissionAdministrator)),
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionChannel,
					Name:        "channel",
					Description: "The channel to use for donation leaderboard",
					Required:    true,
				},
			},
		},
		Handler: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			if i.GuildID == "" {
				respondError(s, i.Interaction, fmt.Errorf("this command can only be used in a server"))
				return
			}
			if _, ok := requireAdmin(s, i); !ok {
				return
			}

			ctx, cancel := cmdContext()
			defer cancel()
			data := i.ApplicationCommandData()

			var channelID string
			for _, opt := range data.Options {
				if opt.Name == "channel" {
					channelID = opt.ChannelValue(s).ID
					break
				}
			}

			if channelID == "" {
				respondError(s, i.Interaction, fmt.Errorf("channel option is required"))
				return
			}

			guildID := i.GuildID

			if err := b.guildService.UpdateDonationChannel(ctx, guildID, channelID); err != nil {
				respondError(s, i.Interaction, err)
				return
			}

			if err := b.donationService.CreateOrUpdateLeaderboard(ctx, guildID); err != nil {
				respondError(s, i.Interaction, fmt.Errorf("failed to create leaderboard: %w", err))
				return
			}

			channel := data.Options[0].ChannelValue(s)
			respondSuccess(s, i.Interaction, fmt.Sprintf("✅ Donation channel set to <#%s>", channel.ID))
		},
	}
}
