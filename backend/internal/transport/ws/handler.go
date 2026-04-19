package ws

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/kanielv/mafiacv/backend/internal/lobby"
	"github.com/kanielv/mafiacv/backend/internal/models"
	"github.com/kanielv/mafiacv/backend/internal/storyclient"
)

const storyIntroTimeout = 60 * time.Second
const defaultStoryTheme = "classic noir"
const phaseSeconds = 10
const gameEndNarrationDelay = 20 * time.Second

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
	case "end-discussion":
		h.handleEndDiscussion(client, msg.Data)
	case "nominate":
		h.handleNominate(client, msg.Data)
	case "submit-vote":
		h.handleSubmitVote(client, msg.Data)
	case "end-vote-recap":
		h.handleEndVoteRecap(client, msg.Data)
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
		h.SendToClient(client.ID, MarshalMessage("sheriff-result", map[string]any{
			"targetId": payload.TargetID,
			"role":     sheriffRevealedRole(h.Manager.GetPlayerRole(payload.TargetID)),
			"round":    payload.Round,
		}))

	}

	h.SendToClient(client.ID, MarshalMessage("action-acknowledged", map[string]any{
		"action": payload.Action,
		"round":  payload.Round,
	}))

	if !h.Manager.AllNightActionsSubmitted(payload.LobbyID) {
		return
	}

	resolution, players, err := h.Manager.ResolveNight(payload.LobbyID)
	if err != nil {
		log.Printf("resolve night for %s: %v", payload.LobbyID, err)
		return
	}

	// Order matters: send phase-changed before players-updated so the client
	// can snapshot pre-death alive state on day transition and reveal deaths
	// after the night recap finishes displaying.
	h.BroadcastToRoom(payload.LobbyID, MarshalMessage("phase-changed", map[string]any{
		"phase": "day",
		"round": resolution.Round,
	}))
	h.BroadcastToRoom(payload.LobbyID, MarshalMessage("players-updated", map[string]any{
		"players": sanitizePlayers(players),
	}))

	go h.generateNightRecap(payload.LobbyID, resolution, players, h.Manager.GetRoleConfig(payload.LobbyID))

	if outcome, err := h.Manager.CheckWinCondition(payload.LobbyID); err == nil && outcome.Winner != "" {
		go h.endGameFlow(payload.LobbyID, outcome.Winner, "night_recap")
	}
}

func (h *Hub) generateNightRecap(lobbyID string, res lobby.NightResolution, players []models.Player, roles models.RoleConfig) {
	if h.Story == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), storyIntroTimeout)
	defer cancel()

	storyPlayers := make([]storyclient.Player, 0, len(players))
	nameOf := make(map[string]string, len(players))
	for _, p := range players {
		storyPlayers = append(storyPlayers, storyclient.Player{Name: p.Name, IsAlive: p.IsAlive})
		nameOf[p.SocketID] = p.Name
	}

	var events []storyclient.Event
	if res.KilledID != "" {
		events = append(events, storyclient.Event{
			EventType: "kill", Target: res.KilledName, Result: "died",
		})
	} else if res.KillBlocked {
		events = append(events, storyclient.Event{
			EventType: "kill", Target: nameOf[res.ProtectedID], Result: "saved",
		})
	}
	if res.SheriffActorID != "" {
		events = append(events, storyclient.Event{
			EventType: "investigate",
			Actor:     nameOf[res.SheriffActorID],
			Target:    nameOf[res.SheriffTargetID],
			Result:    sheriffRevealedRole(res.SheriffRole),
		})
	}

	resp, err := h.Story.GenerateStory(ctx, storyclient.GenerateRequest{
		LobbyID:    lobbyID,
		StoryType:  "night_recap",
		Round:      res.Round,
		Events:     events,
		Players:    storyPlayers,
		RoleConfig: map[string]int(roles),
	})
	if err != nil {
		log.Printf("story generate night_recap for %s: %v", lobbyID, err)
		return
	}

	h.BroadcastToRoom(lobbyID, MarshalMessage("story-narration", map[string]any{
		"storyType": resp.StoryType,
		"story":     resp.Story,
		"round":     resp.Round,
	}))
}

