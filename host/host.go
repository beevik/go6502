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
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"github.com/beevik/cmd"
	"github.com/beevik/go6502/asm"
	"github.com/beevik/go6502/cpu"
	"github.com/beevik/go6502/disasm"
	"github.com/beevik/go6502/term"
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
	input          *bufio.Scanner
	output         *bufio.Writer
	rawMode        bool
	rawTerminal    *term.Terminal
	rawInputState  *term.State
	rawOutputState *term.State
	theme          *disasm.Theme
	prompt         string
	mem            *cpu.FlatMemory
	cpu            *cpu.CPU
	debugger       *cpu.Debugger
	lastCmd        *cmd.Command
	lastArgs       []string
	lastLine       string
	state          state
	miniAddr       uint16
	assembly       []string
	exprParser     *exprParser
	sourceCode     map[string][]string
	sourceMap      *asm.SourceMap
	settings       *settings
	annotations    map[uint16]string
}

// IoState represents the state of the host's I/O subsystem. It is returned
// by calls to EnableRawMode and EnableProcessedMode.
type IoState struct {
	input   *bufio.Scanner
	output  *bufio.Writer
	rawMode bool
}

// New creates a new 6502 host environment.
func New() *Host {
	console := struct {
		io.Reader
		io.Writer
	}{
		os.Stdin,
		os.Stdout,
	}

	theme := &disasm.Theme{
		Addr:       term.BrightWhite,
		Code:       term.White,
		Inst:       term.BrightCyan,
		Operand:    term.Green,
		RegName:    term.BrightYellow,
		RegValue:   term.BrightGreen,
		RegEqual:   term.White,
		Source:     term.BrightGreen,
		Annotation: term.BrightYellow,
		Reset:      term.Reset,
	}

	h := &Host{
		rawMode:     false,
		rawTerminal: term.NewTerminal(console, ""),
		theme:       theme,
		exprParser:  newExprParser(),
		sourceCode:  make(map[string][]string),
		sourceMap:   asm.NewSourceMap(),
		settings:    newSettings(),
		annotations: make(map[uint16]string),
	}

	// Set up raw terminal callbacks.
	h.rawTerminal.AutoCompleteCallback = h.autocomplete
	h.rawTerminal.HistoryTestCallback = h.historyTest

	// Initialize host state.
	h.setState(stateProcessingCommands)

	// Create the emulated CPU and memory.
	h.mem = cpu.NewFlatMemory()
	h.cpu = cpu.NewCPU(cpu.CMOS, h.mem)

	// Create a CPU debugger and attach it to the CPU.
	h.debugger = cpu.NewDebugger(h)
	h.cpu.AttachDebugger(h.debugger)

	// Attach this host as a CPU BRK handler.
	h.cpu.AttachBrkHandler(h)

	return h
}

// Cleanup cleans up all resources initialized by the call to New().
func (h *Host) Cleanup() {
	h.disableRawMode()
}

func (h *Host) enableRawMode() {
	if !h.rawMode {
		var err error
		h.rawInputState, err = term.MakeRawInput(int(os.Stdin.Fd()))
		if err != nil {
			panic(err)
		}

		h.rawOutputState, err = term.MakeRawOutput(int(os.Stdout.Fd()))
		if err != nil {
			term.Restore(int(os.Stdin.Fd()), h.rawInputState)
			panic(err)
		}
		h.rawMode = true
	}
}

func (h *Host) disableRawMode() {
	if h.rawMode {
		if h.rawOutputState != nil {
			term.Restore(int(os.Stdout.Fd()), h.rawOutputState)
			h.rawOutputState = nil
		}
		if h.rawInputState != nil {
			term.Restore(int(os.Stdin.Fd()), h.rawInputState)
			h.rawInputState = nil
		}
		h.rawMode = false
	}
}

func (h *Host) autocomplete(line string, pos int, key rune) (newLine string, newPos int, ok bool) {
	if key == '\t' {
		matches := cmds.Autocomplete(line[:pos])

		// Exactly one match, so use it to autocomplete.
		if len(matches) == 1 {
			match := matches[0] + " "
			return match, len(match), true
		}

		// More than one match, so display all of them and autocomplete the
		// matches' shared prefix.
		if len(matches) > 1 {
			// Echo the typed line before displaying matches.
			fmt.Fprintln(h, h.prompt+line)

			prefix := sharedPrefix(matches)

			// Modify the list of matches by stripping everything before the
			// final space. Also, calculate a display width for each match for
			// a cleaner looking output.
			width := 8
			for i, m := range matches {
				wsIndex := strings.LastIndex(m, " ")
				if wsIndex != -1 {
					matches[i] = m[wsIndex+1:]
				}
				l := len(m) + 2
				if l > width {
					width = l
				}
			}

			// Display all possible matches.
			nr := 78 / width
			for i := 0; i < len(matches); i++ {
				fmt.Fprint(h, matches[i]+strings.Repeat(" ", width-len(matches[i])))
				if i%nr == nr-1 && i != len(matches)-1 {
					fmt.Fprintln(h)
				}
			}
			fmt.Fprintln(h)

			return prefix, len(prefix), true
		}
	}

	return "", 0, false
}

