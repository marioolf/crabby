package session

import (
	"sort"
	"testing"
)

func TestClassify(t *testing.T) {
	cases := []struct {
		present, attached bool
		want              State
	}{
		{false, false, Stopped},
		{true, false, Waiting},
		{true, true, Running},
	}
	for _, c := range cases {
		if got := Classify(c.present, c.attached); got != c.want {
			t.Errorf("Classify(%v,%v) = %q, want %q", c.present, c.attached, got, c.want)
		}
	}
}

func TestRankOrdersRunningWaitingStopped(t *testing.T) {
	states := []State{Stopped, Running, Waiting, Stopped, Running}
	sort.SliceStable(states, func(i, j int) bool { return Rank(states[i]) < Rank(states[j]) })
	want := []State{Running, Running, Waiting, Stopped, Stopped}
	for i := range want {
		if states[i] != want[i] {
			t.Fatalf("sorted = %v, want %v", states, want)
		}
	}
}

func TestName(t *testing.T) {
	if Name("payments") != "crabby_payments" {
		t.Fatalf("Name = %q", Name("payments"))
	}
}
