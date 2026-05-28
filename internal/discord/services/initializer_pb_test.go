package services

import (
	"context"
	"fmt"
	"testing"

	"osrs-events/internal/database"
)

type mockInitializerPBStore struct {
	categories []*database.PBCategory
	states     []*database.PBLeaderboardMessage
	legacy     []*database.LegacyPBLeaderboardMessage
}

func (m *mockInitializerPBStore) GetGuild(context.Context, string) (*database.Guild, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m *mockInitializerPBStore) SaveGuild(context.Context, *database.Guild) error {
	return fmt.Errorf("not implemented")
}
func (m *mockInitializerPBStore) GetActiveEvent(context.Context, string, string) (*database.Event, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m *mockInitializerPBStore) GetActivePBCategories(context.Context) ([]*database.PBCategory, error) {
	return m.categories, nil
}
func (m *mockInitializerPBStore) GetPBGroupBundleMessage(context.Context, string, string) (*database.PBLeaderboardMessage, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m *mockInitializerPBStore) UpsertPBGroupBundleMessage(context.Context, *database.PBLeaderboardMessage) error {
	return fmt.Errorf("not implemented")
}
func (m *mockInitializerPBStore) ListPBGroupBundleMessagesByGuild(context.Context, string) ([]*database.PBLeaderboardMessage, error) {
	return m.states, nil
}
func (m *mockInitializerPBStore) DeletePBGroupBundleMessagesByGuild(context.Context, string) error {
	return nil
}
func (m *mockInitializerPBStore) ListLegacyPBLeaderboardMessagesByGuild(context.Context, string) ([]*database.LegacyPBLeaderboardMessage, error) {
	return m.legacy, nil
}
func (m *mockInitializerPBStore) DeleteLegacyPBLeaderboardMessagesByGuild(context.Context, string) error {
	return nil
}

type mockPBBundleRefresher struct {
	globalRebuildCalls int
	refreshAllCalls    int
}

func (m *mockPBBundleRefresher) GlobalRebuildGroupBundles(context.Context, string) error {
	m.globalRebuildCalls++
	return nil
}

func (m *mockPBBundleRefresher) RefreshAllGroupBundles(context.Context, string) error {
	m.refreshAllCalls++
	return nil
}

func TestEnsurePBLeaderboardMessages_GlobalRebuildWhenGroupStateMissing(t *testing.T) {
	store := &mockInitializerPBStore{
		categories: []*database.PBCategory{
			{Slug: "inferno", GroupName: "Minigames"},
			{Slug: "nightmare", GroupName: "Bosses"},
		},
	}
	pb := &mockPBBundleRefresher{}
	initializer := &InitializerService{
		store:     store,
		pbService: pb,
	}

	modified, err := initializer.ensurePBLeaderboardMessages(context.Background(), &database.Guild{
		GuildID:                "g1",
		PbLeaderboardChannelID: "channel-1",
	})
	if err != nil {
		t.Fatalf("ensurePBLeaderboardMessages returned error: %v", err)
	}
	if !modified {
		t.Fatalf("expected modified=true when a required group state is missing")
	}
	if pb.globalRebuildCalls != 1 {
		t.Fatalf("expected one global rebuild call, got %d", pb.globalRebuildCalls)
	}
	if pb.refreshAllCalls != 0 {
		t.Fatalf("expected refresh-all not to run during rebuild path")
	}
}
