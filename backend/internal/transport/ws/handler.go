package ws

import (
	"encoding/json"
	"log"
	"time"

	"github.com/kanielv/mafiacv/backend/internal/models"
)

// HandleMessage dispatches incoming WebSocket messages by event type.
func (h *Hub) HandleMessage(client *Client, msg models.WSMessage) {
	switch msg.Event {
	case "create-lobby":
		h.handleCreateLobby(client, msg.Data)
	case "join-lobby":
		h.handleJoinLobby(client, msg.Data)
	case "start-game":
		h.handleStartGame(client, msg.Data)
	case "chat-message":
		h.handleChatMessage(client, msg.Data)
	default:
		log.Printf("unknown event: %s", msg.Event)
		h.SendToClient(client.ID, MarshalMessage("error", map[string]string{
			"message": "unknown event: " + msg.Event,
		}))
	}
}

func (h *Hub) handleCreateLobby(client *Client, data json.RawMessage) {
	var payload struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(data, &payload); err != nil || payload.Name == "" {
		h.SendToClient(client.ID, MarshalMessage("error", map[string]string{
			"message": "name is required",
		}))
		return
	}

	lobbyID, players := h.Manager.CreateLobby(client.ID, payload.Name)
	h.JoinRoom(client.ID, lobbyID)

	h.SendToClient(client.ID, MarshalMessage("lobby-created", map[string]any{
		"lobbyId": lobbyID,
		"players": players,
	}))
}

func (h *Hub) handleJoinLobby(client *Client, data json.RawMessage) {
	var payload struct {
		LobbyID string `json:"lobbyId"`
		Name    string `json:"name"`
	}
	if err := json.Unmarshal(data, &payload); err != nil || payload.LobbyID == "" || payload.Name == "" {
		h.SendToClient(client.ID, MarshalMessage("error", map[string]string{
			"message": "lobbyId and name are required",
		}))
		return
	}

	lobbyID, players, err := h.Manager.JoinLobby(payload.LobbyID, client.ID, payload.Name)
	if err != nil {
		h.SendToClient(client.ID, MarshalMessage("error", map[string]string{
			"message": err.Error(),
		}))
		return
	}

	h.JoinRoom(client.ID, lobbyID)

	// Send chat history to the joining player
	history := h.Manager.GetChatHistory(lobbyID)
	if history == nil {
		history = []models.ChatMessage{}
	}
	h.SendToClient(client.ID, MarshalMessage("chat-history", map[string]any{
		"messages": history,
	}))

	// Broadcast updated player list to entire lobby
	h.BroadcastToRoom(lobbyID, MarshalMessage("players-updated", map[string]any{
		"players": players,
	}))
}

func (h *Hub) handleStartGame(client *Client, data json.RawMessage) {
	var payload struct {
		LobbyID string         `json:"lobbyId"`
		Roles   map[string]int `json:"roles"`
	}
	if err := json.Unmarshal(data, &payload); err != nil || payload.LobbyID == "" {
		h.SendToClient(client.ID, MarshalMessage("error", map[string]string{
			"message": "lobbyId is required",
		}))
		return
	}

	if len(payload.Roles) > 0 {
		if err := h.Manager.SetRoleConfig(payload.LobbyID, client.ID, models.RoleConfig(payload.Roles)); err != nil {
			h.SendToClient(client.ID, MarshalMessage("error", map[string]string{
				"message": err.Error(),
			}))
			return
		}
	}

	players, err := h.Manager.StartGame(payload.LobbyID)
	if err != nil {
		h.SendToClient(client.ID, MarshalMessage("error", map[string]string{
			"message": err.Error(),
		}))
		return
	}

	h.BroadcastToRoom(payload.LobbyID, MarshalMessage("game-started", map[string]any{}))

	// Send each player their individual role
	for _, p := range players {
		h.SendToClient(p.SocketID, MarshalMessage("roles-assigned", map[string]string{
			"role": p.Role,
		}))
	}
}

func (h *Hub) handleChatMessage(client *Client, data json.RawMessage) {
	var payload struct {
		LobbyID string `json:"lobbyId"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(data, &payload); err != nil || payload.LobbyID == "" || payload.Content == "" {
		return
	}

	senderName := h.Manager.GetPlayerName(client.ID)

	chatMsg := models.ChatMessage{
		LobbyID:    payload.LobbyID,
		SenderID:   client.ID,
		SenderName: senderName,
		Content:    payload.Content,
		Timestamp:  time.Now(),
	}

	h.Manager.AddChatMessage(payload.LobbyID, chatMsg)
	h.BroadcastToRoom(payload.LobbyID, MarshalMessage("chat-message", chatMsg))
}

// handleDisconnect cleans up lobby state when a client disconnects.
func (h *Hub) handleDisconnect(client *Client) {
	lobbyID, remaining := h.Manager.RemovePlayer(client.ID)
	if lobbyID == "" {
		return
	}

	h.BroadcastToRoom(lobbyID, MarshalMessage("user-disconnected", map[string]string{
		"socketId": client.ID,
	}))

	if len(remaining) > 0 {
		h.BroadcastToRoom(lobbyID, MarshalMessage("players-updated", map[string]any{
			"players": remaining,
		}))
	}
}
