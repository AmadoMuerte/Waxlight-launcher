package updater

import "context"

// SignatureVerifier verifies Authenticode digital signatures on Windows executables.
type SignatureVerifier interface {
	// Verify checks that the file at executablePath has a valid Authenticode signature
	// from a trusted publisher. Returns nil if the signature is valid, or an error
	// describing why the verification failed.
	Verify(ctx context.Context, executablePath string) error
}
