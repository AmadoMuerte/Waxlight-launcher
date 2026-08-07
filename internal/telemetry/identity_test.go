package telemetry

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestIdentityCreatedWhenMissing(t *testing.T) {
	store := newFakeStore(t, nil)
	store.values = map[string]string{}

	id := newIdentity(store)
	installationID := id.ID(context.Background())
	if installationID == "" {
		t.Fatal("identity was not created")
	}
	if _, err := uuid.Parse(installationID); err != nil {
		t.Fatalf("installation ID %q is not a valid UUID: %v", installationID, err)
	}
	if store.values[installationIDKey] != installationID {
		t.Fatal("installation ID was not persisted")
	}
}

func TestIdentityReusedWhenPresent(t *testing.T) {
	store := newFakeStore(t, nil)
	store.values = map[string]string{
		installationIDKey: "550e8400-e29b-41d4-a716-446655440000",
	}

	id := newIdentity(store)
	if installationID := id.ID(context.Background()); installationID != "550e8400-e29b-41d4-a716-446655440000" {
		t.Fatalf("existing installation ID was not reused: %q", installationID)
	}
}

func TestIdentityStableAcrossDisableEnable(t *testing.T) {
	store := newFakeStore(t, map[string]string{})
	store.values = map[string]string{}

	id := newIdentity(store)
	first := id.ID(context.Background())
	if second := id.ID(context.Background()); second != first {
		t.Fatalf("installation ID changed between reads: %q -> %q", first, second)
	}

	// Toggling telemetry on and off touches the settings record only; the
	// installation ID lives in its own settings key and must survive.
	store.values = map[string]string{
		installationIDKey: first,
	}
	store.setEnabled(false)
	if afterDisable := id.ID(context.Background()); afterDisable != first {
		t.Fatalf("installation ID regenerated after disabling telemetry: %q", afterDisable)
	}
	store.setEnabled(true)
	if afterEnable := id.ID(context.Background()); afterEnable != first {
		t.Fatalf("installation ID regenerated after re-enabling telemetry: %q", afterEnable)
	}
}

func TestIdentityReplacesMalformedValue(t *testing.T) {
	store := newFakeStore(t, nil)
	store.values = map[string]string{installationIDKey: "not-a-uuid"}

	id := newIdentity(store)
	installationID := id.ID(context.Background())
	if _, err := uuid.Parse(installationID); err != nil {
		t.Fatalf("malformed stored value was not replaced with a valid UUID: %v", err)
	}
}

func TestIdentityHasNoMachineFingerprint(t *testing.T) {
	store := newFakeStore(t, nil)
	store.values = map[string]string{}

	id := newIdentity(store)
	installationID := id.ID(context.Background())
	lower := strings.ToLower(installationID)
	for _, forbidden := range []string{"hostname", "machine", "mac", "cpu", "disk", "user"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("installation ID appears derived from %q: %q", forbidden, installationID)
		}
	}
}
