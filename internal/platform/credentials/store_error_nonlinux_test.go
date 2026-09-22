//go:build !linux

package credentials

import (
	"errors"
	"fmt"
	"testing"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/accounts"
	keyring "github.com/zalando/go-keyring"
)

func TestMapStoreErrorKeepsNonLinuxMessagesProviderSafe(t *testing.T) {
	tests := []struct {
		name   string
		source error
		want   error
		text   string
	}{
		{name: "missing", source: fmt.Errorf("provider detail: %w", keyring.ErrNotFound), want: accounts.ErrCredentialsNotFound, text: "account secret not found"},
		{name: "unsupported", source: fmt.Errorf("provider detail: %w", keyring.ErrUnsupportedPlatform), want: accounts.ErrStoreUnavailable, text: "credential store unavailable"},
		{name: "locked", source: errors.New("provider locked detail"), want: accounts.ErrStoreLocked, text: "credential store locked: native credential store is locked"},
		{name: "permission", source: errors.New("provider denied detail"), want: accounts.ErrPermissionDenied, text: "credential store permission denied: native credential store denied access"},
		{name: "unknown", source: errors.New("provider detail"), want: accounts.ErrStoreUnavailable, text: "credential store unavailable: native credential store operation failed"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			got := mapStoreError(test.source)

			// Then
			if !errors.Is(got, test.want) {
				t.Fatalf("mapStoreError() = %v, want errors.Is(..., %v)", got, test.want)
			}
			if got.Error() != test.text {
				t.Fatalf("mapStoreError() = %q, want %q", got, test.text)
			}
		})
	}
}
