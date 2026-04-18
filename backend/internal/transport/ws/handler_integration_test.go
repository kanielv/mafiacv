package ws

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/kanielv/mafiacv/backend/internal/lobby"
	"github.com/kanielv/mafiacv/backend/internal/models"
)

// --- Test Helpers ---

func setupTestServer(t *testing.T) (*httptest.Server, *Hub) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	mgr := lobby.NewManager()
	hub := NewHub(mgr, nil)
	go hub.Run()
	router := gin.New()
	router.GET("/ws", ServeWS(hub))
	server := httptest.NewServer(router.Handler())
	t.Cleanup(func() { server.Close() })
	return server, hub
}

func connectWS(t *testing.T, server *httptest.Server) (*websocket.Conn, string) {
	t.Helper()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	// Read the automatic "connected" event
	msg := readMessage(t, conn)
	if msg.Event != "connected" {
		t.Fatalf("expected 'connected', got %q", msg.Event)
	}
	var data struct {
		SocketID string `json:"socketId"`
	}
	if err := json.Unmarshal(msg.Data, &data); err != nil {
		t.Fatalf("unmarshal connected data: %v", err)
	}
	if data.SocketID == "" {
		t.Fatal("expected non-empty socketId")
	}
	return conn, data.SocketID
}

func sendEvent(t *testing.T, conn *websocket.Conn, event string, data any) {
	t.Helper()
	if err := conn.WriteMessage(websocket.TextMessage, MarshalMessage(event, data)); err != nil {
		t.Fatalf("send %q failed: %v", event, err)
	}
}

func readMessage(t *testing.T, conn *websocket.Conn) models.WSMessage {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, raw, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	var msg models.WSMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		t.Fatalf("unmarshal message: %v", err)
	}
	return msg
}

func readMessageWithTimeout(t *testing.T, conn *websocket.Conn, d time.Duration) (models.WSMessage, bool) {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(d))
	_, raw, err := conn.ReadMessage()
	if err != nil {
		return models.WSMessage{}, false
	}
	var msg models.WSMessage
	json.Unmarshal(raw, &msg)
	return msg, true
}

// --- Happy-Path Tests ---

func TestIntegration_CreateLobby(t *testing.T) {
	server, _ := setupTestServer(t)
	conn, _ := connectWS(t, server)

	sendEvent(t, conn, "create-lobby", map[string]string{"name": "Alice"})

	msg := readMessage(t, conn)
	if msg.Event != "lobby-created" {
		t.Fatalf("expected 'lobby-created', got %q", msg.Event)
	}

	var data struct {
		LobbyID string          `json:"lobbyId"`
		Players []models.Player `json:"players"`
	}
	if err := json.Unmarshal(msg.Data, &data); err != nil {
		t.Fatalf("unmarshal lobby-created: %v", err)
	}
	if data.LobbyID == "" {
		t.Error("expected non-empty lobbyId")
	}
	if len(data.Players) != 1 {
		t.Fatalf("expected 1 player, got %d", len(data.Players))
	}
	if data.Players[0].Name != "Alice" {
		t.Errorf("expected player name 'Alice', got %q", data.Players[0].Name)
	}
}

func TestIntegration_JoinLobby(t *testing.T) {
	server, _ := setupTestServer(t)
	connA, _ := connectWS(t, server)
	connB, _ := connectWS(t, server)

	// Alice creates a lobby
	sendEvent(t, connA, "create-lobby", map[string]string{"name": "Alice"})
	createMsg := readMessage(t, connA)
	var createData struct {
		LobbyID string `json:"lobbyId"`
	}
	json.Unmarshal(createMsg.Data, &createData)
	lobbyID := createData.LobbyID

	// Bob joins
	sendEvent(t, connB, "join-lobby", map[string]any{"lobbyId": lobbyID, "name": "Bob"})

	// Bob should receive chat-history first
	chatHistMsg := readMessage(t, connB)
	if chatHistMsg.Event != "chat-history" {
		t.Fatalf("expected 'chat-history', got %q", chatHistMsg.Event)
	}

	// Bob should receive players-updated
	playersMsg := readMessage(t, connB)
	if playersMsg.Event != "players-updated" {
		t.Fatalf("expected 'players-updated' for Bob, got %q", playersMsg.Event)
	}
	var playersData struct {
		Players []models.Player `json:"players"`
	}
	json.Unmarshal(playersMsg.Data, &playersData)
	if len(playersData.Players) != 2 {
		t.Fatalf("expected 2 players, got %d", len(playersData.Players))
	}

	// Alice should also receive players-updated
	aliceMsg := readMessage(t, connA)
	if aliceMsg.Event != "players-updated" {
		t.Fatalf("expected 'players-updated' for Alice, got %q", aliceMsg.Event)
	}
}

