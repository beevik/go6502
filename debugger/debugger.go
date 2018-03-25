package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"

	"github.com/beevik/cmd"
	"github.com/beevik/go6502"
	"github.com/beevik/go6502/asm"
	"github.com/beevik/go6502/disasm"
)

var signature = "56og"

var cmds = cmd.NewTree("Debugger", []cmd.Command{
	{Name: "help", Shortcut: "?", Param: (*host).OnHelp},
	{Name: "assemble", Description: "Assemble a file", Param: (*host).OnAssemble},
	{Name: "load", Description: "Load a binary", Param: (*host).OnLoad},
	{Name: "registers", Shortcut: "r", Description: "Display register contents", Param: (*host).OnRegisters},
	{Name: "disassemble", Shortcut: "d", Description: "Disassemble code", Param: (*host).OnDisassemble},
	{Name: "step", Description: "Step the debugger", Subcommands: cmd.NewTree("Step", []cmd.Command{
		{Name: "help", Shortcut: "?", Param: (*host).OnHelp},
		{Name: "in", Description: "Step in to routine", Param: (*host).OnStepIn},
		{Name: "over", Description: "Step over a routine", Param: (*host).OnStepOver},
	})},
	{Name: "run", Description: "Run the CPU", Param: (*host).OnRun},
	{Name: "exports", Description: "List exported addresses", Param: (*host).OnExports},
	{Name: "breakpoint", Shortcut: "b", Description: "Breakpoint commands", Subcommands: cmd.NewTree("Breakpoint", []cmd.Command{
		{Name: "help", Shortcut: "?", Param: (*host).OnHelp},
		{Name: "list", Description: "List breakpoints", Param: (*host).OnBreakpointList},
		{Name: "add", Description: "Add a breakpoint", Param: (*host).OnBreakpointAdd},
		{Name: "remove", Description: "Remove a breakpoint", Param: (*host).OnBreakpointRemove},
		{Name: "enable", Description: "Enable a breakpoint", Param: (*host).OnBreakpointEnable},
		{Name: "disable", Description: "Disable a breakpoint", Param: (*host).OnBreakpointDisable},
	})},
	{Name: "databreakpoint", Shortcut: "db", Description: "Data breakpoint commands", Subcommands: cmd.NewTree("Data breakpoint", []cmd.Command{
		{Name: "help", Shortcut: "?", Param: (*host).OnHelp},
		{Name: "list", Description: "List data breakpoints", Param: (*host).OnDataBreakpointList},
		{Name: "add", Description: "Add a data breakpoint", Param: (*host).OnDataBreakpointAdd},
		{Name: "remove", Description: "Remove a data breakpoint", Param: (*host).OnDataBreakpointRemove},
		{Name: "enable", Description: "Enable a data breakpoint", Param: (*host).OnDataBreakpointEnable},
		{Name: "disable", Description: "Disable a data breakpoint", Param: (*host).OnDataBreakpointDisable},
	})},
	{Name: "quit", Description: "Quit the program", Param: (*host).OnQuit},

	// Shortcuts to nested commands
	{Name: "ba", Param: (*host).OnBreakpointAdd},
	{Name: "br", Param: (*host).OnBreakpointRemove},
	{Name: "bl", Param: (*host).OnBreakpointList},
	{Name: "be", Param: (*host).OnBreakpointEnable},
	{Name: "bd", Param: (*host).OnBreakpointDisable},
	{Name: "dbl", Param: (*host).OnDataBreakpointList},
	{Name: "dba", Param: (*host).OnDataBreakpointAdd},
	{Name: "dbr", Param: (*host).OnDataBreakpointRemove},
	{Name: "dbe", Param: (*host).OnDataBreakpointEnable},
	{Name: "dbd", Param: (*host).OnDataBreakpointDisable},
	{Name: "si", Param: (*host).OnStepIn},
	{Name: "s", Param: (*host).OnStepOver},
})

