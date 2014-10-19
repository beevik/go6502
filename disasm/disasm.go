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
	"#$%s",    // REL
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

var hex string = "0123456789ABCDEF"

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
func Disassemble(m *go6502.Memory, addr go6502.Address) (line string, next go6502.Address) {
	opcode := m.ReadByte(addr)
	idata := &go6502.Instructions[opcode]
	operand := m.ReadBytes(addr+1, int(idata.Length)-1)
	fmtString := "%s " + modeFormat[idata.Mode]
	line = fmt.Sprintf(fmtString, idata.Name, hexString(operand))
	next = addr + go6502.Address(idata.Length)
	return
}
