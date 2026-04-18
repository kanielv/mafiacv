// Package storyclient is a thin HTTP client for the mafiacv story-service.
// It mirrors the service's JSON shapes locally so the backend does not need
// to import the story-service Go module.
package storyclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const defaultTimeout = 45 * time.Second

type Player struct {
	Name    string `json:"name"`
	IsAlive bool   `json:"isAlive"`
}

type Event struct {
	EventType string `json:"eventType"`
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
	StoryType  string         `json:"storyType"`
	Round      int            `json:"round"`
	Events     []Event        `json:"events"`
	Players    []Player       `json:"players"`
	RoleConfig map[string]int `json:"roleConfig"`
}

type GenerateResponse struct {
	LobbyID   string `json:"lobbyId"`
	Round     int    `json:"round"`
	StoryType string `json:"storyType"`
	Story     string `json:"story"`
}

type Client struct {
	baseURL string
	http    *http.Client
}

func New(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		http:    &http.Client{Timeout: defaultTimeout},
	}
}

func (c *Client) InitGame(ctx context.Context, r InitRequest) error {
	return c.post(ctx, "/api/v1/story/init", r, nil)
}

func (c *Client) GenerateStory(ctx context.Context, r GenerateRequest) (GenerateResponse, error) {
	var resp GenerateResponse
	if err := c.post(ctx, "/api/v1/story/generate", r, &resp); err != nil {
		return GenerateResponse{}, err
	}
	return resp, nil
}

func (c *Client) CleanupGame(ctx context.Context, lobbyID string) error {
	return c.post(ctx, "/api/v1/story/cleanup", map[string]string{"lobbyId": lobbyID}, nil)
}

func (c *Client) post(ctx context.Context, path string, body any, out any) error {
	buf, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("storyclient: encode %s: %w", path, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(buf))
	if err != nil {
		return fmt.Errorf("storyclient: build request %s: %w", path, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("storyclient: %s: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("storyclient: %s: status %d: %s", path, resp.StatusCode, string(msg))
	}

	if out == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("storyclient: decode %s: %w", path, err)
	}
	return nil
}
