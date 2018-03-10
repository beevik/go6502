package main

import (
	"flag"
	"fmt"
	"io"
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

	run(r.Code, r.Origin, r.Exports)
}

func findExport(exports []asm.Export, origin uint16, names ...string) uint16 {
	table := make(map[string]uint16)
	for _, e := range exports {
		table[e.Label] = e.Addr
	}
	for _, n := range names {
		if a, ok := table[n]; ok {
			return a
		}
	}
	return origin
}

func loadMonitor(mem *go6502.FlatMemory) {
	file, err := os.Open("monitor.bin")
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	b := make([]byte, 0x800)
	_, err = io.ReadFull(file, b)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}

	mem.StoreBytes(uint16(0xf800), b)
}

func run(code []byte, origin uint16, exports []asm.Export) {

	fmt.Printf("\nRunning assembled code...\n")

	mem := go6502.NewFlatMemory()
	err := mem.StoreBytes(origin, code)
	if err != nil {
		panic(err)
	}

	loadMonitor(mem)

	pc := findExport(exports, origin, "START", "COLD.START", "RESTART")

	cpu := go6502.NewCPU(go6502.CMOS, mem)
	cpu.SetPC(pc)

	// Output initial state.
	fmt.Printf("                             A=%02X X=%02X Y=%02X PS=[%s] SP=%02X PC=%04X C=%d\n",
		cpu.Reg.A, cpu.Reg.X, cpu.Reg.Y, psString(&cpu.Reg),
		cpu.Reg.SP, cpu.Reg.PC,
		cpu.Cycles)

	buf := make([]byte, 3)

	// Step each instruction and output state after.
	for {
		pcStart := cpu.Reg.PC
		opcode, _ := cpu.Mem.LoadByte(pcStart)
		line, pcNext, _ := disasm.Disassemble(cpu.Mem, pcStart)
		cpu.Step()
		b := buf[:pcNext-pcStart]
		cpu.Mem.LoadBytes(pcStart, b)
		fmt.Printf("%04X- %-8s  %-11s  A=%02X X=%02X Y=%02X PS=[%s] SP=%02X PC=%04X C=%d\n",
			pcStart, codeString(b), line,
			cpu.Reg.A, cpu.Reg.X, cpu.Reg.Y, psString(&cpu.Reg),
			cpu.Reg.SP, cpu.Reg.PC,
			cpu.Cycles)
		if opcode == 0 {
			break
		}
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
		v(r.Sign, 'N'),
		v(r.Zero, 'Z'),
		v(r.Carry, 'C'),
		v(r.InterruptDisable, 'I'),
		v(r.Decimal, 'D'),
		v(r.Overflow, 'V'),
	}
	return string(b)
}
