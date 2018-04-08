package host

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/beevik/cmd"
	"github.com/beevik/go6502"
	"github.com/beevik/go6502/asm"
	"github.com/beevik/go6502/disasm"
)

// Create a command tree, where the parameter stored with each command is
// a host callback capable of handling the command.
var cmds = cmd.NewTree("Debugger", []cmd.Command{
	{Name: "help", Shortcut: "?", Param: (*Host).cmdHelp},
	{Name: "annotate", Description: "Annotate an address", Param: (*Host).cmdAnnotate},
	{Name: "assemble", Shortcut: "a", Description: "Assemble a file and save the binary", Param: (*Host).cmdAssemble},
	{Name: "breakpoint", Shortcut: "b", Description: "Breakpoint commands", Subcommands: cmd.NewTree("Breakpoint", []cmd.Command{
		{Name: "help", Shortcut: "?", Param: (*Host).cmdHelp},
		{Name: "list", Description: "List breakpoints", Param: (*Host).cmdBreakpointList},
		{Name: "add", Description: "Add a breakpoint", Param: (*Host).cmdBreakpointAdd},
		{Name: "remove", Description: "Remove a breakpoint", Param: (*Host).cmdBreakpointRemove},
		{Name: "enable", Description: "Enable a breakpoint", Param: (*Host).cmdBreakpointEnable},
		{Name: "disable", Description: "Disable a breakpoint", Param: (*Host).cmdBreakpointDisable},
	})},
	{Name: "databreakpoint", Shortcut: "db", Description: "Data breakpoint commands", Subcommands: cmd.NewTree("Data breakpoint", []cmd.Command{
		{Name: "help", Shortcut: "?", Param: (*Host).cmdHelp},
		{Name: "list", Description: "List data breakpoints", Param: (*Host).cmdDataBreakpointList},
		{Name: "add", Description: "Add a data breakpoint", Param: (*Host).cmdDataBreakpointAdd},
		{Name: "remove", Description: "Remove a data breakpoint", Param: (*Host).cmdDataBreakpointRemove},
		{Name: "enable", Description: "Enable a data breakpoint", Param: (*Host).cmdDataBreakpointEnable},
		{Name: "disable", Description: "Disable a data breakpoint", Param: (*Host).cmdDataBreakpointDisable},
	})},
	{Name: "disassemble", Shortcut: "d", Description: "Disassemble code", Param: (*Host).cmdDisassemble},
	{Name: "evaluate", Shortcut: "e", Description: "Evaluate an expression", Param: (*Host).cmdEval},
	{Name: "exports", Description: "List exported addresses", Param: (*Host).cmdExports},
	{Name: "load", Description: "Load a binary file", Param: (*Host).cmdLoad},
	{Name: "memory", Description: "Memory commands", Subcommands: cmd.NewTree("Memory", []cmd.Command{
		{Name: "help", Shortcut: "?", Param: (*Host).cmdHelp},
		{Name: "dump", Description: "Dump memory starting at address", Param: (*Host).cmdMemoryDump},
	})},
	{Name: "quit", Description: "Quit the program", Param: (*Host).cmdQuit},
	{Name: "registers", Shortcut: "r", Description: "Display register contents", Param: (*Host).cmdRegisters},
	{Name: "run", Description: "Run the CPU", Param: (*Host).cmdRun},
	{Name: "set", Description: "Set a host setting", Param: (*Host).cmdSet},
	{Name: "step", Description: "Step the debugger", Subcommands: cmd.NewTree("Step", []cmd.Command{
		{Name: "help", Shortcut: "?", Param: (*Host).cmdHelp},
		{Name: "in", Description: "Step in to routine", Param: (*Host).cmdStepIn},
		{Name: "over", Description: "Step over a routine", Param: (*Host).cmdStepOver},
	})},

	// Shortcuts to nested commands
	{Name: "ba", Param: (*Host).cmdBreakpointAdd},
	{Name: "br", Param: (*Host).cmdBreakpointRemove},
	{Name: "bl", Param: (*Host).cmdBreakpointList},
	{Name: "be", Param: (*Host).cmdBreakpointEnable},
	{Name: "bd", Param: (*Host).cmdBreakpointDisable},
	{Name: "dbl", Param: (*Host).cmdDataBreakpointList},
	{Name: "dba", Param: (*Host).cmdDataBreakpointAdd},
	{Name: "dbr", Param: (*Host).cmdDataBreakpointRemove},
	{Name: "dbe", Param: (*Host).cmdDataBreakpointEnable},
	{Name: "dbd", Param: (*Host).cmdDataBreakpointDisable},
	{Name: "m", Param: (*Host).cmdMemoryDump},
	{Name: "si", Param: (*Host).cmdStepIn},
	{Name: "s", Param: (*Host).cmdStepOver},
})

