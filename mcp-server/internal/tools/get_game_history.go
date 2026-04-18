package tools

import (
	"context"
	"encoding/json"

	"github.com/kanielv/mafiacv/mcp-server/internal/storage"
)

type getGameHistoryArgs struct {
	LobbyID  string `json:"lobbyId"`
	MaxRound *int   `json:"maxRound,omitempty"`
}

const getGameHistorySchema = `{
  "type": "object",
  "required": ["lobbyId"],
  "properties": {
    "lobbyId":  {"type": "string", "minLength": 1},
    "maxRound": {"type": "integer", "minimum": 0}
  },
  "additionalProperties": false
}`

func getGameHistoryTool(store *storage.Store) Tool {
	return Tool{
		Name:        "get_game_history",
		Description: "Retrieve all events and narratives for a lobby, optionally capped at a round.",
		InputSchema: json.RawMessage(getGameHistorySchema),
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var a getGameHistoryArgs
			if err := decodeArgs(raw, &a); err != nil {
				return nil, err
			}
			if a.LobbyID == "" {
				return nil, invalidArgf("lobbyId is required")
			}
			if a.MaxRound != nil && *a.MaxRound < 0 {
				return nil, invalidArgf("maxRound must be >= 0")
			}
			return store.GetGameHistory(ctx, a.LobbyID, a.MaxRound)
		},
	}
}
