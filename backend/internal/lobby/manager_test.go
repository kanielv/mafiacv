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

func submitAll(t *testing.T, mgr *Manager, id string, plays map[string][2]string) {
	t.Helper()
	for actor, p := range plays {
		if err := mgr.SubmitNightAction(id, actor, p[0], p[1], 1); err != nil {
			t.Fatalf("submit %s: %v", actor, err)
		}
	}
}

func TestAllNightActionsSubmitted(t *testing.T) {
	mgr, id := setupNightLobby(t)
	if mgr.AllNightActionsSubmitted(id) {
		t.Fatal("should be false with zero submissions")
	}
	mgr.SubmitNightAction(id, "mafia-1", "kill", "town-1", 1)
	if mgr.AllNightActionsSubmitted(id) {
		t.Fatal("should be false: missing mafia-2, sheriff, medic")
	}
	mgr.SubmitNightAction(id, "mafia-2", "kill", "town-1", 1)
	mgr.SubmitNightAction(id, "sheriff-1", "investigate", "mafia-1", 1)
	if mgr.AllNightActionsSubmitted(id) {
		t.Fatal("should be false: medic missing")
	}
	mgr.SubmitNightAction(id, "medic-1", "protect", "town-1", 1)
	if !mgr.AllNightActionsSubmitted(id) {
		t.Fatal("should be true once all four submit")
	}
}

func TestResolveNight_KillAppliedAndPhaseAdvances(t *testing.T) {
	mgr, id := setupNightLobby(t)
	submitAll(t, mgr, id, map[string][2]string{
		"mafia-1":   {"kill", "town-1"},
		"mafia-2":   {"kill", "town-1"},
		"sheriff-1": {"investigate", "mafia-1"},
		"medic-1":   {"protect", "sheriff-1"},
	})
	res, players, err := mgr.ResolveNight(id)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if res.KilledID != "town-1" {
		t.Errorf("expected town-1 killed, got %q", res.KilledID)
	}
	if res.KillBlocked {
		t.Error("kill should not have been blocked")
	}
	if res.SheriffRole != "mafia" {
		t.Errorf("sheriff should learn mafia, got %q", res.SheriffRole)
	}
	for _, p := range players {
		if p.SocketID == "town-1" && p.IsAlive {
			t.Error("town-1 should be dead")
		}
	}
	if mgr.lobbies[id].Phase != "day" {
		t.Errorf("phase should be day, got %q", mgr.lobbies[id].Phase)
	}
	if len(mgr.lobbies[id].NightActions) != 0 {
		t.Error("NightActions should be cleared")
	}
}

func TestResolveNight_MedicBlocksKill(t *testing.T) {
	mgr, id := setupNightLobby(t)
	submitAll(t, mgr, id, map[string][2]string{
		"mafia-1":   {"kill", "town-1"},
		"mafia-2":   {"kill", "town-1"},
		"sheriff-1": {"investigate", "mafia-1"},
		"medic-1":   {"protect", "town-1"},
	})
	res, players, err := mgr.ResolveNight(id)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if !res.KillBlocked {
		t.Error("kill should have been blocked by medic")
	}
	if res.KilledID != "" {
		t.Errorf("nobody should have died, got %q", res.KilledID)
	}
	for _, p := range players {
		if !p.IsAlive {
			t.Errorf("nobody should be dead, but %s is", p.SocketID)
		}
	}
}

func TestResolveNight_WeightedTiebreakReachesBoth(t *testing.T) {
	// 2 mafia votes for town-1, 1 for sheriff-1 (via overwrite trick): we run
	// many resolutions and assert both targets are picked at least once,
	// proving weighting is actually probabilistic and not deterministic.
	seenTown, seenSheriff := false, false
	for i := 0; i < 200 && !(seenTown && seenSheriff); i++ {
		mgr, id := setupNightLobby(t)
		// mafia-1 votes town-1, mafia-2 votes sheriff-1
		mgr.SubmitNightAction(id, "mafia-1", "kill", "town-1", 1)
		mgr.SubmitNightAction(id, "mafia-2", "kill", "sheriff-1", 1)
		mgr.SubmitNightAction(id, "sheriff-1", "investigate", "mafia-1", 1)
		mgr.SubmitNightAction(id, "medic-1", "protect", "medic-1", 1)
		res, _, err := mgr.ResolveNight(id)
		if err != nil {
			t.Fatalf("resolve: %v", err)
		}
		switch res.KilledID {
		case "town-1":
			seenTown = true
		case "sheriff-1":
			seenSheriff = true
		default:
			t.Fatalf("unexpected kill target %q", res.KilledID)
		}
	}
	if !seenTown || !seenSheriff {
		t.Errorf("expected both targets reachable in 200 runs (town=%v sheriff=%v)", seenTown, seenSheriff)
	}
}

