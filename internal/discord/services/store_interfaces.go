package services

import (
	"context"
	"time"

	"osrs-events/internal/database"
	"osrs-events/internal/firebase"
	"osrs-events/internal/osrs"
)

// AccountStore is the minimal persistence interface for account workflows.
type AccountStore interface {
	SaveAccount(ctx context.Context, acc *database.Account) error
	GetAccountByRSN(ctx context.Context, rsn, discordUserID string) (*database.Account, error)
	GetAccountsByDiscordID(ctx context.Context, discordUserID string) ([]*database.Account, error)
	DeleteAccount(ctx context.Context, id int64) error
	UpdateAccountRSN(ctx context.Context, id int64, newRSN string) error
	GetParticipant(ctx context.Context, discordUserID, guildID string) (*database.Participant, error)
	SaveParticipant(ctx context.Context, p *database.Participant) error
	DeleteParticipant(ctx context.Context, discordUserID, guildID string) error
	GetActiveEvent(ctx context.Context, guildID string, eventType string) (*database.Event, error)
}

// SnapshotStore is the minimal persistence interface for snapshot workflows.
type SnapshotStore interface {
	GetAccountsByGuild(ctx context.Context, guildID string) ([]*database.Account, error)
	GetEvent(ctx context.Context, id int64) (*database.Event, error)
	CreateSnapshot(ctx context.Context, snap *database.Snapshot) error
	GetSnapshot(ctx context.Context, eventID, accountID int64) (*database.Snapshot, error)
	UpdateSnapshotCurrentValue(ctx context.Context, snapshotID int64, currentValue int64) error
	GetSnapshotsByEvent(ctx context.Context, eventID int64) ([]*database.Snapshot, error)
	GetAccount(ctx context.Context, id int64) (*database.Account, error)
	GetSnapshotsWithAccounts(ctx context.Context, eventID int64) ([]*database.SnapshotWithAccount, error)
	UpdateParticipantPoints(ctx context.Context, updates []*database.ParticipantPointUpdate) error
}

// EventStore is the minimal persistence interface for event workflows.
type EventStore interface {
	GetActiveEvent(ctx context.Context, guildID string, eventType string) (*database.Event, error)
	GetActiveEvents(ctx context.Context, guildID string, eventType string) ([]*database.Event, error)
	GetAllEventsByGuildAndType(ctx context.Context, guildID string, eventType string) ([]*database.Event, error)
	CreateEvent(ctx context.Context, event *database.Event) error
	DeactivateEvent(ctx context.Context, eventID int64) error
}

// ParticipantStore is the minimal persistence interface for participant points workflows.
type ParticipantStore interface {
	GetParticipant(ctx context.Context, discordUserID, guildID string) (*database.Participant, error)
	UpdateParticipantPoints(ctx context.Context, updates []*database.ParticipantPointUpdate) error
}

// LeaderboardStore is the minimal persistence interface for leaderboard workflows.
type LeaderboardStore interface {
	GetGuild(ctx context.Context, guildID string) (*database.Guild, error)
	GetActiveEvent(ctx context.Context, guildID string, eventType string) (*database.Event, error)
	GetSnapshotsWithAccounts(ctx context.Context, eventID int64) ([]*database.SnapshotWithAccount, error)
	GetAccountsByGuild(ctx context.Context, guildID string) ([]*database.Account, error)
	CountActiveAccountsByDiscordID(ctx context.Context, discordUserID string) (int, error)
	GetParticipant(ctx context.Context, discordUserID, guildID string) (*database.Participant, error)
	GetParticipantsByGuild(ctx context.Context, guildID string) ([]*database.Participant, error)
	GetTotalGainedByParticipant(ctx context.Context, guildID, eventType string) (map[string]int64, error)
}

// GuildStore is the minimal persistence interface for guild workflows.
type GuildStore interface {
	GetGuild(ctx context.Context, guildID string) (*database.Guild, error)
	SaveGuild(ctx context.Context, guild *database.Guild) error
}

