package storage

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

//go:embed schema.sql
var schemaSQL string

type Store struct {
	db *sql.DB
}

type GameSettings struct {
	LobbyID    string         `json:"lobbyId"`
	Theme      string         `json:"theme"`
	Players    []string       `json:"players"`
	RoleConfig map[string]int `json:"roleConfig"`
	CreatedAt  time.Time      `json:"createdAt"`
}

type GameEvent struct {
	ID        int64     `json:"id"`
	LobbyID   string    `json:"lobbyId"`
	Round     int       `json:"round"`
	EventType string    `json:"eventType"`
	Actor     string    `json:"actor"`
	Target    string    `json:"target"`
	Result    string    `json:"result"`
	CreatedAt time.Time `json:"createdAt"`
}

type Narrative struct {
	ID        int64     `json:"id"`
	LobbyID   string    `json:"lobbyId"`
	Round     int       `json:"round"`
	StoryType string    `json:"storyType"`
	Story     string    `json:"story"`
	CreatedAt time.Time `json:"createdAt"`
}

type GameHistory struct {
	Settings   *GameSettings `json:"settings,omitempty"`
	Events     []GameEvent   `json:"events"`
	Narratives []Narrative   `json:"narratives"`
}

func New(dbPath string) (*Store, error) {
	if dbPath == "" {
		return nil, errors.New("storage: dbPath is required")
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("storage: open %q: %w", dbPath, err)
	}

	db.SetMaxOpenConns(1)

	if _, err := db.ExecContext(context.Background(), schemaSQL); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("storage: apply schema: %w", err)
	}

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) InitGame(ctx context.Context, g GameSettings) error {
	players, err := json.Marshal(g.Players)
	if err != nil {
		return fmt.Errorf("storage: marshal players: %w", err)
	}
	roles, err := json.Marshal(g.RoleConfig)
	if err != nil {
		return fmt.Errorf("storage: marshal roleConfig: %w", err)
	}

	createdAt := g.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}

	const q = `INSERT INTO game_settings
		(lobby_id, theme, players, role_config, created_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(lobby_id) DO UPDATE SET
			theme       = excluded.theme,
			players     = excluded.players,
			role_config = excluded.role_config,
			created_at  = excluded.created_at`
	if _, err := s.db.ExecContext(ctx, q, g.LobbyID, g.Theme, string(players), string(roles), formatTime(createdAt)); err != nil {
		return fmt.Errorf("storage: init_game: %w", err)
	}
	return nil
}

func (s *Store) StoreGameEvent(ctx context.Context, e GameEvent) (int64, error) {
	createdAt := e.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}

	const q = `INSERT INTO game_events
		(lobby_id, round, event_type, actor, target, result, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`
	res, err := s.db.ExecContext(ctx, q, e.LobbyID, e.Round, e.EventType, e.Actor, e.Target, e.Result, formatTime(createdAt))
	if err != nil {
		return 0, fmt.Errorf("storage: store_game_event: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("storage: store_game_event id: %w", err)
	}
	return id, nil
}

func (s *Store) StoreNarrative(ctx context.Context, n Narrative) (int64, error) {
	createdAt := n.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}

	const q = `INSERT INTO narratives
		(lobby_id, round, story_type, story, created_at)
		VALUES (?, ?, ?, ?, ?)`
	res, err := s.db.ExecContext(ctx, q, n.LobbyID, n.Round, n.StoryType, n.Story, formatTime(createdAt))
	if err != nil {
		return 0, fmt.Errorf("storage: store_narrative: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("storage: store_narrative id: %w", err)
	}
	return id, nil
}

func (s *Store) GetGameHistory(ctx context.Context, lobbyID string, maxRound *int) (GameHistory, error) {
	history := GameHistory{
		Events:     []GameEvent{},
		Narratives: []Narrative{},
	}

	settings, err := s.getSettings(ctx, lobbyID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return history, err
	}
	if err == nil {
		history.Settings = settings
	}

	events, err := s.getEvents(ctx, lobbyID, maxRound)
	if err != nil {
		return history, err
	}
	history.Events = events

	narratives, err := s.getNarratives(ctx, lobbyID, maxRound)
	if err != nil {
		return history, err
	}
	history.Narratives = narratives

	return history, nil
}

