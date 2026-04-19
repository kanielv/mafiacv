# Story Type: `game_ending`

## Context
The new win-condition flow calls `story-service` with `storyType = "game_ending"`, but the service rejects it: `storyType "game_ending" is not valid`. The story-service and mcp-server both maintain explicit allowlists for story types, and the prompt builder dispatches per type. We need to register `game_ending` end-to-end and give it a prompt that narrates a conclusive ending with role reveals.

The backend already sends the winner in the Events slice as `{EventType: "game_ending", Result: winner}` — no backend wiring change needed for the signal itself, but the final roster needs to carry **roles** so the ending can reveal who was mafia.

## Design Decisions (confirmed)
- Role reveals **allowed** in the ending (overrides the system-prompt rule for this one type).
- Winner is read from `req.Events[0].Result` — no new request field.
- Length stays 3–5 sentences to match existing cadence / TTS layer.
- Persist `game_ending` via mcp-server like every other type.

## Changes

### 1. story-service — register the type
`story-service/internal/story/types.go`
- Add `StoryTypeGameEnding StoryType = "game_ending"`.
- Add it to the `Valid()` switch.

### 2. story-service — carry role on Player
`story-service/internal/story/types.go` — extend `Player`:
```go
type Player struct {
    Name    string `json:"name"`
    IsAlive bool   `json:"isAlive"`
    Role    string `json:"role,omitempty"` // only populated for game_ending
}
```
Optional field, `omitempty` — existing request shapes keep working.

### 3. story-service — prompt case for game_ending
`story-service/internal/story/prompts.go` — add a new case in the `BuildUserPrompt` switch:

```go
case StoryTypeGameEnding:
    winner := ""
    if len(req.Events) > 0 {
        winner = req.Events[0].Result
    }
    fmt.Fprintf(&b, "\nFinal outcome: %s wins.\n", winner)
    b.WriteString("Final roster with roles:\n")
    for _, p := range req.Players {
        state := "alive"
        if !p.IsAlive { state = "dead" }
        role := p.Role
        if role == "" { role = "unknown" }
        fmt.Fprintf(&b, "- %s (%s, %s)\n", p.Name, role, state)
    }
    b.WriteString("\nTask: Narrate the ending of the game. ")
    b.WriteString("Reveal which players were mafia and which were townsfolk (including medic, sheriff if present). ")
    b.WriteString("Describe how the town settles (or falls) after the winning side prevails. ")
    b.WriteString("This is the final narration — give it a conclusive feel. ")
    b.WriteString("For THIS narration only, you MAY reveal hidden roles; the usual rule against revealing roles does not apply to the ending.")
```

The local override line is important because `BuildSystemPrompt` installs "Never reveal which players hold hidden roles" as a hard rule. The user-prompt explicit override has proven reliable for Gemini when framed as "for THIS narration only".

### 4. mcp-server — accept `game_ending` in store_narrative
`mcp-server/internal/tools/store_narrative.go`
- Add `"game_ending": true` to `validStoryTypes`.
- Add `"game_ending"` to the JSON schema enum on line 30.

### 5. backend — add Role to the storyclient Player + populate for ending
`backend/internal/storyclient/client.go` — extend `Player`:
```go
type Player struct {
    Name    string `json:"name"`
    IsAlive bool   `json:"isAlive"`
    Role    string `json:"role,omitempty"`
}
```

`backend/internal/transport/ws/handler.go` — in `endGameFlow`, populate `Role` on the story payload (this is the only place we want role info to leave the backend):
```go
storyPlayers = append(storyPlayers, storyclient.Player{
    Name:    p.Name,
    IsAlive: p.IsAlive,
    Role:    p.Role,
})
```
Leave all other call sites (`generateNightRecap`, `generateVoteRecap`, `generateGameIntro`) unchanged — their players stay role-less, so no role leaks into mid-game narrations.

## Critical Files
- `story-service/internal/story/types.go`
- `story-service/internal/story/prompts.go`
- `mcp-server/internal/tools/store_narrative.go`
- `backend/internal/storyclient/client.go`
- `backend/internal/transport/ws/handler.go` (only `endGameFlow`)

## Reused / Unchanged
- `writeSetup`, `writePriorNarratives`, `writeRoster`, `writeEvents` — reused as-is; the ending naturally gets the last two narratives for continuity.
- Gemini config — no per-type temperature/model/timeout change.
- Response shape (`GenerateResponse`) — unchanged.
- Backend broadcast of `game_ending` narration via `story-narration` — already wired.
- Frontend `GameOver` page + `game-ended` handling — already shipped; this fix only unblocks the broadcast.

## Verification
1. **Build**: `go build ./...` in `backend/`, `story-service/`, `mcp-server/`.
2. **Unit** (story-service): if there are existing prompt tests, add a table row for `game_ending` asserting that `BuildUserPrompt` includes the winner and role lines.
3. **End-to-end via `./run-dev.sh`**:
   - 4-player lobby, 1 mafia + 3 town. Vote the mafia out day 1 → verify:
     - backend logs no longer show `storyType "game_ending" is not valid`
     - `story-narration` with `storyType: "game_ending"` is broadcast
     - Frontend lands on `/GameOver` with the ending text + role reveals
     - `store_narrative` persists the ending in `mcp-server.db` (spot-check with `sqlite3 story-service/mcp-server.db "select storyType from narratives where lobbyId='...'"`)
4. **Mafia-win path**: repeat with mafia killing off town in two nights; ending should reveal the mafia and describe their victory.
