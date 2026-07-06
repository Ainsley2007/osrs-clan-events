package discord

import (
	"context"
	"log"
	"time"

	"github.com/bwmarrin/discordgo"
)

func (b *Bot) guildDelete(s *discordgo.Session, gd *discordgo.GuildDelete) {
	// Unavailable=true is a Discord outage; the guild may return — do not delete data.
	if gd.Unavailable {
		log.Printf("Guild %s became unavailable (outage) — skipping data cleanup", gd.Guild.ID)
		return
	}

	log.Printf("Bot removed from guild: %s (ID: %s)", gd.Guild.Name, gd.Guild.ID)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := b.Store.DeleteGuild(ctx, gd.Guild.ID); err != nil {
		log.Printf("Failed to clean up guild %s: %v", gd.Guild.ID, err)
		return
	}

	log.Printf("Successfully cleaned up all data for guild %s", gd.Guild.ID)
}
