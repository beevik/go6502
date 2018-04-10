// Copyright 2018 Brett Vickers.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package host allows you to create a "host" that emulates a computer system
// with a 6502 CPU, 64K of memory, a built-in assembler, a built-in debugger,
// and other useful tools.
//
// Within the host it is possible to assemble and load machine code into
// memory, debug and step through machine code, measure the number of CPU
// cycles elapsed, set address and data breakpoints, dump the contents of
// memory, disassemble the contents of memory, manipulate CPU registers and
// memory, and evaluate arbitrary expressions.
package host

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/beevik/cmd"
	"github.com/beevik/go6502/asm"
	"github.com/beevik/go6502/cpu"
	"github.com/beevik/go6502/disasm"
)

var cmds *cmd.Tree

func init() {
	// Create a command tree, where the parameter stored with each command is
	// a host callback capable of handling the command.
	cmds = cmd.NewTree("go6502", []cmd.Command{
		{
			Name:     "help",
			Shortcut: "?",
			Data:     (*Host).cmdHelp,
		},
		{
			Name:  "annotate",
			Brief: "Annotate an address",
			Description: "Provide a code annotation at a memory address." +
				" When disassembling code at this address, the annotation will" +
				" be displayed.",
			HelpText: "annotate <address> <string>",
			Data:     (*Host).cmdAnnotate,
		},
		{
			Name:     "assemble",
			Shortcut: "a",
			Brief:    "Assemble a file and save the binary",
			Description: "Run the cross-assembler on the specified file," +
				" producing a binary file and source map file if successful.",
			HelpText: "assemble <filename>",
			Data:     (*Host).cmdAssemble,
		},
		{
			Name:     "breakpoint",
			Shortcut: "b",
			Brief:    "Breakpoint commands",
			Subcommands: cmd.NewTree("Breakpoint", []cmd.Command{
				{
					Name:        "list",
					Brief:       "List breakpoints",
					Description: "List all current breakpoints.",
					HelpText:    "breakpoint list",
					Data:        (*Host).cmdBreakpointList,
				},
				{
					Name:  "add",
					Brief: "Add a breakpoint",
					Description: "Add a breakpoint at the specified address." +
						" The breakpoints starts enabled.",
					HelpText: "breakpoint add <address>",
					Data:     (*Host).cmdBreakpointAdd,
				},
				{
					Name:        "remove",
					Brief:       "Remove a breakpoint",
					Description: "Remove a breakpoint at the specified address.",
					HelpText:    "breakpoint remove <address>",
					Data:        (*Host).cmdBreakpointRemove,
				},
				{
					Name:        "enable",
					Brief:       "Enable a breakpoint",
					Description: "Enable a previously added breakpoint.",
					HelpText:    "breakpoint enable <address>",
					Data:        (*Host).cmdBreakpointEnable,
				},
				{
					Name:  "disable",
					Brief: "Disable a breakpoint",
					Description: "Disable a previously added breakpoint. This" +
						" prevents the breakpoint from being hit when running the" +
						" CPU",
					HelpText: "breakpoint disable <address>",
					Data:     (*Host).cmdBreakpointDisable,
				},
			}),
		},
		{
			Name:     "databreakpoint",
			Shortcut: "db",
			Brief:    "Data breakpoint commands",
			Subcommands: cmd.NewTree("Data breakpoint", []cmd.Command{
				{
					Name:        "list",
					Brief:       "List data breakpoints",
					Description: "List all current data breakpoints.",
					HelpText:    "databreakpoint list",
					Data:        (*Host).cmdDataBreakpointList,
				},
				{
					Name:  "add",
					Brief: "Add a data breakpoint",
					Description: "Add a new data breakpoint at the specified" +
						" memory address. When the CPU stores data at this address, the " +
						" breakpoint will stop the CPU. Optionally, a byte " +
						" value may be specified, and the CPU will stop only " +
						" when this value is stored. The data breakpoint starts" +
						" enabled.",
					HelpText: "databreakpoint add <address> [<value>]",
					Data:     (*Host).cmdDataBreakpointAdd,
				},
				{
					Name:  "remove",
					Brief: "Remove a data breakpoint",
					Description: "Remove a previously added data breakpoint at" +
						" the specified memory address.",
					HelpText: "databreakpoint remove <address>",
					Data:     (*Host).cmdDataBreakpointRemove,
				},
				{
					Name:        "enable",
					Brief:       "Enable a data breakpoint",
					Description: "Enable a previously added breakpoint.",
					HelpText:    "databreakpoint enable <address>",
					Data:        (*Host).cmdDataBreakpointEnable,
				},
				{
					Name:        "disable",
					Brief:       "Disable a data breakpoint",
					Description: "Disable a previously added breakpoint.",
					HelpText:    "databreakpoint disable <address>",
					Data:        (*Host).cmdDataBreakpointDisable,
				},
			}),
		},
		{
			Name:     "disassemble",
			Shortcut: "d",
			Brief:    "Disassemble code",
			Description: "Disassemble machine code starting at the requested" +
				" address. The number of instructions to disassemble may be" +
				" specified as an option.",
			HelpText: "disassemble <address> [<count>]",
			Data:     (*Host).cmdDisassemble,
		},
		{
			Name:        "evaluate",
			Shortcut:    "e",
			Brief:       "Evaluate an expression",
			Description: "Evaluate a mathemetical expression.",
			HelpText:    "evaluate <expression>",
			Data:        (*Host).cmdEval,
		},
		{
			Name:  "exports",
			Brief: "List exported addresses",
			Description: "Display a list of all memory addresses exported by" +
				" loaded binary files. Exported addresses are stored in a binary" +
				" file's associated source map file.",
			HelpText: "exports",
			Data:     (*Host).cmdExports,
		},
		{
			Name:  "load",
			Brief: "Load a binary file",
			Description: "Load the contents of a binary file into the emulated" +
				" system's memory. If the file has an associated source map, it" +
				" will be loaded too. If the file contains raw binary data, you must" +
				" specify the address where the data will be loaded.",
			HelpText: "load <filename> [<address>]",
			Data:     (*Host).cmdLoad,
		},
		{
			Name:  "memory",
			Brief: "Memory commands",
			Subcommands: cmd.NewTree("Memory", []cmd.Command{
				{
					Name:  "dump",
					Brief: "Dump memory at address",
					Description: "Dump the contents of memory starting from the" +
						" specified address. The number of bytes to dump may be" +
						" specified as an option.",
					HelpText: "memory <address> [<bytes>]",
					Data:     (*Host).cmdMemoryDump,
				},
			}),
		},
		{
			Name:        "quit",
			Brief:       "Quit the program",
			Description: "Quit the program.",
			HelpText:    "quit",
			Data:        (*Host).cmdQuit,
		},
		{
			Name:     "registers",
			Shortcut: "r",
			Brief:    "Display register contents",
			Description: "Display the current contents of all CPU registers, and" +
				" disassemble the instruction at the current program counter address.",
			HelpText: "registers",
			Data:     (*Host).cmdRegisters,
		},
		{
			Name:  "run",
			Brief: "Run the CPU",
			Description: "Run the CPU until a breakpoint is hit or until the " +
				"user types Ctrl-C.",
			HelpText: "run",
			Data:     (*Host).cmdRun,
		},
		{
			Name:  "set",
			Brief: "Set a configuration variable",
			Description: "Set the value of a configuration variable. Type the set" +
				" command without a variable name or value to display the current" +
				" values of all configuration variables.",
			HelpText: "set <var> <value>",
			Data:     (*Host).cmdSet,
		},
		{
			Name:  "step",
			Brief: "Step the debugger",
			Subcommands: cmd.NewTree("Step", []cmd.Command{
				{
					Name:  "in",
					Brief: "Step into next instruction",
					Description: "Step the CPU by a single instruction. If the" +
						" instruction is a subroutine call, step into the subroutine." +
						" The number of steps may be specified as an option.",
					HelpText: "step in [<count>]",
					Data:     (*Host).cmdStepIn,
				},
				{
					Name:  "over",
					Brief: "Step over next instruction",
					Description: "Step the CPU by a single instruction. If the" +
						" instruction is a subroutine call, step over the subroutine." +
						" The number of steps may be specified as an option.",
					HelpText: "step over [<count>]",
					Data:     (*Host).cmdStepOver,
				},
			}),
		},

		// Aliases for nested commands
		{Name: "ba", Alias: "breakpoint add"},
		{Name: "br", Alias: "breakpoint remove"},
		{Name: "bl", Alias: "breakpoint list"},
		{Name: "be", Alias: "breakpoint enable"},
		{Name: "bd", Alias: "breakpoint disable"},
		{Name: "dbl", Alias: "databreakpoint list"},
		{Name: "dba", Alias: "databreakpoint add"},
		{Name: "dbr", Alias: "databreakpoint remove"},
		{Name: "dbe", Alias: "databreakpoint enable"},
		{Name: "dbd", Alias: "databreakpoint disable"},
		{Name: "m", Alias: "memory dump"},
		{Name: "s", Alias: "step over"},
		{Name: "si", Alias: "step in"},
	})
}

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

