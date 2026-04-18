# Story Generator Service - System Design

## Context

MafiaCV currently has no game narration. The frontend has stub pages for Night/Day phases that listen for a `story-generated` event that the backend never sends. This plan designs a **separate Go service** that uses the **Google Gemini API (`gemini-2.5-flash`)** to generate dramatic game narration, with an Anthropic MCP server for persistent round-by-round context.

## Architecture Overview

```
+------------------+         +-----------------------+         +---------------------+
|   Go Backend     |  REST   |   Story Service       |  HTTPS  |   Gemini API        |
|   (port 8080)    |-------->|   (port 8090)         |-------->|   (Google Cloud)    |
|   Gin + WS Hub   |<--------|   Gin REST API        |<--------|   gemini-2.5-flash  |
+------------------+         +-----------+-----------+         +---------------------+
        |                                |
        | WebSocket                      | MCP (stdio JSON-RPC)
        | broadcast                      v
        v                    +-----------------------+
+------------------+         |   MCP Server          |
|   React Frontend |         |   (child process)     |
|   (players)      |         |   SQLite storage      |
+------------------+         +-----------------------+
```

**Flow:** Backend detects phase transition -> POSTs to Story Service -> Story Service retrieves context via MCP -> builds prompt -> calls Gemini API -> stores narrative via MCP -> returns story -> Backend broadcasts via WebSocket.

## Project Structure

```
story-service/
  cmd/story/main.go              # Entry point
  internal/
    config/config.go             # Env-based configuration
    server/server.go             # Gin router + graceful shutdown
    server/routes.go             # Route registration
    handler/handler.go           # HTTP handlers
    handler/request.go           # Request structs
    handler/response.go          # Response structs
    story/service.go             # Core orchestration
    story/prompts.go             # Prompt templates per narration type
    story/types.go               # Domain types (StoryType, GameEvent, etc.)
    gemini/client.go             # Wraps google.golang.org/genai, exposes Generate(ctx, systemPrompt, userPrompt)
    gemini/types.go              # Internal request/response shaping (mostly passthrough to SDK types)
    mcp/client.go                # MCP client (spawns server, JSON-RPC over stdio)
    mcp/types.go                 # MCP protocol types
  mcp-server/
    main.go                      # Standalone MCP server binary
    tools.go                     # 5 MCP tools (see below)
    storage.go                   # SQLite persistence
    schema.sql                   # DDL
  go.mod
  Dockerfile
```

## MCP Server Design

### SQLite Schema

Three tables: `game_events` (kills, saves, votes per round), `narratives` (generated stories), `game_settings` (theme, players, roles per lobby). Indexed on `lobby_id + round`.

### MCP Tools

| Tool | Purpose |
|------|---------|
| `init_game` | Store theme, player names, role config for a new game |
| `store_game_event` | Record a round event (kill, save, investigate, vote, eliminate) |
| `get_game_history` | Retrieve all events + narratives for a lobby (with optional round limit) |
| `store_narrative` | Persist a generated story for continuity |
| `cleanup_game` | Delete all data for a finished game |

The MCP server runs as a child process of the story service, communicating over stdio (JSON-RPC per MCP spec).

## REST API

### `POST /api/v1/story/init`
Called once at game start. Stores theme, players, roles via MCP.
```json
{ "lobbyId": "abc123", "players": [...], "roleConfig": {"mafia": 2, "medic": 1}, "theme": "classic noir" }
```

### `POST /api/v1/story/generate`
Main endpoint. Called at each phase transition.
```json
{
  "lobbyId": "abc123",
  "storyType": "game_intro | night_recap | day_intro | vote_recap",
  "round": 1,
  "events": [{"eventType": "kill", "actor": "Alice", "target": "Bob", "result": "killed"}],
  "players": [{"name": "Alice", "isAlive": true}, ...],
  "roleConfig": {"mafia": 2, "medic": 1}
}
```
Returns: `{ "story": "The town of Willowbrook wakes to find...", "storyType": "night_recap", "lobbyId": "abc123", "round": 1 }`

