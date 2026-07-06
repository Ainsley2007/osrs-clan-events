package discord

import (
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"

	"osrs-events/internal/discord/services"
)

const (
	maxStatsEmbedsPerMessage = 10
	maxStatsEmbedDescLen     = 4000 // below Discord's 4096 limit
)

func buildStatsEmbeds(botwStats, sotwStats []services.EventStats) []*discordgo.MessageEmbed {
	var embeds []*discordgo.MessageEmbed
	embeds = append(embeds, buildTypeStatsEmbedChunks("🗡️ BOTW progress", botwStats, "KC", 0xFF6B6B)...)

	remaining := maxStatsEmbedsPerMessage - len(embeds)
	if remaining > 0 {
		sotwEmbeds := buildTypeStatsEmbedChunks("⚔️ SOTW progress", sotwStats, "XP", 0x4ECDC4)
		if len(sotwEmbeds) > remaining {
			sotwEmbeds = sotwEmbeds[:remaining]
			markStatsTruncated(sotwEmbeds[len(sotwEmbeds)-1])
		}
		embeds = append(embeds, sotwEmbeds...)
	} else if len(sotwStats) > 0 {
		markStatsTruncated(embeds[len(embeds)-1])
	}

	if len(embeds) > maxStatsEmbedsPerMessage {
		embeds = embeds[:maxStatsEmbedsPerMessage]
		markStatsTruncated(embeds[len(embeds)-1])
	}

	return embeds
}

func buildTypeStatsEmbedChunks(title string, stats []services.EventStats, unit string, color int) []*discordgo.MessageEmbed {
	if len(stats) == 0 {
		return nil
	}

	var descriptions []string
	var b strings.Builder
	b.WriteString("```\n")

	for i, eventStat := range stats {
		section := formatWeekSection(eventStat, unit)
		if b.Len() > len("```\n") && b.Len()+len(section)+len("```") > maxStatsEmbedDescLen {
			b.WriteString("```")
			descriptions = append(descriptions, b.String())
			b.Reset()
			b.WriteString("```\n")
		}
		b.WriteString(section)
		if i < len(stats)-1 && !strings.HasSuffix(section, "\n") {
			b.WriteString("\n")
		}
	}

	if b.Len() > len("```\n") {
		b.WriteString("```")
		descriptions = append(descriptions, b.String())
	}

	embeds := make([]*discordgo.MessageEmbed, 0, len(descriptions))
	for i, desc := range descriptions {
		embedTitle := title
		if len(descriptions) > 1 {
			embedTitle = fmt.Sprintf("%s (%d/%d)", title, i+1, len(descriptions))
		}
		embeds = append(embeds, &discordgo.MessageEmbed{
			Title:       embedTitle,
			Description: desc,
			Color:       color,
		})
	}
	return embeds
}

func formatWeekSection(eventStat services.EventStats, unit string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Week %d — %s  (%d pts)\n", eventStat.WeekNumber, eventStat.MetricName, eventStat.Points)
	for _, acc := range eventStat.AccountStats {
		fmt.Fprintf(&b, "  %-18s %10s %s\n", acc.RSN, formatNumber(acc.Gain), unit)
	}
	b.WriteString("\n")
	return b.String()
}

func markStatsTruncated(embed *discordgo.MessageEmbed) {
	if embed == nil {
		return
	}
	const note = "…more history not shown (embed limit)"
	if embed.Footer == nil {
		embed.Footer = &discordgo.MessageEmbedFooter{Text: note}
		return
	}
	embed.Footer.Text = embed.Footer.Text + " · " + note
}
