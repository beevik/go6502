// Copyright 2014-2018 Brett Vickers. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cpu

import "strings"

// An opsym is an internal symbol used to associate an opcode's data
// with its instructions.
type opsym byte

const (
	symADC opsym = iota
	symAND
	symASL
	symBCC
	symBCS
	symBEQ
	symBIT
	symBMI
	symBNE
	symBPL
	symBRA
	symBRK
	symBVC
	symBVS
	symCLC
	symCLD
	symCLI
	symCLV
	symCMP
	symCPX
	symCPY
	symDEC
	symDEX
	symDEY
	symEOR
	symINC
	symINX
	symINY
	symJMP
	symJSR
	symLDA
	symLDX
	symLDY
	symLSR
	symNOP
	symORA
	symPHA
	symPHP
	symPHX
	symPHY
	symPLA
	symPLP
	symPLX
	symPLY
	symROL
	symROR
	symRTI
	symRTS
	symSBC
	symSEC
	symSED
	symSEI
	symSTA
	symSTZ
	symSTX
	symSTY
	symTAX
	symTAY
	symTRB
	symTSB
	symTSX
	symTXA
	symTXS
	symTYA
)

type instfunc func(c *CPU, inst *Instruction, operand []byte)

// Emulator implementation for each opcode
type opcodeImpl struct {
	sym  opsym
	name string
	fn   [2]instfunc // NMOS=0, CMOS=1
}

