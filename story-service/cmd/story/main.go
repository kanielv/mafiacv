package main

import (
	"context"
	"log"

	"github.com/kanielv/mafiacv/story-service/internal/config"
	"github.com/kanielv/mafiacv/story-service/internal/gemini"
	"github.com/kanielv/mafiacv/story-service/internal/server"
)

func main() {
	cfg := config.Load()

	ctx := context.Background()
	client, err := gemini.New(ctx, cfg.GeminiAPIKey, cfg.GeminiModel)
	if err != nil {
		log.Fatalf("gemini init: %v", err)
	}

	systemPrompt := "You are a dramatic noir narrator for a Mafia party game. Respond in 3-5 sentences."
	userPrompt := "Night 1 recap: the mafia killed Alice. The medic saved no one. Narrate the town waking up."

	story, err := client.Generate(ctx, systemPrompt, userPrompt)
	if err != nil {
		log.Fatalf("gemini test generate: %v", err)
	}

	log.Printf("gemini test story: %s", story)
	srv := server.New(cfg)
	server.Run(srv)
}