func main() {
	h := newHost()

	// Create a goroutine to handle ctrl-C.
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for {
			<-c
			h.Println()
			if h.state == stateProcessingCommands {
				h.Prompt()
			}
			h.state = stateProcessingCommands
		}
	}()

	// Run commands contained in the command-line files.
	args := os.Args[1:]
	if len(args) > 0 {
		for _, filename := range args {
			file, err := os.Open(filename)
			if err != nil {
				exitOnError(err)
			}
			h.RunCommands(file, os.Stdout, false)
			file.Close()
		}
	}

	// Start the interactive debugger.
	h.RunCommands(os.Stdin, os.Stdout, true)
}

type state byte

const (
	stateProcessingCommands state = iota
	stateRunning
	stateBreakpoint
	stateStepOverBreakpoint
)

type host struct {
	interactive bool
	input       *bufio.Scanner
	output      *bufio.Writer

	mem      *go6502.FlatMemory
	cpu      *go6502.CPU
	debugger *go6502.Debugger

	sourceMap asm.SourceMap
	buf       []byte
	state     state
	settings  map[string]interface{}
}

func newHost() *host {
	h := &host{
		buf:   make([]byte, 3),
		mem:   go6502.NewFlatMemory(),
		state: stateProcessingCommands,
		settings: map[string]interface{}{
			"maxStepDisplayCount": 20,
		},
	}

	h.cpu = go6502.NewCPU(go6502.CMOS, h.mem)
	h.debugger = go6502.NewDebugger(h)
	h.cpu.AttachDebugger(h.debugger)

	return h
}

func (h *host) Print(args ...interface{}) {
	fmt.Fprint(h.output, args...)
}

func (h *host) Printf(format string, args ...interface{}) {
	fmt.Fprintf(h.output, format, args...)
	h.Flush()
}

func (h *host) Println(args ...interface{}) {
	fmt.Fprintln(h.output, args...)
	h.Flush()
}

func (h *host) Flush() {
	h.output.Flush()
}

func (h *host) GetLine() (string, error) {
	if h.input.Scan() {
		return h.input.Text(), nil
	}
	if h.input.Err() != nil {
		return "", h.input.Err()
	}
	return "", io.EOF
}

func (h *host) Prompt() {
	if h.interactive {
		h.Printf("* ")
		h.Flush()
	}
}

func (h *host) DisplayPC() {
	if h.interactive {
		disStr, _ := h.Disassemble(h.cpu.Reg.PC)
		regStr := disasm.GetRegisterString(&h.cpu.Reg)
		fmt.Print(disStr)
		fmt.Printf("  %s C=%d\n", regStr, h.cpu.Cycles)
	}
}

func (h *host) RunCommands(r io.Reader, w io.Writer, interactive bool) {
	h.input = bufio.NewScanner(r)
	h.output = bufio.NewWriter(w)
	h.interactive = interactive

	h.DisplayPC()

	var sel cmd.Selection
	for {
		h.Prompt()

		line, err := h.GetLine()
		if err != nil {
			break
		}

		if line != "" {
			sel, err = cmds.Lookup(line)
			switch {
			case err == cmd.ErrNotFound:
				h.Println("Command not found.")
				continue
			case err == cmd.ErrAmbiguous:
				h.Println("Command is ambiguous.")
				continue
			case err != nil:
				h.Printf("ERROR: %v.\n", err)
				continue
			}
		}
		if sel.Command == nil {
			continue
		}

		handler := sel.Command.Param.(func(*host, cmd.Selection) error)
		err = handler(h, sel)
		if err != nil {
			break
		}
	}
}

func (h *host) OnHelp(sel cmd.Selection) error {
	commands := sel.Command.Tree
	h.Printf("%s commands:\n", commands.Title)
	for _, c := range commands.Commands {
		if c.Description != "" {
			h.Printf("    %-15s  %s\n", c.Name, c.Description)
		}
	}
	return nil
}