func sharedPrefix(strings []string) string {
	helper := func(a, b string) string {
		l := min(len(a), len(b))
		for i := 0; i < l; i++ {
			if a[i] != b[i] {
				return a[:i]
			}
		}
		return a[:l]
	}
	if len(strings) == 0 {
		return ""
	}
	result := strings[0]
	for _, s := range strings[1:] {
		result = helper(s, result)
	}
	return result
}

func (h *Host) historyTest(line string) bool {
	if h.state == stateMiniAssembler {
		return false
	}
	return line != "" && line != h.lastLine
}

// Write writes the contents of p into the output device currently assigned
// to the host. It returns the number of bytes written.
func (h *Host) Write(p []byte) (n int, err error) {
	if h.rawMode {
		return h.rawTerminal.Write(p)
	}
	if h.output == nil {
		return len(p), nil
	}
	n, err = h.output.Write(p)
	h.output.Flush()
	return n, err
}

// AssembleFile assembles a file on disk and stores the result in a compiled
// 'bin' file. A source map file is also produced.
func (h *Host) AssembleFile(filename string) error {
	return h.cmdAssembleFile(new(cmd.Command), []string{filename})
}

// EnableRawMode enables the raw interactive console mode. The original I/O
// state is returned so that it may be restored afterwards.
func (h *Host) EnableRawMode() *IoState {
	ioState := &IoState{h.input, h.output, h.rawMode}
	if !h.rawMode {
		h.enableRawMode()
		h.rawMode = true
	}
	return ioState
}

// EnableProcessedMode disables raw mode and enters the processed I/O mode,
// where input is read from the reader r and output is written to the writer
// w. The original I/O state is returned so that it may be restored
// afterwards.
func (h *Host) EnableProcessedMode(r io.Reader, w io.Writer) *IoState {
	ioState := &IoState{h.input, h.output, h.rawMode}
	h.disableRawMode()
	h.input = bufio.NewScanner(r)
	if w == nil {
		h.output = nil
	} else {
		h.output = bufio.NewWriter(w)
	}
	return ioState
}

// RestoreIoState restores a previously saved I/O state.
func (h *Host) RestoreIoState(state *IoState) {
	h.input = state.input
	h.output = state.output
	if state.rawMode {
		h.enableRawMode()
	} else {
		h.disableRawMode()
	}
}

// RunCommands accepts host commands from a reader and outputs the results
// to a writer. If the commands are interactive, a prompt is displayed while
// the host waits for the the next command to be entered.
func (h *Host) RunCommands(interactive bool) {
	if interactive {
		fmt.Fprintln(h)
		h.displayPC()
	}

	for {
		line, err := h.readLine(interactive)

		// ctrl-C?
		if h.rawMode && err == io.EOF {
			h.Break()
			continue
		}

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

		h.lastLine = line
	}
}

func (h *Host) setState(s state) {
	h.state = s
	switch h.state {
	case stateMiniAssembler:
		h.prompt = term.Cyan + "! " + term.Reset
	default:
		h.prompt = term.Green + "* " + term.Reset
	}
	h.rawTerminal.SetPrompt(h.prompt)
}

func (h *Host) processCommand(line string) error {
	var n cmd.Node
	var args []string
	if line != "" {
		var err error
		n, args, err = cmds.Lookup(line)
		switch {
		case err == cmd.ErrNotFound:
			fmt.Fprintln(h, "Command not found.")
			return nil
		case err == cmd.ErrAmbiguous:
			fmt.Fprintln(h, "Command is ambiguous.")
			return nil
		case err != nil:
			fmt.Fprintf(h, "ERROR: %v.\n", err)
			return nil
		}
	} else if h.lastCmd != nil {
		n = h.lastCmd
		args = h.lastArgs
	}

	if st, ok := n.(*cmd.Tree); ok {
		st.DisplayHelp(h)
		h.lastCmd, h.lastArgs = nil, nil
		return nil
	}

	if c, ok := n.(*cmd.Command); ok {
		h.lastCmd, h.lastArgs = c, args
		handler := c.Data.(func(*Host, *cmd.Command, []string) error)
		return handler(h, c, args)
	}

	return nil
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
		h.setState(stateProcessingCommands)
	}()

	if len(h.assembly) == 0 {
		fmt.Fprintln(h, "No assembly code entered.")
		return nil
	}

	fmt.Fprintln(h, "Assembling inline code...")
	s := strings.Join(h.assembly, "\n")
	a, sm, err := asm.Assemble(strings.NewReader(s), "inline", h.miniAddr, h, 0)

	if err != nil {
		for _, e := range a.Errors {
			fmt.Fprintln(h, e)
		}
		fmt.Fprintln(h, "Assembly failed.")
		return nil
	}

	if int(h.miniAddr)+len(a.Code) > 64*1024 {
		fmt.Fprintln(h, "Assembly failed. Code goes beyond 64K.")
		return nil
	}

	h.mem.StoreBytes(h.miniAddr, a.Code)
	h.sourceMap.Merge(sm)

	for addr, end := int(h.miniAddr), int(h.miniAddr)+len(a.Code); addr < end; {
		d, next := disasm.Disassemble(h.cpu, uint16(addr), disasm.ShowBasic, "", h.theme)
		fmt.Fprintln(h, d)
		if next < uint16(addr) {
			break
		}
		addr = int(next)
	}

	fmt.Fprintf(h, "Code successfully assembled at $%04X.\n", h.miniAddr)
	return nil
}

