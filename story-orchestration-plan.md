# Plan: Story Orchestration Layer

## Context

The story-service has a working Gemini client (`internal/gemini`) and a working MCP client (`internal/mcp`) — but the glue between them is missing. Today `cmd/story/main.go` calls `gemini.Generate` with a hardcoded prompt and prints the result; nothing reads prior history, builds context-aware prompts, or persists narratives back to the MCP server. Without this layer, the REST handlers in the next step would each repeat the same fetch-build-call-store flow, and there would be no way to maintain continuity across rounds.

This step adds `internal/story/` — a single orchestration service whose methods correspond to the REST endpoints the handler layer will expose (`InitGame`, `GenerateStory`, `CleanupGame`). Handlers and route wiring are explicitly out of scope; they are step 6 of the parent plan ([story-service-plan.md](story-service-plan.md)).

## Scope

Create `story-service/internal/story/` containing:
- `types.go` — `StoryType` enum + request/response structs the future handler layer will unmarshal into
- `prompts.go` — pure functions that turn a request + `mcp.History` into `(systemPrompt, userPrompt)` strings, one per story type
- `service.go` — `Service` struct holding `*gemini.Client` and `*mcp.Client`; exposes `InitGame`, `GenerateStory`, `CleanupGame`
- `prompts_test.go` — table-driven tests over the prompt builders (pure, no network)
- `service_test.go` — orchestration test with a `geminiStub` (in-package, no SDK) plus the real MCP client against a freshly built binary

Also update `internal/config/config.go` to add `MCPBinaryPath` + `MCPDBPath`, and update `cmd/story/main.go` to construct the MCP client and the `story.Service` and replace the throwaway `Generate` demo with a commented-out reference (actual handler wiring comes next step).

## Design

### `types.go`

```go
type StoryType string

const (
    StoryTypeGameIntro  StoryType = "game_intro"
    StoryTypeNightRecap StoryType = "night_recap"
    StoryTypeDayIntro   StoryType = "day_intro"
    StoryTypeVoteRecap  StoryType = "vote_recap"
)

type Player struct {
    Name    string `json:"name"`
    IsAlive bool   `json:"isAlive"`
}

type Event struct {
    EventType string `json:"eventType"` // kill|save|investigate|vote|eliminate
    Actor     string `json:"actor"`
    Target    string `json:"target"`
    Result    string `json:"result"`
}

type InitRequest struct {
    LobbyID    string         `json:"lobbyId"`
    Theme      string         `json:"theme"`
    Players    []string       `json:"players"`
    RoleConfig map[string]int `json:"roleConfig"`
}

type GenerateRequest struct {
    LobbyID    string         `json:"lobbyId"`
    StoryType  StoryType      `json:"storyType"`
    Round      int            `json:"round"`
    Events     []Event        `json:"events"`
    Players    []Player       `json:"players"`
    RoleConfig map[string]int `json:"roleConfig"`
}

type GenerateResponse struct {
    LobbyID   string    `json:"lobbyId"`
    Round     int       `json:"round"`
    StoryType StoryType `json:"storyType"`
    Story     string    `json:"story"`
}
```

`StoryType.Valid()` helper returns true for the four allowed values; used for request validation and to mirror the MCP server's `validStoryTypes` check in [store_narrative.go](mcp-server/internal/tools/store_narrative.go).

### `prompts.go`

Pure functions only — no I/O, no SDK calls. This keeps the prompt layer unit-testable without stubs.

```go
func BuildSystemPrompt(theme string) string
func BuildUserPrompt(req GenerateRequest, history mcp.History) (string, error)
```

- `BuildSystemPrompt`: narrator persona + theme + hard constraints (3–5 sentences, PG-13, never reveal hidden roles, maintain continuity).
- `BuildUserPrompt`: switches on `req.StoryType` to pick a template. Each template is a small `text/template` or plain `fmt` builder that renders:
  - Game setup (theme + player roster + role counts from `history.Settings` if available, otherwise from `req`).
  - Prior narratives summary (up to the last 2 from `history.Narratives`) for continuity.
  - Current-round events from `req.Events`.
  - Alive/dead roster from `req.Players`.
- Returns a stable, deterministic string so the tests can snapshot it.

Reuses `mcp.History`, `mcp.StoredNarrative`, and `mcp.StoredSettings` directly — no duplicate types.

### `service.go`

```go
type Service struct {
    gemini *gemini.Client
    mcp    *mcp.Client
}

func New(g *gemini.Client, m *mcp.Client) *Service

func (s *Service) InitGame(ctx context.Context, r InitRequest) error
func (s *Service) GenerateStory(ctx context.Context, r GenerateRequest) (GenerateResponse, error)
func (s *Service) CleanupGame(ctx context.Context, lobbyID string) error
```

`InitGame`:
1. Validate `LobbyID` non-empty, `Players` non-empty, `RoleConfig` non-nil.
2. Delegate to `mcp.InitGame(ctx, mcp.GameSettings{...})`.

