//go:build !windows

package updater

import (
	"context"
	"fmt"
)

type NopSignatureVerifier struct{}

func NewSignatureVerifier(_ []string) *NopSignatureVerifier {
	return &NopSignatureVerifier{}
}

func (*NopSignatureVerifier) Verify(_ context.Context, _ string) error {
	return fmt.Errorf("signature verification is only supported on Windows")
}
