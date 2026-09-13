package credentials

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/accounts"
	keyring "github.com/zalando/go-keyring"
)

func TestMapStoreError(t *testing.T) {
	typedCause := errors.New("native cause")
	tests := []struct {
		name           string
		source         error
		want           error
		wantNativeKind nativeStoreErrorKind
		wantTypedCause bool
		wantRetryable  bool
		wantNil        bool
	}{
		{name: "nil input", wantNil: true},
		{name: "unavailable native error", source: fmt.Errorf("wrapped: %w", &nativeStoreError{kind: nativeStoreErrorUnavailable, cause: typedCause}), want: accounts.ErrStoreUnavailable, wantNativeKind: nativeStoreErrorUnavailable, wantTypedCause: true},
		{name: "locked native error", source: fmt.Errorf("wrapped: %w", &nativeStoreError{kind: nativeStoreErrorLocked, cause: typedCause}), want: accounts.ErrStoreLocked, wantNativeKind: nativeStoreErrorLocked, wantTypedCause: true},
		{name: "unlock native error", source: fmt.Errorf("wrapped: %w", &nativeStoreError{kind: nativeStoreErrorUnlock, cause: typedCause}), want: accounts.ErrStoreUnlockFailed, wantNativeKind: nativeStoreErrorUnlock, wantTypedCause: true, wantRetryable: true},
		{name: "missing native error", source: fmt.Errorf("wrapped: %w", &nativeStoreError{kind: nativeStoreErrorMissing, cause: typedCause}), want: accounts.ErrCredentialsNotFound, wantNativeKind: nativeStoreErrorMissing, wantTypedCause: true},
		{name: "permission native error", source: fmt.Errorf("wrapped: %w", &nativeStoreError{kind: nativeStoreErrorPermission, cause: typedCause}), want: accounts.ErrPermissionDenied, wantNativeKind: nativeStoreErrorPermission, wantTypedCause: true},
		{name: "desktop session native error", source: fmt.Errorf("wrapped: %w", &nativeStoreError{kind: nativeStoreErrorDesktopSession, cause: typedCause}), want: accounts.ErrStoreDesktopSession, wantNativeKind: nativeStoreErrorDesktopSession, wantTypedCause: true, wantRetryable: true},
		{name: "unknown native error without cause", source: fmt.Errorf("wrapped: %w", &nativeStoreError{kind: nativeStoreErrorUnknown}), want: accounts.ErrStoreUnknown, wantNativeKind: nativeStoreErrorUnknown},
		{name: "not found", source: fmt.Errorf("wrapped: %w", keyring.ErrNotFound), want: accounts.ErrCredentialsNotFound},
		{name: "unsupported platform", source: fmt.Errorf("wrapped: %w", keyring.ErrUnsupportedPlatform), want: accounts.ErrStoreUnavailable},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			got := mapStoreError(test.source)

			// Then
			if test.wantNil {
				if got != nil {
					t.Fatalf("mapStoreError() = %v, want nil", got)
				}
				return
			}
			if !errors.Is(got, test.want) {
				t.Fatalf("mapStoreError() = %v, want errors.Is(..., %v)", got, test.want)
			}
			if test.wantTypedCause && !errors.Is(got, typedCause) {
				t.Fatalf("mapStoreError() = %v, typed cause was lost", got)
			}
			if test.wantRetryable && !errors.Is(got, accounts.ErrStoreUnavailable) {
				t.Fatalf("mapStoreError() = %v, retryable store identity was lost", got)
			}
			if test.wantNativeKind == 0 {
				return
			}
			var nativeErr *nativeStoreError
			if !errors.As(got, &nativeErr) {
				t.Fatalf("mapStoreError() = %v, native error identity was lost", got)
			}
			if nativeErr.kind != test.wantNativeKind {
				t.Fatalf("native error kind = %v, want %v", nativeErr.kind, test.wantNativeKind)
			}
		})
	}
}

func TestStoreMapsContextCancellation(t *testing.T) {
	// Given
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	store := newStoreWithBackend(&memoryBackend{values: map[string]string{}})

	// When
	_, err := store.Get(cancelled, "account")

	// Then
	if !errors.Is(err, accounts.ErrStoreUnavailable) {
		t.Fatalf("unexpected cancellation error: %v", err)
	}
	if errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation semantics changed: %v", err)
	}
}
