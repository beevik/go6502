// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package term provides support functions for dealing with terminals, as
// commonly found on UNIX systems.
//
// Putting a terminal into raw mode is the most common requirement:
//
//	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
//	if err != nil {
//	        panic(err)
//	}
//	defer term.Restore(int(os.Stdin.Fd()), oldState)
//
// Note that on non-Unix systems os.Stdin.Fd() may not be 0.
package term

// State contains the state of a terminal.
type State struct {
	state
}

// IsTerminal returns whether the given file descriptor is a terminal.
func IsTerminal(fd int) bool {
	return isTerminal(fd)
}

// MakeRawInput puts the terminal connected to the given file descriptor into
// raw input mode and returns the previous state of the terminal so that it
// can be restored.
func MakeRawInput(fd int) (*State, error) {
	return makeRawInput(fd)
}

// MakeRawOutput puts the terminal connected to the given file descriptor into
// raw output mode and returns the previous state of the terminal so that it
// can be restored.
func MakeRawOutput(fd int) (*State, error) {
	return makeRawOutput(fd)
}

// GetState returns the current state of a terminal which may be useful to
// restore the terminal after a signal.
func GetState(fd int) (*State, error) {
	return getState(fd)
}

// PeekKey scans the input buffer for the presence of a "key-down" event
// for the specified key character. Currently supported only on Windows.
func PeekKey(fd int, key rune) bool {
	return peekKey(fd, key)
}

// Restore restores the terminal connected to the given file descriptor to a
// previous state.
func Restore(fd int, oldState *State) error {
	return restore(fd, oldState)
}

// GetSize returns the visible dimensions of the given terminal.
//
// These dimensions don't include any scrollback buffer height.
func GetSize(fd int) (width, height int, err error) {
	return getSize(fd)
}
