// Copyright 2014-2018 Brett Vickers. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package disasm implements a 6502 instruction set
// disassembler.
package disasm

import (
	"fmt"

	"github.com/beevik/go6502/cpu"
)

// Disassembler formatting for addressing modes
var modeFormat = []string{
	"#$%s",    // IMM
	"%s",      // IMP
	"$%s",     // REL
	"$%s",     // ZPG
	"$%s,X",   // ZPX
	"$%s,Y",   // ZPY
	"$%s",     // ABS
	"$%s,X",   // ABX
	"$%s,Y",   // ABY
	"($%s)",   // IND
	"($%s,X)", // IDX
	"($%s),Y", // IDY
	"%s",      // ACC
}

var hex = "0123456789ABCDEF"

// Return a hexadecimal string representation of the byte slice.
func hexString(b []byte) string {
	hexlen := len(b) * 2
	hexbuf := make([]byte, hexlen)
	j := hexlen - 1
	for _, n := range b {
		hexbuf[j] = hex[n&0xf]
		hexbuf[j-1] = hex[n>>4]
		j -= 2
	}
	return string(hexbuf)
}

// Disassemble the machine code in memory 'm' at address 'addr'. Return a
// 'line' string representing the disassembled instruction and a 'next'
// address that starts the following line of machine code.
func Disassemble(m cpu.Memory, addr uint16) (line string, next uint16) {
	opcode := m.LoadByte(addr)
	set := cpu.GetInstructionSet(cpu.CMOS)
	inst := set.Lookup(opcode)

	var buf [2]byte
	operand := buf[:inst.Length-1]
	m.LoadBytes(addr+1, operand)

	if inst.Mode == cpu.REL {
		// Convert relative offset to absolute address.
		operand = buf[:]
		braddr := int(addr) + int(inst.Length) + byteToInt(operand[0])
		operand[0] = byte(braddr)
		operand[1] = byte(braddr >> 8)
	}
	format := "%s   " + modeFormat[inst.Mode]
	line = fmt.Sprintf(format, inst.Name, hexString(operand))
	next = addr + uint16(inst.Length)
	return line, next
}

// GetRegisterString returns a string describing the contents of the 6502
// registers.
func GetRegisterString(r *cpu.Registers) string {
	return fmt.Sprintf("A=%02X X=%02X Y=%02X PS=[%s] SP=%02X PC=%04X",
		r.A, r.X, r.Y, getStatusBits(r), r.SP, r.PC)
}

// GetCompactRegisterString returns a compact string describing the contents
// of the 6502 registers. It excludes the program counter and stack pointer.
func GetCompactRegisterString(r *cpu.Registers) string {
	return fmt.Sprintf("A=%02X X=%02X Y=%02X PS=[%s]", r.A, r.X, r.Y, getStatusBits(r))
}

func getStatusBits(r *cpu.Registers) string {
	v := func(bit bool, ch byte) byte {
		if bit {
			return ch
		}
		return '-'
	}
	b := []byte{
		v(r.Sign, 'N'),
		v(r.Zero, 'Z'),
		v(r.Carry, 'C'),
		v(r.InterruptDisable, 'I'),
		v(r.Decimal, 'D'),
		v(r.Overflow, 'V'),
	}
	return string(b)
}

func byteToInt(b byte) int {
	if b >= 0x80 {
		return int(b) - 256
	}
	return int(b)
}
