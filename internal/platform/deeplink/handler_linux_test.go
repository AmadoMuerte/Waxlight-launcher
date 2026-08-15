//go:build linux

package deeplink

import (
	"strings"
	"testing"
)

func TestDesktopEntryUsesExecutableAndProtocol(t *testing.T) {
	entry := desktopEntry("/home/test/Waxlight Launcher/waxlight")
	for _, want := range []string{
		`Exec="/home/test/Waxlight Launcher/waxlight" %u`,
		"MimeType=x-scheme-handler/waxlight;",
	} {
		if !strings.Contains(entry, want) {
			t.Fatalf("desktop entry missing %q: %s", want, entry)
		}
	}
}
