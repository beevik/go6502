package go6502

// Registers contains the state of all 6502 registers.
type Registers struct {
	A                byte    // accumulator
	X                byte    // X indexing register
	Y                byte    // Y indexing register
	SP               byte    // stack pointer ($100 + SP = stack memory location)
	PC               Address // program counter
	Carry            bool    // PS: Carry bit
	Zero             bool    // PS: Zero bit
	InterruptDisable bool    // PS: Interrupt disable bit
	Decimal          bool    // PS: Decimal bit
	Break            bool    // PS: Break bit
	Overflow         bool    // PS: Overflow bit
	Negative         bool    // PS: Negative bit
}

// GetPS returns the CPU processor status byte value.
func (r *Registers) GetPS() byte {
	var ps byte
	if r.Carry {
		ps |= (1 << 0)
	}
	if r.Zero {
		ps |= (1 << 1)
	}
	if r.InterruptDisable {
		ps |= (1 << 2)
	}
	if r.Decimal {
		ps |= (1 << 3)
	}
	if r.Break {
		ps |= (1 << 4)
	}
	if r.Overflow {
		ps |= (1 << 6)
	}
	if r.Negative {
		ps |= (1 << 7)
	}
	return ps
}

// SetPS updates the CPU processor status byte.
func (r *Registers) SetPS(ps byte) {
	r.Carry = ((ps & (1 << 0)) != 0)
	r.Zero = ((ps & (1 << 1)) != 0)
	r.InterruptDisable = ((ps & (1 << 2)) != 0)
	r.Decimal = ((ps & (1 << 3)) != 0)
	r.Break = ((ps & (1 << 4)) != 0)
	r.Overflow = ((ps & (1 << 6)) != 0)
	r.Negative = ((ps & (1 << 7)) != 0)
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
	r.SetPS(0)
}
