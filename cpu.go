// Copyright 2014 Brett Vickers. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package go6502 implements a 6502 CPU instruction
// set and emulator.
package go6502

// Architecture selects the CPU chip: 6502 or 65c02
type Architecture byte

const (
	// NMOS 6502 CPU
	NMOS Architecture = iota

	// CMOS 65c02 CPU
	CMOS
)

// CPU represents a single 6502 CPU. It contains a pointer to the
// memory associated with the CPU.
type CPU struct {
	Arch         Architecture // CPU architecture
	Reg          Registers    // CPU registers
	Mem          Memory       // assigned memory
	Cycles       uint64       // total executed CPU cycles
	instructions *InstructionSet
	pageCrossed  bool
	deltaCycles  int8
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
		Arch:         arch,
		Mem:          m,
		instructions: GetInstructionSet(arch),
	}

	cpu.Reg.Init()
	return cpu
}

// SetPC updates the CPU program counter to 'addr'.
func (cpu *CPU) SetPC(addr Address) {
	cpu.Reg.PC = addr
}

// Step the cpu by one instruction.
func (cpu *CPU) Step() {
	// Grab the next opcode at the current PC
	opcode, err := cpu.Mem.LoadByte(cpu.Reg.PC)
	if err != nil {
		panic(err)
	}

	// Look up the instruction data for the opcode
	inst := cpu.instructions.Lookup(opcode)

	// If the instruction is undefined, reset the CPU (for now).
	if inst.fn == nil {
		cpu.reset()
		return
	}

	// Fetch the operand (if any) and advance the PC
	var buf [2]byte
	operand := buf[:inst.Length-1]
	err = cpu.Mem.LoadBytes(cpu.Reg.PC+1, operand)
	cpu.Reg.PC += Address(inst.Length)
	if err != nil {
		panic(err)
	}

	// Execute the instruction
	cpu.pageCrossed = false
	cpu.deltaCycles = 0
	err = inst.fn(cpu, inst, operand)
	if err != nil {
		panic(err)
	}

	// Update the CPU cycle counter, with special-case logic
	// to handle a page boundary crossing
	cpu.Cycles += uint64(int8(inst.Cycles) + cpu.deltaCycles)
	if cpu.pageCrossed {
		cpu.Cycles += uint64(inst.BPCycles)
	}
}

// Load a byte value using the requested addressing mode
// and the variable-sized instruction operand.
func (cpu *CPU) load(mode Mode, operand []byte) (byte, error) {
	switch mode {
	case IMM:
		return operand[0], nil
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
		addr, err := cpu.Mem.LoadAddress(zpaddr)
		if err != nil {
			return 0, err
		}
		return cpu.Mem.LoadByte(addr)
	case IDY:
		zpaddr := operandToAddress(operand)
		addr, err := cpu.Mem.LoadAddress(zpaddr)
		if err != nil {
			return 0, err
		}
		addr, cpu.pageCrossed = offsetAddress(addr, cpu.Reg.Y)
		return cpu.Mem.LoadByte(addr)
	case ACC:
		return cpu.Reg.A, nil
	default:
		panic("Invalid addressing mode")
	}
}

// Load a 16-bit address value using the requested addressing mode
// and the 16-bit instruction operand.
func (cpu *CPU) loadAddress(mode Mode, operand []byte) (Address, error) {
	switch mode {
	case ABS:
		return operandToAddress(operand), nil
	case IND:
		addr := operandToAddress(operand)
		return cpu.Mem.LoadAddress(addr)
	default:
		panic("Invalid addressing mode")
	}
}