var impl = []opcodeImpl{
	{symADC, "ADC", [2]instfunc{(*CPU).adcn, (*CPU).adcc}},
	{symAND, "AND", [2]instfunc{(*CPU).and, (*CPU).and}},
	{symASL, "ASL", [2]instfunc{(*CPU).asl, (*CPU).asl}},
	{symBCC, "BCC", [2]instfunc{(*CPU).bcc, (*CPU).bcc}},
	{symBCS, "BCS", [2]instfunc{(*CPU).bcs, (*CPU).bcs}},
	{symBEQ, "BEQ", [2]instfunc{(*CPU).beq, (*CPU).beq}},
	{symBIT, "BIT", [2]instfunc{(*CPU).bit, (*CPU).bit}},
	{symBMI, "BMI", [2]instfunc{(*CPU).bmi, (*CPU).bmi}},
	{symBNE, "BNE", [2]instfunc{(*CPU).bne, (*CPU).bne}},
	{symBPL, "BPL", [2]instfunc{(*CPU).bpl, (*CPU).bpl}},
	{symBRA, "BRA", [2]instfunc{nil, (*CPU).bra}},
	{symBRK, "BRK", [2]instfunc{(*CPU).brk, (*CPU).brk}},
	{symBVC, "BVC", [2]instfunc{(*CPU).bvc, (*CPU).bvc}},
	{symBVS, "BVS", [2]instfunc{(*CPU).bvs, (*CPU).bvs}},
	{symCLC, "CLC", [2]instfunc{(*CPU).clc, (*CPU).clc}},
	{symCLD, "CLD", [2]instfunc{(*CPU).cld, (*CPU).cld}},
	{symCLI, "CLI", [2]instfunc{(*CPU).cli, (*CPU).cli}},
	{symCLV, "CLV", [2]instfunc{(*CPU).clv, (*CPU).clv}},
	{symCMP, "CMP", [2]instfunc{(*CPU).cmp, (*CPU).cmp}},
	{symCPX, "CPX", [2]instfunc{(*CPU).cpx, (*CPU).cpx}},
	{symCPY, "CPY", [2]instfunc{(*CPU).cpy, (*CPU).cpy}},
	{symDEC, "DEC", [2]instfunc{(*CPU).dec, (*CPU).dec}},
	{symDEX, "DEX", [2]instfunc{(*CPU).dex, (*CPU).dex}},
	{symDEY, "DEY", [2]instfunc{(*CPU).dey, (*CPU).dey}},
	{symEOR, "EOR", [2]instfunc{(*CPU).eor, (*CPU).eor}},
	{symINC, "INC", [2]instfunc{(*CPU).inc, (*CPU).inc}},
	{symINX, "INX", [2]instfunc{(*CPU).inx, (*CPU).inx}},
	{symINY, "INY", [2]instfunc{(*CPU).iny, (*CPU).iny}},
	{symJMP, "JMP", [2]instfunc{(*CPU).jmpn, (*CPU).jmpc}},
	{symJSR, "JSR", [2]instfunc{(*CPU).jsr, (*CPU).jsr}},
	{symLDA, "LDA", [2]instfunc{(*CPU).lda, (*CPU).lda}},
	{symLDX, "LDX", [2]instfunc{(*CPU).ldx, (*CPU).ldx}},
	{symLDY, "LDY", [2]instfunc{(*CPU).ldy, (*CPU).ldy}},
	{symLSR, "LSR", [2]instfunc{(*CPU).lsr, (*CPU).lsr}},
	{symNOP, "NOP", [2]instfunc{(*CPU).nop, (*CPU).nop}},
	{symORA, "ORA", [2]instfunc{(*CPU).ora, (*CPU).ora}},
	{symPHA, "PHA", [2]instfunc{(*CPU).pha, (*CPU).pha}},
	{symPHP, "PHP", [2]instfunc{(*CPU).php, (*CPU).php}},
	{symPHX, "PHX", [2]instfunc{nil, (*CPU).phx}},
	{symPHY, "PHY", [2]instfunc{nil, (*CPU).phy}},
	{symPLA, "PLA", [2]instfunc{(*CPU).pla, (*CPU).pla}},
	{symPLP, "PLP", [2]instfunc{(*CPU).plp, (*CPU).plp}},
	{symPLX, "PLX", [2]instfunc{nil, (*CPU).plx}},
	{symPLY, "PLY", [2]instfunc{nil, (*CPU).ply}},
	{symROL, "ROL", [2]instfunc{(*CPU).rol, (*CPU).rol}},
	{symROR, "ROR", [2]instfunc{(*CPU).ror, (*CPU).ror}},
	{symRTI, "RTI", [2]instfunc{(*CPU).rti, (*CPU).rti}},
	{symRTS, "RTS", [2]instfunc{(*CPU).rts, (*CPU).rts}},
	{symSBC, "SBC", [2]instfunc{(*CPU).sbcn, (*CPU).sbcc}},
	{symSEC, "SEC", [2]instfunc{(*CPU).sec, (*CPU).sec}},
	{symSED, "SED", [2]instfunc{(*CPU).sed, (*CPU).sed}},
	{symSEI, "SEI", [2]instfunc{(*CPU).sei, (*CPU).sei}},
	{symSTA, "STA", [2]instfunc{(*CPU).sta, (*CPU).sta}},
	{symSTX, "STX", [2]instfunc{(*CPU).stx, (*CPU).stx}},
	{symSTY, "STY", [2]instfunc{(*CPU).sty, (*CPU).sty}},
	{symSTZ, "STZ", [2]instfunc{nil, (*CPU).stz}},
	{symTAX, "TAX", [2]instfunc{(*CPU).tax, (*CPU).tax}},
	{symTAY, "TAY", [2]instfunc{(*CPU).tay, (*CPU).tay}},
	{symTRB, "TRB", [2]instfunc{nil, (*CPU).trb}},
	{symTSB, "TSB", [2]instfunc{nil, (*CPU).tsb}},
	{symTSX, "TSX", [2]instfunc{(*CPU).tsx, (*CPU).tsx}},
	{symTXA, "TXA", [2]instfunc{(*CPU).txa, (*CPU).txa}},
	{symTXS, "TXS", [2]instfunc{(*CPU).txs, (*CPU).txs}},
	{symTYA, "TYA", [2]instfunc{(*CPU).tya, (*CPU).tya}},
}

// Mode describes a memory addressing mode.
type Mode byte

// All possible memory addressing modes
const (
	IMM Mode = iota // Immediate
	IMP             // Implied (no operand)
	REL             // Relative
	ZPG             // Zero Page
	ZPX             // Zero Page,X
	ZPY             // Zero Page,Y
	ABS             // Absolute
	ABX             // Absolute,X
	ABY             // Absolute,Y
	IND             // (Indirect)
	IDX             // (Indirect,X)
	IDY             // (Indirect),Y
	ACC             // Accumulator (no operand)
)

