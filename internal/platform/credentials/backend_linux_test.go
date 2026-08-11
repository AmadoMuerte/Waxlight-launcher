//go:build linux

package credentials

import "testing"

func TestLinuxBackendUsesDefaultCollectionAlias(t *testing.T) {
	const want = "/org/freedesktop/secrets/aliases/default"
	if got := string(defaultCollectionAlias); got != want {
		t.Fatalf("unexpected Secret Service collection: %q", got)
	}
}
