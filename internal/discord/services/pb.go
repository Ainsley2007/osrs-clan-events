package services

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"osrs-events/internal/database"

	"github.com/bwmarrin/discordgo"
)

const (
	PBApproveEmoji = "✅"
	PBRejectEmoji  = "❌"
)

var pbTimePattern = regexp.MustCompile(`^(?:(\d+):)?([0-5]?\d):([0-5]\d)\.(\d{2})$`)

type PBService struct {
	store   PBStore
	session *discordgo.Session
	logger  Logger
}

func NewPBService(store PBStore, session *discordgo.Session, logger Logger) *PBService {
	return &PBService{
		store:   store,
		session: session,
		logger:  logger,
	}
}

type PBSubmissionInput struct {
	GuildID       string
	CategorySlug  string
	DiscordUserID string
	DisplayName   string
	TimeText      *string
	ProofURL      string
}

type PBModerationResult struct {
	Submission           *database.PBSubmission
	Category             *database.PBCategory
	ImprovedPersonalBest bool
}

func ParsePBTimeStrict(raw string) (int64, string, error) {
	matches := pbTimePattern.FindStringSubmatch(strings.TrimSpace(raw))
	if len(matches) != 5 {
		return 0, "", fmt.Errorf("time must be `MM:SS.xx` or `H:MM:SS.xx`")
	}

	hours := 0
	var err error
	if matches[1] != "" {
		hours, err = strconv.Atoi(matches[1])
		if err != nil {
			return 0, "", fmt.Errorf("invalid hours in time")
		}
	}
	minutes, err := strconv.Atoi(matches[2])
	if err != nil {
		return 0, "", fmt.Errorf("invalid minutes in time")
	}
	seconds, err := strconv.Atoi(matches[3])
	if err != nil {
		return 0, "", fmt.Errorf("invalid seconds in time")
	}
	hundredths, err := strconv.Atoi(matches[4])
	if err != nil {
		return 0, "", fmt.Errorf("invalid hundredths in time")
	}

	totalCentiseconds := int64(hours*360000 + minutes*6000 + seconds*100 + hundredths)

	if hours > 0 {
		return totalCentiseconds, fmt.Sprintf("%d:%02d:%02d.%02d", hours, minutes, seconds, hundredths), nil
	}
	return totalCentiseconds, fmt.Sprintf("%02d:%02d.%02d", minutes, seconds, hundredths), nil
}

func FormatPBTimeCentiseconds(total int64) string {
	hours := total / 360000
	remainder := total % 360000
	minutes := remainder / 6000
	remainder = remainder % 6000
	seconds := remainder / 100
	hundredths := remainder % 100

	if hours > 0 {
		return fmt.Sprintf("%d:%02d:%02d.%02d", hours, minutes, seconds, hundredths)
	}
	return fmt.Sprintf("%02d:%02d.%02d", minutes, seconds, hundredths)
}

func (s *PBService) SubmitPB(ctx context.Context, input *PBSubmissionInput) (*database.PBSubmission, *database.PBCategory, error) {
	category, err := s.store.GetPBCategoryBySlug(ctx, input.CategorySlug)
	if err != nil {
		return nil, nil, fmt.Errorf("unknown PB category")
	}
	if !category.IsActive {
		return nil, nil, fmt.Errorf("that PB category is not active yet")
	}

	guild, err := s.store.GetGuild(ctx, input.GuildID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load guild settings: %w", err)
	}
	if guild.PbProofsChannelID == "" {
		return nil, nil, fmt.Errorf("PB channels are not initialized yet")
	}

	now := time.Now().UTC()
	submission := &database.PBSubmission{
		GuildID:       input.GuildID,
		CategorySlug:  input.CategorySlug,
		DiscordUserID: input.DiscordUserID,
		DisplayName:   input.DisplayName,
		ProofURL:      input.ProofURL,
		Status:        "pending",
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if input.TimeText != nil && strings.TrimSpace(*input.TimeText) != "" {
		timeCS, normalized, parseErr := ParsePBTimeStrict(*input.TimeText)
		if parseErr != nil {
			return nil, nil, parseErr
		}
		submission.TimeText = &normalized
		submission.TimeCentiseconds = &timeCS
	}

	if err := s.store.CreatePBSubmission(ctx, submission); err != nil {
		return nil, nil, err
	}

	embed := s.buildSubmissionProofEmbed(submission, category)
	msg, err := s.session.ChannelMessageSendEmbed(guild.PbProofsChannelID, embed)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to post proof for review: %w", err)
	}

	if err := s.store.UpdatePBSubmissionProofMessageID(ctx, submission.ID, msg.ID, now); err != nil {
		return nil, nil, fmt.Errorf("failed to save proof message id: %w", err)
	}

	submission.ProofMessageID = &msg.ID

	if err := s.session.MessageReactionAdd(guild.PbProofsChannelID, msg.ID, PBApproveEmoji); err != nil && s.logger != nil {
		s.logger.Printf("failed to add approve reaction for pb submission %d: %v", submission.ID, err)
	}
	if err := s.session.MessageReactionAdd(guild.PbProofsChannelID, msg.ID, PBRejectEmoji); err != nil && s.logger != nil {
		s.logger.Printf("failed to add reject reaction for pb submission %d: %v", submission.ID, err)
	}

	return submission, category, nil
}

