package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/beevik/go6502"
	"github.com/beevik/go6502/asm"
	"github.com/beevik/go6502/disasm"
	"github.com/beevik/prefixtree"
)

var signature = "56og"

var cmds = newCommands([]command{
	{name: "assemble", description: "Assemble a file", handler: (*host).OnAssemble},
	{name: "load", description: "Load a binary", handler: (*host).OnLoad},
	{name: "registers", description: "Display register contents", handler: (*host).OnRegisters},
	{name: "step", description: "Step the CPU", handler: (*host).OnStepCPU},
	{name: "quit", description: "Quit the program", handler: (*host).OnQuit},
	{name: "r", handler: (*host).OnRegisters},
	{name: "s", handler: (*host).OnStepCPU},
})

func main() {
	h := newHost()

	args := os.Args[1:]

	switch {
	case len(args) == 0:
		h.Repl()
	default:
		for _, filename := range args {
			h.Exec(filename)
		}
	}
}

func exitOnError(err error) {
	fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
	os.Exit(1)
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
	sourceMap asm.SourceMap
	mem       *go6502.FlatMemory
	cpu       *go6502.CPU
	debugger  *go6502.Debugger
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

func (h *host) Load(code []byte, origin uint16) {
	h.mem.StoreBytes(origin, code)
}

func (h *host) SetStart(addr uint16) {
	h.cpu.SetPC(addr)
}

func (h *host) Repl() {
	c := newConn(os.Stdin, os.Stdout)
	c.interactive = true
	h.RunCommands(c)
}

func (h *host) Exec(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		exitOnError(err)
	}

	c := newConn(file, os.Stdout)
	return h.RunCommands(c)
}

