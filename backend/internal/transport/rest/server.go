package rest

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
)

func NewRouter() *mux.Router {
	router := mux.NewRouter()
	router.HandleFunc("/", TestHandler)
	return router
}

func TestHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello from TestHandler!")
}
