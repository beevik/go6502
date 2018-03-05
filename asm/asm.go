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
	"strconv"
	"strings"

	"github.com/beevik/go6502"
)

// TODO:
//  - Add .PAD pseudo-op

var (
	errParse = errors.New("parse error")
)

var modeName = []string{
	"IMM",
	"IMP",
	"REL",
	"ZPG",
	"ZPX",
	"ZPY",
	"ABS",
	"ABX",
	"ABY",
	"IND",
	"IDX",
	"IDY",
	"ACC",
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

type pseudoOpData struct {
	fn    func(a *assembler, line, label fstring, param interface{}) error
	param interface{}
}

var pseudoOps = map[string]pseudoOpData{
	".eq":     pseudoOpData{fn: (*assembler).parseMacro, param: nil},
	".equ":    pseudoOpData{fn: (*assembler).parseMacro, param: nil},
	"=":       pseudoOpData{fn: (*assembler).parseMacro, param: nil},
	".or":     pseudoOpData{fn: (*assembler).parseOrigin, param: nil},
	".org":    pseudoOpData{fn: (*assembler).parseOrigin, param: nil},
	".db":     pseudoOpData{fn: (*assembler).parseData, param: 1},
	".byte":   pseudoOpData{fn: (*assembler).parseData, param: 1},
	".dw":     pseudoOpData{fn: (*assembler).parseData, param: 2},
	".word":   pseudoOpData{fn: (*assembler).parseData, param: 2},
	".dd":     pseudoOpData{fn: (*assembler).parseData, param: 4},
	".dword":  pseudoOpData{fn: (*assembler).parseData, param: 4},
	".dh":     pseudoOpData{fn: (*assembler).parseHexString, param: nil},
	".hs":     pseudoOpData{fn: (*assembler).parseHexString, param: nil},
	".align":  pseudoOpData{fn: (*assembler).parseAlign, param: nil},
	".ex":     pseudoOpData{fn: (*assembler).parseExport, param: nil},
	".export": pseudoOpData{fn: (*assembler).parseExport, param: nil},
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
		offset, _ := relOffset(i.operand.getValue(), i.addr+int(i.inst.Length))
		return byteString([]byte{i.inst.Opcode, offset})
	case sz == 0:
		return byteString([]byte{i.inst.Opcode})
	case sz == 1:
		return byteString([]byte{i.inst.Opcode, byte(i.operand.getValue())})
	default:
		return byteString([]byte{i.inst.Opcode, byte(i.operand.getValue()), byte(i.operand.getValue() >> 8)})
	}
}

// Format an operand string based on the instruction's addressing mode.
func (i *instruction) operandString() string {
	number := i.operand.getValue()

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
	modeGuess     go6502.Mode // addressing mode guesed based on operand string
	expr          *expr       // expression tree, used to resolve value
	forceAbsolute bool        // operand must use 2-byte absolute address
	forceLSB      bool        // operand must use least significant byte
	forceMSB      bool        // operand must use most significant byte
}

func (o *operand) getValue() int {
	v := o.expr.value
	if v < 0 {
		v = 0x10000 + v
	}

	switch {
	case o.forceLSB:
		return v & 0xff
	case o.forceMSB:
		return (v >> 8) & 0xff
	default:
		return v
	}
}

// Return the size of the operand in bytes.
func (o *operand) size() int {
	switch {
	case o.modeGuess == go6502.IMP:
		return 0
	case o.forceLSB || o.forceMSB:
		return 1
	case o.expr.address || o.forceAbsolute || o.expr.value > 0xff || o.expr.value < -128:
		return 2
	default:
		return 1
	}
}

// A data segment contains 1 or more expressions that are evaluated to
// produce binary data.
type data struct {
	addr  int     // address assigned to the segment
	unit  int     // unit size (1 or 2 bytes)
	exprs []*expr // all expressions in the data segment
}

func (d *data) address() int {
	return d.addr
}

