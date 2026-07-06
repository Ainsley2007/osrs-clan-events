package discord

import (
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func (b *Bot) trackedCommand() Command {
	return Command{
		Definition: &discordgo.ApplicationCommand{
			Name:        "tracked",
			Description: "View tracked OSRS accounts",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionUser,
					Name:        "user",
					Description: "The user to view accounts for (admin only)",
					Required:    false,
				},
			},
		},
		Handler: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			ctx, cancel := cmdContext()
			defer cancel()

			if i.GuildID == "" {
				respondError(s, i.Interaction, fmt.Errorf("this command can only be used in a server"))
				return
			}

			targetUser, isOtherUser, err := getTargetUser(s, i, "user")
			if err != nil {
				respondError(s, i.Interaction, err)
				return
			}

			accounts, err := b.accountService.GetTrackedAccounts(ctx, targetUser)
			if err != nil {
				respondError(s, i.Interaction, err)
				return
			}

			var title string
			if isOtherUser {
				user, _ := s.User(targetUser)
				title = fmt.Sprintf("Tracked Accounts for %s", user.Username)
			} else {
				title = "Your Tracked Accounts"
			}

			if len(accounts) == 0 {
				embed := &discordgo.MessageEmbed{
					Title:       title,
					Description: "No accounts tracked",
					Color:       0x5865F2,
				}
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Embeds: []*discordgo.MessageEmbed{embed},
						Flags:  discordgo.MessageFlagsEphemeral,
					},
				})
				return
			}

			var rsnList []string
			for _, acc := range accounts {
				status := "✅"
				if !acc.IsActive {
					status = "❌"
				}
				rsnList = append(rsnList, fmt.Sprintf("%s `%s`", status, acc.RSN))
			}

			embed := &discordgo.MessageEmbed{
				Title:       title,
				Description: strings.Join(rsnList, "\n"),
				Color:       0x5865F2,
			}

			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Embeds: []*discordgo.MessageEmbed{embed},
					Flags:  discordgo.MessageFlagsEphemeral,
				},
			})
		},
	}
}
