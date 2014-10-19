// Copyright 2014 Brett Vickers. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package go6502 implements a 6502 CPU instruction
// set and emulator.
package go6502

const (
	interruptPeriod = 60
)

// Cpu represents a single 6502 CPU. It contains a pointer to the
// memory associated with the CPU.
type Cpu struct {
	Reg         Registers // CPU registers
	Mem         *Memory   // assigned memory
	Cycles      uint64    // total executed CPU cycles
	pageCrossed bool
	extraCycles uint32
}

// Return a new Cpu object, bound to the specified memory.
func NewCpu(m *Memory) *Cpu {
	cpu := &Cpu{Mem: m}
	cpu.Reg.Init()
	return cpu
}

// Set the CPU program counter to 'addr'.
func (cpu *Cpu) SetPC(addr Address) {
	cpu.Reg.PC = addr
}

// Step the cpu by one instruction.
func (cpu *Cpu) Step() {
	opcode := cpu.Mem.ReadByte(cpu.Reg.PC)
	cpu.Reg.PC++

	// Look up the instruction for the opcode
	idata := &Instructions[opcode]

	// Fetch the operand (if any) and advance the PC
	operand := cpu.Mem.ReadBytes(cpu.Reg.PC, int(idata.Length)-1)
	cpu.Reg.PC += Address(idata.Length - 1)

	// Execute the opcode instruction
	cpu.pageCrossed = false
	cpu.extraCycles = 0
	if idata.fn != nil {
		idata.fn(cpu, idata, operand)
	}

	// Update the CPU cycle counter, with special case logic
	// when a page boundary is crossed
	cpu.Cycles += uint64(idata.Cycles) + uint64(cpu.extraCycles)
	if cpu.pageCrossed {
		cpu.Cycles += uint64(idata.BPCycles)
	}
}

// Run the Cpu indefinitely.
func (cpu *Cpu) Run() {
	exitRequired := false
	interruptCounter := 0
	for {
		// Single-step the processor by one instruction
		cpu.Step()

		// Check for interrupts
		if interruptCounter < 0 {
			interruptCounter += interruptPeriod
			if exitRequired {
				break
			}
		}
	}

}

// Load a byte value using the requested addressing mode
// and the variable-sized instruction operand.
func (cpu *Cpu) load(mode Mode, operand []byte) byte {
	switch mode {
	case IMM:
		return operand[0]
	case ZPG:
		zpaddr := operandToAddress(operand)
		return cpu.Mem.ReadByte(zpaddr)
	case ZPX:
		zpaddr := operandToAddress(operand)
		zpaddr = offsetZeroPage(zpaddr, cpu.Reg.X)
		return cpu.Mem.ReadByte(zpaddr)
	case ZPY:
		zpaddr := operandToAddress(operand)
		zpaddr = offsetZeroPage(zpaddr, cpu.Reg.Y)
		return cpu.Mem.ReadByte(zpaddr)
	case ABS:
		addr := operandToAddress(operand)
		return cpu.Mem.ReadByte(addr)
	case ABX:
		addr := operandToAddress(operand)
		addr, cpu.pageCrossed = offsetAddress(addr, cpu.Reg.X)
		return cpu.Mem.ReadByte(addr)
	case ABY:
		addr := operandToAddress(operand)
		addr, cpu.pageCrossed = offsetAddress(addr, cpu.Reg.Y)
		return cpu.Mem.ReadByte(addr)
	case IDX:
		zpaddr := operandToAddress(operand)
		zpaddr = offsetZeroPage(zpaddr, cpu.Reg.X)
		addr := cpu.Mem.ReadAddress(zpaddr)
		return cpu.Mem.ReadByte(addr)
	case IDY:
		zpaddr := operandToAddress(operand)
		addr := cpu.Mem.ReadAddress(zpaddr)
		addr, cpu.pageCrossed = offsetAddress(addr, cpu.Reg.Y)
		return cpu.Mem.ReadByte(addr)
	case ACC:
		return cpu.Reg.A
	default:
		panic("Invalid addressing mode")
	}
}

// Load a 16-bit address value using the requested addressing mode
// and the 16-bit instruction operand.
func (cpu *Cpu) loadAddress(mode Mode, operand []byte) Address {
	switch mode {
	case ABS:
		return operandToAddress(operand)
	case IND:
		addr := operandToAddress(operand)
		return cpu.Mem.ReadAddress(addr)
	default:
		panic("Invalid addressing mode")
	}
}

