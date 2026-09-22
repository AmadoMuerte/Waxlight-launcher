//go:build linux

package credentials

import (
	"errors"
	"testing"

	dbus "github.com/godbus/dbus/v5"
	keyring "github.com/zalando/go-keyring"
)

func TestLinuxBackendUsesDefaultCollectionAlias(t *testing.T) {
	const want = "/org/freedesktop/secrets/aliases/default"
	if got := string(defaultCollectionAlias); got != want {
		t.Fatalf("unexpected Secret Service collection: %q", got)
	}
}

func TestWrapLinuxStoreErrorClassifiesExactDBusNamesByOperationAndTarget(t *testing.T) {
	tests := []struct {
		name      string
		operation nativeStoreErrorOperation
		target    nativeStoreErrorTarget
		source    error
		want      nativeStoreErrorKind
		wantNil   bool
	}{
		{name: "nil", operation: nativeStoreErrorOperationService, target: nativeStoreErrorTargetService, wantNil: true},
		{name: "locked", operation: nativeStoreErrorOperationUnlock, target: nativeStoreErrorTargetCollection, source: dbus.NewError("org.freedesktop.Secret.Error.IsLocked", []any{"localized"}), want: nativeStoreErrorLocked},
		{name: "access denied", operation: nativeStoreErrorOperationService, target: nativeStoreErrorTargetService, source: dbus.NewError("org.freedesktop.DBus.Error.AccessDenied", []any{"localized"}), want: nativeStoreErrorPermission},
		{name: "authentication failed", operation: nativeStoreErrorOperationService, target: nativeStoreErrorTargetService, source: dbus.NewError("org.freedesktop.DBus.Error.AuthFailed", []any{"localized"}), want: nativeStoreErrorPermission},
		{name: "no session", operation: nativeStoreErrorOperationService, target: nativeStoreErrorTargetService, source: dbus.NewError("org.freedesktop.Secret.Error.NoSession", []any{"localized"}), want: nativeStoreErrorDesktopSession},
		{name: "no reply", operation: nativeStoreErrorOperationService, target: nativeStoreErrorTargetService, source: dbus.NewError("org.freedesktop.DBus.Error.NoReply", []any{"localized"}), want: nativeStoreErrorDesktopSession},
		{name: "disconnected", operation: nativeStoreErrorOperationService, target: nativeStoreErrorTargetService, source: dbus.NewError("org.freedesktop.DBus.Error.Disconnected", []any{"localized"}), want: nativeStoreErrorDesktopSession},
		{name: "service unavailable", operation: nativeStoreErrorOperationService, target: nativeStoreErrorTargetService, source: dbus.NewError("org.freedesktop.DBus.Error.ServiceUnknown", []any{"localized"}), want: nativeStoreErrorUnavailable},
		{name: "service has no owner", operation: nativeStoreErrorOperationService, target: nativeStoreErrorTargetService, source: dbus.NewError("org.freedesktop.DBus.Error.NameHasNoOwner", []any{"localized"}), want: nativeStoreErrorUnavailable},
		{name: "collection missing", operation: nativeStoreErrorOperationSearch, target: nativeStoreErrorTargetCollection, source: dbus.NewError("org.freedesktop.Secret.Error.NoSuchObject", []any{"localized"}), want: nativeStoreErrorUnavailable},
		{name: "item unlock missing", operation: nativeStoreErrorOperationUnlock, target: nativeStoreErrorTargetItem, source: dbus.NewError("org.freedesktop.Secret.Error.NoSuchObject", []any{"localized"}), want: nativeStoreErrorMissing},
		{name: "collection create missing", operation: nativeStoreErrorOperationCreate, target: nativeStoreErrorTargetCollection, source: dbus.NewError("org.freedesktop.Secret.Error.NoSuchObject", []any{"localized"}), want: nativeStoreErrorUnavailable},
		{name: "keyring item missing", operation: nativeStoreErrorOperationSearch, target: nativeStoreErrorTargetItem, source: keyring.ErrNotFound, want: nativeStoreErrorMissing},
		{name: "unknown D-Bus name", operation: nativeStoreErrorOperationRead, target: nativeStoreErrorTargetItem, source: dbus.NewError("org.example.Unknown", []any{"collection is locked"}), want: nativeStoreErrorUnknown},
		{name: "unknown local error", operation: nativeStoreErrorOperationService, target: nativeStoreErrorTargetService, source: errors.New("access denied"), want: nativeStoreErrorUnknown},
		{name: "generic item unlock wrapper failure", operation: nativeStoreErrorOperationUnlock, target: nativeStoreErrorTargetItem, source: errors.New("unlock failed"), want: nativeStoreErrorUnlock},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			// When
			got := wrapLinuxStoreError(test.operation, test.target, test.source)

			// Then
			if test.wantNil {
				if got != nil {
					t.Fatalf("wrapped nil error = %v, want nil", got)
				}
				return
			}
			var nativeErr *nativeStoreError
			if !errors.As(got, &nativeErr) {
				t.Fatalf("wrapped error lost native identity: %v", got)
			}
			if nativeErr.kind != test.want {
				t.Fatalf("native error kind = %v, want %v", nativeErr.kind, test.want)
			}
			if nativeErr.operation != test.operation {
				t.Fatalf("native error operation = %v, want %v", nativeErr.operation, test.operation)
			}
			if nativeErr.target != test.target {
				t.Fatalf("native error target = %v, want %v", nativeErr.target, test.target)
			}
			if !errors.Is(got, test.source) {
				t.Fatalf("wrapped error lost cause: %v", got)
			}
		})
	}
}