func (d *data) bytes() int {
	n := 0
	for _, e := range d.exprs {
		if e.isString {
			n += len(e.stringLiteral.str)
		} else {
			n += d.unit
		}
	}
	return n
}

// A bytes segment contains raw binary data.
type bytedata struct {
	addr int
	b    []byte
}

func (b *bytedata) address() int {
	return b.addr
}

// An alignment segment contains alignment data.
type alignment struct {
	addr  int
	align int
	pad   int
}

func (a *alignment) address() int {
	return a.addr
}

// An export segment contains an exported address.
type export struct {
	addr int
	expr *expr
}

func (e *export) address() int {
	return e.addr
}

// An asmerror is used to keep track of errors encountered
// during assembly.
type asmerror struct {
	line fstring // row & column of assembly code causing the error
	msg  string  // error message
}

// An unevaluated expression
type uneval struct {
	expr  *expr
	segno int // The expression's segment index
}

// The assembler is a state object used during the assembly of
// machine code from assembly code.
type assembler struct {
	origin      int              // requested origin
	pc          int              // the program counter
	code        []byte           // generated machine code
	scanner     *bufio.Scanner   // scans the io reader
	scopeLabel  fstring          // label currently in scope
	macros      map[string]*expr // macro -> expression
	labels      map[string]int   // label -> segment index
	exports     []Export         // exported addresses
	segments    []segment        // segment of machine code
	unevaluated []uneval         // expressions requiring evaluation
	verbose     bool             // verbose output
	exprParser  exprParser       // used to parse math expressions
	errors      []asmerror       // errors encountered during assembly
}

// An Export describes an exported address.
type Export struct {
	Label string
	Addr  go6502.Address
}

// Result of the Assemble function.
type Result struct {
	Code    []byte         // Assembled machine code
	Origin  go6502.Address // Code origin address
	Exports []Export       // Exported addresses
}

// Assemble reads data from the provided stream and attempts to assemble
// it into 6502 byte code.
func Assemble(r io.Reader, verbose bool) (*Result, error) {
	a := &assembler{
		origin:   0x600,
		pc:       -1,
		scanner:  bufio.NewScanner(r),
		macros:   make(map[string]*expr),
		labels:   make(map[string]int),
		exports:  make([]Export, 0),
		segments: make([]segment, 0, 32),
		verbose:  verbose,
	}

	// Assembly consists of the following steps
	steps := []func(a *assembler) error{
		(*assembler).parse,                        // Parse the assembly code
		(*assembler).evaluateExpressions,          // Evaluate operand & macro expressions
		(*assembler).assignAddresses,              // Assign addresses to instructions
		(*assembler).resolveLabels,                // Resolve labels to addresses
		(*assembler).evaluateExpressions,          // Do another evaluation pass with resolved labels
		(*assembler).handleUnevaluatedExpressions, // Cause error if there are unevaluated expressions
		(*assembler).generateCode,                 // Generate the machine code
	}

	// Execute assembler steps, breaking if an error is encountered
	// in any one of them.
	for _, step := range steps {
		err := step(a)
		if err != nil {
			return nil, err
		}
		if len(a.errors) > 0 {
			return nil, errParse
		}
	}

	result := &Result{
		Code:    a.code,
		Origin:  go6502.Address(a.origin),
		Exports: a.exports,
	}
	return result, nil
}

// Read the assembly code and perform the initial parsing. Build up
// machine code segments, the macro table, the label table, and a
// list of unevaluated expression trees.
func (a *assembler) parse() error {
	a.logSection("Parsing assembly code")
	row := 1
	for a.scanner.Scan() {
		text := a.scanner.Text()
		line := newFstring(row, text)
		err := a.parseLine(line.stripTrailingComment())
		if err != nil {
			return err
		}
		row++
	}
	return nil
}

// Add an expression to the "unevaluated" list.
func (a *assembler) pushUnevaluated(e *expr) {
	a.unevaluated = append(a.unevaluated, uneval{expr: e, segno: len(a.segments)})
}

