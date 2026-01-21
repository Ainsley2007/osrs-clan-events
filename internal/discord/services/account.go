package services

import (
	"context"
	"fmt"
	"strings"

	"osrs-events/internal/database"
)

type AccountService struct {
	store database.Store
}

func NewAccountService(store database.Store) *AccountService {
	return &AccountService{store: store}
}

func (s *AccountService) AddAccount(ctx context.Context, discordUserID, guildID, rsn string) error {
	rsn = strings.TrimSpace(rsn)
	if rsn == "" {
		return fmt.Errorf("RSN cannot be empty")
	}

	existing, err := s.store.GetAccountByRSN(ctx, rsn, discordUserID)
	if err == nil && existing != nil {
		return fmt.Errorf("you already have an account with RSN: %s", rsn)
	}

	participant, err := s.store.GetParticipant(ctx, discordUserID, guildID)
	if err != nil {
		participant = &database.Participant{
			DiscordUserID:   discordUserID,
			GuildID:         guildID,
			TotalPointsBotw: 0,
			TotalPointsSotw: 0,
		}
		if err := s.store.SaveParticipant(ctx, participant); err != nil {
			return fmt.Errorf("failed to create participant: %w", err)
		}
	}

	account := &database.Account{
		RSN:           rsn,
		DiscordUserID: discordUserID,
		ErrorCount:    0,
		IsActive:      true,
	}

	if err := s.store.SaveAccount(ctx, account); err != nil {
		return fmt.Errorf("failed to add account: %w", err)
	}

	return nil
}

func (s *AccountService) RemoveAccount(ctx context.Context, discordUserID, rsn string) error {
	rsn = strings.TrimSpace(rsn)
	if rsn == "" {
		return fmt.Errorf("RSN cannot be empty")
	}

	account, err := s.store.GetAccountByRSN(ctx, rsn, discordUserID)
	if err != nil {
		return fmt.Errorf("account not found")
	}

	if err := s.store.DeleteAccount(ctx, account.ID); err != nil {
		return fmt.Errorf("failed to remove account: %w", err)
	}

	return nil
}

func (s *AccountService) RenameAccount(ctx context.Context, discordUserID, oldRSN, newRSN string) error {
	oldRSN = strings.TrimSpace(oldRSN)
	newRSN = strings.TrimSpace(newRSN)

	if oldRSN == "" || newRSN == "" {
		return fmt.Errorf("RSN cannot be empty")
	}

	account, err := s.store.GetAccountByRSN(ctx, oldRSN, discordUserID)
	if err != nil {
		return fmt.Errorf("account not found")
	}

	if err := s.store.UpdateAccountRSN(ctx, account.ID, newRSN); err != nil {
		return fmt.Errorf("failed to rename account: %w", err)
	}

	return nil
}

func (s *AccountService) GetTrackedAccounts(ctx context.Context, discordUserID string) ([]*database.Account, error) {
	accounts, err := s.store.GetAccountsByDiscordID(ctx, discordUserID)
	if err != nil {
		return nil, fmt.Errorf("failed to get accounts: %w", err)
	}
	return accounts, nil
}

func (s *AccountService) ExitCompetition(ctx context.Context, discordUserID, guildID string) error {
	accounts, err := s.store.GetAccountsByDiscordID(ctx, discordUserID)
	if err != nil {
		return fmt.Errorf("failed to get accounts: %w", err)
	}

	for _, account := range accounts {
		if err := s.store.DeleteAccount(ctx, account.ID); err != nil {
			return fmt.Errorf("failed to delete account: %w", err)
		}
	}

	if err := s.store.DeleteParticipant(ctx, discordUserID, guildID); err != nil {
		return fmt.Errorf("failed to exit competition: %w", err)
	}

	return nil
}
