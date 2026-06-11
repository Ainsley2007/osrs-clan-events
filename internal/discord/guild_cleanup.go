package discord

import (
	"context"
	"log"
	"time"

	"github.com/bwmarrin/discordgo"
)

func (b *Bot) cleanupRemovedGuilds(ready *discordgo.Ready) {
	log.Printf("Startup guild cleanup: waiting for guild cache (%d guild(s) in ready payload)", len(ready.Guilds))
	waitForGuildCache(b.Session, ready)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	sessionGuilds := make(map[string]struct{}, len(b.Session.State.Guilds))
	for _, g := range b.Session.State.Guilds {
		sessionGuilds[g.ID] = struct{}{}
	}

	dbGuildIDs, err := b.Store.ListGuildIDs(ctx)
	if err != nil {
		log.Printf("Startup guild cleanup: failed to list DB guilds: %v", err)
		return
	}

	log.Printf("Startup guild cleanup: comparing %d DB guild(s) against %d session guild(s)", len(dbGuildIDs), len(sessionGuilds))

	purgedRemoved := 0
	for _, guildID := range dbGuildIDs {
		if _, ok := sessionGuilds[guildID]; ok {
			continue
		}
		log.Printf("Startup guild cleanup: purging guild %s (bot no longer in server)", guildID)
		if err := b.Store.DeleteGuild(ctx, guildID); err != nil {
			log.Printf("Startup guild cleanup: failed to purge guild %s: %v", guildID, err)
			continue
		}
		purgedRemoved++
	}

	purgedOrphans := 0
	if n, err := b.Store.PurgeOrphanedEvents(ctx); err != nil {
		log.Printf("Startup guild cleanup: failed to purge orphaned events: %v", err)
	} else {
		purgedOrphans = n
	}

	if purgedRemoved == 0 && purgedOrphans == 0 {
		log.Printf("Startup guild cleanup: complete — no stale data found")
	} else {
		log.Printf("Startup guild cleanup: complete — purged %d removed guild(s), %d orphan guild(s)", purgedRemoved, purgedOrphans)
	}
}

func waitForGuildCache(session *discordgo.Session, ready *discordgo.Ready) {
	expected := len(ready.Guilds)
	if expected == 0 {
		return
	}

	deadline := time.Now().Add(30 * time.Second)
	lastCount := -1
	stableSince := time.Now()

	for time.Now().Before(deadline) {
		count := len(session.State.Guilds)
		if count >= expected {
			if count == lastCount && time.Since(stableSince) >= 2*time.Second {
				log.Printf("Startup guild cleanup: guild cache ready (%d guild(s))", count)
				return
			}
			if count != lastCount {
				stableSince = time.Now()
			}
		}
		lastCount = count
		time.Sleep(250 * time.Millisecond)
	}

	log.Printf("Startup guild cleanup: guild cache wait timed out (have %d guild(s), ready listed %d)", len(session.State.Guilds), expected)
}
