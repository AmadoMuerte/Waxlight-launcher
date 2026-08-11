package instancedirectory

import (
	"io"
	"os"

	"github.com/waxlight/waxlight-launcher/internal/infrastructure/securefs"
)

// LaunchLogs opens and hardens launcher-owned instance log files.
type LaunchLogs struct{}

// Open creates a fresh launch log with exclusive ownership and restricted
// permissions, hardened against symlink or attribute tricks.
func (LaunchLogs) Open(path string) (io.WriteCloser, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	if err := securefs.Apply(path, 0o600, false); err != nil {
		_ = file.Close()
		return nil, err
	}
	return file, nil
}

// Harden secures the logs directory and its contents.
func (LaunchLogs) Harden(directory string) error {
	return HardenLogs(directory)
}
