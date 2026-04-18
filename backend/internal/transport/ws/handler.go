package ws

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/kanielv/mafiacv/backend/internal/models"
	"github.com/kanielv/mafiacv/backend/internal/storyclient"
)

const storyIntroTimeout = 60 * time.Second
const defaultStoryTheme = "classic noir"

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
	case "night-action":
		h.handleNightAction(client, msg.Data)
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
		Theme   string         `json:"theme"`
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

	// Fire-and-forget narration. Gemini can take 5–30s; we don't block the WS
	// response on it. On failure we log and drop — the frontend should render
	// without narration rather than error.
	theme := payload.Theme
	if theme == "" {
		theme = defaultStoryTheme
	}
	go h.generateGameIntro(payload.LobbyID, players, h.Manager.GetRoleConfig(payload.LobbyID), theme)
}

func (h *Hub) generateGameIntro(lobbyID string, players []models.Player, roles models.RoleConfig, theme string) {
	if h.Story == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), storyIntroTimeout)
	defer cancel()

	names := make([]string, 0, len(players))
	storyPlayers := make([]storyclient.Player, 0, len(players))
	for _, p := range players {
		names = append(names, p.Name)
		storyPlayers = append(storyPlayers, storyclient.Player{Name: p.Name, IsAlive: p.IsAlive})
	}
	roleConfig := map[string]int(roles)

	if err := h.Story.InitGame(ctx, storyclient.InitRequest{
		LobbyID:    lobbyID,
		Theme:      theme,
		Players:    names,
		RoleConfig: roleConfig,
	}); err != nil {
		log.Printf("story init_game for %s: %v", lobbyID, err)
		return
	}

	resp, err := h.Story.GenerateStory(ctx, storyclient.GenerateRequest{
		LobbyID:    lobbyID,
		StoryType:  "game_intro",
		Round:      0,
		Players:    storyPlayers,
		RoleConfig: roleConfig,
	})
	if err != nil {
		log.Printf("story generate game_intro for %s: %v", lobbyID, err)
		return
	}

	h.BroadcastToRoom(lobbyID, MarshalMessage("story-narration", map[string]any{
		"storyType": resp.StoryType,
		"story":     resp.Story,
		"round":     resp.Round,
	}))
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

// handleNightAction routes a role's night action through the manager and
// emits private follow-up events only to the actor.
func (h *Hub) handleNightAction(client *Client, data json.RawMessage) {
	var payload struct {
		LobbyID  string `json:"lobbyId"`
		Action   string `json:"action"`
		TargetID string `json:"targetId"`
		Round    int    `json:"round"`
	}
	if err := json.Unmarshal(data, &payload); err != nil || payload.LobbyID == "" || payload.Action == "" || payload.TargetID == "" {
		h.SendToClient(client.ID, MarshalMessage("error", map[string]string{
			"message": "lobbyId, action, and targetId are required",
		}))
		return
	}

	if err := h.Manager.SubmitNightAction(payload.LobbyID, client.ID, payload.Action, payload.TargetID, payload.Round); err != nil {
		h.SendToClient(client.ID, MarshalMessage("error", map[string]string{
			"message": err.Error(),
		}))
		return
	}

	if payload.Action == "investigate" {
		role := h.Manager.GetPlayerRole(payload.TargetID)
		h.SendToClient(client.ID, MarshalMessage("sheriff-result", map[string]any{
			"targetId": payload.TargetID,
			"role":     role,
			"round":    payload.Round,
		}))
		return
	}

	h.SendToClient(client.ID, MarshalMessage("action-acknowledged", map[string]any{
		"action": payload.Action,
		"round":  payload.Round,
	}))
}

// sanitizePlayers returns a copy of players with Role zeroed out, so
// broadcast payloads don't leak roles after the game has started.
func sanitizePlayers(players []models.Player) []models.Player {
	out := make([]models.Player, len(players))
	for i, p := range players {
		p.Role = ""
		out[i] = p
	}
	return out
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
			"players": sanitizePlayers(remaining),
		}))
	}
}
