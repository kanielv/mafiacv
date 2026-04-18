# Plan: MCP Client for Story Service

## Context

The MCP server at [mcp-server/](mcp-server/) is complete and exposes 5 tools (`init_game`, `store_game_event`, `get_game_history`, `store_narrative`, `cleanup_game`) over stdio JSON-RPC 2.0 (protocol version `2024-11-05`). The story service at [story-service/](story-service/) needs a client that spawns the MCP server as a child process and provides Go methods the story orchestrator can call. Without this client, the story service cannot read or write round-by-round game context and cannot produce continuity-aware narration.

This plan covers building only the client — not the orchestration layer or REST handlers (those follow in steps 5–6 of the parent plan in [story-service-plan.md](story-service-plan.md)).

## Scope

Create `story-service/internal/mcp/` with:
- `types.go` — protocol wire types + domain types mirroring the server's tool args/results
- `client.go` — child-process lifecycle + JSON-RPC dispatcher + typed convenience methods
- `client_test.go` — integration test that spawns the real server binary

## Protocol Contract (from server)

- Transport: newline-delimited JSON on stdin/stdout of the child. Logs on stderr.
- Handshake: client sends `initialize` → server replies with `protocolVersion: "2024-11-05"` + `serverInfo` → client sends `notifications/initialized` (no reply).
- Tool call: `tools/call` with `{name, arguments}` → server replies `{content: [{type:"text", text:"<json-or-string>"}], isError?: bool}`.
- Unknown tool → JSON-RPC error (code `-32601`). Tool-level failure → result with `isError: true` and the error message in `content[0].text`.
- Server tolerates `ping` and `tools/list` too (useful for health).

## Design

### `types.go`
- JSON-RPC envelopes: `rpcRequest`, `rpcResponse`, `rpcError` — mirror the shapes in [stdio.go](mcp-server/internal/rpc/stdio.go).
- `toolCallResult { Content []contentItem; IsError bool }`.
- Domain types matching tool args, reused by callers:
  - `GameSettings { LobbyID, Theme string; Players []string; RoleConfig map[string]int }`
  - `GameEvent { LobbyID string; Round int; EventType, Actor, Target, Result string }`
  - `Narrative { LobbyID string; Round int; StoryType, Story string }`
  - `History` — shape returned by `get_game_history` (decoded from `content[0].text` JSON). Mirror [storage.go](mcp-server/internal/storage/storage.go) return type; confirm fields when implementing.

### `client.go`

**Lifecycle**
- `New(ctx context.Context, binaryPath, dbPath string) (*Client, error)` — `exec.CommandContext` the server, wire stdin/stdout pipes, redirect stderr to the service's logger (prefixed `mcp-server:`), `Start()`, then run `initialize` handshake + send `initialized` notification. Fail fast if handshake doesn't complete within ~5s.
- `Close() error` — close stdin (signals the server's scanner to exit), `cmd.Wait()` with a bounded timeout, SIGKILL on timeout.

**Dispatcher**
- Single `readLoop` goroutine over `bufio.Scanner` on stdout (match the server's 8MB buffer ceiling — history payloads grow).
- `pending map[int64]chan rpcResponse` guarded by a mutex; monotonic `nextID` counter.
- `call(ctx, method string, params any) (json.RawMessage, error)` — assigns ID, registers channel, writes request under a write mutex (single encoder), awaits response or `ctx.Done()`. On `ctx` cancellation, deregister the pending entry so a late reply doesn't leak.
- If `readLoop` exits (pipe closed / server died), fail all pending calls with a sentinel error and mark the client unusable.

**Typed methods** (each builds `tools/call` params, dispatches, then decodes `content[0].text`):
- `InitGame(ctx, GameSettings) error`
- `StoreGameEvent(ctx, GameEvent) (int64, error)` — returns the inserted row ID
- `GetGameHistory(ctx, lobbyID string, maxRound *int) (History, error)`
- `StoreNarrative(ctx, Narrative) (int64, error)`
- `CleanupGame(ctx, lobbyID string) error`
- `Ping(ctx) error` — used by the `/api/v1/health` handler later

**Error handling**
- JSON-RPC `error` field → wrap as `*RPCError{Code, Message}`.
- `isError: true` in result → return `errors.New(content[0].text)` so the story service can log/surface it.
- Context cancellation respected everywhere.

### Integration points (not built in this step, documented for reference)
- `config.Load()` gains `MCPBinaryPath` (default `./mcp-server/mcp-server`) and `MCPDBPath` (default `./mcp-server.db`) — already on disk today.
- [cmd/story/main.go](story-service/cmd/story/main.go) constructs the client after `gemini.New` and passes it into `server.New`.

## Files to create
- [story-service/internal/mcp/types.go](story-service/internal/mcp/types.go)
- [story-service/internal/mcp/client.go](story-service/internal/mcp/client.go)
- [story-service/internal/mcp/client_test.go](story-service/internal/mcp/client_test.go)

## Files to reference (read-only, for types/contracts)
- [mcp-server/internal/rpc/stdio.go](mcp-server/internal/rpc/stdio.go) — wire format, codes
- [mcp-server/internal/tools/*.go](mcp-server/internal/tools/) — arg shapes per tool
- [mcp-server/internal/storage/storage.go](mcp-server/internal/storage/storage.go) — `GetGameHistory` return shape

## Verification

1. **Unit (no child process):** a fake `io.ReadWriteCloser` pair exercising handshake, a successful `tools/call`, a tool-level `isError`, and a JSON-RPC error. Confirms dispatcher correctness without the binary.
2. **Integration test** (`client_test.go`, guarded by `testing.Short()` skip): build the real `mcp-server` binary into `t.TempDir()` via `go build`, point the client at a fresh temp SQLite DB, then call `InitGame` → `StoreGameEvent` → `GetGameHistory` → assert the event round-trips → `CleanupGame` → assert history is empty. Cleanly `Close()` and confirm the child exits.
3. **Smoke:** `go test ./internal/mcp -run TestClientRoundTrip -v` from `story-service/`, plus a throwaway `main` that opens the client and calls `Ping` to confirm the handshake against the already-built binary at `mcp-server/mcp-server`.
