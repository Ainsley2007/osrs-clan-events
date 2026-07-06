package discord

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"osrs-events/internal/scheduler"

	"github.com/bwmarrin/discordgo"
)

// SessionNotifier implements scheduler.Notifier using a Discord session.
type SessionNotifier struct {
	session *discordgo.Session
}

func NewSessionNotifier(session *discordgo.Session) *SessionNotifier {
	return &SessionNotifier{session: session}
}

func (n *SessionNotifier) SendRolloverCompleteLog(channelID string, completed, new []scheduler.RolloverEvent, unresolvedRSNs []string) {
	sendRolloverCompleteLog(n.session, channelID, completed, new, unresolvedRSNs)
}

func (n *SessionNotifier) SendAccountNotFoundDM(discordUserID, guildID, rsn string) error {
	return sendAccountNotFoundDM(n.session, discordUserID, guildID, rsn)
}

func sendAccountNotFoundDM(s *discordgo.Session, discordUserID, guildID, rsn string) error {
	if s == nil {
		return nil
	}
	if discordUserID == "" || rsn == "" {
		return nil
	}

	dmChannel, err := s.UserChannelCreate(discordUserID)
	if err != nil {
		return err
	}

	embed := &discordgo.MessageEmbed{
		Title: "Action Required: OSRS Account Not Found",
		Description: fmt.Sprintf(
			"We couldn't fetch stats for **%s** in one of your competitions.\n\nThis usually means the account was renamed or removed from Hiscores.\nPlease use `/rename` in your server to update this account name.\n\nServer ID: `%s`",
			rsn,
			guildID,
		),
		Color:     0xFF9900,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	_, err = s.ChannelMessageSendEmbed(dmChannel.ID, embed)
	return err
}

func sendRolloverCompleteLog(s *discordgo.Session, channelID string, completedEvents, newEvents []scheduler.RolloverEvent, unresolvedRSNs []string) {
	if s == nil || channelID == "" {
		return
	}

	var description strings.Builder

	if len(completedEvents) > 0 {
		description.WriteString("**Completed Competitions:**\n")
		for _, event := range completedEvents {
			description.WriteString(fmt.Sprintf("- %s: %s (Week %d)\n", eventDisplayName(event.EventType), event.MetricName, event.WeekNumber))
		}
		description.WriteString("\n")
	}

	if len(newEvents) > 0 {
		description.WriteString("**New Competitions Started:**\n")
		for _, event := range newEvents {
			description.WriteString(fmt.Sprintf("- %s: %s (Week %d)\n", eventDisplayName(event.EventType), event.MetricName, event.WeekNumber))
		}
		description.WriteString("\n")
	}

	description.WriteString("Points have been calculated and awarded. Competitions rolled over automatically.")

	if len(unresolvedRSNs) > 0 {
		description.WriteString("\n\n")
		description.WriteString(formatUnresolvedMissingAccounts(unresolvedRSNs))
	}

	embed := &discordgo.MessageEmbed{
		Title:       "🔄 Weekly Rollover Complete",
		Description: description.String(),
		Color:       0x00AA00,
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
	}

	s.ChannelMessageSendEmbed(channelID, embed)
}

func formatUnresolvedMissingAccounts(rsns []string) string {
	unique := make(map[string]struct{}, len(rsns))
	for _, rsn := range rsns {
		unique[rsn] = struct{}{}
	}
	uniqueRSNs := make([]string, 0, len(unique))
	for rsn := range unique {
		uniqueRSNs = append(uniqueRSNs, rsn)
	}
	sort.Strings(uniqueRSNs)

	rsnPreview := strings.Join(uniqueRSNs, ", ")
	if len(rsnPreview) > 900 {
		rsnPreview = rsnPreview[:900] + "..."
	}

	return fmt.Sprintf(
		"**Unresolved Missing Accounts:** %d\n\n%s",
		len(uniqueRSNs),
		rsnPreview,
	)
}

func eventDisplayName(eventType string) string {
	if eventType == "botw" {
		return "Boss of the Week"
	}
	return "Skill of the Week"
}
