package scheduler

// RolloverEvent summarizes a competition that completed or started during rollover.
type RolloverEvent struct {
	EventType  string
	MetricName string
	WeekNumber int
}

// Notifier sends user-facing Discord messages triggered by background jobs.
type Notifier interface {
	SendRolloverCompleteLog(channelID string, completed, new []RolloverEvent, unresolvedRSNs []string)
	SendAccountNotFoundDM(discordUserID, guildID, rsn string) error
}

type noopNotifier struct{}

func (noopNotifier) SendRolloverCompleteLog(string, []RolloverEvent, []RolloverEvent, []string) {}
func (noopNotifier) SendAccountNotFoundDM(string, string, string) error                        { return nil }
