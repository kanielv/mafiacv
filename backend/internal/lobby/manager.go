package lobby

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"

	"github.com/kanielv/mafiacv/backend/internal/models"
)

const maxChatHistory = 100

type Manager struct {
	mu          sync.RWMutex
	lobbies     map[string]*models.Lobby
	playerLobby map[string]string // socketID -> lobbyID
}

func NewManager() *Manager {
	return &Manager{
		lobbies:     make(map[string]*models.Lobby),
		playerLobby: make(map[string]string),
	}
}

func (m *Manager) CreateLobby(hostSocketID, name string) *models.Lobby {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := generateID()
	lobby := &models.Lobby{
		ID: id,
		Players: []models.Player{
			{SocketID: hostSocketID, Name: name, IsAlive: true},
		},
		HostID:      hostSocketID,
		Started:     false,
		ChatHistory: []models.ChatMessage{},
		RoleConfig:  make(models.RoleConfig),
	}

	m.lobbies[id] = lobby
	m.playerLobby[hostSocketID] = id
	return lobby
}

func (m *Manager) JoinLobby(lobbyID, socketID, name string) (*models.Lobby, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	lobby, ok := m.lobbies[lobbyID]
	if !ok {
		return nil, errors.New("lobby not found")
	}
	if lobby.Started {
		return nil, errors.New("game already started")
	}

	lobby.Players = append(lobby.Players, models.Player{
		SocketID: socketID,
		Name:     name,
		IsAlive:  true,
	})
	m.playerLobby[socketID] = lobbyID
	return lobby, nil
}

func (m *Manager) RemovePlayer(socketID string) (lobbyID string, remaining []models.Player) {
	m.mu.Lock()
	defer m.mu.Unlock()

	lobbyID, ok := m.playerLobby[socketID]
	if !ok {
		return "", nil
	}
	delete(m.playerLobby, socketID)

	lobby, ok := m.lobbies[lobbyID]
	if !ok {
		return lobbyID, nil
	}

	// Remove player from lobby
	players := make([]models.Player, 0, len(lobby.Players))
	for _, p := range lobby.Players {
		if p.SocketID != socketID {
			players = append(players, p)
		}
	}
	lobby.Players = players

	// Delete lobby if empty
	if len(lobby.Players) == 0 {
		delete(m.lobbies, lobbyID)
		return lobbyID, nil
	}

	return lobbyID, lobby.Players
}

func (m *Manager) SetRoleConfig(lobbyID, hostID string, config models.RoleConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	lobby, ok := m.lobbies[lobbyID]
	if !ok {
		return errors.New("lobby not found")
	}
	if lobby.HostID != hostID {
		return errors.New("only the host can configure roles")
	}
	if lobby.Started {
		return errors.New("game already started")
	}

	validRoles := map[string]bool{"mafia": true, "medic": true, "sheriff": true, "jester": true}
	totalSpecial := 0
	for role, count := range config {
		if !validRoles[role] {
			return fmt.Errorf("unknown role: %s", role)
		}
		if count < 0 {
			return fmt.Errorf("negative count for role: %s", role)
		}
		totalSpecial += count
	}

	if totalSpecial > len(lobby.Players) {
		return errors.New("total special roles exceed player count")
	}

	lobby.RoleConfig = config
	return nil
}

func (m *Manager) GetRoleConfig(lobbyID string) models.RoleConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	lobby, ok := m.lobbies[lobbyID]
	if !ok {
		return nil
	}
	return lobby.RoleConfig
}

func (m *Manager) StartGame(lobbyID string) ([]models.Player, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	lobby, ok := m.lobbies[lobbyID]
	if !ok {
		return nil, errors.New("lobby not found")
	}
	if lobby.Started {
		return nil, errors.New("game already started")
	}

	config := lobby.RoleConfig
	if config == nil {
		config = make(models.RoleConfig)
	}

	// Default mafia count if not set
	if config["mafia"] == 0 {
		mafiaCount := len(lobby.Players) / 3
		if mafiaCount < 1 {
			mafiaCount = 1
		}
		config["mafia"] = mafiaCount
	}

	// Build role list from config
	var roles []string
	for role, count := range config {
		for i := 0; i < count; i++ {
			roles = append(roles, role)
		}
	}

	if len(roles) > len(lobby.Players) {
		return nil, errors.New("more special roles than players")
	}

	// Fill remaining with town
	for len(roles) < len(lobby.Players) {
		roles = append(roles, "town")
	}

	// Shuffle roles using Fisher-Yates with crypto/rand
	for i := len(roles) - 1; i > 0; i-- {
		jBytes := make([]byte, 1)
		rand.Read(jBytes)
		j := int(jBytes[0]) % (i + 1)
		roles[i], roles[j] = roles[j], roles[i]
	}

	// Assign roles to players
	for i := range lobby.Players {
		lobby.Players[i].Role = roles[i]
	}

	lobby.Started = true

	result := make([]models.Player, len(lobby.Players))
	copy(result, lobby.Players)
	return result, nil
}

func (m *Manager) GetLobby(lobbyID string) (*models.Lobby, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	lobby, ok := m.lobbies[lobbyID]
	return lobby, ok
}

func (m *Manager) GetPlayersInLobby(lobbyID string) []models.Player {
	m.mu.RLock()
	defer m.mu.RUnlock()

	lobby, ok := m.lobbies[lobbyID]
	if !ok {
		return nil
	}
	return lobby.Players
}

func (m *Manager) AddChatMessage(lobbyID string, msg models.ChatMessage) {
	m.mu.Lock()
	defer m.mu.Unlock()

	lobby, ok := m.lobbies[lobbyID]
	if !ok {
		return
	}

	lobby.ChatHistory = append(lobby.ChatHistory, msg)
	if len(lobby.ChatHistory) > maxChatHistory {
		lobby.ChatHistory = lobby.ChatHistory[len(lobby.ChatHistory)-maxChatHistory:]
	}
}

func (m *Manager) GetChatHistory(lobbyID string) []models.ChatMessage {
	m.mu.RLock()
	defer m.mu.RUnlock()

	lobby, ok := m.lobbies[lobbyID]
	if !ok {
		return nil
	}
	return lobby.ChatHistory
}

// GetPlayerName returns the name of a player by socket ID.
func (m *Manager) GetPlayerName(socketID string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	lobbyID, ok := m.playerLobby[socketID]
	if !ok {
		return ""
	}
	lobby, ok := m.lobbies[lobbyID]
	if !ok {
		return ""
	}
	for _, p := range lobby.Players {
		if p.SocketID == socketID {
			return p.Name
		}
	}
	return ""
}

// GetPlayerLobbyID returns the lobby ID for a given socket ID.
func (m *Manager) GetPlayerLobbyID(socketID string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	id, ok := m.playerLobby[socketID]
	return id, ok
}

func generateID() string {
	b := make([]byte, 3)
	rand.Read(b)
	return hex.EncodeToString(b)
}
