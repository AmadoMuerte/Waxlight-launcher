package bootstrap

import (
	"errors"
	"fmt"
	"testing"

	"github.com/waxlight/waxlight-launcher/internal/accounts"
)

func TestCredentialStoreUnavailable(t *testing.T) {
	for name, tc := range map[string]struct {
		err  error
		want bool
	}{
		"locked":      {fmt.Errorf("wrapped: %w", accounts.ErrStoreLocked), true},
		"unavailable": {fmt.Errorf("wrapped: %w", accounts.ErrStoreUnavailable), true},
		"denied":      {fmt.Errorf("wrapped: %w", accounts.ErrPermissionDenied), true},
		"corrupt":     {accounts.ErrCorruptCredentials, false},
		"other":       {errors.New("database failure"), false},
	} {
		t.Run(name, func(t *testing.T) {
			if got := credentialStoreUnavailable(tc.err); got != tc.want {
				t.Fatalf("credentialStoreUnavailable(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