// Break interrupts a running CPU.
func (h *Host) Break() {
	switch h.state {
	case stateRunning:
		h.state = stateInterrupted

	case stateProcessingCommands:
		fmt.Fprintln(h, "Type 'quit' to exit the application.")

	case stateMiniAssembler:
		h.assembly = nil
		h.setState(stateProcessingCommands)
		fmt.Fprintln(h, "Interactive assembly canceled.")
	}
}

func (h *Host) readLine(interactive bool) (string, error) {
	if h.rawMode {
		return h.rawTerminal.ReadLine()
	}
	if h.input == nil {
		return "", errors.New("no input reader assigned")
	}
	if interactive {
		fmt.Fprintf(h, "%s", h.prompt)
	}
	if h.input.Scan() {
		return h.input.Text(), nil
	}
	if h.input.Err() != nil {
		return "", h.input.Err()
	}
	return "", io.EOF
}

func (h *Host) displayPC() {
	d, _ := disasm.Disassemble(h.cpu, h.cpu.Reg.PC, disasm.ShowFull, "", h.theme)
	fmt.Fprintln(h, d)
}

func (h *Host) cmdAnnotate(c *cmd.Command, args []string) error {
	if len(args) < 1 {
		c.DisplayUsage(h)
		return nil
	}

	addr, err := h.parseExpr(args[0])
	if err != nil {
		fmt.Fprintf(h, "%v\n", err)
		return nil
	}

	var annotation string
	if len(args) >= 2 {
		annotation = strings.Join(args[1:], " ")
	}

	if annotation == "" {
		delete(h.annotations, addr)
		fmt.Fprintf(h, "Annotation removed at $%04X.\n", addr)
	} else {
		h.annotations[addr] = annotation
		fmt.Fprintf(h, "Annotation added at $%04X.\n", addr)
	}

	return nil
}

func (h *Host) cmdAssembleFile(c *cmd.Command, args []string) error {
	if len(args) < 1 {
		c.DisplayUsage(h)
		return nil
	}

	path := args[0]
	if filepath.Ext(path) == "" {
		path += ".asm"
	}

	var options asm.Option
	if len(args) > 1 {
		verbose, err := stringToBool(args[1])
		if err != nil {
			c.DisplayUsage(h)
			return nil
		}
		if verbose {
			options |= asm.Verbose
		}
	}

	err := asm.AssembleFile(path, options, h)
	if err != nil {
		fmt.Fprintf(h, "Failed to assemble (%v).\n", err)
	}

	return nil
}

func (h *Host) cmdAssembleInteractive(c *cmd.Command, args []string) error {
	if len(args) == 0 {
		c.DisplayUsage(h)
		return nil
	}

	addr, err := h.parseAddr(args[0], 0)
	if err != nil {
		fmt.Fprintf(h, "%v\n", err)
		return nil
	}

	h.setState(stateMiniAssembler)
	h.miniAddr = addr
	h.assembly = nil
	h.lastCmd = nil

	fmt.Fprintln(h, "Enter assembly language instructions.")
	fmt.Fprintln(h, "Type END to assemble, Ctrl-C to cancel.")
	return nil
}

func (h *Host) cmdAssembleMap(c *cmd.Command, args []string) error {
	if len(args) < 2 {
		c.DisplayUsage(h)
		return nil
	}

	addr, err := h.parseAddr(args[1], 0)
	if err != nil {
		fmt.Fprintln(h, "Invalid origin address.")
		return nil
	}

	binFilename := args[0]
	binFile, err := os.Open(binFilename)
	if err != nil {
		if filepath.Ext(binFilename) == "" {
			binFilename += ".bin"
			binFile, err = os.Open(binFilename)
		}
		if err != nil {
			fmt.Fprintf(h, "%v\n", err)
			return nil
		}
	}
	defer binFile.Close()

	code, err := io.ReadAll(binFile)
	if err != nil {
		fmt.Fprintf(h, "%v\n", err)
		return nil
	}

	sourceMap := asm.NewSourceMap()
	sourceMap.Origin = addr
	sourceMap.Size = uint32(len(code))
	sourceMap.CRC = crc32.ChecksumIEEE(code)

	ext := filepath.Ext(binFilename)
	mapFilename := binFilename[:len(binFilename)-len(ext)] + ".map"

	mapFile, err := os.OpenFile(mapFilename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		fmt.Fprintf(h, "%v\n", err)
		return nil
	}
	defer mapFile.Close()

	_, err = sourceMap.WriteTo(mapFile)
	if err != nil {
		fmt.Fprintf(h, "%v\n", err)
		return nil
	}

	fmt.Fprintf(h, "Saved source map '%s'.\n", mapFilename)
	return nil
}

