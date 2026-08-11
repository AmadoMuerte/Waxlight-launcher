package errs

import (
	"errors"
	"strings"
	"testing"
)

func TestPublicErrorStringRedactsInternalCause(t *testing.T) {
	sentinel := "WAXLIGHT_TEST_SESSION_KEY_DO_NOT_LEAK"
	err := &AppError{Code: ErrSecretStorage, Message: "Could not save the account session", Cause: errors.New(sentinel)}
	if strings.Contains(err.Error(), sentinel) {
		t.Fatalf("internal cause leaked through public error: %v", err)
	}
	if !errors.Is(err, err.Cause) {
		t.Fatal("internal cause is unavailable to trusted backend code")
	}
}