// A Host represents a fully emulated 6502 system, 64K of memory, a built-in
// assembler, a built-in debugger, and other useful tools.
type Host struct {
	input       *bufio.Scanner
	output      *bufio.Writer
	interactive bool
	mem         *cpu.FlatMemory
	cpu         *cpu.CPU
	debugger    *cpu.Debugger
	lastCmd     *cmd.Selection
	state       state
	exprParser  *exprParser
	sourceMap   *asm.SourceMap
	settings    *settings
	annotations map[uint16]string
}

// New creates a new 6502 host environment.
func New() *Host {
	h := &Host{
		state:       stateProcessingCommands,
		exprParser:  newExprParser(),
		settings:    newSettings(),
		annotations: make(map[uint16]string),
	}

	// Create the emulated CPU and memory.
	h.mem = cpu.NewFlatMemory()
	h.cpu = cpu.NewCPU(cpu.CMOS, h.mem)

	// Create a CPU debugger and attach it to the CPU.
	h.debugger = cpu.NewDebugger(newDebugHandler(h))
	h.cpu.AttachDebugger(h.debugger)

	return h
}

// RunCommands accepts host commands from a reader and outputs the results
// to a writer. If the commands are interactive, a prompt is displayed while
// the host waits for the the next command to be entered.
func (h *Host) RunCommands(r io.Reader, w io.Writer, interactive bool) {
	h.input = bufio.NewScanner(r)
	h.output = bufio.NewWriter(w)
	h.interactive = interactive

	if interactive {
		h.println()
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

		handler := c.Command.Data.(func(*Host, cmd.Selection) error)
		err = handler(h, c)
		if err != nil {
			break
		}
	}
}