func (h *Host) cmdBreakpointList(c *cmd.Command, args []string) error {
	bp := h.debugger.GetBreakpoints()
	if len(bp) == 0 {
		fmt.Fprintln(h, "No breakpoints set.")
		return nil
	}

	disabled := func(b *cpu.Breakpoint) string {
		if b.Disabled {
			return "(disabled)"
		}
		return ""
	}

	fmt.Fprintln(h, "Breakpoints:")
	for _, b := range bp {
		fmt.Fprintf(h, "   $%04X %s\n", b.Address, disabled(b))
	}
	return nil
}

func (h *Host) cmdBreakpointAdd(c *cmd.Command, args []string) error {
	if len(args) < 1 {
		c.DisplayUsage(h)
		return nil
	}

	addr, err := h.parseExpr(args[0])
	if err != nil {
		fmt.Fprintf(h, "%v\n", err)
		return nil
	}

	h.debugger.AddBreakpoint(addr)
	fmt.Fprintf(h, "Breakpoint added at $%04x.\n", addr)
	return nil
}

func (h *Host) cmdBreakpointRemove(c *cmd.Command, args []string) error {
	if len(args) < 1 {
		c.DisplayUsage(h)
		return nil
	}

	addr, err := h.parseExpr(args[0])
	if err != nil {
		fmt.Fprintf(h, "%v\n", err)
		return nil
	}

	if h.debugger.GetBreakpoint(addr) == nil {
		fmt.Fprintf(h, "No breakpoint was set on $%04X.\n", addr)
		return nil
	}

	h.debugger.RemoveBreakpoint(addr)
	fmt.Fprintf(h, "Breakpoint at $%04x removed.\n", addr)
	return nil
}

func (h *Host) cmdBreakpointEnable(c *cmd.Command, args []string) error {
	if len(args) < 1 {
		c.DisplayUsage(h)
		return nil
	}

	addr, err := h.parseExpr(args[0])
	if err != nil {
		fmt.Fprintf(h, "%v\n", err)
		return nil
	}

	b := h.debugger.GetBreakpoint(addr)
	if b == nil {
		fmt.Fprintf(h, "No breakpoint was set on $%04X.\n", addr)
		return nil
	}

	b.Disabled = false
	fmt.Fprintf(h, "Breakpoint at $%04x enabled.\n", addr)
	return nil
}

func (h *Host) cmdBreakpointDisable(c *cmd.Command, args []string) error {
	if len(args) < 1 {
		c.DisplayUsage(h)
		return nil
	}

	addr, err := h.parseExpr(args[0])
	if err != nil {
		fmt.Fprintf(h, "%v\n", err)
		return nil
	}

	b := h.debugger.GetBreakpoint(addr)
	if b == nil {
		fmt.Fprintf(h, "No breakpoint was set on $%04X.\n", addr)
		return nil
	}

	b.Disabled = true
	fmt.Fprintf(h, "Breakpoint at $%04x disabled.\n", addr)
	return nil
}

func (h *Host) cmdDataBreakpointList(c *cmd.Command, args []string) error {
	bp := h.debugger.GetDataBreakpoints()
	if len(bp) == 0 {
		fmt.Fprintln(h, "No data breakpoints set.")
		return nil
	}

	disabled := func(d *cpu.DataBreakpoint) string {
		if d.Disabled {
			return "(disabled)"
		}
		return ""
	}

	fmt.Fprintln(h, "Data breakpoints:")
	for _, b := range h.debugger.GetDataBreakpoints() {
		if b.Conditional {
			fmt.Fprintf(h, "   $%04X on value $%02X %s\n", b.Address, b.Value, disabled(b))
		} else {
			fmt.Fprintf(h, "   $%04X %s\n", b.Address, disabled(b))
		}
	}
	return nil
}

func (h *Host) cmdDataBreakpointAdd(c *cmd.Command, args []string) error {
	if len(args) < 1 {
		c.DisplayUsage(h)
		return nil
	}

	addr, err := h.parseExpr(args[0])
	if err != nil {
		fmt.Fprintf(h, "%v\n", err)
		return nil
	}

	if len(args) > 1 {
		value, err := h.parseExpr(args[1])
		if err != nil {
			fmt.Fprintf(h, "%v\n", err)
			return nil
		}
		h.debugger.AddConditionalDataBreakpoint(addr, byte(value))
		fmt.Fprintf(h, "Conditional data Breakpoint added at $%04x for value $%02X.\n", addr, value)
	} else {
		h.debugger.AddDataBreakpoint(addr)
		fmt.Fprintf(h, "Data breakpoint added at $%04x.\n", addr)
	}

	return nil
}

func (h *Host) cmdDataBreakpointRemove(c *cmd.Command, args []string) error {
	if len(args) < 1 {
		c.DisplayUsage(h)
		return nil
	}

	addr, err := h.parseExpr(args[0])
	if err != nil {
		fmt.Fprintf(h, "%v\n", err)
		return nil
	}

	if h.debugger.GetDataBreakpoint(addr) == nil {
		fmt.Fprintf(h, "No data breakpoint was set on $%04X.\n", addr)
		return nil
	}

	h.debugger.RemoveDataBreakpoint(addr)
	fmt.Fprintf(h, "Data breakpoint at $%04x removed.\n", addr)
	return nil
}