func TestIntegration_StartGame(t *testing.T) {
	server, _ := setupTestServer(t)
	connA, _ := connectWS(t, server)
	connB, _ := connectWS(t, server)

	// Create and join
	sendEvent(t, connA, "create-lobby", map[string]string{"name": "Alice"})
	createMsg := readMessage(t, connA)
	var createData struct {
		LobbyID string `json:"lobbyId"`
	}
	json.Unmarshal(createMsg.Data, &createData)
	lobbyID := createData.LobbyID

	sendEvent(t, connB, "join-lobby", map[string]any{"lobbyId": lobbyID, "name": "Bob"})
	// Drain join messages: Bob gets chat-history + players-updated, Alice gets players-updated
	readMessage(t, connB) // chat-history
	readMessage(t, connB) // players-updated
	readMessage(t, connA) // players-updated

	// Start game
	sendEvent(t, connA, "start-game", map[string]string{"lobbyId": lobbyID})

	// Both should receive game-started
	msgA := readMessage(t, connA)
	if msgA.Event != "game-started" {
		t.Fatalf("expected 'game-started' for Alice, got %q", msgA.Event)
	}
	msgB := readMessage(t, connB)
	if msgB.Event != "game-started" {
		t.Fatalf("expected 'game-started' for Bob, got %q", msgB.Event)
	}
}

func TestIntegration_ChatMessage(t *testing.T) {
	server, _ := setupTestServer(t)
	connA, _ := connectWS(t, server)
	connB, _ := connectWS(t, server)

	// Create and join
	sendEvent(t, connA, "create-lobby", map[string]string{"name": "Alice"})
	createMsg := readMessage(t, connA)
	var createData struct {
		LobbyID string `json:"lobbyId"`
	}
	json.Unmarshal(createMsg.Data, &createData)
	lobbyID := createData.LobbyID

	sendEvent(t, connB, "join-lobby", map[string]any{"lobbyId": lobbyID, "name": "Bob"})
	readMessage(t, connB) // chat-history
	readMessage(t, connB) // players-updated
	readMessage(t, connA) // players-updated

	// Alice sends a chat message
	sendEvent(t, connA, "chat-message", map[string]string{"lobbyId": lobbyID, "content": "hello everyone"})

	// Both should receive the chat message
	chatA := readMessage(t, connA)
	if chatA.Event != "chat-message" {
		t.Fatalf("expected 'chat-message' for Alice, got %q", chatA.Event)
	}
	var chatDataA models.ChatMessage
	json.Unmarshal(chatA.Data, &chatDataA)
	if chatDataA.SenderName != "Alice" {
		t.Errorf("expected sender 'Alice', got %q", chatDataA.SenderName)
	}
	if chatDataA.Content != "hello everyone" {
		t.Errorf("expected content 'hello everyone', got %q", chatDataA.Content)
	}

	chatB := readMessage(t, connB)
	if chatB.Event != "chat-message" {
		t.Fatalf("expected 'chat-message' for Bob, got %q", chatB.Event)
	}
}

