# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

MafiaCV is a web app for facilitating in-person Mafia (Werewolf) games. It handles real-time role assignment and game coordination via WebSockets, with Google Gemini AI for game narration and Google Cloud TTS for speech.

## Commands

### Frontend (`cd frontend`)
```bash
npm run dev       # Dev server at http://localhost:3000
npm run build     # TypeScript check + Vite production build
npm run lint      # ESLint (0 warnings allowed)
npm run preview   # Preview production build
```

### Backend (`cd backend`)
```bash
go run ./cmd/app/main.go   # Dev server at http://localhost:8080
go test ./...               # Run all tests
go test ./internal/lobby/   # Run a specific package's tests
go build -o main ./cmd/app  # Build binary
```

## Architecture

**Full stack:** React 18 + TypeScript frontend (Vite, Mantine UI, Tailwind) talking to a Go backend (Gin + Gorilla WebSocket).

**Primary communication is WebSocket**, not REST. The only REST endpoints are `GET /` (health check) and `GET /ws` (upgrade to WebSocket). All game logic flows through WebSocket events.

### Backend layers (`backend/internal/`)
- **`app/app.go`** — wires everything together: creates `lobby.Manager`, creates `ws.Hub`, starts the hub goroutine, then starts the Gin router on `:8080`
- **`lobby/manager.go`** — in-memory game state (thread-safe with `sync.RWMutex`). Owns lobbies and the socketID→lobbyID mapping. Max 100 chat messages per lobby. Handles role configuration (`SetRoleConfig`) and role assignment during `StartGame`.
- **`transport/ws/`** — WebSocket layer:
  - `hub.go` — central broker; owns `Clients` map and `Rooms` (lobbyID→clientID→Client) for targeted broadcasting
  - `client.go` — one per connection; runs `ReadPump`/`WritePump` goroutines; 4096-byte max message, 60s ping timeout
  - `handler.go` — dispatches incoming events (`create-lobby`, `join-lobby`, `select-roles`, `start-game`, `chat-message`) to lobby manager then broadcasts back
  - `upgrade.go` — HTTP→WebSocket upgrade + `MarshalMessage` helper
- **`transport/rest/server.go`** — Gin router setup; CORS allows all origins
- **`models/`** — `Player` (with `Role`), `Lobby` (with `RoleConfig`), `WSMessage`, `ChatMessage`

### Frontend WebSocket (`frontend/src/socket.ts`)
- Connects to `ws://localhost:8080/ws` on load
- Auto-reconnects with exponential backoff (1s → 30s)
- Event-based pub/sub: `sendEvent(name, data)` / `onEvent(name, callback)`

### Key WebSocket events
| Direction | Event | Trigger |
|-----------|-------|---------|
| C→S | `create-lobby` | Host creates a game |
| C→S | `join-lobby` | Player joins by lobby ID |
| C→S | `select-roles` | Host configures role counts before starting |
| C→S | `start-game` | Host starts the game |
| C→S | `chat-message` | In-lobby chat |
| S→C | `connected` | Initial socket ID assignment |
| S→C | `lobby-created` | Confirms creation with lobby ID |
| S→C | `players-updated` | Broadcast after any roster change |
| S→C | `roles-updated` | Broadcast after host configures roles |
| S→C | `game-started` | Broadcast when host starts |
| S→C | `roles-assigned` | Individual role delivery (sent per-player at game start) |
| S→C | `user-disconnected` | Player dropped |

### Role Assignment
- **Available roles:** Mafia, Medic, Sheriff, Jester, Town (default filler)
- Host configures role counts via `select-roles` before starting; stored as `RoleConfig map[string]int` on the Lobby
- If mafia count is not configured, it defaults to `floor(playerCount/3)` (minimum 1)
- Unassigned players automatically become Town
- On `start-game`, roles are shuffled (Fisher-Yates) and assigned to players; each player receives their role individually via `roles-assigned`

## Key Files to Know
- `backend/cmd/app/main.go` — entry point
- `backend/internal/app/app.go` — dependency wiring
- `backend/internal/lobby/manager.go` — all lobby state mutations
- `backend/internal/transport/ws/handler.go` — add new WebSocket events here
- `frontend/src/socket.ts` — WebSocket client abstraction
- `frontend/src/pages/Home/Pages.tsx` — main lobby UI + WebSocket integration
- `frontend/src/pages/Home/RoleSelection.tsx` — host role configuration UI (Mafia, Medic, Sheriff, Jester)