// Return the address assigned to the requested segment.
func (a *assembler) segaddr(segno int) int {
	switch {
	case segno < len(a.segments):
		return a.segments[segno].address()
	default:
		return a.pc
	}
}

// Evaluate all unevaluated expression trees using macros and labels.
func (a *assembler) evaluateExpressions() error {
	a.logSection("Evaluating expressions")
	for {
		var unevaluated []uneval
		for _, u := range a.unevaluated {
			if u.expr.eval(a.segaddr(u.segno), a.macros, a.labels) {
				a.log("%-25s Val:$%X", u.expr.String(), u.expr.value)
			} else {
				a.log("%-25s Val:????? isaddr:%v", u.expr.String(), u.expr.address)
				unevaluated = append(unevaluated, u)
			}
		}
		if len(unevaluated) == len(a.unevaluated) {
			break
		}
		a.unevaluated = unevaluated
	}
	return nil
}

// Determine addresses of all code segments.
func (a *assembler) assignAddresses() error {
	a.logSection("Assigning addresses")
	a.pc = a.origin
	for _, s := range a.segments {
		switch ss := s.(type) {
		case *instruction:
			ss.addr = a.pc
			ss.inst = findMatchingInstruction(ss.opcode, ss.operand)
			if ss.inst == nil {
				a.addError(ss.opcode, "invalid addressing mode for opcode '%s'", ss.opcode.str)
				return errParse
			}
			a.log("%04X  %s Len:%d Mode:%s Opcode:%02X",
				ss.addr, ss.opcode.str, ss.inst.Length,
				modeName[ss.inst.Mode], ss.inst.Opcode)
			a.pc += int(ss.inst.Length)

		case *data:
			ss.addr = a.pc
			bytes := ss.bytes()
			a.log("%04X  .DB Len:%d", ss.addr, bytes)
			a.pc += bytes

		case *bytedata:
			ss.addr = a.pc
			a.log("%04X  .HS Len:%d", ss.addr, len(ss.b))
			a.pc += len(ss.b)

		case *alignment:
			ss.addr = a.pc
			ss.pad = ss.align*((a.pc+ss.align-1)/ss.align) - a.pc
			a.log("%04X  .ALIGN Len:%d", ss.addr, ss.pad)
			a.pc += ss.pad

		case *export:
			ss.addr = a.pc
		}
	}
	return nil
}

// Resolve all labels to addresses.
func (a *assembler) resolveLabels() error {
	a.logSection("Resolving labels")
	for label, segno := range a.labels {
		addr := a.segaddr(segno)
		a.log("%-15s Seg:%-3d Addr:$%04X", label, segno, addr)
		a.macros[label] = &expr{op: opNumber, value: addr, evaluated: true}
	}
	return nil
}

// Cause an error if there are any unevaluated expressions.
func (a *assembler) handleUnevaluatedExpressions() error {
	if len(a.unevaluated) > 0 {
		for _, u := range a.unevaluated {
			a.addError(u.expr.line, "unresolved expression")
		}
		return errParse
	}
	return nil
}

