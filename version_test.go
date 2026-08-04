package waxlight

import "testing"

func TestVersionMatchesEmbeddedApplicationConfig(t *testing.T) {
	if Version() == "" || Version() == "0.0.0" {
		t.Fatalf("embedded application version was not loaded: %q", Version())
	}
}
