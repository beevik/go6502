package cpu_test

import (
	"os"
	"strings"
	"testing"

	"github.com/beevik/go6502/asm"
	"github.com/beevik/go6502/cpu"
)

func loadCPU(t *testing.T, asmString string) *cpu.CPU {
	b := strings.NewReader(asmString)
	r, sm, err := asm.Assemble(b, "test.asm", 0x1000, os.Stdout, 0)
	if err != nil {
		t.Error(err)
		return nil
	}

	mem := cpu.NewFlatMemory()
	cpu := cpu.NewCPU(cpu.NMOS, mem)
	mem.StoreBytes(sm.Origin, r.Code)
	cpu.SetPC(sm.Origin)
	return cpu
}

func stepCPU(cpu *cpu.CPU, steps int) {
	for i := 0; i < steps; i++ {
		cpu.Step()
	}
}

func runCPU(t *testing.T, asmString string, steps int) *cpu.CPU {
	cpu := loadCPU(t, asmString)
	if cpu != nil {
		stepCPU(cpu, steps)
	}
	return cpu
}

func expectPC(t *testing.T, cpu *cpu.CPU, pc uint16) {
	if cpu.Reg.PC != pc {
		t.Errorf("PC incorrect. exp: $%04X, got: $%04X", pc, cpu.Reg.PC)
	}
}

func expectCycles(t *testing.T, cpu *cpu.CPU, cycles uint64) {
	if cpu.Cycles != cycles {
		t.Errorf("Cycles incorrect. exp: %d, got: %d", cycles, cpu.Cycles)
	}
}

func expectACC(t *testing.T, cpu *cpu.CPU, acc byte) {
	if cpu.Reg.A != acc {
		t.Errorf("Accumulator incorrect. exp: $%02X, got: $%02X", acc, cpu.Reg.A)
	}
}

func expectSP(t *testing.T, cpu *cpu.CPU, sp byte) {
	if cpu.Reg.SP != sp {
		t.Errorf("stack pointer incorrect. exp: %02X, got $%02X", sp, cpu.Reg.SP)
	}
}

func expectMem(t *testing.T, cpu *cpu.CPU, addr uint16, v byte) {
	got := cpu.Mem.LoadByte(addr)
	if got != v {
		t.Errorf("Memory at $%04X incorrect. exp: $%02X, got: $%02X", addr, v, got)
	}
}

func TestAccumulator(t *testing.T) {
	asm := `
	.ORG $1000
	LDA #$5E
	STA $15
	STA $1500`

	cpu := runCPU(t, asm, 3)
	if cpu == nil {
		return
	}

	expectPC(t, cpu, 0x1007)
	expectCycles(t, cpu, 9)
	expectACC(t, cpu, 0x5e)
	expectMem(t, cpu, 0x15, 0x5e)
	expectMem(t, cpu, 0x1500, 0x5e)
}

func TestStack(t *testing.T) {
	asm := `
	.ORG $1000
	LDA #$11
	PHA
	LDA #$12
	PHA
	LDA #$13
	PHA

	PLA
	STA $2000
	PLA
	STA $2001
	PLA
	STA $2002`

	cpu := loadCPU(t, asm)
	stepCPU(cpu, 6)

	expectSP(t, cpu, 0xfc)
	expectACC(t, cpu, 0x13)
	expectMem(t, cpu, 0x1ff, 0x11)
	expectMem(t, cpu, 0x1fe, 0x12)
	expectMem(t, cpu, 0x1fd, 0x13)

	stepCPU(cpu, 6)
	expectACC(t, cpu, 0x11)
	expectSP(t, cpu, 0xff)
	expectMem(t, cpu, 0x2000, 0x13)
	expectMem(t, cpu, 0x2001, 0x12)
	expectMem(t, cpu, 0x2002, 0x11)
}

func TestIndirect(t *testing.T) {
	asm := `
	.ORG $1000
	LDX #$80
	LDY #$40
	LDA #$EE
	STA $2000,X
	STA $2000,Y

	LDA #$11
	STA $06
	LDA #$05
	STA $07
	LDX #$01
	LDY #$01
	LDA #$BB
	STA ($05,X)
	STA ($06),Y
	`

	cpu := runCPU(t, asm, 14)
	expectMem(t, cpu, 0x2080, 0xee)
	expectMem(t, cpu, 0x2040, 0xee)
	expectMem(t, cpu, 0x0511, 0xbb)
	expectMem(t, cpu, 0x0512, 0xbb)
}

func TestPageCross(t *testing.T) {
	asm := `
	.ORG $1000
	LDA #$55		; 2 cycles
	STA $1101		; 4 cycles
	LDA #$00		; 2 cycles
	LDX #$FF		; 2 cycles
	LDA $1002,X		; 5 cycles`

	cpu := runCPU(t, asm, 5)
	if cpu == nil {
		return
	}

	expectPC(t, cpu, 0x100c)
	expectCycles(t, cpu, 15)
	expectACC(t, cpu, 0x55)
	expectMem(t, cpu, 0x1101, 0x55)
}

func TestUnused65c02(t *testing.T) {
	asm := `
	.ORG $1000
	.ARCH 65c02
	.DH 0200
	.DH 03
	.DH 07
	.DH 0b
	.DH 0f
	.DH fc0102`

	cpu := runCPU(t, asm, 6)
	if cpu == nil {
		return
	}

	expectPC(t, cpu, 0x1009)
	expectCycles(t, cpu, 10)
}
