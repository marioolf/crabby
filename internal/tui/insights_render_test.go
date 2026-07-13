package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/marioolf/crabby/internal/insights"
	"github.com/marioolf/crabby/internal/project"
	"github.com/marioolf/crabby/internal/session"
)

func TestViewRendersInsightsAndSummary(t *testing.T) {
	m := model{
		insights: insights.New(),
		items: []item{
			{
				project: project.Project{Name: "payments"},
				state:   session.Waiting,
				branch:  "main",
				working: true,
				uptime:  2*time.Hour + 14*time.Minute,
				insight: insights.Insight{
					Found:         true,
					Model:         "Opus 4.8",
					SessionTokens: 132000,
					TodayTokens:   132000,
					LastActivity:  time.Now().Add(-20 * time.Second),
					Activity:      insights.Editing,
					Detail:        "Edit",
				},
			},
			{project: project.Project{Name: "docs"}, state: session.Stopped},
		},
	}

	out := m.View()
	for _, want := range []string{
		"payments",
		"Editing files", // activity from the transcript tail
		"20s ago",       // last activity
		"main",          // branch
		"up 2h14m",      // uptime
		"Opus 4.8",      // model
		"132k tok",      // session tokens
		"Stopped",       // the stopped project
		"2 workspaces",  // global summary
		"1 working",
		"132k tokens today",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("View() is missing %q", want)
		}
	}
}

func TestViewWithoutTranscriptFallsBack(t *testing.T) {
	m := model{
		insights: insights.New(),
		items: []item{
			{project: project.Project{Name: "plain"}, state: session.Waiting},
		},
	}
	out := m.View()
	if strings.Contains(out, "tok") {
		t.Error("no token figure should appear without a transcript")
	}
	if !strings.Contains(out, "Idle") {
		t.Error("an alive session with no transcript should read as Idle")
	}
}
