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
	lobby.Phase = "intro"
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

// NightResolution describes the outcome of a night phase.
type NightResolution struct {
	Round           int
	KilledID        string
	KilledName      string
	ProtectedID     string
	KillBlocked     bool
	SheriffActorID  string
	SheriffTargetID string
	SheriffRole     string
}

func nightActorRole(role string) bool {
	return role == "mafia" || role == "sheriff" || role == "medic"
}

func (m *Manager) eligibleNightActorsLocked(lobby *models.Lobby) []*models.Player {
	out := make([]*models.Player, 0)
	for i := range lobby.Players {
		p := &lobby.Players[i]
		if p.IsAlive && nightActorRole(p.Role) {
			out = append(out, p)
		}
	}
	return out
}

// AllNightActionsSubmitted reports whether every alive mafia/sheriff/medic
// has an entry in NightActions for the current round.
func (m *Manager) AllNightActionsSubmitted(lobbyID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	lobby, ok := m.lobbies[lobbyID]
	if !ok || !lobby.Started {
		return false
	}
	actors := m.eligibleNightActorsLocked(lobby)
	if len(actors) == 0 {
		return false
	}
	for _, p := range actors {
		na, ok := lobby.NightActions[p.SocketID]
		if !ok || na.Round != lobby.Round {
			return false
		}
	}
	return true
}

// ResolveNight applies night actions, marks any death, advances phase to "day",
// and clears NightActions. Returns the resolution and the post-resolution
// player list.
func (m *Manager) ResolveNight(lobbyID string) (NightResolution, []models.Player, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	lobby, ok := m.lobbies[lobbyID]
	if !ok {
		return NightResolution{}, nil, errors.New("lobby not found")
	}
	if !lobby.Started {
		return NightResolution{}, nil, errors.New("game not started")
	}

	res := NightResolution{Round: lobby.Round}

	roleOf := make(map[string]string, len(lobby.Players))
	nameOf := make(map[string]string, len(lobby.Players))
	for _, p := range lobby.Players {
		roleOf[p.SocketID] = p.Role
		nameOf[p.SocketID] = p.Name
	}

	killVotes := map[string]int{}
	var orderedTargets []string
	var protectedID string
	for _, na := range lobby.NightActions {
		if na.Round != lobby.Round {
			continue
		}
		switch na.Action {
		case "kill":
			if roleOf[na.ActorID] != "mafia" {
				continue
			}
			if _, seen := killVotes[na.TargetID]; !seen {
				orderedTargets = append(orderedTargets, na.TargetID)
			}
			killVotes[na.TargetID]++
		case "protect":
			if roleOf[na.ActorID] == "medic" {
				protectedID = na.TargetID
			}
		case "investigate":
			if roleOf[na.ActorID] == "sheriff" {
				res.SheriffActorID = na.ActorID
				res.SheriffTargetID = na.TargetID
				res.SheriffRole = roleOf[na.TargetID]
			}
		}
	}

	killedID := pickWeighted(orderedTargets, killVotes)
	res.ProtectedID = protectedID
	if killedID != "" {
		if killedID == protectedID {
			res.KillBlocked = true
		} else {
			for i := range lobby.Players {
				if lobby.Players[i].SocketID == killedID {
					lobby.Players[i].IsAlive = false
					res.KilledID = killedID
					res.KilledName = nameOf[killedID]
					break
				}
			}
		}
	}

	lobby.Phase = "day"
	lobby.NightActions = map[string]models.NightAction{}

	out := make([]models.Player, len(lobby.Players))
	copy(out, lobby.Players)
	return res, out, nil
}

// VoteResolution describes the outcome of the day vote phase.
type VoteResolution struct {
	Round       int
	NomineeID   string
	NomineeName string
	Eliminated  bool
	YesVotes    int
	NoVotes     int
}

