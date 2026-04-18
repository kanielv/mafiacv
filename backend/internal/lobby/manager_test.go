package lobby

import (
	"testing"

	"github.com/kanielv/mafiacv/backend/internal/models"
)

func TestCreateLobby(t *testing.T) {
	mgr := NewManager()
	lobbyID, players := mgr.CreateLobby("host-1", "Alice")

	// Lobby should have a non-empty ID
	if lobbyID == "" {
		t.Fatal("expected lobby to have an ID")
	}

	// Host should be the only player
	if len(players) != 1 {
		t.Fatalf("expected 1 player, got %d", len(players))
	}
	if players[0].SocketID != "host-1" {
		t.Errorf("expected socket ID host-1, got %s", players[0].SocketID)
	}
	if players[0].Name != "Alice" {
		t.Errorf("expected name Alice, got %s", players[0].Name)
	}
	if !players[0].IsAlive {
		t.Error("expected player to be alive")
	}

	// Lobby should be retrievable
	if !mgr.LobbyExists(lobbyID) {
		t.Fatal("expected lobby to be found in manager")
	}

	// Reverse index: player should map to this lobby
	playerLobbyID, ok := mgr.GetPlayerLobbyID("host-1")
	if !ok || playerLobbyID != lobbyID {
		t.Errorf("expected player lobby ID %s, got %s", lobbyID, playerLobbyID)
	}
}

func TestJoinLobby(t *testing.T) {
	mgr := NewManager()
	lobbyID, _ := mgr.CreateLobby("host-1", "Alice")

	_, players, err := mgr.JoinLobby(lobbyID, "player-1", "Bob")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// There should be two players
	if len(players) != 2 {
		t.Fatalf("expected 2 players, got %d", len(players))
	}

	// Alice
	if players[0].SocketID != "host-1" {
		t.Errorf("expected socket ID host-1, got %s", players[0].SocketID)
	}
	if players[0].Name != "Alice" {
		t.Errorf("expected name Alice, got %s", players[0].Name)
	}
	if !players[0].IsAlive {
		t.Error("expected player to be alive")
	}

	// Bob
	if players[1].SocketID != "player-1" {
		t.Errorf("expected socket ID player-1, got %s", players[1].SocketID)
	}
	if players[1].Name != "Bob" {
		t.Errorf("expected name Bob, got %s", players[1].Name)
	}
	if !players[1].IsAlive {
		t.Error("expected player to be alive")
	}

	// Reverse index: player should map to this lobby
	playerLobbyID, ok := mgr.GetPlayerLobbyID("host-1")
	if !ok || playerLobbyID != lobbyID {
		t.Errorf("expected player lobby ID %s, got %s", lobbyID, playerLobbyID)
	}

}

func TestJoinLobby_NotFound(t *testing.T) {
	mgr := NewManager()

	_, _, err := mgr.JoinLobby("fake-lobby", "player-1", "Alice")
	if err == nil {
		t.Error("expected join error, got nil")
	}

}

func TestJoinLobby_AlreadyStarted(t *testing.T) {
	mgr := NewManager()

	lobbyID, _ := mgr.CreateLobby("host-1", "Alice")

	_, _, err := mgr.JoinLobby(lobbyID, "player-1", "Bob")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// setupNightLobby creates a started lobby with hand-assigned roles so tests
// can exercise SubmitNightAction without fighting the Fisher–Yates shuffle.
func setupNightLobby(t *testing.T) (*Manager, string) {
	t.Helper()
	mgr := NewManager()
	lobbyID, _ := mgr.CreateLobby("mafia-1", "M1")
	mgr.JoinLobby(lobbyID, "mafia-2", "M2")
	mgr.JoinLobby(lobbyID, "sheriff-1", "S")
	mgr.JoinLobby(lobbyID, "medic-1", "D")
	mgr.JoinLobby(lobbyID, "town-1", "T")

	lobby := mgr.lobbies[lobbyID]
	lobby.Started = true
	lobby.Round = 1
	lobby.NightActions = map[string]models.NightAction{}
	roleByID := map[string]string{
		"mafia-1":   "mafia",
		"mafia-2":   "mafia",
		"sheriff-1": "sheriff",
		"medic-1":   "medic",
		"town-1":    "town",
	}
	for i := range lobby.Players {
		lobby.Players[i].Role = roleByID[lobby.Players[i].SocketID]
	}
	return mgr, lobbyID
}

func TestSubmitNightAction_MafiaCannotKillMafia(t *testing.T) {
	mgr, id := setupNightLobby(t)
	if err := mgr.SubmitNightAction(id, "mafia-1", "kill", "mafia-2", 1); err == nil {
		t.Error("expected error targeting fellow mafia")
	}
}

func TestSubmitNightAction_SheriffCannotInvestigateSelf(t *testing.T) {
	mgr, id := setupNightLobby(t)
	if err := mgr.SubmitNightAction(id, "sheriff-1", "investigate", "sheriff-1", 1); err == nil {
		t.Error("expected error investigating self")
	}
}

func TestSubmitNightAction_WrongRoleRejected(t *testing.T) {
	mgr, id := setupNightLobby(t)
	if err := mgr.SubmitNightAction(id, "town-1", "kill", "sheriff-1", 1); err == nil {
		t.Error("expected error: town submitted kill")
	}
}

func TestSubmitNightAction_DeadActorRejected(t *testing.T) {
	mgr, id := setupNightLobby(t)
	lobby := mgr.lobbies[id]
	for i := range lobby.Players {
		if lobby.Players[i].SocketID == "mafia-1" {
			lobby.Players[i].IsAlive = false
		}
	}
	if err := mgr.SubmitNightAction(id, "mafia-1", "kill", "town-1", 1); err == nil {
		t.Error("expected error for dead actor")
	}
}

func TestSubmitNightAction_DeadTargetRejected(t *testing.T) {
	mgr, id := setupNightLobby(t)
	lobby := mgr.lobbies[id]
	for i := range lobby.Players {
		if lobby.Players[i].SocketID == "town-1" {
			lobby.Players[i].IsAlive = false
		}
	}
	if err := mgr.SubmitNightAction(id, "mafia-1", "kill", "town-1", 1); err == nil {
		t.Error("expected error for dead target")
	}
}

func TestSubmitNightAction_WrongRoundRejected(t *testing.T) {
	mgr, id := setupNightLobby(t)
	if err := mgr.SubmitNightAction(id, "mafia-1", "kill", "town-1", 2); err == nil {
		t.Error("expected error for wrong round")
	}
}

func TestSubmitNightAction_OverwriteAllowed(t *testing.T) {
	mgr, id := setupNightLobby(t)
	if err := mgr.SubmitNightAction(id, "mafia-1", "kill", "town-1", 1); err != nil {
		t.Fatalf("first kill failed: %v", err)
	}
	if err := mgr.SubmitNightAction(id, "mafia-1", "kill", "sheriff-1", 1); err != nil {
		t.Fatalf("overwrite kill failed: %v", err)
	}
	got := mgr.lobbies[id].NightActions["mafia-1"].TargetID
	if got != "sheriff-1" {
		t.Errorf("expected overwrite to sheriff-1, got %s", got)
	}
}
