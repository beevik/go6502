// Copyright 2014-2018 Brett Vickers. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package cpu implements a 6502 CPU instruction
// set and emulator.
package cpu

// Architecture selects the CPU chip: 6502 or 65c02
type Architecture byte

const (
	// NMOS 6502 CPU
	NMOS Architecture = iota

	// CMOS 65c02 CPU
	CMOS
)

// BrkHandler is an interface implemented by types that wish to be notified
// when a BRK instruction is about to be executed.
type BrkHandler interface {
	OnBrk(cpu *CPU)
}

// CPU represents a single 6502 CPU. It contains a pointer to the
// memory associated with the CPU.
type CPU struct {
	Arch        Architecture    // CPU architecture
	Reg         Registers       // CPU registers
	Mem         Memory          // assigned memory
	Cycles      uint64          // total executed CPU cycles
	LastPC      uint16          // Previous program counter
	InstSet     *InstructionSet // Instruction set used by the CPU
	pageCrossed bool
	deltaCycles int8
	debugger    *Debugger
	brkHandler  BrkHandler
	storeByte   func(cpu *CPU, addr uint16, v byte)
}

// Interrupt vectors
const (
	vectorNMI   = 0xfffa
	vectorReset = 0xfffc
	vectorIRQ   = 0xfffe
	vectorBRK   = 0xfffe
)

// NewCPU creates an emulated 6502 CPU bound to the specified memory.
func NewCPU(arch Architecture, m Memory) *CPU {
	cpu := &CPU{
		Arch:      arch,
		Mem:       m,
		InstSet:   GetInstructionSet(arch),
		storeByte: (*CPU).storeByteNormal,
	}

	cpu.Reg.Init()
	return cpu
}

// SetPC updates the CPU program counter to 'addr'.
func (cpu *CPU) SetPC(addr uint16) {
	cpu.Reg.PC = addr
}

// GetInstruction returns the instruction opcode at the requested address.
func (cpu *CPU) GetInstruction(addr uint16) *Instruction {
	opcode := cpu.Mem.LoadByte(addr)
	return cpu.InstSet.Lookup(opcode)
}

// NextAddr returns the address of the next instruction following the
// instruction at addr.
func (cpu *CPU) NextAddr(addr uint16) uint16 {
	opcode := cpu.Mem.LoadByte(addr)
	inst := cpu.InstSet.Lookup(opcode)
	return addr + uint16(inst.Length)
}

// Step the cpu by one instruction.
func (cpu *CPU) Step() {
	// Grab the next opcode at the current PC
	opcode := cpu.Mem.LoadByte(cpu.Reg.PC)

	// Look up the instruction data for the opcode
	inst := cpu.InstSet.Lookup(opcode)

	// If the instruction is undefined, reset the CPU (for now).
	if inst.fn == nil {
		cpu.reset()
		return
	}

	// If a BRK instruction is about to be executed and a BRK handler has been
	// installed, call the BRK handler instead of executing the instruction.
	if inst.Opcode == 0x00 && cpu.brkHandler != nil {
		cpu.brkHandler.OnBrk(cpu)
		return
	}

	// Fetch the operand (if any) and advance the PC
	var buf [2]byte
	operand := buf[:inst.Length-1]
	cpu.Mem.LoadBytes(cpu.Reg.PC+1, operand)
	cpu.LastPC = cpu.Reg.PC
	cpu.Reg.PC += uint16(inst.Length)

	// Execute the instruction
	cpu.pageCrossed = false
	cpu.deltaCycles = 0
	inst.fn(cpu, inst, operand)

	// Update the CPU cycle counter, with special-case logic
	// to handle a page boundary crossing
	cpu.Cycles += uint64(int8(inst.Cycles) + cpu.deltaCycles)
	if cpu.pageCrossed {
		cpu.Cycles += uint64(inst.BPCycles)
	}

	// Update the debugger so it handle breakpoints.
	if cpu.debugger != nil {
		cpu.debugger.onUpdatePC(cpu, cpu.Reg.PC)
	}
}