// Store the value 'v' using the specified addressing mode and the
// variable-sized instruction operand.
func (cpu *CPU) store(mode Mode, operand []byte, v byte) error {
	switch mode {
	case ZPG:
		zpaddr := operandToAddress(operand)
		return cpu.Mem.StoreByte(zpaddr, v)
	case ZPX:
		zpaddr := operandToAddress(operand)
		zpaddr = offsetZeroPage(zpaddr, cpu.Reg.X)
		return cpu.Mem.StoreByte(zpaddr, v)
	case ZPY:
		zpaddr := operandToAddress(operand)
		zpaddr = offsetZeroPage(zpaddr, cpu.Reg.Y)
		return cpu.Mem.StoreByte(zpaddr, v)
	case ABS:
		addr := operandToAddress(operand)
		return cpu.Mem.StoreByte(addr, v)
	case ABX:
		addr := operandToAddress(operand)
		addr, cpu.pageCrossed = offsetAddress(addr, cpu.Reg.X)
		return cpu.Mem.StoreByte(addr, v)
	case ABY:
		addr := operandToAddress(operand)
		addr, cpu.pageCrossed = offsetAddress(addr, cpu.Reg.Y)
		return cpu.Mem.StoreByte(addr, v)
	case IDX:
		zpaddr := operandToAddress(operand)
		zpaddr = offsetZeroPage(zpaddr, cpu.Reg.X)
		addr, err := cpu.Mem.LoadAddress(zpaddr)
		if err != nil {
			return err
		}
		return cpu.Mem.StoreByte(addr, v)
	case IDY:
		zpaddr := operandToAddress(operand)
		addr, err := cpu.Mem.LoadAddress(zpaddr)
		if err != nil {
			return err
		}
		addr, cpu.pageCrossed = offsetAddress(addr, cpu.Reg.Y)
		return cpu.Mem.StoreByte(addr, v)
	case ACC:
		cpu.Reg.A = v
		return nil
	default:
		panic("Invalid addressing mode")
	}
}

// Execute a branch using the instruction operand.
func (cpu *CPU) branch(operand []byte) {
	offset := operandToAddress(operand)
	oldPC := cpu.Reg.PC
	if offset < 0x80 {
		cpu.Reg.PC += Address(offset)
	} else {
		cpu.Reg.PC -= Address(0x100 - offset)
	}
	cpu.deltaCycles++
	if ((cpu.Reg.PC ^ oldPC) & 0xff00) != 0 {
		cpu.deltaCycles++
	}
}

// Push a value 'v' onto the stack.
func (cpu *CPU) push(v byte) error {
	err := cpu.Mem.StoreByte(stackAddress(cpu.Reg.SP), v)
	cpu.Reg.SP--
	return err
}

// Push the address 'addr' onto the stack.
func (cpu *CPU) pushAddress(addr Address) error {
	err := cpu.push(byte(addr >> 8))
	if err != nil {
		return err
	}
	return cpu.push(byte(addr))
}

// Pop a value from the stack and return it.
func (cpu *CPU) pop() (byte, error) {
	cpu.Reg.SP++
	return cpu.Mem.LoadByte(stackAddress(cpu.Reg.SP))
}

// Pop a 16-bit address off the stack.
func (cpu *CPU) popAddress() (Address, error) {
	lo, err := cpu.pop()
	if err != nil {
		return 0, err
	}

	hi, err := cpu.pop()
	if err != nil {
		return 0, err
	}

	return Address(lo) | (Address(hi) << 8), nil
}

// Update the Zero and Negative flags based on the value of 'v'.
func (cpu *CPU) updateNZ(v byte) {
	cpu.Reg.Zero = (v == 0)
	cpu.Reg.Sign = ((v & 0x80) != 0)
}

// Handle an handleInterrupt by storing the program counter and status
// flags on the stack. Then switch the program counter to the requested
// address.
func (cpu *CPU) handleInterrupt(brk bool, addr Address) error {
	err := cpu.pushAddress(cpu.Reg.PC)
	if err != nil {
		return err
	}

	err = cpu.push(cpu.Reg.SavePS(brk))
	if err != nil {
		return err
	}

	cpu.Reg.InterruptDisable = true
	if cpu.Arch == CMOS {
		cpu.Reg.Decimal = false
	}

	cpu.Reg.PC, err = cpu.Mem.LoadAddress(addr)
	return err
}

// Generate a maskable IRQ (hardware) interrupt request.
func (cpu *CPU) irq() error {
	if !cpu.Reg.InterruptDisable {
		return cpu.handleInterrupt(false, vectorIRQ)
	}
	return nil
}

// Generate a non-maskable interrupt.
func (cpu *CPU) nmi() error {
	return cpu.handleInterrupt(false, vectorNMI)
}

// Generate a reset signal.
func (cpu *CPU) reset() error {
	var err error
	cpu.Reg.PC, err = cpu.Mem.LoadAddress(vectorReset)
	return err
}

// Add with carry (CMOS)
func (cpu *CPU) adcc(inst *Instruction, operand []byte) error {
	acc := uint32(cpu.Reg.A)
	addv, err := cpu.load(inst.Mode, operand)
	add := uint32(addv)
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
	return err
}

