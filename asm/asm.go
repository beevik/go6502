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
	"strings"

	"github.com/beevik/go6502"
)

var (
	errParse = errors.New("parse error")
)

//
// operand
//

// An operand represents the right-hand side of an assembly
// instruction.
type operand struct {
	value int         // resolved numeric value
	mode  go6502.Mode // candidate addressing mode
	expr  *expr       // expression tree, used to resolve value
}

// Return the size of the operand in bytes
func (o *operand) size() int {
	switch {
	case o.mode == go6502.IMP:
		return 0
	case o.expr.address:
		return 2
	case o.expr.number > 0xffff:
		return 4
	case o.expr.number > 0xff:
		return 2
	default:
		return 1
	}
}

//
// segment
//

// A segment is a placeholder for a few bytes of
// machine code, usually representing a single instruction.
type segment struct {
	addr    int                 // resolved machine code address
	opcode  fstring             // instruction opcode string
	operand operand             // instruction operand data
	inst    *go6502.Instruction // resolved 6502 instruction
}

//
// asmerror
//

// An asmerror is used to keep track of errors encountered
// during assembly.
type asmerror struct {
	line fstring // row & column of assembly code causing the error
	msg  string  // error message
}

//
// assembler
//

// The assembler is a state object used during the assembly of
// machine code from assembly code.
type assembler struct {
	pc         int              // the program counter
	code       []byte           // generated machine code
	scanner    *bufio.Scanner   // scans the io reader
	currlabel  fstring          // label currently in scope
	macros     map[string]*expr // .EQ macro -> expression
	labels     map[string]int   // label -> segment index
	segments   []segment        // segment of machine code
	uneval     []*expr          // expressions requiring evaluation
	verbose    bool             // verbose output for debugging
	exprParser exprParser       // used to parse operand and macro expressions
	errors     []asmerror       // errors encountered during assembly
}

