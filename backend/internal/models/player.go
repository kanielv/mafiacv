package models

type Player struct {
	SocketID string `json:"socketID"`
	Name     string `json:"name"`
	IsAlive  bool   `json:"isAlive"`
	Role     string `json:"role,omitempty"`
}
