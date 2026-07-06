package discord

import (
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
)

func (b *Bot) renameCommand() Command {
	return Command{
		Definition: &discordgo.ApplicationCommand{
			Name:        "rename",
			Description: "Rename a tracked OSRS account",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:         discordgo.ApplicationCommandOptionString,
					Name:         "current-rsn",
					Description:  "The current RuneScape username",
					Required:     true,
					Autocomplete: true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "new-rsn",
					Description: "The new RuneScape username",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionUser,
					Name:        "user",
					Description: "The user whose account to rename (admin only)",
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

			var currentRSN, newRSN string
			for _, opt := range data.Options {
				switch opt.Name {
				case "current-rsn":
					currentRSN = opt.StringValue()
				case "new-rsn":
					newRSN = opt.StringValue()
				}
			}

			targetUser, isOtherUser, err := getTargetUser(s, i, "user")
			if err != nil {
				respondError(s, i.Interaction, err)
				return
			}

			actorID, _ := interactionActorID(i)

			if err := respondDeferred(s, i.Interaction, "⏳ Renaming account..."); err != nil {
				log.Printf("Failed to defer rename interaction: %v", err)
				return
			}

			goSafe("rename-account", func() {
				ctx, cancel := cmdContext()
				defer cancel()
				if err := b.accountService.RenameAccount(ctx, targetUser, i.GuildID, currentRSN, newRSN); err != nil {
					editDeferredContent(s, i.Interaction, fmt.Sprintf("❌ Failed to rename account: %v", err))
					return
				}

				if isOtherUser {
					user, _ := s.User(targetUser)
					b.logAction(ctx, i.GuildID, fmt.Sprintf("✏️ <@%s> renamed account `%s` → `%s` for <@%s>", actorID, currentRSN, newRSN, user.ID))
					editDeferredContent(s, i.Interaction, fmt.Sprintf("✅ Renamed `%s` to `%s` for <@%s>.", currentRSN, newRSN, targetUser))
				} else {
					b.logAction(ctx, i.GuildID, fmt.Sprintf("✏️ <@%s> renamed account `%s` → `%s`", targetUser, currentRSN, newRSN))
					editDeferredContent(s, i.Interaction, fmt.Sprintf("✅ Renamed `%s` to `%s`.", currentRSN, newRSN))
				}
			})
		},
	}
}
