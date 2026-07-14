package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/marioolf/crabby/internal/config"
	"github.com/marioolf/crabby/internal/insights"
	"github.com/marioolf/crabby/internal/project"
	"github.com/marioolf/crabby/internal/session"
	"github.com/marioolf/crabby/internal/tmux"
)

// dashModel builds a dashboard model wide enough that the detail column is not
// truncated, so substring assertions on rendered values are reliable.
func dashModel(rows []wsRow) model {
	return model{
		cfg:          config.Default(),
		tmux:         tmux.New("tmux"),
		insights:     insights.New(),
		width:        200,
		height:       50,
		rows:         rows,
		lastActivity: map[string]time.Time{},
		working:      map[string]bool{},
	}
}

func TestDashboardRendersInsightsAndSummary(t *testing.T) {
	rows := []wsRow{
		{
			proj:   project.Project{Name: "payments"},
			branch: "main",
			insight: insights.Insight{
				Found: true, Model: "Opus 4.8",
				SessionTokens: 132000, TodayTokens: 132000,
				LastActivity: time.Now().Add(-20 * time.Second),
				Activity:     insights.Editing, Detail: "Edit",
			},
			tasks: []taskRow{{
				task:    project.Task{Name: "main"},
				state:   session.Waiting,
				working: true,
				uptime:  2*time.Hour + 14*time.Minute,
			}},
		},
		{
			proj:  project.Project{Name: "docs"},
			tasks: []taskRow{{task: project.Task{Name: "main"}, state: session.Stopped}},
		},
	}
	out := dashModel(rows).View()
	for _, want := range []string{
		"payments",
		"docs",
		"Opus 4.8",      // model, in the detail column
		"132k",          // token figure
		"Editing files", // activity from the transcript tail
		"20s ago",       // last activity
		"2h14m",         // uptime
		"2 workspaces",  // global summary
		"1 working",
		"132k tokens today",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("dashboard is missing %q", want)
		}
	}
}

func TestDashboardTaskNavigation(t *testing.T) {
	rows := []wsRow{{
		proj: project.Project{Name: "payments"},
		tasks: []taskRow{
			{task: project.Task{Name: "refactor"}, state: session.Waiting, working: true},
			{task: project.Task{Name: "tests"}, state: session.Stopped},
		},
	}}
	m := dashModel(rows)
	m.focus = paneTasks

	if _, tr, ok := m.selectedTask(); !ok || tr.task.Name != "refactor" {
		t.Fatalf("initial task = %+v (ok %v), want refactor", tr, ok)
	}
	m.moveDown()
	if _, tr, _ := m.selectedTask(); tr.task.Name != "tests" {
		t.Fatalf("after moveDown task = %q, want tests", tr.task.Name)
	}
	m.moveDown() // clamp at the last task
	if m.taskCursor != 1 {
		t.Fatalf("taskCursor = %d, want it clamped at 1", m.taskCursor)
	}

	out := m.View()
	for _, want := range []string{"refactor", "tests", "Stopped"} {
		if !strings.Contains(out, want) {
			t.Errorf("dashboard is missing %q", want)
		}
	}
}

func TestDashboardWithoutTranscriptFallsBack(t *testing.T) {
	rows := []wsRow{{
		proj:  project.Project{Name: "plain"},
		tasks: []taskRow{{task: project.Task{Name: "main"}, state: session.Waiting}},
	}}
	out := dashModel(rows).View()
	if strings.Contains(out, "tok") {
		t.Error("no token figure should appear without a transcript")
	}
	if !strings.Contains(out, "Idle") {
		t.Error("an alive session with no transcript should read as Idle")
	}
}

func TestFormatHelpers(t *testing.T) {
	cases := []struct{ got, want string }{
		{formatTokens(950), "950"},
		{formatTokens(12000), "12k"},
		{formatDuration(5 * time.Minute), "5m"},
		{formatDuration(2*time.Hour + 14*time.Minute), "2h14m"},
		{formatAgo(20 * time.Second), "20s ago"},
		{plural(1, "workspace"), "1 workspace"},
		{plural(3, "workspace"), "3 workspaces"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("got %q, want %q", c.got, c.want)
		}
	}
}
