package tools

import (
	"context"
	"encoding/json"

	"github.com/kanielv/mafiacv/mcp-server/internal/storage"
)

type initGameArgs struct {
	LobbyID    string         `json:"lobbyId"`
	Theme      string         `json:"theme"`
	Players    []string       `json:"players"`
	RoleConfig map[string]int `json:"roleConfig"`
}

const initGameSchema = `{
  "type": "object",
  "required": ["lobbyId", "players", "roleConfig"],
  "properties": {
    "lobbyId":    {"type": "string", "minLength": 1},
    "theme":      {"type": "string"},
    "players":    {"type": "array", "items": {"type": "string"}},
    "roleConfig": {"type": "object", "additionalProperties": {"type": "integer", "minimum": 0}}
  },
  "additionalProperties": false
}`

func initGameTool(store *storage.Store) Tool {
	return Tool{
		Name:        "init_game",
		Description: "Store theme, player names, and role configuration for a new game.",
		InputSchema: json.RawMessage(initGameSchema),
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var a initGameArgs
			if err := decodeArgs(raw, &a); err != nil {
				return nil, err
			}
			if a.LobbyID == "" {
				return nil, invalidArgf("lobbyId is required")
			}
			if len(a.Players) == 0 {
				return nil, invalidArgf("players must be non-empty")
			}
			if a.RoleConfig == nil {
				return nil, invalidArgf("roleConfig is required")
			}
			if err := store.InitGame(ctx, storage.GameSettings{
				LobbyID:    a.LobbyID,
				Theme:      a.Theme,
				Players:    a.Players,
				RoleConfig: a.RoleConfig,
			}); err != nil {
				return nil, err
			}
			return map[string]any{"ok": true}, nil
		},
	}
}
