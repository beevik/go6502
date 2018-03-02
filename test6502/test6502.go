package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/beevik/go6502"
	"github.com/beevik/go6502/asm"
	"github.com/beevik/go6502/disasm"
)

var verbose = flag.Bool("v", false, "Verbose output")

func main() {
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		fmt.Println("Syntax: test6502 [options] file")
		fmt.Println("Options:")
		flag.PrintDefaults()
		os.Exit(0)
	}

	file, err := os.Open(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	fmt.Printf("Assembling %s...\n", args[0])
	r, err := asm.Assemble(file, *verbose)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}

	if len(r.Exports) > 0 {
		fmt.Println("Exported addresses:")
		for _, e := range r.Exports {
			fmt.Printf("  %-15s $%04X\n", e.Label, e.Addr)
		}
	}

	run(r.Code, r.Origin)
}

func run(code []byte, origin go6502.Address) {
	fmt.Printf("\nRunning assembled code...\n")

	mem := go6502.NewMemory()
	mem.CopyBytes(origin, code)

	cpu := go6502.NewCPU(mem)
	cpu.SetPC(origin)

	// Output initial state.
	fmt.Printf("                             A=%02X X=%02X Y=%02X PS=[%s] SP=%02X PC=%04X C=%d\n",
		cpu.Reg.A, cpu.Reg.X, cpu.Reg.Y, psString(&cpu.Reg),
		cpu.Reg.SP, cpu.Reg.PC,
		cpu.Cycles)

	// Step each instruction and output state after.
	for i := 0; !cpu.Reg.Break; i++ {
		pcStart := cpu.Reg.PC
		line, pcNext := disasm.Disassemble(cpu.Mem, pcStart)
		cpu.Step()
		bc := cpu.Mem.LoadBytes(pcStart, int(pcNext-pcStart))
		fmt.Printf("%04X- %-8s  %-11s  A=%02X X=%02X Y=%02X PS=[%s] SP=%02X PC=%04X C=%d\n",
			pcStart, codeString(bc), line,
			cpu.Reg.A, cpu.Reg.X, cpu.Reg.Y, psString(&cpu.Reg),
			cpu.Reg.SP, cpu.Reg.PC,
			cpu.Cycles)
	}
}

func codeString(bc []byte) string {
	switch len(bc) {
	case 1:
		return fmt.Sprintf("%02X", bc[0])
	case 2:
		return fmt.Sprintf("%02X %02X", bc[0], bc[1])
	case 3:
		return fmt.Sprintf("%02X %02X %02X", bc[0], bc[1], bc[2])
	default:
		return ""
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