// Store the value 'v' using the specified addressing mode and the
// variable-sized instruction operand.
func (cpu *Cpu) store(mode Mode, operand []byte, v byte) {
	switch mode {
	case ZPG:
		zpaddr := operandToAddress(operand)
		cpu.Mem.WriteByte(zpaddr, v)
	case ZPX:
		zpaddr := operandToAddress(operand)
		zpaddr = offsetZeroPage(zpaddr, cpu.Reg.X)
		cpu.Mem.WriteByte(zpaddr, v)
	case ZPY:
		zpaddr := operandToAddress(operand)
		zpaddr = offsetZeroPage(zpaddr, cpu.Reg.Y)
		cpu.Mem.WriteByte(zpaddr, v)
	case ABS:
		addr := operandToAddress(operand)
		cpu.Mem.WriteByte(addr, v)
	case ABX:
		addr := operandToAddress(operand)
		addr, cpu.pageCrossed = offsetAddress(addr, cpu.Reg.X)
		cpu.Mem.WriteByte(addr, v)
	case ABY:
		addr := operandToAddress(operand)
		addr, cpu.pageCrossed = offsetAddress(addr, cpu.Reg.Y)
		cpu.Mem.WriteByte(addr, v)
	case IDX:
		zpaddr := operandToAddress(operand)
		zpaddr = offsetZeroPage(zpaddr, cpu.Reg.X)
		addr := cpu.Mem.ReadAddress(zpaddr)
		cpu.Mem.WriteByte(addr, v)
	case IDY:
		zpaddr := operandToAddress(operand)
		addr := cpu.Mem.ReadAddress(zpaddr)
		addr, cpu.pageCrossed = offsetAddress(addr, cpu.Reg.Y)
		cpu.Mem.WriteByte(addr, v)
	case ACC:
		cpu.Reg.A = v
	default:
		panic("Invalid addressing mode")
	}
}

