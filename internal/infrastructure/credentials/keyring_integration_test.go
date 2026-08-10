//go:build integration && (linux || windows)

package credentials

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/waxlight/waxlight-launcher/internal/application"
)

func TestNativeCredentialStoreRoundTrip(t *testing.T) {
	store := NewStore(t.TempDir())
	id := "integration-" + uuid.NewString()
	secret := application.Secret{SessionKey: "integration-session-key", SessionSignature: "integration-signature"}
	if err := store.Set(context.Background(), id, secret); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Delete(context.Background(), id) })
	updated := application.Secret{SessionKey: "updated-session-key", SessionSignature: "updated-signature"}
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
	if _, err := store.Get(context.Background(), id); !errors.Is(err, application.ErrSecretNotFound) {
		t.Fatalf("unexpected missing error: %v", err)
	}
}