// Add with carry (NMOS)
func (cpu *CPU) adcn(inst *Instruction, operand []byte) error {
	acc := uint32(cpu.Reg.A)
	addv, err := cpu.load(inst.Mode, operand)
	add := uint32(addv)
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
	return err
}

// Boolean AND
func (cpu *CPU) and(inst *Instruction, operand []byte) error {
	v, err := cpu.load(inst.Mode, operand)
	cpu.Reg.A &= v
	cpu.updateNZ(cpu.Reg.A)
	return err
}

// Arithmetic Shift Left
func (cpu *CPU) asl(inst *Instruction, operand []byte) error {
	v, err := cpu.load(inst.Mode, operand)
	if err != nil {
		return err
	}
	cpu.Reg.Carry = ((v & 0x80) == 0x80)
	v = v << 1
	cpu.updateNZ(v)
	err = cpu.store(inst.Mode, operand, v)
	if cpu.Arch == CMOS && inst.Mode == ABX && !cpu.pageCrossed {
		cpu.deltaCycles--
	}
	return err
}

// Branch if Carry Clear
func (cpu *CPU) bcc(inst *Instruction, operand []byte) error {
	if !cpu.Reg.Carry {
		cpu.branch(operand)
	}
	return nil
}

// Branch if Carry Set
func (cpu *CPU) bcs(inst *Instruction, operand []byte) error {
	if cpu.Reg.Carry {
		cpu.branch(operand)
	}
	return nil
}

// Branch if EQual (to zero)
func (cpu *CPU) beq(inst *Instruction, operand []byte) error {
	if cpu.Reg.Zero {
		cpu.branch(operand)
	}
	return nil
}

// Bit Test
func (cpu *CPU) bit(inst *Instruction, operand []byte) error {
	v, err := cpu.load(inst.Mode, operand)
	cpu.Reg.Zero = ((v & cpu.Reg.A) == 0)
	cpu.Reg.Sign = ((v & 0x80) != 0)
	cpu.Reg.Overflow = ((v & 0x40) != 0)
	return err
}

// Branch if MInus (negative)
func (cpu *CPU) bmi(inst *Instruction, operand []byte) error {
	if cpu.Reg.Sign {
		cpu.branch(operand)
	}
	return nil
}

// Branch if Not Equal (not zero)
func (cpu *CPU) bne(inst *Instruction, operand []byte) error {
	if !cpu.Reg.Zero {
		cpu.branch(operand)
	}
	return nil
}

// Branch if PLus (positive)
func (cpu *CPU) bpl(inst *Instruction, operand []byte) error {
	if !cpu.Reg.Sign {
		cpu.branch(operand)
	}
	return nil
}

// Branch always (65c02 only)
func (cpu *CPU) bra(inst *Instruction, operand []byte) error {
	cpu.branch(operand)
	return nil
}

// Break
func (cpu *CPU) brk(inst *Instruction, operand []byte) error {
	cpu.Reg.PC++
	return cpu.handleInterrupt(true, vectorBRK)
}

// Branch if oVerflow Clear
func (cpu *CPU) bvc(inst *Instruction, operand []byte) error {
	if !cpu.Reg.Overflow {
		cpu.branch(operand)
	}
	return nil
}

// Branch if oVerflow Set
func (cpu *CPU) bvs(inst *Instruction, operand []byte) error {
	if cpu.Reg.Overflow {
		cpu.branch(operand)
	}
	return nil
}

// Clear Carry flag
func (cpu *CPU) clc(inst *Instruction, operand []byte) error {
	cpu.Reg.Carry = false
	return nil
}

// Clear Decimal flag
func (cpu *CPU) cld(inst *Instruction, operand []byte) error {
	cpu.Reg.Decimal = false
	return nil
}

// Clear InterruptDisable flag
func (cpu *CPU) cli(inst *Instruction, operand []byte) error {
	cpu.Reg.InterruptDisable = false
	return nil
}

// Clear oVerflow flag
func (cpu *CPU) clv(inst *Instruction, operand []byte) error {
	cpu.Reg.Overflow = false
	return nil
}

// Compare to accumulator
func (cpu *CPU) cmp(inst *Instruction, operand []byte) error {
	v, err := cpu.load(inst.Mode, operand)
	cpu.Reg.Carry = (cpu.Reg.A >= v)
	cpu.updateNZ(cpu.Reg.A - v)
	return err
}

