package discord

import (
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
)

func (b *Bot) addAccountCommand() Command {
	return Command{
		Definition: &discordgo.ApplicationCommand{
			Name:        "add",
			Description: "Add an OSRS account to track for competitions",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "rsn",
					Description: "The RuneScape username to track",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionUser,
					Name:        "user",
					Description: "The user to add the account for (admin only)",
					Required:    false,
				},
			},
		},
		Handler: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
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

			if err := respondDeferred(s, i.Interaction, "⏳ Adding account..."); err != nil {
				log.Printf("Failed to defer add interaction: %v", err)
				return
			}

		go func() {
			ctx, cancel := cmdContext()
			defer cancel()
			result, err := b.accountService.AddAccount(ctx, targetUser, i.GuildID, rsn)
				if err != nil {
					editDeferredContent(s, i.Interaction, fmt.Sprintf("❌ Failed to add account: %v", err))
					return
				}

				if isOtherUser {
					user, _ := s.User(targetUser)
					b.logAction(ctx, i.GuildID, fmt.Sprintf("➕ <@%s> added account `%s` for <@%s>", i.Member.User.ID, rsn, user.ID))
				} else {
					b.logAction(ctx, i.GuildID, fmt.Sprintf("➕ <@%s> added account `%s`", targetUser, rsn))
				}

				if result.JoinedGuild {
					if isOtherUser {
						editDeferredContent(s, i.Interaction, fmt.Sprintf("✅ Account `%s` was already linked to <@%s>. They're now participating in this server's competitions.", rsn, targetUser))
					} else {
						editDeferredContent(s, i.Interaction, fmt.Sprintf("✅ Account `%s` was already linked to you. You're now participating in this server's competitions.", rsn))
					}
				} else {
					if isOtherUser {
						editDeferredContent(s, i.Interaction, fmt.Sprintf("✅ Added RSN `%s` for <@%s>.", rsn, targetUser))
					} else {
						editDeferredContent(s, i.Interaction, fmt.Sprintf("✅ Added RSN `%s`.", rsn))
					}
				}
			}()
		},
	}
}
