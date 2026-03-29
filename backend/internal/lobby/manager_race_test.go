package lobby

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"
)

// TestJoinLobbyRace proves that the race condition is fixed.
// Run with: go test -race ./internal/lobby/
func TestJoinLobbyRace(t *testing.T) {
	mgr := NewManager()
	lobbyID, _ := mgr.CreateLobby("host-1", "Host")

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			_, players, _ := mgr.JoinLobby(lobbyID, fmt.Sprintf("p-%d", n), fmt.Sprintf("Player%d", n))
			if players != nil {
				// This reads the returned players copy — should be safe now
				_, _ = json.Marshal(players)
			}
		}(i)
	}
	wg.Wait()
}
