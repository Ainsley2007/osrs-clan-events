package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"osrs-events/internal/database"
	"osrs-events/internal/proofstorage"
)

func TestParsePBTimeStrict(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantCS     int64
		wantOutput string
		wantErr    bool
	}{
		{
			name:       "minutes format",
			input:      "59:12.34",
			wantCS:     355234,
			wantOutput: "59:12.34",
		},
		{
			name:       "hours format",
			input:      "1:05:07.08",
			wantCS:     390708,
			wantOutput: "1:05:07.08",
		},
		{
			name:    "invalid no hundredths",
			input:   "12:34",
			wantErr: true,
		},
		{
			name:    "invalid range",
			input:   "61:99.99",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCS, gotOutput, err := ParsePBTimeStrict(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotCS != tt.wantCS {
				t.Fatalf("centiseconds mismatch: got %d, want %d", gotCS, tt.wantCS)
			}
			if gotOutput != tt.wantOutput {
				t.Fatalf("normalized output mismatch: got %q, want %q", gotOutput, tt.wantOutput)
			}
		})
	}
}

func TestRankPBLeaderboardPlaceRows_TiedFirstPlaceShowsFourPlayers(t *testing.T) {
	now := time.Now().UTC()
	records := []*database.PBRecord{
		{DisplayName: "Alice", TimeText: "01:00.00", TimeCentiseconds: 6000, UpdatedAt: now},
		{DisplayName: "Bob", TimeText: "01:00.00", TimeCentiseconds: 6000, UpdatedAt: now.Add(time.Minute)},
		{DisplayName: "Charlie", TimeText: "01:01.00", TimeCentiseconds: 6100, UpdatedAt: now.Add(2 * time.Minute)},
		{DisplayName: "Diana", TimeText: "01:02.00", TimeCentiseconds: 6200, UpdatedAt: now.Add(3 * time.Minute)},
	}

	rows := rankPBLeaderboardPlaceRows(records)
	if len(rows) != 4 {
		t.Fatalf("expected 4 visible rows, got %d", len(rows))
	}
	if rows[0].place != 1 || rows[1].place != 1 || rows[2].place != 2 || rows[3].place != 3 {
		t.Fatalf("unexpected place assignment: %+v", rows)
	}
	if rows[0].record.DisplayName != "Alice" || rows[1].record.DisplayName != "Bob" {
		t.Fatalf("expected Alice before Bob within first-place tie")
	}
}

func TestRankPBLeaderboardPlaceRows_AllTiedFirstShowsOnlyGold(t *testing.T) {
	now := time.Now().UTC()
	records := []*database.PBRecord{
		{DisplayName: "Alice", TimeCentiseconds: 6000, UpdatedAt: now},
		{DisplayName: "Bob", TimeCentiseconds: 6000, UpdatedAt: now.Add(time.Minute)},
		{DisplayName: "Charlie", TimeCentiseconds: 6000, UpdatedAt: now.Add(2 * time.Minute)},
	}

	rows := rankPBLeaderboardPlaceRows(records)
	if len(rows) != 3 {
		t.Fatalf("expected 3 visible rows, got %d", len(rows))
	}
	for _, row := range rows {
		if row.place != 1 {
			t.Fatalf("expected all rows to be first place, got place %d", row.place)
		}
	}
}

func TestBuildLeaderboardEmbed_SingleEntryShowsVacantSecondAndThird(t *testing.T) {
	service := &PBService{}
	category := &database.PBCategory{Slug: "vorkath", DisplayName: "Vorkath"}
	now := time.Now().UTC()
	records := []*database.PBRecord{
		{
			DisplayName:      "Alice",
			TimeText:         "01:00.00",
			TimeCentiseconds: 6000,
			ProofURL:         "https://proof-a",
			UpdatedAt:        now,
		},
	}

	embed := service.buildLeaderboardEmbed(category, records)
	desc := embed.Description
	if strings.Contains(desc, "No approved PBs yet.") {
		t.Fatalf("expected filled board, got empty message: %q", desc)
	}
	if !strings.Contains(desc, "🥈 - No record - Vacant") || !strings.Contains(desc, "🥉 - No record - Vacant") {
		t.Fatalf("expected vacant 2nd and 3rd lines, got %q", desc)
	}
	if strings.Count(desc, "🥇") != 1 {
		t.Fatalf("expected one gold row, got %q", desc)
	}
}