func (h *Host) cmdDataBreakpointEnable(c *cmd.Command, args []string) error {
	if len(args) < 1 {
		c.DisplayUsage(h)
		return nil
	}

	addr, err := h.parseExpr(args[0])
	if err != nil {
		fmt.Fprintf(h, "%v\n", err)
		return nil
	}

	b := h.debugger.GetDataBreakpoint(addr)
	if b == nil {
		fmt.Fprintf(h, "No data breakpoint was set on $%04X.\n", addr)
		return nil
	}

	b.Disabled = false
	fmt.Fprintf(h, "Data breakpoint at $%04x enabled.\n", addr)
	return nil
}

func (h *Host) cmdDataBreakpointDisable(c *cmd.Command, args []string) error {
	if len(args) < 1 {
		c.DisplayUsage(h)
		return nil
	}

	addr, err := h.parseExpr(args[0])
	if err != nil {
		fmt.Fprintf(h, "%v\n", err)
		return nil
	}

	b := h.debugger.GetDataBreakpoint(addr)
	if b == nil {
		fmt.Fprintf(h, "No data breakpoint was set on $%04X.\n", addr)
		return nil
	}

	b.Disabled = true
	fmt.Fprintf(h, "Data breakpoint at $%04x disabled.\n", addr)
	return nil
}

func (h *Host) cmdDisassemble(c *cmd.Command, args []string) error {
	if len(args) == 0 {
		args = []string{"$"}
	}

	addr, err := h.parseAddr(args[0], h.settings.NextDisasmAddr)
	if err != nil {
		fmt.Fprintf(h, "%v\n", err)
		return nil
	}

	count := h.settings.DisasmLines
	if len(args) > 1 {
		l, err := h.parseExpr(args[1])
		if err != nil {
			fmt.Fprintf(h, "%v\n", err)
			return nil
		}
		count = int(l)
	}

	for i := 0; i < count; i++ {
		d, next := disasm.Disassemble(h.cpu, addr, disasm.ShowBasic, h.annotations[addr], h.theme)
		fmt.Fprintln(h, d)
		addr = next
	}

	h.settings.NextDisasmAddr = addr
	h.lastArgs = []string{"$", strconv.Itoa(count)}
	return nil
}

func (h *Host) cmdExports(c *cmd.Command, args []string) error {
	if len(h.sourceMap.Exports) == 0 {
		fmt.Fprintln(h, "No active exports.")
		return nil
	}

	fmt.Fprintln(h, "Exported addresses:")
	for _, e := range h.sourceMap.Exports {
		fmt.Fprintf(h, "   %-16s $%04X\n", e.Label, e.Address)
	}
	return nil
}

func (h *Host) cmdEvaluate(c *cmd.Command, args []string) error {
	if len(args) < 1 {
		c.DisplayUsage(h)
		return nil
	}

	expr := strings.Join(args, " ")
	v, err := h.parseExpr(expr)
	if err != nil {
		fmt.Fprintf(h, "%v\n", err)
		return nil
	}

	fmt.Fprintf(h, "$%04X\n", v)
	return nil
}

func (h *Host) cmdExecute(c *cmd.Command, args []string) error {
	if len(args) < 1 {
		c.DisplayUsage(h)
		return nil
	}

	file, err := os.Open(args[0])
	if err != nil {
		fmt.Fprintf(h, "%v\n", err)
		return nil
	}
	defer file.Close()

	ioState := h.EnableProcessedMode(file, os.Stdout)
	h.RunCommands(false)
	h.RestoreIoState(ioState)

	return nil
}

func (h *Host) cmdHelp(c *cmd.Command, args []string) error {
	if len(args) == 0 {
		cmds.DisplayHelp(h)
		return nil
	}

	n, _, err := cmds.Lookup(strings.Join(args, " "))
	if err != nil {
		fmt.Fprintf(h, "%v.\n\n", err)
		return nil
	}

	n.DisplayHelp(h)
	return nil
}

