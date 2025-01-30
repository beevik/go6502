// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package term

import (
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

type state struct {
	mode uint32
}

type keyRecord struct {
	EventType       uint32
	KeyDown         uint32
	RepeatCount     uint16
	VirtualKeyCode  uint16
	VirtualScanCode uint16
	UnicodeChar     uint16
	ControlKeyState uint32
}

var (
	dll                  *windows.DLL
	procPeekConsoleInput *windows.Proc
	peekBuf              []keyRecord
)

func isTerminal(fd int) bool {
	var st uint32
	err := windows.GetConsoleMode(windows.Handle(fd), &st)
	return err == nil
}

func makeRawInput(fd int) (*State, error) {
	var mode uint32
	if err := windows.GetConsoleMode(windows.Handle(fd), &mode); err != nil {
		return nil, err
	}

	var enable uint32 = windows.ENABLE_VIRTUAL_TERMINAL_INPUT
	var disable uint32 = windows.ENABLE_PROCESSED_INPUT |
		windows.ENABLE_LINE_INPUT |
		windows.ENABLE_ECHO_INPUT |
		windows.ENABLE_MOUSE_INPUT |
		windows.ENABLE_WINDOW_INPUT
	newMode := (mode & ^disable) | enable

	if err := windows.SetConsoleMode(windows.Handle(fd), newMode); err != nil {
		return nil, err
	}

	return &State{state{mode}}, nil
}

func makeRawOutput(fd int) (*State, error) {
	var mode uint32
	if err := windows.GetConsoleMode(windows.Handle(fd), &mode); err != nil {
		return nil, err
	}

	var enable uint32 = windows.ENABLE_PROCESSED_OUTPUT |
		windows.ENABLE_WRAP_AT_EOL_OUTPUT |
		windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING
	var disable uint32 = windows.DISABLE_NEWLINE_AUTO_RETURN
	newMode := (mode & ^disable) | enable

	if err := windows.SetConsoleMode(windows.Handle(fd), newMode); err != nil {
		return nil, err
	}
	return &State{state{mode}}, nil
}

func peekKey(fd int, key rune) bool {
	// Lazy-allocate the peek buffer.
	if peekBuf == nil {
		// Lazy-load the kernel32 DLL.
		if dll == nil {
			dll = windows.MustLoadDLL("kernel32.dll")
		}
		procPeekConsoleInput = dll.MustFindProc("PeekConsoleInputW")
		peekBuf = make([]keyRecord, 16)
	}

	// Peek at the console input buffer.
	var n int
	for {
		_, _, err := syscall.SyscallN(procPeekConsoleInput.Addr(),
			uintptr(windows.Handle(fd)),
			uintptr(unsafe.Pointer(&peekBuf[0])),
			uintptr(len(peekBuf)),
			uintptr(unsafe.Pointer(&n)))

		if err != 0 {
			return false
		}
		if n < len(peekBuf) {
			break
		}

		// The record buffer wasn't large enough to hold all events, so grow
		// it and try again.
		peekBuf = make([]keyRecord, len(peekBuf)*2)
	}

	// Search the record buffer for a key-down event containing the requested
	// key.
	for _, r := range peekBuf[:n] {
		if r.EventType == uint32(1) && r.KeyDown == uint32(1) && rune(r.UnicodeChar) == key {
			return true
		}
	}

	return false
}

func getState(fd int) (*State, error) {
	var st uint32
	if err := windows.GetConsoleMode(windows.Handle(fd), &st); err != nil {
		return nil, err
	}
	return &State{state{st}}, nil
}

func restore(fd int, state *State) error {
	return windows.SetConsoleMode(windows.Handle(fd), state.mode)
}

func getSize(fd int) (width, height int, err error) {
	var info windows.ConsoleScreenBufferInfo
	if err := windows.GetConsoleScreenBufferInfo(windows.Handle(fd), &info); err != nil {
		return 0, 0, err
	}
	return int(info.Window.Right - info.Window.Left + 1), int(info.Window.Bottom - info.Window.Top + 1), nil
}
