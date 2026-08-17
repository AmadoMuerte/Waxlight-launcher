//go:build linux

package deeplink

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopEntryUsesExecutableAndProtocol(t *testing.T) {
	entry := desktopEntry("/home/test/Waxlight Launcher/waxlight")
	for _, want := range []string{
		`Exec="/home/test/Waxlight Launcher/waxlight" %u`,
		"NoDisplay=true",
		"MimeType=x-scheme-handler/waxlight;",
		"X-Waxlight-Managed=true",
	} {
		if !strings.Contains(entry, want) {
			t.Fatalf("desktop entry missing %q: %s", want, entry)
		}
	}
}

func TestWriteManagedDesktopFileCreatesHandler(t *testing.T) {
	path := filepath.Join(t.TempDir(), desktopFileName)
	want := desktopEntry("/opt/waxlight/waxlight")

	if err := writeManagedDesktopFile(path, want); err != nil {
		t.Fatalf("writeManagedDesktopFile() error = %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(got) != want {
		t.Fatalf("handler contents differ\nwant:\n%s\ngot:\n%s", want, got)
	}
}

func TestWriteManagedDesktopFileRefreshesManagedHandler(t *testing.T) {
	path := filepath.Join(t.TempDir(), desktopFileName)
	old := desktopEntry("/old/location/waxlight")
	want := desktopEntry("/new/location/waxlight")

	if err := os.WriteFile(path, []byte(old), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := writeManagedDesktopFile(path, want); err != nil {
		t.Fatalf("writeManagedDesktopFile() error = %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(got) != want {
		t.Fatalf("managed handler was not refreshed\nwant:\n%s\ngot:\n%s", want, got)
	}
}

func TestWriteManagedDesktopFilePreservesUserEntry(t *testing.T) {
	path := filepath.Join(t.TempDir(), desktopFileName)
	userEntry := `[Desktop Entry]
Type=Application
Name=Custom Waxlight Handler
Exec=env WEBKIT_DISABLE_DMABUF_RENDERER=1 /custom/waxlight %u
NoDisplay=true
MimeType=x-scheme-handler/waxlight;
`

	if err := os.WriteFile(path, []byte(userEntry), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := writeManagedDesktopFile(path, desktopEntry("/new/location/waxlight")); err != nil {
		t.Fatalf("writeManagedDesktopFile() error = %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(got) != userEntry {
		t.Fatalf("user-managed handler was overwritten\nwant:\n%s\ngot:\n%s", userEntry, got)
	}
}
