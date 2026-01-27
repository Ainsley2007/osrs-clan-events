package discord

import (
	"context"
	"time"

	"github.com/bwmarrin/discordgo"
)

func (b *Bot) logAction(ctx context.Context, guildID, message string) {
	guild, err := b.GuildService.GetOrCreateGuild(ctx, guildID)
	if err != nil || guild.LogChannelID == "" {
		return
	}

	embed := &discordgo.MessageEmbed{
		Description: message,
		Color:       0x5865F2,
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
	}

	b.Session.ChannelMessageSendEmbed(guild.LogChannelID, embed)
}

func logAction(s *discordgo.Session, channelID, title, description string, color int) {
	if channelID == "" {
		return
	}

	embed := &discordgo.MessageEmbed{
		Title:       title,
		Description: description,
		Color:       color,
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
	}

	s.ChannelMessageSendEmbed(channelID, embed)
}
