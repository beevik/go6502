// Copyright 2014 Brett Vickers. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package go6502 implements a 6502 CPU instruction
// set and emulator.
package go6502

const (
	interruptPeriod = 60
)

// CPU represents a single 6502 CPU. It contains a pointer to the
// memory associated with the CPU.
type CPU struct {
	Reg         Registers // CPU registers
	Mem         *Memory   // assigned memory
	Cycles      uint64    // total executed CPU cycles
	CMOS        bool      // true if 65C02
	pageCrossed bool
	extraCycles uint32
}

// Interrupt vectors
const (
	vectorNMI   = 0xfffa
	vectorReset = 0xfffc
	vectorIRQ   = 0xfffe
	vectorBRK   = 0xfffe
)

// NewCPU creates a new 65C02 CPU object bound to the specified memory.
func NewCPU(m *Memory) *CPU {
	cpu := &CPU{Mem: m, CMOS: true}
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
	opcode := cpu.Mem.LoadByte(cpu.Reg.PC)

	// Look up the instruction data for the opcode
	inst := &Instructions[opcode]

	// Fetch the operand (if any) and advance the PC
	operand := cpu.Mem.LoadBytes(cpu.Reg.PC+1, int(inst.Length)-1)
	cpu.Reg.PC += Address(inst.Length)

	// Execute the instruction
	cpu.pageCrossed = false
	cpu.extraCycles = 0
	switch {
	case cpu.CMOS && inst.fnCMOS != nil:
		inst.fnCMOS(cpu, inst, operand)
	case !cpu.CMOS && inst.fnNMOS != nil:
		inst.fnNMOS(cpu, inst, operand)
	}

	// Update the CPU cycle counter, with special-case logic
	// to handle a page boundary crossing
	cpu.Cycles += uint64(inst.Cycles) + uint64(cpu.extraCycles)
	if cpu.pageCrossed {
		cpu.Cycles += uint64(inst.BPCycles)
	}
}

// Load a byte value using the requested addressing mode
// and the variable-sized instruction operand.
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

