// Copyright 2014 Brett Vickers. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package asm implements a 6502 macro assembler.
package asm

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/beevik/go6502"
)

// TODO:
//  - String and character expressions
//  - Display addressing mode strings in debug output
//  - High byte (/) and low byte(#) prefixes
//  - Format bytes data as 3 bytes per row

var (
	errParse = errors.New("parse error")
)

var hex = "0123456789ABCDEF"

var absolutePrefixes = []string{
	"a:",
	"abs:",
}

var modeFormat = []string{
	"#$%s",    // IMM
	"%s",      // IMP
	"$%s",     // REL
	"$%s",     // ZPG
	"$%s,X",   // ZPX
	"$%s,Y",   // ZPY
	"$%s",     // ABS
	"$%s,X",   // ABX
	"$%s,Y",   // ABY
	"($%s)",   // IND
	"($%s,X)", // IDX
	"($%s),Y", // IDY
	"%s",      // ACC
}

var pseudoOps = map[string]func(a *assembler, line, label fstring) error{
	".eq":   (*assembler).parseMacro,
	".equ":  (*assembler).parseMacro,
	".or":   (*assembler).parseOrigin,
	".org":  (*assembler).parseOrigin,
	".db":   (*assembler).parseBytes,
	".byte": (*assembler).parseBytes,
	".at":   (*assembler).parseAt,
}

// A segment is a small chunk of machine code that may represent a single
// instruction or a group of byte data.
type segment interface {
	address() int
}

// An instruction segment contains a single instruction, including its
// opcode and operand data.
type instruction struct {
	addr    int                 // address assigned to the segment
	opcode  fstring             // opcode string
	inst    *go6502.Instruction // selected instruction data for the opcode
	operand operand             // parameter data for the instruction
}

func (i *instruction) address() int {
	return i.addr
}

// Format a byte code string for an instruction.
func (i *instruction) codeString() string {
	sz := i.inst.Length - 1
	switch {
	case i.inst.Mode == go6502.REL:
		offset, _ := relOffset(i.operand.expr.value, i.addr+int(i.inst.Length))
		return fmt.Sprintf("%02X %02X   ", i.inst.Opcode, offset)
	case sz == 0:
		return fmt.Sprintf("%02X      ", i.inst.Opcode)
	case sz == 1:
		return fmt.Sprintf("%02X %02X   ", i.inst.Opcode, i.operand.expr.value)
	default:
		return fmt.Sprintf("%02X %02X %02X", i.inst.Opcode, i.operand.expr.value&0xff, i.operand.expr.value>>8)
	}
}

// Format an operand string based on the instruction's addressing mode.
func (i *instruction) operandString() string {
	number := i.operand.expr.value

	var n string
	switch i.inst.Length {
	case 2:
		n = fmt.Sprintf("%02X", number)
	default:
		n = fmt.Sprintf("%04X", number)
	}

	return fmt.Sprintf(modeFormat[i.inst.Mode], n)
}

// An operand represents the parameter(s) of an assembly instruction.
type operand struct {
	value         int         // resolved numeric value
	modeGuess     go6502.Mode // addressing mode guesed based on operand string
	expr          *expr       // expression tree, used to resolve value
	forceAbsolute bool        // operand must use 2-byte absolute address
}

// Return the size of the operand in bytes.
func (o *operand) size() int {
	switch {
	case o.modeGuess == go6502.IMP:
		return 0
	case o.expr.address || o.forceAbsolute || o.expr.value > 0xff:
		return 2
	default:
		return 1
	}
}

// A data segment contains one or more bytes of byte data.
type data struct {
	addr  int    // address assigned to the segment
	bytes []byte // resolved byte data
	expr  *expr  // expression used for .at
}

func (d *data) address() int {
	return d.addr
}

// An asmerror is used to keep track of errors encountered
// during assembly.
type asmerror struct {
	line fstring // row & column of assembly code causing the error
	msg  string  // error message
}

