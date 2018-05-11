// Copyright 2014-2018 Brett Vickers. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cpu

import "errors"

// Errors
var (
	ErrMemoryOutOfBounds = errors.New("Memory access out of bounds")
)

// The Memory interface presents an interface to the CPU through which all
// memory accesses occur.
type Memory interface {
	// LoadByte loads a single byte from the address and returns it.
	LoadByte(addr uint16) byte

	// LoadBytes loads multiple bytes from the address and stores them into
	// the buffer 'b'.
	LoadBytes(addr uint16, b []byte)

	// LoadAddress loads a 16-bit address value from the requested address and
	// returns it.
	LoadAddress(addr uint16) uint16

	// StoreByte stores a byte to the requested address.
	StoreByte(addr uint16, v byte)

	// StoreBytes stores multiple bytes to the requested address.
	StoreBytes(addr uint16, b []byte)

	// StoreAddres stores a 16-bit address 'v' to the requested address.
	StoreAddress(addr uint16, v uint16)
}

// FlatMemory represents an entire 16-bit address space as a singular
// 64K buffer.
type FlatMemory struct {
	b [64 * 1024]byte
}

// NewFlatMemory creates a new 16-bit memory space.
func NewFlatMemory() *FlatMemory {
	return &FlatMemory{}
}

// LoadByte loads a single byte from the address and returns it.
func (m *FlatMemory) LoadByte(addr uint16) byte {
	return m.b[addr]
}

// LoadBytes loads multiple bytes from the address and returns them.
func (m *FlatMemory) LoadBytes(addr uint16, b []byte) {
	if int(addr)+len(b) <= len(m.b) {
		copy(b, m.b[addr:])
	} else {
		r0 := len(m.b) - int(addr)
		r1 := len(b) - r0
		copy(b, m.b[addr:])
		copy(b[r0:], make([]byte, r1))
	}
}

// LoadAddress loads a 16-bit address value from the requested address and
// returns it.
//
// When the address spans 2 pages (i.e., address ends in 0xff), the low
// byte of the loaded address comes from a page-wrapped address.  For example,
// LoadAddress on $12FF reads the low byte from $12FF and the high byte from
// $1200. This mimics the behavior of the NMOS 6502.
func (m *FlatMemory) LoadAddress(addr uint16) uint16 {
	if (addr & 0xff) == 0xff {
		return uint16(m.b[addr]) | uint16(m.b[addr-0xff])<<8
	}
	return uint16(m.b[addr]) | uint16(m.b[addr+1])<<8
}

// StoreByte stores a byte at the requested address.
func (m *FlatMemory) StoreByte(addr uint16, v byte) {
	m.b[addr] = v
}

// StoreBytes stores multiple bytes to the requested address.
func (m *FlatMemory) StoreBytes(addr uint16, b []byte) {
	copy(m.b[addr:], b)
}

// StoreAddress stores a 16-bit address value to the requested address.
func (m *FlatMemory) StoreAddress(addr uint16, v uint16) {
	m.b[addr] = byte(v & 0xff)
	if (addr & 0xff) == 0xff {
		m.b[addr-0xff] = byte(v >> 8)
	} else {
		m.b[addr+1] = byte(v >> 8)
	}
}

// Return the offset address 'addr' + 'offset'. If the offset
// crossed a page boundary, return 'pageCrossed' as true.
func offsetAddress(addr uint16, offset byte) (newAddr uint16, pageCrossed bool) {
	newAddr = addr + uint16(offset)
	pageCrossed = ((newAddr & 0xff00) != (addr & 0xff00))
	return newAddr, pageCrossed
}

// Offset a zero-page address 'addr' by 'offset'. If the address
// exceeds the zero-page address space, wrap it.
func offsetZeroPage(addr uint16, offset byte) uint16 {
	addr += uint16(offset)
	if addr >= 0x100 {
		addr -= 0x100
	}
	return addr
}

// Convert a 1- or 2-byte operand into an address.
func operandToAddress(operand []byte) uint16 {
	switch {
	case len(operand) == 1:
		return uint16(operand[0])
	case len(operand) == 2:
		return uint16(operand[0]) | uint16(operand[1])<<8
	}
	return 0
}

// Given a 1-byte stack pointer register, return the stack
// corresponding memory address.
func stackAddress(offset byte) uint16 {
	return uint16(0x100) + uint16(offset)
}
