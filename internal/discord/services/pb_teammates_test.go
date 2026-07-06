package services

import (
	"testing"

	"osrs-events/internal/database"
)

func TestFormatLeaderboardDisplayName(t *testing.T) {
	if got := formatLeaderboardDisplayName("Alice", nil); got != "Alice" {
		t.Fatalf("solo submitter: got %q", got)
	}
	if got := formatLeaderboardDisplayName("Alice", []string{"Bob", "Charlie"}); got != "Alice, Bob, Charlie" {
		t.Fatalf("with teammates: got %q", got)
	}
}

func TestParseDiscordMentionIDs(t *testing.T) {
	raw := "Nice run <@123> and <@!456>"
	ids := parseDiscordMentionIDs(raw)
	if len(ids) != 2 || ids[0] != "123" || ids[1] != "456" {
		t.Fatalf("unexpected ids: %#v", ids)
	}
}

func TestParseDiscordMentionIDs_EmptyWhenNoMentions(t *testing.T) {
	if ids := parseDiscordMentionIDs("no mentions here"); len(ids) != 0 {
		t.Fatalf("expected no ids, got %#v", ids)
	}
}

func TestSubmissionLeaderboardDisplayName_Fallback(t *testing.T) {
	sub := &database.PBSubmission{DisplayName: "Alice"}
	if got := submissionLeaderboardDisplayName(sub); got != "Alice" {
		t.Fatalf("fallback: got %q", got)
	}
	sub.LeaderboardDisplayName = "Alice, Bob"
	if got := submissionLeaderboardDisplayName(sub); got != "Alice, Bob" {
		t.Fatalf("leaderboard name: got %q", got)
	}
}
