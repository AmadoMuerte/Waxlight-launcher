package mods

import (
	"context"
	"errors"
	"os"
	"testing"
)

func TestFriendlyInstallError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"missing file", os.ErrNotExist, "The downloaded mod file is missing. Download it again and retry."},
		{"cancelled", context.Canceled, "Mod installation was cancelled"},
		{"same file name", errors.New("mod file already exists"), "A different mod file with this name already exists in the instance"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := friendlyInstallError(test.err); got != test.want {
				t.Fatalf("friendlyInstallError() = %q, want %q", got, test.want)
			}
		})
	}
}
