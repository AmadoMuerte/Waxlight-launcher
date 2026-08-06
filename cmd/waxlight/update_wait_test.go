package main

import "testing"

func TestUpdateWaitPIDFromArgs(t *testing.T) {
	if got := updateWaitPIDFromArgs([]string{"--update-wait-pid", "1234"}); got != 1234 {
		t.Fatalf("expected pid 1234, got %d", got)
	}
	if got := updateWaitPIDFromArgs([]string{"-v", "--update-wait-pid", "42", "extra"}); got != 42 {
		t.Fatalf("expected pid 42 after unrelated flags, got %d", got)
	}
	if got := updateWaitPIDFromArgs(nil); got != 0 {
		t.Fatalf("expected 0 for empty args, got %d", got)
	}
	if got := updateWaitPIDFromArgs([]string{"--update-wait-pid"}); got != 0 {
		t.Fatalf("expected 0 when the pid is missing, got %d", got)
	}
	if got := updateWaitPIDFromArgs([]string{"--update-wait-pid", "abc"}); got != 0 {
		t.Fatalf("expected 0 for a malformed pid, got %d", got)
	}
	if got := updateWaitPIDFromArgs([]string{"--update-wait-pid", "0"}); got != 0 {
		t.Fatalf("expected 0 for a zero pid, got %d", got)
	}
	if got := updateWaitPIDFromArgs([]string{"--update-wait-pid", "-5"}); got != 0 {
		t.Fatalf("expected 0 for a negative pid, got %d", got)
	}
	if got := updateWaitPIDFromArgs([]string{"--update-wait-pid"}); got != 0 {
		t.Fatalf("expected 0 for a dangling flag, got %d", got)
	}
}