func (h *host) OnAssemble(sel cmd.Selection) error {
	if len(sel.Args) < 1 {
		h.Println("Syntax: assemble [filename]")
		return nil
	}

	filename := sel.Args[0]
	if filepath.Ext(filename) == "" {
		filename += ".asm"
	}

	file, err := os.Open(filename)
	if err != nil {
		h.Printf("Failed to open '%s': %v\n", filepath.Base(filename), err)
		return nil
	}
	defer file.Close()

	r, err := asm.Assemble(file, filename, false)
	if err != nil {
		h.Printf("Failed to assemble: %s\n%v\n", filepath.Base(filename), err)
		return nil
	}

	file.Close()

	ext := filepath.Ext(filename)
	filePrefix := filename[0 : len(filename)-len(ext)]
	binFilename := filePrefix + ".bin"
	file, err = os.OpenFile(binFilename, os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		h.Printf("Failed to create '%s': %v\n", filepath.Base(binFilename), err)
		return nil
	}

	var hdr [6]byte
	copy(hdr[:4], []byte(signature))
	hdr[4] = byte(r.Origin)
	hdr[5] = byte(r.Origin >> 8)
	_, err = file.Write(hdr[:])
	if err != nil {
		h.Printf("Failed to write '%s': %v\n", filepath.Base(binFilename), err)
		return nil
	}

	_, err = file.Write(r.Code)
	if err != nil {
		h.Printf("Failed to write '%s': %v\n", filepath.Base(binFilename), err)
		return nil
	}

	file.Close()

	mapFilename := filePrefix + ".map"
	file, err = os.OpenFile(mapFilename, os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		h.Printf("Failed to create '%s': %v\n", filepath.Base(mapFilename), err)
		return nil
	}

	_, err = r.SourceMap.WriteTo(file)
	if err != nil {
		h.Printf("Failed to write '%s': %v\n", filepath.Base(mapFilename), err)
		return nil
	}

	file.Close()

	h.Printf("Assembled '%s' to '%s'.\n", filepath.Base(filename), filepath.Base(binFilename))
	return nil
}

func (h *host) OnLoad(sel cmd.Selection) error {
	if len(sel.Args) < 1 {
		h.Println("Syntax: load [filename] [addr]")
		return nil
	}

	filename := sel.Args[0]
	if filepath.Ext(filename) == "" {
		filename += ".bin"
	}

	addr := -1
	if len(sel.Args) >= 2 {
		addr = h.ParseAddr(sel.Args[1])
		if addr < 0 {
			h.Printf("Unable to parse address '%s'\n", sel.Args[1])
			return nil
		}
	}

	_, err := h.Load(filename, addr)
	return err
}

func (h *host) OnRegisters(sel cmd.Selection) error {
	reg := disasm.GetRegisterString(&h.cpu.Reg)
	fmt.Printf("%s\n", reg)
	return nil
}

func (h *host) OnDisassemble(sel cmd.Selection) error {
	// TODO: write me
	return nil
}

func (h *host) OnStepIn(sel cmd.Selection) error {
	// Parse the number of steps.
	count := 1
	if len(sel.Args) > 0 {
		n, err := strconv.ParseInt(sel.Args[0], 10, 16)
		if err == nil {
			count = int(n)
		}
	}

	maxStepDisplayCount := h.settings["maxStepDisplayCount"].(int)

	// Step the CPU count times.
	h.state = stateRunning
	for i := count - 1; i >= 0 && h.state == stateRunning; i-- {
		h.Step()
		switch {
		case i == maxStepDisplayCount:
			h.Println("...")
		case i < maxStepDisplayCount:
			h.DisplayPC()
		}
	}
	h.state = stateProcessingCommands

	return nil
}

func (h *host) OnStepOver(sel cmd.Selection) error {
	// Parse the number of steps.
	count := 1
	if len(sel.Args) > 0 {
		n, err := strconv.ParseInt(sel.Args[0], 10, 16)
		if err == nil {
			count = int(n)
		}
	}

	maxStepDisplayCount := h.settings["maxStepDisplayCount"].(int)

	// Step over the next instruction count times.
	h.state = stateRunning
	for i := count - 1; i >= 0 && h.state == stateRunning; i-- {
		h.StepOver()
		switch {
		case i == maxStepDisplayCount:
			h.Println("...")
		case i < maxStepDisplayCount:
			h.DisplayPC()
		}
	}
	h.state = stateProcessingCommands

	return nil
}