// AttachBrkHandler attaches a handler that is called whenever the BRK
// instruction is executed.
func (cpu *CPU) AttachBrkHandler(handler BrkHandler) {
	cpu.brkHandler = handler
}

// AttachDebugger attaches a debugger to the CPU. The debugger receives
// notifications whenever the CPU executes an instruction or stores a byte
// to memory.
func (cpu *CPU) AttachDebugger(debugger *Debugger) {
	cpu.debugger = debugger
	cpu.storeByte = (*CPU).storeByteDebugger
}

// DetachDebugger detaches the currently debugger from the CPU.
func (cpu *CPU) DetachDebugger() {
	cpu.debugger = nil
	cpu.storeByte = (*CPU).storeByteNormal
}

// Load a byte value from using the requested addressing mode
// and the operand to determine where to load it from.
func (cpu *CPU) load(mode Mode, operand []byte) byte {
	switch mode {
	case IMM:
		return operand[0]
	case ZPG:
		zpaddr := operandToAddress(operand)
		return cpu.Mem.LoadByte(zpaddr)
	case ZPX:
		zpaddr := operandToAddress(operand)
		zpaddr = offsetZeroPage(zpaddr, cpu.Reg.X)
		return cpu.Mem.LoadByte(zpaddr)
	case ZPY:
		zpaddr := operandToAddress(operand)
		zpaddr = offsetZeroPage(zpaddr, cpu.Reg.Y)
		return cpu.Mem.LoadByte(zpaddr)
	case ABS:
		addr := operandToAddress(operand)
		return cpu.Mem.LoadByte(addr)
	case ABX:
		addr := operandToAddress(operand)
		addr, cpu.pageCrossed = offsetAddress(addr, cpu.Reg.X)
		return cpu.Mem.LoadByte(addr)
	case ABY:
		addr := operandToAddress(operand)
		addr, cpu.pageCrossed = offsetAddress(addr, cpu.Reg.Y)
		return cpu.Mem.LoadByte(addr)
	case IDX:
		zpaddr := operandToAddress(operand)
		zpaddr = offsetZeroPage(zpaddr, cpu.Reg.X)
		addr := cpu.Mem.LoadAddress(zpaddr)
		return cpu.Mem.LoadByte(addr)
	case IDY:
		zpaddr := operandToAddress(operand)
		addr := cpu.Mem.LoadAddress(zpaddr)
		addr, cpu.pageCrossed = offsetAddress(addr, cpu.Reg.Y)
		return cpu.Mem.LoadByte(addr)
	case ACC:
		return cpu.Reg.A
	default:
		panic("Invalid addressing mode")
	}
}

// Load a 16-bit address value from memory using the requested addressing mode
// and the 16-bit instruction operand.
func (cpu *CPU) loadAddress(mode Mode, operand []byte) uint16 {
	switch mode {
	case ABS:
		return operandToAddress(operand)
	case IND:
		addr := operandToAddress(operand)
		return cpu.Mem.LoadAddress(addr)
	default:
		panic("Invalid addressing mode")
	}
}

// Store a byte value using the specified addressing mode and the
// variable-sized instruction operand to determine where to store it.
func (cpu *CPU) store(mode Mode, operand []byte, v byte) {
	switch mode {
	case ZPG:
		zpaddr := operandToAddress(operand)
		cpu.storeByte(cpu, zpaddr, v)
	case ZPX:
		zpaddr := operandToAddress(operand)
		zpaddr = offsetZeroPage(zpaddr, cpu.Reg.X)
		cpu.storeByte(cpu, zpaddr, v)
	case ZPY:
		zpaddr := operandToAddress(operand)
		zpaddr = offsetZeroPage(zpaddr, cpu.Reg.Y)
		cpu.storeByte(cpu, zpaddr, v)
	case ABS:
		addr := operandToAddress(operand)
		cpu.storeByte(cpu, addr, v)
	case ABX:
		addr := operandToAddress(operand)
		addr, cpu.pageCrossed = offsetAddress(addr, cpu.Reg.X)
		cpu.storeByte(cpu, addr, v)
	case ABY:
		addr := operandToAddress(operand)
		addr, cpu.pageCrossed = offsetAddress(addr, cpu.Reg.Y)
		cpu.storeByte(cpu, addr, v)
	case IDX:
		zpaddr := operandToAddress(operand)
		zpaddr = offsetZeroPage(zpaddr, cpu.Reg.X)
		addr := cpu.Mem.LoadAddress(zpaddr)
		cpu.storeByte(cpu, addr, v)
	case IDY:
		zpaddr := operandToAddress(operand)
		addr := cpu.Mem.LoadAddress(zpaddr)
		addr, cpu.pageCrossed = offsetAddress(addr, cpu.Reg.Y)
		cpu.storeByte(cpu, addr, v)
	case ACC:
		cpu.Reg.A = v
	default:
		panic("Invalid addressing mode")
	}
}

