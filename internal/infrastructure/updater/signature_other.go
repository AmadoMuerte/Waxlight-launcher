//go:build !windows

package updater

import "context"

// ChecksumOnlySignatureVerifier is used on platforms where Authenticode does
// not apply. LauncherUpdateService has already required and verified SHA-256
// before this verifier is called.
type ChecksumOnlySignatureVerifier struct{}

func NewSignatureVerifier(_ []string) *ChecksumOnlySignatureVerifier {
	return &ChecksumOnlySignatureVerifier{}
}

func (*ChecksumOnlySignatureVerifier) Verify(_ context.Context, _ string) error {
	return nil
}
