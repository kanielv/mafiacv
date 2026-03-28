package app

import (
	"log"

	"github.com/kanielv/mafiacv/backend/internal/lobby"
	"github.com/kanielv/mafiacv/backend/internal/transport/rest"
	"github.com/kanielv/mafiacv/backend/internal/transport/ws"
)

func Run() {
	mgr := lobby.NewManager()
	hub := ws.NewHub(mgr)
	go hub.Run()

	router := rest.NewRouter(hub)
	if err := router.Run(":8080"); err != nil {
		log.Fatal(err)
	}
}