`GenerateStory` — the core flow:
1. Validate request (`LobbyID`, `StoryType.Valid()`, `Round >= 0`).
2. `history, err := s.mcp.GetGameHistory(ctx, r.LobbyID, nil)` — nil `maxRound` so the prompt builder itself can trim to the last N narratives.
3. Persist the raw current-round events via `mcp.StoreGameEvent` in a loop. **Chosen: persist events BEFORE generation** so the history reflects what the narrator "saw," and so a later retry sees consistent state. If an insert fails, abort with wrapped error — we would rather fail than narrate over incomplete state.
4. `systemPrompt := prompts.BuildSystemPrompt(theme)` where `theme` comes from `history.Settings.Theme`, falling back to `""`.
5. `userPrompt, err := prompts.BuildUserPrompt(r, history)`.
6. `story, err := s.gemini.Generate(ctx, systemPrompt, userPrompt)` — the Gemini client already applies its own 25s timeout internally.
7. `_, err := s.mcp.StoreNarrative(ctx, mcp.Narrative{LobbyID, Round, StoryType: string(r.StoryType), Story: story})` — failure here logs but does NOT fail the response, because the user-visible story has already been generated and the caller should still get it.
8. Return `GenerateResponse{LobbyID, Round, StoryType, Story}`.

`CleanupGame`:
1. Validate `lobbyID`.
2. Delegate to `mcp.CleanupGame`.

All methods respect `ctx` cancellation; errors wrapped with `fmt.Errorf("story: <op>: %w", err)` so handlers can `errors.Is`-classify later.

### `config` additions

Add to `Config`:
```go
MCPBinaryPath string // default "./mcp-server/mcp-server"
MCPDBPath     string // default "./mcp-server.db"
```
Wire via `getEnv("MCP_BINARY_PATH", ...)` / `getEnv("MCP_DB_PATH", ...)`. Do not make them required — defaults are fine in dev.

### `cmd/story/main.go` changes

- After `gemini.New`, spawn the MCP client: `mcpClient, err := mcp.New(ctx, cfg.MCPBinaryPath, cfg.MCPDBPath)`.
- Defer `mcpClient.Close()`.
- Construct `svc := story.New(geminiClient, mcpClient)` and pass to `server.New(cfg, svc)`.
- Update `server.New` signature to accept the service; it can stash it on the server struct for the handler layer to consume next step. Today `server.go` only has `/api/v1/health`; that remains untouched in this step.
- Remove the throwaway `Generate()` demo call; it is superseded.

## Files to Create / Modify

**Create:**
- [story-service/internal/story/types.go](story-service/internal/story/types.go)
- [story-service/internal/story/prompts.go](story-service/internal/story/prompts.go)
- [story-service/internal/story/service.go](story-service/internal/story/service.go)
- [story-service/internal/story/prompts_test.go](story-service/internal/story/prompts_test.go)
- [story-service/internal/story/service_test.go](story-service/internal/story/service_test.go)

**Modify:**
- [story-service/internal/config/config.go](story-service/internal/config/config.go) — add MCP path fields
- [story-service/cmd/story/main.go](story-service/cmd/story/main.go) — spawn MCP client, construct Service, pass to server
- [story-service/internal/server/server.go](story-service/internal/server/server.go) — accept `*story.Service` (stored; not used until next step)

## Reused Existing Code

- `gemini.Client.Generate(ctx, systemPrompt, userPrompt)` → [story-service/internal/gemini/client.go:39](story-service/internal/gemini/client.go)
- `mcp.Client.{InitGame, StoreGameEvent, StoreNarrative, GetGameHistory, CleanupGame}` → [story-service/internal/mcp/client.go](story-service/internal/mcp/client.go)
- `mcp.History`, `mcp.StoredSettings`, `mcp.StoredGameEvent`, `mcp.StoredNarrative` → [story-service/internal/mcp/types.go](story-service/internal/mcp/types.go)
- StoryType enum mirrors [mcp-server/internal/tools/store_narrative.go](mcp-server/internal/tools/store_narrative.go) `validStoryTypes`

## Verification

1. **Unit — prompts (no network):** `go test ./internal/story -run TestBuildPrompts -v`. Table cases: each `StoryType` with and without prior history, with and without `Settings`; assert the output contains the theme, alive-player names, and prior-narrative hints; assert role-revealing fields (e.g., per-player role) never appear in the user prompt.
2. **Unit — service validation:** `service_test.go` cases for empty `LobbyID`, invalid `StoryType`, negative `Round` — assert typed errors without any MCP/Gemini calls (validation runs first; `nil` clients are fine for these cases).
3. **Integration — full orchestration:** `service_test.go/TestGenerateStoryRoundTrip` (skipped with `-short`):
   - Build the real `mcp-server` into `t.TempDir()` (same pattern as [story-service/internal/mcp/client_test.go](story-service/internal/mcp/client_test.go)).
   - Spawn the MCP client against a temp SQLite.
   - Use a local `geminiStub` type in the test file that satisfies a narrow `generator` interface (`Generate(ctx, sys, user string) (string, error)`). Introduce this interface in `service.go` so tests can swap the SDK; the real `gemini.Client` already matches the shape.
   - Call `InitGame`, then `GenerateStory` for `game_intro` and `night_recap`; assert (a) stub recorded the expected prompt substrings, (b) `mcp.GetGameHistory` now contains the stored narrative, (c) response fields round-trip.
   - `CleanupGame`, assert history empties.
4. **Smoke — real Gemini:** `GEMINI_API_KEY=... go run ./cmd/story` starts the service; a tiny throwaway `_smoke_test.go` (build-tag `smoke`) can hit `svc.GenerateStory` against live Gemini to eyeball story quality once before wiring handlers.
5. **Build gate:** `go build ./... && go vet ./...` from `story-service/` must pass.
