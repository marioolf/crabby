// Package insights reads AI Agent's own session transcripts to surface
// factual, per-project information on the dashboard — the model in use, tokens
// spent, and a best-effort hint of what Agent is doing right now.
//
// Everything here comes from the JSONL transcripts AI Agent writes under
// ~/.claude/projects/<encoded-path>/<session-id>.jsonl. That is a durable,
// append-only file source: no terminal scraping, no guessing. Fields that
// cannot be read reliably are simply left empty rather than estimated.
//
// Limitations (by design, not defects):
//   - Activity is inferred from the tail of the transcript, which can lag the
//     live terminal by a moment; callers gate it on tmux's "working" signal.
//   - A project is matched to its transcript directory by re-encoding its path
//     the way AI Agent does ('/' and '.' become '-'); if that directory is
//     absent we fall back to reading the `cwd` recorded inside each transcript.
//   - There is no reliable source for progress percentages or completion times,
//     so none are produced.
package insights

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Activity is a coarse, transcript-derived hint of the Agent's current action. It
// describes what the last recorded step was; callers decide how to present it
// (e.g. only while the session is actively producing output).
type Activity string

const (
	Unknown    Activity = ""
	Thinking   Activity = "thinking"
	Editing    Activity = "editing" // Edit/Write/NotebookEdit
	Reading    Activity = "reading" // Read/Grep/Glob/LS
	Running    Activity = "running" // Bash or any other tool (see Detail)
	Responding Activity = "responding"
)

// Insight is the factual snapshot for one project. Zero values mean "unknown" —
// a caller should only render fields that carry data (Found reports whether a
// transcript was located at all).
type Insight struct {
	Found         bool
	Model         string    // friendly name, e.g. "Opus 4.8"; "" if unknown
	SessionTokens int       // input+output across the current session transcript
	TodayTokens   int       // input+output dated today, across the project's transcripts
	LastActivity  time.Time // timestamp of the last transcript record
	Activity      Activity  // what the tail of the transcript shows
	Detail        string    // tool name when Activity is Running
}

// Collector reads transcripts and caches per-file parsing state so repeated
// dashboard refreshes only touch the bytes that were appended since last time.
// It is safe for concurrent use.
type Collector struct {
	mu    sync.Mutex
	root  string                // ~/.claude/projects, "" if unavailable
	files map[string]*fileState // keyed by transcript path
	dirs  map[string]string     // project abs path -> transcript dir (fallback cache)
}

// fileState is the incrementally-maintained parse of a single transcript file.
type fileState struct {
	offset      int64          // bytes parsed so far
	tokensByDay map[string]int // "2006-01-02" (local) -> input+output tokens
	model       string
	lastTS      time.Time
	activity    Activity
	detail      string
}

// New returns a Collector rooted at the current user's Agent projects
// directory. If it cannot be located, the Collector still works and simply
// reports every project as having no transcript.
func New() *Collector {
	c := &Collector{
		files: map[string]*fileState{},
		dirs:  map[string]string{},
	}
	if home, err := os.UserHomeDir(); err == nil {
		c.root = filepath.Join(home, ".claude", "projects")
	}
	return c
}

// For returns the current insight for a project rooted at path. It re-reads only
// the transcript bytes added since the previous call.
func (c *Collector) For(path string) Insight {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.root == "" {
		return Insight{}
	}
	dir := c.transcriptDir(path)
	if dir == "" {
		return Insight{}
	}
	transcripts, err := filepath.Glob(filepath.Join(dir, "*.jsonl"))
	if err != nil || len(transcripts) == 0 {
		return Insight{}
	}

	now := time.Now()
	today := now.Format("2006-01-02")
	startOfToday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	var current string
	var currentMod time.Time
	todayTokens := 0
	for _, f := range transcripts {
		info, err := os.Stat(f)
		if err != nil {
			continue
		}
		// A file untouched before today cannot hold today's records, so skip the
		// read entirely — this is what keeps the scan cheap.
		if !info.ModTime().Before(startOfToday) {
			st := c.update(f, info.Size())
			todayTokens += st.tokensByDay[today]
		}
		if info.ModTime().After(currentMod) {
			current, currentMod = f, info.ModTime()
		}
	}

	// The most recently written transcript is the active session: its model,
	// tail activity, and running token total describe "now".
	cur := c.update(current, sizeOf(current))
	ins := Insight{
		Found:         true,
		Model:         sanitizeString(friendlyModel(cur.model)),
		SessionTokens: sum(cur.tokensByDay),
		TodayTokens:   todayTokens,
		LastActivity:  cur.lastTS,
		Activity:      cur.activity,
		Detail:        sanitizeString(cur.detail),
	}
	return ins
}

// update parses any newly-appended lines of a transcript and returns its cached
// state. Callers hold c.mu.
func (c *Collector) update(path string, size int64) *fileState {
	st := c.files[path]
	if st == nil {
		st = &fileState{tokensByDay: map[string]int{}}
		c.files[path] = st
	}
	// File shrank or was replaced (new session reusing a name): start over.
	if size < st.offset {
		*st = fileState{tokensByDay: map[string]int{}}
	}
	if size == st.offset {
		return st // nothing new
	}

	f, err := os.Open(path)
	if err != nil {
		return st
	}
	defer f.Close()
	if _, err := f.Seek(st.offset, 0); err != nil {
		return st
	}

	r := bufio.NewReader(f)
	var consumed int64
	for {
		line, err := r.ReadBytes('\n')
		if err != nil {
			// No trailing newline yet: a partial line still being written. Leave
			// it unconsumed so it is re-read once complete.
			break
		}
		consumed += int64(len(line))
		st.apply(line)
	}
	st.offset += consumed
	return st
}