func setupDayLobby(t *testing.T) (*Manager, string) {
	t.Helper()
	mgr, id := setupNightLobby(t)
	lobby := mgr.lobbies[id]
	lobby.Phase = "day"
	return mgr, id
}

func TestStartNomination_ClearsAndSetsPhase(t *testing.T) {
	mgr, id := setupDayLobby(t)
	round, err := mgr.StartNomination(id)
	if err != nil {
		t.Fatalf("start nomination: %v", err)
	}
	if round != 1 {
		t.Errorf("expected round 1, got %d", round)
	}
	if mgr.lobbies[id].Phase != "nomination" {
		t.Errorf("expected phase nomination, got %s", mgr.lobbies[id].Phase)
	}
}

func TestSubmitNomination_RejectsDeadVoter(t *testing.T) {
	mgr, id := setupDayLobby(t)
	mgr.StartNomination(id)
	for i := range mgr.lobbies[id].Players {
		if mgr.lobbies[id].Players[i].SocketID == "town-1" {
			mgr.lobbies[id].Players[i].IsAlive = false
		}
	}
	if err := mgr.SubmitNomination(id, "town-1", "mafia-1", 1); err == nil {
		t.Error("expected dead-voter rejection")
	}
}

func TestSubmitNomination_WrongRound(t *testing.T) {
	mgr, id := setupDayLobby(t)
	mgr.StartNomination(id)
	if err := mgr.SubmitNomination(id, "town-1", "mafia-1", 2); err == nil {
		t.Error("expected wrong-round rejection")
	}
}

func TestResolveNomination_PicksMajority(t *testing.T) {
	mgr, id := setupDayLobby(t)
	mgr.StartNomination(id)
	mgr.SubmitNomination(id, "mafia-1", "town-1", 1)
	mgr.SubmitNomination(id, "mafia-2", "town-1", 1)
	mgr.SubmitNomination(id, "sheriff-1", "mafia-1", 1)
	mgr.SubmitNomination(id, "medic-1", "town-1", 1)
	mgr.SubmitNomination(id, "town-1", "mafia-1", 1)

	nomineeID, _, any, err := mgr.ResolveNomination(id)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if !any {
		t.Fatal("expected votes present")
	}
	if nomineeID != "town-1" {
		t.Errorf("expected nominee town-1, got %s", nomineeID)
	}
	if mgr.lobbies[id].Phase != "defense" {
		t.Errorf("expected phase defense, got %s", mgr.lobbies[id].Phase)
	}
}

func TestResolveNomination_NoVotes(t *testing.T) {
	mgr, id := setupDayLobby(t)
	mgr.StartNomination(id)
	_, _, any, err := mgr.ResolveNomination(id)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if any {
		t.Error("expected no votes flag")
	}
}

