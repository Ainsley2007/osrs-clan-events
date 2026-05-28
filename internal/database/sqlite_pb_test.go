package database

import (
	"context"
	"testing"
	"time"
)

func TestPBSubmissionModerationLifecycle(t *testing.T) {
	ctx := context.Background()
	store, err := NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	if err := store.SaveGuild(ctx, &Guild{GuildID: "g1"}); err != nil {
		t.Fatalf("save guild: %v", err)
	}

	now := time.Now().UTC()
	timeText := "01:02.34"
	timeCS := int64(6234)
	sub := &PBSubmission{
		GuildID:          "g1",
		CategorySlug:     "inferno",
		DiscordUserID:    "u1",
		DisplayName:      "PlayerOne",
		TimeText:         &timeText,
		TimeCentiseconds: &timeCS,
		ProofURL:         "https://example.com/proof.png",
		Status:           pbSubmissionStatusPending,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := store.CreatePBSubmission(ctx, sub); err != nil {
		t.Fatalf("create submission: %v", err)
	}
	if err := store.UpdatePBSubmissionProofMessageID(ctx, sub.ID, "msg-1", now); err != nil {
		t.Fatalf("update proof message id: %v", err)
	}

	pending, err := store.GetPendingPBSubmissionByProofMessageID(ctx, "g1", "msg-1")
	if err != nil {
		t.Fatalf("expected pending submission: %v", err)
	}
	if pending.ID != sub.ID {
		t.Fatalf("pending submission mismatch: got %d want %d", pending.ID, sub.ID)
	}

	if err := store.ApprovePBSubmission(ctx, sub.ID, "admin-1", now); err != nil {
		t.Fatalf("approve submission: %v", err)
	}
	if _, err := store.GetPendingPBSubmissionByProofMessageID(ctx, "g1", "msg-1"); err == nil {
		t.Fatalf("expected pending lookup to fail after approval")
	}
}

func TestUpsertPBRecordIfBetter(t *testing.T) {
	ctx := context.Background()
	store, err := NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	if err := store.SaveGuild(ctx, &Guild{GuildID: "g1"}); err != nil {
		t.Fatalf("save guild: %v", err)
	}

	now := time.Now().UTC()
	baseTimeText := "01:02.34"
	baseTimeCS := int64(6234)
	sub := &PBSubmission{
		GuildID:          "g1",
		CategorySlug:     "inferno",
		DiscordUserID:    "u1",
		DisplayName:      "PlayerOne",
		TimeText:         &baseTimeText,
		TimeCentiseconds: &baseTimeCS,
		ProofURL:         "https://example.com/proof-base.png",
		Status:           pbSubmissionStatusAccepted,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := store.CreatePBSubmission(ctx, sub); err != nil {
		t.Fatalf("create base submission: %v", err)
	}

	inserted, err := store.UpsertPBRecordIfBetter(ctx, &PBRecord{
		GuildID:           "g1",
		CategorySlug:      "inferno",
		DiscordUserID:     "u1",
		DisplayName:       "PlayerOne",
		TimeText:          baseTimeText,
		TimeCentiseconds:  baseTimeCS,
		ProofSubmissionID: sub.ID,
		ProofURL:          sub.ProofURL,
		CreatedAt:         now,
		UpdatedAt:         now,
	})
	if err != nil {
		t.Fatalf("insert record: %v", err)
	}
	if !inserted {
		t.Fatalf("expected initial insert to return updated=true")
	}

	slowerUpdated, err := store.UpsertPBRecordIfBetter(ctx, &PBRecord{
		GuildID:           "g1",
		CategorySlug:      "inferno",
		DiscordUserID:     "u1",
		DisplayName:       "PlayerOne",
		TimeText:          "01:05.00",
		TimeCentiseconds:  6500,
		ProofSubmissionID: sub.ID,
		ProofURL:          "https://example.com/proof-slower.png",
		CreatedAt:         now,
		UpdatedAt:         now.Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("upsert slower record: %v", err)
	}
	if slowerUpdated {
		t.Fatalf("expected slower time not to update record")
	}

	fasterUpdated, err := store.UpsertPBRecordIfBetter(ctx, &PBRecord{
		GuildID:           "g1",
		CategorySlug:      "inferno",
		DiscordUserID:     "u1",
		DisplayName:       "PlayerOne",
		TimeText:          "01:01.00",
		TimeCentiseconds:  6100,
		ProofSubmissionID: sub.ID,
		ProofURL:          "https://example.com/proof-faster.png",
		CreatedAt:         now,
		UpdatedAt:         now.Add(2 * time.Minute),
	})
	if err != nil {
		t.Fatalf("upsert faster record: %v", err)
	}
	if !fasterUpdated {
		t.Fatalf("expected faster time to update record")
	}

	records, err := store.GetTopPBRecords(ctx, "g1", "inferno", 3)
	if err != nil {
		t.Fatalf("get top records: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected one pb record, got %d", len(records))
	}
	if records[0].TimeCentiseconds != 6100 {
		t.Fatalf("expected best time 6100cs, got %d", records[0].TimeCentiseconds)
	}
}
