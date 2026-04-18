package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os/exec"
	"strconv"
	"sync"
	"time"
)

const (
	protocolVersion = "2024-11-05"
	clientName      = "mafiacv-story-service"
	clientVersion   = "0.1.0"

	handshakeTimeout = 5 * time.Second
	shutdownTimeout  = 3 * time.Second
	scannerMaxBytes  = 8 * 1024 * 1024
)

// ErrClientClosed is returned by calls issued after the client has shut down
// or the underlying child process has exited.
var ErrClientClosed = errors.New("mcp: client closed")

// Client speaks JSON-RPC 2.0 over the MCP server's stdio. It owns the child
// process for its lifetime.
type Client struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser

	enc     *json.Encoder
	writeMu sync.Mutex

	pendingMu sync.Mutex
	pending   map[int64]chan rpcResponse
	nextID    int64

	closed   chan struct{}
	closeErr error
	closeMu  sync.Mutex
}

// New spawns the MCP server binary, performs the initialize handshake, and
// returns a ready-to-use client. `dbPath` is passed through as MCP_DB_PATH.
func New(ctx context.Context, binaryPath, dbPath string) (*Client, error) {
	cmd := exec.CommandContext(ctx, binaryPath)
	cmd.Env = append(cmd.Environ(), "MCP_DB_PATH="+dbPath)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("mcp: stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("mcp: stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("mcp: stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("mcp: start %q: %w", binaryPath, err)
	}

	c := &Client{
		cmd:     cmd,
		stdin:   stdin,
		stdout:  stdout,
		enc:     json.NewEncoder(stdin),
		pending: map[int64]chan rpcResponse{},
		closed:  make(chan struct{}),
	}

	go c.pipeStderr(stderr)
	go c.readLoop()

	hctx, cancel := context.WithTimeout(ctx, handshakeTimeout)
	defer cancel()
	if err := c.handshake(hctx); err != nil {
		_ = c.Close()
		return nil, err
	}
	return c, nil
}

func (c *Client) handshake(ctx context.Context) error {
	params := map[string]any{
		"protocolVersion": protocolVersion,
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    clientName,
			"version": clientVersion,
		},
	}
	if _, err := c.call(ctx, "initialize", params); err != nil {
		return fmt.Errorf("mcp: initialize: %w", err)
	}
	if err := c.notify("notifications/initialized", nil); err != nil {
		return fmt.Errorf("mcp: initialized notification: %w", err)
	}
	return nil
}

func (c *Client) pipeStderr(r io.ReadCloser) {
	defer r.Close()
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), scannerMaxBytes)
	for scanner.Scan() {
		log.Printf("mcp-server: %s", scanner.Text())
	}
}

func (c *Client) readLoop() {
	scanner := bufio.NewScanner(c.stdout)
	scanner.Buffer(make([]byte, 64*1024), scannerMaxBytes)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var resp rpcResponse
		if err := json.Unmarshal(line, &resp); err != nil {
			log.Printf("mcp: decode response: %v", err)
			continue
		}
		id, ok := parseID(resp.ID)
		if !ok {
			// Server-sent notification (no id). Not expected today; ignore.
			continue
		}
		c.pendingMu.Lock()
		ch, found := c.pending[id]
		delete(c.pending, id)
		c.pendingMu.Unlock()
		if !found {
			continue // late reply after caller cancelled
		}
		ch <- resp
	}

	err := scanner.Err()
	if err == nil {
		err = io.EOF
	}
	c.shutdown(fmt.Errorf("mcp: read loop exited: %w", err))
}

func (c *Client) shutdown(cause error) {
	c.closeMu.Lock()
	select {
	case <-c.closed:
		c.closeMu.Unlock()
		return
	default:
	}
	if c.closeErr == nil {
		c.closeErr = cause
	}
	close(c.closed)
	c.closeMu.Unlock()

	c.pendingMu.Lock()
	for id, ch := range c.pending {
		close(ch)
		delete(c.pending, id)
	}
	c.pendingMu.Unlock()
}

// Close shuts stdin to signal the server, waits briefly for the process, and
// SIGKILLs on timeout.
func (c *Client) Close() error {
	_ = c.stdin.Close()

	done := make(chan error, 1)
	go func() { done <- c.cmd.Wait() }()

	var waitErr error
	select {
	case waitErr = <-done:
	case <-time.After(shutdownTimeout):
		_ = c.cmd.Process.Kill()
		waitErr = <-done
	}
	c.shutdown(ErrClientClosed)
	if waitErr != nil && !isExpectedExit(waitErr) {
		return fmt.Errorf("mcp: child exit: %w", waitErr)
	}
	return nil
}

