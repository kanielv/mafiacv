# Win Condition & Game End

## Context
The game loop currently runs indefinitely — `ResolveNight` and `ResolveDayVote` update `IsAlive` but nothing checks whether the game is over. We need the backend to detect a mafia or town victory the moment it occurs, narrate an ending in the following phase, and tear the lobby down cleanly while leaving the server process running for other lobbies. The story-service already exposes a `CleanupGame` endpoint that is defined but never called today — ending the game is the natural place to invoke it.

## Design Decisions (confirmed with user)
- **Win rule**: town wins when mafia count = 0; mafia wins when `mafia >= non-mafia alive`. Non-mafia = town + medic + sheriff + jester (jester counts as town for now — no solo win).
- **Night-triggered end**: play night_recap as usual, then ending narration during the `day` phase, then shut down.
- **Vote-triggered end**: play ending narration during `vote-recap` phase, then shut down.
- **Shutdown scope**: per-lobby only — call story cleanup, broadcast `game-ended`, delete the lobby from `Manager.lobbies`. Clients stay connected; server keeps running.

## Approach

### 1. Add win-check helper in the lobby package
`backend/internal/lobby/manager.go`

- Add a package type `GameOutcome`:
  ```go
  type GameOutcome struct {
      Winner     string // "mafia" | "town" | ""  ("" = no winner yet)
      MafiaAlive int
      TownAlive  int // non-mafia alive
  }
  ```
- Add `CheckWinCondition(lobbyID string) GameOutcome` — read-locked, counts alive players by role, applies the rule above.
- Add `EndGame(lobbyID string)` — write-locked; sets `lobby.Phase = "ended"` and `lobby.Winner`; returns the final player snapshot for broadcast. Does **not** delete the lobby yet — that happens after cleanup so stray events can still be ignored safely.
- Add `DeleteLobby(lobbyID string)` — write-locked; removes the entry from `m.lobbies` (and `m.players` reverse map if present).
- Extend `models.Lobby` (`backend/internal/models/lobby.go`) with `Winner string` so the phase transitions can carry the result.

### 2. Wire win check into night resolution (day narration path)
`backend/internal/transport/ws/handler.go` — `handleNightAction` after `ResolveNight`:

- After the existing `phase-changed → day` + `players-updated` broadcast, call `CheckWinCondition`.
- If `Winner != ""`, launch `go h.endGameFlow(lobbyID, winner, players, afterNarration: "night_recap")`.
- The normal `generateNightRecap` still runs (user wants night recap played first).

### 3. Wire win check into vote resolution (vote-recap narration path)
`handler.go` — `resolveDayVote` after `ResolveDayVote`:

- After existing `phase-changed → vote-recap` + `players-updated`, call `CheckWinCondition`.
- If `Winner != ""`, launch `go h.endGameFlow(lobbyID, winner, players, afterNarration: "vote_recap")`.
- The normal `generateVoteRecap` still runs.

### 4. New orchestrator: `endGameFlow`
`handler.go` — new helper:

1. Call `h.Story.GenerateStory` with a new `StoryType = "game_ending"`, passing `winner`, final player list, and role config. Reuse `storyclient.GenerateRequest` — no schema change needed; just add a new `StoryType` constant. (Note: the story-service itself may need a prompt for this story type; out of scope for this backend plan — a plain-text fallback is acceptable if the service echoes back.)
2. Broadcast the `story-narration` message (existing `game_ending` type signals the frontend that this is the final beat).
3. Wait for a short narration window (reuse `phaseSeconds`-style delay or an explicit `gameEndDelay = 20 * time.Second`) to let the ending play out on clients.
4. Broadcast a new event `game-ended` with `{winner, reason, players}` so the frontend can route to a results screen.
5. Call `h.Story.CleanupGame(ctx, lobbyID)` (already implemented in `storyclient/client.go:77`).
6. Call `h.Manager.DeleteLobby(lobbyID)` to drop in-memory state.

Guard against double-fire: `endGameFlow` should check phase is still not `"ended"` before acting, and `EndGame` should be a no-op if already ended. This prevents a race where both the night recap and a subsequent event try to end the game.

### 5. Frontend (minimal)
`frontend/src/socket.ts` + `frontend/src/context/GameContext.tsx`:

- Subscribe to new `game-ended` event; set a `winner` field in game context.
- Render a simple results view when phase is `"ended"` or `winner` is set. Existing `story-narration` handler already plays the ending narration — no change needed there.

## Critical Files
- `backend/internal/lobby/manager.go` — add `CheckWinCondition`, `EndGame`, `DeleteLobby`, wire into existing methods.
- `backend/internal/models/lobby.go` — add `Winner` field.
- `backend/internal/transport/ws/handler.go` — call win check after `ResolveNight` and `ResolveDayVote`; add `endGameFlow`.
- `backend/internal/storyclient/client.go` — already has `CleanupGame` (reuse, no edit).
- `frontend/src/context/GameContext.tsx`, `frontend/src/socket.ts` — add `game-ended` handler + results render.

## Reused Utilities (no rewriting)
- `Manager.GetRoleConfig` — for story request payload.
- `storyclient.Client.GenerateStory` / `CleanupGame` — no new HTTP methods.
- `Hub.BroadcastToRoom` + `MarshalMessage` — existing broadcast helpers.
- `sanitizePlayers` — reuse for the final player snapshot (roles can optionally be revealed in the `game-ended` payload; TBD — default: keep sanitized for now).

## Verification
1. **Unit test** for `CheckWinCondition` in `backend/internal/lobby/manager_test.go` (create if missing) covering: all mafia dead → town; mafia = non-mafia alive → mafia; mixed survivors → no winner.
2. **Integration (manual)** via `./run-dev.sh`:
   - Start a 4-player lobby with 1 mafia, 3 town. Have mafia kill two nights in a row → after the second night recap, expect a `day` phase, ending narration broadcast, then `game-ended` with `winner: "mafia"`, then lobby removed.
   - Start a 4-player lobby with 1 mafia, 3 town. Vote out the mafia on day 1 → during `vote-recap`, expect ending narration, `game-ended` with `winner: "town"`, then lobby removed.
3. **Logs**: verify `CleanupGame` is called for the lobby ID and no goroutine continues timers after end (grep logs for the lobby ID post-end).
4. **No regressions**: run any non-terminal night/vote cycles — `phase-changed` events must continue advancing `nomination → defense → vote → vote-recap → night` exactly as today when no win condition is met.
