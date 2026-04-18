// Package rpc implements a minimal JSON-RPC 2.0 transport over stdio,
// sufficient for the MCP methods this server supports: initialize,
// notifications/initialized, tools/list, and tools/call.
package rpc

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"sync"

	"github.com/kanielv/mafiacv/mcp-server/internal/tools"
)

const (
	protocolVersion = "2024-11-05"
	serverName      = "mafiacv-mcp-server"
	serverVersion   = "0.1.0"
)

// JSON-RPC 2.0 error codes.
const (
	codeParseError     = -32700
	codeInvalidRequest = -32600
	codeMethodNotFound = -32601
	codeInvalidParams  = -32602
	codeInternalError  = -32603
)

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type contentItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type toolCallResult struct {
	Content []contentItem `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

// Server dispatches JSON-RPC messages from a reader to the tool registry,
// writing responses to a writer. Writes are serialized so concurrent dispatch
// is safe to layer on later if needed.
type Server struct {
	registry *tools.Registry
	enc      *json.Encoder
	writeMu  sync.Mutex
}

func NewServer(registry *tools.Registry, w io.Writer) *Server {
	return &Server{registry: registry, enc: json.NewEncoder(w)}
}

// Serve reads newline-delimited JSON-RPC messages from r until EOF or ctx is
// cancelled. Each request is handled synchronously; MCP over stdio uses
// ordered, single-threaded delivery.
func (s *Server) Serve(ctx context.Context, r io.Reader) error {
	scanner := bufio.NewScanner(r)
	// Allow large tool payloads (default 64KB is easy to exceed with history).
	scanner.Buffer(make([]byte, 64*1024), 8*1024*1024)

	for scanner.Scan() {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var req request
		if err := json.Unmarshal(line, &req); err != nil {
			s.writeError(nil, codeParseError, "parse error", err.Error())
			continue
		}
		if req.JSONRPC != "2.0" {
			s.writeError(req.ID, codeInvalidRequest, "invalid jsonrpc version", req.JSONRPC)
			continue
		}
		s.dispatch(ctx, req)
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("rpc: read: %w", err)
	}
	return nil
}

func (s *Server) dispatch(ctx context.Context, req request) {
	isNotification := len(req.ID) == 0

	switch req.Method {
	case "initialize":
		s.writeResult(req.ID, map[string]any{
			"protocolVersion": protocolVersion,
			"capabilities": map[string]any{
				"tools": map[string]any{"listChanged": false},
			},
			"serverInfo": map[string]any{
				"name":    serverName,
				"version": serverVersion,
			},
		})
	case "notifications/initialized", "initialized":
		// Client-sent notification; no response.
	case "ping":
		s.writeResult(req.ID, map[string]any{})
	case "tools/list":
		s.writeResult(req.ID, map[string]any{"tools": s.registry.List()})
	case "tools/call":
		s.handleToolCall(ctx, req)
	default:
		if isNotification {
			return
		}
		s.writeError(req.ID, codeMethodNotFound, "method not found", req.Method)
	}
}

func (s *Server) handleToolCall(ctx context.Context, req request) {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if len(req.Params) > 0 {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			s.writeError(req.ID, codeInvalidParams, "invalid params", err.Error())
			return
		}
	}
	if params.Name == "" {
		s.writeError(req.ID, codeInvalidParams, "tool name required", nil)
		return
	}

	result, err := s.registry.Call(ctx, params.Name, params.Arguments)
	if err != nil {
		// Unknown tool is a protocol error; everything else is a tool-level
		// error reported via isError so the model can see the message.
		if errors.Is(err, tools.ErrUnknownTool) {
			s.writeError(req.ID, codeMethodNotFound, err.Error(), nil)
			return
		}
		s.writeResult(req.ID, toolCallResult{
			Content: []contentItem{{Type: "text", Text: err.Error()}},
			IsError: true,
		})
		return
	}

	text, err := encodeResult(result)
	if err != nil {
		s.writeError(req.ID, codeInternalError, "encode tool result", err.Error())
		return
	}
	s.writeResult(req.ID, toolCallResult{
		Content: []contentItem{{Type: "text", Text: text}},
	})
}

func encodeResult(v any) (string, error) {
	if v == nil {
		return "", nil
	}
	if s, ok := v.(string); ok {
		return s, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (s *Server) writeResult(id json.RawMessage, result any) {
	if len(id) == 0 {
		return // notification — no reply
	}
	s.write(response{JSONRPC: "2.0", ID: id, Result: result})
}

func (s *Server) writeError(id json.RawMessage, code int, message string, data any) {
	if len(id) == 0 && code != codeParseError {
		return
	}
	s.write(response{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &rpcError{Code: code, Message: message, Data: data},
	})
}

func (s *Server) write(resp response) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if err := s.enc.Encode(resp); err != nil {
		log.Printf("rpc: write response: %v", err)
	}
}
