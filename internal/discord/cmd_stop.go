package discord

import (
	"errors"
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
)

func (b *Bot) stopCommand() Command {
	return Command{
		Definition: &discordgo.ApplicationCommand{
			Name:                    "stop",
			Description:             "Stop all active competitions and award points",
			DefaultMemberPermissions: ptr(int64(discordgo.PermissionAdministrator)),
		},
		Handler: b.handleStop,
	}
}

func (b *Bot) handleStop(s *discordgo.Session, i *discordgo.InteractionCreate) {
	actor, ok := interactionActor(i)
	if !ok {
		respondError(s, i.Interaction, errors.New("could not resolve command user"))
		return
	}
	if !hasAdminPermission(s, i.GuildID, actor.ID) {
		respondError(s, i.Interaction, errors.New("you must be an administrator to use this command"))
		return
	}

	ctx, cancel := cmdContext()
	defer cancel()

	activeBotwEvents, err := b.eventService.GetActiveEvents(ctx, i.GuildID, "botw")
	if err != nil {
		respondError(s, i.Interaction, fmt.Errorf("failed to get BOTW events: %w", err))
		return
	}

	activeSotwEvents, err := b.eventService.GetActiveEvents(ctx, i.GuildID, "sotw")
	if err != nil {
		respondError(s, i.Interaction, fmt.Errorf("failed to get SOTW events: %w", err))
		return
	}

	if len(activeBotwEvents) == 0 && len(activeSotwEvents) == 0 {
		respondError(s, i.Interaction, errors.New("no active competitions to stop"))
		return
	}

	// Build stopped events for logging (detailed info goes to logging channel only)
	var stoppedEvents []string
	for _, event := range activeBotwEvents {
		stoppedEvents = append(stoppedEvents, fmt.Sprintf("**BOTW:** %s (Week %d)", event.MetricJsonID, event.WeekNumber))
	}
	for _, event := range activeSotwEvents {
		stoppedEvents = append(stoppedEvents, fmt.Sprintf("**SOTW:** %s (Week %d)", event.MetricJsonID, event.WeekNumber))
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "✅ Weekly competitions stopped successfully!",
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})

	// Do heavy work asynchronously (completing events, leaderboard updates, logging)
	go func() {
		ctx, cancel := cmdContext()
		defer cancel()
		var botwPointsAwarded, sotwPointsAwarded int

		for _, event := range activeBotwEvents {
			if err := b.eventService.CompleteEvent(ctx, event); err != nil {
				log.Printf("Failed to complete BOTW event: %v", err)
				continue
			}
			count, _ := b.snapshotService.CountSnapshotsForEvent(ctx, event.ID)
			botwPointsAwarded = count
		}

		for _, event := range activeSotwEvents {
			if err := b.eventService.CompleteEvent(ctx, event); err != nil {
				log.Printf("Failed to complete SOTW event: %v", err)
				continue
			}
			count, _ := b.snapshotService.CountSnapshotsForEvent(ctx, event.ID)
			sotwPointsAwarded = count
		}

		if len(activeBotwEvents) > 0 || len(activeSotwEvents) > 0 {
			b.leaderboardService.RefreshLeaderboards(ctx, i.GuildID)
		}

		stoppedBy := ""
		if actor, ok := interactionActor(i); ok {
			stoppedBy = actor.ID
		}

		guild, err := b.guildService.GetGuild(ctx, i.GuildID)
		if err == nil && guild.LogChannelID != "" {
			sendCompetitionStoppedLog(s, guild.LogChannelID, stoppedEvents, botwPointsAwarded, sotwPointsAwarded, stoppedBy)
		}
	}()
}