// The assembler is a state object used during the assembly of
// machine code from assembly code.
type assembler struct {
	origin     int              // requested origin
	pc         int              // the program counter
	code       []byte           // generated machine code
	scanner    *bufio.Scanner   // scans the io reader
	scopeLabel fstring          // label currently in scope
	macros     map[string]*expr // .EQ macro -> expression
	labels     map[string]int   // label -> segment index
	segments   []segment        // segment of machine code
	uneval     []*expr          // expressions requiring evaluation
	verbose    bool             // verbose output for debugging
	exprParser exprParser       // used to parse operand and macro expressions
	errors     []asmerror       // errors encountered during assembly
}

// Options for Assemble function.
type Options struct {
	Verbose bool
}

// Result of the Assemble function.
type Result struct {
	Code   []byte         // Assembled machine code
	Origin go6502.Address // Code origin address
}

// Assemble reads data from the provided stream and attempts to assemble
// it into 6502 byte code.
func Assemble(r io.Reader, o Options) (*Result, error) {
	a := &assembler{
		origin:   0x600,
		pc:       0x600,
		scanner:  bufio.NewScanner(r),
		macros:   make(map[string]*expr),
		labels:   make(map[string]int),
		segments: make([]segment, 0, 32),
		verbose:  o.Verbose,
	}

	// Assembly consists of the following steps
	steps := []func(a *assembler){
		(*assembler).parse,                        // Parse the assembly code
		(*assembler).evaluateExpressions,          // Evaluate operand & macro expressions
		(*assembler).assignAddresses,              // Assign addresses to instructions
		(*assembler).resolveLabels,                // Resolve labels to addresses
		(*assembler).evaluateExpressions,          // Do another evaluation pass with resolved labels
		(*assembler).handleUnevaluatedExpressions, // Cause error if there are unevaluated expressions
		(*assembler).generateCode,                 // Generate the machine code
	}

	// Execute assembler steps, breaking if an error is encountered
	// in any one of them
	for _, step := range steps {
		step(a)
		if len(a.errors) > 0 {
			return nil, errParse
		}
	}

	return &Result{Code: a.code, Origin: go6502.Address(a.origin)}, nil
}

// Read the assembly code and perform the initial parsing. Build up
// machine code segments, the macro table, the label table, and a
// list of unevaluated expression trees.
func (a *assembler) parse() {
	a.logSection("Parsing assembly code")
	row := 1
	for a.scanner.Scan() {
		text := a.scanner.Text()
		line := newFstring(row, text)
		a.parseLine(line.stripTrailingComment())
		row++
	}
}

// Evaluate all unevaluated expression trees using macros.
func (a *assembler) evaluateExpressions() {
	a.logSection("Evaluating expressions")
	for {
		var uneval []*expr
		for _, e := range a.uneval {
			if e.eval(a.macros, a.labels) {
				a.log("%-25s Val:$%X", e.String(), e.value)
			} else {
				a.log("%-25s Val:????? isaddr:%v", e.String(), e.address)
				uneval = append(uneval, e)
			}
		}
		if len(uneval) == len(a.uneval) {
			break
		}
		a.uneval = uneval
	}
}

// Determine addresses of all code segments.
func (a *assembler) assignAddresses() {
	a.logSection("Assigning addresses")
	for _, s := range a.segments {
		switch ss := s.(type) {
		case *instruction:
			ss.addr = a.pc
			ss.inst = findMatchingInstruction(ss.opcode, ss.operand)
			if ss.inst == nil {
				a.addError(ss.opcode, "invalid addressing mode for opcode '%s'", ss.opcode.str)
				return
			}
			a.log("%04X  %s Len:%d Mode:%d Opcode:%02X",
				ss.addr, ss.opcode.str, ss.inst.Length,
				ss.inst.Mode, ss.inst.Opcode)
			a.pc += int(ss.inst.Length)

		case *data:
			ss.addr = a.pc
			a.log("%04X  bytedata Len:%d", ss.addr, len(ss.bytes))
			a.pc += len(ss.bytes)
		}
	}
}

