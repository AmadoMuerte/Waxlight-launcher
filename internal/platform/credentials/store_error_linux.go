//go:build linux

package credentials

import (
	"fmt"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/accounts"
)

func mapUntypedStoreError(err error) error {
	return fmt.Errorf("%w: %w", accounts.ErrStoreUnknown, err)
}