func (h *Host) cmdList(c *cmd.Command, args []string) error {
	if len(args) == 0 {
		args = []string{"$"}
	}

	// Parse the address.
	addr, err := h.parseAddr(args[0], h.settings.NextSourceAddr)
	if err != nil {
		fmt.Fprintf(h, "%v\n", err)
		return nil
	}

	// Parse the number of lines to display.
	count := h.settings.SourceLines
	if len(args) > 1 {
		v, err := h.parseExpr(strings.Join(args[1:], " "))
		if err != nil {
			fmt.Fprintf(h, "%v\n", err)
			return nil
		}
		count = int(v)
	}

	// Keep track of the last displayed line number for each source file.
	last := make(map[string]int)

	// Search around the address for an address with source code, and attempt
	// to display the first source code line.
	var buf [3]byte
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

		addr = h.cpu.NextAddr(orig)
		cn := addr - orig
		h.cpu.Mem.LoadBytes(orig, buf[:cn])
		cs := codeString(buf[:cn])
		fmt.Fprintf(h, "%s%04X%s- %s%-8s%s\t%s%s%s\n",
			h.theme.Addr, orig, h.theme.Reset,
			h.theme.Code, cs, h.theme.Reset,
			h.theme.Source, lines[li-1], h.theme.Source)

		last[fn] = li
		break
	}

	if len(last) == 0 {
		fmt.Fprintf(h, "No source code found for address $%04X.\n", addr)
		return nil
	}

	// Display remaining source code lines.
	for i := 0; i < count-1; i++ {
		var orig uint16
		var fn string
		var li int
		for j := 0; j < 2; j++ {
			orig = addr + uint16(j)
			fn, li, err = h.sourceMap.Find(int(orig))
			if err == nil {
				break
			}
		}
		if err != nil {
			break
		}

		lines, err := h.getSourceLines(fn)
		if err != nil {
			last[fn] = li
			continue
		}

		addr = h.cpu.NextAddr(orig)
		cn := addr - orig
		h.cpu.Mem.LoadBytes(orig, buf[:cn])
		cs := codeString(buf[:cn])

		l, ok := last[fn]
		if !ok {
			l = li - 1
		}

		for i, j := l, min(li, len(lines)); i < j; i++ {
			var c string
			if i == j-1 {
				c = cs
			}
			fmt.Fprintf(h, "%s%04X%s- %s%-8s%s\t%s%s%s\n",
				h.theme.Addr, orig, h.theme.Reset,
				h.theme.Code, c, h.theme.Reset,
				h.theme.Source, lines[i], h.theme.Reset)
		}

		last[fn] = li
	}

	h.settings.NextSourceAddr = addr
	h.lastArgs = []string{"$", strconv.Itoa(count)}
	return nil
}

func (h *Host) cmdLoad(c *cmd.Command, args []string) error {
	if len(args) < 1 {
		c.DisplayUsage(h)
		return nil
	}

	filename := args[0]

	loadAddr := -1
	if len(args) >= 2 {
		addr, err := h.parseExpr(args[1])
		if err != nil {
			fmt.Fprintf(h, "%v\n", err)
			return nil
		}
		loadAddr = int(addr)
	}

	_, err := h.load(filename, loadAddr)
	return err
}

func (h *Host) cmdMemoryDump(c *cmd.Command, args []string) error {
	if len(args) == 0 {
		args = []string{"$"}
	}

	var addr uint16
	if len(args) > 0 {
		var err error
		addr, err = h.parseAddr(args[0], h.settings.NextMemDumpAddr)
		if err != nil {
			fmt.Fprintf(h, "%v\n", err)
			return nil
		}
	}

	bytes := uint16(h.settings.MemDumpBytes)
	if len(args) >= 2 {
		var err error
		bytes, err = h.parseExpr(args[1])
		if err != nil {
			fmt.Fprintf(h, "%v\n", err)
			return nil
		}
	}

	h.dumpMemory(addr, bytes)

	h.settings.NextMemDumpAddr = addr + bytes
	h.lastArgs = []string{"$", strconv.Itoa(int(bytes))}
	return nil
}

func (h *Host) cmdMemorySet(c *cmd.Command, args []string) error {
	if len(args) < 2 {
		c.DisplayUsage(h)
		return nil
	}

	addr, err := h.parseAddr(args[0], h.settings.NextMemDumpAddr)
	if err != nil {
		fmt.Fprintf(h, "%v\n", err)
		return nil
	}

	for i := 1; i < len(args); i++ {
		v, err := h.parseExpr(args[i])
		if err != nil {
			fmt.Fprintf(h, "%v\n", err)
			return nil
		}
		h.mem.StoreByte(addr, byte(v))
		addr++
	}

	return nil
}

func (h *Host) cmdMemoryCopy(c *cmd.Command, args []string) error {
	if len(args) < 3 {
		c.DisplayUsage(h)
		return nil
	}

	dst, err := h.parseAddr(args[0], 0)
	if err != nil {
		fmt.Fprintf(h, "%v\n", err)
		return nil
	}

	src0, err := h.parseAddr(args[1], 0)
	if err != nil {
		fmt.Fprintf(h, "%v\n", err)
		return nil
	}

	src1, err := h.parseAddr(args[2], 0)
	if err != nil {
		fmt.Fprintf(h, "%v\n", err)
		return nil
	}

	if src1 < src0 {
		fmt.Fprintln(h, "Source-end address must be greater than source-begin address.")
		return nil
	}

	b := make([]byte, src1-src0+1)
	h.cpu.Mem.LoadBytes(src0, b)
	h.cpu.Mem.StoreBytes(dst, b)
	fmt.Fprintf(h, "%d bytes copied from $%04X to $%04X.\n", len(b), src0, dst)
	return nil
}

func (h *Host) cmdQuit(c *cmd.Command, args []string) error {
	return errors.New("exiting program")
}

