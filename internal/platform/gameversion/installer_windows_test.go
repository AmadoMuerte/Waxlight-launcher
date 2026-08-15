//go:build windows

package gameversion

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

const installerHelperMode = "WAXLIGHT_INSTALLER_HELPER"

func TestRunWindowsInstallerCancelsProcessTree(t *testing.T) {
	target := t.TempDir()
	signal := filepath.Join(target, "child-started")
	ctx, cancel := context.WithCancel(context.Background())
	command := exec.Command(os.Args[0], "-test.run=TestWindowsInstallerHelperProcess")
	command.Env = append(os.Environ(), installerHelperMode+"=parent", "WAXLIGHT_INSTALLER_SIGNAL="+signal)
	done := make(chan error, 1)
	go func() {
		_, err := runWindowsInstaller(ctx, command)
		done <- err
	}()

	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(signal); err == nil {
			break
		}
		if time.Now().After(deadline) {
			cancel()
			t.Fatal("installer child did not start")
		}
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("installer error = %v, want context cancellation", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("installer process tree did not stop")
	}
	if err := os.RemoveAll(target); err != nil {
		t.Fatalf("remove installer target after cancellation: %v", err)
	}
}

func TestWindowsInstallerHelperProcess(t *testing.T) {
	switch os.Getenv(installerHelperMode) {
	case "parent":
		time.Sleep(300 * time.Millisecond)
		child := exec.Command(os.Args[0], "-test.run=TestWindowsInstallerHelperProcess")
		child.Env = append(os.Environ(), installerHelperMode+"=child")
		if err := child.Run(); err != nil {
			t.Fatal(err)
		}
	case "child":
		file, err := os.Create(os.Getenv("WAXLIGHT_INSTALLER_SIGNAL"))
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		if _, err := file.WriteString("started"); err != nil {
			t.Fatal(err)
		}
		if err := file.Sync(); err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Hour)
	}
}
