package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/kanielv/mafiacv/story-service/internal/story"
)

type fakeService struct {
	initErr     error
	generateErr error
	cleanupErr  error
	resp        story.GenerateResponse

	lastInit     story.InitRequest
	lastGenerate story.GenerateRequest
	lastCleanup  string
}

func (f *fakeService) InitGame(_ context.Context, r story.InitRequest) error {
	f.lastInit = r
	return f.initErr
}

func (f *fakeService) GenerateStory(_ context.Context, r story.GenerateRequest) (story.GenerateResponse, error) {
	f.lastGenerate = r
	if f.generateErr != nil {
		return story.GenerateResponse{}, f.generateErr
	}
	return f.resp, nil
}

func (f *fakeService) CleanupGame(_ context.Context, lobbyID string) error {
	f.lastCleanup = lobbyID
	return f.cleanupErr
}

type fakePinger struct{ err error }

func (f *fakePinger) Ping(_ context.Context) error { return f.err }

func newRouter(svc storyService, p mcpPinger) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	registerRoutes(r, &deps{story: svc, mcp: p})
	return r
}

func do(r *gin.Engine, method, path string, body any) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestInit_HappyPath(t *testing.T) {
	svc := &fakeService{}
	r := newRouter(svc, &fakePinger{})
	w := do(r, "POST", "/api/v1/story/init", story.InitRequest{
		LobbyID:    "L1",
		Players:    []string{"A", "B"},
		RoleConfig: map[string]int{"mafia": 1},
		Theme:      "noir",
	})
	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	if svc.lastInit.LobbyID != "L1" {
		t.Errorf("service not invoked with lobbyId, got %+v", svc.lastInit)
	}
}

func TestInit_InvalidMapsTo400(t *testing.T) {
	svc := &fakeService{initErr: fmt.Errorf("%w: lobbyId is required", story.ErrInvalid)}
	r := newRouter(svc, &fakePinger{})
	w := do(r, "POST", "/api/v1/story/init", map[string]any{})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "lobbyId is required") {
		t.Errorf("unexpected body: %s", w.Body.String())
	}
}

func TestGenerate_HappyPath(t *testing.T) {
	svc := &fakeService{resp: story.GenerateResponse{
		LobbyID:   "L1",
		Round:     0,
		StoryType: story.StoryTypeGameIntro,
		Story:     "A shadow fell.",
	}}
	r := newRouter(svc, &fakePinger{})
	w := do(r, "POST", "/api/v1/story/generate", story.GenerateRequest{
		LobbyID:   "L1",
		StoryType: story.StoryTypeGameIntro,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var got story.GenerateResponse
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Story != "A shadow fell." || got.StoryType != story.StoryTypeGameIntro {
		t.Errorf("unexpected response: %+v", got)
	}
}

func TestGenerate_BadJSON(t *testing.T) {
	r := newRouter(&fakeService{}, &fakePinger{})
	req := httptest.NewRequest("POST", "/api/v1/story/generate", strings.NewReader("{not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
}

func TestGenerate_InternalError(t *testing.T) {
	svc := &fakeService{generateErr: errors.New("llm outage")}
	r := newRouter(svc, &fakePinger{})
	w := do(r, "POST", "/api/v1/story/generate", story.GenerateRequest{
		LobbyID: "L1", StoryType: story.StoryTypeGameIntro,
	})
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestCleanup_HappyPath(t *testing.T) {
	svc := &fakeService{}
	r := newRouter(svc, &fakePinger{})
	w := do(r, "POST", "/api/v1/story/cleanup", map[string]string{"lobbyId": "L1"})
	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d", w.Code)
	}
	if svc.lastCleanup != "L1" {
		t.Errorf("lobbyID not passed through, got %q", svc.lastCleanup)
	}
}

func TestHealth_OK(t *testing.T) {
	r := newRouter(&fakeService{}, &fakePinger{})
	w := do(r, "GET", "/api/v1/health", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"mcp":"ok"`) {
		t.Errorf("body missing mcp ok: %s", w.Body.String())
	}
}

func TestHealth_MCPDown(t *testing.T) {
	r := newRouter(&fakeService{}, &fakePinger{err: errors.New("pipe closed")})
	w := do(r, "GET", "/api/v1/health", nil)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "pipe closed") {
		t.Errorf("body missing mcp error: %s", w.Body.String())
	}
}
