package database

import (
	"context"
	"time"
)

type Guild struct {
	GuildID              string
	LogChannelID         string
	BotwCategoryID       string
	BotwChannelID        string
	BotwOverallChannelID string
	BotwMsgID            string
	BotwOverallMsgID     string
	SotwCategoryID       string
	SotwChannelID        string
	SotwOverallChannelID string
	SotwMsgID            string
	SotwOverallMsgID     string
	DonationChannelID    string
	DonationMsgID        string
	IntervalDay          string
	IntervalTime         string
}

type Participant struct {
	DiscordUserID   string
	GuildID         string
	TotalPointsBotw int
	TotalPointsSotw int
}

type Account struct {
	ID            int64
	RSN           string
	DiscordUserID string
	ErrorCount    int
	IsActive      bool
}

type Event struct {
	ID            int64
	GuildID       string
	Type          string // 'botw' or 'sotw'
	WeekNumber    int
	MetricJsonID  string // e.g. 'botw_vorkath'
	BossesToTrack string // JSON array of boss names for BOTW (e.g. ["Dagannoth Prime", "Dagannoth Rex", "Dagannoth Supreme"])
	StartTime     time.Time
	EndTime       time.Time
	IsActive      bool
	PointsPerKC   float64
	PointsPerXP   float64
	ThresholdKC   int
	XPThreshold   int
}

type Snapshot struct {
	ID           int64
	EventID      int64
	AccountID    int64
	StartValue   int64
	CurrentValue int64
}

type SnapshotWithAccount struct {
	Snapshot *Snapshot
	Account  *Account
}

type ParticipantPointUpdate struct {
	DiscordUserID string
	GuildID       string
	BotwPoints    int
	SotwPoints    int
}

type Donation struct {
	ID            int64
	GuildID       string
	DiscordUserID string
	Amount        int64
	CreatedAt     time.Time
	CreatedBy     string
}

type DonationSpending struct {
	ID          int64
	GuildID     string
	Amount      int64
	Description string
	CreatedAt   time.Time
	CreatedBy   string
}

type Store interface {
	// Guilds
	SaveGuild(ctx context.Context, guild *Guild) error
	GetGuild(ctx context.Context, guildID string) (*Guild, error)
	DeleteGuild(ctx context.Context, guildID string) error

	// Participants
	SaveParticipant(ctx context.Context, p *Participant) error
	GetParticipant(ctx context.Context, discordUserID, guildID string) (*Participant, error)
	GetParticipantsByGuild(ctx context.Context, guildID string) ([]*Participant, error)
	DeleteParticipant(ctx context.Context, discordUserID, guildID string) error

	// Accounts
	SaveAccount(ctx context.Context, acc *Account) error
	GetAccount(ctx context.Context, id int64) (*Account, error)
	GetAccountsByDiscordID(ctx context.Context, discordUserID string) ([]*Account, error)
	CountActiveAccountsByDiscordID(ctx context.Context, discordUserID string) (int, error)
	GetAccountsByGuild(ctx context.Context, guildID string) ([]*Account, error)
	GetAccountByRSN(ctx context.Context, rsn, discordUserID string) (*Account, error)
	GetActiveAccounts(ctx context.Context) ([]*Account, error)
	DeleteAccount(ctx context.Context, id int64) error
	UpdateAccountRSN(ctx context.Context, id int64, newRSN string) error

	// Events
	SaveEvent(ctx context.Context, event *Event) error
	GetEvent(ctx context.Context, id int64) (*Event, error)
	GetActiveEvent(ctx context.Context, guildID string, eventType string) (*Event, error)
	GetActiveEvents(ctx context.Context, guildID string, eventType string) ([]*Event, error)
	GetAllEventsByGuildAndType(ctx context.Context, guildID string, eventType string) ([]*Event, error)
	CreateEvent(ctx context.Context, event *Event) error
	GetPendingStartEvents(ctx context.Context) ([]*Event, error)
	GetAllActiveEvents(ctx context.Context) ([]*Event, error)
	GetExpiringEvents(ctx context.Context) ([]*Event, error)
	GetStaleEvents(ctx context.Context) ([]*Event, error)
	DeactivateEvent(ctx context.Context, eventID int64) error

	// Snapshots
	SaveSnapshot(ctx context.Context, snap *Snapshot) error
	CreateSnapshot(ctx context.Context, snap *Snapshot) error
	GetSnapshot(ctx context.Context, eventID, accountID int64) (*Snapshot, error)
	GetSnapshotsByEvent(ctx context.Context, eventID int64) ([]*Snapshot, error)
	UpdateSnapshotCurrentValue(ctx context.Context, snapshotID int64, currentValue int64) error
	GetSnapshotsWithAccounts(ctx context.Context, eventID int64) ([]*SnapshotWithAccount, error)

	// Points
	UpdateParticipantPoints(ctx context.Context, updates []*ParticipantPointUpdate) error

	// Donations
	SaveDonation(ctx context.Context, donation *Donation) error
	GetDonationsByGuild(ctx context.Context, guildID string) ([]*Donation, error)
	GetTotalDonatedByUser(ctx context.Context, guildID, discordUserID string) (int64, error)
	SaveDonationSpending(ctx context.Context, spending *DonationSpending) error
	GetTotalSpent(ctx context.Context, guildID string) (int64, error)
	UpdateDonationChannel(ctx context.Context, guildID, channelID string) error
	UpdateDonationMessageID(ctx context.Context, guildID, messageID string) error

	Close() error
}
