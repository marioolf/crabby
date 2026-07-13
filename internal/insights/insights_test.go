package insights

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeTranscript creates a transcript file for projectPath under a temp HOME,
// returning the project path. lines are raw JSONL records.
func writeTranscript(t *testing.T, projectPath, name string, lines ...string) {
	t.Helper()
	home := os.Getenv("HOME")
	dir := filepath.Join(home, ".claude", "projects", encodePath(projectPath))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// asst builds an assistant record with the given timestamp, tokens, and a
// single trailing content block of the given type/name.
func asst(ts time.Time, in, out int, blockType, toolName string) string {
	return fmt.Sprintf(
		`{"type":"assistant","timestamp":%q,"message":{"role":"assistant","model":"claude-opus-4-8","usage":{"input_tokens":%d,"output_tokens":%d},"content":[{"type":%q,"name":%q}]}}`,
		ts.Format(time.RFC3339), in, out, blockType, toolName)
}

func TestForReadsModelTokensAndActivity(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	proj := filepath.Join(t.TempDir(), "payments")
	now := time.Now()
	writeTranscript(t, proj, "s1.jsonl",
		asst(now.Add(-2*time.Minute), 100, 50, "text", ""),
		asst(now.Add(-1*time.Minute), 200, 80, "tool_use", "Edit"),
	)

	ins := New().For(proj)
	if !ins.Found {
		t.Fatal("Found = false, want true")
	}
	if ins.Model != "Opus 4.8" {
		t.Fatalf("Model = %q, want Opus 4.8", ins.Model)
	}
	if ins.SessionTokens != 100+50+200+80 {
		t.Fatalf("SessionTokens = %d, want 430", ins.SessionTokens)
	}
	if ins.TodayTokens != 430 {
		t.Fatalf("TodayTokens = %d, want 430", ins.TodayTokens)
	}
	if ins.Activity != Editing || ins.Detail != "Edit" {
		t.Fatalf("activity = %q/%q, want editing/Edit", ins.Activity, ins.Detail)
	}
	if ins.LastActivity.IsZero() {
		t.Fatal("LastActivity not set")
	}
}

func TestForActivityFromTail(t *testing.T) {
	cases := []struct {
		blockType, tool string
		want            Activity
		detail          string
	}{
		{"thinking", "", Thinking, ""},
		{"text", "", Responding, ""},
		{"tool_use", "Bash", Running, "Bash"},
		{"tool_use", "Read", Reading, "Read"},
		{"tool_use", "Write", Editing, "Write"},
	}
	for _, c := range cases {
		t.Run(c.blockType+"/"+c.tool, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			proj := filepath.Join(t.TempDir(), "p")
			writeTranscript(t, proj, "s.jsonl", asst(time.Now(), 1, 1, c.blockType, c.tool))
			ins := New().For(proj)
			if ins.Activity != c.want || ins.Detail != c.detail {
				t.Fatalf("got %q/%q, want %q/%q", ins.Activity, ins.Detail, c.want, c.detail)
			}
		})
	}
}

func TestForTodayVsSessionTokens(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	proj := filepath.Join(t.TempDir(), "p")
	now := time.Now()
	writeTranscript(t, proj, "s.jsonl",
		asst(now.AddDate(0, 0, -1), 1000, 1000, "text", ""), // yesterday
		asst(now, 300, 200, "text", ""),                     // today
	)
	ins := New().For(proj)
	if ins.SessionTokens != 2500 {
		t.Fatalf("SessionTokens = %d, want 2500", ins.SessionTokens)
	}
	if ins.TodayTokens != 500 {
		t.Fatalf("TodayTokens = %d, want 500 (today only)", ins.TodayTokens)
	}
}

func TestForIncrementalAppend(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	home := os.Getenv("HOME")
	proj := filepath.Join(t.TempDir(), "p")
	writeTranscript(t, proj, "s.jsonl", asst(time.Now(), 100, 100, "text", ""))

	c := New()
	if got := c.For(proj).SessionTokens; got != 200 {
		t.Fatalf("first read = %d, want 200", got)
	}

	// Append another turn and re-read; only the new bytes should be parsed.
	path := filepath.Join(home, ".claude", "projects", encodePath(proj), "s.jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Fprintln(f, asst(time.Now(), 50, 50, "tool_use", "Bash"))
	f.Close()

	ins := c.For(proj)
	if ins.SessionTokens != 300 {
		t.Fatalf("after append = %d, want 300", ins.SessionTokens)
	}
	if ins.Activity != Running || ins.Detail != "Bash" {
		t.Fatalf("activity = %q/%q, want running/Bash", ins.Activity, ins.Detail)
	}
}

func TestForNoTranscript(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ins := New().For(filepath.Join(t.TempDir(), "nothing"))
	if ins.Found {
		t.Fatal("Found = true for a project with no transcript")
	}
}

func TestFriendlyModel(t *testing.T) {
	cases := map[string]string{
		"claude-opus-4-8":           "Opus 4.8",
		"claude-sonnet-5":           "Sonnet 5",
		"claude-haiku-4-5-20251001": "Haiku 4.5",
		"":                          "",
		"claude-future-9":           "future-9", // unknown: vendor prefix stripped
	}
	for id, want := range cases {
		if got := friendlyModel(id); got != want {
			t.Fatalf("friendlyModel(%q) = %q, want %q", id, got, want)
		}
	}
}
