package discord

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"osrs-events/internal/discord/services"

	"github.com/bwmarrin/discordgo"
)

const maxQueueMetricAutocompleteChoices = 25

func (b *Bot) queueEventCommand() Command {
	return Command{
		Definition: &discordgo.ApplicationCommand{
			Name:                     "queue-event",
			Description:              "Queue a boss or skill for an upcoming competition week",
			DefaultMemberPermissions: ptr(int64(discordgo.PermissionAdministrator)),
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "add",
					Description: "Append a metric to the queue",
					Options: []*discordgo.ApplicationCommandOption{
						typeChoiceOption(),
						metricOption(true),
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "list",
					Description: "Show queued metrics",
					Options:     []*discordgo.ApplicationCommandOption{typeChoiceOption()},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "remove",
					Description: "Remove a metric from the queue by position",
					Options: []*discordgo.ApplicationCommandOption{
						typeChoiceOption(),
						{
							Type:        discordgo.ApplicationCommandOptionInteger,
							Name:        "position",
							Description: "1-based position in the queue",
							Required:    true,
							MinValue:    ptr(1.0),
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "clear",
					Description: "Remove all queued metrics for a type",
					Options:     []*discordgo.ApplicationCommandOption{typeChoiceOption()},
				},
			},
		},
		Handler: b.handleQueueEvent,
	}
}

func typeChoiceOption() *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionString,
		Name:        "type",
		Description: "BOTW or SOTW",
		Required:    true,
		Choices: []*discordgo.ApplicationCommandOptionChoice{
			{Name: "BOTW", Value: "botw"},
			{Name: "SOTW", Value: "sotw"},
		},
	}
}

func metricOption(required bool) *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type:         discordgo.ApplicationCommandOptionString,
		Name:         "metric",
		Description:  "Boss or skill name",
		Required:     required,
		Autocomplete: true,
	}
}

func (b *Bot) handleQueueEvent(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.GuildID == "" {
		respondError(s, i.Interaction, errors.New("this command can only be used in a server"))
		return
	}
	if !hasAdminPermission(s, i.GuildID, i.Member.User.ID) {
		respondError(s, i.Interaction, errors.New("you must be an administrator to use this command"))
		return
	}

	sub := i.ApplicationCommandData().Options[0]
	ctx, cancel := cmdContext()
	defer cancel()

	switch sub.Name {
	case "add":
		eventType := sub.Options[0].StringValue()
		metric := strings.TrimSpace(sub.Options[1].StringValue())
		if metric == "" {
			respondError(s, i.Interaction, errors.New("metric is required"))
			return
		}
		var canonical string
		var err error
		if eventType == "botw" {
			canonical, err = b.EventService.AddBotwQueue(ctx, i.GuildID, metric)
		} else {
			canonical, err = b.EventService.AddSotwQueue(ctx, i.GuildID, metric)
		}
		if err != nil {
			if errors.Is(err, services.ErrUnknownBoss) || errors.Is(err, services.ErrUnknownSkill) {
				respondError(s, i.Interaction, fmt.Errorf("unknown metric %q for %s", metric, strings.ToUpper(eventType)))
				return
			}
			respondError(s, i.Interaction, err)
			return
		}
		queue, _ := b.listQueueForType(ctx, i.GuildID, eventType)
		position := len(queue)
		label := queueTypeLabel(eventType)
		respondSuccess(s, i.Interaction, fmt.Sprintf("✅ Queued **%s** for %s (position %d).", canonical, label, position))
		b.logAction(ctx, i.GuildID, fmt.Sprintf("📋 <@%s> queued **%s** for %s (position %d).", i.Member.User.ID, canonical, label, position))

	case "list":
		eventType := sub.Options[0].StringValue()
		queue, err := b.listQueueForType(ctx, i.GuildID, eventType)
		if err != nil {
			respondError(s, i.Interaction, err)
			return
		}
		label := queueTypeLabel(eventType)
		if len(queue) == 0 {
			respondSuccess(s, i.Interaction, fmt.Sprintf("📋 %s queue is empty.", label))
			return
		}
		var lines strings.Builder
		fmt.Fprintf(&lines, "📋 **%s queue:**\n", label)
		for idx, name := range queue {
			fmt.Fprintf(&lines, "%d. %s\n", idx+1, name)
		}
		respondSuccess(s, i.Interaction, strings.TrimSpace(lines.String()))

	case "remove":
		eventType := sub.Options[0].StringValue()
		position := int(sub.Options[1].IntValue())
		var removed string
		var err error
		if eventType == "botw" {
			removed, err = b.EventService.RemoveBotwQueueAt(ctx, i.GuildID, position)
		} else {
			removed, err = b.EventService.RemoveSotwQueueAt(ctx, i.GuildID, position)
		}
		if err != nil {
			respondError(s, i.Interaction, err)
			return
		}
		label := queueTypeLabel(eventType)
		respondSuccess(s, i.Interaction, fmt.Sprintf("✅ Removed **%s** from %s queue (was position %d).", removed, label, position))
		b.logAction(ctx, i.GuildID, fmt.Sprintf("📋 <@%s> removed **%s** from %s queue (position %d).", i.Member.User.ID, removed, label, position))

	case "clear":
		eventType := sub.Options[0].StringValue()
		var n int
		var err error
		if eventType == "botw" {
			n, err = b.EventService.ClearBotwQueue(ctx, i.GuildID)
		} else {
			n, err = b.EventService.ClearSotwQueue(ctx, i.GuildID)
		}
		if err != nil {
			respondError(s, i.Interaction, err)
			return
		}
		label := queueTypeLabel(eventType)
		respondSuccess(s, i.Interaction, fmt.Sprintf("✅ Cleared %d entr%s from %s queue.", n, pluralY(n), label))
		b.logAction(ctx, i.GuildID, fmt.Sprintf("📋 <@%s> cleared %d entr%s from %s queue.", i.Member.User.ID, n, pluralY(n), label))

	default:
		respondError(s, i.Interaction, fmt.Errorf("unknown subcommand %q", sub.Name))
	}
}

