package mcp

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// TestClientRoundTrip builds the real mcp-server binary and exercises the
// full client against it. Skipped in -short mode.
func TestClientRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in -short mode")
	}

	tmp := t.TempDir()
	binPath := filepath.Join(tmp, "mcp-server")
	dbPath := filepath.Join(tmp, "test.db")

	build := exec.Command("go", "build", "-o", binPath, "./cmd/mcp-server")
	build.Dir = "../../../mcp-server"
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build mcp-server: %v\n%s", err, out)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := New(ctx, binPath, dbPath)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	}()

	if err := client.Ping(ctx); err != nil {
		t.Fatalf("Ping: %v", err)
	}

	lobbyID := "lobby-1"
	if err := client.InitGame(ctx, GameSettings{
		LobbyID:    lobbyID,
		Theme:      "noir",
		Players:    []string{"Alice", "Bob", "Carol"},
		RoleConfig: map[string]int{"mafia": 1, "medic": 1},
	}); err != nil {
		t.Fatalf("InitGame: %v", err)
	}

	id, err := client.StoreGameEvent(ctx, GameEvent{
		LobbyID:   lobbyID,
		Round:     1,
		EventType: "kill",
		Actor:     "Alice",
		Target:    "Bob",
		Result:    "killed",
	})
	if err != nil {
		t.Fatalf("StoreGameEvent: %v", err)
	}
	if id <= 0 {
		t.Fatalf("StoreGameEvent id = %d, want > 0", id)
	}

	narrativeID, err := client.StoreNarrative(ctx, Narrative{
		LobbyID:   lobbyID,
		Round:     1,
		StoryType: "night_recap",
		Story:     "The town wakes to tragedy.",
	})
	if err != nil {
		t.Fatalf("StoreNarrative: %v", err)
	}
	if narrativeID <= 0 {
		t.Fatalf("StoreNarrative id = %d, want > 0", narrativeID)
	}

	history, err := client.GetGameHistory(ctx, lobbyID, nil)
	if err != nil {
		t.Fatalf("GetGameHistory: %v", err)
	}
	if history.Settings == nil || history.Settings.Theme != "noir" {
		t.Errorf("history.Settings = %+v, want theme=noir", history.Settings)
	}
	if len(history.Events) != 1 || history.Events[0].Actor != "Alice" {
		t.Errorf("history.Events = %+v, want one Alice event", history.Events)
	}
	if len(history.Narratives) != 1 || history.Narratives[0].StoryType != "night_recap" {
		t.Errorf("history.Narratives = %+v, want one night_recap", history.Narratives)
	}

	// Tool-level error: empty lobbyId should come back as isError.
	if err := client.InitGame(ctx, GameSettings{LobbyID: ""}); err == nil {
		t.Errorf("InitGame with empty lobby: expected error, got nil")
	}

	if err := client.CleanupGame(ctx, lobbyID); err != nil {
		t.Fatalf("CleanupGame: %v", err)
	}

	afterCleanup, err := client.GetGameHistory(ctx, lobbyID, nil)
	if err != nil {
		t.Fatalf("GetGameHistory post-cleanup: %v", err)
	}
	if len(afterCleanup.Events) != 0 || len(afterCleanup.Narratives) != 0 || afterCleanup.Settings != nil {
		t.Errorf("post-cleanup history not empty: %+v", afterCleanup)
	}
}

func TestCallAfterClose(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in -short mode")
	}
	tmp := t.TempDir()
	binPath := filepath.Join(tmp, "mcp-server")
	dbPath := filepath.Join(tmp, "test.db")

	build := exec.Command("go", "build", "-o", binPath, "./cmd/mcp-server")
	build.Dir = "../../../mcp-server"
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}

	ctx := context.Background()
	client, err := New(ctx, binPath, dbPath)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := client.Ping(ctx); err == nil {
		t.Errorf("Ping after Close: expected error, got nil")
	}
}
