package go6502

import (
	"errors"
	"io/ioutil"
	"os"
)

// An Address on 6502 is always 16-bit.
type Address uint16

// Memory errors
var (
	ErrMemoryOutOfBounds = errors.New("Memory out of bounds")
	ErrMemoryReadOnly    = errors.New("Memory is read-only")
)

// A MemoryBank represents a region of memory. This interface is used for
// every type of memory including RAM, ROM, IO and peripheral buffers.
type MemoryBank interface {
	AddressRange() (start Address, end Address)
	LoadByte(addr Address) byte
	LoadAddress(addr Address) Address
	StoreByte(addr Address, v byte)
}

// The access bit mask is used to indicate memory access: read and/or write.
type access int

const (
	read access = 1 << iota
	write
)

// A page is a 256-byte chunk of memory.
type page struct {
	read  MemoryBank // memory bank used for this page's reads
	write MemoryBank // memory bank used for this page's writes
}

// SystemMemory represents the current configuration of system memory. It
// may consist of multiple memory banks, each with different address
// ranges and access patterns (e.g, RAM, ROM, video memory).
type SystemMemory struct {
	banks map[MemoryBank]access
	pages [256]page
}

// NewSystemMemory creates a new system memory object.
func NewSystemMemory() *SystemMemory {
	return &SystemMemory{
		banks: make(map[MemoryBank]access),
	}
}

// AddBank adds a memory bank to the set of all known memory banks. The
// bank starts inactive for reads and writes.
func (m *SystemMemory) AddBank(b MemoryBank) {
	m.banks[b] = 0
}

// RemoveBank removes a memory bank from the set of all known memory banks.
// If it was active for reads or writes, it is deactivated first.
func (m *SystemMemory) RemoveBank(b MemoryBank) {
	active, ok := m.banks[b]
	if !ok {
		return
	}

	if active != 0 {
		m.DeactivateBank(b, active)
	}
	delete(m.banks, b)
}

// ActivateBank activates a memory bank so that it handles all accesses
// to its addresses. Read and write access may be configured independently.
func (m *SystemMemory) ActivateBank(b MemoryBank, access access) {
	active, ok := m.banks[b]
	if !ok {
		return
	}

	enableReads := (access&read) != 0 && (active&read) == 0
	enableWrites := (access&write) != 0 && (active&write) == 0
	if !enableReads && !enableWrites {
		return
	}

	m.banks[b] = m.banks[b] | access

	start, end := b.AddressRange()
	for i, j := start>>8, end>>8; i < j; i++ {
		if enableReads {
			m.pages[i].read = b
		}
		if enableWrites {
			m.pages[i].write = b
		}
	}
}

// DeactivateBank deactivates a memory bank so that it no longer handles
// accesses to its addresses. Read and write access may be configured
// independently.
func (m *SystemMemory) DeactivateBank(b MemoryBank, access access) {
	active, ok := m.banks[b]
	if !ok {
		return
	}

	disableReads := (access&read) != 0 && (active&read) != 0
	disableWrites := (access&write) != 0 && (active&write) != 0
	if !disableReads && !disableWrites {
		return
	}

	m.banks[b] = m.banks[b] &^ access

	start, end := b.AddressRange()
	for i, j := start>>8, end>>8; i < j; i++ {
		if disableReads {
			m.pages[i].read = nil
		}
		if disableWrites {
			m.pages[i].write = nil
		}
	}
}

// LoadByte loads a byte from the requested address and returns it.
func (m *SystemMemory) LoadByte(addr Address) (byte, error) {
	b := m.pages[addr>>8].read
	if b == nil {
		return 0, ErrMemoryOutOfBounds
	}
	return b.LoadByte(addr), nil
}

// LoadAddress loads a 16-bit address from the requested address and
// returns it.
func (m *SystemMemory) LoadAddress(addr Address) (Address, error) {
	b := m.pages[addr>>8].read
	if b == nil {
		return 0, ErrMemoryOutOfBounds
	}
	return b.LoadAddress(addr), nil
}

// StoreByte stores a byte to the requested address.
func (m *SystemMemory) StoreByte(addr Address, v byte) error {
	b := m.pages[addr>>8].write
	if b == nil {
		return ErrMemoryOutOfBounds
	}
	b.StoreByte(addr, v)
	return nil
}

// RAM represents a random-access memory bank that can be read and written.
type RAM struct {
	start Address
	end   Address
	buf   []byte
}