// Resolve all address labels.
func (a *assembler) resolveLabels() {
	a.logSection("Resolving labels")
	for label, segno := range a.labels {
		var addr int
		switch {
		case segno < len(a.segments):
			addr = a.segments[segno].address()
		default:
			addr = a.pc
		}
		a.log("%-15s Seg:%-3d Addr:$%04X", label, segno, addr)
		a.macros[label] = &expr{op: opNumber, value: addr, evaluated: true}
	}
}

// Cause an error if there are any unevaluated expressions.
func (a *assembler) handleUnevaluatedExpressions() {
	if len(a.uneval) > 0 {
		for _, e := range a.uneval {
			a.addError(e.line, "unresolved expression")
		}
	}
}

// Generate code
func (a *assembler) generateCode() {
	a.logSection("Generating code")
	for _, s := range a.segments {
		switch ss := s.(type) {
		case *instruction:
			a.code = append(a.code, ss.inst.Opcode)
			switch {
			case ss.inst.Length == 1:
				a.log("%04X- %s  %s", ss.addr, ss.codeString(), ss.opcode.str)
			case ss.inst.Mode == go6502.REL:
				offset, err := relOffset(ss.operand.expr.value, ss.addr+int(ss.inst.Length))
				if err != nil {
					a.addError(ss.opcode, "branch offset out of bounds")
				}
				a.code = append(a.code, offset)
				a.log("%04X- %s  %s  %s", ss.addr, ss.codeString(), ss.opcode.str, ss.operandString())
			case ss.inst.Length == 2:
				a.code = append(a.code, byte(ss.operand.expr.value))
				a.log("%04X- %s  %s  %s", ss.addr, ss.codeString(), ss.opcode.str, ss.operandString())
			case ss.inst.Length == 3:
				a.code = append(a.code, byte(ss.operand.expr.value&0xff))
				a.code = append(a.code, byte(ss.operand.expr.value>>8))
				a.log("%04X- %s  %s  %s", ss.addr, ss.codeString(), ss.opcode.str, ss.operandString())
			default:
				panic("invalid operand")
			}

		case *data:
			if ss.expr != nil {
				v := ss.expr.value
				ss.bytes = []byte{byte(v & 0xff), byte(v >> 8)}
			}
			a.code = append(a.code, ss.bytes...)
			a.log("%04X-*%s", ss.addr, byteString(ss.bytes))
		}
	}
}

// Parse a single line of assembly code.
func (a *assembler) parseLine(line fstring) (err error) {
	// Skip empty (or comment-only) lines
	if line.isEmpty() {
		return
	}

	a.log("---")

	switch {
	case line.startsWith(whitespace):
		err = a.parseUnlabeledLine(line.consumeWhitespace())
	default:
		err = a.parseLabeledLine(line)
	}
	return
}

// Parse a line of assembly code that contains no label.
func (a *assembler) parseUnlabeledLine(line fstring) (err error) {
	a.logLine(line, "unlabeled_line")

	// Is the next word a pseudo-op, rather than an opcode?
	if line.startsWithChar('.') {
		var pseudoOp fstring
		pseudoOp, line = line.consumeWhile(wordChar)
		err = a.parsePseudoOp(line.consumeWhitespace(), fstring{}, pseudoOp)
		return
	}

	err = a.parseInstruction(line)
	return
}

