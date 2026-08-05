package updater

import "context"

// SignatureVerifier performs the platform-specific signature policy after the
// downloaded package has already passed the mandatory SHA-256 verification.
//
// Windows builds enforce Authenticode only when at least one trusted publisher
// subject or certificate thumbprint is configured. Unsigned development builds
// therefore remain updateable in checksum-only mode, while signed production
// builds fail closed on an invalid or unexpected publisher.
type SignatureVerifier interface {
	Verify(ctx context.Context, executablePath string) error
}
