package storage_test

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/kanielv/mafiacv/mcp-server/internal/storage"
)

func newStore(t *testing.T) *storage.Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := storage.New(path)
	if err != nil {
		t.Fatalf("storage.New: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestNew_EmptyPath(t *testing.T) {
	if _, err := storage.New(""); err == nil {
		t.Fatal("expected error for empty dbPath")
	}
}

func TestNew_AppliesSchemaIdempotently(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reopen.db")

	s1, err := storage.New(path)
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	if err := s1.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	s2, err := storage.New(path)
	if err != nil {
		t.Fatalf("second open (IF NOT EXISTS should make this a no-op): %v", err)
	}
	_ = s2.Close()
}

func TestInitGame_InsertAndUpsert(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	orig := storage.GameSettings{
		LobbyID:    "lobby-1",
		Theme:      "classic noir",
		Players:    []string{"Alice", "Bob", "Carol"},
		RoleConfig: map[string]int{"mafia": 1, "medic": 1},
	}
	if err := s.InitGame(ctx, orig); err != nil {
		t.Fatalf("InitGame: %v", err)
	}

	h, err := s.GetGameHistory(ctx, "lobby-1", nil)
	if err != nil {
		t.Fatalf("GetGameHistory: %v", err)
	}
	if h.Settings == nil {
		t.Fatal("expected settings, got nil")
	}
	if h.Settings.Theme != "classic noir" {
		t.Errorf("theme: got %q, want %q", h.Settings.Theme, "classic noir")
	}
	if got, want := len(h.Settings.Players), 3; got != want {
		t.Errorf("players len: got %d, want %d", got, want)
	}
	if h.Settings.RoleConfig["mafia"] != 1 {
		t.Errorf("roleConfig mafia: got %d, want 1", h.Settings.RoleConfig["mafia"])
	}
	if h.Settings.CreatedAt.IsZero() {
		t.Error("createdAt should be populated")
	}

	// Upsert: same lobby_id, different theme and players.
	updated := storage.GameSettings{
		LobbyID:    "lobby-1",
		Theme:      "cyberpunk",
		Players:    []string{"Dave", "Eve"},
		RoleConfig: map[string]int{"mafia": 2},
	}
	if err := s.InitGame(ctx, updated); err != nil {
		t.Fatalf("InitGame upsert: %v", err)
	}

	h, err = s.GetGameHistory(ctx, "lobby-1", nil)
	if err != nil {
		t.Fatalf("GetGameHistory after upsert: %v", err)
	}
	if h.Settings.Theme != "cyberpunk" {
		t.Errorf("theme after upsert: got %q, want %q", h.Settings.Theme, "cyberpunk")
	}
	if got, want := len(h.Settings.Players), 2; got != want {
		t.Errorf("players len after upsert: got %d, want %d", got, want)
	}
	if h.Settings.RoleConfig["mafia"] != 2 {
		t.Errorf("roleConfig mafia after upsert: got %d, want 2", h.Settings.RoleConfig["mafia"])
	}
}

func TestStoreGameEvent_ReturnsIDAndPersists(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	e1 := storage.GameEvent{
		LobbyID: "lobby-1", Round: 1, EventType: "kill",
		Actor: "Alice", Target: "Bob", Result: "killed",
	}
	e2 := storage.GameEvent{
		LobbyID: "lobby-1", Round: 2, EventType: "save",
		Actor: "Carol", Target: "Dave", Result: "saved",
	}

	id1, err := s.StoreGameEvent(ctx, e1)
	if err != nil {
		t.Fatalf("StoreGameEvent 1: %v", err)
	}
	id2, err := s.StoreGameEvent(ctx, e2)
	if err != nil {
		t.Fatalf("StoreGameEvent 2: %v", err)
	}
	if id1 == 0 || id2 == 0 || id1 == id2 {
		t.Errorf("expected distinct non-zero ids, got %d and %d", id1, id2)
	}

	h, err := s.GetGameHistory(ctx, "lobby-1", nil)
	if err != nil {
		t.Fatalf("GetGameHistory: %v", err)
	}
	if got, want := len(h.Events), 2; got != want {
		t.Fatalf("events len: got %d, want %d", got, want)
	}

	// Verify every field round-trips correctly. Using distinct values for
	// actor/target/result across the two events ensures a swapped-column
	// bug in the INSERT would be caught.
	checks := []struct {
		got  storage.GameEvent
		want storage.GameEvent
		id   int64
	}{
		{h.Events[0], e1, id1},
		{h.Events[1], e2, id2},
	}
	for i, c := range checks {
		if c.got.ID != c.id {
			t.Errorf("event[%d] ID: got %d, want %d", i, c.got.ID, c.id)
		}
		if c.got.LobbyID != c.want.LobbyID {
			t.Errorf("event[%d] LobbyID: got %q, want %q", i, c.got.LobbyID, c.want.LobbyID)
		}
		if c.got.Round != c.want.Round {
			t.Errorf("event[%d] Round: got %d, want %d", i, c.got.Round, c.want.Round)
		}
		if c.got.EventType != c.want.EventType {
			t.Errorf("event[%d] EventType: got %q, want %q", i, c.got.EventType, c.want.EventType)
		}
		if c.got.Actor != c.want.Actor {
			t.Errorf("event[%d] Actor: got %q, want %q", i, c.got.Actor, c.want.Actor)
		}
		if c.got.Target != c.want.Target {
			t.Errorf("event[%d] Target: got %q, want %q", i, c.got.Target, c.want.Target)
		}
		if c.got.Result != c.want.Result {
			t.Errorf("event[%d] Result: got %q, want %q", i, c.got.Result, c.want.Result)
		}
		if c.got.CreatedAt.IsZero() {
			t.Errorf("event[%d] CreatedAt should be populated", i)
		}
	}
}

func TestStoreNarrative_ReturnsIDAndPersists(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	want := storage.Narrative{
		LobbyID: "lobby-1", Round: 3, StoryType: "night_recap",
		Story: "The town wakes to find Bob missing...",
	}

	id, err := s.StoreNarrative(ctx, want)
	if err != nil {
		t.Fatalf("StoreNarrative: %v", err)
	}
	if id == 0 {
		t.Error("expected non-zero id")
	}

	h, err := s.GetGameHistory(ctx, "lobby-1", nil)
	if err != nil {
		t.Fatalf("GetGameHistory: %v", err)
	}
	if got, wantLen := len(h.Narratives), 1; got != wantLen {
		t.Fatalf("narratives len: got %d, want %d", got, wantLen)
	}

	got := h.Narratives[0]
	if got.ID != id {
		t.Errorf("ID: got %d, want %d", got.ID, id)
	}
	if got.LobbyID != want.LobbyID {
		t.Errorf("LobbyID: got %q, want %q", got.LobbyID, want.LobbyID)
	}
	if got.Round != want.Round {
		t.Errorf("Round: got %d, want %d", got.Round, want.Round)
	}
	if got.StoryType != want.StoryType {
		t.Errorf("StoryType: got %q, want %q", got.StoryType, want.StoryType)
	}
	if got.Story != want.Story {
		t.Errorf("Story: got %q, want %q", got.Story, want.Story)
	}
	if got.CreatedAt.IsZero() {
		t.Error("CreatedAt should be populated")
	}
}

func TestGetGameHistory_FiltersByLobbyAndRound(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	// Seed two lobbies, three rounds each. Actor/Target encode lobby+round so
	// cross-lobby or wrong-round leaks are detectable via content, not just counts.
	for _, lobby := range []string{"A", "B"} {
		for round := 1; round <= 3; round++ {
			if _, err := s.StoreGameEvent(ctx, storage.GameEvent{
				LobbyID: lobby, Round: round, EventType: "kill",
				Actor: lobby + "-actor", Target: fmt.Sprintf("%s-r%d-target", lobby, round),
				Result: "killed",
			}); err != nil {
				t.Fatalf("seed event: %v", err)
			}
			if _, err := s.StoreNarrative(ctx, storage.Narrative{
				LobbyID: lobby, Round: round, StoryType: "night_recap",
				Story: fmt.Sprintf("%s-r%d-story", lobby, round),
			}); err != nil {
				t.Fatalf("seed narrative: %v", err)
			}
		}
	}

	// No round limit: all 3 rounds for lobby A, nothing from B.
	h, err := s.GetGameHistory(ctx, "A", nil)
	if err != nil {
		t.Fatalf("GetGameHistory: %v", err)
	}
	if len(h.Events) != 3 || len(h.Narratives) != 3 {
		t.Fatalf("expected 3+3 for lobby A, got %d events, %d narratives", len(h.Events), len(h.Narratives))
	}
	for i, e := range h.Events {
		wantRound := i + 1
		if e.LobbyID != "A" {
			t.Errorf("event[%d] cross-lobby leak: LobbyID=%q", i, e.LobbyID)
		}
		if e.Round != wantRound {
			t.Errorf("event[%d] Round: got %d, want %d", i, e.Round, wantRound)
		}
		if wantTarget := fmt.Sprintf("A-r%d-target", wantRound); e.Target != wantTarget {
			t.Errorf("event[%d] Target: got %q, want %q", i, e.Target, wantTarget)
		}
	}
	for i, n := range h.Narratives {
		wantRound := i + 1
		if n.LobbyID != "A" {
			t.Errorf("narrative[%d] cross-lobby leak: LobbyID=%q", i, n.LobbyID)
		}
		if wantStory := fmt.Sprintf("A-r%d-story", wantRound); n.Story != wantStory {
			t.Errorf("narrative[%d] Story: got %q, want %q", i, n.Story, wantStory)
		}
	}

	// maxRound = 2 → only rounds 1 and 2.
	maxRound := 2
	h, err = s.GetGameHistory(ctx, "A", &maxRound)
	if err != nil {
		t.Fatalf("GetGameHistory with maxRound: %v", err)
	}
	if len(h.Events) != 2 || len(h.Narratives) != 2 {
		t.Fatalf("maxRound=2: got %d events, %d narratives; want 2/2", len(h.Events), len(h.Narratives))
	}
	if h.Events[0].Round != 1 || h.Events[1].Round != 2 {
		t.Errorf("event rounds under maxRound=2: got [%d, %d], want [1, 2]",
			h.Events[0].Round, h.Events[1].Round)
	}
	if h.Narratives[0].Round != 1 || h.Narratives[1].Round != 2 {
		t.Errorf("narrative rounds under maxRound=2: got [%d, %d], want [1, 2]",
			h.Narratives[0].Round, h.Narratives[1].Round)
	}
}

func TestGetGameHistory_MissingLobbyReturnsEmpty(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	h, err := s.GetGameHistory(ctx, "does-not-exist", nil)
	if err != nil {
		t.Fatalf("GetGameHistory: %v", err)
	}
	if h.Settings != nil {
		t.Errorf("expected nil Settings, got %+v", h.Settings)
	}
	if h.Events == nil || len(h.Events) != 0 {
		t.Errorf("expected empty non-nil Events, got %#v", h.Events)
	}
	if h.Narratives == nil || len(h.Narratives) != 0 {
		t.Errorf("expected empty non-nil Narratives, got %#v", h.Narratives)
	}
}

func TestCleanupGame_RemovesOnlyTargetLobby(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	for _, lobby := range []string{"keep", "drop"} {
		if err := s.InitGame(ctx, storage.GameSettings{
			LobbyID: lobby, Theme: lobby + "-theme",
			Players: []string{lobby + "-player"}, RoleConfig: map[string]int{"mafia": 1},
		}); err != nil {
			t.Fatalf("InitGame %s: %v", lobby, err)
		}
		if _, err := s.StoreGameEvent(ctx, storage.GameEvent{
			LobbyID: lobby, Round: 1, EventType: "kill",
			Actor: lobby + "-actor", Target: lobby + "-target", Result: "killed",
		}); err != nil {
			t.Fatalf("StoreGameEvent %s: %v", lobby, err)
		}
		if _, err := s.StoreNarrative(ctx, storage.Narrative{
			LobbyID: lobby, Round: 1, StoryType: "night_recap", Story: lobby + "-story",
		}); err != nil {
			t.Fatalf("StoreNarrative %s: %v", lobby, err)
		}
	}

	if err := s.CleanupGame(ctx, "drop"); err != nil {
		t.Fatalf("CleanupGame: %v", err)
	}

	dropped, err := s.GetGameHistory(ctx, "drop", nil)
	if err != nil {
		t.Fatalf("GetGameHistory drop: %v", err)
	}
	if dropped.Settings != nil || len(dropped.Events) != 0 || len(dropped.Narratives) != 0 {
		t.Errorf("drop lobby not fully cleaned: %+v", dropped)
	}

	kept, err := s.GetGameHistory(ctx, "keep", nil)
	if err != nil {
		t.Fatalf("GetGameHistory keep: %v", err)
	}
	if kept.Settings == nil {
		t.Fatal("keep lobby settings lost")
	}
	if kept.Settings.Theme != "keep-theme" {
		t.Errorf("keep settings Theme: got %q, want %q", kept.Settings.Theme, "keep-theme")
	}
	if len(kept.Settings.Players) != 1 || kept.Settings.Players[0] != "keep-player" {
		t.Errorf("keep settings Players: got %v, want [keep-player]", kept.Settings.Players)
	}
	if len(kept.Events) != 1 || kept.Events[0].Actor != "keep-actor" || kept.Events[0].Target != "keep-target" {
		t.Errorf("keep events: got %+v", kept.Events)
	}
	if len(kept.Narratives) != 1 || kept.Narratives[0].Story != "keep-story" {
		t.Errorf("keep narratives: got %+v", kept.Narratives)
	}
}

func TestCreatedAt_RoundTripsWithExplicitTime(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	want := time.Date(2026, 4, 16, 12, 30, 45, 0, time.UTC)
	if err := s.InitGame(ctx, storage.GameSettings{
		LobbyID: "t", Theme: "x", Players: []string{"p"}, RoleConfig: map[string]int{}, CreatedAt: want,
	}); err != nil {
		t.Fatalf("InitGame: %v", err)
	}

	h, err := s.GetGameHistory(ctx, "t", nil)
	if err != nil {
		t.Fatalf("GetGameHistory: %v", err)
	}
	if !h.Settings.CreatedAt.Equal(want) {
		t.Errorf("createdAt round-trip: got %s, want %s", h.Settings.CreatedAt, want)
	}
}

// The TEXT datetime column stores second granularity. A time with sub-second
// precision should truncate (not round) on round-trip.
func TestCreatedAt_TruncatesSubsecond(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	input := time.Date(2026, 4, 16, 12, 30, 45, 987_654_321, time.UTC)
	wantTruncated := time.Date(2026, 4, 16, 12, 30, 45, 0, time.UTC)

	if err := s.InitGame(ctx, storage.GameSettings{
		LobbyID: "t", Theme: "x", Players: []string{"p"}, RoleConfig: map[string]int{}, CreatedAt: input,
	}); err != nil {
		t.Fatalf("InitGame: %v", err)
	}

	h, err := s.GetGameHistory(ctx, "t", nil)
	if err != nil {
		t.Fatalf("GetGameHistory: %v", err)
	}
	if !h.Settings.CreatedAt.Equal(wantTruncated) {
		t.Errorf("createdAt sub-second handling: got %s, want %s (truncated)",
			h.Settings.CreatedAt, wantTruncated)
	}
	if h.Settings.CreatedAt.Nanosecond() != 0 {
		t.Errorf("expected 0 nanoseconds after round-trip, got %d", h.Settings.CreatedAt.Nanosecond())
	}
}

// After New() closes and we open the same path again, existing data must
// still be readable — confirms the schema re-apply is a real no-op and not
// a silent data wipe.
func TestNew_PreservesDataAcrossReopen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "persist.db")

	s1, err := storage.New(path)
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	if err := s1.InitGame(ctx, storage.GameSettings{
		LobbyID: "l", Theme: "persisted", Players: []string{"p"}, RoleConfig: map[string]int{"mafia": 1},
	}); err != nil {
		t.Fatalf("InitGame: %v", err)
	}
	if err := s1.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	s2, err := storage.New(path)
	if err != nil {
		t.Fatalf("second open: %v", err)
	}
	defer s2.Close()

	h, err := s2.GetGameHistory(ctx, "l", nil)
	if err != nil {
		t.Fatalf("GetGameHistory after reopen: %v", err)
	}
	if h.Settings == nil || h.Settings.Theme != "persisted" {
		t.Errorf("data lost across reopen: %+v", h.Settings)
	}
}