// Execute a branch using the instruction operand.
func (cpu *CPU) branch(operand []byte) {
	offset := operandToAddress(operand)
	oldPC := cpu.Reg.PC
	if offset < 0x80 {
		cpu.Reg.PC += uint16(offset)
	} else {
		cpu.Reg.PC -= uint16(0x100 - offset)
	}
	cpu.deltaCycles++
	if ((cpu.Reg.PC ^ oldPC) & 0xff00) != 0 {
		cpu.deltaCycles++
	}
}

// Store the byte value 'v' add the address 'addr'.
func (cpu *CPU) storeByteNormal(addr uint16, v byte) {
	cpu.Mem.StoreByte(addr, v)
}

// Store the byte value 'v' add the address 'addr'.
func (cpu *CPU) storeByteDebugger(addr uint16, v byte) {
	cpu.debugger.onDataStore(cpu, addr, v)
	cpu.Mem.StoreByte(addr, v)
}

// Push a value 'v' onto the stack.
func (cpu *CPU) push(v byte) {
	cpu.storeByte(cpu, stackAddress(cpu.Reg.SP), v)
	cpu.Reg.SP--
}

// Push the address 'addr' onto the stack.
func (cpu *CPU) pushAddress(addr uint16) {
	cpu.push(byte(addr >> 8))
	cpu.push(byte(addr))
}

// Pop a value from the stack and return it.
func (cpu *CPU) pop() byte {
	cpu.Reg.SP++
	return cpu.Mem.LoadByte(stackAddress(cpu.Reg.SP))
}

// Pop a 16-bit address off the stack.
func (cpu *CPU) popAddress() uint16 {
	lo := cpu.pop()
	hi := cpu.pop()
	return uint16(lo) | (uint16(hi) << 8)
}

// Update the Zero and Negative flags based on the value of 'v'.
func (cpu *CPU) updateNZ(v byte) {
	cpu.Reg.Zero = (v == 0)
	cpu.Reg.Sign = ((v & 0x80) != 0)
}

// Handle a handleInterrupt by storing the program counter and status flags on
// the stack. Then switch the program counter to the requested address.
func (cpu *CPU) handleInterrupt(brk bool, addr uint16) {
	cpu.pushAddress(cpu.Reg.PC)
	cpu.push(cpu.Reg.SavePS(brk))

	cpu.Reg.InterruptDisable = true
	if cpu.Arch == CMOS {
		cpu.Reg.Decimal = false
	}

	cpu.Reg.PC = cpu.Mem.LoadAddress(addr)
}

// Generate a maskable IRQ (hardware) interrupt request (unused)
func (cpu *CPU) irq() {
	if !cpu.Reg.InterruptDisable {
		cpu.handleInterrupt(false, vectorIRQ)
	}
}

// Generate a non-maskable interrupt (unused)
func (cpu *CPU) nmi() {
	cpu.handleInterrupt(false, vectorNMI)
}

// Generate a reset signal.
func (cpu *CPU) reset() {
	cpu.Reg.PC = cpu.Mem.LoadAddress(vectorReset)
}

