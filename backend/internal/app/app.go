package app

import (
	"log"

	"github.com/kanielv/mafiacv/backend/internal/transport/rest"
)

func Run() {
	router := rest.NewRouter()
	if err := router.Run(":8080"); err != nil {
		log.Fatal(err)
	}
}