type displayFlags uint8

const (
	displayRegisters displayFlags = 1 << iota
	displayCycles
	displayAnnotations

	displayAll = displayRegisters | displayCycles | displayAnnotations
)

type state byte

const (
	stateProcessingCommands state = iota
	stateRunning
	stateBreakpoint
	stateStepOverBreakpoint
)

// A Host represents a fully operational 6502-based system with memory,
// an assembler, a built-in debugger, and other useful tools. With the host it
// is possible to assemble and load machine code into memory, debug and step
// through machine code, set address and data breakpoints, dump the contents
// of emulated memory, disassemble the contents of emulated memory, manipulate
// CPU registers and memory, and evaluate arbitrary expressions.
type Host struct {
	input       *bufio.Scanner
	output      *bufio.Writer
	interactive bool
	mem         *go6502.FlatMemory
	cpu         *go6502.CPU
	debugger    *go6502.Debugger
	handler     *handler
	lastCmd     *cmd.Selection
	exprParser  *exprParser
	sourceMap   *asm.SourceMap
	state       state
	settings    *settings
	annotations map[uint16]string
}

// New creates a new 6502 host environment.
func New() *Host {
	h := &Host{
		mem:         go6502.NewFlatMemory(),
		exprParser:  newExprParser(),
		state:       stateProcessingCommands,
		settings:    newSettings(),
		annotations: make(map[uint16]string),
	}

	// Create the emulated CPU.
	h.cpu = go6502.NewCPU(go6502.CMOS, h.mem)

	// Create a handler for the CPU debugger's notifications.
	h.handler = &handler{h}

	// Create a CPU debugger and attach it to the CPU.
	h.debugger = go6502.NewDebugger(h.handler)
	h.cpu.AttachDebugger(h.debugger)

	return h
}

func (h *Host) write(p []byte) (n int, err error) {
	return h.output.Write(p)
}

func (h *Host) print(args ...interface{}) {
	fmt.Fprint(h.output, args...)
}

func (h *Host) printf(format string, args ...interface{}) {
	fmt.Fprintf(h.output, format, args...)
	h.flush()
}

func (h *Host) println(args ...interface{}) {
	fmt.Fprintln(h.output, args...)
	h.flush()
}

func (h *Host) flush() {
	h.output.Flush()
}

func (h *Host) getLine() (string, error) {
	if h.input.Scan() {
		return h.input.Text(), nil
	}
	if h.input.Err() != nil {
		return "", h.input.Err()
	}
	return "", io.EOF
}

func (h *Host) prompt() {
	if h.interactive {
		h.printf("* ")
		h.flush()
	}
}

func (h *Host) displayPC() {
	if h.interactive {
		d, _ := h.disassemble(h.cpu.Reg.PC, displayAll)
		h.println(d)
	}
}

