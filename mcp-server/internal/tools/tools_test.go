package tools_test

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/kanielv/mafiacv/mcp-server/internal/storage"
	"github.com/kanielv/mafiacv/mcp-server/internal/tools"
)

func newRegistry(t *testing.T) (*tools.Registry, *storage.Store) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := storage.New(path)
	if err != nil {
		t.Fatalf("storage.New: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return tools.New(s), s
}

func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

func TestRegistry_ListsAllFiveTools(t *testing.T) {
	r, _ := newRegistry(t)
	got := r.List()
	want := []string{"cleanup_game", "get_game_history", "init_game", "store_game_event", "store_narrative"}
	if len(got) != len(want) {
		t.Fatalf("expected %d tools, got %d", len(want), len(got))
	}
	for i, name := range want {
		if got[i].Name != name {
			t.Errorf("tool[%d]: got %q want %q", i, got[i].Name, name)
		}
		if len(got[i].InputSchema) == 0 {
			t.Errorf("tool %q has empty schema", name)
		}
	}
}

func TestRegistry_UnknownTool(t *testing.T) {
	r, _ := newRegistry(t)
	_, err := r.Call(context.Background(), "nope", json.RawMessage(`{}`))
	if !errors.Is(err, tools.ErrUnknownTool) {
		t.Fatalf("expected ErrUnknownTool, got %v", err)
	}
}

func TestInitGame_HappyPath(t *testing.T) {
	r, _ := newRegistry(t)
	args := mustJSON(t, map[string]any{
		"lobbyId":    "abc",
		"theme":      "noir",
		"players":    []string{"Alice", "Bob"},
		"roleConfig": map[string]int{"mafia": 1},
	})
	res, err := r.Call(context.Background(), "init_game", args)
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	m, ok := res.(map[string]any)
	if !ok || m["ok"] != true {
		t.Fatalf("unexpected result: %#v", res)
	}
}

func TestInitGame_MissingLobbyID(t *testing.T) {
	r, _ := newRegistry(t)
	args := mustJSON(t, map[string]any{
		"players":    []string{"A"},
		"roleConfig": map[string]int{"mafia": 1},
	})
	_, err := r.Call(context.Background(), "init_game", args)
	if !errors.Is(err, tools.ErrInvalidArgs) {
		t.Fatalf("expected ErrInvalidArgs, got %v", err)
	}
}

func TestInitGame_UnknownFieldRejected(t *testing.T) {
	r, _ := newRegistry(t)
	args := json.RawMessage(`{"lobbyId":"a","players":["A"],"roleConfig":{"mafia":1},"bogus":true}`)
	_, err := r.Call(context.Background(), "init_game", args)
	if !errors.Is(err, tools.ErrInvalidArgs) {
		t.Fatalf("expected ErrInvalidArgs, got %v", err)
	}
}

func TestStoreGameEvent_HappyPathAndInvalidEnum(t *testing.T) {
	r, _ := newRegistry(t)
	ok := mustJSON(t, map[string]any{
		"lobbyId": "abc", "round": 1, "eventType": "kill",
		"actor": "Alice", "target": "Bob", "result": "killed",
	})
	res, err := r.Call(context.Background(), "store_game_event", ok)
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if _, ok := res.(map[string]any)["id"]; !ok {
		t.Fatalf("expected id in result, got %#v", res)
	}

	bad := mustJSON(t, map[string]any{
		"lobbyId": "abc", "round": 1, "eventType": "smooch",
		"actor": "A", "target": "B", "result": "r",
	})
	if _, err := r.Call(context.Background(), "store_game_event", bad); !errors.Is(err, tools.ErrInvalidArgs) {
		t.Fatalf("expected ErrInvalidArgs for bad eventType, got %v", err)
	}
}

func TestStoreNarrative_InvalidStoryType(t *testing.T) {
	r, _ := newRegistry(t)
	args := mustJSON(t, map[string]any{
		"lobbyId": "abc", "round": 1, "storyType": "epilogue", "story": "...",
	})
	if _, err := r.Call(context.Background(), "store_narrative", args); !errors.Is(err, tools.ErrInvalidArgs) {
		t.Fatalf("expected ErrInvalidArgs, got %v", err)
	}
}

func TestGetGameHistory_MaxRoundFilter(t *testing.T) {
	r, _ := newRegistry(t)
	ctx := context.Background()

	if _, err := r.Call(ctx, "init_game", mustJSON(t, map[string]any{
		"lobbyId": "g1", "theme": "noir",
		"players": []string{"A", "B"}, "roleConfig": map[string]int{"mafia": 1},
	})); err != nil {
		t.Fatalf("init: %v", err)
	}

	for _, round := range []int{1, 2, 3} {
		if _, err := r.Call(ctx, "store_game_event", mustJSON(t, map[string]any{
			"lobbyId": "g1", "round": round, "eventType": "kill",
			"actor": "A", "target": "B", "result": "killed",
		})); err != nil {
			t.Fatalf("store round %d: %v", round, err)
		}
	}

	res, err := r.Call(ctx, "get_game_history", mustJSON(t, map[string]any{
		"lobbyId": "g1", "maxRound": 2,
	}))
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	hist, ok := res.(storage.GameHistory)
	if !ok {
		t.Fatalf("unexpected type: %T", res)
	}
	if len(hist.Events) != 2 {
		t.Fatalf("expected 2 events (rounds 1-2), got %d", len(hist.Events))
	}
	for _, e := range hist.Events {
		if e.Round > 2 {
			t.Errorf("event round %d exceeds maxRound", e.Round)
		}
	}
}

func TestCleanupGame_RemovesData(t *testing.T) {
	r, _ := newRegistry(t)
	ctx := context.Background()

	_, _ = r.Call(ctx, "init_game", mustJSON(t, map[string]any{
		"lobbyId": "c1", "theme": "x",
		"players": []string{"A"}, "roleConfig": map[string]int{"mafia": 1},
	}))
	if _, err := r.Call(ctx, "cleanup_game", mustJSON(t, map[string]any{"lobbyId": "c1"})); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	res, err := r.Call(ctx, "get_game_history", mustJSON(t, map[string]any{"lobbyId": "c1"}))
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	hist := res.(storage.GameHistory)
	if hist.Settings != nil || len(hist.Events) != 0 || len(hist.Narratives) != 0 {
		t.Fatalf("expected empty history after cleanup, got %#v", hist)
	}
}