// Parse a line of assembly code that starts with a label.
func (a *assembler) parseLabeledLine(line fstring) (err error) {
	a.logLine(line, "labeled_line")

	// Parse the label field
	var label fstring
	label, line, err = a.parseLabel(line)
	if err != nil {
		return
	}

	// Is the next word a pseudo-op, rather than an opcode?
	if line.startsWithChar('.') {
		var pseudoOp fstring
		pseudoOp, line = line.consumeWhile(wordChar)
		err = a.parsePseudoOp(line.consumeWhitespace(), label, pseudoOp)
		return
	}

	// Store the label.
	err = a.storeLabel(line, label)
	if err != nil {
		return
	}

	// Parse any instruction following the label
	if !line.isEmpty() {
		err = a.parseInstruction(line)
	}
	return
}

// Store a label into the assembler's label list.
func (a *assembler) storeLabel(line, label fstring) error {
	// If the label starts with '.', it is a local label. So append
	// it to the active scope label.
	if label.startsWithChar('.') {
		if a.scopeLabel.isEmpty() {
			a.addError(label, "no global label '%s' previously defined", label.str)
			return errParse
		}
		label.str = a.scopeLabel.str + label.str
	} else {
		a.scopeLabel = label
	}

	if _, found := a.labels[label.str]; found {
		a.addError(label, "label '%s' used more than once", label.str)
		return errParse
	}

	// Associate the label with its segment number.
	a.labels[label.str] = len(a.segments)
	a.logLine(line, "label=%s [%d]", label.str, len(a.segments))
	return nil
}

// Parse a label string at the beginning of a line of assembly code.
func (a *assembler) parseLabel(line fstring) (label fstring, out fstring, err error) {
	// Make sure label starts with a valid label character.
	if !line.startsWith(labelStartChar) {
		s, _ := line.consumeUntil(whitespace)
		a.addError(line, "invalid label '%s'", s.str)
		err = errParse
		return
	}

	// Grab the label and advance the line past it.
	label, line = line.consumeWhile(labelChar)

	// Skip colon after label.
	if line.startsWithChar(':') {
		line = line.consume(1)
	}

	// If the next character isn't whitespace, we encountered an invalid label character
	if !line.isEmpty() && !line.startsWith(whitespace) {
		s, _ := line.consumeUntil(whitespace)
		a.addError(line, "invalid label '%s%s'", label.str, s.str)
		err = errParse
		return
	}

	// Skip trailing whitespace
	out = line.consumeWhitespace()
	return
}

// Parse a pseudo-op beginning with "." (such as ".EQ").
func (a *assembler) parsePseudoOp(line, label, pseudoOp fstring) (err error) {
	fn, ok := pseudoOps[strings.ToLower(pseudoOp.str)]
	if !ok {
		a.addError(pseudoOp, "invalid directive '%s'", pseudoOp.str)
		return errParse
	}
	return fn(a, line, label)
}

// Parse an ".EQ" macro definition.
func (a *assembler) parseMacro(line, label fstring) (err error) {
	if label.str == "" {
		a.addError(line, ".EQ must begin with a label")
		return
	}

	a.logLine(line, "macro=%s", label.str)

	// Parse the macro expression.
	var e *expr
	e, line, err = a.exprParser.parse(line, a.scopeLabel, true)
	if err != nil {
		a.addExprErrors()
		return
	}

	// Attempt to evaluate the macro expression immediately. If not possible,
	// add it to a list of unevaluated expressions.
	if !e.eval(a.macros, a.labels) {
		a.uneval = append(a.uneval, e)
	}

	a.logLine(line, "expr=%s", e.String())
	a.logLine(line, "val=$%X", e.value)

	// Track the macro for later substitution.
	a.macros[label.str] = e
	return
}

// Parse an ".ORG" origin definition
func (a *assembler) parseOrigin(line, label fstring) (err error) {
	if len(a.segments) > 0 {
		a.addError(line, "origin directive must appear before first instruction")
		return
	}

	a.logLine(line, "origin=")

	var e *expr
	e, line, err = a.exprParser.parse(line, a.scopeLabel, true)
	if err != nil {
		a.addExprErrors()
		return
	}

	if !e.eval(a.macros, a.labels) {
		a.addError(e.identifier, "unable to evaluate expression")
		return
	}

	a.logLine(line, "expr=%s", e.String())
	a.logLine(line, "val=$%04X", e.value)

	a.origin = e.value
	a.pc = e.value
	return
}