func (h *Host) cmdRegister(c *cmd.Command, args []string) error {
	if len(args) == 0 {
		fmt.Fprintf(h, disasm.GetRegisterString(&h.cpu.Reg, h.theme)+
			disasm.GetCyclesString(h.cpu, h.theme)+"\n")
		return nil
	}

	if len(args) == 1 {
		c.DisplayUsage(h)
		return nil
	}

	key, value := strings.ToUpper(args[0]), strings.Join(args[1:], " ")

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
			fmt.Fprintf(h, "%v\n", err)
			return nil
		}

		*flag = v
		fmt.Fprintf(h, "Status flag %s set to %v.\n", flagName, v)
	} else {
		v, err := h.exprParser.Parse(value, h)
		if err != nil {
			fmt.Fprintf(h, "%v\n", err)
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
			fmt.Fprintf(h, "Unknown register '%s'.\n", key)
			return nil
		}

		switch sz {
		case 1:
			fmt.Fprintf(h, "Register %s set to $%02X.\n", strings.ToUpper(key), byte(v))
		case 2:
			fmt.Fprintf(h, "Register %s set to $%04X.\n", strings.ToUpper(key), uint16(v))
		}
	}

	if h.rawMode {
		fmt.Fprintf(h, disasm.GetRegisterString(&h.cpu.Reg, h.theme)+
			disasm.GetCyclesString(h.cpu, h.theme)+"\n")
	}

	return nil
}

func (h *Host) cmdRun(c *cmd.Command, args []string) error {
	if len(args) > 0 {
		pc, err := h.parseExpr(args[0])
		if err != nil {
			fmt.Fprintf(h, "%v\n", err)
			return nil
		}
		h.cpu.SetPC(pc)
	}

	fmt.Fprintf(h, "Running from $%04X. Press ctrl-C to break.\n", h.cpu.Reg.PC)

	h.state = stateRunning
	for step := 0; h.state == stateRunning; step++ {
		h.step()
		h.breakCheck(step)
	}

	if h.state == stateInterrupted {
		h.displayPC()
	}

	h.setState(stateProcessingCommands)
	h.settings.NextDisasmAddr = h.cpu.Reg.PC
	return nil
}

func (h *Host) breakCheck(step int) {
	// To prevent performance degradation, only test for ctrl-C once every 128
	// CPU steps.
	if (step & 127) == 127 {
		// Peek at the console's input buffer to see if it contains a key-down
		// event for ctrl-C. This is only necessary on Windows, where there is
		// no ability to detect a break signal. On all other platforms,
		// term.PeekKey() is a no-op that returns false.
		const CtrlC rune = 3
		if h.rawMode && term.PeekKey(int(os.Stdin.Fd()), CtrlC) {
			// If ctrl-C was detected, flush the input buffer by reading lines
			// until the ctrl-C is encountered.
			for {
				_, err := h.rawTerminal.ReadLine()
				if err == io.EOF {
					break
				}
			}
			h.Break()
		}
	}
}

func (h *Host) cmdSet(c *cmd.Command, args []string) error {
	switch len(args) {
	case 0:
		fmt.Fprintln(h, "Variables:")
		h.settings.Display(h)

	case 1:
		c.DisplayUsage(h)

	default:
		key, value := strings.ToLower(args[0]), strings.Join(args[1:], " ")
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
			fmt.Fprintln(h, "Setting updated.")
		} else {
			fmt.Fprintf(h, "%v\n", err)
		}

		h.onSettingsUpdate()
	}

	return nil
}

func (h *Host) cmdStepIn(c *cmd.Command, args []string) error {
	// Parse the number of steps.
	count := 1
	if len(args) > 0 {
		n, err := h.parseExpr(args[0])
		if err == nil {
			count = int(n)
		}
	}

	if count == 0 {
		h.displayPC()
	} else {
		h.setState(stateRunning)
		for i := count - 1; i >= 0 && h.state == stateRunning; i-- {
			h.step()
			switch {
			case i == h.settings.MaxStepLines:
				fmt.Fprintln(h, "...")
			case i < h.settings.MaxStepLines:
				h.displayPC()
			}
		}
	}

	h.setState(stateProcessingCommands)
	h.settings.NextDisasmAddr = h.cpu.Reg.PC
	return nil
}

func (h *Host) cmdStepOver(c *cmd.Command, args []string) error {
	// Parse the number of steps.
	count := 1
	if len(args) > 0 {
		n, err := h.parseExpr(args[0])
		if err == nil {
			count = int(n)
		}
	}

	if count == 0 {
		h.displayPC()
	} else {
		h.setState(stateRunning)
		for i := count - 1; i >= 0 && h.state == stateRunning; i-- {
			h.stepOver()
			switch {
			case i == h.settings.MaxStepLines:
				fmt.Fprintln(h, "...")
			case i < h.settings.MaxStepLines:
				h.displayPC()
			}
		}
	}

	h.setState(stateProcessingCommands)
	h.settings.NextDisasmAddr = h.cpu.Reg.PC
	return nil
}

