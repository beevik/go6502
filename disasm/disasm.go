// Copyright 2014 Brett Vickers. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package disasm implements a 6502 instruction set
// disassembler.
package disasm

import (
	"fmt"

	"github.com/beevik/go6502"
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
func Disassemble(m go6502.Memory, addr uint16) (line string, next uint16, err error) {
	opcode, err := m.LoadByte(addr)
	if err != nil {
		return "", 0, err
	}
	set := go6502.GetInstructionSet(go6502.CMOS)
	inst := set.Lookup(opcode)

	var buf [2]byte
	operand := buf[:inst.Length-1]
	err = m.LoadBytes(addr+1, operand)
	if err != nil {
		return "", 0, err
	}

	if inst.Mode == go6502.REL {
		// Convert relative offset to absolute address.
		operand = buf[:2]
		braddr := int(addr) + int(inst.Length+operand[0])
		if operand[0] > 0x7f {
			braddr -= 256
		}
		operand[0] = byte(braddr)
		operand[1] = byte(braddr >> 8)
	}
	format := "%s " + modeFormat[inst.Mode]
	line = fmt.Sprintf(format, inst.Name, hexString(operand))
	next = addr + uint16(inst.Length)
	return line, next, err
}
