//go:build !linux

package deeplink

// RegisterHandler is unnecessary outside Linux.
func RegisterHandler() error {
	return nil
}
