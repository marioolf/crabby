package windows

import (
	"reflect"
	"testing"
)

func TestWSLArgsPassesPathAndArgsPositionally(t *testing.T) {
	got := wslArgs(`C:\Users\mario\work`, []string{"start", "payments"})
	want := []string{
		"bash", "-lc", remoteScript,
		"crabby",              // $0
		`C:\Users\mario\work`, // $1 (converted by wslpath inside the shell)
		"start", "payments",   // $2, $3 -> "$@" after shift
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("wslArgs mismatch\n got: %#v\nwant: %#v", got, want)
	}
}

func TestWSLArgsNoArgs(t *testing.T) {
	got := wslArgs(`C:\proj`, nil)
	want := []string{"bash", "-lc", remoteScript, "crabby", `C:\proj`}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("wslArgs mismatch\n got: %#v\nwant: %#v", got, want)
	}
}
