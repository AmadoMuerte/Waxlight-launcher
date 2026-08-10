//go:build windows

package main

import (
	"context"
	"runtime"
	"syscall"
	"unsafe"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	whMouseLL     = 14
	wmXButtonDown = 0x020B
	xButton1      = 1
	xButton2      = 2
)

type lowLevelMouseInput struct {
	point     struct{ x, y int32 }
	mouseData uint32
	flags     uint32
	time      uint32
	extraInfo uintptr
}

var (
	user32                   = syscall.NewLazyDLL("user32.dll")
	setWindowsHookEx         = user32.NewProc("SetWindowsHookExW")
	callNextHookEx           = user32.NewProc("CallNextHookEx")
	getMessage               = user32.NewProc("GetMessageW")
	getForegroundWindow      = user32.NewProc("GetForegroundWindow")
	getWindowThreadProcessID = user32.NewProc("GetWindowThreadProcessId")
)

func installMouseNavigation(ctx context.Context) {
	go runMouseNavigationHook(ctx)
}

func runMouseNavigationHook(ctx context.Context) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	callback := syscall.NewCallback(func(code int, wParam, lParam uintptr) uintptr {
		if code >= 0 && wParam == wmXButtonDown && foregroundProcessIsCurrent() {
			input := (*lowLevelMouseInput)(unsafe.Pointer(lParam))
			switch input.mouseData >> 16 {
			case xButton1:
				wailsruntime.EventsEmit(ctx, "navigation:mouse", -1)
			case xButton2:
				wailsruntime.EventsEmit(ctx, "navigation:mouse", 1)
			}
		}
		result, _, _ := callNextHookEx.Call(0, uintptr(code), wParam, lParam)
		return result
	})

	hook, _, _ := setWindowsHookEx.Call(whMouseLL, callback, 0, 0)
	if hook == 0 {
		return
	}

	var message [7]uintptr
	for {
		result, _, _ := getMessage.Call(uintptr(unsafe.Pointer(&message[0])), 0, 0, 0)
		if int32(result) <= 0 {
			return
		}
	}
}

func foregroundProcessIsCurrent() bool {
	window, _, _ := getForegroundWindow.Call()
	if window == 0 {
		return false
	}
	var processID uint32
	getWindowThreadProcessID.Call(window, uintptr(unsafe.Pointer(&processID)))
	return processID == uint32(syscall.Getpid())
}