// Break interrupts a running CPU.
func (h *Host) Break() {
	h.println()

	if h.state == stateRunning {
		h.displayPC()
	}
	if h.state == stateProcessingCommands {
		h.prompt()
	}
	h.state = stateProcessingCommands
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

func (h *Host) cmdAnnotate(c cmd.Selection) error {
	if len(c.Args) < 1 {
		h.displayHelpText(c.Command)
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
		h.displayHelpText(c.Command)
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
		h.displayHelpText(c.Command)
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
		h.displayHelpText(c.Command)
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
		h.displayHelpText(c.Command)
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
		h.displayHelpText(c.Command)
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
		h.displayHelpText(c.Command)
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
		h.displayHelpText(c.Command)
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
		h.displayHelpText(c.Command)
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
		h.displayHelpText(c.Command)
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
		h.displayHelpText(c.Command)
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
	switch {
	case len(c.Args) == 0:
		h.displayCommands(cmds)
	default:
		s, err := cmds.Lookup(strings.Join(c.Args, " "))
		if err != nil {
			h.printf("%v\n", err)
		} else {
			switch {
			case s.Command.Subcommands != nil:
				h.displayCommands(s.Command.Subcommands)
			default:
				if s.Command.HelpText != "" {
					h.printf("Syntax: %s\n\n", s.Command.HelpText)
				}
				switch {
				case s.Command.Description != "":
					h.printf("Description:\n%s\n\n", indentWrap(3, s.Command.Description))
				case s.Command.Brief != "":
					h.printf("Description:\n%s.\n\n", indentWrap(3, s.Command.Brief))
				}
			}
		}
	}
	return nil
}

func (h *Host) cmdLoad(c cmd.Selection) error {
	if len(c.Args) < 1 {
		h.displayHelpText(c.Command)
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
		h.displayHelpText(c.Command)
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
	d, _ := h.disassemble(h.cpu.Reg.PC, displayAll)
	h.println(d)
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
		h.println("Variables:")
		h.settings.Display(h.output)

	case 1:
		h.displayHelpText(c.Command)

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

func (h *Host) displayHelpText(c *cmd.Command) {
	if c.HelpText != "" {
		h.printf("Syntax: %s\n", c.HelpText)
	} else {
		h.println("<no help text>")
	}
}

func (h *Host) displayCommands(commands *cmd.Tree) {
	h.printf("%s commands:\n", commands.Title)
	for _, c := range commands.Commands {
		if c.Brief != "" {
			h.printf("    %-15s  %s\n", c.Name, c.Brief)
		}
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

func (h *Host) onBreakpoint(cpu *cpu.CPU, b *cpu.Breakpoint) {
	if b.StepOver {
		h.state = stateStepOverBreakpoint
	} else {
		h.state = stateBreakpoint
		h.printf("Breakpoint hit at $%04X.\n", b.Address)
		h.displayPC()
	}
}

func (h *Host) onDataBreakpoint(cpu *cpu.CPU, b *cpu.DataBreakpoint) {
	h.printf("Data breakpoint hit on address $%04X.\n", b.Address)

	h.state = stateBreakpoint

	if cpu.LastPC != cpu.Reg.PC {
		d, _ := h.disassemble(cpu.LastPC, displayAll)
		h.println(d)
	}

	h.displayPC()
}
