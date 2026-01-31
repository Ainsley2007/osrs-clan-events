package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"osrs-events/internal/database"
)

type AccountService struct {
	store              AccountStore
	snapshotService    *SnapshotService
	leaderboardService *LeaderboardService
	logger             Logger
}

func NewAccountService(store AccountStore, snapshotService *SnapshotService, leaderboardService *LeaderboardService, logger Logger) *AccountService {
	return &AccountService{
		store:              store,
		snapshotService:    snapshotService,
		leaderboardService: leaderboardService,
		logger:             logger,
	}
}

// AddAccountResult indicates whether the account was new or the user joined this guild with an existing account.
type AddAccountResult struct {
	JoinedGuild bool // true if account already existed and user was added as participant to this guild
}

func (s *AccountService) AddAccount(ctx context.Context, discordUserID, guildID, rsn string) (*AddAccountResult, error) {
	rsn = strings.TrimSpace(rsn)
	if rsn == "" {
		return nil, fmt.Errorf("RSN cannot be empty")
	}

	existing, err := s.store.GetAccountByRSN(ctx, rsn, discordUserID)
	if err == nil && existing != nil {
		// Account already exists; ensure participant in this guild and create snapshots for this guild's events
		_, err := s.store.GetParticipant(ctx, discordUserID, guildID)
		if err != nil {
			participant := &database.Participant{
				DiscordUserID:   discordUserID,
				GuildID:         guildID,
				TotalPointsBotw: 0,
				TotalPointsSotw: 0,
			}
			if err := s.store.SaveParticipant(ctx, participant); err != nil {
				return nil, fmt.Errorf("failed to create participant for this guild: %w", err)
			}
		}
		if err := s.createSnapshotsForActiveEvents(ctx, existing, guildID); err != nil {
			s.logger.Printf("Warning: failed to create snapshots for existing account %s in guild: %v", rsn, err)
		}
		return &AddAccountResult{JoinedGuild: true}, nil
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
			return nil, fmt.Errorf("failed to create participant: %w", err)
		}
	}

	account := &database.Account{
		RSN:           rsn,
		DiscordUserID: discordUserID,
		ErrorCount:    0,
		IsActive:      true,
	}

	if err := s.store.SaveAccount(ctx, account); err != nil {
		return nil, fmt.Errorf("failed to add account: %w", err)
	}

	if err := s.createSnapshotsForActiveEvents(ctx, account, guildID); err != nil {
		s.logger.Printf("Warning: failed to create snapshots for new account %s: %v", rsn, err)
	}

	return &AddAccountResult{JoinedGuild: false}, nil
}

func (s *AccountService) createSnapshotsForActiveEvents(ctx context.Context, account *database.Account, guildID string) error {
	// Get all active events for this guild
	botwEvent, err := s.store.GetActiveEvent(ctx, guildID, "botw")
	if err != nil {
		if !isNoActiveEventErr(err) {
			return fmt.Errorf("failed to get active BOTW event: %w", err)
		}
		botwEvent = nil
	}

	sotwEvent, err := s.store.GetActiveEvent(ctx, guildID, "sotw")
	if err != nil {
		if !isNoActiveEventErr(err) {
			return fmt.Errorf("failed to get active SOTW event: %w", err)
		}
		sotwEvent = nil
	}

	// Use UTC for all time comparisons
	now := time.Now().UTC()
	var eventsToCreate []*database.Event

	// Collect events that are active and have started
	if botwEvent != nil {
		eventStartUTC := botwEvent.StartTime.UTC()
		if eventStartUTC.Before(now) || eventStartUTC.Equal(now) {
			eventsToCreate = append(eventsToCreate, botwEvent)
			s.logger.Printf("Creating BOTW snapshot for account %s in Week %d event", account.RSN, botwEvent.WeekNumber)
		} else {
			s.logger.Printf("BOTW Week %d event has not started yet (starts at %v UTC, now is %v UTC)", botwEvent.WeekNumber, eventStartUTC, now)
		}
	} else {
		s.logger.Printf("No active BOTW event found for guild %s", guildID)
	}

	if sotwEvent != nil {
		eventStartUTC := sotwEvent.StartTime.UTC()
		if eventStartUTC.Before(now) || eventStartUTC.Equal(now) {
			eventsToCreate = append(eventsToCreate, sotwEvent)
			s.logger.Printf("Creating SOTW snapshot for account %s in Week %d event", account.RSN, sotwEvent.WeekNumber)
		} else {
			s.logger.Printf("SOTW Week %d event has not started yet (starts at %v UTC, now is %v UTC)", sotwEvent.WeekNumber, eventStartUTC, now)
		}
	} else {
		s.logger.Printf("No active SOTW event found for guild %s", guildID)
	}

	if len(eventsToCreate) == 0 {
		s.logger.Printf("No snapshots created for account %s - no active events that have started", account.RSN)
		return nil
	}

	// Create snapshots for all events efficiently (fetch stats once)
	if err := s.snapshotService.CreateSnapshotsForAccount(ctx, eventsToCreate, account); err != nil {
		return fmt.Errorf("failed to create snapshots: %w", err)
	}

	return nil
}

func (s *AccountService) RemoveAccount(ctx context.Context, discordUserID, guildID, rsn string) error {
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

	// Update leaderboards after account removal
	s.leaderboardService.RefreshLeaderboards(ctx, guildID)

	return nil
}

func (s *AccountService) RenameAccount(ctx context.Context, discordUserID, guildID, oldRSN, newRSN string) error {
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

	// Update account RSN in memory for snapshot creation
	account.RSN = newRSN

	// Create snapshots for active events with the new RSN
	// This will update existing snapshots or create new ones if needed
	if err := s.createSnapshotsForActiveEvents(ctx, account, guildID); err != nil {
		// Log error but don't fail rename - snapshots can be updated later
		s.logger.Printf("Warning: failed to create snapshots after rename: %v", err)
	}

	s.leaderboardService.RefreshLeaderboards(ctx, guildID)
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

	// Update leaderboards after exiting competition
	s.leaderboardService.RefreshLeaderboards(ctx, guildID)

	return nil
}

func isNoActiveEventErr(err error) bool {
	return err != nil && err.Error() == "no active event found"
}
