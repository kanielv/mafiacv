package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/kanielv/mafiacv/mcp-server/internal/storage"
)

type Handler func(ctx context.Context, args json.RawMessage) (any, error)

type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
	Handler     Handler         `json:"-"`
}

type Registry struct {
	tools map[string]Tool
}

func New(store *storage.Store) *Registry {
	r := &Registry{tools: map[string]Tool{}}
	r.Register(initGameTool(store))
	r.Register(storeGameEventTool(store))
	r.Register(getGameHistoryTool(store))
	r.Register(storeNarrativeTool(store))
	r.Register(cleanupGameTool(store))
	return r
}

func (r *Registry) Register(t Tool) {
	r.tools[t.Name] = t
}

func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

func (r *Registry) List() []Tool {
	out := make([]Tool, 0, len(r.tools))
	for _, t := range r.tools {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (r *Registry) Call(ctx context.Context, name string, args json.RawMessage) (any, error) {
	t, ok := r.tools[name]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownTool, name)
	}
	return t.Handler(ctx, args)
}

// decodeArgs unmarshals raw JSON args into dst, rejecting unknown fields so
// that typo'd parameters fail loudly instead of being silently dropped.
func decodeArgs(raw json.RawMessage, dst any) error {
	if len(raw) == 0 {
		raw = json.RawMessage("{}")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidArgs, err)
	}
	return nil
}

func invalidArgf(format string, a ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidArgs, fmt.Sprintf(format, a...))
}