func TestBuildLeaderboardEmbed_TwoPlacesShowsVacantThirdOnly(t *testing.T) {
	service := &PBService{}
	category := &database.PBCategory{Slug: "vorkath", DisplayName: "Vorkath"}
	now := time.Now().UTC()
	records := []*database.PBRecord{
		{DisplayName: "Alice", TimeText: "01:00.00", TimeCentiseconds: 6000, ProofURL: "https://a", UpdatedAt: now},
		{DisplayName: "Bob", TimeText: "01:01.00", TimeCentiseconds: 6100, ProofURL: "https://b", UpdatedAt: now},
	}

	desc := service.buildLeaderboardEmbed(category, records).Description
	if !strings.Contains(desc, "🥉 - No record - Vacant") {
		t.Fatalf("expected vacant 3rd line, got %q", desc)
	}
	if strings.Contains(desc, "🥈 - No record - Vacant") {
		t.Fatalf("did not expect vacant 2nd line, got %q", desc)
	}
}

func TestBuildLeaderboardEmbed_EmptyBoardShowsNoVacantLines(t *testing.T) {
	service := &PBService{}
	category := &database.PBCategory{Slug: "vorkath", DisplayName: "Vorkath"}

	desc := service.buildLeaderboardEmbed(category, nil).Description
	if !strings.Contains(desc, "No approved PBs yet.") {
		t.Fatalf("expected empty board message, got %q", desc)
	}
	if strings.Contains(desc, "Vacant") {
		t.Fatalf("empty board should not show vacant lines, got %q", desc)
	}
}

func TestBuildLeaderboardEmbed_TiedFirstPlaceUsesDuplicateGoldMedals(t *testing.T) {
	service := &PBService{}
	category := &database.PBCategory{Slug: "inferno", DisplayName: "The Inferno"}
	now := time.Now().UTC()
	records := []*database.PBRecord{
		{DisplayName: "Alice", TimeText: "01:00.00", TimeCentiseconds: 6000, ProofURL: "https://proof-a", UpdatedAt: now},
		{DisplayName: "Bob", TimeText: "01:00.00", TimeCentiseconds: 6000, ProofURL: "https://proof-b", UpdatedAt: now.Add(time.Minute)},
		{DisplayName: "Charlie", TimeText: "01:01.00", TimeCentiseconds: 6100, ProofURL: "https://proof-c", UpdatedAt: now.Add(2 * time.Minute)},
		{DisplayName: "Diana", TimeText: "01:02.00", TimeCentiseconds: 6200, ProofURL: "https://proof-d", UpdatedAt: now.Add(3 * time.Minute)},
	}

	embed := service.buildLeaderboardEmbed(category, records)
	desc := embed.Description
	if strings.Count(desc, "🥇") != 2 {
		t.Fatalf("expected two gold medals, got description %q", desc)
	}
	if !strings.Contains(desc, "🥈") || !strings.Contains(desc, "🥉") {
		t.Fatalf("expected silver and bronze medals in description: %q", desc)
	}
}