func (h *host) OnRun(sel cmd.Selection) error {
	if len(sel.Args) > 0 {
		pc := h.ParseAddr(sel.Args[0])
		if pc < 0 {
			h.Printf("Unable to parse address '%s'\n", sel.Args[0])
			return nil
		}
		h.cpu.SetPC(uint16(pc))
	}

	h.Printf("Running from $%04X. Press ctrl-C to break.\n", h.cpu.Reg.PC)

	h.state = stateRunning
	for h.state == stateRunning {
		h.Step()
	}
	h.state = stateProcessingCommands

	return nil
}

func (h *host) OnExports(sel cmd.Selection) error {
	for _, e := range h.sourceMap.Exports {
		h.Printf("%-16s $%04X\n", e.Label, e.Addr)
	}
	return nil
}

func (h *host) OnBreakpointList(sel cmd.Selection) error {
	h.Println("Addr  Enabled")
	h.Println("----- -------")
	for _, b := range h.debugger.GetBreakpoints() {
		h.Printf("$%04X %v\n", b.Address, !b.Disabled)
	}
	return nil
}

func (h *host) OnBreakpointAdd(sel cmd.Selection) error {
	if len(sel.Args) < 1 {
		h.Printf("Syntax: breakpoint add [addr]\n")
		return nil
	}

	addr := h.ParseAddr(sel.Args[0])
	if addr < 0 {
		h.Printf("Invalid breakpoint address '%v'\n", sel.Args[0])
		return nil
	}

	h.debugger.AddBreakpoint(uint16(addr))
	h.Printf("Breakpoint added at $%04x.\n", addr)
	return nil
}

func (h *host) OnBreakpointRemove(sel cmd.Selection) error {
	if len(sel.Args) < 1 {
		h.Printf("Syntax: breakpoint remove [addr]\n")
		return nil
	}

	addr := h.ParseAddr(sel.Args[0])
	if addr < 0 {
		h.Printf("Invalid breakpoint address '%v'\n", sel.Args[0])
		return nil
	}

	if h.debugger.GetBreakpoint(uint16(addr)) == nil {
		h.Printf("No breakpoint was set on $%04X.\n", addr)
		return nil
	}

	h.debugger.RemoveBreakpoint(uint16(addr))
	h.Printf("Breakpoint at $%04x removed.\n", addr)
	return nil
}

func (h *host) OnBreakpointEnable(sel cmd.Selection) error {
	if len(sel.Args) < 1 {
		h.Printf("Syntax: breakpoint enable [addr]\n")
		return nil
	}

	addr := h.ParseAddr(sel.Args[0])
	if addr < 0 {
		h.Printf("Invalid breakpoint address '%v'\n", sel.Args[0])
		return nil
	}

	b := h.debugger.GetBreakpoint(uint16(addr))
	if b == nil {
		h.Printf("No breakpoint was set on $%04X.\n", addr)
		return nil
	}

	b.Disabled = false
	h.Printf("Breakpoint at $%04x enabled.\n", addr)
	return nil
}

func (h *host) OnBreakpointDisable(sel cmd.Selection) error {
	if len(sel.Args) < 1 {
		h.Printf("Syntax: breakpoint disable [addr]\n")
		return nil
	}

	addr := h.ParseAddr(sel.Args[0])
	if addr < 0 {
		h.Printf("Invalid breakpoint address '%v'\n", sel.Args[0])
		return nil
	}

	b := h.debugger.GetBreakpoint(uint16(addr))
	if b == nil {
		h.Printf("No breakpoint was set on $%04X.\n", addr)
		return nil
	}

	b.Disabled = true
	h.Printf("Breakpoint at $%04x disabled.\n", addr)
	return nil
}

func (h *host) OnDataBreakpointList(sel cmd.Selection) error {
	h.Println("Addr  Enabled  Value")
	h.Println("----- -------  -----")
	for _, b := range h.debugger.GetDataBreakpoints() {
		if b.Conditional {
			h.Printf("$%04X %-5v    $%02X\n", b.Address, !b.Disabled, b.Value)
		} else {
			h.Printf("$%04X %-5v    <none>\n", b.Address, !b.Disabled)
		}
	}
	return nil
}