// Compare to X register
func (cpu *CPU) cpx(inst *Instruction, operand []byte) error {
	v, err := cpu.load(inst.Mode, operand)
	cpu.Reg.Carry = (cpu.Reg.X >= v)
	cpu.updateNZ(cpu.Reg.X - v)
	return err
}

// Compare to Y register
func (cpu *CPU) cpy(inst *Instruction, operand []byte) error {
	v, err := cpu.load(inst.Mode, operand)
	cpu.Reg.Carry = (cpu.Reg.Y >= v)
	cpu.updateNZ(cpu.Reg.Y - v)
	return err
}

// Decrement memory value
func (cpu *CPU) dec(inst *Instruction, operand []byte) error {
	v, err := cpu.load(inst.Mode, operand)
	if err != nil {
		return err
	}
	v--
	cpu.updateNZ(v)
	return cpu.store(inst.Mode, operand, v)
}

// Decrement X register
func (cpu *CPU) dex(inst *Instruction, operand []byte) error {
	cpu.Reg.X--
	cpu.updateNZ(cpu.Reg.X)
	return nil
}

// Decrement Y register
func (cpu *CPU) dey(inst *Instruction, operand []byte) error {
	cpu.Reg.Y--
	cpu.updateNZ(cpu.Reg.Y)
	return nil
}

// Boolean XOR
func (cpu *CPU) eor(inst *Instruction, operand []byte) error {
	v, err := cpu.load(inst.Mode, operand)
	cpu.Reg.A ^= v
	cpu.updateNZ(cpu.Reg.A)
	return err
}

// Increment memory value
func (cpu *CPU) inc(inst *Instruction, operand []byte) error {
	v, err := cpu.load(inst.Mode, operand)
	if err != nil {
		return err
	}
	v++
	cpu.updateNZ(v)
	return cpu.store(inst.Mode, operand, v)
}

// Increment X register
func (cpu *CPU) inx(inst *Instruction, operand []byte) error {
	cpu.Reg.X++
	cpu.updateNZ(cpu.Reg.X)
	return nil
}

// Increment Y register
func (cpu *CPU) iny(inst *Instruction, operand []byte) error {
	cpu.Reg.Y++
	cpu.updateNZ(cpu.Reg.Y)
	return nil
}

// Jump to memory address (NMOS 6502)
func (cpu *CPU) jmpn(inst *Instruction, operand []byte) error {
	var err error
	cpu.Reg.PC, err = cpu.loadAddress(inst.Mode, operand)
	return err
}

// Jump to memory address (CMOS 65c02)
func (cpu *CPU) jmpc(inst *Instruction, operand []byte) error {
	if inst.Mode == IND && operand[0] == 0xff {
		// Fix bug in NMOS 6502 address loading. In NMOS 6502, a JMP ($12FF)
		// would load LSB of jmp target from $12FF and MSB from $1200.
		// In CMOS, it loads the MSB from $1300.
		addr0 := Address(operand[1])<<8 | 0xff
		addr1 := addr0 + 1
		lo, err := cpu.Mem.LoadByte(addr0)
		if err != nil {
			return err
		}
		hi, err := cpu.Mem.LoadByte(addr1)
		if err != nil {
			return err
		}
		cpu.Reg.PC = Address(lo) | Address(hi)<<8
		cpu.deltaCycles++
		return nil
	}

	var err error
	cpu.Reg.PC, err = cpu.loadAddress(inst.Mode, operand)
	return err
}

// Jump to subroutine
func (cpu *CPU) jsr(inst *Instruction, operand []byte) error {
	addr, err := cpu.loadAddress(inst.Mode, operand)
	cpu.Reg.PC--
	err = cpu.pushAddress(cpu.Reg.PC)
	cpu.Reg.PC = addr
	return err
}

// load Accumulator
func (cpu *CPU) lda(inst *Instruction, operand []byte) error {
	var err error
	cpu.Reg.A, err = cpu.load(inst.Mode, operand)
	cpu.updateNZ(cpu.Reg.A)
	return err
}

// load the X register
func (cpu *CPU) ldx(inst *Instruction, operand []byte) error {
	var err error
	cpu.Reg.X, err = cpu.load(inst.Mode, operand)
	cpu.updateNZ(cpu.Reg.X)
	return err
}

