package project

import "testing"

func TestRegisterFindAndUpdate(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	// Empty registry to start.
	got, err := Load()
	if err != nil {
		t.Fatalf("Load empty: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty registry, got %d", len(got))
	}

	p := Project{Name: "payments", Path: "/home/x/payments", Session: "crabby_payments"}
	if err := Register(p); err != nil {
		t.Fatalf("Register: %v", err)
	}

	found, err := Find("payments")
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if found != p {
		t.Fatalf("Find returned %+v, want %+v", found, p)
	}

	// Re-registering the same name updates in place, not appends.
	p.Path = "/home/x/moved"
	if err := Register(p); err != nil {
		t.Fatalf("Register update: %v", err)
	}
	all, _ := Load()
	if len(all) != 1 {
		t.Fatalf("expected 1 project after update, got %d", len(all))
	}

	byPath, err := FindByPath("/home/x/moved")
	if err != nil {
		t.Fatalf("FindByPath: %v", err)
	}
	if byPath.Name != "payments" {
		t.Fatalf("FindByPath name = %q", byPath.Name)
	}
}

func TestFindNotFound(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if _, err := Find("nope"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestRemove(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if err := Register(Project{Name: "a", Path: "/a", Session: "crabby_a"}); err != nil {
		t.Fatal(err)
	}
	if err := Register(Project{Name: "b", Path: "/b", Session: "crabby_b"}); err != nil {
		t.Fatal(err)
	}

	if err := Remove("a"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := Find("a"); err != ErrNotFound {
		t.Fatalf("project 'a' still present after Remove")
	}
	if _, err := Find("b"); err != nil {
		t.Fatalf("Remove deleted the wrong project: %v", err)
	}

	if err := Remove("missing"); err != ErrNotFound {
		t.Fatalf("Remove(missing) = %v, want ErrNotFound", err)
	}
}
