package story

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/kanielv/mafiacv/story-service/internal/mcp"
)

// generator is the narrow contract the orchestrator needs from the LLM
// client. *gemini.Client satisfies it; tests supply an in-memory stub.
type generator interface {
	Generate(ctx context.Context, systemPrompt, userPrompt string) (string, error)
}

// historyStore is the subset of mcp.Client used here. Keeping it as an
// interface leaves room for test doubles without reaching into the real
// client's child-process lifecycle.
type historyStore interface {
	InitGame(ctx context.Context, g mcp.GameSettings) error
	StoreGameEvent(ctx context.Context, e mcp.GameEvent) (int64, error)
	StoreNarrative(ctx context.Context, n mcp.Narrative) (int64, error)
	GetGameHistory(ctx context.Context, lobbyID string, maxRound *int) (mcp.History, error)
	CleanupGame(ctx context.Context, lobbyID string) error
}

type Service struct {
	gen   generator
	store historyStore
}

// ErrInvalid is returned for validation failures. Handlers can map this to a
// 400 response.
var ErrInvalid = errors.New("story: invalid request")

func New(g generator, s historyStore) *Service {
	return &Service{gen: g, store: s}
}

func (s *Service) InitGame(ctx context.Context, r InitRequest) error {
	if r.LobbyID == "" {
		return fmt.Errorf("%w: lobbyId is required", ErrInvalid)
	}
	if len(r.Players) == 0 {
		return fmt.Errorf("%w: players must be non-empty", ErrInvalid)
	}
	if r.RoleConfig == nil {
		return fmt.Errorf("%w: roleConfig is required", ErrInvalid)
	}
	if err := s.store.InitGame(ctx, mcp.GameSettings{
		LobbyID:    r.LobbyID,
		Theme:      r.Theme,
		Players:    r.Players,
		RoleConfig: r.RoleConfig,
	}); err != nil {
		return fmt.Errorf("story: init_game: %w", err)
	}
	return nil
}

func (s *Service) GenerateStory(ctx context.Context, r GenerateRequest) (GenerateResponse, error) {
	if err := validateGenerate(r); err != nil {
		return GenerateResponse{}, err
	}

	history, err := s.store.GetGameHistory(ctx, r.LobbyID, nil)
	if err != nil {
		return GenerateResponse{}, fmt.Errorf("story: fetch history: %w", err)
	}

	// Persist events before generation so the history reflects what the
	// narrator saw and any retry operates on consistent state.
	for _, e := range r.Events {
		if _, err := s.store.StoreGameEvent(ctx, mcp.GameEvent{
			LobbyID:   r.LobbyID,
			Round:     r.Round,
			EventType: e.EventType,
			Actor:     e.Actor,
			Target:    e.Target,
			Result:    e.Result,
		}); err != nil {
			return GenerateResponse{}, fmt.Errorf("story: store event %s: %w", e.EventType, err)
		}
	}

	theme := ""
	if history.Settings != nil {
		theme = history.Settings.Theme
	}
	systemPrompt := BuildSystemPrompt(theme)
	userPrompt, err := BuildUserPrompt(r, history)
	if err != nil {
		return GenerateResponse{}, fmt.Errorf("story: build prompt: %w", err)
	}

	storyText, err := s.gen.Generate(ctx, systemPrompt, userPrompt)
	if err != nil {
		return GenerateResponse{}, fmt.Errorf("story: generate: %w", err)
	}

	// Narrative persistence is best-effort: the caller already has the story
	// in hand, so a storage hiccup shouldn't fail the response.
	if _, err := s.store.StoreNarrative(ctx, mcp.Narrative{
		LobbyID:   r.LobbyID,
		Round:     r.Round,
		StoryType: string(r.StoryType),
		Story:     storyText,
	}); err != nil {
		log.Printf("story: store_narrative (non-fatal): %v", err)
	}

	return GenerateResponse{
		LobbyID:   r.LobbyID,
		Round:     r.Round,
		StoryType: r.StoryType,
		Story:     storyText,
	}, nil
}

func (s *Service) CleanupGame(ctx context.Context, lobbyID string) error {
	if lobbyID == "" {
		return fmt.Errorf("%w: lobbyId is required", ErrInvalid)
	}
	if err := s.store.CleanupGame(ctx, lobbyID); err != nil {
		return fmt.Errorf("story: cleanup_game: %w", err)
	}
	return nil
}

func validateGenerate(r GenerateRequest) error {
	if r.LobbyID == "" {
		return fmt.Errorf("%w: lobbyId is required", ErrInvalid)
	}
	if !r.StoryType.Valid() {
		return fmt.Errorf("%w: storyType %q is not valid", ErrInvalid, r.StoryType)
	}
	if r.Round < 0 {
		return fmt.Errorf("%w: round must be >= 0", ErrInvalid)
	}
	return nil
}
