package story

import (
	"strings"
	"testing"
	"time"

	"github.com/kanielv/mafiacv/story-service/internal/mcp"
)

func TestBuildSystemPrompt(t *testing.T) {
	withTheme := BuildSystemPrompt("1920s noir")
	if !strings.Contains(withTheme, "1920s noir") {
		t.Errorf("system prompt missing theme: %s", withTheme)
	}
	if !strings.Contains(withTheme, "3 to 5 sentences") {
		t.Errorf("system prompt missing length constraint: %s", withTheme)
	}
	if !strings.Contains(withTheme, "Never reveal") {
		t.Errorf("system prompt missing role-hiding rule: %s", withTheme)
	}

	noTheme := BuildSystemPrompt("")
	if strings.Contains(noTheme, "Adopt the following tone") {
		t.Errorf("system prompt should omit theme line when empty: %s", noTheme)
	}
}

func TestBuildUserPrompt(t *testing.T) {
	basePlayers := []Player{
		{Name: "Alice", IsAlive: true},
		{Name: "Bob", IsAlive: false},
		{Name: "Carol", IsAlive: true},
	}

	cases := []struct {
		name        string
		req         GenerateRequest
		history     mcp.History
		mustContain []string
		mustNotHave []string
	}{
		{
			name: "game_intro with settings, no prior narratives",
			req: GenerateRequest{
				LobbyID:    "L1",
				StoryType:  StoryTypeGameIntro,
				Round:      0,
				Players:    basePlayers,
				RoleConfig: map[string]int{"mafia": 2, "medic": 1},
			},
			history: mcp.History{
				Settings: &mcp.StoredSettings{
					LobbyID:    "L1",
					Theme:      "noir",
					Players:    []string{"Alice", "Bob", "Carol"},
					RoleConfig: map[string]int{"mafia": 2, "medic": 1},
				},
			},
			mustContain: []string{
				"Theme: noir",
				"Players: Alice, Bob, Carol",
				"Role counts: mafia:2, medic:1",
				"Alive: Alice, Carol",
				"Fallen: Bob",
				"opening narration",
			},
			mustNotHave: []string{"Prior narration"},
		},
		{
			name: "night_recap with events and prior narrative",
			req: GenerateRequest{
				LobbyID:   "L1",
				StoryType: StoryTypeNightRecap,
				Round:     1,
				Players:   basePlayers,
				Events: []Event{
					{EventType: "kill", Actor: "?", Target: "Bob", Result: "killed"},
					{EventType: "save", Actor: "?", Target: "Alice", Result: "saved"},
				},
			},
			history: mcp.History{
				Settings: &mcp.StoredSettings{Theme: "noir"},
				Narratives: []mcp.StoredNarrative{
					{Round: 0, StoryType: "game_intro", Story: "Night falls on Willowbrook.", CreatedAt: time.Now()},
				},
			},
			mustContain: []string{
				"Theme: noir",
				"Night 1 events:",
				"kill: ? -> Bob (killed)",
				"save: ? -> Alice (saved)",
				"Prior narration",
				"Night falls on Willowbrook.",
				"waking",
			},
		},
		{
			name: "day_intro uses round number",
			req: GenerateRequest{
				LobbyID:   "L1",
				StoryType: StoryTypeDayIntro,
				Round:     2,
				Players:   basePlayers,
			},
			mustContain: []string{"Open day 2"},
		},
		{
			name: "vote_recap with events",
			req: GenerateRequest{
				LobbyID:   "L1",
				StoryType: StoryTypeVoteRecap,
				Round:     3,
				Players:   basePlayers,
				Events: []Event{
					{EventType: "eliminate", Actor: "town", Target: "Carol", Result: "eliminated"},
				},
			},
			mustContain: []string{
				"Day 3 vote results",
				"eliminate: town -> Carol (eliminated)",
				"outcome of the vote",
			},
		},
		{
			name: "trims prior narratives to last two",
			req: GenerateRequest{
				LobbyID:   "L1",
				StoryType: StoryTypeNightRecap,
				Round:     5,
				Players:   basePlayers,
			},
			history: mcp.History{
				Narratives: []mcp.StoredNarrative{
					{Round: 1, StoryType: "night_recap", Story: "FIRST"},
					{Round: 2, StoryType: "day_intro", Story: "SECOND"},
					{Round: 3, StoryType: "vote_recap", Story: "THIRD"},
					{Round: 4, StoryType: "night_recap", Story: "FOURTH"},
				},
			},
			mustContain: []string{"THIRD", "FOURTH"},
			mustNotHave: []string{"FIRST", "SECOND"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := BuildUserPrompt(tc.req, tc.history)
			if err != nil {
				t.Fatalf("BuildUserPrompt: %v", err)
			}
			for _, want := range tc.mustContain {
				if !strings.Contains(out, want) {
					t.Errorf("expected output to contain %q\nOutput:\n%s", want, out)
				}
			}
			for _, unwanted := range tc.mustNotHave {
				if strings.Contains(out, unwanted) {
					t.Errorf("expected output NOT to contain %q\nOutput:\n%s", unwanted, out)
				}
			}
		})
	}
}

func TestBuildUserPromptInvalidType(t *testing.T) {
	_, err := BuildUserPrompt(GenerateRequest{StoryType: StoryType("bogus")}, mcp.History{})
	if err == nil {
		t.Fatalf("expected error for invalid story type")
	}
}

func TestStoryTypeValid(t *testing.T) {
	for _, s := range []StoryType{StoryTypeGameIntro, StoryTypeNightRecap, StoryTypeDayIntro, StoryTypeVoteRecap} {
		if !s.Valid() {
			t.Errorf("%q should be valid", s)
		}
	}
	if StoryType("bogus").Valid() {
		t.Errorf("bogus should be invalid")
	}
}