func (h *host) OnDataBreakpointAdd(sel cmd.Selection) error {
	if len(sel.Args) < 1 {
		h.Printf("Syntax: databreakpoint add [addr] [value]\n")
		return nil
	}

	addr := h.ParseAddr(sel.Args[0])
	if addr < 0 {
		h.Printf("Invalid data breakpoint address '%v'\n", sel.Args[0])
		return nil
	}

	if len(sel.Args) > 1 {
		value := h.ParseByte(sel.Args[1])
		if value < 0 {
			h.Printf("Invalid conditional value '%v'\n", sel.Args[1])
			return nil
		}
		h.debugger.AddConditionalDataBreakpoint(uint16(addr), byte(value))
		h.Printf("Conditional data Breakpoint added at $%04x for value $%02X.\n", addr, value)
	} else {
		h.debugger.AddDataBreakpoint(uint16(addr))
		h.Printf("Data breakpoint added at $%04x.\n", addr)
	}

	return nil
}

func (h *host) OnDataBreakpointRemove(sel cmd.Selection) error {
	if len(sel.Args) < 1 {
		h.Printf("Syntax: databreakpoint remove [addr]\n")
		return nil
	}

	addr := h.ParseAddr(sel.Args[0])
	if addr < 0 {
		h.Printf("Invalid data breakpoint address '%v'\n", sel.Args[0])
		return nil
	}

	if h.debugger.GetDataBreakpoint(uint16(addr)) == nil {
		h.Printf("No data breakpoint was set on $%04X.\n", addr)
		return nil
	}

	h.debugger.RemoveDataBreakpoint(uint16(addr))
	h.Printf("Data breakpoint at $%04x removed.\n", addr)
	return nil
}

func (h *host) OnDataBreakpointEnable(sel cmd.Selection) error {
	if len(sel.Args) < 1 {
		h.Printf("Syntax: databreakpoint enable [addr]\n")
		return nil
	}

	addr := h.ParseAddr(sel.Args[0])
	if addr < 0 {
		h.Printf("Invalid data breakpoint address '%v'\n", sel.Args[0])
		return nil
	}

	b := h.debugger.GetDataBreakpoint(uint16(addr))
	if b == nil {
		h.Printf("No data breakpoint was set on $%04X.\n", addr)
		return nil
	}

	b.Disabled = false
	h.Printf("Data breakpoint at $%04x enabled.\n", addr)
	return nil
}

func (h *host) OnDataBreakpointDisable(sel cmd.Selection) error {
	if len(sel.Args) < 1 {
		h.Printf("Syntax: databreakpoint disable [addr]\n")
		return nil
	}

	addr := h.ParseAddr(sel.Args[0])
	if addr < 0 {
		h.Printf("Invalid data breakpoint address '%v'\n", sel.Args[0])
		return nil
	}

	b := h.debugger.GetDataBreakpoint(uint16(addr))
	if b == nil {
		h.Printf("No data breakpoint was set on $%04X.\n", addr)
		return nil
	}

	b.Disabled = true
	h.Printf("Data breakpoint at $%04x disabled.\n", addr)
	return nil
}

func (h *host) OnQuit(sel cmd.Selection) error {
	return errors.New("Exiting program")
}

func (h *host) Load(filename string, addr int) (origin uint16, err error) {
	filename, err = filepath.Abs(filename)
	if err != nil {
		h.Printf("Failed to open '%s': %v\n", filepath.Base(filename), err)
		return 0, nil
	}

	file, err := os.Open(filename)
	if err != nil {
		h.Printf("Failed to open '%s': %v\n", filepath.Base(filename), err)
		return 0, nil
	}
	defer file.Close()

	b, err := ioutil.ReadAll(file)
	if err != nil {
		h.Printf("Failed to read '%s': %v\n", filepath.Base(filename), err)
		return 0, nil
	}

	file.Close()

	code := b
	if len(b) >= 6 && string(b[:4]) == signature {
		if addr == -1 {
			addr = int(b[4]) | int(b[5])<<8
		}
		code = b[6:]
	}
	if addr == -1 {
		h.Printf("File '%s' has no signature and requires an address\n", filepath.Base(filename))
		return 0, nil
	}
	if addr+len(code) > 0x10000 {
		h.Printf("File '%s' exceeded 64K memory bounds\n", filepath.Base(filename))
		return 0, nil
	}

	origin = uint16(addr)
	cpu := h.cpu
	cpu.Mem.StoreBytes(origin, code)
	h.Printf("Loaded '%s' to $%04X..$%04X\n", filepath.Base(filename), origin, addr+len(code)-1)

	ext := filepath.Ext(filename)
	filePrefix := filename[:len(filename)-len(ext)]
	filename = filePrefix + ".map"

	file, err = os.Open(filename)
	if err == nil {
		_, err = h.sourceMap.ReadFrom(file)
		if err != nil {
			h.Printf("Failed to read '%s': %v\n", filepath.Base(filename), err)
		} else {
			h.Printf("Loaded '%s' source map\n", filepath.Base(filename))
		}
	}

	file.Close()

	cpu.SetPC(origin)
	return origin, nil
}

