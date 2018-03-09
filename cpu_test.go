package go6502

import (
	"testing"
)

func TestUnused65c02(t *testing.T) {
	code := []byte{
		0x02, 0x00,
		0x03,
		0x07,
		0x0b,
		0x0f,
		0xfc, 0x01, 0x02,
	}

	cpu := NewCPU(CMOS, NewMemory())
	cpu.Mem.CopyBytes(0x1000, code)
	cpu.SetPC(0x1000)
	for i := 0; i < 6; i++ {
		cpu.Step()
	}

	if cpu.Reg.PC != 0x1009 {
		t.Errorf("PC incorrect. exp: $%04X, got: $%04X", 0x1009, cpu.Reg.PC)
	}
	if cpu.Cycles != 10 {
		t.Errorf("Cycles incorrect. exp: %d, got: %d", 10, cpu.Cycles)
	}
}