func (s *PBService) HandleApproval(ctx context.Context, guildID, proofMessageID, reviewerDiscordID string) (*PBModerationResult, error) {
	submission, err := s.store.GetPendingPBSubmissionByProofMessageID(ctx, guildID, proofMessageID)
	if err != nil {
		return nil, err
	}

	if submission.TimeText == nil || submission.TimeCentiseconds == nil {
		return nil, fmt.Errorf("submission has no valid time and cannot be approved")
	}

	now := time.Now().UTC()
	if err := s.store.ApprovePBSubmission(ctx, submission.ID, reviewerDiscordID, now); err != nil {
		return nil, err
	}

	record := &database.PBRecord{
		GuildID:           submission.GuildID,
		CategorySlug:      submission.CategorySlug,
		DiscordUserID:     submission.DiscordUserID,
		DisplayName:       submission.DisplayName,
		TimeText:          *submission.TimeText,
		TimeCentiseconds:  *submission.TimeCentiseconds,
		ProofSubmissionID: submission.ID,
		ProofURL:          submission.ProofURL,
		UpdatedAt:         now,
		CreatedAt:         now,
	}
	improved, err := s.store.UpsertPBRecordIfBetter(ctx, record)
	if err != nil {
		return nil, fmt.Errorf("failed to update pb record: %w", err)
	}

	category, err := s.store.GetPBCategoryBySlug(ctx, submission.CategorySlug)
	if err != nil {
		return nil, fmt.Errorf("failed to load pb category: %w", err)
	}

	if err := s.RefreshLeaderboard(ctx, guildID, submission.CategorySlug); err != nil {
		return nil, err
	}

	return &PBModerationResult{
		Submission:           submission,
		Category:             category,
		ImprovedPersonalBest: improved,
	}, nil
}

func (s *PBService) HandleRejection(ctx context.Context, guildID, proofMessageID, reviewerDiscordID string) (*database.PBSubmission, error) {
	submission, err := s.store.GetPendingPBSubmissionByProofMessageID(ctx, guildID, proofMessageID)
	if err != nil {
		return nil, err
	}
	if err := s.store.RejectPBSubmission(ctx, submission.ID, reviewerDiscordID, time.Now().UTC()); err != nil {
		return nil, err
	}
	return submission, nil
}

func (s *PBService) RefreshLeaderboard(ctx context.Context, guildID, categorySlug string) error {
	category, err := s.store.GetPBCategoryBySlug(ctx, categorySlug)
	if err != nil {
		return err
	}

	guild, err := s.store.GetGuild(ctx, guildID)
	if err != nil {
		return fmt.Errorf("failed to load guild settings: %w", err)
	}
	if guild.PbLeaderboardChannelID == "" {
		return fmt.Errorf("PB leaderboard channel is not configured")
	}

	records, err := s.store.GetTopPBRecords(ctx, guildID, categorySlug, 3)
	if err != nil {
		return fmt.Errorf("failed to load top pb records: %w", err)
	}

	embed := s.buildLeaderboardEmbed(category, records)
	now := time.Now().UTC()

	state, stateErr := s.store.GetPBLeaderboardMessage(ctx, guildID, categorySlug)
	if stateErr == nil && state != nil {
		if _, err := s.session.ChannelMessageEditEmbed(state.ChannelID, state.MessageID, embed); err == nil {
			state.UpdatedAt = now
			return s.store.UpsertPBLeaderboardMessage(ctx, state)
		}
	}

	msg, err := s.session.ChannelMessageSendEmbed(guild.PbLeaderboardChannelID, embed)
	if err != nil {
		return fmt.Errorf("failed to send pb leaderboard embed: %w", err)
	}
	return s.store.UpsertPBLeaderboardMessage(ctx, &database.PBLeaderboardMessage{
		GuildID:      guildID,
		CategorySlug: categorySlug,
		ChannelID:    guild.PbLeaderboardChannelID,
		MessageID:    msg.ID,
		UpdatedAt:    now,
	})
}