// Add with carry (CMOS)
func (cpu *CPU) adcc(inst *Instruction, operand []byte) {
	acc := uint32(cpu.Reg.A)
	add := uint32(cpu.load(inst.Mode, operand))
	carry := boolToUint32(cpu.Reg.Carry)
	var v uint32

	cpu.Reg.Overflow = (((acc ^ add) & 0x80) == 0)

	switch cpu.Reg.Decimal {
	case true:
		cpu.deltaCycles++

		lo := (acc & 0x0f) + (add & 0x0f) + carry

		var carrylo uint32
		if lo >= 0x0a {
			carrylo = 0x10
			lo -= 0xa
		}

		hi := (acc & 0xf0) + (add & 0xf0) + carrylo

		if hi >= 0xa0 {
			cpu.Reg.Carry = true
			if hi >= 0x180 {
				cpu.Reg.Overflow = false
			}
			hi -= 0xa0
		} else {
			cpu.Reg.Carry = false
			if hi < 0x80 {
				cpu.Reg.Overflow = false
			}
		}

		v = hi | lo

	case false:
		v = acc + add + carry
		if v >= 0x100 {
			cpu.Reg.Carry = true
			if v >= 0x180 {
				cpu.Reg.Overflow = false
			}
		} else {
			cpu.Reg.Carry = false
			if v < 0x80 {
				cpu.Reg.Overflow = false
			}
		}
	}

	cpu.Reg.A = byte(v)
	cpu.updateNZ(cpu.Reg.A)
}

// Add with carry (NMOS)
func (cpu *CPU) adcn(inst *Instruction, operand []byte) {
	acc := uint32(cpu.Reg.A)
	add := uint32(cpu.load(inst.Mode, operand))
	carry := boolToUint32(cpu.Reg.Carry)
	var v uint32

	switch cpu.Reg.Decimal {
	case true:
		lo := (acc & 0x0f) + (add & 0x0f) + carry

		var carrylo uint32
		if lo >= 0x0a {
			carrylo = 0x10
			lo -= 0x0a
		}

		hi := (acc & 0xf0) + (add & 0xf0) + carrylo

		if hi >= 0xa0 {
			cpu.Reg.Carry = true
			hi -= 0xa0
		} else {
			cpu.Reg.Carry = false
		}

		v = hi | lo

		cpu.Reg.Overflow = ((acc^v)&0x80) != 0 && ((acc^add)&0x80) == 0

	case false:
		v = acc + add + carry
		cpu.Reg.Carry = (v >= 0x100)
		cpu.Reg.Overflow = (((acc & 0x80) == (add & 0x80)) && ((acc & 0x80) != (v & 0x80)))
	}

	cpu.Reg.A = byte(v)
	cpu.updateNZ(cpu.Reg.A)
}

// Boolean AND
func (cpu *CPU) and(inst *Instruction, operand []byte) {
	cpu.Reg.A &= cpu.load(inst.Mode, operand)
	cpu.updateNZ(cpu.Reg.A)
}

// Arithmetic Shift Left
func (cpu *CPU) asl(inst *Instruction, operand []byte) {
	v := cpu.load(inst.Mode, operand)
	cpu.Reg.Carry = ((v & 0x80) == 0x80)
	v = v << 1
	cpu.updateNZ(v)
	cpu.store(inst.Mode, operand, v)
	if cpu.Arch == CMOS && inst.Mode == ABX && !cpu.pageCrossed {
		cpu.deltaCycles--
	}
}

// Branch if Carry Clear
func (cpu *CPU) bcc(inst *Instruction, operand []byte) {
	if !cpu.Reg.Carry {
		cpu.branch(operand)
	}
}

// Branch if Carry Set
func (cpu *CPU) bcs(inst *Instruction, operand []byte) {
	if cpu.Reg.Carry {
		cpu.branch(operand)
	}
}

// Branch if EQual (to zero)
func (cpu *CPU) beq(inst *Instruction, operand []byte) {
	if cpu.Reg.Zero {
		cpu.branch(operand)
	}
}

// Bit Test
func (cpu *CPU) bit(inst *Instruction, operand []byte) {
	v := cpu.load(inst.Mode, operand)
	cpu.Reg.Zero = ((v & cpu.Reg.A) == 0)
	cpu.Reg.Sign = ((v & 0x80) != 0)
	cpu.Reg.Overflow = ((v & 0x40) != 0)
}

