package gemini

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/genai"
)

// per-request upper bound for a single Gemini call.
const generateTimeout = 25 * time.Second

type Client struct {
	sdk   *genai.Client
	model string
}

// Constructs a Gemini client.
func New(ctx context.Context, apiKey, model string) (*Client, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("gemini: apikey is required")
	}
	if model == "" {
		return nil, fmt.Errorf("gemini: model is required")
	}

	sdk, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("gemini: new client: %w", err)
	}

	return &Client{sdk: sdk, model: model}, nil
}

func (c *Client) Generate(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, generateTimeout)
	defer cancel()

	cfg := &genai.GenerateContentConfig{
		SystemInstruction: genai.NewContentFromText(systemPrompt, genai.RoleUser),
		Temperature:       genai.Ptr[float32](0.9),
	}

	contents := []*genai.Content{
		genai.NewContentFromText(userPrompt, genai.RoleUser),
	}

	resp, err := c.sdk.Models.GenerateContent(ctx, c.model, contents, cfg)
	if err != nil {
		return "", fmt.Errorf("gemini: generate content: %w", err)
	}

	text := resp.Text()
	if text == "" {
		return "", fmt.Errorf("gemini: empty response")
	}
	return text, nil
}