func TestBuildLeaderboardEmbed_ShowsTopThreeFastest(t *testing.T) {
	service := &PBService{}
	category := &database.PBCategory{
		Slug:          "inferno",
		DisplayName:   "Inferno",
		EmbedImageURL: "https://oldschool.runescape.wiki/w/Inferno#/media/File:Inferno_logo.png",
	}

	now := time.Now().UTC()
	records := []*database.PBRecord{
		{DisplayName: "Charlie", TimeText: "01:05.50", TimeCentiseconds: 6550, UpdatedAt: now},
		{DisplayName: "Alpha", TimeText: "01:01.00", TimeCentiseconds: 6100, UpdatedAt: now},
		{DisplayName: "Bravo", TimeText: "01:03.20", TimeCentiseconds: 6320, UpdatedAt: now},
		{DisplayName: "Delta", TimeText: "01:10.00", TimeCentiseconds: 7000, UpdatedAt: now},
	}

	embed := service.buildLeaderboardEmbed(category, records)
	if embed == nil {
		t.Fatalf("embed is nil")
	}
	if embed.Thumbnail == nil || embed.Thumbnail.URL != category.EmbedImageURL {
		t.Fatalf("thumbnail url mismatch: got %+v", embed.Thumbnail)
	}
	if embed.Footer == nil || embed.Footer.Text != "Last updated" {
		t.Fatalf("expected last updated footer")
	}
	if embed.Timestamp == "" {
		t.Fatalf("expected embed timestamp")
	}

	desc := embed.Description
	if strings.Contains(desc, "<@") {
		t.Fatalf("leaderboard should not include mentions: %q", desc)
	}
	for _, name := range []string{"Alpha", "Bravo", "Charlie"} {
		if !strings.Contains(desc, name) {
			t.Fatalf("expected name %q in leaderboard: %q", name, desc)
		}
	}
	if strings.Contains(desc, "Delta") {
		t.Fatalf("expected 4th place player excluded from top-three places: %q", desc)
	}
	if !strings.Contains(desc, "[Proof](") {
		t.Fatalf("expected proof links in leaderboard: %q", desc)
	}
	if !(strings.Index(desc, "Alpha") < strings.Index(desc, "Bravo") &&
		strings.Index(desc, "Bravo") < strings.Index(desc, "Charlie")) {
		t.Fatalf("expected sorted order Alpha -> Bravo -> Charlie, got %q", desc)
	}
	for _, medal := range []string{"🥇", "🥈", "🥉"} {
		if !strings.Contains(desc, medal) {
			t.Fatalf("expected medal rank emoji %s in leaderboard: %q", medal, desc)
		}
	}
}

type mockPBStore struct {
	activeCategories []*database.PBCategory
	topRecordsBySlug map[string][]*database.PBRecord
}

func (m *mockPBStore) GetGuild(context.Context, string) (*database.Guild, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m *mockPBStore) GetActivePBCategories(context.Context) ([]*database.PBCategory, error) {
	return m.activeCategories, nil
}
func (m *mockPBStore) GetPBCategoryBySlug(context.Context, string) (*database.PBCategory, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m *mockPBStore) CreatePBSubmission(context.Context, *database.PBSubmission) error {
	return fmt.Errorf("not implemented")
}
func (m *mockPBStore) UpdatePBSubmissionProofMessageID(context.Context, int64, string, time.Time) error {
	return fmt.Errorf("not implemented")
}
func (m *mockPBStore) GetPendingPBSubmissionByProofMessageID(context.Context, string, string) (*database.PBSubmission, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m *mockPBStore) ApprovePBSubmission(context.Context, int64, string, time.Time) error {
	return fmt.Errorf("not implemented")
}
func (m *mockPBStore) RejectPBSubmission(context.Context, int64, string, time.Time) error {
	return fmt.Errorf("not implemented")
}
func (m *mockPBStore) UpsertPBRecordIfBetter(context.Context, *database.PBRecord) (bool, error) {
	return false, fmt.Errorf("not implemented")
}
func (m *mockPBStore) GetPBRecordByUserAndCategory(context.Context, string, string, string) (*database.PBRecord, error) {
	return nil, database.ErrPBRecordNotFound
}
func (m *mockPBStore) GetPBRecordsByCategory(_ context.Context, _ string, categorySlug string) ([]*database.PBRecord, error) {
	if records, ok := m.topRecordsBySlug[categorySlug]; ok {
		return records, nil
	}
	return nil, nil
}
func (m *mockPBStore) GetPBGroupBundleMessage(context.Context, string, string) (*database.PBLeaderboardMessage, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m *mockPBStore) UpsertPBGroupBundleMessage(context.Context, *database.PBLeaderboardMessage) error {
	return fmt.Errorf("not implemented")
}
func (m *mockPBStore) ListPBGroupBundleMessagesByGuild(context.Context, string) ([]*database.PBLeaderboardMessage, error) {
	return nil, nil
}
func (m *mockPBStore) DeletePBGroupBundleMessagesByGuild(context.Context, string) error {
	return nil
}

