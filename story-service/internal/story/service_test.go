package story

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kanielv/mafiacv/story-service/internal/mcp"
)

// geminiStub records prompts and returns a scripted response.
type geminiStub struct {
	mu      sync.Mutex
	calls   []geminiCall
	respond func(sys, user string) (string, error)
}

type geminiCall struct{ System, User string }

func (g *geminiStub) Generate(_ context.Context, sys, user string) (string, error) {
	g.mu.Lock()
	g.calls = append(g.calls, geminiCall{System: sys, User: user})
	g.mu.Unlock()
	if g.respond != nil {
		return g.respond(sys, user)
	}
	return "A shadow fell across Willowbrook.", nil
}

func TestValidateGenerate(t *testing.T) {
	cases := []struct {
		name string
		req  GenerateRequest
	}{
		{"empty lobby", GenerateRequest{StoryType: StoryTypeGameIntro}},
		{"invalid story type", GenerateRequest{LobbyID: "L1", StoryType: StoryType("bogus")}},
		{"negative round", GenerateRequest{LobbyID: "L1", StoryType: StoryTypeNightRecap, Round: -1}},
	}
	s := New(&geminiStub{}, nil) // store nil is fine — validation runs first
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := s.GenerateStory(context.Background(), tc.req)
			if err == nil {
				t.Fatalf("expected error")
			}
			if !errors.Is(err, ErrInvalid) {
				t.Fatalf("expected ErrInvalid, got %v", err)
			}
		})
	}
}

func TestInitGameValidation(t *testing.T) {
	s := New(&geminiStub{}, nil)
	cases := []InitRequest{
		{},
		{LobbyID: "L1"},
		{LobbyID: "L1", Players: []string{"A"}},
	}
	for i, r := range cases {
		err := s.InitGame(context.Background(), r)
		if err == nil || !errors.Is(err, ErrInvalid) {
			t.Errorf("case %d: expected ErrInvalid, got %v", i, err)
		}
	}
}

func TestCleanupGameValidation(t *testing.T) {
	s := New(&geminiStub{}, nil)
	if err := s.CleanupGame(context.Background(), ""); !errors.Is(err, ErrInvalid) {
		t.Errorf("expected ErrInvalid, got %v", err)
	}
}

