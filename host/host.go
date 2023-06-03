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
	"hash/crc32"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/beevik/cmd"
	"github.com/beevik/go6502/asm"
	"github.com/beevik/go6502/cpu"
	"github.com/beevik/go6502/disasm"
)

type displayFlags uint8

const (
	displayVerbose displayFlags = 1 << iota
	displayRegisters
	displayCycles
	displayAnnotations

	displayAll = displayRegisters | displayCycles | displayAnnotations
)

type state byte

const (
	stateProcessingCommands state = iota
	stateMiniAssembler
	stateRunning
	stateInterrupted
	stateBreakpoint
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
	miniAddr    uint16
	assembly    []string
	exprParser  *exprParser
	sourceCode  map[string][]string
	sourceMap   *asm.SourceMap
	settings    *settings
	annotations map[uint16]string
}

// New creates a new 6502 host environment.
func New() *Host {
	h := &Host{
		state:       stateProcessingCommands,
		exprParser:  newExprParser(),
		sourceCode:  make(map[string][]string),
		sourceMap:   asm.NewSourceMap(),
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

// AssembleFile assembles a file on disk and stores the result in a compiled
// 'bin' file. A source map file is also produced.
func (h *Host) AssembleFile(filename string) error {
	s := cmd.Selection{
		Command: &cmd.Command{},
		Args:    []string{filename},
	}

	h.output = bufio.NewWriter(os.Stdout)
	h.interactive = true

	return h.cmdAssembleFile(s)
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

		switch h.state {
		case stateProcessingCommands:
			err = h.processCommand(line)
		case stateMiniAssembler:
			err = h.processMiniAssembler(line)
		default:
			panic("invalid state")
		}

		if err != nil {
			break
		}
	}
}

func (h *Host) processCommand(line string) error {
	var c cmd.Selection
	if line != "" {
		var err error
		c, err = cmds.Lookup(line)
		switch {
		case err == cmd.ErrNotFound:
			h.println("Command not found.")
			return nil
		case err == cmd.ErrAmbiguous:
			h.println("Command is ambiguous.")
			return nil
		case err != nil:
			h.printf("ERROR: %v.\n", err)
			return nil
		}
	} else if h.lastCmd != nil {
		c = *h.lastCmd
	}

	if c.Command == nil {
		return nil
	}
	if c.Command.Data == nil && c.Command.Subtree != nil {
		h.displayCommands(c.Command.Subtree, nil)
		return nil
	}

	h.lastCmd = &c

	handler := c.Command.Data.(func(*Host, cmd.Selection) error)
	return handler(h, c)
}

func (h *Host) processMiniAssembler(line string) error {
	line = strings.ToUpper(line)

	fields := strings.Fields(line)
	switch {
	case len(fields) == 0:
		return nil
	case fields[0] == "END":
		return h.assembleInline()
	}

	h.assembly = append(h.assembly, line)
	return nil
}

func (h *Host) assembleInline() error {
	defer func() {
		h.assembly = nil
		h.miniAddr = 0
		h.state = stateProcessingCommands
	}()

	if len(h.assembly) == 0 {
		h.println("No assembly code entered.")
		return nil
	}

	h.println("Assembling inline code...")
	s := strings.Join(h.assembly, "\n")
	a, _, err := asm.Assemble(strings.NewReader(s), "inline", h.output, 0)

	if err != nil {
		for _, e := range a.Errors {
			h.println(e)
		}
		h.println("Assembly failed.")
		return nil
	}

	if int(h.miniAddr)+len(a.Code) > 64*1024 {
		h.println("Assembly failed. Code goes beyond 64K.")
		return nil
	}

	h.mem.StoreBytes(h.miniAddr, a.Code)
	h.sourceMap.ClearRange(int(h.miniAddr), len(a.Code))

	for addr, end := int(h.miniAddr), int(h.miniAddr)+len(a.Code); addr < end; {
		d, next := h.disassemble(uint16(addr), 0)
		h.println(d)
		if next < uint16(addr) {
			break
		}
		addr = int(next)
	}

	h.printf("Code successfully assembled at $%04X.\n", h.miniAddr)
	return nil
}

// Break interrupts a running CPU.
func (h *Host) Break() {
	h.println()

	switch h.state {
	case stateRunning:
		h.state = stateInterrupted

	case stateProcessingCommands:
		h.println("Type 'quit' to exit the application.")
		h.prompt()
		return

	case stateMiniAssembler:
		h.println("Interactive assembly canceled.")
		h.assembly = nil
		h.state = stateProcessingCommands
		h.prompt()
	}
}

func (h *Host) printf(format string, args ...any) {
	fmt.Fprintf(h.output, format, args...)
	h.flush()
}

func (h *Host) println(args ...any) {
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
	if !h.interactive {
		return
	}

	switch h.state {
	case stateProcessingCommands:
		h.printf("* ")
	case stateMiniAssembler:
		h.printf("%2d  ", len(h.assembly)+1)
	}
	h.flush()
}

func (h *Host) displayPC() {
	if h.interactive {
		d, _ := h.disassemble(h.cpu.Reg.PC, displayAll)
		h.println(d)
	}
}

func (h *Host) cmdAnnotate(c cmd.Selection) error {
	if len(c.Args) < 1 {
		h.displayUsage(c.Command)
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

func (h *Host) cmdAssembleFile(c cmd.Selection) error {
	if len(c.Args) < 1 {
		h.displayUsage(c.Command)
		return nil
	}

	filename := c.Args[0]
	if filepath.Ext(filename) == "" {
		filename += ".asm"
	}

	var options asm.Option
	if len(c.Args) > 1 {
		verbose, err := stringToBool(c.Args[1])
		if err != nil {
			h.displayUsage(c.Command)
			return nil
		}
		if verbose {
			options |= asm.Verbose
		}
	}

	file, err := os.Open(filename)
	if err != nil {
		h.printf("Failed to open '%s': %v\n", filepath.Base(filename), err)
		return nil
	}
	defer file.Close()

	assembly, sourceMap, err := asm.Assemble(file, filename, h.output, options)
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

func (h *Host) cmdAssembleInteractive(c cmd.Selection) error {
	if len(c.Args) == 0 {
		h.displayUsage(c.Command)
		return nil
	}

	addr, err := h.parseAddr(c.Args[0], 0)
	if err != nil {
		h.printf("%v\n", err)
		return nil
	}

	h.state = stateMiniAssembler
	h.miniAddr = addr
	h.assembly = nil
	h.lastCmd = nil

	h.println("Enter assembly language instructions.")
	h.println("Type END to assemble, Ctrl-C to cancel.")
	return nil
}

func (h *Host) cmdAssembleMap(c cmd.Selection) error {
	if len(c.Args) < 2 {
		h.displayUsage(c.Command)
		return nil
	}

	addr, err := h.parseAddr(c.Args[1], 0)
	if err != nil {
		h.println("Invalid origin address.")
		return nil
	}

	filename := c.Args[0]
	file, err := os.Open(filename)
	if err != nil {
		if path.Ext(filename) == "" {
			filename = filename + ".bin"
			file, err = os.Open(filename)
		}
		if err != nil {
			h.printf("%v\n", err)
			return nil
		}
	}
	defer file.Close()

	code, err := ioutil.ReadAll(file)
	if err != nil {
		h.printf("%v\n", err)
		return nil
	}
	file.Close()

	sourceMap := asm.NewSourceMap()
	sourceMap.Origin = addr
	sourceMap.Size = uint32(len(code))
	sourceMap.CRC = crc32.ChecksumIEEE(code)

	ext := filepath.Ext(filename)
	filePrefix := filename[:len(filename)-len(ext)]
	filename = filePrefix + ".map"

	file, err = os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		h.printf("%v\n", err)
		return nil
	}

	_, err = sourceMap.WriteTo(file)
	if err != nil {
		h.printf("%v\n", err)
		return nil
	}
	file.Close()

	h.printf("Saved source map '%s'.\n", filename)
	return nil
}

func (h *Host) cmdBreakpointList(c cmd.Selection) error {
	bp := h.debugger.GetBreakpoints()
	if len(bp) == 0 {
		h.println("No breakpoints set.")
		return nil
	}

	disabled := func(b *cpu.Breakpoint) string {
		if b.Disabled {
			return "(disabled)"
		}
		return ""
	}

	h.println("Breakpoints:")
	for _, b := range bp {
		h.printf("   $%04X %s\n", b.Address, disabled(b))
	}
	return nil
}

func (h *Host) cmdBreakpointAdd(c cmd.Selection) error {
	if len(c.Args) < 1 {
		h.displayUsage(c.Command)
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
		h.displayUsage(c.Command)
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
		h.displayUsage(c.Command)
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
		h.displayUsage(c.Command)
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
	bp := h.debugger.GetDataBreakpoints()
	if len(bp) == 0 {
		h.println("No data breakpoints set.")
		return nil
	}

	disabled := func(d *cpu.DataBreakpoint) string {
		if d.Disabled {
			return "(disabled)"
		}
		return ""
	}

	h.println("Data breakpoints:")
	for _, b := range h.debugger.GetDataBreakpoints() {
		if b.Conditional {
			h.printf("   $%04X on value $%02X %s\n", b.Address, b.Value, disabled(b))
		} else {
			h.printf("   $%04X %s\n", b.Address, disabled(b))
		}
	}
	return nil
}

func (h *Host) cmdDataBreakpointAdd(c cmd.Selection) error {
	if len(c.Args) < 1 {
		h.displayUsage(c.Command)
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
		h.displayUsage(c.Command)
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
		h.displayUsage(c.Command)
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
		h.displayUsage(c.Command)
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

	addr, err := h.parseAddr(c.Args[0], h.settings.NextDisasmAddr)
	if err != nil {
		h.printf("%v\n", err)
		return nil
	}

	lines := h.settings.DisasmLines
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
	if len(h.sourceMap.Exports) == 0 {
		h.println("No active exports.")
		return nil
	}

	h.println("Exported addresses:")
	for _, e := range h.sourceMap.Exports {
		h.printf("   %-16s $%04X\n", e.Label, e.Address)
	}
	return nil
}

func (h *Host) cmdEvaluate(c cmd.Selection) error {
	if len(c.Args) < 1 {
		h.displayUsage(c.Command)
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

func (h *Host) cmdExecute(c cmd.Selection) error {
	if len(c.Args) < 1 {
		h.displayUsage(c.Command)
		return nil
	}

	file, err := os.Open(c.Args[0])
	if err != nil {
		h.printf("%v\n", err)
		return nil
	}
	defer file.Close()

	input, interactive := h.input, h.interactive

	h.RunCommands(file, h.output, false)

	h.input, h.interactive = input, interactive

	return nil
}

func (h *Host) cmdHelp(c cmd.Selection) error {
	switch {
	case len(c.Args) == 0:
		h.displayCommands(cmds, nil)
	default:
		s, err := cmds.Lookup(strings.Join(c.Args, " "))
		if err != nil {
			h.printf("%v\n", err)
		} else {
			switch {
			case s.Command.Subtree != nil:
				h.displayCommands(s.Command.Subtree, s.Command)
			default:
				if s.Command.Usage != "" {
					h.printf("Usage: %s\n\n", s.Command.Usage)
				}
				switch {
				case s.Command.Description != "":
					h.printf("Description:\n%s\n\n", indentWrap(3, s.Command.Description))
				case s.Command.Brief != "":
					h.printf("Description:\n%s.\n\n", indentWrap(3, s.Command.Brief))
				}
				if s.Command.Shortcuts != nil {
					switch {
					case len(s.Command.Shortcuts) > 1:
						h.printf("Shortcuts: %s\n\n", strings.Join(s.Command.Shortcuts, ", "))
					default:
						h.printf("Shortcut: %s\n\n", s.Command.Shortcuts[0])
					}
				}
			}
		}
	}
	return nil
}

func (h *Host) cmdList(c cmd.Selection) error {
	if len(c.Args) == 0 {
		c.Args = []string{"$"}
	}

	// Parse the address.
	addr, err := h.parseAddr(c.Args[0], h.settings.NextSourceAddr)
	if err != nil {
		h.printf("%v\n", err)
		return nil
	}

	// Parse the number of lines to display.
	nl := h.settings.SourceLines
	if len(c.Args) > 1 {
		v, err := h.parseExpr(strings.Join(c.Args[1:], " "))
		if err != nil {
			h.printf("%v\n", err)
			return nil
		}
		nl = int(v)
	}

	// Keep track of the last displayed line number for each source file.
	last := make(map[string]int)

	b := make([]byte, 3)

	// Search around the address for an address with source code, and attempt
	// to display the first source code line.
	for _, o := range []int{0, -1, -2, +1, +2, -3, +3, -4, +4, -5, +5} {
		orig := uint16(int(addr) + o)

		fn, li, err := h.sourceMap.Find(int(orig))
		if err != nil {
			continue
		}

		lines, err := h.getSourceLines(fn)
		if err != nil {
			continue
		}

		_, addr = disasm.Disassemble(h.cpu.Mem, orig)
		cn := addr - orig
		h.cpu.Mem.LoadBytes(orig, b[:cn])
		cs := codeString(b[:cn])
		h.printf("%04X- %-8s\t%s\n", orig, cs, lines[li-1])

		last[fn] = li
		break
	}

	if len(last) == 0 {
		h.printf("No source code found for address $%04X.\n", addr)
		return nil
	}

	// Display remaining source code lines.
	for i := 0; i < nl-1; i++ {
		orig := addr

		fn, li, err := h.sourceMap.Find(int(orig))
		if err != nil {
			continue
		}

		lines, err := h.getSourceLines(fn)
		if err != nil {
			last[fn] = li
			continue
		}

		_, addr = disasm.Disassemble(h.cpu.Mem, orig)
		cn := addr - orig
		h.cpu.Mem.LoadBytes(orig, b[:cn])
		cs := codeString(b[:cn])

		l, ok := last[fn]
		if !ok {
			l = li - 1
		}

		for i, j := l, min(li, len(lines)); i < j; i++ {
			var c string
			if i == j-1 {
				c = cs
			}
			h.printf("%04X- %-8s\t%s\n", orig, c, lines[i])
		}

		last[fn] = li
	}

	h.settings.NextSourceAddr = addr
	h.lastCmd.Args = []string{"$", fmt.Sprintf("%d", nl)}
	return nil
}

func (h *Host) cmdLoad(c cmd.Selection) error {
	if len(c.Args) < 1 {
		h.displayUsage(c.Command)
		return nil
	}

	filename := c.Args[0]

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
	if len(c.Args) == 0 {
		c.Args = []string{"$"}
	}

	var addr uint16
	if len(c.Args) > 0 {
		var err error
		addr, err = h.parseAddr(c.Args[0], h.settings.NextMemDumpAddr)
		if err != nil {
			h.printf("%v\n", err)
			return nil
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

func (h *Host) cmdMemorySet(c cmd.Selection) error {
	if len(c.Args) < 2 {
		h.displayUsage(c.Command)
		return nil
	}

	addr, err := h.parseAddr(c.Args[0], h.settings.NextMemDumpAddr)
	if err != nil {
		h.printf("%v\n", err)
		return nil
	}

	for i := 1; i < len(c.Args); i++ {
		v, err := h.parseExpr(c.Args[i])
		if err != nil {
			h.printf("%v\n", err)
			return nil
		}
		h.mem.StoreByte(addr, byte(v))
		addr++
	}

	return nil
}

func (h *Host) cmdMemoryCopy(c cmd.Selection) error {
	if len(c.Args) < 3 {
		h.displayUsage(c.Command)
		return nil
	}

	dst, err := h.parseAddr(c.Args[0], 0)
	if err != nil {
		h.printf("%v\n", err)
		return nil
	}

	src0, err := h.parseAddr(c.Args[1], 0)
	if err != nil {
		h.printf("%v\n", err)
		return nil
	}

	src1, err := h.parseAddr(c.Args[2], 0)
	if err != nil {
		h.printf("%v\n", err)
		return nil
	}

	if src1 < src0 {
		h.println("Source-end address must be greater than source-begin address.")
		return nil
	}

	b := make([]byte, src1-src0+1)
	h.cpu.Mem.LoadBytes(src0, b)
	h.cpu.Mem.StoreBytes(dst, b)
	h.printf("%d bytes copied from $%04X to $%04X.\n", len(b), src0, dst)
	return nil
}

func (h *Host) cmdQuit(c cmd.Selection) error {
	return errors.New("exiting program")
}

func (h *Host) cmdRegister(c cmd.Selection) error {
	if len(c.Args) == 0 {
		h.printf("%s C=%d\n", disasm.GetRegisterString(&h.cpu.Reg), h.cpu.Cycles)
		return nil
	}

	if len(c.Args) == 1 {
		h.displayUsage(c.Command)
		return nil
	}

	key, value := strings.ToUpper(c.Args[0]), strings.Join(c.Args[1:], " ")

	var flag *bool
	var flagName string
	switch {
	case key == "N" || key == "SIGN":
		flag, flagName = &h.cpu.Reg.Sign, "SIGN"
	case key == "Z" || key == "ZERO":
		flag, flagName = &h.cpu.Reg.Zero, "ZERO"
	case key == "C" || key == "CARRY":
		flag, flagName = &h.cpu.Reg.Carry, "CARRY"
	case key == "I" || key == "INTERRUPT_DISABLE":
		flag, flagName = &h.cpu.Reg.InterruptDisable, "INTERRUPT_DISABLE"
	case key == "D" || key == "DECIMAL":
		flag, flagName = &h.cpu.Reg.Decimal, "DECIMAL"
	case key == "V" || key == "OVERFLOW":
		flag, flagName = &h.cpu.Reg.Overflow, "OVERFLOW"
	}

	if flag != nil {
		v, err := stringToBool(value)
		if err != nil {
			h.printf("%v\n", err)
			return nil
		}

		*flag = v
		h.printf("Status flag %s set to %v.\n", flagName, v)
	} else {
		v, err := h.exprParser.Parse(value, h)
		if err != nil {
			h.printf("%v\n", err)
			return nil
		}

		var sz int
		switch key {
		case "A":
			h.cpu.Reg.A, sz = byte(v), 1
		case "X":
			h.cpu.Reg.X, sz = byte(v), 1
		case "Y":
			h.cpu.Reg.Y, sz = byte(v), 1
		case "SP":
			v = 0x0100 | (v & 0xff)
			h.cpu.Reg.SP, sz = byte(v), 2
		case ".":
			key = "pc"
			fallthrough
		case "PC":
			h.cpu.Reg.PC, sz = uint16(v), 2
		default:
			h.printf("Unknown register '%s'.\n", key)
			return nil
		}

		switch sz {
		case 1:
			h.printf("Register %s set to $%02X.\n", strings.ToUpper(key), byte(v))
		case 2:
			h.printf("Register %s set to $%04X.\n", strings.ToUpper(key), uint16(v))
		}
	}

	if h.interactive {
		h.printf("%s C=%d\n", disasm.GetRegisterString(&h.cpu.Reg), h.cpu.Cycles)
	}

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

	if h.state == stateInterrupted {
		h.displayPC()
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
		h.displayUsage(c.Command)

	default:
		key, value := strings.ToLower(c.Args[0]), strings.Join(c.Args[1:], " ")
		v, errV := h.exprParser.Parse(value, h)

		// Setting a debugger setting?
		var err error
		switch h.settings.Kind(key) {
		case reflect.Invalid:
			err = fmt.Errorf("setting '%s' not found", key)
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

	if count == 0 {
		h.displayPC()
	} else {
		h.state = stateRunning
		for i := count - 1; i >= 0 && h.state == stateRunning; i-- {
			h.step()
			switch {
			case i == h.settings.MaxStepLines:
				h.println("...")
			case i < h.settings.MaxStepLines:
				h.displayPC()
			}
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

	if count == 0 {
		h.displayPC()
	} else {
		h.state = stateRunning
		for i := count - 1; i >= 0 && h.state == stateRunning; i-- {
			h.stepOver()
			switch {
			case i == h.settings.MaxStepLines:
				h.println("...")
			case i < h.settings.MaxStepLines:
				h.displayPC()
			}
		}
	}

	h.state = stateProcessingCommands
	h.settings.NextDisasmAddr = h.cpu.Reg.PC
	return nil
}

func (h *Host) load(filename string, addr int) (origin uint16, err error) {
	filename, err = filepath.Abs(filename)
	basefile := filepath.Base(filename)
	if err != nil {
		h.printf("%v\n", err)
		return 0, nil
	}

	file, err := os.Open(filename)
	if err != nil {
		if path.Ext(filename) == "" {
			filename = filename + ".bin"
			file, err = os.Open(filename)
		}
		if err != nil {
			h.printf("%v\n", err)
			return 0, nil
		}
	}
	defer file.Close()

	a := &asm.Assembly{}
	_, err = a.ReadFrom(file)
	if err != nil {
		h.printf("%v\n", err)
		return 0, nil
	}

	file.Close()

	ext := filepath.Ext(filename)
	filePrefix := filename[:len(filename)-len(ext)]
	filename = filePrefix + ".map"

	// Try loading a source map file if it exists.
	file, err = os.Open(filename)
	var sourceMap *asm.SourceMap
	if err == nil {
		sourceMap = asm.NewSourceMap()
		_, err = sourceMap.ReadFrom(file)
		if err != nil {
			h.printf("Failed to read source map '%s': %v\n", filepath.Base(filename), err)
			sourceMap = nil
		} else {
			if crc32.ChecksumIEEE(a.Code) == sourceMap.CRC {
				h.printf("Loaded source map from '%s'.\n", filepath.Base(filename))
				if len(h.sourceMap.Files) == 0 {
					h.sourceMap = sourceMap
				} else {
					h.sourceMap.Merge(sourceMap)
				}
			} else {
				h.printf("Source map CRC doesn't match for '%s'.\n", basefile)
				sourceMap = nil
			}
		}
	}
	file.Close()

	// Set the origin address using either the value from the source map file
	// or the value passed to this function.
	originSet := false
	if sourceMap != nil {
		origin, originSet = sourceMap.Origin, true
	}
	if addr != -1 {
		origin, originSet = uint16(addr), true
	}
	if !originSet {
		h.printf("File '%s' has no source map and requires an origin address.\n", basefile)
		return 0, nil
	}

	// Copy the code to the CPU memory and adjust the program counter.
	h.cpu.Mem.StoreBytes(origin, a.Code)
	h.printf("Loaded '%s' to $%04X..$%04X.\n", basefile, origin, int(origin)+len(a.Code)-1)

	h.settings.NextDisasmAddr = origin
	return origin, nil
}

func (h *Host) step() {
	h.cpu.Step()
}

func (h *Host) stepOver() {
	cpu := h.cpu

	inst := cpu.GetInstruction(cpu.Reg.PC)
	nextaddr := cpu.Reg.PC + uint16(inst.Length)
	cpu.Step()

	// If a JSR was just stepped, keep stepping until the return address
	// is hit or a corresponding RTS is stepped.
	if inst.Name == "JSR" {
		count := 1
	loop:
		for h.state == stateRunning && cpu.Reg.PC != nextaddr {
			inst := cpu.GetInstruction(cpu.Reg.PC)
			cpu.Step()
			switch inst.Name {
			case "JSR":
				count++
			case "RTS":
				count--
				if count == 0 {
					break loop
				}
			}
		}
	}
}

func (h *Host) onSettingsUpdate() {
	h.exprParser.hexMode = h.settings.HexMode
}

func (h *Host) parseAddr(s string, next uint16) (uint16, error) {
	switch s {
	case "$":
		if next != 0 {
			return next, nil
		}
		fallthrough

	case ".":
		return h.cpu.Reg.PC, nil

	default:
		return h.parseExpr(s)
	}
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

	if h.settings.CompactMode && (flags&displayVerbose) == 0 {
		str = fmt.Sprintf("%04X- %-8s  %-15s", addr, codeString(b[:l]), line)
		if (flags & displayRegisters) != 0 {
			str = disasm.GetCompactRegisterString(&h.cpu.Reg) + "  " + str
		}
	} else {
		str = fmt.Sprintf("%04X-   %-8s    %-15s", addr, codeString(b[:l]), line)

		if (flags & displayRegisters) != 0 {
			str += " " + disasm.GetRegisterString(&h.cpu.Reg)
		}
		if (flags & displayCycles) != 0 {
			str += fmt.Sprintf(" C=%d", h.cpu.Cycles)
		}
	}

	if (flags & displayAnnotations) != 0 {
		if anno, ok := h.annotations[addr]; ok {
			str += " ; " + anno
		}
	}

	return str, next
}

func (h *Host) dumpMemory(addr0, bytes uint16) {
	addr1 := addr0 + bytes - 1
	if addr1 < addr0 {
		addr1 = 0xffff
	}

	buf := []byte("    -" + strings.Repeat(" ", 35))

	// Don't align display for short dumps.
	if addr1-addr0 < 8 {
		addrToBuf(addr0, buf[0:4])
		for a, c1, c2 := uint32(addr0), 6, 32; a <= uint32(addr1); a, c1, c2 = a+1, c1+3, c2+1 {
			m := h.cpu.Mem.LoadByte(uint16(a))
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

func (h *Host) displayUsage(c *cmd.Command) {
	if c.Usage != "" {
		h.printf("Usage: %s\n", c.Usage)
	}
}

func (h *Host) displayCommands(commands *cmd.Tree, c *cmd.Command) {
	h.printf("%s commands:\n", commands.Title)
	for _, c := range commands.Commands {
		if c.Brief != "" {
			h.printf("    %-15s  %s\n", c.Name, c.Brief)
		}
	}
	h.println()

	if c != nil && c.Shortcuts != nil && len(c.Shortcuts) > 0 {
		switch {
		case len(c.Shortcuts) > 1:
			h.printf("Shortcuts: %s\n\n", strings.Join(c.Shortcuts, ", "))
		default:
			h.printf("Shortcut: %s\n\n", c.Shortcuts[0])
		}
	}
}

func (h *Host) getSourceLines(filename string) (lines []string, err error) {
	var ok bool
	if lines, ok = h.sourceCode[filename]; ok {
		return lines, nil
	}

	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if lines == nil {
		return lines, nil
	}

	h.sourceCode[filename] = lines
	return lines, nil
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
			return int64(e.Address), nil
		}
	}

	return 0, fmt.Errorf("identifier '%s' not found", s)
}

func (h *Host) onBreakpoint(cpu *cpu.CPU, b *cpu.Breakpoint) {
	h.state = stateBreakpoint
	h.printf("Breakpoint hit at $%04X.\n", b.Address)
	h.displayPC()
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
