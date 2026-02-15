package discord

import (
	"context"
	"log"
	"time"

	"github.com/bwmarrin/discordgo"
)

func (b *Bot) guildDelete(s *discordgo.Session, gd *discordgo.GuildDelete) {
	// GuildDelete is triggered when:
	// 1. Bot is kicked/removed from guild
	// 2. Bot loses access to guild
	// 3. Guild is deleted
	
	log.Printf("🚪 Bot removed from guild: %s (ID: %s)", gd.Guild.Name, gd.Guild.ID)
	
	// Clean up guild data (will cascade delete all events, participants, snapshots, donations)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	guild, err := b.Store.GetGuild(ctx, gd.Guild.ID)
	if err != nil {
		log.Printf("Guild %s not found in database, nothing to clean up", gd.Guild.ID)
		return
	}
	
	// Delete guild from database - foreign key constraints will cascade delete:
	// - All events for this guild
	// - All participants
	// - All snapshots (via events)
	// - All donations
	if err := b.Store.DeleteGuild(ctx, gd.Guild.ID); err != nil {
		log.Printf("❌ Failed to clean up guild %s: %v", gd.Guild.ID, err)
		return
	}
	
	log.Printf("✅ Successfully cleaned up all data for guild %s", gd.Guild.ID)
	_ = guild // Avoid unused variable warning
}
