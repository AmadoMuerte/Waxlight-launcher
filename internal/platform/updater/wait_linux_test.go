//go:build linux

package updater

import (
	"os/exec"
	"testing"
	"time"
)

func TestWaitForParentReturnsOnceProcessExits(t *testing.T) {
	command := exec.Command("sleep", "1")
	if err := command.Start(); err != nil {
		t.Skipf("cannot start child process: %v", err)
	}
	reaped := make(chan struct{})
	go func() {
		_ = command.Wait()
		close(reaped)
	}()
	defer func() {
		<-reaped
	}()

	started := time.Now()
	WaitForParent(command.Process.Pid, 5*time.Second)
	elapsed := time.Since(started)

	if elapsed >= 2*time.Second {
		t.Fatalf("WaitForParent did not return promptly after the process exited: %v", elapsed)
	}
}

func TestWaitForParentRespectsTimeout(t *testing.T) {
	command := exec.Command("sleep", "30")
	if err := command.Start(); err != nil {
		t.Skipf("cannot start child process: %v", err)
	}
	defer func() {
		_ = command.Process.Kill()
		_ = command.Wait()
	}()

	started := time.Now()
	WaitForParent(command.Process.Pid, 300*time.Millisecond)
	elapsed := time.Since(started)

	if elapsed < 250*time.Millisecond {
		t.Fatalf("WaitForParent returned before the timeout elapsed: %v", elapsed)
	}
}

func TestWaitForParentIgnoresInvalidPID(t *testing.T) {
	WaitForParent(-1, 100*time.Millisecond)
	WaitForParent(0, 100*time.Millisecond)
}
