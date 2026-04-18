package ws

import (
	"context"
	"log"
	"sync"

	"github.com/kanielv/mafiacv/backend/internal/lobby"
	"github.com/kanielv/mafiacv/backend/internal/storyclient"
)

// StoryClient is the contract the Hub needs from the story-service client.
// Kept narrow so tests can supply a stub without real HTTP.
type StoryClient interface {
	InitGame(ctx context.Context, r storyclient.InitRequest) error
	GenerateStory(ctx context.Context, r storyclient.GenerateRequest) (storyclient.GenerateResponse, error)
	CleanupGame(ctx context.Context, lobbyID string) error
}

type Hub struct {
	Clients    map[string]*Client
	Rooms      map[string]map[string]*Client // roomID -> clientID -> Client
	Register   chan *Client
	Unregister chan *Client
	Manager    *lobby.Manager
	Story      StoryClient
	mu         sync.RWMutex
}

func NewHub(manager *lobby.Manager, story StoryClient) *Hub {
	return &Hub{
		Clients:    make(map[string]*Client),
		Rooms:      make(map[string]map[string]*Client),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
		Manager:    manager,
		Story:      story,
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.Register:
			h.mu.Lock()
			h.Clients[client.ID] = client
			h.mu.Unlock()
			log.Printf("client connected: %s", client.ID)

		case client := <-h.Unregister:
			h.mu.Lock()
			if _, ok := h.Clients[client.ID]; ok {
				delete(h.Clients, client.ID)
				close(client.Send)

				// Remove from all rooms
				for roomID, room := range h.Rooms {
					delete(room, client.ID)
					if len(room) == 0 {
						delete(h.Rooms, roomID)
					}
				}
			}
			h.mu.Unlock()

			// Handle lobby cleanup and notify remaining players
			h.handleDisconnect(client)
			log.Printf("client disconnected: %s", client.ID)
		}
	}
}

func (h *Hub) JoinRoom(clientID, roomID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.Rooms[roomID]; !ok {
		h.Rooms[roomID] = make(map[string]*Client)
	}
	if client, ok := h.Clients[clientID]; ok {
		h.Rooms[roomID][clientID] = client
	}
}

func (h *Hub) LeaveRoom(clientID, roomID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if room, ok := h.Rooms[roomID]; ok {
		delete(room, clientID)
		if len(room) == 0 {
			delete(h.Rooms, roomID)
		}
	}
}

func (h *Hub) BroadcastToRoom(roomID string, msg []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if room, ok := h.Rooms[roomID]; ok {
		for _, client := range room {
			select {
			case client.Send <- msg:
			default:
				// Client send buffer full, skip
			}
		}
	}
}

func (h *Hub) SendToClient(clientID string, msg []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if client, ok := h.Clients[clientID]; ok {
		select {
		case client.Send <- msg:
		default:
		}
	}
}