func isExpectedExit(err error) bool {
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		// Killed after stdin close / SIGKILL — treat as clean shutdown.
		return true
	}
	return false
}

func (c *Client) notify(method string, params any) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return c.enc.Encode(rpcNotification{JSONRPC: "2.0", Method: method, Params: params})
}

func (c *Client) call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	select {
	case <-c.closed:
		return nil, ErrClientClosed
	default:
	}

	c.pendingMu.Lock()
	c.nextID++
	id := c.nextID
	ch := make(chan rpcResponse, 1)
	c.pending[id] = ch
	c.pendingMu.Unlock()

	req := rpcRequest{JSONRPC: "2.0", ID: id, Method: method, Params: params}

	c.writeMu.Lock()
	err := c.enc.Encode(req)
	c.writeMu.Unlock()
	if err != nil {
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
		return nil, fmt.Errorf("mcp: write %s: %w", method, err)
	}

	select {
	case resp, ok := <-ch:
		if !ok {
			return nil, c.shutdownErr()
		}
		if resp.Error != nil {
			return nil, resp.Error
		}
		return resp.Result, nil
	case <-ctx.Done():
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
		return nil, ctx.Err()
	case <-c.closed:
		return nil, c.shutdownErr()
	}
}

func (c *Client) shutdownErr() error {
	c.closeMu.Lock()
	defer c.closeMu.Unlock()
	if c.closeErr != nil {
		return c.closeErr
	}
	return ErrClientClosed
}

// callTool dispatches a tools/call and decodes the single-text-content result
// into `out`. If the server sets isError, the text is returned as an error.
func (c *Client) callTool(ctx context.Context, name string, args any, out any) error {
	params := map[string]any{"name": name, "arguments": args}
	raw, err := c.call(ctx, "tools/call", params)
	if err != nil {
		return err
	}
	var tr toolCallResult
	if err := json.Unmarshal(raw, &tr); err != nil {
		return fmt.Errorf("mcp: decode tool result: %w", err)
	}
	text := ""
	if len(tr.Content) > 0 {
		text = tr.Content[0].Text
	}
	if tr.IsError {
		if text == "" {
			text = "unknown tool error"
		}
		return fmt.Errorf("mcp: tool %s: %s", name, text)
	}
	if out == nil || text == "" {
		return nil
	}
	if err := json.Unmarshal([]byte(text), out); err != nil {
		return fmt.Errorf("mcp: decode %s payload: %w", name, err)
	}
	return nil
}

// Ping calls the server's ping method. Useful for /health.
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.call(ctx, "ping", map[string]any{})
	return err
}

func (c *Client) InitGame(ctx context.Context, g GameSettings) error {
	return c.callTool(ctx, "init_game", g, nil)
}

func (c *Client) StoreGameEvent(ctx context.Context, e GameEvent) (int64, error) {
	var out struct {
		ID int64 `json:"id"`
	}
	if err := c.callTool(ctx, "store_game_event", e, &out); err != nil {
		return 0, err
	}
	return out.ID, nil
}

func (c *Client) StoreNarrative(ctx context.Context, n Narrative) (int64, error) {
	var out struct {
		ID int64 `json:"id"`
	}
	if err := c.callTool(ctx, "store_narrative", n, &out); err != nil {
		return 0, err
	}
	return out.ID, nil
}

func (c *Client) GetGameHistory(ctx context.Context, lobbyID string, maxRound *int) (History, error) {
	args := map[string]any{"lobbyId": lobbyID}
	if maxRound != nil {
		args["maxRound"] = *maxRound
	}
	var h History
	if err := c.callTool(ctx, "get_game_history", args, &h); err != nil {
		return History{}, err
	}
	return h, nil
}

func (c *Client) CleanupGame(ctx context.Context, lobbyID string) error {
	return c.callTool(ctx, "cleanup_game", map[string]any{"lobbyId": lobbyID}, nil)
}

func parseID(raw json.RawMessage) (int64, bool) {
	if len(raw) == 0 {
		return 0, false
	}
	// IDs may arrive as numbers or strings depending on peer; we always send
	// numbers, but be lenient on decode.
	var n int64
	if err := json.Unmarshal(raw, &n); err == nil {
		return n, true
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		if v, err := strconv.ParseInt(s, 10, 64); err == nil {
			return v, true
		}
	}
	return 0, false
}
