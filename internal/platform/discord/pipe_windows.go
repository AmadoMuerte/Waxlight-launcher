//go:build windows

package discord

import (
	"net"
	"strconv"
	"time"

	"github.com/Microsoft/go-winio"
)

func pipePaths() []string {
	paths := make([]string, 10)
	for index := range paths {
		paths[index] = `\\?\pipe\discord-ipc-` + strconv.Itoa(index)
	}
	return paths
}

func dialPipe(path string, timeout time.Duration) (net.Conn, error) {
	return winio.DialPipe(path, &timeout)
}
