package discord

import (
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
)

func (b *Bot) removeCommand() Command {
	return Command{
		Definition: &discordgo.ApplicationCommand{
			Name:        "remove",
			Description: "Remove an OSRS account from tracking",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:         discordgo.ApplicationCommandOptionString,
					Name:         "rsn",
					Description:  "The RuneScape username to remove",
					Required:     true,
					Autocomplete: true,
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

			if err := respondDeferred(s, i.Interaction, "⏳ Removing account..."); err != nil {
				log.Printf("Failed to defer remove interaction: %v", err)
				return
			}

		go func() {
			ctx, cancel := cmdContext()
			defer cancel()
			if err := b.AccountService.RemoveAccount(ctx, targetUser, i.GuildID, rsn); err != nil {
					editDeferredContent(s, i.Interaction, fmt.Sprintf("❌ Failed to remove account: %v", err))
					return
				}

				if isOtherUser {
					user, _ := s.User(targetUser)
					b.logAction(ctx, i.GuildID, fmt.Sprintf("➖ <@%s> removed account `%s` from <@%s>", i.Member.User.ID, rsn, user.ID))
					editDeferredContent(s, i.Interaction, fmt.Sprintf("✅ Removed RSN `%s` from <@%s>.", rsn, targetUser))
				} else {
					b.logAction(ctx, i.GuildID, fmt.Sprintf("➖ <@%s> removed account `%s`", targetUser, rsn))
					editDeferredContent(s, i.Interaction, fmt.Sprintf("✅ Removed RSN `%s`.", rsn))
				}
			}()
		},
	}
}