func (b *Bot) listQueueForType(ctx context.Context, guildID, eventType string) ([]string, error) {
	if eventType == "botw" {
		return b.EventService.ListBotwQueue(ctx, guildID)
	}
	return b.EventService.ListSotwQueue(ctx, guildID)
}

func queueTypeLabel(eventType string) string {
	if eventType == "botw" {
		return "BOTW"
	}
	return "SOTW"
}

func pluralY(n int) string {
	if n == 1 {
		return "y"
	}
	return "ies"
}

func (b *Bot) handleQueueEventAutocomplete(s *discordgo.Session, i *discordgo.InteractionCreate) {
	ctx, cancel := cmdContext()
	defer cancel()

	sub := i.ApplicationCommandData().Options[0]
	eventType := sub.Options[0].StringValue()
	query := ""
	for _, opt := range sub.Options {
		if opt.Name == "metric" && opt.Focused {
			query = opt.StringValue()
			break
		}
	}

	var names []string
	var err error
	if eventType == "botw" {
		names, err = b.EventService.BossNamesFromConfig(ctx)
	} else {
		names, err = b.EventService.SkillNamesFromConfig(ctx)
	}
	if err != nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionApplicationCommandAutocompleteResult,
			Data: &discordgo.InteractionResponseData{Choices: []*discordgo.ApplicationCommandOptionChoice{}},
		})
		return
	}

	choices := queueMetricAutocompleteChoices(names, query)
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{Choices: choices},
	})
}

func queueMetricAutocompleteChoices(names []string, query string) []*discordgo.ApplicationCommandOptionChoice {
	query = strings.ToLower(strings.TrimSpace(query))
	slices.Sort(names)

	var matches []string
	for _, name := range names {
		if query == "" || strings.HasPrefix(strings.ToLower(name), query) {
			matches = append(matches, name)
		}
	}

	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0, min(len(matches), maxQueueMetricAutocompleteChoices))
	for _, name := range matches {
		if len(choices) >= maxQueueMetricAutocompleteChoices {
			break
		}
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{Name: name, Value: name})
	}
	return choices
}