// sheriffRevealedRole masks non-mafia roles as "town" so the sheriff's
// investigation only distinguishes mafia from everyone else.
func sheriffRevealedRole(role string) string {
	if role == "mafia" {
		return "mafia"
	}
	return "town"
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

// handleEndDiscussion is sent by the host client when the day discussion
// timer expires. The server transitions into the nomination phase and
// chains the rest of the day-voting loop server-side.
func (h *Hub) handleEndDiscussion(client *Client, data json.RawMessage) {
	var payload struct {
		LobbyID string `json:"lobbyId"`
	}
	if err := json.Unmarshal(data, &payload); err != nil || payload.LobbyID == "" {
		return
	}
	if !h.Manager.IsHost(payload.LobbyID, client.ID) {
		return
	}

	phase, _, ok := h.Manager.GetPhaseAndRound(payload.LobbyID)
	if !ok || phase != "day" {
		// Idempotent: ignore if we already progressed past discussion.
		return
	}

	round, err := h.Manager.StartNomination(payload.LobbyID)
	if err != nil {
		log.Printf("start nomination for %s: %v", payload.LobbyID, err)
		return
	}

	h.BroadcastToRoom(payload.LobbyID, MarshalMessage("phase-changed", map[string]any{
		"phase": "nomination",
		"round": round,
	}))

	go h.nominationTimer(payload.LobbyID, round)
}

func (h *Hub) handleNominate(client *Client, data json.RawMessage) {
	var payload struct {
		LobbyID  string `json:"lobbyId"`
		TargetID string `json:"targetId"`
		Round    int    `json:"round"`
	}
	if err := json.Unmarshal(data, &payload); err != nil || payload.LobbyID == "" || payload.TargetID == "" {
		h.SendToClient(client.ID, MarshalMessage("error", map[string]string{
			"message": "lobbyId and targetId are required",
		}))
		return
	}

	if err := h.Manager.SubmitNomination(payload.LobbyID, client.ID, payload.TargetID, payload.Round); err != nil {
		h.SendToClient(client.ID, MarshalMessage("error", map[string]string{
			"message": err.Error(),
		}))
		return
	}

	h.SendToClient(client.ID, MarshalMessage("nomination-acknowledged", map[string]any{
		"targetId": payload.TargetID,
		"round":    payload.Round,
	}))

	if h.Manager.AllNominationsSubmitted(payload.LobbyID) {
		h.resolveNomination(payload.LobbyID, payload.Round)
	}
}

func (h *Hub) handleSubmitVote(client *Client, data json.RawMessage) {
	var payload struct {
		LobbyID string `json:"lobbyId"`
		Vote    bool   `json:"vote"`
		Round   int    `json:"round"`
	}
	if err := json.Unmarshal(data, &payload); err != nil || payload.LobbyID == "" {
		h.SendToClient(client.ID, MarshalMessage("error", map[string]string{
			"message": "lobbyId is required",
		}))
		return
	}

	if err := h.Manager.SubmitDayVote(payload.LobbyID, client.ID, payload.Vote, payload.Round); err != nil {
		h.SendToClient(client.ID, MarshalMessage("error", map[string]string{
			"message": err.Error(),
		}))
		return
	}

	h.SendToClient(client.ID, MarshalMessage("vote-acknowledged", map[string]any{
		"vote":  payload.Vote,
		"round": payload.Round,
	}))

	if h.Manager.AllDayVotesSubmitted(payload.LobbyID) {
		h.resolveDayVote(payload.LobbyID, payload.Round)
	}
}

// nominationTimer force-resolves the nomination phase after phaseSeconds if
// it hasn't already been resolved by all-submitted.
func (h *Hub) nominationTimer(lobbyID string, round int) {
	time.Sleep(phaseSeconds * time.Second)
	phase, curRound, ok := h.Manager.GetPhaseAndRound(lobbyID)
	if !ok || phase != "nomination" || curRound != round {
		return
	}
	h.resolveNomination(lobbyID, round)
}

func (h *Hub) resolveNomination(lobbyID string, round int) {
	phase, curRound, ok := h.Manager.GetPhaseAndRound(lobbyID)
	if !ok || phase != "nomination" || curRound != round {
		return
	}

	nomineeID, nomineeName, hasVotes, err := h.Manager.ResolveNomination(lobbyID)
	if err != nil {
		log.Printf("resolve nomination for %s: %v", lobbyID, err)
		return
	}

	if !hasVotes {
		// No nominations: skip to next night.
		newRound, _, err := h.Manager.AdvanceToNightFromNomination(lobbyID)
		if err != nil {
			log.Printf("advance to night (no nominations) for %s: %v", lobbyID, err)
			return
		}
		h.BroadcastToRoom(lobbyID, MarshalMessage("phase-changed", map[string]any{
			"phase":    "night",
			"round":    newRound,
			"skipVote": true,
			"reason":   "no-nominations",
		}))
		return
	}

	h.BroadcastToRoom(lobbyID, MarshalMessage("phase-changed", map[string]any{
		"phase":       "defense",
		"round":       round,
		"nomineeId":   nomineeID,
		"nomineeName": nomineeName,
	}))

	go h.defenseTimer(lobbyID, round)
}

func (h *Hub) defenseTimer(lobbyID string, round int) {
	time.Sleep(phaseSeconds * time.Second)
	phase, curRound, ok := h.Manager.GetPhaseAndRound(lobbyID)
	if !ok || phase != "defense" || curRound != round {
		return
	}

	newRound, nomineeID, nomineeName, err := h.Manager.EndDefense(lobbyID)
	if err != nil {
		log.Printf("end defense for %s: %v", lobbyID, err)
		return
	}

	h.BroadcastToRoom(lobbyID, MarshalMessage("phase-changed", map[string]any{
		"phase":       "vote",
		"round":       newRound,
		"nomineeId":   nomineeID,
		"nomineeName": nomineeName,
	}))

	go h.voteTimer(lobbyID, newRound)
}

func (h *Hub) voteTimer(lobbyID string, round int) {
	time.Sleep(phaseSeconds * time.Second)
	phase, curRound, ok := h.Manager.GetPhaseAndRound(lobbyID)
	if !ok || phase != "vote" || curRound != round {
		return
	}
	h.resolveDayVote(lobbyID, round)
}

func (h *Hub) resolveDayVote(lobbyID string, round int) {
	phase, curRound, ok := h.Manager.GetPhaseAndRound(lobbyID)
	if !ok || phase != "vote" || curRound != round {
		return
	}

	res, players, err := h.Manager.ResolveDayVote(lobbyID)
	if err != nil {
		log.Printf("resolve day vote for %s: %v", lobbyID, err)
		return
	}

	// Transition into vote-recap; the client reads the narration on that
	// screen and the host advances to the next night when it's done.
	h.BroadcastToRoom(lobbyID, MarshalMessage("phase-changed", map[string]any{
		"phase": "vote-recap",
		"round": round,
		"voteResult": map[string]any{
			"nomineeId":   res.NomineeID,
			"nomineeName": res.NomineeName,
			"eliminated":  res.Eliminated,
			"yesVotes":    res.YesVotes,
			"noVotes":     res.NoVotes,
		},
	}))

	if res.Eliminated {
		h.BroadcastToRoom(lobbyID, MarshalMessage("players-updated", map[string]any{
			"players": sanitizePlayers(players),
		}))
	}

	go h.generateVoteRecap(lobbyID, res, players, h.Manager.GetRoleConfig(lobbyID))

	if outcome, err := h.Manager.CheckWinCondition(lobbyID); err == nil && outcome.Winner != "" {
		go h.endGameFlow(lobbyID, outcome.Winner, "vote_recap")
	}
}

func (h *Hub) handleEndVoteRecap(client *Client, data json.RawMessage) {
	var payload struct {
		LobbyID string `json:"lobbyId"`
	}
	if err := json.Unmarshal(data, &payload); err != nil || payload.LobbyID == "" {
		return
	}
	if !h.Manager.IsHost(payload.LobbyID, client.ID) {
		return
	}

	phase, _, ok := h.Manager.GetPhaseAndRound(payload.LobbyID)
	if !ok || phase != "vote-recap" {
		return
	}

	newRound, err := h.Manager.AdvanceToNightFromVoteRecap(payload.LobbyID)
	if err != nil {
		log.Printf("advance to night from vote-recap for %s: %v", payload.LobbyID, err)
		return
	}

	h.BroadcastToRoom(payload.LobbyID, MarshalMessage("phase-changed", map[string]any{
		"phase": "night",
		"round": newRound,
	}))
}

func (h *Hub) generateVoteRecap(lobbyID string, res lobby.VoteResolution, players []models.Player, roles models.RoleConfig) {
	if h.Story == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), storyIntroTimeout)
	defer cancel()

	storyPlayers := make([]storyclient.Player, 0, len(players))
	for _, p := range players {
		storyPlayers = append(storyPlayers, storyclient.Player{Name: p.Name, IsAlive: p.IsAlive})
	}

	result := "survived"
	if res.Eliminated {
		result = "eliminated"
	}

	resp, err := h.Story.GenerateStory(ctx, storyclient.GenerateRequest{
		LobbyID:   lobbyID,
		StoryType: "vote_recap",
		Round:     res.Round,
		Events: []storyclient.Event{{
			EventType: "eliminate",
			Target:    res.NomineeName,
			Result:    result,
		}},
		Players:    storyPlayers,
		RoleConfig: map[string]int(roles),
	})
	if err != nil {
		log.Printf("story generate vote_recap for %s: %v", lobbyID, err)
		return
	}

	h.BroadcastToRoom(lobbyID, MarshalMessage("story-narration", map[string]any{
		"storyType": resp.StoryType,
		"story":     resp.Story,
		"round":     resp.Round,
	}))
}