// TestGenerateStoryRoundTrip builds the real mcp-server, spawns the client,
// and exercises the orchestrator with a Gemini stub.
func TestGenerateStoryRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in -short mode")
	}

	tmp := t.TempDir()
	binPath := filepath.Join(tmp, "mcp-server")
	dbPath := filepath.Join(tmp, "story.db")

	build := exec.Command("go", "build", "-o", binPath, "./cmd/mcp-server")
	build.Dir = "../../../mcp-server"
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build mcp-server: %v\n%s", err, out)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	mcpClient, err := mcp.New(ctx, binPath, dbPath)
	if err != nil {
		t.Fatalf("mcp.New: %v", err)
	}
	defer func() {
		if err := mcpClient.Close(); err != nil {
			t.Errorf("mcp.Close: %v", err)
		}
	}()

	counter := 0
	stub := &geminiStub{
		respond: func(_, _ string) (string, error) {
			counter++
			return fmt.Sprintf("STORY #%d", counter), nil
		},
	}
	svc := New(stub, mcpClient)

	lobbyID := "lobby-L1"
	if err := svc.InitGame(ctx, InitRequest{
		LobbyID:    lobbyID,
		Theme:      "noir",
		Players:    []string{"Alice", "Bob", "Carol"},
		RoleConfig: map[string]int{"mafia": 1, "medic": 1},
	}); err != nil {
		t.Fatalf("InitGame: %v", err)
	}

	// Round 0: game_intro, no events.
	intro, err := svc.GenerateStory(ctx, GenerateRequest{
		LobbyID:   lobbyID,
		StoryType: StoryTypeGameIntro,
		Round:     0,
		Players: []Player{
			{Name: "Alice", IsAlive: true},
			{Name: "Bob", IsAlive: true},
			{Name: "Carol", IsAlive: true},
		},
	})
	if err != nil {
		t.Fatalf("GenerateStory intro: %v", err)
	}
	if intro.Story != "STORY #1" || intro.StoryType != StoryTypeGameIntro || intro.Round != 0 {
		t.Errorf("unexpected intro response: %+v", intro)
	}

	// Round 1: night_recap with events.
	recap, err := svc.GenerateStory(ctx, GenerateRequest{
		LobbyID:   lobbyID,
		StoryType: StoryTypeNightRecap,
		Round:     1,
		Players: []Player{
			{Name: "Alice", IsAlive: true},
			{Name: "Bob", IsAlive: false},
			{Name: "Carol", IsAlive: true},
		},
		Events: []Event{
			{EventType: "kill", Actor: "?", Target: "Bob", Result: "killed"},
		},
	})
	if err != nil {
		t.Fatalf("GenerateStory recap: %v", err)
	}
	if recap.Story != "STORY #2" {
		t.Errorf("expected STORY #2, got %q", recap.Story)
	}

	// Verify stub saw the theme in system and recap hints in user prompt on
	// second call, plus the prior-narrative continuity block.
	if len(stub.calls) != 2 {
		t.Fatalf("expected 2 Gemini calls, got %d", len(stub.calls))
	}
	secondUser := stub.calls[1].User
	for _, want := range []string{"Theme: noir", "Night 1 events:", "kill: ? -> Bob (killed)", "Prior narration", "STORY #1"} {
		if !strings.Contains(secondUser, want) {
			t.Errorf("second user prompt missing %q:\n%s", want, secondUser)
		}
	}
	if !strings.Contains(stub.calls[0].System, "noir") {
		t.Errorf("first system prompt should contain theme: %s", stub.calls[0].System)
	}

	// Verify narratives + event were persisted.
	history, err := mcpClient.GetGameHistory(ctx, lobbyID, nil)
	if err != nil {
		t.Fatalf("GetGameHistory: %v", err)
	}
	if len(history.Narratives) != 2 {
		t.Errorf("expected 2 narratives, got %d", len(history.Narratives))
	}
	if len(history.Events) != 1 || history.Events[0].Target != "Bob" {
		t.Errorf("expected 1 event targeting Bob, got %+v", history.Events)
	}

	// Cleanup wipes everything.
	if err := svc.CleanupGame(ctx, lobbyID); err != nil {
		t.Fatalf("CleanupGame: %v", err)
	}
	post, err := mcpClient.GetGameHistory(ctx, lobbyID, nil)
	if err != nil {
		t.Fatalf("post-cleanup history: %v", err)
	}
	if post.Settings != nil || len(post.Events) != 0 || len(post.Narratives) != 0 {
		t.Errorf("expected empty history after cleanup, got %+v", post)
	}
}

func TestGenerateStoryGeminiFailure(t *testing.T) {
	stub := &geminiStub{
		respond: func(_, _ string) (string, error) { return "", fmt.Errorf("llm outage") },
	}
	s := New(stub, &memStore{})
	_, err := s.GenerateStory(context.Background(), GenerateRequest{
		LobbyID:   "L1",
		StoryType: StoryTypeGameIntro,
		Round:     0,
	})
	if err == nil || !strings.Contains(err.Error(), "llm outage") {
		t.Errorf("expected wrapped llm error, got %v", err)
	}
}

// memStore is a tiny in-memory historyStore for unit-level tests that don't
// want to spawn the real MCP server.
type memStore struct {
	settings   *mcp.StoredSettings
	events     []mcp.StoredGameEvent
	narratives []mcp.StoredNarrative
	nextID     int64
}

func (m *memStore) InitGame(_ context.Context, g mcp.GameSettings) error {
	m.settings = &mcp.StoredSettings{LobbyID: g.LobbyID, Theme: g.Theme, Players: g.Players, RoleConfig: g.RoleConfig}
	return nil
}
func (m *memStore) StoreGameEvent(_ context.Context, e mcp.GameEvent) (int64, error) {
	m.nextID++
	m.events = append(m.events, mcp.StoredGameEvent{ID: m.nextID, LobbyID: e.LobbyID, Round: e.Round, EventType: e.EventType, Actor: e.Actor, Target: e.Target, Result: e.Result})
	return m.nextID, nil
}
func (m *memStore) StoreNarrative(_ context.Context, n mcp.Narrative) (int64, error) {
	m.nextID++
	m.narratives = append(m.narratives, mcp.StoredNarrative{ID: m.nextID, LobbyID: n.LobbyID, Round: n.Round, StoryType: n.StoryType, Story: n.Story})
	return m.nextID, nil
}
func (m *memStore) GetGameHistory(_ context.Context, _ string, _ *int) (mcp.History, error) {
	return mcp.History{Settings: m.settings, Events: m.events, Narratives: m.narratives}, nil
}
func (m *memStore) CleanupGame(_ context.Context, _ string) error {
	m.settings = nil
	m.events = nil
	m.narratives = nil
	return nil
}
