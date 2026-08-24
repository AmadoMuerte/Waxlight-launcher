// Package mutations coordinates writes that must pause during data relocation.
package mutations

import (
	"sync"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/errs"
)

type Gate struct {
	mu         sync.Mutex
	relocating bool
	active     int
}

// Begin atomically acquires a mutation slot unless relocation has started.
func (gate *Gate) Begin() error {
	gate.mu.Lock()
	defer gate.mu.Unlock()
	if gate.relocating {
		return busyError()
	}
	gate.active++
	return nil
}

func (gate *Gate) End() {
	gate.mu.Lock()
	if gate.active > 0 {
		gate.active--
	}
	gate.mu.Unlock()
}

// BeginRelocation atomically excludes both existing and future mutations.
func (gate *Gate) BeginRelocation() error {
	gate.mu.Lock()
	defer gate.mu.Unlock()
	if gate.relocating || gate.active > 0 {
		return busyError()
	}
	gate.relocating = true
	return nil
}

func (gate *Gate) EndRelocation() {
	gate.mu.Lock()
	gate.relocating = false
	gate.mu.Unlock()
}

func (gate *Gate) Busy() bool {
	gate.mu.Lock()
	defer gate.mu.Unlock()
	return gate.relocating
}

func busyError() error {
	return errs.NewError(errs.ErrDataFolderBusy, "The data folder is being moved; wait for the relocation to finish")
}
