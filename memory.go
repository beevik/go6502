package go6502

import "errors"

// An Address on 6502 is always 16-bit.
type Address uint16

// Errors
var (
	ErrMemoryOutOfBounds = errors.New("Memory access out of bounds")
)

// The Memory interface presents an interface to the CPU through which all
// memory accesses occur.
type Memory interface {
	// LoadByte loads a single byte from the address and returns it.
	LoadByte(addr Address) (byte, error)

	// LoadBytes loads multiple bytes from the address and stores them into
	// the buffer 'b'.
	LoadBytes(addr Address, b []byte) error

	// LoadAddress loads a 16-bit address value from the requested address and
	// returns it.
	LoadAddress(addr Address) (Address, error)

	// StoreByte stores a byte to the requested address.
	StoreByte(addr Address, v byte) error

	// StoreBytes stores multiple bytes to the requested address.
	StoreBytes(addr Address, b []byte) error

	// StoreAddres stores a 16-bit address 'v' to the requested address.
	StoreAddress(addr Address, v Address) error
}

// FlatMemory represents an entire 16-bit address space as a singular
// 64K buffer.
type FlatMemory struct {
	b []byte
}

// NewFlatMemory creates a new 16-bit memory space.
func NewFlatMemory() *FlatMemory {
	return &FlatMemory{
		b: make([]byte, 65536),
	}
}

// LoadByte loads a single byte from the address and returns it.
func (m *FlatMemory) LoadByte(addr Address) (byte, error) {
	if int(addr) >= len(m.b) {
		return 0, ErrMemoryOutOfBounds
	}
	return m.b[addr], nil
}

// LoadBytes loads multiple bytes from the address and returns them.
func (m *FlatMemory) LoadBytes(addr Address, b []byte) error {
	if int(addr)+len(b) > len(m.b) {
		return ErrMemoryOutOfBounds
	}
	copy(b, m.b[addr:])
	return nil
}

// LoadAddress loads a 16-bit address value from the requested address and
// returns it.
//
// When the address spans 2 pages (i.e., address ends in 0xff), the low
// byte of the loaded address comes from a page-wrapped address.  For example,
// LoadAddress on $12FF reads the low byte from $12FF and the high byte from
// $1200. This mimics the behavior of the NMOS 6502.
func (m *FlatMemory) LoadAddress(addr Address) (Address, error) {
	if int(addr)+2 > len(m.b) {
		return 0, ErrMemoryOutOfBounds
	}

	if (addr & 0xff) == 0xff {
		return Address(m.b[addr]) | Address(m.b[addr-0xff])<<8, nil
	}
	return Address(m.b[addr]) | Address(m.b[addr+1])<<8, nil
}

// StoreByte stores a byte at the requested address.
func (m *FlatMemory) StoreByte(addr Address, v byte) error {
	if int(addr) >= len(m.b) {
		return ErrMemoryOutOfBounds
	}

	m.b[addr] = v
	return nil
}

// StoreBytes stores multiple bytes to the requested address.
func (m *FlatMemory) StoreBytes(addr Address, b []byte) error {
	if int(addr)+len(b) > len(m.b) {
		return ErrMemoryOutOfBounds
	}

	copy(m.b[int(addr):], b)
	return nil
}

// StoreAddress stores a 16-bit address value to the requested address.
func (m *FlatMemory) StoreAddress(addr Address, v Address) error {
	if int(addr)+2 > len(m.b) {
		return ErrMemoryOutOfBounds
	}
	m.b[addr] = byte(v & 0xff)
	m.b[addr+1] = byte(v >> 8)
	return nil
}

// Return the offset address 'addr' + 'offset'. If the offset
// crossed a page boundary, return 'pageCrossed' as true.
func offsetAddress(addr Address, offset byte) (newAddr Address, pageCrossed bool) {
	newAddr = addr + Address(offset)
	pageCrossed = ((newAddr & 0xff00) != (addr & 0xff00))
	return newAddr, pageCrossed
}

// Offset a zero-page address 'addr' by 'offset'. If the address
// exceeds the zero-page address space, wrap it.
func offsetZeroPage(addr Address, offset byte) Address {
	addr += Address(offset)
	if addr >= 0x100 {
		addr -= 0x100
	}
	return addr
}

// Convert a 1- or 2-byte operand into an address.
func operandToAddress(operand []byte) Address {
	switch {
	case len(operand) == 1:
		return Address(operand[0])
	case len(operand) == 2:
		return Address(operand[0]) | Address(operand[1])<<8
	}
	return 0
}

// Given a 1-byte stack pointer register, return the stack
// corresponding memory address.
func stackAddress(offset byte) Address {
	return Address(0x100) + Address(offset)
}
