//go:build linux && desktop

package mousenavigation

/*
#cgo pkg-config: gtk+-3.0 webkit2gtk-4.1
#include "mouse_navigation_linux.h"
*/
import "C"

import "sync"

var navigationHandler struct {
	sync.RWMutex
	callback func(int)
}

func Install(callback func(int)) {
	navigationHandler.Lock()
	navigationHandler.callback = callback
	navigationHandler.Unlock()
	C.installMouseNavigationHandler()
}

//export emitMouseNavigation
func emitMouseNavigation(direction C.int) {
	navigationHandler.RLock()
	callback := navigationHandler.callback
	navigationHandler.RUnlock()
	if callback != nil {
		callback(int(direction))
	}
}