func TestIntegration_ChatHistory_OnJoin(t *testing.T) {
	server, _ := setupTestServer(t)
	connA, _ := connectWS(t, server)

	// Alice creates lobby and sends a chat message
	sendEvent(t, connA, "create-lobby", map[string]string{"name": "Alice"})
	createMsg := readMessage(t, connA)
	var createData struct {
		LobbyID string `json:"lobbyId"`
	}
	json.Unmarshal(createMsg.Data, &createData)
	lobbyID := createData.LobbyID

	sendEvent(t, connA, "chat-message", map[string]string{"lobbyId": lobbyID, "content": "first message"})
	readMessage(t, connA) // drain Alice's broadcast

	// Bob joins later
	connB, _ := connectWS(t, server)
	sendEvent(t, connB, "join-lobby", map[string]any{"lobbyId": lobbyID, "name": "Bob"})

	// Bob should receive chat-history with the prior message
	chatHistMsg := readMessage(t, connB)
	if chatHistMsg.Event != "chat-history" {
		t.Fatalf("expected 'chat-history', got %q", chatHistMsg.Event)
	}
	var histData struct {
		Messages []models.ChatMessage `json:"messages"`
	}
	json.Unmarshal(chatHistMsg.Data, &histData)
	if len(histData.Messages) != 1 {
		t.Fatalf("expected 1 message in history, got %d", len(histData.Messages))
	}
	if histData.Messages[0].Content != "first message" {
		t.Errorf("expected content 'first message', got %q", histData.Messages[0].Content)
	}
}

// --- Error-Path Tests ---

func TestIntegration_CreateLobby_MissingName(t *testing.T) {
	server, _ := setupTestServer(t)
	conn, _ := connectWS(t, server)

	sendEvent(t, conn, "create-lobby", map[string]string{})

	msg := readMessage(t, conn)
	if msg.Event != "error" {
		t.Fatalf("expected 'error', got %q", msg.Event)
	}
	var errData struct {
		Message string `json:"message"`
	}
	json.Unmarshal(msg.Data, &errData)
	if errData.Message != "name is required" {
		t.Errorf("expected 'name is required', got %q", errData.Message)
	}
}

func TestIntegration_JoinLobby_NotFound(t *testing.T) {
	server, _ := setupTestServer(t)
	conn, _ := connectWS(t, server)

	sendEvent(t, conn, "join-lobby", map[string]string{"lobbyId": "nonexistent", "name": "Alice"})

	msg := readMessage(t, conn)
	if msg.Event != "error" {
		t.Fatalf("expected 'error', got %q", msg.Event)
	}
	var errData struct {
		Message string `json:"message"`
	}
	json.Unmarshal(msg.Data, &errData)
	if errData.Message != "lobby not found" {
		t.Errorf("expected 'lobby not found', got %q", errData.Message)
	}
}

func TestIntegration_JoinLobby_AlreadyStarted(t *testing.T) {
	server, _ := setupTestServer(t)
	connA, _ := connectWS(t, server)
	connB, _ := connectWS(t, server)

	// Create lobby, join, start game
	sendEvent(t, connA, "create-lobby", map[string]string{"name": "Alice"})
	createMsg := readMessage(t, connA)
	var createData struct {
		LobbyID string `json:"lobbyId"`
	}
	json.Unmarshal(createMsg.Data, &createData)
	lobbyID := createData.LobbyID

	sendEvent(t, connB, "join-lobby", map[string]any{"lobbyId": lobbyID, "name": "Bob"})
	readMessage(t, connB) // chat-history
	readMessage(t, connB) // players-updated
	readMessage(t, connA) // players-updated

	sendEvent(t, connA, "start-game", map[string]string{"lobbyId": lobbyID})
	readMessage(t, connA) // game-started
	readMessage(t, connB) // game-started

	// Charlie tries to join a started game
	connC, _ := connectWS(t, server)
	sendEvent(t, connC, "join-lobby", map[string]any{"lobbyId": lobbyID, "name": "Charlie"})

	msg := readMessage(t, connC)
	if msg.Event != "error" {
		t.Fatalf("expected 'error', got %q", msg.Event)
	}
	var errData struct {
		Message string `json:"message"`
	}
	json.Unmarshal(msg.Data, &errData)
	if errData.Message != "game already started" {
		t.Errorf("expected 'game already started', got %q", errData.Message)
	}
}

func TestIntegration_JoinLobby_MissingFields(t *testing.T) {
	server, _ := setupTestServer(t)
	conn, _ := connectWS(t, server)

	// Missing both fields
	sendEvent(t, conn, "join-lobby", map[string]string{})
	msg := readMessage(t, conn)
	if msg.Event != "error" {
		t.Fatalf("expected 'error', got %q", msg.Event)
	}
	var errData struct {
		Message string `json:"message"`
	}
	json.Unmarshal(msg.Data, &errData)
	if errData.Message != "lobbyId and name are required" {
		t.Errorf("expected 'lobbyId and name are required', got %q", errData.Message)
	}
}

