package updater

import "context"

// SignatureVerifier performs the platform-specific signature policy after the
// downloaded package has already passed the mandatory SHA-256 verification.
//
// Windows builds require a configured trusted publisher and a valid
// Authenticode signature. Other platforms use the platform-appropriate
// checksum-only policy.
type SignatureVerifier interface {
	Verify(ctx context.Context, executablePath string) error
}
