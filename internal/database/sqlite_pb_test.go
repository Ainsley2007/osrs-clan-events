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
		GuildID:                "g1",
		CategorySlug:           "inferno",
		DiscordUserID:          "u1",
		DisplayName:            "PlayerOne",
		LeaderboardDisplayName: "PlayerOne",
		TimeText:               &timeText,
		TimeCentiseconds:       &timeCS,
		ProofURL:               "https://example.com/proof.png",
		Status:                 pbSubmissionStatusPending,
		CreatedAt:              now,
		UpdatedAt:              now,
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

	if err := store.ApprovePBSubmission(ctx, sub.ID, "admin-1", now); err != ErrPBSubmissionNotPending {
		t.Fatalf("expected ErrPBSubmissionNotPending on second approve, got %v", err)
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
		GuildID:                "g1",
		CategorySlug:           "inferno",
		DiscordUserID:          "u1",
		DisplayName:            "PlayerOne",
		LeaderboardDisplayName: "PlayerOne",
		TimeText:               &baseTimeText,
		TimeCentiseconds:       &baseTimeCS,
		ProofURL:               "https://example.com/proof-base.png",
		Status:                 pbSubmissionStatusAccepted,
		CreatedAt:              now,
		UpdatedAt:              now,
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

	records, err := store.GetPBRecordsByCategory(ctx, "g1", "inferno")
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

func TestGetActivePBCategories_GroupedOrder(t *testing.T) {
	ctx := context.Background()
	store, err := NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	categories, err := store.GetActivePBCategories(ctx)
	if err != nil {
		t.Fatalf("GetActivePBCategories returned error: %v", err)
	}
	if len(categories) != 26 {
		t.Fatalf("expected 26 seeded categories, got %d", len(categories))
	}

	wantOrder := []string{
		"inferno", "fortis_colosseum", "fight_caves",
		"duke_sucellus", "duke_sucellus_awakened",
		"the_leviathan", "the_leviathan_awakened",
		"vardorvis", "vardorvis_awakened",
		"the_whisperer", "the_whisperer_awakened",
		"corrupted_gauntlet", "demonic_brutus", "doom_of_mokhaiotl",
		"gauntlet", "phosanis_nightmare", "yama",
		"alchemical_hydra", "araxxor", "grotesque_guardians",
		"phantom_muspah", "vorkath", "zulrah",
		"toa_expert_300_solo", "toa_expert_500_solo", "toa_expert_400_4man",
	}
	for i, slug := range wantOrder {
		if categories[i].Slug != slug {
			t.Fatalf("category order mismatch at index %d: got %s want %s", i, categories[i].Slug, slug)
		}
	}

	minigameGroup := categories[:3]
	for i, category := range minigameGroup {
		if category.GroupName != "Minigames" {
			t.Fatalf("expected Minigames group for %s, got %s", category.Slug, category.GroupName)
		}
		if category.GroupOrder != 1 {
			t.Fatalf("expected group_order 1 for %s, got %d", category.Slug, category.GroupOrder)
		}
		if category.DisplayOrder != i+1 {
			t.Fatalf("display order mismatch for %s: got %d want %d", category.Slug, category.DisplayOrder, i+1)
		}
	}

	dt2Group := categories[3:11]
	for i, category := range dt2Group {
		if category.GroupName != "DT2 Bosses" {
			t.Fatalf("expected DT2 Bosses group for %s, got %s", category.Slug, category.GroupName)
		}
		if category.GroupOrder != 2 {
			t.Fatalf("expected group_order 2 for %s, got %d", category.Slug, category.GroupOrder)
		}
		if category.DisplayOrder != i+1 {
			t.Fatalf("display order mismatch for %s: got %d want %d", category.Slug, category.DisplayOrder, i+1)
		}
	}

	if categories[3].EmbedImageURL != categories[4].EmbedImageURL {
		t.Fatalf("expected awakened duke to reuse normal duke thumbnail")
	}

	soloDuoGroup := categories[11:17]
	if len(soloDuoGroup) != 6 {
		t.Fatalf("expected 6 solo & duo categories, got %d", len(soloDuoGroup))
	}
	for i, category := range soloDuoGroup {
		if category.GroupName != "Solo & Duo Bosses (A-Z)" {
			t.Fatalf("expected Solo & Duo Bosses (A-Z) group for %s, got %s", category.Slug, category.GroupName)
		}
		if category.GroupOrder != 3 {
			t.Fatalf("expected group_order 3 for %s, got %d", category.Slug, category.GroupOrder)
		}
		if category.DisplayOrder != i+1 {
			t.Fatalf("display order mismatch for %s: got %d want %d", category.Slug, category.DisplayOrder, i+1)
		}
	}

	slayerGroup := categories[17:23]
	if len(slayerGroup) != 6 {
		t.Fatalf("expected 6 slayer boss categories, got %d", len(slayerGroup))
	}
	for i, category := range slayerGroup {
		if category.GroupName != "Slayer Bosses (A-Z)" {
			t.Fatalf("expected Slayer Bosses (A-Z) group for %s, got %s", category.Slug, category.GroupName)
		}
		if category.GroupOrder != 4 {
			t.Fatalf("expected group_order 4 for %s, got %d", category.Slug, category.GroupOrder)
		}
		if category.DisplayOrder != i+1 {
			t.Fatalf("display order mismatch for %s: got %d want %d", category.Slug, category.DisplayOrder, i+1)
		}
	}

	toaGroup := categories[23:]
	if len(toaGroup) != 3 {
		t.Fatalf("expected 3 Tombs of Amascut categories, got %d", len(toaGroup))
	}
	for i, category := range toaGroup {
		if category.GroupName != "Tombs of Amascut" {
			t.Fatalf("expected Tombs of Amascut group for %s, got %s", category.Slug, category.GroupName)
		}
		if category.GroupOrder != 5 {
			t.Fatalf("expected group_order 5 for %s, got %d", category.Slug, category.GroupOrder)
		}
		if category.DisplayOrder != i+1 {
			t.Fatalf("display order mismatch for %s: got %d want %d", category.Slug, category.DisplayOrder, i+1)
		}
	}
}

func TestPBGroupBundleMessageStateLifecycle(t *testing.T) {
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
	state := &PBLeaderboardMessage{
		GuildID:   "g1",
		GroupName: "Minigames",
		ChannelID: "ch-1",
		MessageID: "msg-1",
		UpdatedAt: now,
	}
	if err := store.UpsertPBGroupBundleMessage(ctx, state); err != nil {
		t.Fatalf("upsert group state: %v", err)
	}

	loaded, err := store.GetPBGroupBundleMessage(ctx, "g1", "Minigames")
	if err != nil {
		t.Fatalf("get group state: %v", err)
	}
	if loaded.MessageID != "msg-1" {
		t.Fatalf("message id mismatch: got %s", loaded.MessageID)
	}

	loaded.MessageID = "msg-2"
	loaded.UpdatedAt = now.Add(time.Minute)
	if err := store.UpsertPBGroupBundleMessage(ctx, loaded); err != nil {
		t.Fatalf("update group state: %v", err)
	}

	all, err := store.ListPBGroupBundleMessagesByGuild(ctx, "g1")
	if err != nil {
		t.Fatalf("list group states: %v", err)
	}
	if len(all) != 1 || all[0].MessageID != "msg-2" {
		t.Fatalf("expected one updated state with msg-2, got %+v", all)
	}
}

func TestGetPBRecordsByCategory_ReturnsAllRecordsSorted(t *testing.T) {
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
	submissions := []*PBSubmission{
		{
			GuildID: "g1", CategorySlug: "inferno", DiscordUserID: "u1",
			DisplayName: "Alice", LeaderboardDisplayName: "Alice",
			TimeText: strPtr("01:00.00"), TimeCentiseconds: int64Ptr(6000), ProofURL: "https://a",
			Status: pbSubmissionStatusAccepted, CreatedAt: now, UpdatedAt: now,
		},
		{
			GuildID: "g1", CategorySlug: "inferno", DiscordUserID: "u2",
			DisplayName: "Bob", LeaderboardDisplayName: "Bob",
			TimeText: strPtr("01:00.00"), TimeCentiseconds: int64Ptr(6000), ProofURL: "https://b",
			Status: pbSubmissionStatusAccepted, CreatedAt: now, UpdatedAt: now.Add(time.Minute),
		},
		{
			GuildID: "g1", CategorySlug: "inferno", DiscordUserID: "u3",
			DisplayName: "Charlie", LeaderboardDisplayName: "Charlie",
			TimeText: strPtr("01:01.00"), TimeCentiseconds: int64Ptr(6100), ProofURL: "https://c",
			Status: pbSubmissionStatusAccepted, CreatedAt: now, UpdatedAt: now.Add(2 * time.Minute),
		},
	}
	for _, submission := range submissions {
		if err := store.CreatePBSubmission(ctx, submission); err != nil {
			t.Fatalf("create submission for %s: %v", submission.DisplayName, err)
		}
	}

	records := []*PBRecord{
		{
			GuildID: "g1", CategorySlug: "inferno", DiscordUserID: "u1", DisplayName: "Alice",
			TimeText: "01:00.00", TimeCentiseconds: 6000, ProofSubmissionID: submissions[0].ID, ProofURL: "https://a",
			CreatedAt: now, UpdatedAt: now,
		},
		{
			GuildID: "g1", CategorySlug: "inferno", DiscordUserID: "u2", DisplayName: "Bob",
			TimeText: "01:00.00", TimeCentiseconds: 6000, ProofSubmissionID: submissions[1].ID, ProofURL: "https://b",
			CreatedAt: now, UpdatedAt: now.Add(time.Minute),
		},
		{
			GuildID: "g1", CategorySlug: "inferno", DiscordUserID: "u3", DisplayName: "Charlie",
			TimeText: "01:01.00", TimeCentiseconds: 6100, ProofSubmissionID: submissions[2].ID, ProofURL: "https://c",
			CreatedAt: now, UpdatedAt: now.Add(2 * time.Minute),
		},
	}
	for _, record := range records {
		if _, err := store.UpsertPBRecordIfBetter(ctx, record); err != nil {
			t.Fatalf("upsert record for %s: %v", record.DisplayName, err)
		}
	}

	got, err := store.GetPBRecordsByCategory(ctx, "g1", "inferno")
	if err != nil {
		t.Fatalf("GetPBRecordsByCategory: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 records, got %d", len(got))
	}
	if got[0].DisplayName != "Alice" || got[1].DisplayName != "Bob" || got[2].DisplayName != "Charlie" {
		t.Fatalf("unexpected sort order: %+v", got)
	}
}

func strPtr(value string) *string {
	return &value
}

func int64Ptr(value int64) *int64 {
	return &value
}