type mockModerationGateStore struct {
	mockPBStore
	guild      *database.Guild
	pending    *database.PBSubmission
	pendingErr error
}

func (m *mockModerationGateStore) GetGuild(context.Context, string) (*database.Guild, error) {
	return m.guild, nil
}

func (m *mockModerationGateStore) GetPendingPBSubmissionByProofMessageID(context.Context, string, string) (*database.PBSubmission, error) {
	if m.pendingErr != nil {
		return nil, m.pendingErr
	}
	if m.pending == nil {
		return nil, fmt.Errorf("pb submission not found")
	}
	return m.pending, nil
}

func TestPendingProofSubmissionForReaction_RequiresProofQueueAndPendingRow(t *testing.T) {
	pending := &database.PBSubmission{ID: 1, GuildID: "g1"}
	proofsChannel := "proofs-ch"

	t.Run("wrong channel", func(t *testing.T) {
		service := &PBService{store: &mockModerationGateStore{
			guild:   &database.Guild{GuildID: "g1", PbProofsChannelID: proofsChannel},
			pending: pending,
		}}
		if _, ok := service.PendingProofSubmissionForReaction(context.Background(), "g1", "other-ch", "msg-1"); ok {
			t.Fatal("expected false for non-proof-queue channel")
		}
	})

	t.Run("no pending submission", func(t *testing.T) {
		service := &PBService{store: &mockModerationGateStore{
			guild: &database.Guild{GuildID: "g1", PbProofsChannelID: proofsChannel},
		}}
		if _, ok := service.PendingProofSubmissionForReaction(context.Background(), "g1", proofsChannel, "msg-1"); ok {
			t.Fatal("expected false when no pending submission")
		}
	})

	t.Run("proof queue pending submission", func(t *testing.T) {
		service := &PBService{store: &mockModerationGateStore{
			guild:   &database.Guild{GuildID: "g1", PbProofsChannelID: proofsChannel},
			pending: pending,
		}}
		got, ok := service.PendingProofSubmissionForReaction(context.Background(), "g1", proofsChannel, "msg-1")
		if !ok || got == nil || got.ID != 1 {
			t.Fatalf("expected pending submission, got ok=%v submission=%v", ok, got)
		}
	})
}

func TestAllLeaderboardGroups_SubmissionRulesFirst(t *testing.T) {
	store := &mockPBStore{
		activeCategories: []*database.PBCategory{
			{Slug: "inferno", GroupName: "Minigames", GroupOrder: 1, DisplayOrder: 1},
		},
	}
	service := &PBService{store: store}

	groups, err := service.allLeaderboardGroups(context.Background())
	if err != nil {
		t.Fatalf("allLeaderboardGroups returned error: %v", err)
	}
	if len(groups) != 2 {
		t.Fatalf("expected rules + minigames groups, got %d", len(groups))
	}
	if !groups[0].RulesOnly || groups[0].Name != SubmissionRulesGroupName {
		t.Fatalf("expected submission rules first, got %#v", groups[0])
	}
	if groups[1].Name != "Minigames" {
		t.Fatalf("expected minigames second, got %s", groups[1].Name)
	}
}

func TestGroupActiveCategories_OrdersByGroupAndDisplay(t *testing.T) {
	store := &mockPBStore{
		activeCategories: []*database.PBCategory{
			{Slug: "c3", GroupName: "B", GroupOrder: 2, DisplayOrder: 2},
			{Slug: "c2", GroupName: "A", GroupOrder: 1, DisplayOrder: 2},
			{Slug: "c1", GroupName: "A", GroupOrder: 1, DisplayOrder: 1},
		},
	}
	service := &PBService{store: store}

	groups, err := service.groupActiveCategories(context.Background())
	if err != nil {
		t.Fatalf("groupActiveCategories returned error: %v", err)
	}
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}
	if groups[0].Name != "A" || groups[1].Name != "B" {
		t.Fatalf("group order mismatch: got %s then %s", groups[0].Name, groups[1].Name)
	}
	if got := groups[0].Categories[0].Slug; got != "c1" {
		t.Fatalf("expected first category in group A to be c1, got %s", got)
	}
}

