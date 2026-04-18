package tools

import (
	"context"
	"encoding/json"

	"github.com/kanielv/mafiacv/mcp-server/internal/storage"
)

type cleanupGameArgs struct {
	LobbyID string `json:"lobbyId"`
}

const cleanupGameSchema = `{
  "type": "object",
  "required": ["lobbyId"],
  "properties": {
    "lobbyId": {"type": "string", "minLength": 1}
  },
  "additionalProperties": false
}`

func cleanupGameTool(store *storage.Store) Tool {
	return Tool{
		Name:        "cleanup_game",
		Description: "Delete all data for a finished game.",
		InputSchema: json.RawMessage(cleanupGameSchema),
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var a cleanupGameArgs
			if err := decodeArgs(raw, &a); err != nil {
				return nil, err
			}
			if a.LobbyID == "" {
				return nil, invalidArgf("lobbyId is required")
			}
			if err := store.CleanupGame(ctx, a.LobbyID); err != nil {
				return nil, err
			}
			return map[string]any{"ok": true}, nil
		},
	}
}