// Assemble reads data from the provided stream and attempts to assemble
// it into 6502 byte code.
func Assemble(r io.Reader) (code []byte, err error) {
	a := &assembler{
		pc:       0x600,
		scanner:  bufio.NewScanner(r),
		macros:   make(map[string]*expr),
		labels:   make(map[string]int),
		segments: make([]segment, 0, 32),
		verbose:  true,
	}

	// Assembly consists of the following steps
	steps := []func(a *assembler){
		(*assembler).parse,                        // Parse the assembly code
		(*assembler).evalExpressions,              // Evaluate operand & macro expressions
		(*assembler).assignAddresses,              // Assign addresses to instructions
		(*assembler).resolveLabels,                // Resolve labels to addresses
		(*assembler).evalExpressions,              // Do another evaluation pass with resolved labels
		(*assembler).handleUnevaluatedExpressions, // Cause error if there are unevaluated expressions
		(*assembler).generateCode,                 // Generate the machine code
	}

	// Execute assembler steps, breaking if an error is encountered
	// in any one of them
	for _, step := range steps {
		step(a)
		if len(a.errors) > 0 {
			err = errParse
			return
		}
	}

	code = a.code
	return
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
func (a *assembler) evalExpressions() {
	a.logSection("Evaluating expressions")
	for {
		var uneval []*expr
		for _, e := range a.uneval {
			if e.eval(a.macros, a.labels) {
				a.log("%-25s Val:$%X", e.String(), e.number)
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
	for i := range a.segments {
		seg := &a.segments[i]
		seg.addr = a.pc

		seg.inst = findMatchingInstruction(seg.opcode, seg.operand)
		if seg.inst == nil {
			a.addError(seg.opcode, "Invalid addressing mode for opcode")
			return
		}
		a.log("%04X  %s Len:%d Mode:%d Opcode:%02X",
			seg.addr, seg.opcode.str, seg.inst.Length,
			seg.inst.Mode, seg.inst.Opcode)

		a.pc += int(seg.inst.Length)
	}
}

// Resolve all address labels.
func (a *assembler) resolveLabels() {
	a.logSection("Resolving labels")
	for label, segno := range a.labels {
		addr := a.segments[segno].addr
		a.log("%-15s Seg:%-3d Addr:$%04X", label, segno, addr)
		a.macros[label] = &expr{op: opNumber, number: addr, evaluated: true}
	}
}

// Cause an error if there are any unevaluated expressions.
func (a *assembler) handleUnevaluatedExpressions() {
	if len(a.uneval) > 0 {
		for _, e := range a.uneval {
			a.addError(e.identifier, "unresolved label")
		}
	}
}

// Generate code
func (a *assembler) generateCode() {
	a.logSection("Generating code")
	for i := range a.segments {
		seg := &a.segments[i]
		a.code = append(a.code, seg.inst.Opcode)
		switch {
		case seg.operand.size() == 0:
			a.log("%04X- %s  %s", seg.addr, codeString(seg), seg.opcode.str)
		case seg.inst.Mode == go6502.REL:
			offset, err := relOffset(seg.operand.expr.number, seg.addr+int(seg.inst.Length))
			if err != nil {
				a.addError(seg.opcode, "Branch offset out of bounds")
			}
			a.code = append(a.code, offset)
			a.log("%04X- %s  %s  $%X", seg.addr, codeString(seg), seg.opcode.str, offset)
		case seg.operand.size() == 1:
			a.code = append(a.code, byte(seg.operand.expr.number))
			a.log("%04X- %s  %s  %s", seg.addr, codeString(seg), seg.opcode.str, operandString(seg))
		case seg.operand.size() == 2:
			a.code = append(a.code, byte(seg.operand.expr.number&0xff))
			a.code = append(a.code, byte(seg.operand.expr.number>>8))
			a.log("%04X- %s  %s  %s", seg.addr, codeString(seg), seg.opcode.str, operandString(seg))
		default:
			panic("invalid operand")
		}
	}
}

func codeString(seg *segment) string {
	sz := seg.operand.size()
	switch {
	case seg.inst.Mode == go6502.REL:
		offset, _ := relOffset(seg.operand.expr.number, seg.addr+int(seg.inst.Length))
		return fmt.Sprintf("%02X %02X   ", seg.inst.Opcode, offset)
	case sz == 0:
		return fmt.Sprintf("%02X      ", seg.inst.Opcode)
	case sz == 1:
		return fmt.Sprintf("%02X %02X   ", seg.inst.Opcode, seg.operand.expr.number)
	default:
		return fmt.Sprintf("%02X %02X %02X", seg.inst.Opcode, seg.operand.expr.number&0xff, seg.operand.expr.number>>8)
	}
}

func operandString(seg *segment) string {
	number := seg.operand.expr.number

	var n string
	switch seg.operand.size() {
	case 1:
		n = fmt.Sprintf("$%02X", number)
	default:
		n = fmt.Sprintf("$%04X", number)
	}

	switch seg.inst.Mode {
	case go6502.IMM:
		return "#" + n
	case go6502.IND:
		return "(" + n + ")"
	case go6502.ABX:
		return n + ",X"
	case go6502.IDX:
		return "(" + n + ",X)"
	case go6502.IDY:
		return "(" + n + "),Y"
	case go6502.ZPX:
		return n + ",X"
	case go6502.ZPY:
		return n + ",Y"
	default:
		return n
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

	// If the label starts with '.', it is a local label. So append
	// it to the active scope label.
	if label.startsWithChar('.') {
		if a.currlabel.isEmpty() {
			a.addError(label, "No global label previously defined")
			return errParse
		}
		label.str = a.currlabel.str + label.str
	} else {
		a.currlabel = label
	}

	// Associate the label with its segment number.
	a.labels[label.str] = len(a.segments)
	a.logLine(line, "label=%s [%d]", label.str, len(a.segments))

	// Parse any instruction following the label
	if !line.isEmpty() {
		err = a.parseInstruction(line)
	}
	return
}

// Parse a label string at the beginning of a line of assembly code.
func (a *assembler) parseLabel(line fstring) (label fstring, out fstring, err error) {

	// Make sure label starts with a valid label character
	if !line.startsWith(labelStartChar) {
		a.addError(line, "Invalid label")
		err = errParse
		return
	}

	// Grab the label and advance the line past it
	label, line = line.consumeWhile(labelChar)

	// If the next character isn't whitespace, we encountered an invalid label character
	if !line.isEmpty() && !line.startsWith(whitespace) {
		a.addError(line, "Invalid label")
		err = errParse
		return
	}

	// Skip trailing whitespace
	out = line.consumeWhitespace()
	return
}

// Parse a pseudo-op beginning with "." (such as ".EQ").
func (a *assembler) parsePseudoOp(line, label, pseudoOp fstring) (err error) {
	switch pseudoOp.str {
	case ".EQ":
		err = a.parseMacro(line, label)
	default:
		a.addError(line, "Invalid pseudo-op")
		err = errParse
	}
	return
}

// Parse an ".EQ" macro definition.
func (a *assembler) parseMacro(line, label fstring) (err error) {
	a.logLine(line, "macro=%s", label.str)

	// Parse the macro expression.
	var e *expr
	e, line, err = a.exprParser.parse(line, true)
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
	a.logLine(line, "val=$%X", e.number)

	// Track the macro for later substitution.
	a.macros[label.str] = e
	return
}

// Parse a 6502 assembly opcode + operand.
func (a *assembler) parseInstruction(line fstring) (err error) {
	// Parse the opcode
	opcode, out := line.consumeWhile(opcodeChar)

	// No opcode characters? Or opcode has invalid suffix?
	if opcode.isEmpty() || (!out.isEmpty() && !out.startsWith(whitespace)) {
		a.addError(out, "Invalid opcode")
		err = errParse
		return
	}

	// Validate the opcode
	instructions := go6502.GetInstructions(opcode.str)
	if instructions == nil {
		a.addError(opcode, "Invalid opcode")
		err = errParse
		return
	}

	out = out.consumeWhitespace()
	a.logLine(out, "op=%s", opcode.str)

	// Parse the operand, if any
	var operand operand
	operand, out, err = a.parseOperand(out)

	// Create a code segment for the instruction
	seg := segment{opcode: opcode, operand: operand}
	a.segments = append(a.segments, seg)
	return
}

// Parse the operand expression following an opcode.
func (a *assembler) parseOperand(line fstring) (o operand, out fstring, err error) {
	switch {
	case line.isEmpty():
		// Handle immediate mode (no operand)
		o.mode, out = go6502.IMP, line
		return

	case line.startsWithChar('('):
		// Handle indirect addressing modes
		var expr fstring
		o.mode, expr, out, err = line.consume(1).consumeIndirect()
		if err != nil {
			a.addError(out, "Indirect addressing mode syntax error")
			return
		}
		o.expr, _, err = a.exprParser.parse(expr, false)
		if err != nil {
			a.addExprErrors()
			return
		}

	case line.startsWithChar('#'):
		// Handle immediate addressing mode
		o.mode = go6502.IMM
		o.expr, out, err = a.exprParser.parse(line.consume(1), false)
		if err != nil {
			a.addExprErrors()
			return
		}

	default:
		// Handle absolute addressing modes
		var expr fstring
		o.mode, expr, out, err = line.consumeAbsolute()
		if err != nil {
			a.addError(out, "Absolute addressing mode syntax error")
			return
		}
		o.expr, _, err = a.exprParser.parse(expr, false)
		if err != nil {
			a.addExprErrors()
			return
		}
	}

	if !o.expr.eval(a.macros, a.labels) {
		a.uneval = append(a.uneval, o.expr)
	}
	a.logLine(out, "expr=%s", o.expr)
	a.logLine(out, "mode=%d", o.mode)
	a.logLine(out, "val=$%X", o.expr.number)

	if !out.isEmpty() && !out.startsWith(whitespace) {
		a.addError(out, "Syntax error in expression")
		err = errParse
		return
	}
	out = out.consumeWhitespace()
	return
}

// Append an error message to the assembler's error state.
func (a *assembler) addError(l fstring, msg string) {
	a.errors = append(a.errors, asmerror{l, msg})
	if a.verbose {
		fmt.Printf("Error: %s\n", msg)
		fmt.Println(l.full)
		for i := 0; i < l.column; i++ {
			fmt.Printf("-")
		}
		fmt.Println("^")
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
			match, qual = (operand.mode == go6502.IMP) && (operand.size() == 0), 0
		case operand.size() == 0:
			match = false
		case inst.Mode == go6502.IMM:
			match, qual = (operand.mode == go6502.IMM) && (operand.size() == 1), 1
		case inst.Mode == go6502.REL:
			match, qual = (operand.mode == go6502.ABS) && (operand.size() <= 2), 1
		case inst.Mode == go6502.ZPG:
			match, qual = (operand.mode == go6502.ABS) && (operand.size() == 1), 1
		case inst.Mode == go6502.ZPX:
			match, qual = (operand.mode == go6502.ABX) && (operand.size() == 1), 1
		case inst.Mode == go6502.ZPY:
			match, qual = (operand.mode == go6502.ABY) && (operand.size() == 1), 1
		case inst.Mode == go6502.ABS:
			match, qual = (operand.mode == go6502.ABS) && (operand.size() <= 2), 2
		case inst.Mode == go6502.ABX:
			match, qual = (operand.mode == go6502.ABX) && (operand.size() <= 2), 2
		case inst.Mode == go6502.ABY:
			match, qual = (operand.mode == go6502.ABY) && (operand.size() <= 2), 2
		case inst.Mode == go6502.IND:
			match, qual = (operand.mode == go6502.IND) && (operand.size() <= 2), 2
		case inst.Mode == go6502.IDX:
			match, qual = (operand.mode == go6502.IDX) && (operand.size() == 1), 1
		case inst.Mode == go6502.IDY:
			match, qual = (operand.mode == go6502.IDY) && (operand.size() == 1), 1
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
func (l fstring) consumeIndirect() (mode go6502.Mode, expr fstring, out fstring, err error) {
	i := l.scanUntil(func(c byte) bool { return c == ',' || c == ')' })
	expr, out = l.trunc(i), l.consume(i)
	switch {
	case out.startsWithString(",X)"):
		mode, out = go6502.IDX, out.consume(3)
	case out.startsWithString("),Y"):
		mode, out = go6502.IDY, out.consume(3)
	case out.startsWithChar(')'):
		mode, out = go6502.IND, out.consume(1)
	default:
		err = errParse
	}
	out = out.consumeWhitespace()
	if !out.isEmpty() {
		err = errParse
	}
	return
}

// Consume an absolute operand expression until an absolute
// addressing mode substring is reached. Return the candidate
// addressing mode and expression substring.
func (l fstring) consumeAbsolute() (mode go6502.Mode, expr fstring, out fstring, err error) {
	i := l.scanUntil(func(c byte) bool { return c == ',' })
	expr, out = l.trunc(i), l.consume(i)
	switch {
	case out.startsWithString(",X"):
		mode, out = go6502.ABX, out.consume(2)
	case out.startsWithString(",Y"):
		mode, out = go6502.ABY, out.consume(2)
	default:
		mode = go6502.ABS
	}
	out = out.consumeWhitespace()
	if !out.isEmpty() {
		err = errParse
	}
	return
}