func TestBuildGroupEmbeds_IncludesEmptyCategoriesAndDynamicCount(t *testing.T) {
	store := &mockPBStore{
		topRecordsBySlug: map[string][]*database.PBRecord{
			"inferno": {
				{DisplayName: "Alpha", TimeText: "01:00.00", TimeCentiseconds: 6000, ProofURL: "https://proof"},
			},
			"fight_caves": {},
		},
	}
	service := &PBService{store: store}

	categories := []*database.PBCategory{
		{Slug: "inferno", DisplayName: "The Inferno", EmbedImageURL: "https://img1"},
		{Slug: "fight_caves", DisplayName: "Fight Caves", EmbedImageURL: "https://img2"},
		{Slug: "new_mode", DisplayName: "New Mode", EmbedImageURL: "https://img3"},
	}

	embeds, err := service.buildGroupEmbeds(context.Background(), "g1", categories)
	if err != nil {
		t.Fatalf("buildGroupEmbeds returned error: %v", err)
	}
	if len(embeds) != 3 {
		t.Fatalf("expected 3 embeds for dynamic category count, got %d", len(embeds))
	}
	if !strings.Contains(embeds[1].Description, "No approved PBs yet.") {
		t.Fatalf("expected empty category description for fight_caves, got %q", embeds[1].Description)
	}
	if !strings.Contains(embeds[2].Description, "No approved PBs yet.") {
		t.Fatalf("expected empty category description for new_mode, got %q", embeds[2].Description)
	}
}

func TestBuildGroupBundleContent_UsesGroupNameHeader(t *testing.T) {
	service := &PBService{}
	if got := service.buildGroupBundleContent("Minigames"); got != "# Minigames" {
		t.Fatalf("expected markdown group header, got %q", got)
	}
	if got := service.buildGroupBundleContent("# Bosses"); got != "# Bosses" {
		t.Fatalf("expected existing markdown prefix preserved, got %q", got)
	}
}

type submitPBMockStore struct {
	mockPBStore
	category *database.PBCategory
	guild    *database.Guild
	record   *database.PBRecord
	created  bool
}

func (m *submitPBMockStore) GetPBCategoryBySlug(context.Context, string) (*database.PBCategory, error) {
	return m.category, nil
}

func (m *submitPBMockStore) GetGuild(context.Context, string) (*database.Guild, error) {
	return m.guild, nil
}

func (m *submitPBMockStore) GetPBRecordByUserAndCategory(context.Context, string, string, string) (*database.PBRecord, error) {
	if m.record == nil {
		return nil, database.ErrPBRecordNotFound
	}
	return m.record, nil
}

func (m *submitPBMockStore) CreatePBSubmission(context.Context, *database.PBSubmission) error {
	m.created = true
	return nil
}

func TestSubmitPB_InvalidTimeReturnsCanonicalMessage(t *testing.T) {
	store := &submitPBMockStore{
		category: &database.PBCategory{Slug: "inferno", IsActive: true},
		guild:    &database.Guild{GuildID: "g1", PbProofsChannelID: "proofs"},
	}
	service := &PBService{store: store}

	invalidTime := "12:34"
	_, _, err := service.SubmitPB(context.Background(), &PBSubmissionInput{
		GuildID:      "g1",
		CategorySlug: "inferno",
		ProofURL:     "https://example.com/proof.png",
		TimeText:     &invalidTime,
	})
	if err == nil || err.Error() != pbTimeFormatUserMessage {
		t.Fatalf("expected canonical user message, got %v", err)
	}
	if store.created {
		t.Fatalf("expected submission not to be created for invalid time")
	}
}