// StartNomination transitions the lobby into the nomination phase and clears
// any stale nominations. Returns the current round.
func (m *Manager) StartNomination(lobbyID string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	lobby, ok := m.lobbies[lobbyID]
	if !ok {
		return 0, errors.New("lobby not found")
	}
	if !lobby.Started {
		return 0, errors.New("game not started")
	}
	lobby.Phase = "nomination"
	lobby.Nominations = map[string]string{}
	lobby.NomineeID = ""
	lobby.NomineeName = ""
	return lobby.Round, nil
}

// SubmitNomination records a single nomination for the current round.
func (m *Manager) SubmitNomination(lobbyID, voterID, targetID string, round int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	lobby, ok := m.lobbies[lobbyID]
	if !ok {
		return errors.New("lobby not found")
	}
	if lobby.Phase != "nomination" {
		return errors.New("not in nomination phase")
	}
	if round != lobby.Round {
		return errors.New("wrong round")
	}

	var voter, target *models.Player
	for i := range lobby.Players {
		p := &lobby.Players[i]
		if p.SocketID == voterID {
			voter = p
		}
		if p.SocketID == targetID {
			target = p
		}
	}
	if voter == nil || !voter.IsAlive {
		return errors.New("voter not alive")
	}
	if target == nil || !target.IsAlive {
		return errors.New("target not alive")
	}

	if lobby.Nominations == nil {
		lobby.Nominations = map[string]string{}
	}
	lobby.Nominations[voterID] = targetID
	return nil
}

// AllNominationsSubmitted reports whether every alive player has nominated.
func (m *Manager) AllNominationsSubmitted(lobbyID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	lobby, ok := m.lobbies[lobbyID]
	if !ok {
		return false
	}
	for _, p := range lobby.Players {
		if !p.IsAlive {
			continue
		}
		if _, ok := lobby.Nominations[p.SocketID]; !ok {
			return false
		}
	}
	return true
}

// ResolveNomination picks the nominee (weighted-random over top tally) and
// transitions the phase to "defense". Returns anyVotes=false if nobody voted.
func (m *Manager) ResolveNomination(lobbyID string) (nomineeID, nomineeName string, anyVotes bool, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	lobby, ok := m.lobbies[lobbyID]
	if !ok {
		return "", "", false, errors.New("lobby not found")
	}
	if lobby.Phase != "nomination" {
		return "", "", false, errors.New("not in nomination phase")
	}

	tally := map[string]int{}
	var ordered []string
	for _, targetID := range lobby.Nominations {
		if _, seen := tally[targetID]; !seen {
			ordered = append(ordered, targetID)
		}
		tally[targetID]++
	}

	if len(ordered) == 0 {
		lobby.Phase = "night" // caller will advance round / reset
		return "", "", false, nil
	}

	// Keep only top-tally targets for weighted random (ties handled by random).
	top := 0
	for _, c := range tally {
		if c > top {
			top = c
		}
	}
	tied := make([]string, 0)
	tiedWeights := map[string]int{}
	for _, id := range ordered {
		if tally[id] == top {
			tied = append(tied, id)
			tiedWeights[id] = 1
		}
	}
	chosen := pickWeighted(tied, tiedWeights)

	for _, p := range lobby.Players {
		if p.SocketID == chosen {
			lobby.NomineeID = chosen
			lobby.NomineeName = p.Name
			nomineeName = p.Name
			break
		}
	}

	lobby.Phase = "defense"
	return chosen, nomineeName, true, nil
}

// StartDefense is a phase-marker (actual timing is driven by the server
// goroutine in the WS layer).
func (m *Manager) StartDefense(lobbyID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	lobby, ok := m.lobbies[lobbyID]
	if !ok {
		return errors.New("lobby not found")
	}
	lobby.Phase = "defense"
	return nil
}

// EndDefense transitions the lobby from defense to vote and clears stale
// vote entries.
func (m *Manager) EndDefense(lobbyID string) (int, string, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	lobby, ok := m.lobbies[lobbyID]
	if !ok {
		return 0, "", "", errors.New("lobby not found")
	}
	if lobby.Phase != "defense" {
		return 0, "", "", errors.New("not in defense phase")
	}
	lobby.Phase = "vote"
	lobby.DayVotes = map[string]models.DayVote{}
	return lobby.Round, lobby.NomineeID, lobby.NomineeName, nil
}

