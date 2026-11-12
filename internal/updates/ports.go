package updates

import "context"

// Source discovers launcher releases. The HTTP implementation lives in
// internal/platform/updater.
type Source interface {
	Check(context.Context, string, string) (Update, error)
}

// Installer applies a verified update package. The platform implementations
// live in internal/platform/updater.
type Installer interface {
	Apply(ctx context.Context, installerPath string, currentPID int) error
}

// SignatureVerifier verifies the publisher signature of a downloaded update
// package. The platform implementations live in internal/platform/updater.
type SignatureVerifier interface {
	Verify(ctx context.Context, executablePath string) error
}

// MutationGate coordinates launcher writes with data-root relocation.
type MutationGate interface {
	Begin() error
	End()
}

// maximumLauncherUpdateSize caps how large a verified launcher update package
// may be.
const maximumLauncherUpdateSize = 512 * 1024 * 1024