func TestSubmitPB_RequiresDeclaredTime(t *testing.T) {
	store := &submitPBMockStore{
		category: &database.PBCategory{Slug: "inferno", IsActive: true},
		guild:    &database.Guild{GuildID: "g1", PbProofsChannelID: "proofs"},
	}
	service := &PBService{store: store}

	_, _, err := service.SubmitPB(context.Background(), &PBSubmissionInput{
		GuildID:      "g1",
		CategorySlug: "inferno",
		ProofURL:     "https://example.com/proof.png",
	})
	if err == nil || !strings.Contains(err.Error(), "time is required") {
		t.Fatalf("expected time required error, got %v", err)
	}
	if store.created {
		t.Fatalf("expected submission not to be created without time")
	}
}

func TestSubmitPB_RejectsNonImprovingSubmission(t *testing.T) {
	store := &submitPBMockStore{
		category: &database.PBCategory{Slug: "inferno", IsActive: true},
		guild:    &database.Guild{GuildID: "g1", PbProofsChannelID: "proofs"},
		record: &database.PBRecord{
			GuildID:          "g1",
			CategorySlug:     "inferno",
			TimeText:         "01:00.00",
			TimeCentiseconds: 6000,
		},
	}
	service := &PBService{store: store}

	slower := "01:01.00"
	_, _, err := service.SubmitPB(context.Background(), &PBSubmissionInput{
		GuildID:       "g1",
		CategorySlug:  "inferno",
		DiscordUserID: "user-1",
		ProofURL:      "https://example.com/proof.png",
		TimeText:      &slower,
	})
	if !IsSubmissionNotImproving(err) {
		t.Fatalf("expected non-improving error, got %v", err)
	}
	if store.created {
		t.Fatalf("expected submission not to be created for non-improving time")
	}
}

type mockProofStore struct {
	persistURL string
	persistErr error
	deleted    []int64
}

func (m *mockProofStore) PersistFromURL(_ context.Context, submissionID int64, _ string) (string, error) {
	if m.persistErr != nil {
		return "", m.persistErr
	}
	if m.persistURL != "" {
		return m.persistURL, nil
	}
	return fmt.Sprintf("https://proof.example/%d.png", submissionID), nil
}

func (m *mockProofStore) DeleteBySubmissionID(_ context.Context, submissionID int64) error {
	m.deleted = append(m.deleted, submissionID)
	return nil
}

type approvalMockStore struct {
	mockPBStore
	pending       *database.PBSubmission
	existing      *database.PBRecord
	approved      bool
	upsertRecord  *database.PBRecord
	upsertCalled  bool
}

func (m *approvalMockStore) GetPendingPBSubmissionByProofMessageID(context.Context, string, string) (*database.PBSubmission, error) {
	if m.pending == nil {
		return nil, fmt.Errorf("pb submission not found")
	}
	return m.pending, nil
}

func (m *approvalMockStore) GetPBRecordByUserAndCategory(context.Context, string, string, string) (*database.PBRecord, error) {
	if m.existing == nil {
		return nil, database.ErrPBRecordNotFound
	}
	return m.existing, nil
}

func (m *approvalMockStore) ApprovePBSubmission(context.Context, int64, string, time.Time) error {
	m.approved = true
	return nil
}

func (m *approvalMockStore) UpsertPBRecordIfBetter(_ context.Context, record *database.PBRecord) (bool, error) {
	m.upsertCalled = true
	m.upsertRecord = record
	return true, nil
}

func (m *approvalMockStore) GetPBCategoryBySlug(context.Context, string) (*database.PBCategory, error) {
	return &database.PBCategory{Slug: "inferno", DisplayName: "Inferno"}, nil
}