// record is the minimal shape we read from a transcript line.
type record struct {
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
	Message   *struct {
		Role  string `json:"role"`
		Model string `json:"model"`
		Usage *struct {
			Input  int `json:"input_tokens"`
			Output int `json:"output_tokens"`
		} `json:"usage"`
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

// contentBlock is one item of an assistant message's content array.
type contentBlock struct {
	Type string `json:"type"` // "thinking" | "text" | "tool_use"
	Name string `json:"name"` // tool name when Type == "tool_use"
}

// apply folds a single transcript line into the file's cached state.
func (st *fileState) apply(line []byte) {
	var rec record
	if json.Unmarshal(line, &rec) != nil {
		return
	}
	if ts := parseTime(rec.Timestamp); !ts.IsZero() {
		st.lastTS = ts
	}
	if rec.Message == nil {
		return
	}
	if rec.Message.Role == "assistant" {
		// AI Agent tags some injected records with a "<synthetic>" model;
		// only accept genuine model ids so that marker never reaches the UI.
		if strings.HasPrefix(rec.Message.Model, "claude-") {
			st.model = rec.Message.Model
		}
		if u := rec.Message.Usage; u != nil {
			day := parseTime(rec.Timestamp).Format("2006-01-02")
			st.tokensByDay[day] += u.Input + u.Output
		}
		st.activity, st.detail = activityOf(rec.Message.Content)
	} else if rec.Message.Role == "user" {
		// A user turn means the Agent's previous turn is finished; the next
		// assistant record will set the real activity again.
		st.activity, st.detail = Responding, ""
	}
}

// activityOf reads an assistant message's content blocks and returns what the
// final meaningful block indicates Agent is doing.
func activityOf(content json.RawMessage) (Activity, string) {
	var blocks []contentBlock
	if json.Unmarshal(content, &blocks) != nil || len(blocks) == 0 {
		return Unknown, ""
	}
	last := blocks[len(blocks)-1]
	switch last.Type {
	case "tool_use":
		switch last.Name {
		case "Edit", "Write", "NotebookEdit", "MultiEdit":
			return Editing, last.Name
		case "Read", "Grep", "Glob", "LS":
			return Reading, last.Name
		default:
			return Running, last.Name
		}
	case "thinking":
		return Thinking, ""
	case "text":
		return Responding, ""
	}
	return Unknown, ""
}

// transcriptDir resolves the AI Agent transcript directory for a project
// path, preferring the direct encoding and falling back to a one-time scan that
// matches the `cwd` recorded inside transcripts. Callers hold c.mu.
func (c *Collector) transcriptDir(path string) string {
	if dir, ok := c.dirs[path]; ok {
		return dir
	}
	encoded := filepath.Join(c.root, encodePath(path))
	if isDir(encoded) {
		c.dirs[path] = encoded
		return encoded
	}
	// Fallback: some paths do not round-trip through the encoding (collisions,
	// unusual characters). Match on the cwd stored in each transcript instead.
	dir := c.scanForCwd(path)
	c.dirs[path] = dir // cache even a miss ("") to avoid rescanning every refresh
	return dir
}

// scanForCwd looks through every project directory for a transcript whose first
// record's cwd matches path. Callers hold c.mu.
func (c *Collector) scanForCwd(path string) string {
	entries, err := os.ReadDir(c.root)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(c.root, e.Name())
		files, _ := filepath.Glob(filepath.Join(dir, "*.jsonl"))
		for _, f := range files {
			if firstCwd(f) == path {
				return dir
			}
		}
	}
	return ""
}

// encodePath mirrors AI Agent's transcript-directory naming: every '/' and
// '.' in the absolute path becomes '-'.
func encodePath(path string) string {
	return strings.NewReplacer("/", "-", ".", "-").Replace(path)
}

// friendlyModel turns a model id like "claude-opus-4-8" into "Opus 4.8".
func friendlyModel(id string) string {
	switch {
	case id == "":
		return ""
	case strings.HasPrefix(id, "claude-opus-4-8"):
		return "Opus 4.8"
	case strings.HasPrefix(id, "claude-opus-4-7"):
		return "Opus 4.7"
	case strings.HasPrefix(id, "claude-sonnet-5"):
		return "Sonnet 5"
	case strings.HasPrefix(id, "claude-haiku-4-5"):
		return "Haiku 4.5"
	case strings.HasPrefix(id, "claude-fable-5"):
		return "Fable 5"
	}
	// Unknown id: strip vendor prefix so synthetic/future models present cleanly.
	if strings.HasPrefix(id, "claude-") {
		return strings.TrimPrefix(id, "claude-")
	}
	return id
}

// --- small helpers ---------------------------------------------------------

func sanitizeString(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r == 0x1b || r == 0x7f {
			continue
		}
		if (r >= 0x00 && r <= 0x1f && r != '\n' && r != '\t') || (r >= 0x80 && r <= 0x9f) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func sum(m map[string]int) int {
	total := 0
	for _, v := range m {
		total += v
	}
	return total
}

func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t.Local()
}

func sizeOf(path string) int64 {
	if info, err := os.Stat(path); err == nil {
		return info.Size()
	}
	return 0
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// firstCwd returns the cwd recorded in a transcript's first parseable record.
func firstCwd(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		var rec struct {
			Cwd string `json:"cwd"`
		}
		if json.Unmarshal(sc.Bytes(), &rec) == nil && rec.Cwd != "" {
			return rec.Cwd
		}
	}
	return ""
}
