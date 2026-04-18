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

func (m *Manager) CreateLobby(hostSocketID, name string) (string, []models.Player) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := generateID()
	for m.lobbies[id] != nil {
		id = generateID()
	}

	players := []models.Player{
		{SocketID: hostSocketID, Name: name, IsAlive: true},
	}

	lobby := &models.Lobby{
		ID:          id,
		Players:     players,
		HostID:      hostSocketID,
		Started:     false,
		ChatHistory: []models.ChatMessage{},
		RoleConfig:  make(models.RoleConfig),
	}

	m.lobbies[id] = lobby
	m.playerLobby[hostSocketID] = id

	result := make([]models.Player, len(players))
	copy(result, players)
	return id, result
}

func (m *Manager) JoinLobby(lobbyID, socketID, name string) (string, []models.Player, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	lobby, ok := m.lobbies[lobbyID]
	if !ok {
		return "", nil, errors.New("lobby not found")
	}
	if lobby.Started {
		return "", nil, errors.New("game already started")
	}

	lobby.Players = append(lobby.Players, models.Player{
		SocketID: socketID,
		Name:     name,
		IsAlive:  true,
	})
	m.playerLobby[socketID] = lobbyID

	result := make([]models.Player, len(lobby.Players))
	copy(result, lobby.Players)
	return lobbyID, result, nil
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

	result := make([]models.Player, len(lobby.Players))
	copy(result, lobby.Players)
	return lobbyID, result
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
	result := make(models.RoleConfig, len(lobby.RoleConfig))
	for k, v := range lobby.RoleConfig {
		result[k] = v
	}
	return result
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
	lobby.Round = 1
	lobby.NightActions = map[string]models.NightAction{}

	result := make([]models.Player, len(lobby.Players))
	copy(result, lobby.Players)
	return result, nil
}

// SubmitNightAction validates and records a role's night action.
func (m *Manager) SubmitNightAction(lobbyID, actorID, action, targetID string, round int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	lobby, ok := m.lobbies[lobbyID]
	if !ok {
		return errors.New("lobby not found")
	}
	if !lobby.Started {
		return errors.New("game not started")
	}
	if round != lobby.Round {
		return errors.New("wrong round")
	}

	var actor, target *models.Player
	for i := range lobby.Players {
		p := &lobby.Players[i]
		if p.SocketID == actorID {
			actor = p
		}
		if p.SocketID == targetID {
			target = p
		}
	}
	if actor == nil {
		return errors.New("actor not in lobby")
	}
	if !actor.IsAlive {
		return errors.New("actor is dead")
	}
	if target == nil {
		return errors.New("target not in lobby")
	}
	if !target.IsAlive {
		return errors.New("target is dead")
	}

	switch action {
	case "kill":
		if actor.Role != "mafia" {
			return errors.New("only mafia can kill")
		}
		if target.Role == "mafia" {
			return errors.New("mafia cannot target mafia")
		}
	case "investigate":
		if actor.Role != "sheriff" {
			return errors.New("only sheriff can investigate")
		}
		if target.SocketID == actor.SocketID {
			return errors.New("sheriff cannot investigate self")
		}
	case "protect":
		if actor.Role != "medic" {
			return errors.New("only medic can protect")
		}
	default:
		return fmt.Errorf("unknown action: %s", action)
	}

	if lobby.NightActions == nil {
		lobby.NightActions = map[string]models.NightAction{}
	}
	lobby.NightActions[actorID] = models.NightAction{
		ActorID:  actorID,
		Action:   action,
		TargetID: targetID,
		Round:    round,
	}
	return nil
}

// GetPlayerRole returns the role for a given socket ID, or empty string.
func (m *Manager) GetPlayerRole(socketID string) string {
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
			return p.Role
		}
	}
	return ""
}

func (m *Manager) LobbyExists(lobbyID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, ok := m.lobbies[lobbyID]
	return ok
}

func (m *Manager) GetPlayersInLobby(lobbyID string) []models.Player {
	m.mu.RLock()
	defer m.mu.RUnlock()

	lobby, ok := m.lobbies[lobbyID]
	if !ok {
		return nil
	}
	result := make([]models.Player, len(lobby.Players))
	copy(result, lobby.Players)
	return result
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
	result := make([]models.ChatMessage, len(lobby.ChatHistory))
	copy(result, lobby.ChatHistory)
	return result
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
