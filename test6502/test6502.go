package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/beevik/go6502"
	"github.com/beevik/go6502/asm"
	"github.com/beevik/go6502/disasm"
	"github.com/beevik/prefixtree"
)

var verbose = flag.Bool("v", false, "Verbose output")

var cmds = newCommands([]command{
	{name: "step", description: "Step the CPU", handler: (*host).OnStepCPU},
	{name: "quit", description: "Quit the program", handler: (*host).OnQuit},
})

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

	h := newHost()
	h.LoadROM()
	h.Load(r.Code, r.Origin)

	addr := findExport(r.Exports, r.Origin, "START", "COLD.START", "RESTART")
	h.SetStart(addr)
	h.Repl()
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

type host struct {
	mem      *go6502.FlatMemory
	cpu      *go6502.CPU
	debugger *go6502.Debugger
}

func newHost() *host {
	h := new(host)

	h.mem = go6502.NewFlatMemory()
	h.cpu = go6502.NewCPU(go6502.CMOS, h.mem)
	h.debugger = go6502.NewDebugger(h)
	h.cpu.AttachDebugger(h.debugger)

	return h
}

func (h *host) OnBreakpoint(cpu *go6502.CPU, addr uint16) {
}

func (h *host) OnDataBreakpoint(cpu *go6502.CPU, addr uint16, v byte) {
}

func (h *host) LoadROM() {
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

	h.mem.StoreBytes(uint16(0xf800), b)
}

func (h *host) Load(code []byte, origin uint16) {
	h.mem.StoreBytes(origin, code)
}

func (h *host) SetStart(addr uint16) {
	h.cpu.SetPC(addr)
}

func (h *host) Repl() error {
	c := newConn(os.Stdin, os.Stdout)
	c.interactive = true
	return h.RunCommands(c)
}

func (h *host) RunCommands(c *conn) error {
	var r commandResult
	for {
		if c.interactive {
			c.Printf("* ")
			c.Flush()
		}

		line, err := c.GetLine()
		if err != nil {
			break
		}

		if !c.interactive {
			c.Printf("* %s\n", line)
		}

		if line != "" {
			r, err = cmds.find(line)
			switch {
			case err == prefixtree.ErrPrefixNotFound:
				c.Println("command not found.")
				continue
			case err == prefixtree.ErrPrefixAmbiguous:
				c.Println("command ambiguous.")
				continue
			case err != nil:
				c.Printf("%v.\n", err)
				continue
			case r.helpText != "":
				c.Printf("%s", r.helpText)
				continue
			}
		}
		if r.cmd == nil {
			continue
		}

		err = r.cmd.handler(h, c, r.args)
		if err != nil {
			break
		}
	}

	return nil
}

func (h *host) OnQuit(c *conn, args string) error {
	return errors.New("Exiting program")
}

func (h *host) OnStepCPU(c *conn, args string) error {
	cpu := h.cpu

	buf := make([]byte, 3)
	pcStart := cpu.Reg.PC
	opcode := cpu.Mem.LoadByte(pcStart)
	line, pcNext := disasm.Disassemble(cpu.Mem, pcStart)

	cpu.Step()

	b := buf[:pcNext-pcStart]
	cpu.Mem.LoadBytes(pcStart, b)

	fmt.Printf("%04X- %-8s  %-11s  A=%02X X=%02X Y=%02X PS=[%s] SP=%02X PC=%04X C=%d\n",
		pcStart, codeString(b), line,
		cpu.Reg.A, cpu.Reg.X, cpu.Reg.Y, psString(&cpu.Reg),
		cpu.Reg.SP, cpu.Reg.PC,
		cpu.Cycles)

	_ = opcode

	return nil
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
