package discord

import (
	"context"
	"fmt"

	"github.com/bwmarrin/discordgo"
)

func (b *Bot) removeCommand() Command {
	return Command{
		Definition: &discordgo.ApplicationCommand{
			Name:        "remove",
			Description: "Remove an OSRS account from tracking",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "rsn",
					Description: "The RuneScape username to remove",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionUser,
					Name:        "user",
					Description: "The user to remove the account from (admin only)",
					Required:    false,
				},
			},
		},
		Handler: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			ctx := context.Background()
			data := i.ApplicationCommandData()

			if i.GuildID == "" {
				respondError(s, i.Interaction, fmt.Errorf("this command can only be used in a server"))
				return
			}

			var rsn string
			for _, opt := range data.Options {
				if opt.Name == "rsn" {
					rsn = opt.StringValue()
					break
				}
			}

			targetUser, isOtherUser, err := getTargetUser(s, i, "user")
			if err != nil {
				respondError(s, i.Interaction, err)
				return
			}

			if err := b.AccountService.RemoveAccount(ctx, targetUser, rsn); err != nil {
				respondError(s, i.Interaction, err)
				return
			}

			var message string
			if isOtherUser {
				user, _ := s.User(targetUser)
				message = fmt.Sprintf("✅ Removed RSN `%s` from <@%s>", rsn, user.ID)
			} else {
				message = fmt.Sprintf("✅ Removed RSN: `%s`", rsn)
			}

			respondSuccess(s, i.Interaction, message)
		},
	}
}