// Opcode data for an (opcode, mode) pair
type opcodeData struct {
	sym      opsym // internal opcode symbol
	mode     Mode  // addressing mode
	opcode   byte  // opcode hex value
	length   byte  // length of opcode + operand in bytes
	cycles   byte  // number of CPU cycles to execute command
	bpcycles byte  // additional CPU cycles if command crosses page boundary
	cmos     bool  // whether the opcode/mode pair is valid only on 65C02
}

// All valid (opcode, mode) pairs
var data = []opcodeData{
	{symLDA, IMM, 0xa9, 2, 2, 0, false},
	{symLDA, ZPG, 0xa5, 2, 3, 0, false},
	{symLDA, ZPX, 0xb5, 2, 4, 0, false},
	{symLDA, ABS, 0xad, 3, 4, 0, false},
	{symLDA, ABX, 0xbd, 3, 4, 1, false},
	{symLDA, ABY, 0xb9, 3, 4, 1, false},
	{symLDA, IDX, 0xa1, 2, 6, 0, false},
	{symLDA, IDY, 0xb1, 2, 5, 1, false},
	{symLDA, IND, 0xb2, 2, 5, 0, true},

	{symLDX, IMM, 0xa2, 2, 2, 0, false},
	{symLDX, ZPG, 0xa6, 2, 3, 0, false},
	{symLDX, ZPY, 0xb6, 2, 4, 0, false},
	{symLDX, ABS, 0xae, 3, 4, 0, false},
	{symLDX, ABY, 0xbe, 3, 4, 1, false},

	{symLDY, IMM, 0xa0, 2, 2, 0, false},
	{symLDY, ZPG, 0xa4, 2, 3, 0, false},
	{symLDY, ZPX, 0xb4, 2, 4, 0, false},
	{symLDY, ABS, 0xac, 3, 4, 0, false},
	{symLDY, ABX, 0xbc, 3, 4, 1, false},

	{symSTA, ZPG, 0x85, 2, 3, 0, false},
	{symSTA, ZPX, 0x95, 2, 4, 0, false},
	{symSTA, ABS, 0x8d, 3, 4, 0, false},
	{symSTA, ABX, 0x9d, 3, 5, 0, false},
	{symSTA, ABY, 0x99, 3, 5, 0, false},
	{symSTA, IDX, 0x81, 2, 6, 0, false},
	{symSTA, IDY, 0x91, 2, 6, 0, false},
	{symSTA, IND, 0x92, 2, 5, 0, true},

	{symSTX, ZPG, 0x86, 2, 3, 0, false},
	{symSTX, ZPY, 0x96, 2, 4, 0, false},
	{symSTX, ABS, 0x8e, 3, 4, 0, false},

	{symSTY, ZPG, 0x84, 2, 3, 0, false},
	{symSTY, ZPX, 0x94, 2, 4, 0, false},
	{symSTY, ABS, 0x8c, 3, 4, 0, false},

	{symSTZ, ZPG, 0x64, 2, 3, 0, true},
	{symSTZ, ZPX, 0x74, 2, 4, 0, true},
	{symSTZ, ABS, 0x9c, 3, 4, 0, true},
	{symSTZ, ABX, 0x9e, 3, 5, 0, true},

	{symADC, IMM, 0x69, 2, 2, 0, false},
	{symADC, ZPG, 0x65, 2, 3, 0, false},
	{symADC, ZPX, 0x75, 2, 4, 0, false},
	{symADC, ABS, 0x6d, 3, 4, 0, false},
	{symADC, ABX, 0x7d, 3, 4, 1, false},
	{symADC, ABY, 0x79, 3, 4, 1, false},
	{symADC, IDX, 0x61, 2, 6, 0, false},
	{symADC, IDY, 0x71, 2, 5, 1, false},
	{symADC, IND, 0x72, 2, 5, 1, true},

	{symSBC, IMM, 0xe9, 2, 2, 0, false},
	{symSBC, ZPG, 0xe5, 2, 3, 0, false},
	{symSBC, ZPX, 0xf5, 2, 4, 0, false},
	{symSBC, ABS, 0xed, 3, 4, 0, false},
	{symSBC, ABX, 0xfd, 3, 4, 1, false},
	{symSBC, ABY, 0xf9, 3, 4, 1, false},
	{symSBC, IDX, 0xe1, 2, 6, 0, false},
	{symSBC, IDY, 0xf1, 2, 5, 1, false},
	{symSBC, IND, 0xf2, 2, 5, 1, true},

	{symCMP, IMM, 0xc9, 2, 2, 0, false},
	{symCMP, ZPG, 0xc5, 2, 3, 0, false},
	{symCMP, ZPX, 0xd5, 2, 4, 0, false},
	{symCMP, ABS, 0xcd, 3, 4, 0, false},
	{symCMP, ABX, 0xdd, 3, 4, 1, false},
	{symCMP, ABY, 0xd9, 3, 4, 1, false},
	{symCMP, IDX, 0xc1, 2, 6, 0, false},
	{symCMP, IDY, 0xd1, 2, 5, 1, false},
	{symCMP, IND, 0xd2, 2, 5, 0, true},

	{symCPX, IMM, 0xe0, 2, 2, 0, false},
	{symCPX, ZPG, 0xe4, 2, 3, 0, false},
	{symCPX, ABS, 0xec, 3, 4, 0, false},

	{symCPY, IMM, 0xc0, 2, 2, 0, false},
	{symCPY, ZPG, 0xc4, 2, 3, 0, false},
	{symCPY, ABS, 0xcc, 3, 4, 0, false},

	{symBIT, IMM, 0x89, 2, 2, 0, true},
	{symBIT, ZPG, 0x24, 2, 3, 0, false},
	{symBIT, ZPX, 0x34, 2, 4, 0, true},
	{symBIT, ABS, 0x2c, 3, 4, 0, false},
	{symBIT, ABX, 0x3c, 3, 4, 1, true},

	{symCLC, IMP, 0x18, 1, 2, 0, false},
	{symSEC, IMP, 0x38, 1, 2, 0, false},
	{symCLI, IMP, 0x58, 1, 2, 0, false},
	{symSEI, IMP, 0x78, 1, 2, 0, false},
	{symCLD, IMP, 0xd8, 1, 2, 0, false},
	{symSED, IMP, 0xf8, 1, 2, 0, false},
	{symCLV, IMP, 0xb8, 1, 2, 0, false},

	{symBCC, REL, 0x90, 2, 2, 1, false},
	{symBCS, REL, 0xb0, 2, 2, 1, false},
	{symBEQ, REL, 0xf0, 2, 2, 1, false},
	{symBNE, REL, 0xd0, 2, 2, 1, false},
	{symBMI, REL, 0x30, 2, 2, 1, false},
	{symBPL, REL, 0x10, 2, 2, 1, false},
	{symBVC, REL, 0x50, 2, 2, 1, false},
	{symBVS, REL, 0x70, 2, 2, 1, false},
	{symBRA, REL, 0x80, 2, 2, 1, true},

	{symBRK, IMP, 0x00, 1, 7, 0, false},

	{symAND, IMM, 0x29, 2, 2, 0, false},
	{symAND, ZPG, 0x25, 2, 3, 0, false},
	{symAND, ZPX, 0x35, 2, 4, 0, false},
	{symAND, ABS, 0x2d, 3, 4, 0, false},
	{symAND, ABX, 0x3d, 3, 4, 1, false},
	{symAND, ABY, 0x39, 3, 4, 1, false},
	{symAND, IDX, 0x21, 2, 6, 0, false},
	{symAND, IDY, 0x31, 2, 5, 1, false},
	{symAND, IND, 0x32, 2, 5, 0, true},

	{symORA, IMM, 0x09, 2, 2, 0, false},
	{symORA, ZPG, 0x05, 2, 3, 0, false},
	{symORA, ZPX, 0x15, 2, 4, 0, false},
	{symORA, ABS, 0x0d, 3, 4, 0, false},
	{symORA, ABX, 0x1d, 3, 4, 1, false},
	{symORA, ABY, 0x19, 3, 4, 1, false},
	{symORA, IDX, 0x01, 2, 6, 0, false},
	{symORA, IDY, 0x11, 2, 5, 1, false},
	{symORA, IND, 0x12, 2, 5, 0, true},

	{symEOR, IMM, 0x49, 2, 2, 0, false},
	{symEOR, ZPG, 0x45, 2, 3, 0, false},
	{symEOR, ZPX, 0x55, 2, 4, 0, false},
	{symEOR, ABS, 0x4d, 3, 4, 0, false},
	{symEOR, ABX, 0x5d, 3, 4, 1, false},
	{symEOR, ABY, 0x59, 3, 4, 1, false},
	{symEOR, IDX, 0x41, 2, 6, 0, false},
	{symEOR, IDY, 0x51, 2, 5, 1, false},
	{symEOR, IND, 0x52, 2, 5, 0, true},

	{symINC, ZPG, 0xe6, 2, 5, 0, false},
	{symINC, ZPX, 0xf6, 2, 6, 0, false},
	{symINC, ABS, 0xee, 3, 6, 0, false},
	{symINC, ABX, 0xfe, 3, 7, 0, false},
	{symINC, ACC, 0x1a, 1, 2, 0, true},

	{symDEC, ZPG, 0xc6, 2, 5, 0, false},
	{symDEC, ZPX, 0xd6, 2, 6, 0, false},
	{symDEC, ABS, 0xce, 3, 6, 0, false},
	{symDEC, ABX, 0xde, 3, 7, 0, false},
	{symDEC, ACC, 0x3a, 1, 2, 0, true},

	{symINX, IMP, 0xe8, 1, 2, 0, false},
	{symINY, IMP, 0xc8, 1, 2, 0, false},

	{symDEX, IMP, 0xca, 1, 2, 0, false},
	{symDEY, IMP, 0x88, 1, 2, 0, false},

	{symJMP, ABS, 0x4c, 3, 3, 0, false},
	{symJMP, ABX, 0x7c, 3, 6, 0, true},
	{symJMP, IND, 0x6c, 3, 5, 0, false},

	{symJSR, ABS, 0x20, 3, 6, 0, false},
	{symRTS, IMP, 0x60, 1, 6, 0, false},

	{symRTI, IMP, 0x40, 1, 6, 0, false},

	{symNOP, IMP, 0xea, 1, 2, 0, false},

	{symTAX, IMP, 0xaa, 1, 2, 0, false},
	{symTXA, IMP, 0x8a, 1, 2, 0, false},
	{symTAY, IMP, 0xa8, 1, 2, 0, false},
	{symTYA, IMP, 0x98, 1, 2, 0, false},
	{symTXS, IMP, 0x9a, 1, 2, 0, false},
	{symTSX, IMP, 0xba, 1, 2, 0, false},

	{symTRB, ZPG, 0x14, 2, 5, 0, true},
	{symTRB, ABS, 0x1c, 3, 6, 0, true},
	{symTSB, ZPG, 0x04, 2, 5, 0, true},
	{symTSB, ABS, 0x0c, 3, 6, 0, true},

	{symPHA, IMP, 0x48, 1, 3, 0, false},
	{symPLA, IMP, 0x68, 1, 4, 0, false},
	{symPHP, IMP, 0x08, 1, 3, 0, false},
	{symPLP, IMP, 0x28, 1, 4, 0, false},
	{symPHX, IMP, 0xda, 1, 3, 0, true},
	{symPLX, IMP, 0xfa, 1, 4, 0, true},
	{symPHY, IMP, 0x5a, 1, 3, 0, true},
	{symPLY, IMP, 0x7a, 1, 4, 0, true},

	{symASL, ACC, 0x0a, 1, 2, 0, false},
	{symASL, ZPG, 0x06, 2, 5, 0, false},
	{symASL, ZPX, 0x16, 2, 6, 0, false},
	{symASL, ABS, 0x0e, 3, 6, 0, false},
	{symASL, ABX, 0x1e, 3, 7, 0, false},

	{symLSR, ACC, 0x4a, 1, 2, 0, false},
	{symLSR, ZPG, 0x46, 2, 5, 0, false},
	{symLSR, ZPX, 0x56, 2, 6, 0, false},
	{symLSR, ABS, 0x4e, 3, 6, 0, false},
	{symLSR, ABX, 0x5e, 3, 7, 0, false},

	{symROL, ACC, 0x2a, 1, 2, 0, false},
	{symROL, ZPG, 0x26, 2, 5, 0, false},
	{symROL, ZPX, 0x36, 2, 6, 0, false},
	{symROL, ABS, 0x2e, 3, 6, 0, false},
	{symROL, ABX, 0x3e, 3, 7, 0, false},

	{symROR, ACC, 0x6a, 1, 2, 0, false},
	{symROR, ZPG, 0x66, 2, 5, 0, false},
	{symROR, ZPX, 0x76, 2, 6, 0, false},
	{symROR, ABS, 0x6e, 3, 6, 0, false},
	{symROR, ABX, 0x7e, 3, 7, 0, false},
}

