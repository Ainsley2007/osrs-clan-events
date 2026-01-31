package services

import (
	"context"
	"errors"
	"fmt"

	"osrs-events/internal/database"
)

// ErrParticipantNotFound is returned when the user is not a participant in the guild.
var ErrParticipantNotFound = errors.New("participant not found")

type ParticipantService struct {
	store ParticipantStore
}

func NewParticipantService(store ParticipantStore) *ParticipantService {
	return &ParticipantService{store: store}
}

// AddPoints adds (or subtracts) BOTW or SOTW points for a participant. The participant must already exist.
// eventType must be "botw" or "sotw". Returns ErrParticipantNotFound if the user has no participant row for the guild.
func (s *ParticipantService) AddPoints(ctx context.Context, guildID, discordUserID, eventType string, amount int) error {
	if eventType != "botw" && eventType != "sotw" {
		return fmt.Errorf("event type must be botw or sotw, got %q", eventType)
	}

	_, err := s.store.GetParticipant(ctx, discordUserID, guildID)
	if err != nil {
		return ErrParticipantNotFound
	}

	update := &database.ParticipantPointUpdate{
		DiscordUserID: discordUserID,
		GuildID:       guildID,
		BotwPoints:    0,
		SotwPoints:    0,
	}
	if eventType == "botw" {
		update.BotwPoints = amount
	} else {
		update.SotwPoints = amount
	}
	return s.store.UpdateParticipantPoints(ctx, []*database.ParticipantPointUpdate{update})
}