func (s *Store) getSettings(ctx context.Context, lobbyID string) (*GameSettings, error) {
	const q = `SELECT lobby_id, theme, players, role_config, created_at
		FROM game_settings WHERE lobby_id = ?`

	var (
		g          GameSettings
		players    string
		roleConfig string
		createdAt  string
	)
	err := s.db.QueryRowContext(ctx, q, lobbyID).Scan(&g.LobbyID, &g.Theme, &players, &roleConfig, &createdAt)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(players), &g.Players); err != nil {
		return nil, fmt.Errorf("storage: decode players: %w", err)
	}
	if err := json.Unmarshal([]byte(roleConfig), &g.RoleConfig); err != nil {
		return nil, fmt.Errorf("storage: decode roleConfig: %w", err)
	}
	g.CreatedAt = parseTime(createdAt)
	return &g, nil
}

func (s *Store) getEvents(ctx context.Context, lobbyID string, maxRound *int) ([]GameEvent, error) {
	q := `SELECT id, lobby_id, round, event_type, actor, target, result, created_at
		FROM game_events WHERE lobby_id = ?`
	args := []any{lobbyID}
	if maxRound != nil {
		q += ` AND round <= ?`
		args = append(args, *maxRound)
	}
	q += ` ORDER BY round ASC, id ASC`

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("storage: get_events: %w", err)
	}
	defer rows.Close()

	events := []GameEvent{}
	for rows.Next() {
		var (
			e         GameEvent
			createdAt string
		)
		if err := rows.Scan(&e.ID, &e.LobbyID, &e.Round, &e.EventType, &e.Actor, &e.Target, &e.Result, &createdAt); err != nil {
			return nil, fmt.Errorf("storage: scan event: %w", err)
		}
		e.CreatedAt = parseTime(createdAt)
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("storage: iterate events: %w", err)
	}
	return events, nil
}

func (s *Store) getNarratives(ctx context.Context, lobbyID string, maxRound *int) ([]Narrative, error) {
	q := `SELECT id, lobby_id, round, story_type, story, created_at
		FROM narratives WHERE lobby_id = ?`
	args := []any{lobbyID}
	if maxRound != nil {
		q += ` AND round <= ?`
		args = append(args, *maxRound)
	}
	q += ` ORDER BY round ASC, id ASC`

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("storage: get_narratives: %w", err)
	}
	defer rows.Close()

	narratives := []Narrative{}
	for rows.Next() {
		var (
			n         Narrative
			createdAt string
		)
		if err := rows.Scan(&n.ID, &n.LobbyID, &n.Round, &n.StoryType, &n.Story, &createdAt); err != nil {
			return nil, fmt.Errorf("storage: scan narrative: %w", err)
		}
		n.CreatedAt = parseTime(createdAt)
		narratives = append(narratives, n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("storage: iterate narratives: %w", err)
	}
	return narratives, nil
}

// sqliteTimeLayout matches the format produced by SQLite's datetime('now').
const sqliteTimeLayout = "2006-01-02 15:04:05"

func formatTime(t time.Time) string {
	return t.UTC().Format(sqliteTimeLayout)
}

func parseTime(s string) time.Time {
	if t, err := time.Parse(sqliteTimeLayout, s); err == nil {
		return t.UTC()
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC()
	}
	return time.Time{}
}

func (s *Store) CleanupGame(ctx context.Context, lobbyID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("storage: cleanup begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, table := range []string{"game_events", "narratives", "game_settings"} {
		if _, err := tx.ExecContext(ctx, "DELETE FROM "+table+" WHERE lobby_id = ?", lobbyID); err != nil {
			return fmt.Errorf("storage: cleanup %s: %w", table, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("storage: cleanup commit: %w", err)
	}
	return nil
}