// Load a 16-bit address value using the requested addressing mode
// and the 16-bit instruction operand.
func (cpu *CPU) loadAddress(mode Mode, operand []byte) Address {
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

// Store the value 'v' using the specified addressing mode and the
// variable-sized instruction operand.
func (cpu *CPU) store(mode Mode, operand []byte, v byte) {
	switch mode {
	case ZPG:
		zpaddr := operandToAddress(operand)
		cpu.Mem.StoreByte(zpaddr, v)
	case ZPX:
		zpaddr := operandToAddress(operand)
		zpaddr = offsetZeroPage(zpaddr, cpu.Reg.X)
		cpu.Mem.StoreByte(zpaddr, v)
	case ZPY:
		zpaddr := operandToAddress(operand)
		zpaddr = offsetZeroPage(zpaddr, cpu.Reg.Y)
		cpu.Mem.StoreByte(zpaddr, v)
	case ABS:
		addr := operandToAddress(operand)
		cpu.Mem.StoreByte(addr, v)
	case ABX:
		addr := operandToAddress(operand)
		addr, cpu.pageCrossed = offsetAddress(addr, cpu.Reg.X)
		cpu.Mem.StoreByte(addr, v)
	case ABY:
		addr := operandToAddress(operand)
		addr, cpu.pageCrossed = offsetAddress(addr, cpu.Reg.Y)
		cpu.Mem.StoreByte(addr, v)
	case IDX:
		zpaddr := operandToAddress(operand)
		zpaddr = offsetZeroPage(zpaddr, cpu.Reg.X)
		addr := cpu.Mem.LoadAddress(zpaddr)
		cpu.Mem.StoreByte(addr, v)
	case IDY:
		zpaddr := operandToAddress(operand)
		addr := cpu.Mem.LoadAddress(zpaddr)
		addr, cpu.pageCrossed = offsetAddress(addr, cpu.Reg.Y)
		cpu.Mem.StoreByte(addr, v)
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
		cpu.Reg.PC += Address(offset)
	} else {
		cpu.Reg.PC -= Address(0x100 - offset)
	}
	cpu.extraCycles++
	if ((cpu.Reg.PC ^ oldPC) & 0xff00) != 0 {
		cpu.extraCycles++
	}
}

// Push a value 'v' onto the stack.
func (cpu *CPU) push(v byte) {
	cpu.Mem.StoreByte(stackAddress(cpu.Reg.SP), v)
	cpu.Reg.SP--
}

// Pop a value from the stack and return it.
func (cpu *CPU) pop() byte {
	cpu.Reg.SP++
	return cpu.Mem.LoadByte(stackAddress(cpu.Reg.SP))
}

// Update the Zero and Negative flags based on the value of 'v'.
func (cpu *CPU) updateNZ(v byte) {
	cpu.Reg.Zero = (v == 0)
	cpu.Reg.Sign = ((v & 0x80) != 0)
}

// Handle an handleInterrupt by storing the program counter and status
// flags on the stack. Then switch the program counter to the requested
// address.
func (cpu *CPU) handleInterrupt(brk bool, addr Address) {
	cpu.push(byte(cpu.Reg.PC >> 8))
	cpu.push(byte(cpu.Reg.PC & 0xff))
	cpu.push(cpu.Reg.SavePS(brk))

	cpu.Reg.InterruptDisable = true
	if cpu.CMOS {
		cpu.Reg.Decimal = false
	}

	cpu.Reg.PC = cpu.Mem.LoadAddress(addr)
}

// Generate a maskable IRQ (hardware) interrupt request.
func (cpu *CPU) irq() {
	if !cpu.Reg.InterruptDisable {
		cpu.handleInterrupt(false, vectorIRQ)
	}
}

// Generate a non-maskable interrupt.
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
		cpu.extraCycles++

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
	cpu.store(inst.Mode, operand, v)
	cpu.updateNZ(v)
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
	cpu.updateNZ(v)
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

// Jump to memory address
func (cpu *CPU) jmp(inst *Instruction, operand []byte) {
	cpu.Reg.PC = cpu.loadAddress(inst.Mode, operand)
}

// Jump to subroutine
func (cpu *CPU) jsr(inst *Instruction, operand []byte) {
	addr := cpu.loadAddress(inst.Mode, operand)
	cpu.Reg.PC--
	cpu.push(byte(cpu.Reg.PC >> 8))
	cpu.push(byte(cpu.Reg.PC & 0xff))
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
	cpu.store(inst.Mode, operand, v)
	cpu.updateNZ(v)
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

// push Accumulator
func (cpu *CPU) pha(inst *Instruction, operand []byte) {
	cpu.push(cpu.Reg.A)
}

// push Processor flags
func (cpu *CPU) php(inst *Instruction, operand []byte) {
	cpu.push(cpu.Reg.SavePS(true))
}

// Pull (pop) Accumulator
func (cpu *CPU) pla(inst *Instruction, operand []byte) {
	cpu.Reg.A = cpu.pop()
	cpu.updateNZ(cpu.Reg.A)
}

// Pull (pop) Processor flags
func (cpu *CPU) plp(inst *Instruction, operand []byte) {
	cpu.Reg.RestorePS(cpu.pop())
}

// Rotate left
func (cpu *CPU) rol(inst *Instruction, operand []byte) {
	tmp := cpu.load(inst.Mode, operand)
	v := (tmp << 1) | boolToByte(cpu.Reg.Carry)
	cpu.Reg.Carry = ((tmp & 0x80) != 0)
	cpu.store(inst.Mode, operand, v)
	cpu.updateNZ(v)
}

// Rotate right
func (cpu *CPU) ror(inst *Instruction, operand []byte) {
	tmp := cpu.load(inst.Mode, operand)
	v := (tmp >> 1) | (boolToByte(cpu.Reg.Carry) << 7)
	cpu.Reg.Carry = ((tmp & 1) != 0)
	cpu.store(inst.Mode, operand, v)
	cpu.updateNZ(v)
}

// Return from interrupt
func (cpu *CPU) rti(inst *Instruction, operand []byte) {
	cpu.Reg.RestorePS(cpu.pop())
	cpu.Reg.Break = false
	cpu.Reg.PC = Address(cpu.pop()) | (Address(cpu.pop()) << 8)
}

// Return from Subroutine
func (cpu *CPU) rts(inst *Instruction, operand []byte) {
	addr := Address(cpu.pop()) | (Address(cpu.pop()) << 8)
	cpu.Reg.PC = addr + Address(1)
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
		cpu.extraCycles++

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

// store Accumulator
func (cpu *CPU) sta(inst *Instruction, operand []byte) {
	cpu.store(inst.Mode, operand, cpu.Reg.A)
}

// store X register
func (cpu *CPU) stx(inst *Instruction, operand []byte) {
	cpu.store(inst.Mode, operand, cpu.Reg.X)
}

// store Y register
func (cpu *CPU) sty(inst *Instruction, operand []byte) {
	cpu.store(inst.Mode, operand, cpu.Reg.Y)
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

// Transfer Stack pointer to X register
func (cpu *CPU) tsx(inst *Instruction, operand []byte) {
	cpu.Reg.X = cpu.Reg.SP
	cpu.updateNZ(cpu.Reg.X)
}

// Transfer X register to Accumulator
func (cpu *CPU) txa(inst *Instruction, operand []byte) {
	cpu.Reg.A = cpu.Reg.X
	cpu.updateNZ(cpu.Reg.A)
}

// Transfer X register to the Stack pointer
func (cpu *CPU) txs(inst *Instruction, operand []byte) {
	cpu.Reg.SP = cpu.Reg.X
}

// Transfer Y register to the Accumulator
func (cpu *CPU) tya(inst *Instruction, operand []byte) {
	cpu.Reg.A = cpu.Reg.Y
	cpu.updateNZ(cpu.Reg.A)
}