### `POST /api/v1/story/cleanup`
Called when game ends. `{ "lobbyId": "abc123" }`

### `GET /api/v1/health`
Returns status of Gemini credentials + MCP connectivity. Verifies `GEMINI_API_KEY` is set and the MCP child process is alive; does **not** make a live Gemini call on every probe.

### Timeout: 30s default, configurable. Backend uses 45s HTTP timeout with fallback to canned narration.

## Gemini Integration

- **SDK:** `google.golang.org/genai` (official Google Go SDK)
- **Model:** `gemini-2.5-flash` (configurable via `GEMINI_MODEL` env var)
- **Auth:** `GEMINI_API_KEY` env var. `config.Load()` fails fast if unset.
- **API call:** `client.Models.GenerateContent(ctx, model, contents, config)` with `config.SystemInstruction` carrying the narrator persona + theme, and `config.Temperature` ≈ 0.9 for creative variance.
- **Prompt strategy:** System instruction sets the narrator persona + theme. User content built from template + MCP history + current events.
- **Safety:** keep default safety settings; narration is PG-13 dramatic, not graphic.
- **Constraints:** 3-5 sentences max, never reveal hidden roles, maintain continuity with previous narrations pulled from MCP history.
- **Timeouts:** per-request context timeout 25s (inside the service's 30s total).

## Backend Integration

### New package: `backend/internal/storyclient/`
Lightweight HTTP client with `InitGame()`, `GenerateStory()`, `CleanupGame()` methods.

### Modified files:
- [hub.go](backend/internal/transport/ws/hub.go) — add `StoryClient` field to `Hub` struct
- [app.go](backend/internal/app/app.go) — create and inject `StoryClient` (URL from `STORY_SERVICE_URL` env, default `http://localhost:8090`)
- [handler.go](backend/internal/transport/ws/handler.go) — in `handleStartGame`, after role assignment, fire async goroutine to call `InitGame` + `GenerateStory("game_intro")` then broadcast `story-narration` event

### New WebSocket event: `story-narration` (S->C)
```json
{ "event": "story-narration", "data": { "storyType": "game_intro", "story": "...", "round": 0 } }
```

Future phase handlers (night end, vote result, etc.) follow the same pattern.

## Implementation Order

1. **Story service skeleton** — `go.mod`, config, Gin server, health endpoint
2. **Gemini client** — wrap `google.golang.org/genai`, expose `Generate(ctx, systemPrompt, userPrompt)`, test with a hardcoded prompt and a real API key from `GEMINI_API_KEY`
3. **MCP server** — SQLite storage, 5 tools, stdio JSON-RPC server
4. **MCP client** — spawn MCP server process, convenience methods
5. **Story orchestration** — prompt templates, context retrieval -> prompt building -> LLM call -> store result
6. **REST handlers** — wire `/api/v1/story/*` endpoints
7. **Backend integration** — `storyclient` package, modify Hub/app/handler
8. **Docker Compose** — add story-service alongside backend + frontend. `GEMINI_API_KEY` passed through from host env or `.env` file; no LLM runtime container or model cache volume required.

## Verification

1. **Unit:** Unit-test the prompt-building layer in isolation (pure functions over templates + MCP history). Cover the Gemini SDK call at the integration layer rather than mocking the SDK — SDK mocks are brittle.
2. **Integration:** POST to `/api/v1/story/generate` with sample game data against a real `GEMINI_API_KEY`, verify coherent story returns.
3. **Smoke test:** run `go run ./cmd/story` with a real `GEMINI_API_KEY` and `curl` the generate endpoint with a sample payload to confirm end-to-end connectivity outside Docker.
4. **End-to-end:** Start all services, create lobby, start game, verify `story-narration` event arrives at frontend with generated story text.
5. **Context continuity:** Generate stories across multiple rounds, verify MCP history is used (stories reference prior events).
