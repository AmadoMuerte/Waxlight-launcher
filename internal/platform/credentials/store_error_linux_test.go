//go:build linux

package credentials

import (
	"errors"
	"testing"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/accounts"
)

func TestMapStoreErrorMapsLinuxUnknownInput(t *testing.T) {
	// Given
	source := errors.New("unclassified native error")

	// When
	got := mapStoreError(source)

	// Then
	if !errors.Is(got, accounts.ErrStoreUnknown) {
		t.Fatalf("mapStoreError() = %v, want errors.Is(..., %v)", got, accounts.ErrStoreUnknown)
	}
}