// Unused opcodes
type unused struct {
	opcode byte
	mode   Mode
	length byte
	cycles byte
}

var unusedData = []unused{
	{0x02, ZPG, 2, 2},
	{0x22, ZPG, 2, 2},
	{0x42, ZPG, 2, 2},
	{0x62, ZPG, 2, 2},
	{0x82, ZPG, 2, 2},
	{0xc2, ZPG, 2, 2},
	{0xe2, ZPG, 2, 2},
	{0x03, ACC, 1, 1},
	{0x13, ACC, 1, 1},
	{0x23, ACC, 1, 1},
	{0x33, ACC, 1, 1},
	{0x43, ACC, 1, 1},
	{0x53, ACC, 1, 1},
	{0x63, ACC, 1, 1},
	{0x73, ACC, 1, 1},
	{0x83, ACC, 1, 1},
	{0x93, ACC, 1, 1},
	{0xa3, ACC, 1, 1},
	{0xb3, ACC, 1, 1},
	{0xc3, ACC, 1, 1},
	{0xd3, ACC, 1, 1},
	{0xe3, ACC, 1, 1},
	{0xf3, ACC, 1, 1},
	{0x44, ZPG, 2, 3},
	{0x54, ZPG, 2, 4},
	{0xd4, ZPG, 2, 4},
	{0xf4, ZPG, 2, 4},
	{0x07, ACC, 1, 1},
	{0x17, ACC, 1, 1},
	{0x27, ACC, 1, 1},
	{0x37, ACC, 1, 1},
	{0x47, ACC, 1, 1},
	{0x57, ACC, 1, 1},
	{0x67, ACC, 1, 1},
	{0x77, ACC, 1, 1},
	{0x87, ACC, 1, 1},
	{0x97, ACC, 1, 1},
	{0xa7, ACC, 1, 1},
	{0xb7, ACC, 1, 1},
	{0xc7, ACC, 1, 1},
	{0xd7, ACC, 1, 1},
	{0xe7, ACC, 1, 1},
	{0xf7, ACC, 1, 1},
	{0x0b, ACC, 1, 1},
	{0x1b, ACC, 1, 1},
	{0x2b, ACC, 1, 1},
	{0x3b, ACC, 1, 1},
	{0x4b, ACC, 1, 1},
	{0x5b, ACC, 1, 1},
	{0x6b, ACC, 1, 1},
	{0x7b, ACC, 1, 1},
	{0x8b, ACC, 1, 1},
	{0x9b, ACC, 1, 1},
	{0xab, ACC, 1, 1},
	{0xbb, ACC, 1, 1},
	{0xcb, ACC, 1, 1},
	{0xdb, ACC, 1, 1},
	{0xeb, ACC, 1, 1},
	{0xfb, ACC, 1, 1},
	{0x5c, ABS, 3, 8},
	{0xdc, ABS, 3, 4},
	{0xfc, ABS, 3, 4},
	{0x0f, ACC, 1, 1},
	{0x1f, ACC, 1, 1},
	{0x2f, ACC, 1, 1},
	{0x3f, ACC, 1, 1},
	{0x4f, ACC, 1, 1},
	{0x5f, ACC, 1, 1},
	{0x6f, ACC, 1, 1},
	{0x7f, ACC, 1, 1},
	{0x8f, ACC, 1, 1},
	{0x9f, ACC, 1, 1},
	{0xaf, ACC, 1, 1},
	{0xbf, ACC, 1, 1},
	{0xcf, ACC, 1, 1},
	{0xdf, ACC, 1, 1},
	{0xef, ACC, 1, 1},
	{0xff, ACC, 1, 1},
}

