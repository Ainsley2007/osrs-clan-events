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
	ID           int64
	GuildID      string
	Type         string // 'botw' or 'sotw'
	WeekNumber   int
	MetricJsonID string // e.g. 'botw_vorkath'
	StartTime    time.Time
	EndTime      time.Time
	IsActive     bool
}

type Snapshot struct {
	ID           int64
	EventID      int64
	AccountID    int64
	StartValue   int64
	CurrentValue int64
}

type Store interface {
	// Guilds
	SaveGuild(ctx context.Context, guild *Guild) error
	GetGuild(ctx context.Context, guildID string) (*Guild, error)

	// Participants
	SaveParticipant(ctx context.Context, p *Participant) error
	GetParticipant(ctx context.Context, discordUserID, guildID string) (*Participant, error)

	// Accounts
	SaveAccount(ctx context.Context, acc *Account) error
	GetAccount(ctx context.Context, id int64) (*Account, error)
	GetAccountsByDiscordID(ctx context.Context, discordUserID string) ([]*Account, error)
	GetActiveAccounts(ctx context.Context) ([]*Account, error)

	// Events
	SaveEvent(ctx context.Context, event *Event) error
	GetEvent(ctx context.Context, id int64) (*Event, error)
	GetActiveEvent(ctx context.Context, guildID string, eventType string) (*Event, error)

	// Snapshots
	SaveSnapshot(ctx context.Context, snap *Snapshot) error
	GetSnapshot(ctx context.Context, eventID, accountID int64) (*Snapshot, error)
	GetSnapshotsByEvent(ctx context.Context, eventID int64) ([]*Snapshot, error)

	Close() error
}
