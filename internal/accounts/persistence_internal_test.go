package accounts

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/errs"
)

func TestCredentialStoreErrorUsesSafeCategoryMessages(t *testing.T) {
	const diagnosticMarker = "WAXLIGHT_SYNTHETIC_CREDENTIAL_DIAGNOSTIC"

	tests := []struct {
		name      string
		category  error
		available bool
		message   string
		retryable bool
	}{
		{name: "unavailable", category: ErrStoreUnavailable, message: "No system credential store is available. Make sure a Secret Service provider is installed and running in this desktop session.", retryable: true},
		{name: "locked", category: ErrStoreLocked, message: "The system credential store is locked. Unlock the desktop keyring and try again.", retryable: true},
		{name: "unlock failed", category: ErrStoreUnlockFailed, available: true, message: "The system credential store is locked and could not be unlocked. On Linux desktop environments such as COSMIC, this can happen when the session login manager does not automatically unlock the system keyring.", retryable: true},
		{name: "permission", category: ErrPermissionDenied, message: "Waxlight does not have permission to access the system credential store.", retryable: false},
		{name: "desktop session", category: ErrStoreDesktopSession, available: true, message: "Waxlight could not connect to the desktop session D-Bus. Start Waxlight from an active desktop session and try again.", retryable: true},
		{name: "unknown", category: ErrStoreUnknown, message: "An unknown system credential-store error occurred. Check that the desktop keyring service is working and try again.", retryable: false},
		{name: "corrupt", category: ErrCorruptCredentials, message: "The saved account session is corrupt and must be replaced", retryable: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cause := errors.New(diagnosticMarker)
			storeErr := fmt.Errorf("%w: %w", test.category, cause)
			if test.available {
				storeErr = fmt.Errorf("%w: %w: %w", test.category, ErrStoreUnavailable, cause)
			}

			err := credentialStoreError("ignored safe fallback", storeErr)

			var appErr *errs.AppError
			if !errors.As(err, &appErr) {
				t.Fatalf("credentialStoreError() = %v, want AppError", err)
			}
			if appErr.Code != errs.ErrSecretStorage || appErr.Message != test.message || appErr.Retryable != test.retryable {
				t.Fatalf("credentialStoreError() = %#v", appErr)
			}
			if !errors.Is(err, test.category) || !errors.Is(err, cause) {
				t.Fatalf("credentialStoreError() lost cause identity: %v", err)
			}
			if strings.Contains(appErr.Error(), diagnosticMarker) {
				t.Fatalf("public error leaked diagnostic marker: %v", appErr)
			}
		})
	}
}