func TestIntegration_StartGame_NotFound(t *testing.T) {
	server, _ := setupTestServer(t)
	conn, _ := connectWS(t, server)

	sendEvent(t, conn, "start-game", map[string]string{"lobbyId": "nonexistent"})

	msg := readMessage(t, conn)
	if msg.Event != "error" {
		t.Fatalf("expected 'error', got %q", msg.Event)
	}
	var errData struct {
		Message string `json:"message"`
	}
	json.Unmarshal(msg.Data, &errData)
	if errData.Message != "lobby not found" {
		t.Errorf("expected 'lobby not found', got %q", errData.Message)
	}
}

func TestIntegration_UnknownEvent(t *testing.T) {
	server, _ := setupTestServer(t)
	conn, _ := connectWS(t, server)

	sendEvent(t, conn, "foobar", map[string]string{})

	msg := readMessage(t, conn)
	if msg.Event != "error" {
		t.Fatalf("expected 'error', got %q", msg.Event)
	}
	var errData struct {
		Message string `json:"message"`
	}
	json.Unmarshal(msg.Data, &errData)
	if !strings.Contains(errData.Message, "unknown event") {
		t.Errorf("expected message containing 'unknown event', got %q", errData.Message)
	}
}

// --- Disconnect Tests ---

func TestIntegration_Disconnect(t *testing.T) {
	server, _ := setupTestServer(t)
	connA, _ := connectWS(t, server)
	connB, socketB := connectWS(t, server)

	// Create and join
	sendEvent(t, connA, "create-lobby", map[string]string{"name": "Alice"})
	createMsg := readMessage(t, connA)
	var createData struct {
		LobbyID string `json:"lobbyId"`
	}
	json.Unmarshal(createMsg.Data, &createData)
	lobbyID := createData.LobbyID

	sendEvent(t, connB, "join-lobby", map[string]any{"lobbyId": lobbyID, "name": "Bob"})
	readMessage(t, connB) // chat-history
	readMessage(t, connB) // players-updated
	readMessage(t, connA) // players-updated

	// Bob disconnects
	connB.Close()

	// Alice should receive user-disconnected
	disconnMsg := readMessage(t, connA)
	if disconnMsg.Event != "user-disconnected" {
		t.Fatalf("expected 'user-disconnected', got %q", disconnMsg.Event)
	}
	var disconnData struct {
		SocketID string `json:"socketId"`
	}
	json.Unmarshal(disconnMsg.Data, &disconnData)
	if disconnData.SocketID != socketB {
		t.Errorf("expected socketId %q, got %q", socketB, disconnData.SocketID)
	}

	// Alice should receive players-updated with 1 player
	playersMsg := readMessage(t, connA)
	if playersMsg.Event != "players-updated" {
		t.Fatalf("expected 'players-updated', got %q", playersMsg.Event)
	}
	var playersData struct {
		Players []models.Player `json:"players"`
	}
	json.Unmarshal(playersMsg.Data, &playersData)
	if len(playersData.Players) != 1 {
		t.Fatalf("expected 1 player remaining, got %d", len(playersData.Players))
	}
	if playersData.Players[0].Name != "Alice" {
		t.Errorf("expected remaining player 'Alice', got %q", playersData.Players[0].Name)
	}
}

func TestIntegration_Disconnect_LastPlayer(t *testing.T) {
	server, hub := setupTestServer(t)
	connA, _ := connectWS(t, server)

	// Create lobby
	sendEvent(t, connA, "create-lobby", map[string]string{"name": "Alice"})
	createMsg := readMessage(t, connA)
	var createData struct {
		LobbyID string `json:"lobbyId"`
	}
	json.Unmarshal(createMsg.Data, &createData)
	lobbyID := createData.LobbyID

	// Verify lobby exists
	if !hub.Manager.LobbyExists(lobbyID) {
		t.Fatal("lobby should exist before disconnect")
	}

	// Last player disconnects
	connA.Close()

	// Give the hub time to process the unregister
	time.Sleep(200 * time.Millisecond)

	// Lobby should be deleted
	if hub.Manager.LobbyExists(lobbyID) {
		t.Error("expected lobby to be deleted after last player disconnects")
	}
}