// Branch if MInus (negative)
func (cpu *CPU) bmi(inst *Instruction, operand []byte) {
	if cpu.Reg.Sign {
		cpu.branch(operand)
	}
}

// Branch if Not Equal (not zero)
func (cpu *CPU) bne(inst *Instruction, operand []byte) {
	if !cpu.Reg.Zero {
		cpu.branch(operand)
	}
}

// Branch if PLus (positive)
func (cpu *CPU) bpl(inst *Instruction, operand []byte) {
	if !cpu.Reg.Sign {
		cpu.branch(operand)
	}
}

// Branch always (65c02 only)
func (cpu *CPU) bra(inst *Instruction, operand []byte) {
	cpu.branch(operand)
}

// Break
func (cpu *CPU) brk(inst *Instruction, operand []byte) {
	cpu.Reg.PC++
	cpu.handleInterrupt(true, vectorBRK)
}

// Branch if oVerflow Clear
func (cpu *CPU) bvc(inst *Instruction, operand []byte) {
	if !cpu.Reg.Overflow {
		cpu.branch(operand)
	}
}

// Branch if oVerflow Set
func (cpu *CPU) bvs(inst *Instruction, operand []byte) {
	if cpu.Reg.Overflow {
		cpu.branch(operand)
	}
}

// Clear Carry flag
func (cpu *CPU) clc(inst *Instruction, operand []byte) {
	cpu.Reg.Carry = false
}

// Clear Decimal flag
func (cpu *CPU) cld(inst *Instruction, operand []byte) {
	cpu.Reg.Decimal = false
}

// Clear InterruptDisable flag
func (cpu *CPU) cli(inst *Instruction, operand []byte) {
	cpu.Reg.InterruptDisable = false
}

// Clear oVerflow flag
func (cpu *CPU) clv(inst *Instruction, operand []byte) {
	cpu.Reg.Overflow = false
}

// Compare to accumulator
func (cpu *CPU) cmp(inst *Instruction, operand []byte) {
	v := cpu.load(inst.Mode, operand)
	cpu.Reg.Carry = (cpu.Reg.A >= v)
	cpu.updateNZ(cpu.Reg.A - v)
}

// Compare to X register
func (cpu *CPU) cpx(inst *Instruction, operand []byte) {
	v := cpu.load(inst.Mode, operand)
	cpu.Reg.Carry = (cpu.Reg.X >= v)
	cpu.updateNZ(cpu.Reg.X - v)
}

// Compare to Y register
func (cpu *CPU) cpy(inst *Instruction, operand []byte) {
	v := cpu.load(inst.Mode, operand)
	cpu.Reg.Carry = (cpu.Reg.Y >= v)
	cpu.updateNZ(cpu.Reg.Y - v)
}

// Decrement memory value
func (cpu *CPU) dec(inst *Instruction, operand []byte) {
	v := cpu.load(inst.Mode, operand) - 1
	cpu.updateNZ(v)
	cpu.store(inst.Mode, operand, v)
}

// Decrement X register
func (cpu *CPU) dex(inst *Instruction, operand []byte) {
	cpu.Reg.X--
	cpu.updateNZ(cpu.Reg.X)
}

// Decrement Y register
func (cpu *CPU) dey(inst *Instruction, operand []byte) {
	cpu.Reg.Y--
	cpu.updateNZ(cpu.Reg.Y)
}

// Boolean XOR
func (cpu *CPU) eor(inst *Instruction, operand []byte) {
	cpu.Reg.A ^= cpu.load(inst.Mode, operand)
	cpu.updateNZ(cpu.Reg.A)
}

// Increment memory value
func (cpu *CPU) inc(inst *Instruction, operand []byte) {
	v := cpu.load(inst.Mode, operand) + 1
	cpu.updateNZ(v)
	cpu.store(inst.Mode, operand, v)
}

// Increment X register
func (cpu *CPU) inx(inst *Instruction, operand []byte) {
	cpu.Reg.X++
	cpu.updateNZ(cpu.Reg.X)
}