// RunCommands accepts debugger commands from a reader and outputs the results
// to a writer. If the commands are interactive, a prompt is displayed while
// the host waits for the the next command to be entered.
func (h *Host) RunCommands(r io.Reader, w io.Writer, interactive bool) {
	h.input = bufio.NewScanner(r)
	h.output = bufio.NewWriter(w)
	h.interactive = interactive

	// Handle ctrl-C for interactive command sessions.
	if interactive {
		h.println()
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)
		go h.handleInterrupt(c)
	}

	h.displayPC()

	for {
		h.prompt()

		line, err := h.getLine()
		if err != nil {
			break
		}

		var c cmd.Selection
		if line != "" {
			c, err = cmds.Lookup(line)
			switch {
			case err == cmd.ErrNotFound:
				h.println("Command not found.")
				continue
			case err == cmd.ErrAmbiguous:
				h.println("Command is ambiguous.")
				continue
			case err != nil:
				h.printf("ERROR: %v.\n", err)
				continue
			}
		} else if h.lastCmd != nil {
			c = *h.lastCmd
		}

		if c.Command == nil {
			continue
		}
		h.lastCmd = &c

		handler := c.Command.Param.(func(*Host, cmd.Selection) error)
		err = handler(h, c)
		if err != nil {
			break
		}
	}
}

func (h *Host) cmdAnnotate(c cmd.Selection) error {
	if len(c.Args) < 1 {
		h.println("Syntax: annotate [address] [string]")
		return nil
	}

	addr, err := h.parseExpr(c.Args[0])
	if err != nil {
		h.printf("%v\n", err)
		return nil
	}

	var annotation string
	if len(c.Args) >= 2 {
		annotation = strings.Join(c.Args[1:], " ")
	}

	if annotation == "" {
		delete(h.annotations, addr)
		h.printf("Annotation removed at $%04X.\n", addr)
	} else {
		h.annotations[addr] = annotation
		h.printf("Annotation added at $%04X.\n", addr)
	}

	return nil
}

func (h *Host) cmdAssemble(c cmd.Selection) error {
	if len(c.Args) < 1 {
		h.println("Syntax: assemble [filename]")
		return nil
	}

	filename := c.Args[0]
	if filepath.Ext(filename) == "" {
		filename += ".asm"
	}

	file, err := os.Open(filename)
	if err != nil {
		h.printf("Failed to open '%s': %v\n", filepath.Base(filename), err)
		return nil
	}
	defer file.Close()

	assembly, sourceMap, err := asm.Assemble(file, filename, false)
	if err != nil {
		h.printf("Failed to assemble: %s\n", filepath.Base(filename))
		for _, e := range assembly.Errors {
			h.println(e)
		}
		return nil
	}

	file.Close()

	ext := filepath.Ext(filename)
	filePrefix := filename[0 : len(filename)-len(ext)]
	binFilename := filePrefix + ".bin"
	file, err = os.OpenFile(binFilename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		h.printf("Failed to create '%s': %v\n", filepath.Base(binFilename), err)
		return nil
	}

	_, err = assembly.WriteTo(file)
	if err != nil {
		h.printf("Failed to save '%s': %v\n", filepath.Base(binFilename), err)
		return nil
	}

	file.Close()

	mapFilename := filePrefix + ".map"
	file, err = os.OpenFile(mapFilename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		h.printf("Failed to create '%s': %v\n", filepath.Base(mapFilename), err)
		return nil
	}

	_, err = sourceMap.WriteTo(file)
	if err != nil {
		h.printf("Failed to write '%s': %v\n", filepath.Base(mapFilename), err)
		return nil
	}

	file.Close()

	h.printf("Assembled '%s' to '%s'.\n", filepath.Base(filename), filepath.Base(binFilename))
	return nil
}

func (h *Host) cmdBreakpointList(c cmd.Selection) error {
	h.println("Addr  Enabled")
	h.println("----- -------")
	for _, b := range h.debugger.GetBreakpoints() {
		h.printf("$%04X %v\n", b.Address, !b.Disabled)
	}
	return nil
}

func (h *Host) cmdBreakpointAdd(c cmd.Selection) error {
	if len(c.Args) < 1 {
		h.printf("Syntax: breakpoint add [addr]\n")
		return nil
	}

	addr, err := h.parseExpr(c.Args[0])
	if err != nil {
		h.printf("%v\n", err)
		return nil
	}

	h.debugger.AddBreakpoint(addr)
	h.printf("Breakpoint added at $%04x.\n", addr)
	return nil
}