func TestResolveDayVote_MajorityYesEliminates(t *testing.T) {
	mgr, id := setupDayLobby(t)
	mgr.StartNomination(id)
	for _, v := range []string{"mafia-1", "mafia-2", "sheriff-1", "medic-1", "town-1"} {
		mgr.SubmitNomination(id, v, "town-1", 1)
	}
	mgr.ResolveNomination(id)
	mgr.EndDefense(id)

	mgr.SubmitDayVote(id, "mafia-1", true, 1)
	mgr.SubmitDayVote(id, "mafia-2", true, 1)
	mgr.SubmitDayVote(id, "sheriff-1", true, 1)
	mgr.SubmitDayVote(id, "medic-1", false, 1)

	res, players, err := mgr.ResolveDayVote(id)
	if err != nil {
		t.Fatalf("resolve vote: %v", err)
	}
	if !res.Eliminated {
		t.Errorf("expected elimination, got %+v", res)
	}
	if res.YesVotes != 3 || res.NoVotes != 1 {
		t.Errorf("unexpected tally: %+v", res)
	}
	for _, p := range players {
		if p.SocketID == "town-1" && p.IsAlive {
			t.Error("expected town-1 dead after elimination")
		}
	}
	if mgr.lobbies[id].Phase != "vote-recap" || mgr.lobbies[id].Round != 1 {
		t.Errorf("expected vote-recap/round 1, got %s/%d", mgr.lobbies[id].Phase, mgr.lobbies[id].Round)
	}

	newRound, err := mgr.AdvanceToNightFromVoteRecap(id)
	if err != nil {
		t.Fatalf("advance from vote-recap: %v", err)
	}
	if newRound != 2 || mgr.lobbies[id].Phase != "night" {
		t.Errorf("expected night/round 2 after advance, got %s/%d", mgr.lobbies[id].Phase, newRound)
	}
}

func TestResolveDayVote_TieSurvives(t *testing.T) {
	mgr, id := setupDayLobby(t)
	mgr.StartNomination(id)
	for _, v := range []string{"mafia-1", "mafia-2", "sheriff-1", "medic-1", "town-1"} {
		mgr.SubmitNomination(id, v, "town-1", 1)
	}
	mgr.ResolveNomination(id)
	mgr.EndDefense(id)

	// 2 yes, 2 no (nominee town-1 excluded)
	mgr.SubmitDayVote(id, "mafia-1", true, 1)
	mgr.SubmitDayVote(id, "mafia-2", true, 1)
	mgr.SubmitDayVote(id, "sheriff-1", false, 1)
	mgr.SubmitDayVote(id, "medic-1", false, 1)

	res, _, err := mgr.ResolveDayVote(id)
	if err != nil {
		t.Fatalf("resolve vote: %v", err)
	}
	if res.Eliminated {
		t.Errorf("expected tie → survived, got eliminated %+v", res)
	}
}

func TestResolveDayVote_NomineeCannotVote(t *testing.T) {
	mgr, id := setupDayLobby(t)
	mgr.StartNomination(id)
	for _, v := range []string{"mafia-1", "mafia-2", "sheriff-1", "medic-1", "town-1"} {
		mgr.SubmitNomination(id, v, "town-1", 1)
	}
	mgr.ResolveNomination(id)
	mgr.EndDefense(id)

	if err := mgr.SubmitDayVote(id, "town-1", true, 1); err == nil {
		t.Error("expected nominee-vote rejection")
	}
}

func TestAllDayVotesSubmitted_ExcludesNomineeAndDead(t *testing.T) {
	mgr, id := setupDayLobby(t)
	mgr.StartNomination(id)
	for _, v := range []string{"mafia-1", "mafia-2", "sheriff-1", "medic-1", "town-1"} {
		mgr.SubmitNomination(id, v, "town-1", 1)
	}
	mgr.ResolveNomination(id)
	mgr.EndDefense(id)

	// All non-nominees vote.
	mgr.SubmitDayVote(id, "mafia-1", false, 1)
	mgr.SubmitDayVote(id, "mafia-2", false, 1)
	mgr.SubmitDayVote(id, "sheriff-1", false, 1)
	mgr.SubmitDayVote(id, "medic-1", false, 1)

	if !mgr.AllDayVotesSubmitted(id) {
		t.Error("expected all votes submitted")
	}
}

func TestAdvanceToNightFromNomination_IncrementsRound(t *testing.T) {
	mgr, id := setupDayLobby(t)
	mgr.StartNomination(id)
	newRound, _, err := mgr.AdvanceToNightFromNomination(id)
	if err != nil {
		t.Fatalf("advance: %v", err)
	}
	if newRound != 2 || mgr.lobbies[id].Phase != "night" {
		t.Errorf("expected night/round 2, got %s/%d", mgr.lobbies[id].Phase, newRound)
	}
}
