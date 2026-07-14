package tui

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDirCandidatesAndCompletion(t *testing.T) {
	base := t.TempDir()
	for _, d := range []string{"alpha", "alps", ".hidden"} {
		if err := os.MkdirAll(filepath.Join(base, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(base, "afile"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Listing a directory: only sub-directories, hidden ones excluded.
	if _, got := dirCandidates(base + string(os.PathSeparator)); !reflect.DeepEqual(got, []string{"alpha", "alps"}) {
		t.Fatalf("children = %v, want [alpha alps]", got)
	}

	// A prefix filters, and a dot prefix reveals hidden directories.
	if _, got := dirCandidates(filepath.Join(base, "alph")); !reflect.DeepEqual(got, []string{"alpha"}) {
		t.Fatalf("prefix alph = %v, want [alpha]", got)
	}
	if _, got := dirCandidates(filepath.Join(base, ".hid")); !reflect.DeepEqual(got, []string{".hidden"}) {
		t.Fatalf("prefix .hid = %v, want [.hidden]", got)
	}

	// Completion: a single match gains a trailing separator to drill further.
	if got, want := completePath(filepath.Join(base, "alph")), filepath.Join(base, "alpha")+string(os.PathSeparator); got != want {
		t.Fatalf("complete alph = %q, want %q", got, want)
	}
	// Several matches extend to the longest common prefix only.
	if got, want := completePath(base+string(os.PathSeparator)), filepath.Join(base, "alp"); got != want {
		t.Fatalf("complete (all) = %q, want %q", got, want)
	}
}
