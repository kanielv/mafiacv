package app

import (
	"net/http"

	"github.com/kanielv/mafiacv/backend/internal/transport/rest"
)

func Run() {
	router := rest.NewRouter()

	// fmt.Println("Starting server")
	http.ListenAndServe(":8080", router)
}