func (h *Host) cmdBreakpointRemove(c cmd.Selection) error {
	if len(c.Args) < 1 {
		h.printf("Syntax: breakpoint remove [addr]\n")
		return nil
	}

	addr, err := h.parseExpr(c.Args[0])
	if err != nil {
		h.printf("%v\n", err)
		return nil
	}

	if h.debugger.GetBreakpoint(addr) == nil {
		h.printf("No breakpoint was set on $%04X.\n", addr)
		return nil
	}

	h.debugger.RemoveBreakpoint(addr)
	h.printf("Breakpoint at $%04x removed.\n", addr)
	return nil
}

func (h *Host) cmdBreakpointEnable(c cmd.Selection) error {
	if len(c.Args) < 1 {
		h.printf("Syntax: breakpoint enable [addr]\n")
		return nil
	}

	addr, err := h.parseExpr(c.Args[0])
	if err != nil {
		h.printf("%v\n", err)
		return nil
	}

	b := h.debugger.GetBreakpoint(addr)
	if b == nil {
		h.printf("No breakpoint was set on $%04X.\n", addr)
		return nil
	}

	b.Disabled = false
	h.printf("Breakpoint at $%04x enabled.\n", addr)
	return nil
}

func (h *Host) cmdBreakpointDisable(c cmd.Selection) error {
	if len(c.Args) < 1 {
		h.printf("Syntax: breakpoint disable [addr]\n")
		return nil
	}

	addr, err := h.parseExpr(c.Args[0])
	if err != nil {
		h.printf("%v\n", err)
		return nil
	}

	b := h.debugger.GetBreakpoint(addr)
	if b == nil {
		h.printf("No breakpoint was set on $%04X.\n", addr)
		return nil
	}

	b.Disabled = true
	h.printf("Breakpoint at $%04x disabled.\n", addr)
	return nil
}

func (h *Host) cmdDataBreakpointList(c cmd.Selection) error {
	h.println("Addr  Enabled  Value")
	h.println("----- -------  -----")
	for _, b := range h.debugger.GetDataBreakpoints() {
		if b.Conditional {
			h.printf("$%04X %-5v    $%02X\n", b.Address, !b.Disabled, b.Value)
		} else {
			h.printf("$%04X %-5v    <none>\n", b.Address, !b.Disabled)
		}
	}
	return nil
}

func (h *Host) cmdDataBreakpointAdd(c cmd.Selection) error {
	if len(c.Args) < 1 {
		h.printf("Syntax: databreakpoint add [addr] [value]\n")
		return nil
	}

	addr, err := h.parseExpr(c.Args[0])
	if err != nil {
		h.printf("%v\n", err)
		return nil
	}

	if len(c.Args) > 1 {
		value, err := h.parseExpr(c.Args[1])
		if err != nil {
			h.printf("%v\n", err)
			return nil
		}
		h.debugger.AddConditionalDataBreakpoint(addr, byte(value))
		h.printf("Conditional data Breakpoint added at $%04x for value $%02X.\n", addr, value)
	} else {
		h.debugger.AddDataBreakpoint(addr)
		h.printf("Data breakpoint added at $%04x.\n", addr)
	}

	return nil
}

func (h *Host) cmdDataBreakpointRemove(c cmd.Selection) error {
	if len(c.Args) < 1 {
		h.printf("Syntax: databreakpoint remove [addr]\n")
		return nil
	}

	addr, err := h.parseExpr(c.Args[0])
	if err != nil {
		h.printf("%v\n", err)
		return nil
	}

	if h.debugger.GetDataBreakpoint(addr) == nil {
		h.printf("No data breakpoint was set on $%04X.\n", addr)
		return nil
	}

	h.debugger.RemoveDataBreakpoint(addr)
	h.printf("Data breakpoint at $%04x removed.\n", addr)
	return nil
}