// endGameFlow runs after a terminal night_recap or vote_recap: it marks the
// lobby ended, waits for the preceding recap narration to play on clients,
// generates and broadcasts the final ending narration, emits a game-ended
// event, and tears down the lobby and its story-service state.
//
// afterNarration is the StoryType of the recap that was just broadcast — the
// flow waits gameEndNarrationDelay so that recap finishes before the ending
// plays on the client.
func (h *Hub) endGameFlow(lobbyID, winner, afterNarration string) {
	finalPlayers, err := h.Manager.EndGame(lobbyID, winner)
	if err != nil {
		log.Printf("end game for %s: %v", lobbyID, err)
		return
	}
	if finalPlayers == nil {
		// Already ended — another trigger won the race.
		return
	}

	// Let the preceding recap narration play before overlaying the ending.
	_ = afterNarration
	time.Sleep(gameEndNarrationDelay)

	round := 0
	if _, r, ok := h.Manager.GetPhaseAndRound(lobbyID); ok {
		round = r
	}

	if h.Story != nil {
		ctx, cancel := context.WithTimeout(context.Background(), storyIntroTimeout)
		storyPlayers := make([]storyclient.Player, 0, len(finalPlayers))
		for _, p := range finalPlayers {
			storyPlayers = append(storyPlayers, storyclient.Player{
				Name:    p.Name,
				IsAlive: p.IsAlive,
				Role:    p.Role,
			})
		}
		resp, err := h.Story.GenerateStory(ctx, storyclient.GenerateRequest{
			LobbyID:    lobbyID,
			StoryType:  "game_ending",
			Round:      round,
			Events:     []storyclient.Event{{EventType: "game_ending", Result: winner}},
			Players:    storyPlayers,
			RoleConfig: map[string]int(h.Manager.GetRoleConfig(lobbyID)),
		})
		cancel()
		if err != nil {
			log.Printf("story generate game_ending for %s: %v", lobbyID, err)
		} else {
			h.BroadcastToRoom(lobbyID, MarshalMessage("story-narration", map[string]any{
				"storyType": resp.StoryType,
				"story":     resp.Story,
				"round":     resp.Round,
			}))
			// Hold so clients can play the ending before results broadcast.
			time.Sleep(gameEndNarrationDelay)
		}
	}

	h.BroadcastToRoom(lobbyID, MarshalMessage("game-ended", map[string]any{
		"winner":  winner,
		"players": sanitizePlayers(finalPlayers),
	}))

	if h.Story != nil {
		ctx, cancel := context.WithTimeout(context.Background(), storyIntroTimeout)
		if err := h.Story.CleanupGame(ctx, lobbyID); err != nil {
			log.Printf("story cleanup for %s: %v", lobbyID, err)
		}
		cancel()
	}

	h.Manager.DeleteLobby(lobbyID)
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
