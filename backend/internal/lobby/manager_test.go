package lobby

import (
	"testing"
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