// Generate machine code.
func (a *assembler) generateCode() error {
	a.logSection("Generating code")
	for _, s := range a.segments {
		switch ss := s.(type) {
		case *instruction:
			a.code = append(a.code, ss.inst.Opcode)
			switch {
			case ss.inst.Length == 1:
				a.log("%04X- %-8s  %s", ss.addr, ss.codeString(), ss.opcode.str)
			case ss.inst.Mode == go6502.REL:
				offset, err := relOffset(ss.operand.getValue(), ss.addr+int(ss.inst.Length))
				if err != nil {
					a.addError(ss.opcode, "branch offset out of bounds")
				}
				a.code = append(a.code, offset)
				a.log("%04X- %-8s  %s  %s", ss.addr, ss.codeString(), ss.opcode.str, ss.operandString())
			case ss.inst.Length == 2:
				a.code = append(a.code, byte(ss.operand.getValue()))
				a.log("%04X- %-8s  %s  %s", ss.addr, ss.codeString(), ss.opcode.str, ss.operandString())
			case ss.inst.Length == 3:
				a.code = append(a.code, toBytes(2, ss.operand.getValue())...)
				a.log("%04X- %-8s  %s  %s", ss.addr, ss.codeString(), ss.opcode.str, ss.operandString())
			default:
				panic("invalid operand")
			}

		case *data:
			start := len(a.code)
			for _, e := range ss.exprs {
				switch {
				case e.isString:
					a.code = append(a.code, []byte(e.stringLiteral.str)...)
				default:
					a.code = append(a.code, toBytes(ss.unit, e.value)...)
				}
			}
			a.logBytes(ss.addr, a.code[start:])

		case *bytedata:
			a.code = append(a.code, ss.b...)
			a.logBytes(ss.addr, ss.b)

		case *alignment:
			pad := make([]byte, ss.pad)
			a.code = append(a.code, pad...)
			a.logBytes(ss.addr, pad)

		case *export:
			if ss.expr.op != opIdentifier || !ss.expr.address {
				a.addError(ss.expr.line, "export is not an address label")
			}
			export := Export{
				Label: ss.expr.identifier.str,
				Addr:  go6502.Address(ss.expr.value),
			}
			a.exports = append(a.exports, export)
		}
	}
	return nil
}

// Parse a single line of assembly code.
func (a *assembler) parseLine(line fstring) error {
	// Skip empty (or comment-only) lines
	if line.isEmpty() {
		return nil
	}

	a.log("---")

	if line.startsWith(whitespace) {
		return a.parseUnlabeledLine(line.consumeWhitespace())
	}
	return a.parseLabeledLine(line)
}

// Parse a line of assembly code that contains no label.
func (a *assembler) parseUnlabeledLine(line fstring) error {
	a.logLine(line, "unlabeled_line")

	// Is the next word a pseudo-op, rather than an opcode?
	if line.startsWith(pseudoOpStartChar) {
		var pseudoOp fstring
		pseudoOp, line = line.consumeWhile(wordChar)
		return a.parsePseudoOp(line.consumeWhitespace(), fstring{}, pseudoOp)
	}

	return a.parseInstruction(line)
}

// Parse a line of assembly code that starts with a label.
func (a *assembler) parseLabeledLine(line fstring) error {
	a.logLine(line, "labeled_line")

	// Parse the label field
	label, line, err := a.parseLabel(line)
	if err != nil {
		return err
	}

	// Is the next word a pseudo-op, rather than an opcode?
	if line.startsWith(pseudoOpStartChar) {
		var pseudoOp fstring
		pseudoOp, line = line.consumeWhile(wordChar)
		return a.parsePseudoOp(line.consumeWhitespace(), label, pseudoOp)
	}

	// Store the label.
	err = a.storeLabel(label)
	if err != nil {
		return err
	}

	// Parse any instruction following the label
	if !line.isEmpty() {
		return a.parseInstruction(line)
	}
	return nil
}

// Store a label into the assembler's label list.
func (a *assembler) storeLabel(label fstring) error {
	// If the label starts with '.', it is a local label. So append
	// it to the active scope label.
	if label.startsWithChar('.') {
		if a.scopeLabel.isEmpty() {
			a.addError(label, "no global label '%s' previously defined", label.str)
			return errParse
		}
		label.str = "~" + a.scopeLabel.str + label.str
	} else {
		a.scopeLabel = label
	}

	if _, found := a.labels[label.str]; found {
		a.addError(label, "label '%s' used more than once", label.str)
		return errParse
	}

	// Associate the label with its segment number.
	segno := len(a.segments)
	a.labels[label.str] = segno
	a.logLine(label, "label=%s", label.str)
	a.logLine(label, "seg=%d", segno)
	return nil
}

