package discord

import (
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

func SendCompetitionStartedLog(s *discordgo.Session, channelID string, botwBoss, sotwSkill string, botwWeek, sotwWeek int, startedBy string) {
	if channelID == "" {
		return
	}

	embed := &discordgo.MessageEmbed{
		Title:       "✅ Competitions Started",
		Description: fmt.Sprintf("**BOTW:** %s (Week %d)\n**SOTW:** %s (Week %d)\n**Duration:** 7 days\n\nStarted by: <@%s>",
			botwBoss, botwWeek, sotwSkill, sotwWeek, startedBy),
		Color:     0x00AA00,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	s.ChannelMessageSendEmbed(channelID, embed)
}

func SendCompetitionStoppedLog(s *discordgo.Session, channelID string, stoppedEvents []string, botwPoints, sotwPoints int, stoppedBy string) {
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
		Timestamp:   time.Now().Format(time.RFC3339),
	}

	s.ChannelMessageSendEmbed(channelID, embed)
}

func SendEventCompletedLog(s *discordgo.Session, channelID string, eventType, metricName string, weekNumber int) {
	if channelID == "" {
		return
	}

	var eventDisplayName string
	if eventType == "botw" {
		eventDisplayName = "Boss of the Week"
	} else {
		eventDisplayName = "Skill of the Week"
	}

	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("✅ %s Completed", eventDisplayName),
		Description: fmt.Sprintf("**%s:** %s\n**Week:** %d\n**Status:** Points calculated, rolling over to next competition",
			getMetricLabel(eventType), metricName, weekNumber),
		Color:     0x00AA00,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	s.ChannelMessageSendEmbed(channelID, embed)
}

func SendNewCompetitionStartedLog(s *discordgo.Session, channelID string, eventType, metricName string, weekNumber int) {
	if channelID == "" {
		return
	}

	var eventDisplayName string
	if eventType == "botw" {
		eventDisplayName = "Boss of the Week"
	} else {
		eventDisplayName = "Skill of the Week"
	}

	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("🎯 New %s Started", eventDisplayName),
		Description: fmt.Sprintf("**%s:** %s\n**Week:** %d\n**Status:** Competition started automatically",
			getMetricLabel(eventType), metricName, weekNumber),
		Color:     0x00AA00,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	s.ChannelMessageSendEmbed(channelID, embed)
}

func SendAccountNotFoundLog(s *discordgo.Session, channelID string, rsn string) {
	if channelID == "" {
		return
	}

	embed := &discordgo.MessageEmbed{
		Title:       "⚠️ Account Not Found",
		Description: fmt.Sprintf("Failed to fetch stats for account **%s**.\nThe account may have been renamed or deleted from the OSRS Hiscores.", rsn),
		Color:       0xFF9900,
		Timestamp:   time.Now().Format(time.RFC3339),
	}

	s.ChannelMessageSendEmbed(channelID, embed)
}

type RolloverResult struct {
	EventType  string
	MetricName string
	WeekNumber int
}

func SendRolloverCompleteLog(s *discordgo.Session, channelID string, completedEvents []RolloverResult, newEvents []RolloverResult) {
	if channelID == "" {
		return
	}

	var description strings.Builder

	if len(completedEvents) > 0 {
		description.WriteString("**Completed Competitions:**\n")
		for _, event := range completedEvents {
			description.WriteString(fmt.Sprintf("- %s: %s (Week %d)\n", getEventDisplayName(event.EventType), event.MetricName, event.WeekNumber))
		}
		description.WriteString("\n")
	}

	if len(newEvents) > 0 {
		description.WriteString("**New Competitions Started:**\n")
		for _, event := range newEvents {
			description.WriteString(fmt.Sprintf("- %s: %s (Week %d)\n", getEventDisplayName(event.EventType), event.MetricName, event.WeekNumber))
		}
		description.WriteString("\n")
	}

	description.WriteString("Points have been calculated and awarded. Competitions rolled over automatically.")

	embed := &discordgo.MessageEmbed{
		Title:       "🔄 Weekly Rollover Complete",
		Description: description.String(),
		Color:       0x00AA00,
		Timestamp:   time.Now().Format(time.RFC3339),
	}

	s.ChannelMessageSendEmbed(channelID, embed)
}

func getEventDisplayName(eventType string) string {
	if eventType == "botw" {
		return "Boss of the Week"
	}
	return "Skill of the Week"
}

func getMetricLabel(eventType string) string {
	if eventType == "botw" {
		return "Boss"
	}
	return "Skill"
}