func (h *host) RunCommands(c *conn) error {
	h.load(c, "monitor.bin", 0xf800)

	var r commandResult
	for {
		if c.interactive {
			c.Printf("%04X* ", h.cpu.Reg.PC)
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

func (h *host) OnAssemble(c *conn, args string) error {
	a := splitArgs(args)

	if len(a) < 1 {
		c.Println("Syntax: assemble [filename]")
		return nil
	}

	filename := a[0]
	if filepath.Ext(filename) == "" {
		filename += ".asm"
	}

	file, err := os.Open(filename)
	if err != nil {
		c.Printf("Failed to open '%s': %v\n", filepath.Base(filename), err)
		return nil
	}
	defer file.Close()

	r, err := asm.Assemble(file, filename, false)
	if err != nil {
		c.Printf("Failed to assemble: %s\n%v\n", filepath.Base(filename), err)
		return nil
	}

	file.Close()

	ext := filepath.Ext(filename)
	filePrefix := filename[0 : len(filename)-len(ext)]
	binFilename := filePrefix + ".bin"
	file, err = os.OpenFile(binFilename, os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		c.Printf("Failed to create '%s': %v\n", filepath.Base(binFilename), err)
		return nil
	}

	var hdr [6]byte
	copy(hdr[:4], []byte(signature))
	hdr[4] = byte(r.Origin)
	hdr[5] = byte(r.Origin >> 8)
	_, err = file.Write(hdr[:])
	if err != nil {
		c.Printf("Failed to write '%s': %v\n", filepath.Base(binFilename), err)
		return nil
	}

	_, err = file.Write(r.Code)
	if err != nil {
		c.Printf("Failed to write '%s': %v\n", filepath.Base(binFilename), err)
		return nil
	}

	file.Close()

	mapFilename := filePrefix + ".map"
	file, err = os.OpenFile(mapFilename, os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		c.Printf("Failed to create '%s': %v\n", filepath.Base(mapFilename), err)
		return nil
	}

	_, err = r.SourceMap.WriteTo(file)
	if err != nil {
		c.Printf("Failed to write '%s': %v\n", filepath.Base(mapFilename), err)
		return nil
	}

	file.Close()

	c.Printf("Assembled '%s' to '%s'.\n", filepath.Base(filename), filepath.Base(binFilename))
	return nil
}

func (h *host) OnLoad(c *conn, args string) error {
	a := splitArgs(args)

	origin := -1
	if len(a) < 1 {
		c.Println("Syntax: load [filename] [addr]")
		return nil
	}

	filename := a[0]
	if filepath.Ext(filename) == "" {
		filename += ".bin"
	}

	if len(a) >= 2 {
		oStr := a[1]
		if startsWith(oStr, "0x") {
			oStr = oStr[2:]
		} else if startsWith(oStr, "$") {
			oStr = oStr[1:]
		}

		o, err := strconv.ParseInt(oStr, 16, 32)
		if err != nil || o < 0 || o > 0xffff {
			c.Printf("Unable to parse address '%s'\n", a[1])
			return nil
		}
		origin = int(o)
	}

	return h.load(c, filename, origin)
}

func (h *host) load(c *conn, filename string, origin int) error {
	filename, err := filepath.Abs(filename)
	if err != nil {
		c.Printf("Failed to open '%s': %v\n", filepath.Base(filename), err)
		return nil
	}

	file, err := os.Open(filename)
	if err != nil {
		c.Printf("Failed to open '%s': %v\n", filepath.Base(filename), err)
		return nil
	}
	defer file.Close()

	b, err := ioutil.ReadAll(file)
	if err != nil {
		c.Printf("Failed to read '%s': %v\n", filepath.Base(filename), err)
		return nil
	}

	file.Close()

	code := b
	if len(b) >= 6 && string(b[:4]) == signature {
		origin = int(b[4]) | int(b[5])<<8
		code = b[6:]
	}
	if origin == -1 {
		c.Printf("File '%s' has no signature and requires an address\n", filepath.Base(filename))
		return nil
	}

	if origin+len(code) > 0x10000 {
		c.Printf("File '%s' exceeded 64K memory bounds\n", filepath.Base(filename))
		return nil
	}

	cpu := h.cpu
	cpu.Mem.StoreBytes(uint16(origin), code)
	c.Printf("Loaded '%s' to $%04X..$%04X\n", filepath.Base(filename), origin, int(origin)+len(code)-1)

	cpu.SetPC(uint16(origin))

	ext := filepath.Ext(filename)
	filePrefix := filename[:len(filename)-len(ext)]
	filename = filePrefix + ".map"

	file, err = os.Open(filename)
	if err == nil {
		_, err = h.sourceMap.ReadFrom(file)
		if err != nil {
			c.Printf("Failled to read '%s': %v\n", filepath.Base(filename), err)
		} else {
			c.Printf("Loaded '%s' source map\n", filepath.Base(filename))
		}
	}

	file.Close()
	return nil
}

func (h *host) OnRegisters(c *conn, args string) error {
	reg := disasm.GetRegisterString(&h.cpu.Reg)
	fmt.Printf("%s\n", reg)
	return nil
}

func (h *host) OnStepCPU(c *conn, args string) error {
	cpu := h.cpu

	buf := make([]byte, 3)
	start := cpu.Reg.PC
	line, next := disasm.Disassemble(cpu.Mem, start)

	cpu.Step()

	b := buf[:next-start]
	cpu.Mem.LoadBytes(start, b)

	regStr := disasm.GetRegisterString(&cpu.Reg)
	fmt.Printf("%04X- %-8s  %-11s  %s C=%d\n",
		start, codeString(b), line, regStr, cpu.Cycles)

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

func splitArgs(args string) []string {
	ss := make([]string, 0)
	for len(args) > 0 {
		i := strings.IndexAny(args, " \t")
		if i == -1 {
			if len(args) > 0 {
				ss = append(ss, args)
			}
			break
		}

		if i > 0 {
			arg := args[:i]
			ss = append(ss, arg)
		}

		for i < len(args) && (args[i] == ' ' || args[i] == '\t') {
			i++
		}
		args = args[i:]
	}
	return ss
}

func startsWith(s, m string) bool {
	if len(s) < len(m) {
		return false
	}
	return s[:len(m)] == m
}

func endsWith(s, m string) bool {
	if len(s) < len(m) {
		return false
	}
	return s[len(s)-len(m):] == m
}
