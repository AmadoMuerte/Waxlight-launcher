package bootstrap

import (
	"errors"
	"fmt"
	"testing"

	"github.com/waxlight/waxlight-launcher/internal/application"
)

func TestCredentialStoreUnavailable(t *testing.T) {
	for name, tc := range map[string]struct {
		err  error
		want bool
	}{
		"locked":      {fmt.Errorf("wrapped: %w", application.ErrStoreLocked), true},
		"unavailable": {fmt.Errorf("wrapped: %w", application.ErrStoreUnavailable), true},
		"denied":      {fmt.Errorf("wrapped: %w", application.ErrPermissionDenied), true},
		"corrupt":     {application.ErrCorruptSecret, false},
		"other":       {errors.New("database failure"), false},
	} {
		t.Run(name, func(t *testing.T) {
			if got := credentialStoreUnavailable(tc.err); got != tc.want {
				t.Fatalf("credentialStoreUnavailable(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