// Increment Y register
func (cpu *CPU) iny(inst *Instruction, operand []byte) {
	cpu.Reg.Y++
	cpu.updateNZ(cpu.Reg.Y)
}

// Jump to memory address (NMOS 6502)
func (cpu *CPU) jmpn(inst *Instruction, operand []byte) {
	cpu.Reg.PC = cpu.loadAddress(inst.Mode, operand)
}

// Jump to memory address (CMOS 65c02)
func (cpu *CPU) jmpc(inst *Instruction, operand []byte) {
	if inst.Mode == IND && operand[0] == 0xff {
		// Fix bug in NMOS 6502 address loading. In NMOS 6502, a JMP ($12FF)
		// would load LSB of jmp target from $12FF and MSB from $1200.
		// In CMOS, it loads the MSB from $1300.
		addr0 := uint16(operand[1])<<8 | 0xff
		addr1 := addr0 + 1
		lo := cpu.Mem.LoadByte(addr0)
		hi := cpu.Mem.LoadByte(addr1)
		cpu.Reg.PC = uint16(lo) | uint16(hi)<<8
		cpu.deltaCycles++
		return
	}

	cpu.Reg.PC = cpu.loadAddress(inst.Mode, operand)
}

// Jump to subroutine
func (cpu *CPU) jsr(inst *Instruction, operand []byte) {
	addr := cpu.loadAddress(inst.Mode, operand)
	cpu.pushAddress(cpu.Reg.PC - 1)
	cpu.Reg.PC = addr
}

// load Accumulator
func (cpu *CPU) lda(inst *Instruction, operand []byte) {
	cpu.Reg.A = cpu.load(inst.Mode, operand)
	cpu.updateNZ(cpu.Reg.A)
}

// load the X register
func (cpu *CPU) ldx(inst *Instruction, operand []byte) {
	cpu.Reg.X = cpu.load(inst.Mode, operand)
	cpu.updateNZ(cpu.Reg.X)
}

// load the Y register
func (cpu *CPU) ldy(inst *Instruction, operand []byte) {
	cpu.Reg.Y = cpu.load(inst.Mode, operand)
	cpu.updateNZ(cpu.Reg.Y)
}

// Logical Shift Right
func (cpu *CPU) lsr(inst *Instruction, operand []byte) {
	v := cpu.load(inst.Mode, operand)
	cpu.Reg.Carry = ((v & 1) == 1)
	v = v >> 1
	cpu.updateNZ(v)
	cpu.store(inst.Mode, operand, v)
	if cpu.Arch == CMOS && inst.Mode == ABX && !cpu.pageCrossed {
		cpu.deltaCycles--
	}
}

// No-operation
func (cpu *CPU) nop(inst *Instruction, operand []byte) {
	// Do nothing
}

// Boolean OR
func (cpu *CPU) ora(inst *Instruction, operand []byte) {
	cpu.Reg.A |= cpu.load(inst.Mode, operand)
	cpu.updateNZ(cpu.Reg.A)
}

// Push Accumulator
func (cpu *CPU) pha(inst *Instruction, operand []byte) {
	cpu.push(cpu.Reg.A)
}

// Push Processor flags
func (cpu *CPU) php(inst *Instruction, operand []byte) {
	cpu.push(cpu.Reg.SavePS(true))
}

// Push X register (65c02 only)
func (cpu *CPU) phx(inst *Instruction, operand []byte) {
	cpu.push(cpu.Reg.X)
}

// Push Y register (65c02 only)
func (cpu *CPU) phy(inst *Instruction, operand []byte) {
	cpu.push(cpu.Reg.Y)
}

// Pull (pop) Accumulator
func (cpu *CPU) pla(inst *Instruction, operand []byte) {
	cpu.Reg.A = cpu.pop()
	cpu.updateNZ(cpu.Reg.A)
}

// Pull (pop) Processor flags
func (cpu *CPU) plp(inst *Instruction, operand []byte) {
	v := cpu.pop()
	cpu.Reg.RestorePS(v)
}

// Pull (pop) X register (65c02 only)
func (cpu *CPU) plx(inst *Instruction, operand []byte) {
	cpu.Reg.X = cpu.pop()
	cpu.updateNZ(cpu.Reg.X)
}

