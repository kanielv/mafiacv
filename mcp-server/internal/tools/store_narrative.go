package tools

import (
	"context"
	"encoding/json"

	"github.com/kanielv/mafiacv/mcp-server/internal/storage"
)

type storeNarrativeArgs struct {
	LobbyID   string `json:"lobbyId"`
	Round     int    `json:"round"`
	StoryType string `json:"storyType"`
	Story     string `json:"story"`
}

var validStoryTypes = map[string]bool{
	"game_intro":  true,
	"night_recap": true,
	"day_intro":   true,
	"vote_recap":  true,
}

const storeNarrativeSchema = `{
  "type": "object",
  "required": ["lobbyId", "round", "storyType", "story"],
  "properties": {
    "lobbyId":   {"type": "string", "minLength": 1},
    "round":     {"type": "integer", "minimum": 0},
    "storyType": {"type": "string", "enum": ["game_intro", "night_recap", "day_intro", "vote_recap"]},
    "story":     {"type": "string", "minLength": 1}
  },
  "additionalProperties": false
}`

func storeNarrativeTool(store *storage.Store) Tool {
	return Tool{
		Name:        "store_narrative",
		Description: "Persist a generated story for continuity across rounds.",
		InputSchema: json.RawMessage(storeNarrativeSchema),
		Handler: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var a storeNarrativeArgs
			if err := decodeArgs(raw, &a); err != nil {
				return nil, err
			}
			if a.LobbyID == "" {
				return nil, invalidArgf("lobbyId is required")
			}
			if a.Round < 0 {
				return nil, invalidArgf("round must be >= 0")
			}
			if !validStoryTypes[a.StoryType] {
				return nil, invalidArgf("storyType %q is not valid", a.StoryType)
			}
			if a.Story == "" {
				return nil, invalidArgf("story is required")
			}
			id, err := store.StoreNarrative(ctx, storage.Narrative{
				LobbyID:   a.LobbyID,
				Round:     a.Round,
				StoryType: a.StoryType,
				Story:     a.Story,
			})
			if err != nil {
				return nil, err
			}
			return map[string]any{"id": id}, nil
		},
	}
}