func (h *Host) cmdDataBreakpointEnable(c cmd.Selection) error {
	if len(c.Args) < 1 {
		h.printf("Syntax: databreakpoint enable [addr]\n")
		return nil
	}

	addr, err := h.parseExpr(c.Args[0])
	if err != nil {
		h.printf("%v\n", err)
		return nil
	}

	b := h.debugger.GetDataBreakpoint(addr)
	if b == nil {
		h.printf("No data breakpoint was set on $%04X.\n", addr)
		return nil
	}

	b.Disabled = false
	h.printf("Data breakpoint at $%04x enabled.\n", addr)
	return nil
}

func (h *Host) cmdDataBreakpointDisable(c cmd.Selection) error {
	if len(c.Args) < 1 {
		h.printf("Syntax: databreakpoint disable [addr]\n")
		return nil
	}

	addr, err := h.parseExpr(c.Args[0])
	if err != nil {
		h.printf("%v\n", err)
		return nil
	}

	b := h.debugger.GetDataBreakpoint(addr)
	if b == nil {
		h.printf("No data breakpoint was set on $%04X.\n", addr)
		return nil
	}

	b.Disabled = true
	h.printf("Data breakpoint at $%04x disabled.\n", addr)
	return nil
}

func (h *Host) cmdDisassemble(c cmd.Selection) error {
	if len(c.Args) == 0 {
		c.Args = []string{"$"}
	}

	var addr uint16
	if len(c.Args) > 0 {
		switch c.Args[0] {
		case "$":
			addr = h.settings.NextDisasmAddr
			if addr == 0 {
				addr = h.cpu.Reg.PC
			}

		case ".":
			addr = h.cpu.Reg.PC

		default:
			a, err := h.parseExpr(c.Args[0])
			if err != nil {
				h.printf("%v\n", err)
				return nil
			}
			addr = a
		}
	}

	lines := h.settings.DisasmLinesToDisplay
	if len(c.Args) > 1 {
		l, err := h.parseExpr(c.Args[1])
		if err != nil {
			h.printf("%v\n", err)
			return nil
		}
		lines = int(l)
	}

	for i := 0; i < lines; i++ {
		d, next := h.disassemble(addr, displayAnnotations)
		h.println(d)
		addr = next
	}

	h.settings.NextDisasmAddr = addr
	h.lastCmd.Args = []string{"$", fmt.Sprintf("%d", lines)}
	return nil
}

func (h *Host) cmdExports(c cmd.Selection) error {
	if h.sourceMap == nil || len(h.sourceMap.Exports) == 0 {
		h.println("No active exports.")
		return nil
	}
	for _, e := range h.sourceMap.Exports {
		h.printf("%-16s $%04X\n", e.Label, e.Addr)
	}
	return nil
}

func (h *Host) cmdEval(c cmd.Selection) error {
	if len(c.Args) < 1 {
		h.println("Syntax: eval [expression]")
		return nil
	}

	expr := strings.Join(c.Args, " ")
	v, err := h.parseExpr(expr)
	if err != nil {
		h.printf("%v\n", err)
		return nil
	}

	h.printf("$%04X\n", v)
	return nil
}

func (h *Host) cmdHelp(c cmd.Selection) error {
	commands := c.Command.Tree
	h.printf("%s commands:\n", commands.Title)
	for _, c := range commands.Commands {
		if c.Description != "" {
			h.printf("    %-15s  %s\n", c.Name, c.Description)
		}
	}
	return nil
}

func (h *Host) cmdLoad(c cmd.Selection) error {
	if len(c.Args) < 1 {
		h.println("Syntax: load [filename] [addr]")
		return nil
	}

	filename := c.Args[0]
	if filepath.Ext(filename) == "" {
		filename += ".bin"
	}

	loadAddr := -1
	if len(c.Args) >= 2 {
		addr, err := h.parseExpr(c.Args[1])
		if err != nil {
			h.printf("%v\n", err)
			return nil
		}
		loadAddr = int(addr)
	}

	_, err := h.load(filename, loadAddr)
	return err
}

