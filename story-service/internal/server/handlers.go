package server

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kanielv/mafiacv/story-service/internal/story"
)

const healthPingTimeout = 2 * time.Second

// storyService is the handler-facing contract satisfied by *story.Service.
// Declaring it here lets tests swap in fakes without touching the real
// orchestration layer.
type storyService interface {
	InitGame(ctx context.Context, r story.InitRequest) error
	GenerateStory(ctx context.Context, r story.GenerateRequest) (story.GenerateResponse, error)
	CleanupGame(ctx context.Context, lobbyID string) error
}

// mcpPinger is the subset of *mcp.Client needed for liveness.
type mcpPinger interface {
	Ping(ctx context.Context) error
}

func (d *deps) initGame(c *gin.Context) {
	var req story.InitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeBindError(c, err)
		return
	}
	if err := d.story.InitGame(c.Request.Context(), req); err != nil {
		writeError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (d *deps) generate(c *gin.Context) {
	var req story.GenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeBindError(c, err)
		return
	}
	resp, err := d.story.GenerateStory(c.Request.Context(), req)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (d *deps) cleanup(c *gin.Context) {
	var req struct {
		LobbyID string `json:"lobbyId"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		writeBindError(c, err)
		return
	}
	if err := d.story.CleanupGame(c.Request.Context(), req.LobbyID); err != nil {
		writeError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (d *deps) health(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), healthPingTimeout)
	defer cancel()
	if err := d.mcp.Ping(ctx); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "degraded", "mcp": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "mcp": "ok"})
}

func writeBindError(c *gin.Context, err error) {
	c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
}

func writeError(c *gin.Context, err error) {
	if errors.Is(err, story.ErrInvalid) {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
}