func (s *PBService) buildSubmissionProofEmbed(submission *database.PBSubmission, category *database.PBCategory) *discordgo.MessageEmbed {
	timeDisplay := "Not provided"
	if submission.TimeText != nil {
		timeDisplay = *submission.TimeText
	}

	return &discordgo.MessageEmbed{
		Title: "PB Submission Review",
		Description: fmt.Sprintf(
			"**Category:** %s\n**Player:** %s\n**Time:** %s\n**Submission ID:** `%d`\n\nReact with %s to accept or %s to reject.",
			category.DisplayName,
			submission.DisplayName,
			timeDisplay,
			submission.ID,
			PBApproveEmoji,
			PBRejectEmoji,
		),
		Color:     0xF97316,
		Timestamp: submission.CreatedAt.Format(time.RFC3339),
		Image: &discordgo.MessageEmbedImage{
			URL: submission.ProofURL,
		},
	}
}

func (s *PBService) buildLeaderboardEmbed(category *database.PBCategory, records []*database.PBRecord) *discordgo.MessageEmbed {
	now := time.Now().UTC()

	sort.Slice(records, func(i, j int) bool {
		if records[i].TimeCentiseconds != records[j].TimeCentiseconds {
			return records[i].TimeCentiseconds < records[j].TimeCentiseconds
		}
		return records[i].UpdatedAt.Before(records[j].UpdatedAt)
	})

	var description strings.Builder
	if len(records) == 0 {
		description.WriteString("No approved PBs yet.\n\nSubmit your proof with `/submit-pb` to get on the board.")
	} else {
		description.WriteString("**Top 3**\n\n")
		for i, record := range records {
			rank := fmt.Sprintf("%d.", i+1)
			switch i {
			case 0:
				rank = "🥇"
			case 1:
				rank = "🥈"
			case 2:
				rank = "🥉"
			}
			description.WriteString(fmt.Sprintf("%s - %s - [Proof](%s) - %s\n", rank, record.TimeText, record.ProofURL, record.DisplayName))
		}
	}

	return &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("🏆 %s PB Leaderboard", category.DisplayName),
		Description: description.String(),
		Color:       0xF97316,
		Timestamp:   now.Format(time.RFC3339),
		Footer: &discordgo.MessageEmbedFooter{
			Text: "Last updated",
		},
		Image: &discordgo.MessageEmbedImage{
			URL: category.EmbedImageURL,
		},
	}
}

func (s *PBService) MarkProofSubmissionAccepted(channelID string, result *PBModerationResult, reviewerDiscordID string, reviewedAt time.Time) error {
	if result == nil || result.Submission == nil || result.Category == nil {
		return nil
	}
	if result.Submission.ProofMessageID == nil {
		return nil
	}

	messageID := *result.Submission.ProofMessageID
	if err := s.session.MessageReactionsRemoveAll(channelID, messageID); err != nil && s.logger != nil {
		s.logger.Printf("failed to clear reactions for accepted pb submission %d: %v", result.Submission.ID, err)
	}

	timeDisplay := "Not provided"
	if result.Submission.TimeText != nil {
		timeDisplay = *result.Submission.TimeText
	}

	reviewerName := reviewerDiscordID
	if reviewer, err := s.session.User(reviewerDiscordID); err == nil && reviewer != nil && reviewer.Username != "" {
		reviewerName = reviewer.Username
	}

	improvementText := "Accepted (existing PB was already faster)."
	if result.ImprovedPersonalBest {
		improvementText = "Accepted and applied as fastest PB."
	}

	embed := &discordgo.MessageEmbed{
		Title: "PB Submission Reviewed",
		Description: fmt.Sprintf(
			"**Status:** Reviewed (Accepted)\n**Result:** %s\n\n**Category:** %s\n**Player:** %s\n**Time:** %s\n**Submission ID:** `%d`\n**Reviewed by:** %s",
			improvementText,
			result.Category.DisplayName,
			result.Submission.DisplayName,
			timeDisplay,
			result.Submission.ID,
			reviewerName,
		),
		Color:     0x22C55E,
		Timestamp: reviewedAt.UTC().Format(time.RFC3339),
		Footer: &discordgo.MessageEmbedFooter{
			Text: "Review finalized",
		},
		Image: &discordgo.MessageEmbedImage{
			URL: result.Submission.ProofURL,
		},
	}

	if _, err := s.session.ChannelMessageEditEmbed(channelID, messageID, embed); err != nil {
		return fmt.Errorf("failed to update accepted proof message: %w", err)
	}

	return nil
}
