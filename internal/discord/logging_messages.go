package discord

import (
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
)

func sendCompetitionStartedLog(s *discordgo.Session, channelID string, botwBoss, sotwSkill string, botwWeek, sotwWeek int, startedBy string) {
	if channelID == "" {
		return
	}

	embed := &discordgo.MessageEmbed{
		Title: "✅ Competitions Started",
		Description: fmt.Sprintf("**BOTW:** %s (Week %d)\n**SOTW:** %s (Week %d)\n**Duration:** 7 days\n\nStarted by: <@%s>",
			botwBoss, botwWeek, sotwSkill, sotwWeek, startedBy),
		Color:     0x00AA00,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	s.ChannelMessageSendEmbed(channelID, embed)
}

func sendCompetitionStoppedLog(s *discordgo.Session, channelID string, stoppedEvents []string, botwPoints, sotwPoints int, stoppedBy string) {
	if channelID == "" {
		return
	}

	description := "**Stopped Competitions:**\n"
	for _, eventDesc := range stoppedEvents {
		description += eventDesc + "\n"
	}
	description += fmt.Sprintf("\n**Points Awarded:**\n- BOTW: %d participants\n- SOTW: %d participants\n\nStopped by: <@%s>",
		botwPoints, sotwPoints, stoppedBy)

	embed := &discordgo.MessageEmbed{
		Title:       "🛑 Competitions Stopped",
		Description: description,
		Color:       0xFF6600,
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
	}

	s.ChannelMessageSendEmbed(channelID, embed)
}