// SubmitDayVote records a yes/no vote on the current nominee.
func (m *Manager) SubmitDayVote(lobbyID, voterID string, vote bool, round int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	lobby, ok := m.lobbies[lobbyID]
	if !ok {
		return errors.New("lobby not found")
	}
	if lobby.Phase != "vote" {
		return errors.New("not in vote phase")
	}
	if round != lobby.Round {
		return errors.New("wrong round")
	}
	if voterID == lobby.NomineeID {
		return errors.New("nominee cannot vote")
	}

	var voter *models.Player
	for i := range lobby.Players {
		if lobby.Players[i].SocketID == voterID {
			voter = &lobby.Players[i]
			break
		}
	}
	if voter == nil || !voter.IsAlive {
		return errors.New("voter not alive")
	}

	if lobby.DayVotes == nil {
		lobby.DayVotes = map[string]models.DayVote{}
	}
	lobby.DayVotes[voterID] = models.DayVote{VoterID: voterID, Vote: vote, Round: round}
	return nil
}

// AllDayVotesSubmitted reports whether every alive player (except the
// nominee) has voted.
func (m *Manager) AllDayVotesSubmitted(lobbyID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	lobby, ok := m.lobbies[lobbyID]
	if !ok {
		return false
	}
	for _, p := range lobby.Players {
		if !p.IsAlive || p.SocketID == lobby.NomineeID {
			continue
		}
		if _, ok := lobby.DayVotes[p.SocketID]; !ok {
			return false
		}
	}
	return true
}

// ResolveDayVote tallies votes, eliminates the nominee on strict majority-yes,
// advances to the next night, and returns the resolution + updated player
// list.
func (m *Manager) ResolveDayVote(lobbyID string) (VoteResolution, []models.Player, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	lobby, ok := m.lobbies[lobbyID]
	if !ok {
		return VoteResolution{}, nil, errors.New("lobby not found")
	}
	if lobby.Phase != "vote" {
		return VoteResolution{}, nil, errors.New("not in vote phase")
	}

	res := VoteResolution{
		Round:       lobby.Round,
		NomineeID:   lobby.NomineeID,
		NomineeName: lobby.NomineeName,
	}
	for _, v := range lobby.DayVotes {
		if v.Round != lobby.Round {
			continue
		}
		if v.Vote {
			res.YesVotes++
		} else {
			res.NoVotes++
		}
	}

	if res.YesVotes > res.NoVotes && lobby.NomineeID != "" {
		for i := range lobby.Players {
			if lobby.Players[i].SocketID == lobby.NomineeID {
				lobby.Players[i].IsAlive = false
				res.Eliminated = true
				break
			}
		}
	}

	// Transition to vote-recap; the next-night advance happens when the
	// recap narration has played (host-driven via AdvanceToNightFromVoteRecap).
	lobby.Phase = "vote-recap"
	lobby.DayVotes = map[string]models.DayVote{}
	lobby.Nominations = map[string]string{}

	out := make([]models.Player, len(lobby.Players))
	copy(out, lobby.Players)
	return res, out, nil
}

// AdvanceToNightFromVoteRecap moves the lobby from vote-recap into the next
// night, incrementing the round and clearing nominee state.
func (m *Manager) AdvanceToNightFromVoteRecap(lobbyID string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	lobby, ok := m.lobbies[lobbyID]
	if !ok {
		return 0, errors.New("lobby not found")
	}
	if lobby.Phase != "vote-recap" {
		return 0, errors.New("not in vote-recap phase")
	}
	lobby.Phase = "night"
	lobby.Round++
	lobby.NomineeID = ""
	lobby.NomineeName = ""
	lobby.NightActions = map[string]models.NightAction{}
	return lobby.Round, nil
}

