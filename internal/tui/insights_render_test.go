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
				workspace:  project.Project{Name: "payments"},
				task:       project.Task{Name: "main"},
				kind:       rowSingle,
				groupStart: true,
				state:      session.Waiting,
				branch:     "main",
				working:    true,
				uptime:     2*time.Hour + 14*time.Minute,
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
			{workspace: project.Project{Name: "docs"}, task: project.Task{Name: "main"}, kind: rowSingle, groupStart: true, state: session.Stopped},
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

func TestMultiTaskGroupingAndNavigation(t *testing.T) {
	pay := project.Project{Name: "payments"}
	m := model{
		insights: insights.New(),
		items: []item{
			{workspace: pay, kind: rowHeader, groupStart: true, branch: "main"},
			{workspace: pay, task: project.Task{Name: "refactor"}, kind: rowTask, state: session.Waiting, working: true},
			{workspace: pay, task: project.Task{Name: "tests"}, kind: rowTask, state: session.Stopped},
		},
	}

	// The header is not selectable; navigation lands on task rows only.
	if m.selectable(0) {
		t.Error("header row should not be selectable")
	}
	if n := m.nextSelectable(0, 1); n != 1 {
		t.Errorf("nextSelectable from header = %d, want 1", n)
	}

	out := m.View()
	for _, want := range []string{"payments", "refactor", "tests", "Working", "Stopped"} {
		if !strings.Contains(out, want) {
			t.Errorf("View() missing %q", want)
		}
	}
}

func TestViewWithoutTranscriptFallsBack(t *testing.T) {
	m := model{
		insights: insights.New(),
		items: []item{
			{workspace: project.Project{Name: "plain"}, task: project.Task{Name: "main"}, kind: rowSingle, groupStart: true, state: session.Waiting},
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