func (h *Host) cmdStepOut(c *cmd.Command, args []string) error {
	count := 1

	h.setState(stateRunning)
	for i := count - 1; i >= 0 && h.state == stateRunning; i-- {
		h.stepOut()
		switch {
		case i == h.settings.MaxStepLines:
			fmt.Fprintln(h, "...")
		case i < h.settings.MaxStepLines:
			h.displayPC()
		}
	}

	h.setState(stateProcessingCommands)
	h.settings.NextDisasmAddr = h.cpu.Reg.PC
	return nil
}

func (h *Host) load(binFilename string, addr int) (origin uint16, err error) {
	binFilename, err = filepath.Abs(binFilename)
	if err != nil {
		fmt.Fprintf(h, "%v\n", err)
		return 0, nil
	}

	ext := filepath.Ext(binFilename)
	binFile, err := os.Open(binFilename)
	if err != nil {
		if ext == "" {
			ext = ".bin"
			binFilename = binFilename + ext
			binFile, err = os.Open(binFilename)
		}
		if err != nil {
			fmt.Fprintf(h, "%v\n", err)
			return 0, nil
		}
	}
	defer binFile.Close()

	a := &asm.Assembly{}
	_, err = a.ReadFrom(binFile)
	if err != nil {
		fmt.Fprintf(h, "%v\n", err)
		return 0, nil
	}

	// Try loading a source map file if it exists.
	mapFilename := binFilename[:len(binFilename)-len(ext)] + ".map"
	mapFile, err := os.Open(mapFilename)
	var sourceMap *asm.SourceMap
	if err == nil {
		defer mapFile.Close()
		sourceMap = asm.NewSourceMap()
		_, err = sourceMap.ReadFrom(mapFile)
		if err != nil {
			fmt.Fprintf(h, "Failed to read source map '%s': %v\n", filepath.Base(mapFilename), err)
			sourceMap = nil
		} else {
			if crc32.ChecksumIEEE(a.Code) == sourceMap.CRC {
				fmt.Fprintf(h, "Loaded source map from '%s'.\n", filepath.Base(mapFilename))
				if len(h.sourceMap.Files) == 0 {
					h.sourceMap = sourceMap
				} else {
					h.sourceMap.Merge(sourceMap)
				}
			} else {
				fmt.Fprintf(h, "Source map CRC doesn't match for '%s'.\n", filepath.Base(binFilename))
				sourceMap = nil
			}
		}
	}

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
		fmt.Fprintf(h, "File '%s' has no source map and requires an origin address.\n", filepath.Base(binFilename))
		return 0, nil
	}

	// Copy the code to the CPU memory and adjust the program counter.
	h.cpu.Mem.StoreBytes(origin, a.Code)
	fmt.Fprintf(h, "Loaded '%s' to $%04X..$%04X.\n", filepath.Base(binFilename), origin, int(origin)+len(a.Code)-1)

	h.settings.NextDisasmAddr = origin
	return origin, nil
}

func (h *Host) step() {
	h.cpu.Step()
}

func (h *Host) stepOver() {
	cpu := h.cpu

	inst := cpu.GetInstruction(cpu.Reg.PC)
	next := cpu.Reg.PC + uint16(inst.Length)
	cpu.Step()

	// If a JSR was just stepped, keep stepping until the return address
	// is hit or a corresponding RTS is stepped.
	if inst.Name == "JSR" {
		count := 1
	loop:
		for step := 0; h.state == stateRunning && cpu.Reg.PC != next; step++ {
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
			h.breakCheck(step)
		}
	}
}

func (h *Host) stepOut() {
	cpu := h.cpu

	for step := 0; h.state == stateRunning; step++ {
		inst := cpu.GetInstruction(cpu.Reg.PC)
		cpu.Step()
		if inst.Name == "RTS" || inst.Name == "RTI" {
			break
		}
		h.breakCheck(step)
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
		fmt.Fprintln(h, string(buf))
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
		fmt.Fprintln(h, string(buf))
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

// OnBrk is called when the CPU is about to execute a BRK instruction.
func (h *Host) OnBrk(cpu *cpu.CPU) {
	h.setState(stateInterrupted)
	fmt.Fprintf(h, "BRK encountered at $%04X.\n", cpu.Reg.PC)
}

// OnBreakpoint is called when the debugger encounters a code breakpoint.
func (h *Host) OnBreakpoint(cpu *cpu.CPU, b *cpu.Breakpoint) {
	h.setState(stateBreakpoint)
	fmt.Fprintf(h, "Breakpoint hit at $%04X.\n", b.Address)
	h.displayPC()
}

// OnDataBreakpoint is called when the debugger encounters a data breakpoint.
func (h *Host) OnDataBreakpoint(cpu *cpu.CPU, b *cpu.DataBreakpoint) {
	fmt.Fprintf(h, "Data breakpoint hit on address $%04X.\n", b.Address)

	h.setState(stateBreakpoint)

	if cpu.LastPC != cpu.Reg.PC {
		d, _ := disasm.Disassemble(h.cpu, cpu.LastPC, disasm.ShowFull, "", h.theme)
		fmt.Fprintln(h, d)
	}

	h.displayPC()
}
