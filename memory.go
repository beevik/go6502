package go6502

import (
	"io/ioutil"
	"os"
)

// An address on 6502 is always 16-bit.
type Address uint16

// Memory represents the entire 16-bit address space of the system.
type Memory struct {
	data []byte
}

// Create a new 16-bit memory space.
func NewMemory() *Memory {
	return &Memory{
		data: make([]byte, 65536),
	}
}

// Load binary 'data' into memory at address 'addr'.
func (m *Memory) LoadBytes(addr Address, data []byte) {
	copy(m.data[int(addr):int(addr)+len(data)], data)
}

// Load binary data from the file at 'filename' into memory
// starting at address 'addr'.
func (m *Memory) LoadFile(addr Address, filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}

	defer file.Close()

	data, err := ioutil.ReadAll(file)
	if err != nil {
		return err
	}

	m.LoadBytes(addr, data)
	return nil
}

// Read a 16-bit address from the memory at address 'addr'.
func (m *Memory) ReadAddress(addr Address) Address {
	if (addr & 0xff00) != ((addr + 1) & 0xff00) {
		return Address(m.data[addr]) | Address(m.data[addr-255])<<8
	} else {
		return Address(m.data[addr]) | Address(m.data[addr+1])<<8
	}
}

// Read a byte from memory at address 'addr'.
func (m *Memory) ReadByte(addr Address) byte {
	return m.data[addr]
}

// Read 'length' bytes of memory starting at address 'addr'
// and return it as a byte slice.
func (m *Memory) ReadBytes(addr Address, length int) []byte {
	return m.data[addr : addr+Address(length)]
}

// Write an address 'v' to memory at the address 'addr'.
func (m *Memory) WriteAddress(addr Address, v Address) {
	m.data[addr] = byte(v & 0xff)
	m.data[addr+1] = byte(v >> 8)
}

// Write a byte 'v' to memory at the address 'addr'.
func (m *Memory) WriteByte(addr Address, v byte) {
	m.data[addr] = v
}

// Write the byte slice 'b' to memory starting at address 'addr'.
func (m *Memory) WriteBytes(addr Address, b []byte) {
	copy(m.data[addr:addr+Address(len(b))], b)
}

// Return the offset address 'addr' + 'offset'. If the offset
// crossed a page boundary, return 'pageCrossed' as true.
func offsetAddress(addr Address, offset byte) (newAddr Address, pageCrossed bool) {
	newAddr = addr + Address(offset)
	pageCrossed = ((newAddr & 0xff00) != (addr & 0xff00))
	return
}

// Offset a zero-page address 'addr' by 'offset'. If the address
// exceeds the zero-page address space, wrap it.
func offsetZeroPage(addr Address, offset byte) Address {
	addr += Address(offset)
	if addr > 0x100 {
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
