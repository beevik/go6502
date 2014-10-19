package go6502

// Register status bits.
type Status byte

const (
	Carry            Status = 1 << 0 // C
	Zero                    = 1 << 1 // Z
	InterruptDisable        = 1 << 2 // I
	Decimal                 = 1 << 3 // D
	Break                   = 1 << 4 // B
	Reserved                = 1 << 5
	Overflow                = 1 << 6 // V
	Negative                = 1 << 7 // S
)

// 6502 registers.
type Registers struct {
	A  byte    // accumulator
	X  byte    // X indexing register
	Y  byte    // Y indexing register
	SP byte    // stack pointer ($100 + SP = stack memory location)
	PC Address // program counter
	PS Status  // processor status bits
}

// Initialize all registers. A, X, Y = 0. SP = 0xff. PC = 0. PS = Reserved.
func (r *Registers) Init() {
	r.A = 0
	r.X = 0
	r.Y = 0
	r.SP = 0xff
	r.PC = 0
	r.PS = Reserved
}

// Return 1 if the process status bit 's' is set. Otherwise return 0.
func (r *Registers) GetStatus(s Status) byte {
	if (r.PS & s) == 0 {
		return 0
	} else {
		return 1
	}
}

// Set process status bit 's' to 1 if 'on' is true. Otherwise set it to 0.
func (r *Registers) SetStatus(s Status, on bool) {
	if on {
		r.PS |= s
	} else {
		r.PS &^= s
	}
}

// Return true if the processor status bit 's' is set.
func (r *Registers) IsStatusSet(s Status) bool {
	return (r.PS & s) != 0
}
