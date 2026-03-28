package models

import (
	"encoding/json"
	"time"
)

// WSMessage is the JSON envelope for all WebSocket communication.
type WSMessage struct {
	Event string          `json:"event"`
	Data  json.RawMessage `json:"data"`
}

type ChatMessage struct {
	LobbyID    string    `json:"lobbyId"`
	SenderID   string    `json:"senderId"`
	SenderName string    `json:"senderName"`
	Content    string    `json:"content"`
	Timestamp  time.Time `json:"timestamp"`
}