// Parse a label string at the beginning of a line of assembly code.
func (a *assembler) parseLabel(line fstring) (label fstring, remain fstring, err error) {
	if !line.startsWith(labelStartChar) {
		s, _ := line.consumeUntil(whitespace)
		a.addError(line, "invalid label '%s'", s.str)
		return fstring{}, line, errParse
	}

	label, line = line.consumeWhile(labelChar)

	// Skip colon after label.
	if line.startsWithChar(':') {
		line = line.consume(1)
	}

	if !line.isEmpty() && !line.startsWith(whitespace) {
		s, _ := line.consumeUntil(whitespace)
		a.addError(line, "invalid label '%s%s'", label.str, s.str)
		return fstring{}, line, errParse
	}

	remain = line.consumeWhitespace()
	return label, remain, nil
}

// Parse a pseudo-op beginning with "." (such as ".EQ").
func (a *assembler) parsePseudoOp(line, label, pseudoOp fstring) error {
	op, ok := pseudoOps[strings.ToLower(pseudoOp.str)]
	if !ok {
		a.addError(pseudoOp, "invalid directive '%s'", pseudoOp.str)
		return errParse
	}
	return op.fn(a, line, label, op.param)
}

// Parse an ".EQ" macro definition.
func (a *assembler) parseMacro(line, label fstring, param interface{}) error {
	if label.str == "" {
		a.addError(line, "macro must begin with a label")
		return errParse
	}

	a.logLine(line, "macro=%s", label.str)

	// Parse the macro expression.
	e, _, err := a.exprParser.parse(line, a.scopeLabel, allowParentheses)
	if err != nil {
		a.addExprErrors()
		return err
	}

	// Attempt to evaluate the macro expression immediately. If not possible,
	// add it to a list of unevaluated expressions.
	if !e.eval(-1, a.macros, a.labels) {
		a.pushUnevaluated(e)
	}

	a.logLine(line, "expr=%s", e.String())
	switch e.evaluated {
	case true:
		a.logLine(line, "val=$%X", e.value)
	case false:
		a.logLine(line, "val=(uneval)")
	}

	// Track the macro for later substitution.
	a.macros[label.str] = e
	return nil
}

// Parse an ".ORG" origin definition
func (a *assembler) parseOrigin(line, label fstring, param interface{}) error {
	if len(a.segments) > 0 {
		a.addError(line, "origin directive must appear before first instruction")
		return errParse
	}

	a.logLine(line, "origin=")

	e, _, err := a.exprParser.parse(line, a.scopeLabel, allowParentheses)
	if err != nil {
		a.addExprErrors()
		return errParse
	}

	if !e.eval(-1, a.macros, a.labels) {
		a.addError(e.identifier, "unable to evaluate expression")
		return errParse
	}

	a.logLine(line, "expr=%s", e.String())
	a.logLine(line, "val=$%04X", e.value)

	a.origin = e.value
	return nil
}

// Parse a data pseudo-op.
func (a *assembler) parseData(line, label fstring, param interface{}) error {
	a.logLine(line, "bytes=")

	seg := &data{
		unit: param.(int),
		addr: -1,
	}

	remain := line
	for !remain.isEmpty() {
		var expr fstring
		expr, remain = remain.consumeUntilChar(',')

		if !remain.isEmpty() {
			remain = remain.consume(1).consumeWhitespace()
		}

		e, _, err := a.exprParser.parse(expr, a.scopeLabel, allowParentheses|allowStrings)
		if err != nil {
			a.addExprErrors()
			return err
		}

		if !e.eval(-1, a.macros, a.labels) {
			a.pushUnevaluated(e)
		}

		seg.exprs = append(seg.exprs, e)
	}

	if !label.isEmpty() {
		err := a.storeLabel(label)
		if err != nil {
			return err
		}
	}

	a.segments = append(a.segments, seg)
	return nil
}