func (h *Host) cmdMemoryDump(c cmd.Selection) error {
	if len(c.Args) < 1 {
		h.println("Syntax: memory dump [addr] [bytes]")
		return nil
	}

	var addr uint16
	if len(c.Args) > 0 {
		switch c.Args[0] {
		case "$":
			addr = h.settings.NextMemDumpAddr
			if addr == 0 {
				addr = h.cpu.Reg.PC
			}

		case ".":
			addr = h.cpu.Reg.PC

		default:
			a, err := h.parseExpr(c.Args[0])
			if err != nil {
				h.printf("%v\n", err)
				return nil
			}
			addr = a
		}
	}

	bytes := uint16(h.settings.MemDumpBytes)
	if len(c.Args) >= 2 {
		var err error
		bytes, err = h.parseExpr(c.Args[1])
		if err != nil {
			h.printf("%v\n", err)
			return nil
		}
	}

	h.dumpMemory(addr, bytes)

	h.settings.NextMemDumpAddr = addr + bytes
	h.lastCmd.Args = []string{"$", fmt.Sprintf("%d", bytes)}
	return nil
}

func (h *Host) cmdQuit(c cmd.Selection) error {
	return errors.New("Exiting program")
}

func (h *Host) cmdRegisters(c cmd.Selection) error {
	reg := disasm.GetRegisterString(&h.cpu.Reg)
	h.printf("%s\n", reg)
	return nil
}

func (h *Host) cmdRun(c cmd.Selection) error {
	if len(c.Args) > 0 {
		pc, err := h.parseExpr(c.Args[0])
		if err != nil {
			h.printf("%v\n", err)
			return nil
		}
		h.cpu.SetPC(pc)
	}

	h.printf("Running from $%04X. Press ctrl-C to break.\n", h.cpu.Reg.PC)

	h.state = stateRunning
	for h.state == stateRunning {
		h.step()
	}
	h.state = stateProcessingCommands

	h.settings.NextDisasmAddr = h.cpu.Reg.PC
	return nil
}

func (h *Host) cmdSet(c cmd.Selection) error {
	switch len(c.Args) {
	case 0:
		h.println("Settings:")
		h.settings.Display(h.output)

	case 1:
		h.println("Syntax: set [name] [value]")

	default:
		key, value := strings.ToLower(c.Args[0]), strings.Join(c.Args[1:], " ")
		v, errV := h.exprParser.Parse(value, h)

		// Setting a register?
		if errV == nil {
			sz := -1
			switch key {
			case "a":
				h.cpu.Reg.A, sz = byte(v), 1
			case "x":
				h.cpu.Reg.X, sz = byte(v), 1
			case "y":
				h.cpu.Reg.Y, sz = byte(v), 1
			case "sp":
				v = 0x0100 | (v & 0xff)
				h.cpu.Reg.SP, sz = byte(v), 2
			case ".":
				key = "pc"
				fallthrough
			case "pc":
				h.cpu.Reg.PC, sz = uint16(v), 2
			case "carry":
				h.cpu.Reg.Carry, sz = intToBool(int(v)), 0
			case "zero":
				h.cpu.Reg.Zero, sz = intToBool(int(v)), 0
			case "decimal":
				h.cpu.Reg.Decimal, sz = intToBool(int(v)), 0
			case "overflow":
				h.cpu.Reg.Overflow, sz = intToBool(int(v)), 0
			case "sign":
				h.cpu.Reg.Sign, sz = intToBool(int(v)), 0
			}

			switch sz {
			case 0:
				h.printf("Register %s set to %v.\n", strings.ToUpper(key), intToBool(int(v)))
				return nil
			case 1:
				h.printf("Register %s set to $%02X.\n", strings.ToUpper(key), byte(v))
				return nil
			case 2:
				h.printf("Register %s set to $%04X.\n", strings.ToUpper(key), uint16(v))
				return nil
			}
		}

		// Setting a debugger setting?
		var err error
		switch h.settings.Kind(key) {
		case reflect.Invalid:
			err = fmt.Errorf("Setting '%s' not found", key)
		case reflect.String:
			err = h.settings.Set(key, value)
		case reflect.Bool:
			var v bool
			v, err = stringToBool(value)
			if err == nil {
				err = h.settings.Set(key, v)
			}
		default:
			err = errV
			if err == nil {
				err = h.settings.Set(key, v)
			}
		}

		if err == nil {
			h.println("Setting updated.")
		} else {
			h.printf("%v\n", err)
		}

		h.onSettingsUpdate()
	}

	return nil
}

