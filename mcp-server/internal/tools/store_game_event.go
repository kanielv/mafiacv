package tools

import (
	"context"
	"encoding/json"

	"github.com/kanielv/mafiacv/mcp-server/internal/storage"
)

type storeGameEventArgs struct {
	LobbyID   string `json:"lobbyId"`
	Round     int    `json:"round"`
	EventType string `json:"eventType"`
	Actor     string `json:"actor"`
	Target    string `json:"target"`
	Result    string `json:"result"`
}

var validEventTypes = map[string]bool{
	"kill":        true,
	"save":        true,
	"investigate": true,
	"vote":        true,
	"eliminate":   true,
	"game_ending": true,
}

const storeGameEventSchema = `{
  "type": "object",
  "required": ["lobbyId", "round", "eventType", "actor", "target", "result"],
  "properties": {
    "lobbyId":   {"type": "string", "minLength": 1},
    "round":     {"type": "integer", "minimum": 0},
    "eventType": {"type": "string", "enum": ["kill", "save", "investigate", "vote", "eliminate", "game_ending"]},
    "actor":     {"type": "string"},
    "target":    {"type": "string"},
    "result":    {"type": "string"}
  },
  "additionalProperties": false
}`

func storeGameEventTool(store *storage.Store) Tool {
	return Tool{
		Name:        "store_game_event",
		Description: "Record a round event (kill, save, investigate, vote, eliminate).",
		InputSchema: json.RawMessage(storeGameEventSchema),
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var a storeGameEventArgs
			if err := decodeArgs(raw, &a); err != nil {
				return nil, err
			}
			if a.LobbyID == "" {
				return nil, invalidArgf("lobbyId is required")
			}
			if a.Round < 0 {
				return nil, invalidArgf("round must be >= 0")
			}
			if !validEventTypes[a.EventType] {
				return nil, invalidArgf("eventType %q is not valid", a.EventType)
			}
			id, err := store.StoreGameEvent(ctx, storage.GameEvent{
				LobbyID:   a.LobbyID,
				Round:     a.Round,
				EventType: a.EventType,
				Actor:     a.Actor,
				Target:    a.Target,
				Result:    a.Result,
			})
			if err != nil {
				return nil, err
			}
			return map[string]any{"id": id}, nil
		},
	}
}
