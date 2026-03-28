package models

type RoleConfig map[string]int

type Lobby struct {
	ID          string        `json:"lobbyId"`
	Players     []Player      `json:"players"`
	HostID      string        `json:"hostId"`
	Started     bool          `json:"started"`
	ChatHistory []ChatMessage `json:"chatHistory"`
	RoleConfig  RoleConfig    `json:"roleConfig"`
}
