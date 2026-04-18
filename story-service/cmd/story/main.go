package main

import (
	"context"
	"log"

	"github.com/kanielv/mafiacv/story-service/internal/config"
	"github.com/kanielv/mafiacv/story-service/internal/gemini"
	"github.com/kanielv/mafiacv/story-service/internal/mcp"
	"github.com/kanielv/mafiacv/story-service/internal/server"
	"github.com/kanielv/mafiacv/story-service/internal/story"
)

func main() {
	cfg := config.Load()

	ctx := context.Background()
	geminiClient, err := gemini.New(ctx, cfg.GeminiAPIKey, cfg.GeminiModel)
	if err != nil {
		log.Fatalf("gemini init: %v", err)
	}

	mcpClient, err := mcp.New(ctx, cfg.MCPBinaryPath, cfg.MCPDBPath)
	if err != nil {
		log.Fatalf("mcp init: %v", err)
	}
	defer func() {
		if err := mcpClient.Close(); err != nil {
			log.Printf("mcp close: %v", err)
		}
	}()

	svc := story.New(geminiClient, mcpClient)

	srv := server.New(cfg, svc, mcpClient)
	server.Run(srv)
}