func (h *Host) cmdStepIn(c cmd.Selection) error {
	// Parse the number of steps.
	count := 1
	if len(c.Args) > 0 {
		n, err := h.parseExpr(c.Args[0])
		if err == nil {
			count = int(n)
		}
	}

	// Step the CPU count times.
	h.state = stateRunning
	for i := count - 1; i >= 0 && h.state == stateRunning; i-- {
		h.step()
		switch {
		case i == h.settings.StepLinesToDisplay:
			h.println("...")
		case i < h.settings.StepLinesToDisplay:
			h.displayPC()
		}
	}
	h.state = stateProcessingCommands

	h.settings.NextDisasmAddr = h.cpu.Reg.PC
	return nil
}

func (h *Host) cmdStepOver(c cmd.Selection) error {
	// Parse the number of steps.
	count := 1
	if len(c.Args) > 0 {
		n, err := h.parseExpr(c.Args[0])
		if err == nil {
			count = int(n)
		}
	}

	// Step over the next instruction count times.
	h.state = stateRunning
	for i := count - 1; i >= 0 && h.state == stateRunning; i-- {
		h.stepOver()
		switch {
		case i == h.settings.StepLinesToDisplay:
			h.println("...")
		case i < h.settings.StepLinesToDisplay:
			h.displayPC()
		}
	}
	h.state = stateProcessingCommands

	h.settings.NextDisasmAddr = h.cpu.Reg.PC
	return nil
}

func (h *Host) load(filename string, addr int) (origin uint16, err error) {
	filename, err = filepath.Abs(filename)
	if err != nil {
		h.printf("Failed to open '%s': %v\n", filepath.Base(filename), err)
		return 0, nil
	}

	file, err := os.Open(filename)
	if err != nil {
		h.printf("Failed to open '%s': %v\n", filepath.Base(filename), err)
		return 0, nil
	}
	defer file.Close()

	a := &asm.Assembly{}
	_, err = a.ReadFrom(file)
	if err != nil {
		h.printf("Failed to read '%s': %v\n", filepath.Base(filename), err)
		return 0, nil
	}

	file.Close()

	origin = a.Origin
	if origin == 0 {
		if addr == -1 {
			h.printf("File '%s' has no signature and requires an address\n", filepath.Base(filename))
			return 0, nil
		}
		origin = uint16(addr)
	}

	cpu := h.cpu
	cpu.Mem.StoreBytes(origin, a.Code)
	h.printf("Loaded '%s' to $%04X..$%04X\n", filepath.Base(filename), origin, int(origin)+len(a.Code)-1)

	ext := filepath.Ext(filename)
	filePrefix := filename[:len(filename)-len(ext)]
	filename = filePrefix + ".map"

	file, err = os.Open(filename)
	if err == nil {
		h.sourceMap = &asm.SourceMap{}
		_, err = h.sourceMap.ReadFrom(file)
		if err != nil {
			h.printf("Failed to read '%s': %v\n", filepath.Base(filename), err)
		} else {
			h.printf("Loaded '%s' source map\n", filepath.Base(filename))
		}
	}

	file.Close()

	cpu.SetPC(origin)
	return origin, nil
}

func (h *Host) step() {
	h.cpu.Step()
}

