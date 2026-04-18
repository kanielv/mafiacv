package server

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kanielv/mafiacv/story-service/internal/config"
)

type deps struct {
	story storyService
	mcp   mcpPinger
}

func New(cfg *config.Config, svc storyService, mcpClient mcpPinger) *http.Server {
	router := gin.Default()
	d := &deps{story: svc, mcp: mcpClient}
	registerRoutes(router, d)

	return &http.Server{
		Addr:    cfg.Port,
		Handler: router,
	}
}

func registerRoutes(r *gin.Engine, d *deps) {
	v1 := r.Group("/api/v1")
	v1.GET("/health", d.health)

	s := v1.Group("/story")
	s.POST("/init", d.initGame)
	s.POST("/generate", d.generate)
	s.POST("/cleanup", d.cleanup)
}

func Run(srv *http.Server) {
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	log.Printf("story-service listening on %s", srv.Addr)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("shutdown error: %v", err)
	}
	log.Println("server stopped")
}