func (h *host) Step() {
	h.cpu.Step()
}

func (h *host) StepOver() {
	cpu := h.cpu

	// JSR instructions need to be handled specially.
	inst := cpu.GetInstruction(cpu.Reg.PC)
	if inst.Name != "JSR" {
		h.cpu.Step()
		return
	}

	// Place a step-over breakpoint on the instruction following the JSR.
	// Either modify an already existing breakpoint on that instrution, or
	// create a temporary one.
	next := h.cpu.Reg.PC + uint16(inst.Length)
	tmpBreakpointCreated := false
	b := h.debugger.GetBreakpoint(next)
	if b == nil {
		b = h.debugger.AddBreakpoint(next)
		tmpBreakpointCreated = true
	}
	b.StepOver = true

	// Run until interrupted.
	for h.state == stateRunning {
		h.Step()
	}
	b.StepOver = false

	// If we were interrupted by the temporary step-over breakpoint,
	// then continue as normal.
	if h.state == stateStepOverBreakpoint {
		h.state = stateRunning
	}

	// Remove the temporarily created breakpoint.
	if tmpBreakpointCreated {
		h.debugger.RemoveBreakpoint(next)
	}
}

func (h *host) OnBreakpoint(cpu *go6502.CPU, b *go6502.Breakpoint) {
	if b.StepOver {
		h.state = stateStepOverBreakpoint
	} else {
		h.state = stateBreakpoint
		h.Printf("Breakpoint hit at $%04X.\n", b.Address)
		h.DisplayPC()
	}
}

func (h *host) OnDataBreakpoint(cpu *go6502.CPU, b *go6502.DataBreakpoint) {
	h.Printf("Data breakpoint hit on address $%04X.\n", b.Address)

	h.state = stateBreakpoint

	if cpu.LastPC != cpu.Reg.PC {
		d, _ := h.Disassemble(cpu.LastPC)
		h.Printf("%s\n", d)
	}

	h.DisplayPC()
}

func (h *host) Disassemble(addr uint16) (str string, next uint16) {
	cpu := h.cpu

	var line string
	line, next = disasm.Disassemble(cpu.Mem, addr)

	l := next - addr
	b := h.buf[:l]
	cpu.Mem.LoadBytes(addr, b)

	str = fmt.Sprintf("%04X- %-8s  %-11s", addr, codeString(b[:l]), line)
	return str, next
}

func (h *host) ParseAddr(s string) int {
	for _, e := range h.sourceMap.Exports {
		if e.Label == s {
			return int(e.Addr)
		}
	}

	base := 10
	if startsWith(s, "0x") {
		s, base = s[2:], 16
	} else if startsWith(s, "$") {
		s, base = s[1:], 16
	}

	o, err := strconv.ParseInt(s, base, 32)
	if err != nil || o < 0 || o > 0xffff {
		return -1
	}

	return int(o)
}

func (h *host) ParseByte(s string) int {
	base := 10
	if startsWith(s, "0x") {
		s, base = s[2:], 16
	} else if startsWith(s, "$") {
		s, base = s[1:], 16
	}

	n, err := strconv.ParseInt(s, base, 32)
	if err != nil || n < -128 || n > 255 {
		return -1
	}
	if n < 0 {
		n = 256 + n
	}

	return int(n)
}