func (h *Host) stepOver() {
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
		h.step()
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

func (h *Host) onSettingsUpdate() {
	h.exprParser.hexMode = h.settings.HexMode
}

func (h *Host) parseExpr(expr string) (uint16, error) {
	v, err := h.exprParser.Parse(expr, h)
	if err != nil {
		return 0, err
	}

	if v < 0 {
		v = 0x10000 + v
	}
	return uint16(v), nil
}

func (h *Host) disassemble(addr uint16, flags displayFlags) (str string, next uint16) {
	cpu := h.cpu

	var line string
	line, next = disasm.Disassemble(cpu.Mem, addr)

	l := next - addr
	b := make([]byte, l)
	cpu.Mem.LoadBytes(addr, b)

	str = fmt.Sprintf("%04X-   %-8s    %-15s", addr, codeString(b[:l]), line)

	if (flags & displayRegisters) != 0 {
		str += " " + disasm.GetRegisterString(&h.cpu.Reg)
	}

	if (flags & displayCycles) != 0 {
		str += fmt.Sprintf(" C=%-12d", h.cpu.Cycles)
	}

	if (flags & displayAnnotations) != 0 {
		if anno, ok := h.annotations[addr]; ok {
			str += " ; " + anno
		}
	}

	return str, next
}

func (h *Host) dumpMemory(addr0, bytes uint16) {
	if bytes < 0 {
		return
	}

	addr1 := addr0 + bytes - 1
	if addr1 < addr0 {
		addr1 = 0xffff
	}

	buf := []byte("    -" + strings.Repeat(" ", 35))

	// Don't align display for short dumps.
	if addr1-addr0 < 8 {
		addrToBuf(addr0, buf[0:4])
		for a, c1, c2 := addr0, 6, 32; a <= addr1; a, c1, c2 = a+1, c1+3, c2+1 {
			m := h.cpu.Mem.LoadByte(a)
			byteToBuf(m, buf[c1:c1+2])
			buf[c2] = toPrintableChar(m)
		}
		h.println(string(buf))
		return
	}

	// Align addr0 and addr1 to 8-byte boundaries.
	start := uint32(addr0) & 0xfff8
	stop := (uint32(addr1) + 8) & 0xffff8
	if stop > 0x10000 {
		stop = 0x10000
	}

	a := uint16(start)
	for r := start; r < stop; r += 8 {
		addrToBuf(a, buf[0:4])
		for c1, c2 := 6, 32; c1 < 29; c1, c2, a = c1+3, c2+1, a+1 {
			if a >= addr0 && a <= addr1 {
				m := h.cpu.Mem.LoadByte(a)
				byteToBuf(m, buf[c1:c1+2])
				buf[c2] = toPrintableChar(m)
			} else {
				buf[c1] = ' '
				buf[c1+1] = ' '
				buf[c2] = ' '
			}
		}
		h.println(string(buf))
	}
}

func (h *Host) resolveIdentifier(s string) (int64, error) {
	s = strings.ToLower(s)

	switch s {
	case "a":
		return int64(h.cpu.Reg.A), nil
	case "x":
		return int64(h.cpu.Reg.X), nil
	case "y":
		return int64(h.cpu.Reg.Y), nil
	case "sp":
		return int64(h.cpu.Reg.SP) | 0x0100, nil
	case ".":
		fallthrough
	case "pc":
		return int64(h.cpu.Reg.PC), nil
	}

	for _, e := range h.sourceMap.Exports {
		if strings.ToLower(e.Label) == s {
			return int64(e.Addr), nil
		}
	}

	return 0, fmt.Errorf("identifier '%s' not found", s)
}

func (h *Host) onBreakpoint(cpu *go6502.CPU, b *go6502.Breakpoint) {
	if b.StepOver {
		h.state = stateStepOverBreakpoint
	} else {
		h.state = stateBreakpoint
		h.printf("Breakpoint hit at $%04X.\n", b.Address)
		h.displayPC()
	}
}

func (h *Host) onDataBreakpoint(cpu *go6502.CPU, b *go6502.DataBreakpoint) {
	h.printf("Data breakpoint hit on address $%04X.\n", b.Address)

	h.state = stateBreakpoint

	if cpu.LastPC != cpu.Reg.PC {
		d, _ := h.disassemble(cpu.LastPC, displayAll)
		h.println(d)
	}

	h.displayPC()
}

func (h *Host) handleInterrupt(c chan os.Signal) {
	for {
		<-c
		h.println()

		if h.state == stateProcessingCommands {
			h.prompt()
		}
		h.state = stateProcessingCommands
	}
}