// Parse a .BYTES pseudo-op
func (a *assembler) parseBytes(line, label fstring) (err error) {
	a.logLine(line, "bytes=")

	b := []byte{}
	for !line.isEmpty() {
		var value int
		var bytes int
		value, bytes, line, err = a.exprParser.parseNumber(line)
		if err != nil {
			a.addExprErrors()
			break
		}

		switch bytes {
		case 1:
			b = append(b, byte(value))
		case 2:
			b = append(b, []byte{byte(value), byte(value >> 8)}...)
		case 4:
			b = append(b, []byte{byte(value), byte(value >> 8), byte(value >> 16), byte(value >> 24)}...)
		}

		_, line = line.consumeWhile(func(c byte) bool { return c == ',' || whitespace(c) })
	}

	if !label.isEmpty() {
		err = a.storeLabel(line, label)
		if err != nil {
			return
		}
	}

	seg := &data{bytes: b}
	a.segments = append(a.segments, seg)
	return
}

// Parse an .AT pseudo-op.
func (a *assembler) parseAt(line, label fstring) (err error) {
	a.logLine(line, "at=")

	// Parse the AT expression.
	var e *expr
	e, line, err = a.exprParser.parse(line, a.scopeLabel, true)
	if err != nil {
		a.addExprErrors()
		return
	}

	// Attempt to evaluate the expression immediately.
	if !e.eval(a.macros, a.labels) {
		a.uneval = append(a.uneval, e)
	}

	if !label.isEmpty() {
		err = a.storeLabel(line, label)
		if err != nil {
			return
		}
	}

	a.logLine(line, "expr=%s", e.String())
	a.logLine(line, "val=$%X", e.value)

	seg := &data{bytes: []byte{0, 0}, expr: e}
	a.segments = append(a.segments, seg)
	return
}

// Parse a 6502 assembly opcode + operand.
func (a *assembler) parseInstruction(line fstring) (err error) {
	// Parse the opcode.
	opcode, out := line.consumeWhile(opcodeChar)

	// No opcode characters? Or opcode has invalid suffix?
	if opcode.isEmpty() || (!out.isEmpty() && !out.startsWith(whitespace)) {
		a.addError(out, "invalid opcode '%s'", opcode.str)
		err = errParse
		return
	}

	// Validate the opcode
	instructions := go6502.GetInstructions(opcode.str)
	if instructions == nil {
		a.addError(opcode, "invalid opcode '%s'", opcode.str)
		err = errParse
		return
	}

	out = out.consumeWhitespace()
	a.logLine(out, "op=%s", opcode.str)

	// Parse the operand, if any
	var operand operand
	operand, out, err = a.parseOperand(out)

	// Create a code segment for the instruction
	seg := &instruction{opcode: opcode, operand: operand}
	a.segments = append(a.segments, seg)
	return
}