// Pull (pop) Y register (65c02 only)
func (cpu *CPU) ply(inst *Instruction, operand []byte) {
	cpu.Reg.Y = cpu.pop()
	cpu.updateNZ(cpu.Reg.Y)
}

// Rotate Left
func (cpu *CPU) rol(inst *Instruction, operand []byte) {
	tmp := cpu.load(inst.Mode, operand)
	v := (tmp << 1) | boolToByte(cpu.Reg.Carry)
	cpu.Reg.Carry = ((tmp & 0x80) != 0)
	cpu.updateNZ(v)
	cpu.store(inst.Mode, operand, v)
	if cpu.Arch == CMOS && inst.Mode == ABX && !cpu.pageCrossed {
		cpu.deltaCycles--
	}
}

// Rotate Right
func (cpu *CPU) ror(inst *Instruction, operand []byte) {
	tmp := cpu.load(inst.Mode, operand)
	v := (tmp >> 1) | (boolToByte(cpu.Reg.Carry) << 7)
	cpu.Reg.Carry = ((tmp & 1) != 0)
	cpu.updateNZ(v)
	cpu.store(inst.Mode, operand, v)
	if cpu.Arch == CMOS && inst.Mode == ABX && !cpu.pageCrossed {
		cpu.deltaCycles--
	}
}

// Return from Interrupt
func (cpu *CPU) rti(inst *Instruction, operand []byte) {
	v := cpu.pop()
	cpu.Reg.RestorePS(v)
	cpu.Reg.PC = cpu.popAddress()
}

// Return from Subroutine
func (cpu *CPU) rts(inst *Instruction, operand []byte) {
	addr := cpu.popAddress()
	cpu.Reg.PC = addr + 1
}

// Subtract with Carry (CMOS)
func (cpu *CPU) sbcc(inst *Instruction, operand []byte) {
	acc := uint32(cpu.Reg.A)
	sub := uint32(cpu.load(inst.Mode, operand))
	carry := boolToUint32(cpu.Reg.Carry)
	cpu.Reg.Overflow = ((acc ^ sub) & 0x80) != 0
	var v uint32

	switch cpu.Reg.Decimal {
	case true:
		cpu.deltaCycles++

		lo := 0x0f + (acc & 0x0f) - (sub & 0x0f) + carry

		var carrylo uint32
		if lo < 0x10 {
			lo -= 0x06
			carrylo = 0
		} else {
			lo -= 0x10
			carrylo = 0x10
		}

		hi := 0xf0 + (acc & 0xf0) - (sub & 0xf0) + carrylo

		if hi < 0x100 {
			cpu.Reg.Carry = false
			if hi < 0x80 {
				cpu.Reg.Overflow = false
			}
			hi -= 0x60
		} else {
			cpu.Reg.Carry = true
			if hi >= 0x180 {
				cpu.Reg.Overflow = false
			}
			hi -= 0x100
		}

		v = hi | lo

	case false:
		v = 0xff + acc - sub + carry
		if v < 0x100 {
			cpu.Reg.Carry = false
			if v < 0x80 {
				cpu.Reg.Overflow = false
			}
		} else {
			cpu.Reg.Carry = true
			if v >= 0x180 {
				cpu.Reg.Overflow = false
			}
		}
	}

	cpu.Reg.A = byte(v)
	cpu.updateNZ(cpu.Reg.A)
}