// Parse a hex-string pseudo-op.
func (a *assembler) parseHexString(line, label fstring, param interface{}) error {
	a.logLine(line, "hexstring=")

	s, remain := line.consumeWhile(hexadecimal)
	if !remain.isEmpty() {
		a.addError(remain, "invalid hex string")
		return errParse
	}

	if len(s.str)%2 != 0 {
		a.addError(s, "hex-string has odd number of characters")
		return errParse
	}

	seg := &bytedata{addr: -1}

	for i := 0; i < len(s.str); i += 2 {
		v := hexToByte(s.str[i:])
		seg.b = append(seg.b, v)
	}

	a.segments = append(a.segments, seg)
	return nil
}

func (a *assembler) parseAlign(line, label fstring, param interface{}) error {
	a.logLine(line, "align=")

	s, remain := line.consumeWhile(decimal)
	if !remain.isEmpty() {
		a.addError(remain, "invalid alignment")
		return errParse
	}

	v, _ := strconv.ParseInt(s.str, 10, 32)
	if (v&(v-1)) != 0 || v > 0x100 {
		a.addError(s, "alignment must be power of 2")
		return errParse
	}

	seg := &alignment{addr: -1, align: int(v)}

	a.segments = append(a.segments, seg)
	return nil
}

// Parse an export pseudo-op
func (a *assembler) parseExport(line, label fstring, param interface{}) error {
	a.logLine(line, "export=")

	// Parse the export expression.
	e, _, err := a.exprParser.parse(line, a.scopeLabel, allowParentheses)
	if err != nil {
		a.addExprErrors()
		return err
	}

	// Attempt to evaluate the expression immediately.
	if !e.eval(-1, a.macros, a.labels) {
		a.pushUnevaluated(e)
	}

	a.logLine(line, "expr=%s", e.String())
	switch e.evaluated {
	case true:
		a.logLine(line, "val=$%X", e.value)
	case false:
		a.logLine(line, "val=(uneval)")
	}

	seg := &export{addr: -1, expr: e}
	a.segments = append(a.segments, seg)
	return nil
}

// Parse a 6502 assembly opcode + operand.
func (a *assembler) parseInstruction(line fstring) error {
	// Parse the opcode.
	opcode, remain := line.consumeWhile(opcodeChar)

	// No opcode characters? Or opcode has invalid suffix?
	if opcode.isEmpty() || (!remain.isEmpty() && !remain.startsWith(whitespace)) {
		a.addError(remain, "invalid opcode '%s'", opcode.str)
		return errParse
	}

	// Validate the opcode
	instructions := go6502.GetInstructions(opcode.str)
	if instructions == nil {
		a.addError(opcode, "invalid opcode '%s'", opcode.str)
		return errParse
	}

	remain = remain.consumeWhitespace()
	a.logLine(remain, "op=%s", opcode.str)

	// Parse the operand, if any.
	operand, remain, err := a.parseOperand(remain)
	if err != nil {
		return err
	}

	// Create a code segment for the instruction
	seg := &instruction{addr: -1, opcode: opcode, operand: operand}
	a.segments = append(a.segments, seg)
	return nil
}

