//go:build !linux && !windows

package optimum

import optimumfeature "github.com/waxlight/waxlight-launcher/internal/optimum"

type Locator struct{}

func NewLocator() Locator { return Locator{} }

func (Locator) Detect() (optimumfeature.Installation, error) {
	return optimumfeature.Installation{}, optimumfeature.ErrUnsupported
}

func (Locator) Inspect(string) (optimumfeature.Installation, error) {
	return optimumfeature.Installation{}, optimumfeature.ErrUnsupported
}

func (Locator) GameVersion(string) string { return "" }

func (Locator) InUse(optimumfeature.Installation) (bool, error) { return false, nil }
