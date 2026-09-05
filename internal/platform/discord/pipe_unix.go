//go:build !windows

package discord

import (
	"net"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

func pipePaths() []string {
	bases := make([]string, 0, 7)
	if runtimeDir := os.Getenv("XDG_RUNTIME_DIR"); runtimeDir != "" {
		bases = append(bases,
			runtimeDir,
			filepath.Join(runtimeDir, "app", "com.discordapp.Discord"),
			filepath.Join(runtimeDir, "snap.discord"),
		)
	}
	for _, name := range []string{"TMPDIR", "TMP", "TEMP"} {
		if tempDir := os.Getenv(name); tempDir != "" {
			bases = append(bases, tempDir)
		}
	}
	bases = append(bases, "/tmp")

	paths := make([]string, 0, len(bases)*10)
	for index := range 10 {
		for _, base := range bases {
			paths = append(paths, filepath.Join(base, "discord-ipc-"+strconv.Itoa(index)))
		}
	}
	return paths
}

func dialPipe(path string, timeout time.Duration) (net.Conn, error) {
	return net.DialTimeout("unix", path, timeout)
}
