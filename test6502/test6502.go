package main

import (
	"fmt"
	"os"

	"github.com/beevik/go6502"
	"github.com/beevik/go6502/asm"
	"github.com/beevik/go6502/disasm"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Syntax: test6502 [file.asm]")
		os.Exit(0)
	}
	assemble(os.Args[1])
}

func assemble(filename string) {
	file, err := os.Open(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	fmt.Printf("Assembling %s...\n\n", filename)
	code, origin, err := asm.Assemble(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nRunning assembled code...\n\n")

	mem := go6502.NewMemory()
	mem.CopyBytes(origin, code)

	cpu := go6502.NewCPU(mem)
	cpu.SetPC(origin)

	for i := 0; !cpu.Reg.Break; i++ {
		pc := cpu.Reg.PC
		line, _ := disasm.Disassemble(cpu.Mem, pc)
		cpu.Step()
		fmt.Printf("%04X-   %-12s  A=%02X X=%02X Y=%02X PS=[%s] SP=%02X PC=%04X Cycles=%d\n",
			pc, line,
			cpu.Reg.A, cpu.Reg.X, cpu.Reg.Y, psString(&cpu.Reg),
			cpu.Reg.SP, cpu.Reg.PC,
			cpu.Cycles)
	}
}

func psString(r *go6502.Registers) string {
	v := func(bit bool, ch byte) byte {
		if bit {
			return ch
		}
		return '-'
	}
	b := []byte{
		v(r.Carry, 'C'),
		v(r.Zero, 'Z'),
		v(r.InterruptDisable, 'I'),
		v(r.Decimal, 'D'),
		v(r.Break, 'B'),
		v(r.Overflow, 'O'),
		v(r.Negative, 'N'),
	}
	return string(b)
}
