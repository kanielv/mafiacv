// Package story orchestrates the fetch-build-generate-store flow between the
// Gemini client and the MCP server. Handlers sit on top of this package.
package story

type StoryType string

const (
	StoryTypeGameIntro   StoryType = "game_intro"
	StoryTypeNightRecap  StoryType = "night_recap"
	StoryTypeDayIntro    StoryType = "day_intro"
	StoryTypeVoteRecap   StoryType = "vote_recap"
	StoryTypeGameEnding  StoryType = "game_ending"
)

func (s StoryType) Valid() bool {
	switch s {
	case StoryTypeGameIntro, StoryTypeNightRecap, StoryTypeDayIntro, StoryTypeVoteRecap, StoryTypeGameEnding:
		return true
	}
	return false
}

type Player struct {
	Name    string `json:"name"`
	IsAlive bool   `json:"isAlive"`
	Role    string `json:"role,omitempty"`
}

type Event struct {
	EventType string `json:"eventType"` // kill|save|investigate|vote|eliminate
	Actor     string `json:"actor"`
	Target    string `json:"target"`
	Result    string `json:"result"`
}

type InitRequest struct {
	LobbyID    string         `json:"lobbyId"`
	Theme      string         `json:"theme"`
	Players    []string       `json:"players"`
	RoleConfig map[string]int `json:"roleConfig"`
}

type GenerateRequest struct {
	LobbyID    string         `json:"lobbyId"`
	StoryType  StoryType      `json:"storyType"`
	Round      int            `json:"round"`
	Events     []Event        `json:"events"`
	Players    []Player       `json:"players"`
	RoleConfig map[string]int `json:"roleConfig"`
}

type GenerateResponse struct {
	LobbyID   string    `json:"lobbyId"`
	Round     int       `json:"round"`
	StoryType StoryType `json:"storyType"`
	Story     string    `json:"story"`
}