// Execute a branch using the instruction operand.
func (cpu *Cpu) branch(operand []byte) {
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
func (cpu *Cpu) push(v byte) {
	cpu.Mem.WriteByte(stackAddress(cpu.Reg.SP), v)
	cpu.Reg.SP--
}

// Pop a value from the stack and return it.
func (cpu *Cpu) pop() byte {
	cpu.Reg.SP++
	return cpu.Mem.ReadByte(stackAddress(cpu.Reg.SP))
}

// Update the Zero and Negative flags based on the value of 'v'.
func (cpu *Cpu) setNZ(v byte) {
	cpu.Reg.SetStatus(Zero, v == 0)
	cpu.Reg.SetStatus(Negative, (v&0x80) != 0)
}

// Handle an interrupt by storing the program counter and status
// flags on the stack. Then go to the subroutine at address 0xfffe.
func (cpu *Cpu) interrupt() {
	cpu.push(byte(cpu.Reg.PC >> 8))
	cpu.push(byte(cpu.Reg.PC & 0xff))
	cpu.push(byte(cpu.Reg.PS))
	cpu.Reg.SetStatus(InterruptDisable, true)
	cpu.Reg.PC = cpu.Mem.ReadAddress(Address(0xfffe))
}

// Add with carry
func (cpu *Cpu) opADC(inst *Instruction, operand []byte) {
	A := uint32(cpu.Reg.A)
	carry := uint32(cpu.Reg.GetStatus(Carry))
	orig := uint32(cpu.load(inst.Mode, operand))
	v := A + orig + carry
	cpu.Reg.SetStatus(Zero, (v&0xff) == 0)
	if cpu.Reg.IsStatusSet(Decimal) {
		if (A&0x0f)+(orig&0x0f)+carry > 9 {
			v += 6
		}
		cpu.Reg.SetStatus(Negative, (v&0x80) != 0)
		cpu.Reg.SetStatus(Overflow, ((A^orig)&0x80) == 0 && ((A^v)&0x80) != 0)
		if v > 0x99 {
			v += 96
		}
		cpu.Reg.SetStatus(Carry, v > 0x99)
	} else {
		cpu.Reg.SetStatus(Negative, (v&0x80) != 0)
		cpu.Reg.SetStatus(Overflow, ((A^orig)&0x80) == 0 && ((A^v)&0x80) != 0)
		cpu.Reg.SetStatus(Carry, v > 0xff)
	}
	cpu.Reg.A = byte(v & 0xff)
}

// Boolean AND
func (cpu *Cpu) opAND(inst *Instruction, operand []byte) {
	cpu.Reg.A &= cpu.load(inst.Mode, operand)
	cpu.setNZ(cpu.Reg.A)
}

// Arithmetic Shift Left
func (cpu *Cpu) opASL(inst *Instruction, operand []byte) {
	v := cpu.load(inst.Mode, operand)
	cpu.Reg.SetStatus(Carry, (v&0x80) == 0x80)
	v = v << 1
	cpu.store(inst.Mode, operand, v)
	cpu.setNZ(v)
}

// Branch if Carry Clear
func (cpu *Cpu) opBCC(inst *Instruction, operand []byte) {
	if !cpu.Reg.IsStatusSet(Carry) {
		cpu.branch(operand)
	}
}

// Branch if Carry Set
func (cpu *Cpu) opBCS(inst *Instruction, operand []byte) {
	if cpu.Reg.IsStatusSet(Carry) {
		cpu.branch(operand)
	}
}

// Branch if EQual (to zero)
func (cpu *Cpu) opBEQ(inst *Instruction, operand []byte) {
	if cpu.Reg.IsStatusSet(Zero) {
		cpu.branch(operand)
	}
}

// Bit Test
func (cpu *Cpu) opBIT(inst *Instruction, operand []byte) {
	v := cpu.load(inst.Mode, operand)
	cpu.setNZ(v)
	cpu.Reg.SetStatus(Overflow, (v&0x40) != 0)
}

// Branch if MInus (negative)
func (cpu *Cpu) opBMI(inst *Instruction, operand []byte) {
	if cpu.Reg.IsStatusSet(Negative) {
		cpu.branch(operand)
	}
}

// Branch if Not Equal (not zero)
func (cpu *Cpu) opBNE(inst *Instruction, operand []byte) {
	if !cpu.Reg.IsStatusSet(Zero) {
		cpu.branch(operand)
	}
}

// Branch if PLus (positive)
func (cpu *Cpu) opBPL(inst *Instruction, operand []byte) {
	if !cpu.Reg.IsStatusSet(Negative) {
		cpu.branch(operand)
	}
}

// Break
func (cpu *Cpu) opBRK(inst *Instruction, operand []byte) {
	cpu.Reg.PC++
	cpu.interrupt()
	cpu.Reg.SetStatus(Break, true)
}

// Branch if oVerflow Clear
func (cpu *Cpu) opBVC(inst *Instruction, operand []byte) {
	if !cpu.Reg.IsStatusSet(Overflow) {
		cpu.branch(operand)
	}
}

// Branch if oVerflow Set
func (cpu *Cpu) opBVS(inst *Instruction, operand []byte) {
	if cpu.Reg.IsStatusSet(Overflow) {
		cpu.branch(operand)
	}
}

// Clear Carry flag
func (cpu *Cpu) opCLC(inst *Instruction, operand []byte) {
	cpu.Reg.SetStatus(Carry, false)
}

// Clear Decimal flag
func (cpu *Cpu) opCLD(inst *Instruction, operand []byte) {
	cpu.Reg.SetStatus(Decimal, false)
}

// Clear InterruptDisable flag
func (cpu *Cpu) opCLI(inst *Instruction, operand []byte) {
	cpu.Reg.SetStatus(InterruptDisable, false)
}

// Clear oVerflow flag
func (cpu *Cpu) opCLV(inst *Instruction, operand []byte) {
	cpu.Reg.SetStatus(Overflow, false)
}

// Compare to accumulator
func (cpu *Cpu) opCMP(inst *Instruction, operand []byte) {
	v := cpu.load(inst.Mode, operand)
	cpu.Reg.SetStatus(Carry, cpu.Reg.A >= v)
	cpu.setNZ(cpu.Reg.A - v)
}

// Compare to X register
func (cpu *Cpu) opCPX(inst *Instruction, operand []byte) {
	v := cpu.load(inst.Mode, operand)
	cpu.Reg.SetStatus(Carry, cpu.Reg.X >= v)
	cpu.setNZ(cpu.Reg.X - v)
}

// Compare to Y register
func (cpu *Cpu) opCPY(inst *Instruction, operand []byte) {
	v := cpu.load(inst.Mode, operand)
	cpu.Reg.SetStatus(Carry, cpu.Reg.Y >= v)
	cpu.setNZ(cpu.Reg.Y - v)
}

// Decrement memory value
func (cpu *Cpu) opDEC(inst *Instruction, operand []byte) {
	v := cpu.load(inst.Mode, operand) - 1
	cpu.setNZ(v)
	cpu.store(inst.Mode, operand, v)
}

// Decrement X register
func (cpu *Cpu) opDEX(inst *Instruction, operand []byte) {
	cpu.Reg.X--
	cpu.setNZ(cpu.Reg.X)
}

// Decrement Y register
func (cpu *Cpu) opDEY(inst *Instruction, operand []byte) {
	cpu.Reg.Y--
	cpu.setNZ(cpu.Reg.Y)
}

// Boolean XOR
func (cpu *Cpu) opEOR(inst *Instruction, operand []byte) {
	cpu.Reg.A ^= cpu.load(inst.Mode, operand)
	cpu.setNZ(cpu.Reg.A)
}

// Increment memory value
func (cpu *Cpu) opINC(inst *Instruction, operand []byte) {
	v := cpu.load(inst.Mode, operand) + 1
	cpu.setNZ(v)
	cpu.store(inst.Mode, operand, v)
}

// Increment X register
func (cpu *Cpu) opINX(inst *Instruction, operand []byte) {
	cpu.Reg.X++
	cpu.setNZ(cpu.Reg.X)
}

// Increment Y register
func (cpu *Cpu) opINY(inst *Instruction, operand []byte) {
	cpu.Reg.Y++
	cpu.setNZ(cpu.Reg.Y)
}

// Jump to memory address
func (cpu *Cpu) opJMP(inst *Instruction, operand []byte) {
	cpu.Reg.PC = cpu.loadAddress(inst.Mode, operand)
}

// Jump to subroutine
func (cpu *Cpu) opJSR(inst *Instruction, operand []byte) {
	addr := cpu.loadAddress(inst.Mode, operand)
	cpu.Reg.PC--
	cpu.push(byte(cpu.Reg.PC >> 8))
	cpu.push(byte(cpu.Reg.PC & 0xff))
	cpu.Reg.PC = addr
}

// load Accumulator
func (cpu *Cpu) opLDA(inst *Instruction, operand []byte) {
	cpu.Reg.A = cpu.load(inst.Mode, operand)
	cpu.setNZ(cpu.Reg.A)
}

// load the X register
func (cpu *Cpu) opLDX(inst *Instruction, operand []byte) {
	cpu.Reg.X = cpu.load(inst.Mode, operand)
	cpu.setNZ(cpu.Reg.X)
}

// load the Y register
func (cpu *Cpu) opLDY(inst *Instruction, operand []byte) {
	cpu.Reg.Y = cpu.load(inst.Mode, operand)
	cpu.setNZ(cpu.Reg.Y)
}

// Logical Shift Right
func (cpu *Cpu) opLSR(inst *Instruction, operand []byte) {
	v := cpu.load(inst.Mode, operand)
	cpu.Reg.SetStatus(Carry, (v&1) == 1)
	v = v >> 1
	cpu.store(inst.Mode, operand, v)
	cpu.setNZ(v)
}

// No-operation
func (cpu *Cpu) opNOP(inst *Instruction, operand []byte) {
	// Do nothing
}

// Boolean OR
func (cpu *Cpu) opORA(inst *Instruction, operand []byte) {
	cpu.Reg.A |= cpu.load(inst.Mode, operand)
	cpu.setNZ(cpu.Reg.A)
}

// push Accumulator
func (cpu *Cpu) opPHA(inst *Instruction, operand []byte) {
	cpu.push(cpu.Reg.A)
}

// push Processor flags
func (cpu *Cpu) opPHP(inst *Instruction, operand []byte) {
	cpu.push(byte(cpu.Reg.PS))
}

// Pull (pop) Accumulator
func (cpu *Cpu) opPLA(inst *Instruction, operand []byte) {
	cpu.Reg.A = cpu.pop()
	cpu.setNZ(cpu.Reg.A)
}

// Pull (pop) Processor flags
func (cpu *Cpu) opPLP(inst *Instruction, operand []byte) {
	cpu.Reg.PS &= Break
	cpu.Reg.PS |= Status(cpu.pop() & ^byte(Break))
}

// Rotate left
func (cpu *Cpu) opROL(inst *Instruction, operand []byte) {
	tmp := cpu.load(inst.Mode, operand)
	v := (tmp << 1) | cpu.Reg.GetStatus(Carry)
	cpu.Reg.SetStatus(Carry, (tmp&0x80) != 0)
	cpu.store(inst.Mode, operand, v)
	cpu.setNZ(v)
}

// Rotate right
func (cpu *Cpu) opROR(inst *Instruction, operand []byte) {
	tmp := cpu.load(inst.Mode, operand)
	v := (tmp >> 1) | (cpu.Reg.GetStatus(Carry) << 7)
	cpu.Reg.SetStatus(Carry, (tmp&1) != 0)
	cpu.store(inst.Mode, operand, v)
	cpu.setNZ(v)
}

// Return from interrupt
func (cpu *Cpu) opRTI(inst *Instruction, operand []byte) {
	cpu.Reg.PS &= Break
	cpu.Reg.PS |= Status(cpu.pop() & ^byte(Break))
	cpu.Reg.PC = Address(cpu.pop()) | (Address(cpu.pop()) << 8)
}

// Return from Subroutine
func (cpu *Cpu) opRTS(inst *Instruction, operand []byte) {
	addr := Address(cpu.pop()) | (Address(cpu.pop()) << 8)
	cpu.Reg.PC = addr + Address(1)
}

// Subtract with Carry
func (cpu *Cpu) opSBC(inst *Instruction, operand []byte) {
	orig := uint32(cpu.load(inst.Mode, operand))
	A := uint32(cpu.Reg.A)
	carry := uint32(cpu.Reg.GetStatus(Carry) ^ 1)
	v := A - orig - carry
	cpu.setNZ(byte(v & 0xff))
	cpu.Reg.SetStatus(Overflow, ((A^v)&0x80) != 0 && ((A^orig)&0x80) != 0)
	if cpu.Reg.IsStatusSet(Decimal) {
		if (A&0xf)-carry < (orig & 0xf) {
			v -= 6
		}
		if v > 0x99 {
			v -= 0x60
		}
	}
	cpu.Reg.SetStatus(Carry, v < 0x100)
	cpu.Reg.A = byte(v & 0xff)
}

// Set Carry flag
func (cpu *Cpu) opSEC(inst *Instruction, operand []byte) {
	cpu.Reg.SetStatus(Carry, true)
}

// Set Decimal flag
func (cpu *Cpu) opSED(inst *Instruction, operand []byte) {
	cpu.Reg.SetStatus(Decimal, true)
}

// Set InterruptDisable flag
func (cpu *Cpu) opSEI(inst *Instruction, operand []byte) {
	cpu.Reg.SetStatus(InterruptDisable, true)
}

// store Accumulator
func (cpu *Cpu) opSTA(inst *Instruction, operand []byte) {
	cpu.store(inst.Mode, operand, cpu.Reg.A)
}

// store X register
func (cpu *Cpu) opSTX(inst *Instruction, operand []byte) {
	cpu.store(inst.Mode, operand, cpu.Reg.X)
}

// store Y register
func (cpu *Cpu) opSTY(inst *Instruction, operand []byte) {
	cpu.store(inst.Mode, operand, cpu.Reg.Y)
}

// Transfer Accumulator to X register
func (cpu *Cpu) opTAX(inst *Instruction, operand []byte) {
	cpu.Reg.X = cpu.Reg.A
	cpu.setNZ(cpu.Reg.X)
}

// Transfer Accumulator to Y register
func (cpu *Cpu) opTAY(inst *Instruction, operand []byte) {
	cpu.Reg.Y = cpu.Reg.A
	cpu.setNZ(cpu.Reg.Y)
}

// Transfer Stack pointer to X register
func (cpu *Cpu) opTSX(inst *Instruction, operand []byte) {
	cpu.Reg.X = cpu.Reg.SP
	cpu.setNZ(cpu.Reg.X)
}

// Transfer X register to Accumulator
func (cpu *Cpu) opTXA(inst *Instruction, operand []byte) {
	cpu.Reg.A = cpu.Reg.X
	cpu.setNZ(cpu.Reg.A)
}

// Transfer X register to the Stack pointer
func (cpu *Cpu) opTXS(inst *Instruction, operand []byte) {
	cpu.Reg.SP = cpu.Reg.X
}

// Transfer Y register to the Accumulator
func (cpu *Cpu) opTYA(inst *Instruction, operand []byte) {
	cpu.Reg.A = cpu.Reg.Y
	cpu.setNZ(cpu.Reg.A)
}