// AdvanceToNightFromNomination is used when nomination yielded no votes: skip
// defense/vote and go straight to the next night.
func (m *Manager) AdvanceToNightFromNomination(lobbyID string) (int, []models.Player, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	lobby, ok := m.lobbies[lobbyID]
	if !ok {
		return 0, nil, errors.New("lobby not found")
	}
	lobby.Phase = "night"
	lobby.Round++
	lobby.NomineeID = ""
	lobby.NomineeName = ""
	lobby.DayVotes = map[string]models.DayVote{}
	lobby.Nominations = map[string]string{}
	lobby.NightActions = map[string]models.NightAction{}

	out := make([]models.Player, len(lobby.Players))
	copy(out, lobby.Players)
	return lobby.Round, out, nil
}

// GetPhaseAndRound returns the current phase and round for a lobby.
func (m *Manager) GetPhaseAndRound(lobbyID string) (string, int, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	lobby, ok := m.lobbies[lobbyID]
	if !ok {
		return "", 0, false
	}
	return lobby.Phase, lobby.Round, true
}

// IsHost reports whether socketID is the host of the lobby.
func (m *Manager) IsHost(lobbyID, socketID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	lobby, ok := m.lobbies[lobbyID]
	if !ok {
		return false
	}
	return lobby.HostID == socketID
}

// pickWeighted picks an id from the list with probability proportional to its
// vote count. Returns "" when the list is empty.
func pickWeighted(ids []string, weights map[string]int) string {
	total := 0
	for _, id := range ids {
		total += weights[id]
	}
	if total <= 0 {
		return ""
	}
	// Use a 2-byte sample so the modulo bias on `total` (≤ ~num players) is
	// negligible. crypto/rand matches the Fisher-Yates shuffle in StartGame.
	buf := make([]byte, 2)
	rand.Read(buf)
	r := (int(buf[0])<<8 | int(buf[1])) % total
	for _, id := range ids {
		r -= weights[id]
		if r < 0 {
			return id
		}
	}
	return ids[len(ids)-1]
}

type GameOutcome struct {
	Winner     string // "mafia" | "town" | ""
	MafiaAlive int
	TownAlive  int
}

func (m *Manager) CheckWinCondition(lobbyID string) (GameOutcome, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	lobby, ok := m.lobbies[lobbyID]
	if !ok {
		return GameOutcome{}, errors.New("lobby not found")
	}

	mafiaAlive := 0
	townAlive := 0
	for i := range lobby.Players {
		p := &lobby.Players[i]
		if !p.IsAlive {
			continue
		}
		if p.Role == "mafia" {
			mafiaAlive++
		} else {
			townAlive++
		}
	}

	out := GameOutcome{MafiaAlive: mafiaAlive, TownAlive: townAlive}
	switch {
	case mafiaAlive == 0:
		out.Winner = "town"
	case mafiaAlive >= townAlive:
		out.Winner = "mafia"
	}
	return out, nil
}

// EndGame marks the lobby as ended, records the winner, and returns the
// final player snapshot for broadcast. No-op (returns nil players, nil err)
// if the lobby is already ended. Does NOT remove the lobby from the manager;
// call DeleteLobby after the ending narration / cleanup completes.
func (m *Manager) EndGame(lobbyID, winner string) ([]models.Player, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	lobby, ok := m.lobbies[lobbyID]
	if !ok {
		return nil, errors.New("lobby not found")
	}
	if lobby.Phase == "ended" {
		return nil, nil
	}

	lobby.Phase = "ended"
	lobby.Winner = winner

	out := make([]models.Player, len(lobby.Players))
	copy(out, lobby.Players)
	return out, nil
}

// DeleteLobby removes a lobby and its player reverse-map entries. Safe to
// call after EndGame once the ending narration has played out.
func (m *Manager) DeleteLobby(lobbyID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	lobby, ok := m.lobbies[lobbyID]
	if !ok {
		return
	}
	for _, p := range lobby.Players {
		delete(m.playerLobby, p.SocketID)
	}
	delete(m.lobbies, lobbyID)
}

func generateID() string {
	b := make([]byte, 3)
	rand.Read(b)
	return hex.EncodeToString(b)
}
