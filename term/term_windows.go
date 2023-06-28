// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package term

import (
	"golang.org/x/sys/windows"
)

type state struct {
	mode uint32
}

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