// An Instruction describes a CPU instruction, including its name,
// its addressing mode, its opcode value, its operand size, and its CPU cycle
// cost.
type Instruction struct {
	Name     string   // all-caps name of the instruction
	Mode     Mode     // addressing mode
	Opcode   byte     // hexadecimal opcode value
	Length   byte     // combined size of opcode and operand, in bytes
	Cycles   byte     // number of CPU cycles to execute the instruction
	BPCycles byte     // additional cycles required if boundary page crossed
	fn       instfunc // emulator implementation of the function
}

// An InstructionSet defines the set of all possible instructions that
// can run on the emulated CPU.
type InstructionSet struct {
	Arch         Architecture
	instructions [256]Instruction          // all instructions by opcode
	variants     map[string][]*Instruction // variants of each instruction
}

// Lookup retrieves a CPU instruction corresponding to the requested opcode.
func (s *InstructionSet) Lookup(opcode byte) *Instruction {
	return &s.instructions[opcode]
}

// GetInstructions returns all CPU instructions whose name matches the
// provided string.
func (s *InstructionSet) GetInstructions(name string) []*Instruction {
	return s.variants[strings.ToUpper(name)]
}

// Create an instruction set for a CPU architecture.
func newInstructionSet(arch Architecture) *InstructionSet {
	set := &InstructionSet{Arch: arch}

	// Create a map from symbol to implementation for fast lookups.
	symToImpl := make(map[opsym]*opcodeImpl, len(impl))
	for i := range impl {
		symToImpl[impl[i].sym] = &impl[i]
	}

	// Create a map from instruction name to the slice of all instruction
	// variants matching that name.
	set.variants = make(map[string][]*Instruction)

	unusedName := "???"

	// For each instruction, create a list of opcode variants valid for
	// the architecture.
	for _, d := range data {
		inst := &set.instructions[d.opcode]

		// If opcode has only a CMOS implementation and this is NMOS, create
		// an unused instruction for it.
		if d.cmos && arch != CMOS {
			inst.Name = unusedName
			inst.Mode = d.mode
			inst.Opcode = d.opcode
			inst.Length = d.length
			inst.Cycles = d.cycles
			inst.BPCycles = 0
			inst.fn = (*CPU).unusedn
			continue
		}

		impl := symToImpl[d.sym]
		if impl.fn[arch] == nil {
			continue // some opcodes have no architecture implementation
		}

		inst.Name = impl.name
		inst.Mode = d.mode
		inst.Opcode = d.opcode
		inst.Length = d.length
		inst.Cycles = d.cycles
		inst.BPCycles = d.bpcycles
		inst.fn = impl.fn[arch]

		set.variants[inst.Name] = append(set.variants[inst.Name], inst)
	}

	// Add unused opcodes to the instruction set. This information is useful
	// mostly for 65c02, where unused operations do something predicable
	// (i.e., eat cycles and nothing else).
	for _, u := range unusedData {
		inst := &set.instructions[u.opcode]
		inst.Name = unusedName
		inst.Mode = u.mode
		inst.Opcode = u.opcode
		inst.Length = u.length
		inst.Cycles = u.cycles
		inst.BPCycles = 0
		switch arch {
		case NMOS:
			inst.fn = (*CPU).unusedn
		case CMOS:
			inst.fn = (*CPU).unusedc
		}
	}

	for i := 0; i < 256; i++ {
		if set.instructions[i].Name == "" {
			panic("missing instruction")
		}
	}
	return set
}

var instructionSets [2]*InstructionSet

// GetInstructionSet returns an instruction set for the requested CPU
// architecture.
func GetInstructionSet(arch Architecture) *InstructionSet {
	if instructionSets[arch] == nil {
		// Lazy-create the instruction set.
		instructionSets[arch] = newInstructionSet(arch)
	}
	return instructionSets[arch]
}
