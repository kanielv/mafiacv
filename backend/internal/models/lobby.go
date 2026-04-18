package models

type RoleConfig map[string]int

type NightAction struct {
	ActorID  string `json:"actorId"`
	Action   string `json:"action"`
	TargetID string `json:"targetId"`
	Round    int    `json:"round"`
}

type Lobby struct {
	ID           string                 `json:"lobbyId"`
	Players      []Player               `json:"players"`
	HostID       string                 `json:"hostId"`
	Started      bool                   `json:"started"`
	ChatHistory  []ChatMessage          `json:"chatHistory"`
	RoleConfig   RoleConfig             `json:"roleConfig"`
	Round        int                    `json:"round"`
	NightActions map[string]NightAction `json:"-"`
}