// Subtract with Carry (NMOS)
func (cpu *CPU) sbcn(inst *Instruction, operand []byte) {
	acc := uint32(cpu.Reg.A)
	sub := uint32(cpu.load(inst.Mode, operand))
	carry := boolToUint32(cpu.Reg.Carry)
	var v uint32

	switch cpu.Reg.Decimal {
	case true:
		lo := 0x0f + (acc & 0x0f) - (sub & 0x0f) + carry

		var carrylo uint32
		if lo < 0x10 {
			lo -= 0x06
			carrylo = 0
		} else {
			lo -= 0x10
			carrylo = 0x10
		}

		hi := 0xf0 + (acc & 0xf0) - (sub & 0xf0) + carrylo

		if hi < 0x100 {
			cpu.Reg.Carry = false
			hi -= 0x60
		} else {
			cpu.Reg.Carry = true
			hi -= 0x100
		}

		v = hi | lo

		cpu.Reg.Overflow = ((acc^v)&0x80) != 0 && ((acc^sub)&0x80) != 0

	case false:
		v = 0xff + acc - sub + carry
		cpu.Reg.Carry = (v >= 0x100)
		cpu.Reg.Overflow = (((acc & 0x80) != (sub & 0x80)) && ((acc & 0x80) != (v & 0x80)))
	}

	cpu.Reg.A = byte(v)
	cpu.updateNZ(byte(v))
}

// Set Carry flag
func (cpu *CPU) sec(inst *Instruction, operand []byte) {
	cpu.Reg.Carry = true
}

// Set Decimal flag
func (cpu *CPU) sed(inst *Instruction, operand []byte) {
	cpu.Reg.Decimal = true
}

// Set InterruptDisable flag
func (cpu *CPU) sei(inst *Instruction, operand []byte) {
	cpu.Reg.InterruptDisable = true
}

// Store Accumulator
func (cpu *CPU) sta(inst *Instruction, operand []byte) {
	cpu.store(inst.Mode, operand, cpu.Reg.A)
}

// Store X register
func (cpu *CPU) stx(inst *Instruction, operand []byte) {
	cpu.store(inst.Mode, operand, cpu.Reg.X)
}

// Store Y register
func (cpu *CPU) sty(inst *Instruction, operand []byte) {
	cpu.store(inst.Mode, operand, cpu.Reg.Y)
}

// Store Zero (65c02 only)
func (cpu *CPU) stz(inst *Instruction, operand []byte) {
	cpu.store(inst.Mode, operand, 0)
}

// Transfer Accumulator to X register
func (cpu *CPU) tax(inst *Instruction, operand []byte) {
	cpu.Reg.X = cpu.Reg.A
	cpu.updateNZ(cpu.Reg.X)
}

// Transfer Accumulator to Y register
func (cpu *CPU) tay(inst *Instruction, operand []byte) {
	cpu.Reg.Y = cpu.Reg.A
	cpu.updateNZ(cpu.Reg.Y)
}

// Test and Reset Bits (65c02 only)
func (cpu *CPU) trb(inst *Instruction, operand []byte) {
	v := cpu.load(inst.Mode, operand)
	cpu.Reg.Zero = ((v & cpu.Reg.A) == 0)
	nv := (v & (cpu.Reg.A ^ 0xff))
	cpu.store(inst.Mode, operand, nv)
}

// Test and Set Bits (65c02 only)
func (cpu *CPU) tsb(inst *Instruction, operand []byte) {
	v := cpu.load(inst.Mode, operand)
	cpu.Reg.Zero = ((v & cpu.Reg.A) == 0)
	nv := (v | cpu.Reg.A)
	cpu.store(inst.Mode, operand, nv)
}

// Transfer stack pointer to X register
func (cpu *CPU) tsx(inst *Instruction, operand []byte) {
	cpu.Reg.X = cpu.Reg.SP
	cpu.updateNZ(cpu.Reg.X)
}

// Transfer X register to Accumulator
func (cpu *CPU) txa(inst *Instruction, operand []byte) {
	cpu.Reg.A = cpu.Reg.X
	cpu.updateNZ(cpu.Reg.A)
}

// Transfer X register to the stack pointer
func (cpu *CPU) txs(inst *Instruction, operand []byte) {
	cpu.Reg.SP = cpu.Reg.X
}

// Transfer Y register to the Accumulator
func (cpu *CPU) tya(inst *Instruction, operand []byte) {
	cpu.Reg.A = cpu.Reg.Y
	cpu.updateNZ(cpu.Reg.A)
}

// Unused instruction (6502)
func (cpu *CPU) unusedn(inst *Instruction, operand []byte) {
	// Do nothing
}

// Unused instruction (65c02)
func (cpu *CPU) unusedc(inst *Instruction, operand []byte) {
	// Do nothing
}
