package services

import (
	"context"
	"fmt"

	"osrs-events/internal/database"
)

type GuildService struct {
	store database.Store
}

func NewGuildService(store database.Store) *GuildService {
	return &GuildService{store: store}
}

func (s *GuildService) GetOrCreateGuild(ctx context.Context, guildID string) (*database.Guild, error) {
	guild, err := s.store.GetGuild(ctx, guildID)
	if err != nil {
		guild = &database.Guild{
			GuildID:      guildID,
			IntervalDay:  "Sunday",
			IntervalTime: "22:00",
		}
		if err := s.store.SaveGuild(ctx, guild); err != nil {
			return nil, fmt.Errorf("failed to create guild: %w", err)
		}
		return guild, nil
	}
	return guild, nil
}

func (s *GuildService) UpdateLogChannel(ctx context.Context, guildID, channelID string) error {
	guild, err := s.GetOrCreateGuild(ctx, guildID)
	if err != nil {
		return err
	}

	guild.LogChannelID = channelID
	return s.store.SaveGuild(ctx, guild)
}