func TestHandleApproval_PersistsProofAndSupersedesPrevious(t *testing.T) {
	timeText := "00:59.00"
	timeCS := int64(5900)
	store := &approvalMockStore{
		pending: &database.PBSubmission{
			ID:               42,
			GuildID:          "g1",
			CategorySlug:     "inferno",
			DiscordUserID:    "user-1",
			DisplayName:      "Alice",
			ProofURL:         "https://cdn.discordapp.com/proof.png",
			TimeText:         &timeText,
			TimeCentiseconds: &timeCS,
		},
		existing: &database.PBRecord{
			ProofSubmissionID: 7,
			TimeCentiseconds:  6000,
		},
	}
	proofStore := &mockProofStore{persistURL: "https://proof.example/42.png"}
	service := &PBService{store: store, proofStore: proofStore}

	result, err := service.HandleApproval(context.Background(), "g1", "msg-1", "mod-1")
	if err != nil {
		t.Fatalf("HandleApproval: %v", err)
	}
	if !store.approved || !store.upsertCalled {
		t.Fatal("expected approval and record upsert")
	}
	if store.upsertRecord.ProofURL != "https://proof.example/42.png" {
		t.Fatalf("expected durable proof url on record, got %q", store.upsertRecord.ProofURL)
	}
	if result.Submission.ProofURL != "https://proof.example/42.png" {
		t.Fatalf("expected durable proof url on submission result, got %q", result.Submission.ProofURL)
	}
	if len(proofStore.deleted) != 1 || proofStore.deleted[0] != 7 {
		t.Fatalf("expected superseded proof deletion for submission 7, got %v", proofStore.deleted)
	}
}

func TestHandleApproval_BlocksUnavailableProof(t *testing.T) {
	timeText := "00:59.00"
	timeCS := int64(5900)
	store := &approvalMockStore{
		pending: &database.PBSubmission{
			ID:               42,
			GuildID:          "g1",
			CategorySlug:     "inferno",
			DiscordUserID:    "user-1",
			ProofURL:         "https://cdn.discordapp.com/proof.png",
			TimeText:         &timeText,
			TimeCentiseconds: &timeCS,
		},
	}
	proofStore := &mockProofStore{persistErr: proofstorage.ErrUnavailableProof}
	service := &PBService{store: store, proofStore: proofStore}

	_, err := service.HandleApproval(context.Background(), "g1", "msg-1", "mod-1")
	if !IsUnavailableSubmissionProof(err) {
		t.Fatalf("expected unavailable proof error, got %v", err)
	}
	if store.approved {
		t.Fatal("expected approval to be blocked")
	}
}

func TestHandleApproval_BlocksNonImprovingSubmission(t *testing.T) {
	timeText := "01:01.00"
	timeCS := int64(6100)
	store := &approvalMockStore{
		pending: &database.PBSubmission{
			ID:               42,
			GuildID:          "g1",
			CategorySlug:     "inferno",
			DiscordUserID:    "user-1",
			ProofURL:         "https://cdn.discordapp.com/proof.png",
			TimeText:         &timeText,
			TimeCentiseconds: &timeCS,
		},
		existing: &database.PBRecord{
			TimeCentiseconds: 6000,
		},
	}
	service := &PBService{store: store, proofStore: &mockProofStore{}}

	_, err := service.HandleApproval(context.Background(), "g1", "msg-1", "mod-1")
	if !IsSubmissionNotImproving(err) {
		t.Fatalf("expected non-improving error, got %v", err)
	}
	if store.approved {
		t.Fatal("expected approval to be blocked")
	}
}

func TestHandleApproval_RequiresProofStorage(t *testing.T) {
	timeText := "00:59.00"
	timeCS := int64(5900)
	store := &approvalMockStore{
		pending: &database.PBSubmission{
			ID:               42,
			GuildID:          "g1",
			CategorySlug:     "inferno",
			DiscordUserID:    "user-1",
			ProofURL:         "https://cdn.discordapp.com/proof.png",
			TimeText:         &timeText,
			TimeCentiseconds: &timeCS,
		},
	}
	service := &PBService{store: store}

	_, err := service.HandleApproval(context.Background(), "g1", "msg-1", "mod-1")
	if err == nil || !strings.Contains(err.Error(), "proof storage is not configured") {
		t.Fatalf("expected proof storage configuration error, got %v", err)
	}
}

func TestIsSubmissionNotImproving(t *testing.T) {
	if !IsSubmissionNotImproving(fmt.Errorf("wrapped: %w", ErrSubmissionNotImproving)) {
		t.Fatal("expected wrapped error to match")
	}
	if IsSubmissionNotImproving(errors.New("other")) {
		t.Fatal("expected unrelated error not to match")
	}
}
