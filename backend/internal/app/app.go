package app

import (
	"log"
	"os"

	"github.com/kanielv/mafiacv/backend/internal/lobby"
	"github.com/kanielv/mafiacv/backend/internal/storyclient"
	"github.com/kanielv/mafiacv/backend/internal/transport/rest"
	"github.com/kanielv/mafiacv/backend/internal/transport/ws"
)

func Run() {
	storyURL := os.Getenv("STORY_SERVICE_URL")
	if storyURL == "" {
		storyURL = "http://localhost:8090"
	}
	story := storyclient.New(storyURL)

	mgr := lobby.NewManager()
	hub := ws.NewHub(mgr, story)
	go hub.Run()

	router := rest.NewRouter(hub)
	if err := router.Run(":8080"); err != nil {
		log.Fatal(err)
	}
}