// load the Y register
func (cpu *CPU) ldy(inst *Instruction, operand []byte) error {
	var err error
	cpu.Reg.Y, err = cpu.load(inst.Mode, operand)
	cpu.updateNZ(cpu.Reg.Y)
	return err
}

// Logical Shift Right
func (cpu *CPU) lsr(inst *Instruction, operand []byte) error {
	v, err := cpu.load(inst.Mode, operand)
	if err != nil {
		return err
	}
	cpu.Reg.Carry = ((v & 1) == 1)
	v = v >> 1
	cpu.updateNZ(v)
	err = cpu.store(inst.Mode, operand, v)
	if cpu.Arch == CMOS && inst.Mode == ABX && !cpu.pageCrossed {
		cpu.deltaCycles--
	}
	return err
}

// No-operation
func (cpu *CPU) nop(inst *Instruction, operand []byte) error {
	// Do nothing
	return nil
}

// Boolean OR
func (cpu *CPU) ora(inst *Instruction, operand []byte) error {
	v, err := cpu.load(inst.Mode, operand)
	cpu.Reg.A |= v
	cpu.updateNZ(cpu.Reg.A)
	return err
}

// Push Accumulator
func (cpu *CPU) pha(inst *Instruction, operand []byte) error {
	return cpu.push(cpu.Reg.A)
}

// Push Processor flags
func (cpu *CPU) php(inst *Instruction, operand []byte) error {
	return cpu.push(cpu.Reg.SavePS(true))
}

// Push X register (65c02 only)
func (cpu *CPU) phx(inst *Instruction, operand []byte) error {
	return cpu.push(cpu.Reg.X)
}

// Push Y register (65c02 only)
func (cpu *CPU) phy(inst *Instruction, operand []byte) error {
	return cpu.push(cpu.Reg.Y)
}

// Pull (pop) Accumulator
func (cpu *CPU) pla(inst *Instruction, operand []byte) error {
	var err error
	cpu.Reg.A, err = cpu.pop()
	cpu.updateNZ(cpu.Reg.A)
	return err
}

// Pull (pop) Processor flags
func (cpu *CPU) plp(inst *Instruction, operand []byte) error {
	v, err := cpu.pop()
	cpu.Reg.RestorePS(v)
	return err
}

// Pull (pop) X register (65c02 only)
func (cpu *CPU) plx(inst *Instruction, operand []byte) error {
	var err error
	cpu.Reg.X, err = cpu.pop()
	cpu.updateNZ(cpu.Reg.X)
	return err
}

// Pull (pop) Y register (65c02 only)
func (cpu *CPU) ply(inst *Instruction, operand []byte) error {
	var err error
	cpu.Reg.Y, err = cpu.pop()
	cpu.updateNZ(cpu.Reg.Y)
	return err
}

// Rotate left
func (cpu *CPU) rol(inst *Instruction, operand []byte) error {
	tmp, err := cpu.load(inst.Mode, operand)
	if err != nil {
		return err
	}
	v := (tmp << 1) | boolToByte(cpu.Reg.Carry)
	cpu.Reg.Carry = ((tmp & 0x80) != 0)
	cpu.updateNZ(v)
	err = cpu.store(inst.Mode, operand, v)
	if cpu.Arch == CMOS && inst.Mode == ABX && !cpu.pageCrossed {
		cpu.deltaCycles--
	}
	return err
}

// Rotate right
func (cpu *CPU) ror(inst *Instruction, operand []byte) error {
	tmp, err := cpu.load(inst.Mode, operand)
	if err != nil {
		return err
	}
	v := (tmp >> 1) | (boolToByte(cpu.Reg.Carry) << 7)
	cpu.Reg.Carry = ((tmp & 1) != 0)
	cpu.updateNZ(v)
	err = cpu.store(inst.Mode, operand, v)
	if cpu.Arch == CMOS && inst.Mode == ABX && !cpu.pageCrossed {
		cpu.deltaCycles--
	}
	return err
}

// Return from interrupt
func (cpu *CPU) rti(inst *Instruction, operand []byte) error {
	v, err := cpu.pop()
	if err != nil {
		return err
	}

	cpu.Reg.RestorePS(v)

	cpu.Reg.PC, err = cpu.popAddress()
	return err
}

