// Copyright 2014-2018 Brett Vickers. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cpu

// Registers contains the state of all 6502 registers.
type Registers struct {
	A                byte   // accumulator
	X                byte   // X indexing register
	Y                byte   // Y indexing register
	SP               byte   // stack pointer ($100 + SP = stack memory location)
	PC               uint16 // program counter
	Carry            bool   // PS: Carry bit
	Zero             bool   // PS: Zero bit
	InterruptDisable bool   // PS: Interrupt disable bit
	Decimal          bool   // PS: Decimal bit
	Overflow         bool   // PS: Overflow bit
	Sign             bool   // PS: Sign bit
}

// Bits assigned to the processor status byte
const (
	CarryBit            = 1 << 0
	ZeroBit             = 1 << 1
	InterruptDisableBit = 1 << 2
	DecimalBit          = 1 << 3
	BreakBit            = 1 << 4
	ReservedBit         = 1 << 5
	OverflowBit         = 1 << 6
	SignBit             = 1 << 7
)

// SavePS saves the CPU processor status into a byte value. The break bit
// is set if requested.
func (r *Registers) SavePS(brk bool) byte {
	var ps byte = ReservedBit // always saved as on
	if r.Carry {
		ps |= CarryBit
	}
	if r.Zero {
		ps |= ZeroBit
	}
	if r.InterruptDisable {
		ps |= InterruptDisableBit
	}
	if r.Decimal {
		ps |= DecimalBit
	}
	if brk {
		ps |= BreakBit
	}
	if r.Overflow {
		ps |= OverflowBit
	}
	if r.Sign {
		ps |= SignBit
	}
	return ps
}

// RestorePS restores the CPU processor status from a byte.
func (r *Registers) RestorePS(ps byte) {
	r.Carry = ((ps & CarryBit) != 0)
	r.Zero = ((ps & ZeroBit) != 0)
	r.InterruptDisable = ((ps & InterruptDisableBit) != 0)
	r.Decimal = ((ps & DecimalBit) != 0)
	r.Overflow = ((ps & OverflowBit) != 0)
	r.Sign = ((ps & SignBit) != 0)
}

func boolToUint32(v bool) uint32 {
	if v {
		return 1
	}
	return 0
}

func boolToByte(v bool) byte {
	if v {
		return 1
	}
	return 0
}

// Init initializes all registers. A, X, Y = 0. SP = 0xff. PC = 0. PS = 0.
func (r *Registers) Init() {
	r.A = 0
	r.X = 0
	r.Y = 0
	r.SP = 0xff
	r.PC = 0
	r.RestorePS(0)
}
