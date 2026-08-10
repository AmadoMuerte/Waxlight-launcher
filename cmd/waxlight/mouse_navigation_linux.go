//go:build linux

package main

/*
#cgo pkg-config: gtk+-3.0 webkit2gtk-4.1
#include "mouse_navigation_linux.h"
*/
import "C"

import (
	"context"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

var mouseNavigation struct {
	sync.RWMutex
	context context.Context
}

func installMouseNavigation(ctx context.Context) {
	mouseNavigation.Lock()
	mouseNavigation.context = ctx
	mouseNavigation.Unlock()
	C.installMouseNavigationHandler()
}

//export emitMouseNavigation
func emitMouseNavigation(direction C.int) {
	mouseNavigation.RLock()
	ctx := mouseNavigation.context
	mouseNavigation.RUnlock()
	if ctx != nil {
		runtime.EventsEmit(ctx, "navigation:mouse", int(direction))
	}
}