// Parse the operand expression following an opcode.
func (a *assembler) parseOperand(line fstring) (o operand, remain fstring, err error) {
	switch {
	case line.isEmpty():
		// Handle immediate mode (no operand)
		o.modeGuess, remain = go6502.IMP, line
		return

	case line.startsWithChar('('):
		// Handle indirect addressing modes
		var expr fstring
		o.modeGuess, expr, remain, err = line.consume(1).consumeIndirect()
		if err != nil {
			a.addError(remain, "unknown addressing mode format")
			return
		}
		o.expr, _, err = a.exprParser.parse(expr, a.scopeLabel, false)
		if err != nil {
			a.addExprErrors()
			return
		}

	case line.startsWithChar('#'):
		// Handle immediate addressing mode
		o.modeGuess = go6502.IMM
		o.expr, remain, err = a.exprParser.parse(line.consume(1), a.scopeLabel, false)
		if err != nil {
			a.addExprErrors()
			return
		}

	default:
		// Handle absolute addressing modes (zero page and full absolute)
		var expr fstring
		o.modeGuess, o.forceAbsolute, expr, remain, err = line.consumeAbsolute()
		if err != nil {
			a.addError(remain, "unknown addressing mode format")
			return
		}
		o.expr, _, err = a.exprParser.parse(expr, a.scopeLabel, false)
		if err != nil {
			a.addExprErrors()
			return
		}
	}

	if !o.expr.eval(a.macros, a.labels) {
		a.uneval = append(a.uneval, o.expr)
	}
	a.logLine(remain, "expr=%s", o.expr)
	a.logLine(remain, "mode=%d", o.modeGuess)
	a.logLine(remain, "val=$%X", o.expr.value)

	if !remain.isEmpty() && !remain.startsWith(whitespace) {
		a.addError(remain, "operand expression")
		err = errParse
		return
	}

	remain = remain.consumeWhitespace()
	return
}

// Append an error message to the assembler's error state.
func (a *assembler) addError(l fstring, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	a.errors = append(a.errors, asmerror{l, msg})
	if a.verbose {
		fmt.Printf("Syntax error: %s\n", msg)
		fmt.Println(l.full)
		for i := 0; i < l.column; i++ {
			fmt.Printf("-")
		}
		fmt.Println("^")
	} else {
		fmt.Fprintf(os.Stderr, "Syntax error line %d, col %d: %s\n", l.row, l.column+1, msg)
	}
}

// Append the expression parser's error to the assembler's
// error state.
func (a *assembler) addExprErrors() {
	for _, e := range a.exprParser.errors {
		a.addError(e.line, e.msg)
	}
}

// In verbose mode, log a string to standard output.
func (a *assembler) log(format string, args ...interface{}) {
	if a.verbose {
		fmt.Printf(format, args...)
		fmt.Printf("\n")
	}
}

// In verbose mode, log a string and its associated line
// of assembly code.
func (a *assembler) logLine(line fstring, format string, args ...interface{}) {
	if a.verbose {
		detail := fmt.Sprintf(format, args...)
		fmt.Printf("%-3d %-3d | %-20s | %s\n", line.row, line.column, detail, line.str)
	}
}

// In verbose mode, log a section header to the standard output.
func (a *assembler) logSection(name string) {
	if a.verbose {
		fmt.Println(strings.Repeat("-", len(name)+6))
		fmt.Printf("-- %s --\n", name)
		fmt.Println(strings.Repeat("-", len(name)+6))
	}
}

// Compute the relative offset of two addresses as a
// two's-complement byte value. If the offset can't
// fit into a byte, return an error.
func relOffset(addr1, addr2 int) (byte, error) {
	diff := addr1 - addr2
	switch {
	case diff < -128 || diff > 127:
		return 0, errParse
	case diff >= 0:
		return byte(diff), nil
	default:
		return byte(256 + diff), nil
	}
}