// Parse the operand expression following an opcode.
func (a *assembler) parseOperand(line fstring) (o operand, remain fstring, err error) {
	switch {
	case line.isEmpty():
		o.modeGuess, remain = go6502.IMP, line
		return

	case line.startsWithChar('('):
		var expr fstring
		o.modeGuess, expr, remain, err = line.consume(1).consumeIndirect()
		if err != nil {
			a.addError(remain, "unknown addressing mode format")
			return
		}
		o.expr, _, err = a.exprParser.parse(expr, a.scopeLabel, 0)
		if err != nil {
			a.addExprErrors()
			return
		}

	case line.startsWithChar('#'):
		o.modeGuess = go6502.IMM
		o.forceLSB = true
		o.expr, remain, err = a.exprParser.parse(line.consume(1), a.scopeLabel, allowParentheses)
		if err != nil {
			a.addExprErrors()
			return
		}

	case line.startsWithChar('/'):
		o.modeGuess = go6502.IMM
		o.forceMSB = true
		o.expr, remain, err = a.exprParser.parse(line.consume(1), a.scopeLabel, allowParentheses)
		if err != nil {
			a.addExprErrors()
			return
		}

	case line.startsWithChar('A') && (line.startsWithString("A:") || line.startsWithString("ABS:")):
		o.forceAbsolute = true
		_, line = line.consumeUntilChar(':')
		line = line.consume(1)
		fallthrough

	default:
		var expr fstring
		o.modeGuess, expr, remain, err = line.consumeAbsolute()
		if err != nil {
			a.addError(remain, "unknown addressing mode format")
			return
		}
		o.expr, _, err = a.exprParser.parse(expr, a.scopeLabel, 0)
		if err != nil {
			a.addExprErrors()
			return
		}
	}

	if !o.expr.eval(-1, a.macros, a.labels) {
		a.pushUnevaluated(o.expr)
	}

	a.logLine(remain, "expr=%s", o.expr)
	a.logLine(remain, "mode=%s", modeName[o.modeGuess])
	switch o.expr.evaluated {
	case true:
		a.logLine(remain, "val=$%X", o.getValue())
	default:
		a.logLine(remain, "val=(uneval)")
	}

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
	fmt.Fprintf(os.Stderr, "Syntax error line %d, col %d: %s\n", l.row, l.column+1, msg)
	if a.verbose {
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
		fmt.Printf("%-3d %-3d | %-20s | %s\n", line.row, line.column+1, detail, line.str)
	}
}

// In verbose mode, log a series of bytes with starting address.
func (a *assembler) logBytes(addr int, b []byte) {
	if a.verbose {
		for i, n := 0, len(b); i < n; i += 3 {
			j := i + 3
			if j > n {
				j = n
			}
			a.log("%04X-*%s", addr+i, byteString(b[i:j]))
		}
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
			match, qual = (operand.modeGuess == go6502.ABS), 1
		case inst.Mode == go6502.ZPG:
			match, qual = (operand.modeGuess == go6502.ABS) && (operand.size() == 1), 1
		case inst.Mode == go6502.ZPX:
			match, qual = (operand.modeGuess == go6502.ABX) && (operand.size() == 1), 1
		case inst.Mode == go6502.ZPY:
			match, qual = (operand.modeGuess == go6502.ABY) && (operand.size() == 1), 1
		case inst.Mode == go6502.ABS:
			match, qual = (operand.modeGuess == go6502.ABS), 2
		case inst.Mode == go6502.ABX:
			match, qual = (operand.modeGuess == go6502.ABX), 2
		case inst.Mode == go6502.ABY:
			match, qual = (operand.modeGuess == go6502.ABY), 2
		case inst.Mode == go6502.IND:
			match, qual = (operand.modeGuess == go6502.IND), 2
		case inst.Mode == go6502.IDX:
			match, qual = (operand.modeGuess == go6502.IDX) && (operand.size() == 1), 1
		case inst.Mode == go6502.IDY:
			match, qual = (operand.modeGuess == go6502.IDY) && (operand.size() == 1), 1
		}
		if match && qual < bestqual {
			bestqual, found = qual, inst
		}
	}
	return found
}

// Consume an operand expression starting with '(' until
// an indirect addressing mode substring is reached. Return
// the candidate addressing mode and expression substring.
func (l fstring) consumeIndirect() (mode go6502.Mode, expr fstring, remain fstring, err error) {
	expr, remain = l.consumeUntil(func(c byte) bool { return c == ',' || c == ')' })

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

	return mode, expr, remain, err
}

// Consume an absolute operand expression until an absolute
// addressing mode substring is reached. Guess the addressing mode,
// and return the expression substring.
func (l fstring) consumeAbsolute() (mode go6502.Mode, expr fstring, remain fstring, err error) {
	expr, remain = l.consumeUntilChar(',')

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

	return mode, expr, remain, err
}