// Return from Subroutine
func (cpu *CPU) rts(inst *Instruction, operand []byte) error {
	addr, err := cpu.popAddress()
	cpu.Reg.PC = addr + 1
	return err
}

// Subtract with Carry (CMOS)
func (cpu *CPU) sbcc(inst *Instruction, operand []byte) error {
	acc := uint32(cpu.Reg.A)
	subv, err := cpu.load(inst.Mode, operand)
	sub := uint32(subv)
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
	return err
}

// Subtract with Carry (NMOS)
func (cpu *CPU) sbcn(inst *Instruction, operand []byte) error {
	acc := uint32(cpu.Reg.A)
	subv, err := cpu.load(inst.Mode, operand)
	sub := uint32(subv)
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
	return err
}

// Set Carry flag
func (cpu *CPU) sec(inst *Instruction, operand []byte) error {
	cpu.Reg.Carry = true
	return nil
}

// Set Decimal flag
func (cpu *CPU) sed(inst *Instruction, operand []byte) error {
	cpu.Reg.Decimal = true
	return nil
}

// Set InterruptDisable flag
func (cpu *CPU) sei(inst *Instruction, operand []byte) error {
	cpu.Reg.InterruptDisable = true
	return nil
}

// store Accumulator
func (cpu *CPU) sta(inst *Instruction, operand []byte) error {
	return cpu.store(inst.Mode, operand, cpu.Reg.A)
}

// store X register
func (cpu *CPU) stx(inst *Instruction, operand []byte) error {
	return cpu.store(inst.Mode, operand, cpu.Reg.X)
}

// store Y register
func (cpu *CPU) sty(inst *Instruction, operand []byte) error {
	return cpu.store(inst.Mode, operand, cpu.Reg.Y)
}

// store zero (65c02 only)
func (cpu *CPU) stz(inst *Instruction, operand []byte) error {
	return cpu.store(inst.Mode, operand, 0)
}

// Transfer Accumulator to X register
func (cpu *CPU) tax(inst *Instruction, operand []byte) error {
	cpu.Reg.X = cpu.Reg.A
	cpu.updateNZ(cpu.Reg.X)
	return nil
}

// Transfer Accumulator to Y register
func (cpu *CPU) tay(inst *Instruction, operand []byte) error {
	cpu.Reg.Y = cpu.Reg.A
	cpu.updateNZ(cpu.Reg.Y)
	return nil
}

// Test and reset bits (65c02 only)
func (cpu *CPU) trb(inst *Instruction, operand []byte) error {
	v, err := cpu.load(inst.Mode, operand)
	if err != nil {
		return err
	}
	cpu.Reg.Zero = ((v & cpu.Reg.A) == 0)
	nv := (v & (cpu.Reg.A ^ 0xff))
	return cpu.store(inst.Mode, operand, nv)
}

// Test and set bits (65c02 only)
func (cpu *CPU) tsb(inst *Instruction, operand []byte) error {
	v, err := cpu.load(inst.Mode, operand)
	if err != nil {
		return err
	}
	cpu.Reg.Zero = ((v & cpu.Reg.A) == 0)
	nv := (v | cpu.Reg.A)
	return cpu.store(inst.Mode, operand, nv)
}

// Transfer Stack pointer to X register
func (cpu *CPU) tsx(inst *Instruction, operand []byte) error {
	cpu.Reg.X = cpu.Reg.SP
	cpu.updateNZ(cpu.Reg.X)
	return nil
}

// Transfer X register to Accumulator
func (cpu *CPU) txa(inst *Instruction, operand []byte) error {
	cpu.Reg.A = cpu.Reg.X
	cpu.updateNZ(cpu.Reg.A)
	return nil
}

// Transfer X register to the Stack pointer
func (cpu *CPU) txs(inst *Instruction, operand []byte) error {
	cpu.Reg.SP = cpu.Reg.X
	return nil
}

// Transfer Y register to the Accumulator
func (cpu *CPU) tya(inst *Instruction, operand []byte) error {
	cpu.Reg.A = cpu.Reg.Y
	cpu.updateNZ(cpu.Reg.A)
	return nil
}

// Unused instruction (6502)
func (cpu *CPU) unusedn(inst *Instruction, operand []byte) error {
	// Do nothing
	return nil
}

// Unused instruction (65c02)
func (cpu *CPU) unusedc(inst *Instruction, operand []byte) error {
	// Do nothing
	return nil
}
