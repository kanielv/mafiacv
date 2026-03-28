package lobby

import (
	"testing"
)

func TestCreateLobby(t *testing.T) {
	mgr := NewManager()
	lobby := mgr.CreateLobby("host-1", "Alice")

	// Lobby should have a non-empty ID
	if lobby.ID == "" {
		t.Fatal("expected lobby to have an ID")
	}

	// Host should be the only player
	if len(lobby.Players) != 1 {
		t.Fatalf("expected 1 player, got %d", len(lobby.Players))
	}
	if lobby.Players[0].SocketID != "host-1" {
		t.Errorf("expected socket ID host-1, got %s", lobby.Players[0].SocketID)
	}
	if lobby.Players[0].Name != "Alice" {
		t.Errorf("expected name Alice, got %s", lobby.Players[0].Name)
	}
	if !lobby.Players[0].IsAlive {
		t.Error("expected player to be alive")
	}

	// Lobby metadata
	if lobby.HostID != "host-1" {
		t.Errorf("expected host ID host-1, got %s", lobby.HostID)
	}
	if lobby.Started {
		t.Error("expected lobby to not be started")
	}

	// Lobby should be retrievable via GetLobby
	found, ok := mgr.GetLobby(lobby.ID)
	if !ok {
		t.Fatal("expected lobby to be found in manager")
	}
	if found.ID != lobby.ID {
		t.Errorf("expected lobby ID %s, got %s", lobby.ID, found.ID)
	}

	// Reverse index: player should map to this lobby
	lobbyID, ok := mgr.GetPlayerLobbyID("host-1")
	if !ok || lobbyID != lobby.ID {
		t.Errorf("expected player lobby ID %s, got %s", lobby.ID, lobbyID)
	}
}

func TestJoinLobby(t *testing.T) {
	mgr := NewManager()
	lobby := mgr.CreateLobby("host-1", "Alice")

	mgr.JoinLobby(lobby.ID, "player-1", "Bob")

	// Lobby should have a non-empty ID
	if lobby.ID == "" {
		t.Fatal("expected lobby to have an ID")
	}

	// There should be two players
	if len(lobby.Players) != 2 {
		t.Fatalf("expected 2 players, got %d", len(lobby.Players))
	}

	// Alice
	if lobby.Players[0].SocketID != "host-1" {
		t.Errorf("expected socket ID host-1, got %s", lobby.Players[0].SocketID)
	}
	if lobby.Players[0].Name != "Alice" {
		t.Errorf("expected name Alice, got %s", lobby.Players[0].Name)
	}
	if !lobby.Players[0].IsAlive {
		t.Error("expected player to be alive")
	}

	// Bob
	if lobby.Players[1].SocketID != "player-1" {
		t.Errorf("expected socket ID player-1, got %s", lobby.Players[1].SocketID)
	}
	if lobby.Players[1].Name != "Bob" {
		t.Errorf("expected name Bob, got %s", lobby.Players[1].Name)
	}
	if !lobby.Players[1].IsAlive {
		t.Error("expected player to be alive")
	}

	// Reverse index: player should map to this lobby
	lobbyID, ok := mgr.GetPlayerLobbyID("host-1")
	if !ok || lobbyID != lobby.ID {
		t.Errorf("expected player lobby ID %s, got %s", lobby.ID, lobbyID)
	}

}

func TestJoinLobby_NotFound(t *testing.T) {
	mgr := NewManager()

	_, err := mgr.JoinLobby("fake-lobby", "player-1", "Alice")
	if err == nil {
		t.Errorf("expected join error, got %s", err)
	}

}

func TestJoinLobby_AlreadyStarted(t *testing.T) {
	mgr := NewManager()

	lobby := mgr.CreateLobby("host-1", "Alice")

	mgr.JoinLobby(lobby.ID, "player-1", "Bob")
}
