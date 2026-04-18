// Package mcp is a client for the mafiacv MCP server. It spawns the server
// as a child process and speaks JSON-RPC 2.0 over its stdin/stdout.
package mcp

import (
	"encoding/json"
	"fmt"
	"time"
)

// JSON-RPC wire envelopes. Mirrors mcp-server/internal/rpc/stdio.go.

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int64  `json:"id,omitempty"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type rpcNotification struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// RPCError is a JSON-RPC 2.0 error object surfaced to callers.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func (e *RPCError) Error() string {
	return fmt.Sprintf("mcp: rpc error %d: %s", e.Code, e.Message)
}

// contentItem mirrors the server's MCP-style tool result content.
type contentItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type toolCallResult struct {
	Content []contentItem `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

// Domain types mirror the server's tool args / storage payloads so callers
// can pass typed values instead of raw maps.

type GameSettings struct {
	LobbyID    string         `json:"lobbyId"`
	Theme      string         `json:"theme,omitempty"`
	Players    []string       `json:"players"`
	RoleConfig map[string]int `json:"roleConfig"`
}

type GameEvent struct {
	LobbyID   string `json:"lobbyId"`
	Round     int    `json:"round"`
	EventType string `json:"eventType"`
	Actor     string `json:"actor"`
	Target    string `json:"target"`
	Result    string `json:"result"`
}

type Narrative struct {
	LobbyID   string `json:"lobbyId"`
	Round     int    `json:"round"`
	StoryType string `json:"storyType"`
	Story     string `json:"story"`
}

// StoredGameEvent is the decoded row returned by get_game_history.
type StoredGameEvent struct {
	ID        int64     `json:"id"`
	LobbyID   string    `json:"lobbyId"`
	Round     int       `json:"round"`
	EventType string    `json:"eventType"`
	Actor     string    `json:"actor"`
	Target    string    `json:"target"`
	Result    string    `json:"result"`
	CreatedAt time.Time `json:"createdAt"`
}

type StoredNarrative struct {
	ID        int64     `json:"id"`
	LobbyID   string    `json:"lobbyId"`
	Round     int       `json:"round"`
	StoryType string    `json:"storyType"`
	Story     string    `json:"story"`
	CreatedAt time.Time `json:"createdAt"`
}

type StoredSettings struct {
	LobbyID    string         `json:"lobbyId"`
	Theme      string         `json:"theme"`
	Players    []string       `json:"players"`
	RoleConfig map[string]int `json:"roleConfig"`
	CreatedAt  time.Time      `json:"createdAt"`
}

// History mirrors storage.GameHistory. Settings is nil if the lobby has no
// init_game record yet.
type History struct {
	Settings   *StoredSettings   `json:"settings,omitempty"`
	Events     []StoredGameEvent `json:"events"`
	Narratives []StoredNarrative `json:"narratives"`
}
