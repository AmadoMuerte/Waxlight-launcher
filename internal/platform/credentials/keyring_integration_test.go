//go:build integration && (linux || windows)

package credentials

import (
	"context"
	"errors"
	"testing"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/accounts"
	"github.com/google/uuid"
)

func TestNativeCredentialStoreRoundTrip(t *testing.T) {
	store := NewStore(t.TempDir())
	id := "integration-" + uuid.NewString()
	secret := accounts.Credential{SessionKey: "integration-session-key", SessionSignature: "integration-signature"}
	if err := store.Set(context.Background(), id, secret); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Delete(context.Background(), id) })
	updated := accounts.Credential{SessionKey: "updated-session-key", SessionSignature: "updated-signature"}
	if err := store.Set(context.Background(), id, updated); err != nil {
		t.Fatal(err)
	}
	got, err := store.Get(context.Background(), id)
	if err != nil || got != updated {
		t.Fatalf("unexpected secret: %#v, %v", got, err)
	}
	if err := store.Delete(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(context.Background(), id); !errors.Is(err, accounts.ErrCredentialsNotFound) {
		t.Fatalf("unexpected missing error: %v", err)
	}
}
