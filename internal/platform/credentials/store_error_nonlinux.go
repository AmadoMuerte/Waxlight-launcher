//go:build !linux

package credentials

import (
	"fmt"
	"strings"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/accounts"
)

func mapUntypedStoreError(err error) error {
	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "locked"), strings.Contains(message, "islocked"):
		return fmt.Errorf("%w: native credential store is locked", accounts.ErrStoreLocked)
	case strings.Contains(message, "denied"), strings.Contains(message, "permission"), strings.Contains(message, "access is denied"):
		return fmt.Errorf("%w: native credential store denied access", accounts.ErrPermissionDenied)
	default:
		return fmt.Errorf("%w: native credential store operation failed", accounts.ErrStoreUnavailable)
	}
}