// NewRAM creates a new RAM memory bank of the requested size. Its
// contents are initialized to zeroes.
func NewRAM(addr Address, size int) *RAM {
	if int(addr)+size > 0x10000 {
		panic("RAM address exceeds 64K")
	}
	if size&0xff != 0 {
		panic("RAM size must be a multiple of the 256-byte page size")
	}
	return &RAM{
		start: addr,
		end:   addr + Address(size),
		buf:   make([]byte, size),
	}
}

// AddressRange returns the range of addresses in the RAM bank.
func (r *RAM) AddressRange() (start Address, end Address) {
	return r.start, r.end
}

// LoadByte returns the value of a byte of memory at the requested address.
func (r *RAM) LoadByte(addr Address) byte {
	return r.buf[addr-r.start]
}

// LoadAddress loads a 16-bit address from the requested memory address.
func (r *RAM) LoadAddress(addr Address) Address {
	i := int(addr - r.start)
	if (i & 0xff) == 0xff {
		return Address(r.buf[i]) | Address(r.buf[i-0xff])<<8
	}
	return Address(r.buf[i]) | Address(r.buf[i+1])<<8
}

// StoreByte stores a byte value at the requested address.
func (r *RAM) StoreByte(addr Address, b byte) {
	r.buf[addr] = b
}

// ROM represents a bank of read-only memory.
type ROM struct {
	start Address
	end   Address
	buf   []byte
}

// NewROM creates a new ROM memory bank initialized with the contents of the
// provided buffer.
func NewROM(addr Address, b []byte) *ROM {
	if int(addr)+len(b) > 0x10000 {
		panic("ROM address space exceeds 64K")
	}
	if len(b)&0xff != 0 {
		panic("ROM size must be a multiple of the 256-byte page size")
	}
	rom := &ROM{
		start: addr,
		end:   addr + Address(len(b)),
		buf:   make([]byte, len(b)),
	}
	copy(rom.buf, b)
	return rom
}

// AddressRange returns the range of addresses in the ROM bank.
func (r *ROM) AddressRange() (start Address, end Address) {
	return r.start, r.end
}

// LoadByte returns the value of a byte of memory at the requested address.
func (r *ROM) LoadByte(addr Address) byte {
	return r.buf[addr-r.start]
}

// LoadAddress loads a 16-bit address from the requested memory address.
func (r *ROM) LoadAddress(addr Address) Address {
	i := int(addr - r.start)
	if (i & 0xff) == 0xff {
		return Address(r.buf[i]) | Address(r.buf[i-0xff])<<8
	}
	return Address(r.buf[i]) | Address(r.buf[i+1])<<8
}

// StoreByte does nothing for ROM.
func (r *ROM) StoreByte(addr Address, b byte) {
}

// Memory represents the entire 16-bit address space of the system.
type Memory struct {
	data []byte
}

// NewMemory creates a new 16-bit memory space.
func NewMemory() *Memory {
	return &Memory{
		data: make([]byte, 65536),
	}
}

// CopyBytes copies binary 'data' into memory at address 'addr'.
func (m *Memory) CopyBytes(addr Address, data []byte) error {
	if int(addr)+len(data) > len(m.data) {
		return errors.New("memory address space exceeded")
	}

	copy(m.data[int(addr):], data)
	return nil
}

// LoadFile loads binary data from the file at 'filename' into memory
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

	return m.CopyBytes(addr, data)
}

// LoadAddress reads a 16-bit address from the memory at address 'addr'.
func (m *Memory) LoadAddress(addr Address) Address {
	if (addr & 0xff) == 0xff {
		return Address(m.data[addr]) | Address(m.data[addr-0xff])<<8
	}
	return Address(m.data[addr]) | Address(m.data[addr+1])<<8
}

// LoadByte reads a byte from memory at address 'addr'.
func (m *Memory) LoadByte(addr Address) byte {
	return m.data[addr]
}

// LoadBytes reads 'length' bytes of memory starting at address 'addr'
// and return it as a byte slice.
func (m *Memory) LoadBytes(addr Address, length int) []byte {
	return m.data[addr : addr+Address(length)]
}

// StoreAddress stores an address 'v' to memory at the address 'addr'.
func (m *Memory) StoreAddress(addr Address, v Address) {
	m.data[addr] = byte(v & 0xff)
	m.data[addr+1] = byte(v >> 8)
}

// StoreByte stores a byte 'v' to memory at the address 'addr'.
func (m *Memory) StoreByte(addr Address, v byte) {
	m.data[addr] = v
}

// StoreBytes stores the byte slice 'b' to memory starting at address 'addr'.
func (m *Memory) StoreBytes(addr Address, b []byte) {
	copy(m.data[addr:addr+Address(len(b))], b)
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