// Given an opcode and operand data, select the best 6502
// instruction match. Prefer the instruction with the shortest
// total length.
func findMatchingInstruction(opcode fstring, operand operand) *go6502.Instruction {
	bestqual := 3
	var found *go6502.Instruction
	for _, inst := range go6502.GetInstructions(opcode.str) {
		match, qual := false, 0
		switch {
		case inst.Mode == go6502.IMP || inst.Mode == go6502.ACC:
			match, qual = (operand.modeGuess == go6502.IMP) && (operand.size() == 0), 0
		case operand.size() == 0:
			match = false
		case inst.Mode == go6502.IMM:
			match, qual = (operand.modeGuess == go6502.IMM) && (operand.size() == 1), 1
		case inst.Mode == go6502.REL:
			match, qual = (operand.modeGuess == go6502.ABS) && (operand.size() <= 2), 1
		case inst.Mode == go6502.ZPG:
			match, qual = (operand.modeGuess == go6502.ABS) && (operand.size() == 1), 1
		case inst.Mode == go6502.ZPX:
			match, qual = (operand.modeGuess == go6502.ABX) && (operand.size() == 1), 1
		case inst.Mode == go6502.ZPY:
			match, qual = (operand.modeGuess == go6502.ABY) && (operand.size() == 1), 1
		case inst.Mode == go6502.ABS:
			match, qual = (operand.modeGuess == go6502.ABS) && (operand.size() <= 2), 2
		case inst.Mode == go6502.ABX:
			match, qual = (operand.modeGuess == go6502.ABX) && (operand.size() <= 2), 2
		case inst.Mode == go6502.ABY:
			match, qual = (operand.modeGuess == go6502.ABY) && (operand.size() <= 2), 2
		case inst.Mode == go6502.IND:
			match, qual = (operand.modeGuess == go6502.IND) && (operand.size() <= 2), 2
		case inst.Mode == go6502.IDX:
			match, qual = (operand.modeGuess == go6502.IDX) && (operand.size() == 1), 1
		case inst.Mode == go6502.IDY:
			match, qual = (operand.modeGuess == go6502.IDY) && (operand.size() == 1), 1
		}
		if !match {
			continue
		}
		if qual < bestqual {
			bestqual, found = qual, inst
		}
	}
	return found
}

// Consume an operand expression starting with '(' until
// an indirect addressing mode substring is reached. Return
// the candidate addressing mode and expression substring.
func (l fstring) consumeIndirect() (mode go6502.Mode, expr fstring, remain fstring, err error) {
	i := l.scanUntil(func(c byte) bool { return c == ',' || c == ')' })
	expr, remain = l.trunc(i), l.consume(i)
	switch {
	case remain.startsWithString(",X)"):
		mode, remain = go6502.IDX, remain.consume(3)
	case remain.startsWithString("),Y"):
		mode, remain = go6502.IDY, remain.consume(3)
	case remain.startsWithChar(')'):
		mode, remain = go6502.IND, remain.consume(1)
	default:
		err = errParse
	}
	remain = remain.consumeWhitespace()
	if !remain.isEmpty() {
		err = errParse
	}
	return
}

// Consume an absolute operand expression until an absolute
// addressing mode substring is reached. Guess the addressing mode,
// and return the expression substring.
func (l fstring) consumeAbsolute() (mode go6502.Mode, forceAbsolute bool, expr fstring, remain fstring, err error) {
	i := l.scanUntil(func(c byte) bool { return c == ',' })
	expr, remain = l.trunc(i), l.consume(i)

	for _, p := range absolutePrefixes {
		if expr.startsWithStringI(p) {
			expr = expr.consume(len(p))
			forceAbsolute = true
			break
		}
	}

	switch {
	case remain.startsWithString(",X"):
		mode, remain = go6502.ABX, remain.consume(2)
	case remain.startsWithString(",Y"):
		mode, remain = go6502.ABY, remain.consume(2)
	default:
		mode = go6502.ABS
	}

	remain = remain.consumeWhitespace()
	if !remain.isEmpty() {
		err = errParse
	}
	return
}

// Return a hexadecimal string representation of a byte slice.
func byteString(b []byte) string {
	if len(b) < 1 {
		return ""
	}
	out := make([]byte, len(b)*3-1)
	i, j := 0, 0
	for n := len(b) - 1; i < n; i, j = i+1, j+3 {
		out[j+0] = hex[(b[i] >> 4)]
		out[j+1] = hex[(b[i] & 0x0f)]
		out[j+2] = ' '
	}
	out[j+0] = hex[(b[i] >> 4)]
	out[j+1] = hex[(b[i] & 0x0f)]
	return string(out)
}