// InitializerStore is the minimal persistence interface for guild initialization.
type InitializerStore interface {
	GetGuild(ctx context.Context, guildID string) (*database.Guild, error)
	SaveGuild(ctx context.Context, guild *database.Guild) error
	GetActiveEvent(ctx context.Context, guildID string, eventType string) (*database.Event, error)
	GetActivePBCategories(ctx context.Context) ([]*database.PBCategory, error)
	GetPBGroupBundleMessage(ctx context.Context, guildID, groupName string) (*database.PBLeaderboardMessage, error)
	UpsertPBGroupBundleMessage(ctx context.Context, message *database.PBLeaderboardMessage) error
	ListPBGroupBundleMessagesByGuild(ctx context.Context, guildID string) ([]*database.PBLeaderboardMessage, error)
	DeletePBGroupBundleMessagesByGuild(ctx context.Context, guildID string) error
	ListLegacyPBLeaderboardMessagesByGuild(ctx context.Context, guildID string) ([]*database.LegacyPBLeaderboardMessage, error)
	DeleteLegacyPBLeaderboardMessagesByGuild(ctx context.Context, guildID string) error
}

// OSRSConfigProvider returns OSRS config (bosses/skills) from e.g. Firebase Remote Config. Selection logic lives in the event service.
type OSRSConfigProvider interface {
	FetchOSRSConfig(ctx context.Context) (*firebase.OSRSConfig, error)
}

// SnapshotManager abstracts snapshot operations used by EventService.
type SnapshotManager interface {
	CreateInitialSnapshotsWithResult(ctx context.Context, eventID int64, guildID, metricName, metricType string) (*InitialSnapshotResult, error)
	CreateInitialSnapshotsForEventsFromStats(ctx context.Context, events []*database.Event, statsByAccountID map[int64]*osrs.PlayerStats) error
	UpdateSnapshotsForEvent(ctx context.Context, event *database.Event) ([]FailedAccountUpdate, error)
	CalculateAndAwardPoints(ctx context.Context, event *database.Event) error
}

// DonationStore is the minimal persistence interface for donation workflows.
type DonationStore interface {
	GetGuild(ctx context.Context, guildID string) (*database.Guild, error)
	SaveDonation(ctx context.Context, donation *database.Donation) error
	GetDonationsByGuild(ctx context.Context, guildID string) ([]*database.Donation, error)
	GetTotalDonatedByUser(ctx context.Context, guildID, discordUserID string) (int64, error)
	SaveDonationSpending(ctx context.Context, spending *database.DonationSpending) error
	GetTotalSpent(ctx context.Context, guildID string) (int64, error)
	UpdateDonationMessageID(ctx context.Context, guildID, messageID string) error
}

type PBStore interface {
	GetGuild(ctx context.Context, guildID string) (*database.Guild, error)
	GetActivePBCategories(ctx context.Context) ([]*database.PBCategory, error)
	GetPBCategoryBySlug(ctx context.Context, slug string) (*database.PBCategory, error)
	CreatePBSubmission(ctx context.Context, submission *database.PBSubmission) error
	UpdatePBSubmissionProofMessageID(ctx context.Context, submissionID int64, messageID string, updatedAt time.Time) error
	GetPendingPBSubmissionByProofMessageID(ctx context.Context, guildID, proofMessageID string) (*database.PBSubmission, error)
	ApprovePBSubmission(ctx context.Context, submissionID int64, reviewerDiscordID string, reviewedAt time.Time) error
	RejectPBSubmission(ctx context.Context, submissionID int64, reviewerDiscordID string, reviewedAt time.Time) error
	GetPBSubmission(ctx context.Context, submissionID int64) (*database.PBSubmission, error)
	UpsertPBRecordIfBetter(ctx context.Context, record *database.PBRecord) (bool, error)
	GetTopPBRecords(ctx context.Context, guildID, categorySlug string, limit int) ([]*database.PBRecord, error)
	GetPBGroupBundleMessage(ctx context.Context, guildID, groupName string) (*database.PBLeaderboardMessage, error)
	UpsertPBGroupBundleMessage(ctx context.Context, message *database.PBLeaderboardMessage) error
	ListPBGroupBundleMessagesByGuild(ctx context.Context, guildID string) ([]*database.PBLeaderboardMessage, error)
	DeletePBGroupBundleMessagesByGuild(ctx context.Context, guildID string) error
	ListLegacyPBLeaderboardMessagesByGuild(ctx context.Context, guildID string) ([]*database.LegacyPBLeaderboardMessage, error)
	DeleteLegacyPBLeaderboardMessagesByGuild(ctx context.Context, guildID string) error
}
