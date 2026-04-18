package story

import (
	"fmt"
	"sort"
	"strings"

	"github.com/kanielv/mafiacv/story-service/internal/mcp"
)

// maxPriorNarratives caps how many previous stories we include for continuity.
// Two is enough context without bloating the prompt or leaking old details.
const maxPriorNarratives = 2

// BuildSystemPrompt returns the narrator persona and hard constraints. The
// theme shapes tone; empty theme produces a neutral dramatic voice.
func BuildSystemPrompt(theme string) string {
	var b strings.Builder
	b.WriteString("You are the narrator of a social deduction Mafia party game.\n")
	if theme != "" {
		fmt.Fprintf(&b, "Adopt the following tone and setting: %s.\n", theme)
	}
	b.WriteString(`Rules you MUST follow:
- Respond with 3 to 5 sentences. No more, no less.
- Keep it PG-13: dramatic and funny, never graphic.
- Never reveal which players hold hidden roles (mafia, medic, detective, etc).
- Never hint that a player holds a role (example detective looking at someone).
- Refer to players by name. Do not invent new characters.
- Stay consistent with prior narrations already written for this game.
- Output only the narration itself. No headings, no meta commentary, no stage directions.`)
	return b.String()
}

// BuildUserPrompt renders the per-request user message. It dispatches on the
// story type so each phase has its own framing, and threads the theme, roster,
// role counts, recent narratives, and current-round events into the template.
func BuildUserPrompt(req GenerateRequest, history mcp.History) (string, error) {
	if !req.StoryType.Valid() {
		return "", fmt.Errorf("story: unknown storyType %q", req.StoryType)
	}

	var b strings.Builder
	writeSetup(&b, req, history)
	writePriorNarratives(&b, history)
	writeRoster(&b, req.Players)

	switch req.StoryType {
	case StoryTypeGameIntro:
		b.WriteString("\nTask: Write the opening narration that sets the stage for round 1. ")
		b.WriteString("Introduce the town and the uneasy mood. Do not describe any events yet.")
	case StoryTypeNightRecap:
		fmt.Fprintf(&b, "\nNight %d events:\n", req.Round)
		writeEvents(&b, req.Events)
		b.WriteString("\nTask: Narrate the town waking to what happened overnight. ")
		b.WriteString("Mention outcomes (who fell, who was spared) without naming the culprits or roles or who mafia is.")
	case StoryTypeDayIntro:
		fmt.Fprintf(&b, "\nTask: Open day %d. Transition from night's aftermath into the coming debate. ", req.Round)
		b.WriteString("Hint at suspicion and the need to vote without accusing anyone.")
	case StoryTypeVoteRecap:
		fmt.Fprintf(&b, "\nDay %d vote results:\n", req.Round)
		writeEvents(&b, req.Events)
		b.WriteString("\nTask: Narrate the outcome of the vote. ")
		b.WriteString("Describe the eliminated player's fate and the town's reaction. Do not reveal their hidden role.")
	}
	return b.String(), nil
}

func writeSetup(b *strings.Builder, req GenerateRequest, history mcp.History) {
	theme := ""
	var roster []string
	var roles map[string]int

	if history.Settings != nil {
		theme = history.Settings.Theme
		roster = history.Settings.Players
		roles = history.Settings.RoleConfig
	}
	if len(roster) == 0 {
		roster = playerNames(req.Players)
	}
	if len(roles) == 0 {
		roles = req.RoleConfig
	}

	b.WriteString("Game setup:\n")
	if theme != "" {
		fmt.Fprintf(b, "- Theme: %s\n", theme)
	}
	if len(roster) > 0 {
		fmt.Fprintf(b, "- Players: %s\n", strings.Join(roster, ", "))
	}
	if len(roles) > 0 {
		fmt.Fprintf(b, "- Role counts: %s\n", formatRoleConfig(roles))
	}
}

func writePriorNarratives(b *strings.Builder, history mcp.History) {
	if len(history.Narratives) == 0 {
		return
	}
	prev := history.Narratives
	if len(prev) > maxPriorNarratives {
		prev = prev[len(prev)-maxPriorNarratives:]
	}
	b.WriteString("\nPrior narration (for continuity):\n")
	for _, n := range prev {
		fmt.Fprintf(b, "- [round %d, %s] %s\n", n.Round, n.StoryType, n.Story)
	}
}

func writeRoster(b *strings.Builder, players []Player) {
	if len(players) == 0 {
		return
	}
	alive := make([]string, 0, len(players))
	dead := make([]string, 0, len(players))
	for _, p := range players {
		if p.IsAlive {
			alive = append(alive, p.Name)
		} else {
			dead = append(dead, p.Name)
		}
	}
	b.WriteString("\nCurrent roster:\n")
	if len(alive) > 0 {
		fmt.Fprintf(b, "- Alive: %s\n", strings.Join(alive, ", "))
	}
	if len(dead) > 0 {
		fmt.Fprintf(b, "- Fallen: %s\n", strings.Join(dead, ", "))
	}
}

func writeEvents(b *strings.Builder, events []Event) {
	if len(events) == 0 {
		b.WriteString("- (no notable events)\n")
		return
	}
	for _, e := range events {
		fmt.Fprintf(b, "- %s: %s -> %s (%s)\n", e.EventType, e.Actor, e.Target, e.Result)
	}
}

func playerNames(players []Player) []string {
	out := make([]string, len(players))
	for i, p := range players {
		out[i] = p.Name
	}
	return out
}

// formatRoleConfig returns a deterministic "mafia:2, medic:1" rendering so
// tests can snapshot output and prompts stay cache-friendly.
func formatRoleConfig(roles map[string]int) string {
	keys := make([]string, 0, len(roles))
	for k := range roles {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s:%d", k, roles[k]))
	}
	return strings.Join(parts, ", ")
}
