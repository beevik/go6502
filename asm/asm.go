// Copyright 2014-2018 Brett Vickers. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package asm implements a 6502 macro assembler.
package asm

import (
	"bufio"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/beevik/go6502/cpu"
)

var (
	errParse = errors.New("parse error")
)

const (
	binSignature       = "go65"
	sourceMapSignature = "sm65"
	versionMajor       = 0
	versionMinor       = 1
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
	fn    func(a *assembler, line, label fstring, param any) error
	param any
}

const hiBitTerm = 1 << 16

var pseudoOps = map[string]pseudoOpData{
	".ar":      {fn: (*assembler).parseArch},
	".arch":    {fn: (*assembler).parseArch},
	"arch":     {fn: (*assembler).parseArch},
	".bin":     {fn: (*assembler).parseBinaryInclude},
	".binary":  {fn: (*assembler).parseBinaryInclude},
	".eq":      {fn: (*assembler).parseEquate},
	".equ":     {fn: (*assembler).parseEquate},
	"equ":      {fn: (*assembler).parseEquate},
	"=":        {fn: (*assembler).parseEquate},
	".or":      {fn: (*assembler).parseOrigin},
	".org":     {fn: (*assembler).parseOrigin},
	"org":      {fn: (*assembler).parseOrigin},
	".db":      {fn: (*assembler).parseData, param: 1},
	".byte":    {fn: (*assembler).parseData, param: 1},
	".dw":      {fn: (*assembler).parseData, param: 2},
	".word":    {fn: (*assembler).parseData, param: 2},
	".dd":      {fn: (*assembler).parseData, param: 4},
	".dword":   {fn: (*assembler).parseData, param: 4},
	".dh":      {fn: (*assembler).parseHexString},
	".hex":     {fn: (*assembler).parseHexString},
	"hex":      {fn: (*assembler).parseHexString},
	".ds":      {fn: (*assembler).parseData, param: 1 | hiBitTerm},
	".tstring": {fn: (*assembler).parseData, param: 1 | hiBitTerm},
	".al":      {fn: (*assembler).parseAlign},
	".align":   {fn: (*assembler).parseAlign},
	".pad":     {fn: (*assembler).parsePadding},
	".ex":      {fn: (*assembler).parseExport},
	".export":  {fn: (*assembler).parseExport},
	"exp":      {fn: (*assembler).parseExport},
}

func init() {
	// The .include pseudo-op must be initialized here to bypass go's overly
	// aggressive initialization loop detection.
	pseudoOps[".in"] = pseudoOpData{fn: (*assembler).parseInclude}
	pseudoOps[".include"] = pseudoOpData{fn: (*assembler).parseInclude}
	pseudoOps["include"] = pseudoOpData{fn: (*assembler).parseInclude}
}

// A segment is a small chunk of machine code that may represent a single
// instruction or a group of byte data.
type segment interface {
	address() int
}

// An instruction segment contains a single instruction, including its
// opcode and operand data.
type instruction struct {
	addr      int              // address assigned to the segment
	fileIndex int              // index of file containing the instruction
	line      int              // the source code line number
	opcode    fstring          // opcode string
	inst      *cpu.Instruction // selected instruction data for the opcode
	operand   operand          // parameter data for the instruction
}

func (i *instruction) address() int {
	return i.addr
}

// Format a byte code string for an instruction.
func (i *instruction) codeString() string {
	sz := i.inst.Length - 1
	switch {
	case i.inst.Mode == cpu.REL:
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
	modeGuess      cpu.Mode // addressing mode guesed based on operand string
	expr           *expr    // expression tree, used to resolve value
	forceImmediate bool     // operand forces an immediate addressing mode
	forceAbsolute  bool     // operand must use 2-byte absolute address
}

func (o *operand) getValue() int {
	v := o.expr.value
	if v < 0 {
		v = 0x10000 + v
	}

	switch {
	case o.forceImmediate:
		return v & 0xff
	default:
		return v
	}
}

// Return the size of the operand in bytes.
func (o *operand) size() int {
	switch {
	case o.modeGuess == cpu.IMP:
		return 0
	case o.forceImmediate:
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
	addr      int     // address assigned to the segment
	unit      int     // unit size (1 or 2 bytes)
	hiBitTerm bool    // terminate last char of string by setting hi bit
	exprs     []*expr // all expressions in the data segment
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

// A padding segment contains padding character and length expressions.
type padding struct {
	addr    int
	pad     int
	value   byte
	valExpr *expr
	lenExpr *expr
}

func (p *padding) address() int {
	return p.addr
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
	line fstring // line causing the error
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
	arch        cpu.Architecture    // requested architecture
	instSet     *cpu.InstructionSet // instructions on current arch
	origin      int                 // requested origin
	pc          int                 // the program counter
	code        []byte              // generated machine code
	r           io.Reader           // the reader passed to Assemble
	scopeLabel  fstring             // label currently in scope
	constants   map[string]*expr    // constant -> expression
	labels      map[string]int      // label -> segment index
	exports     []Export            // exported addresses
	sourceLines []SourceLine        // source code line mappings
	files       []string            // processed files
	segments    []segment           // segment of machine code
	unevaluated []uneval            // expressions requiring evaluation
	out         io.Writer           // output used for verbose output
	verbose     bool                // verbose output
	exprParser  exprParser          // used to parse math expressions
	errors      []asmerror          // errors encountered during assembly
}

// An Export describes an exported address.
type Export struct {
	Label   string
	Address uint16
}

// Assembly contains the assembled machine code and other data associated with
// the machine code.
type Assembly struct {
	Code   []byte   // Assembled machine code
	Errors []string // Errors encountered during assembly
}

// ReadFrom reads machine code from a binary input source.
func (a *Assembly) ReadFrom(r io.Reader) (n int64, err error) {
	a.Errors = []string{}
	a.Code, err = io.ReadAll(r)
	n = int64(len(a.Code))
	if n > 0x10000 {
		return n, fmt.Errorf("code exceeded 64K size")
	}
	return n, err
}

// WriteTo saves machine code as binary data into an output writer.
func (a *Assembly) WriteTo(w io.Writer) (n int64, err error) {
	nn, err := w.Write(a.Code)
	return int64(nn), err
}

// Option type used by the Assembly function.
type Option uint

// Options for the Assemble function.
const (
	Verbose Option = 1 << iota // verbose output during assembly
)

const defaultOrigin = 0x1000

// AssembleFile reads a file containing 6502 assembly code, assembles it,
// and produces a binary output file and a source map file.
func AssembleFile(path string, options Option, out io.Writer) error {
	inFile, err := os.Open(path)
	if err != nil {
		return err
	}
	defer inFile.Close()

	assembly, sourceMap, err := Assemble(inFile, path, defaultOrigin, out, options)
	if err != nil {
		for _, e := range assembly.Errors {
			fmt.Fprintln(out, e)
		}
		return err
	}

	ext := filepath.Ext(path)
	prefix := path[:len(path)-len(ext)]
	binPath := prefix + ".bin"
	binFile, err := os.OpenFile(binPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer binFile.Close()

	_, err = assembly.WriteTo(binFile)
	if err != nil {
		return err
	}

	mapPath := prefix + ".map"
	mapFile, err := os.OpenFile(mapPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer mapFile.Close()

	_, err = sourceMap.WriteTo(mapFile)
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "Assembled '%s' to produce '%s' and '%s'.\n",
		filepath.Base(path),
		filepath.Base(binPath),
		filepath.Base(mapPath))
	return nil
}

// Assemble reads data from the provided stream and attempts to assemble it
// into 6502 byte code.
func Assemble(r io.Reader, filename string, origin uint16, out io.Writer, options Option) (*Assembly, *SourceMap, error) {
	if out == nil {
		out = os.Stdout
	}

	a := &assembler{
		arch:      cpu.NMOS,
		instSet:   cpu.GetInstructionSet(cpu.NMOS),
		origin:    int(origin),
		pc:        -1,
		r:         r,
		constants: make(map[string]*expr),
		labels:    make(map[string]int),
		files:     []string{filename},
		exports:   make([]Export, 0),
		segments:  make([]segment, 0, 32),
		out:       out,
		verbose:   (options & Verbose) != 0,
	}

	// Assembly consists of the following steps
	steps := []func(a *assembler) error{
		(*assembler).parse,                        // Parse the assembly code
		(*assembler).evaluateExpressions,          // Evaluate operand & constant expressions
		(*assembler).assignAddresses,              // Assign addresses to instructions
		(*assembler).resolveLabels,                // Resolve labels to addresses
		(*assembler).evaluateExpressions,          // Do another evaluation pass with resolved labels
		(*assembler).handleUnevaluatedExpressions, // Cause error if there are unevaluated expressions
		(*assembler).generateCode,                 // Generate the machine code
	}

	// Execute assembler steps, breaking if an error is encountered
	// in any one of them.
	var err error
	for _, step := range steps {
		err = step(a)
		if err != nil {
			break
		}
		if len(a.errors) > 0 {
			err = errParse
			break
		}
	}

	errors := make([]string, 0, len(a.errors))
	for _, e := range a.errors {
		filename := a.files[e.line.fileIndex]
		s := fmt.Sprintf("Syntax error in '%s' line %d, col %d: %s", filename, e.line.row, e.line.column+1, e.msg)
		errors = append(errors, s)
	}

	assembly := &Assembly{
		Code:   a.code,
		Errors: errors,
	}

	sourceMap := &SourceMap{
		Origin:  uint16(a.origin),
		Size:    uint32(len(a.code)),
		CRC:     crc32.ChecksumIEEE(a.code),
		Files:   a.files,
		Lines:   a.sourceLines,
		Exports: sortExports(a.exports),
	}

	return assembly, sourceMap, err
}

// Read the assembly code and perform the initial parsing. Build up
// machine code segments, the constants table, the label table, and a
// list of unevaluated expression trees.
func (a *assembler) parse() error {
	a.logSection("Parsing assembly code")

	err := a.parseFile(bufio.NewScanner(a.r), 0)
	if err != nil {
		return err
	}

	// Add an empty byte-data segment to the end of the file, just so the
	// end of the file can be assigned an address and any labels attached
	// to the end of the file will be valid.
	seg := &bytedata{addr: -1, b: []byte{}}
	a.segments = append(a.segments, seg)

	return nil
}

// Parse a single file. This may be called to parse the original file
// passed to the assembler, or it may be called in response to including
// a file.
func (a *assembler) parseFile(scanner *bufio.Scanner, fileIndex int) error {
	row := 1
	for scanner.Scan() {
		text := scanner.Text()
		line := newFstring(fileIndex, row, text)
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
	if segno < len(a.segments) {
		return a.segments[segno].address()
	}
	return -1
}

// Evaluate all unevaluated expression trees using constants and labels.
func (a *assembler) evaluateExpressions() error {
	a.logSection("Evaluating expressions")
	for {
		var unevaluated []uneval
		for _, u := range a.unevaluated {
			if u.expr.eval(a.segaddr(u.segno), a.constants, a.labels) {
				a.log("%-25s Val:$%X", u.expr.String(), u.expr.value)
			} else {
				a.log("%-25s Val:??? isaddr:%v", u.expr.String(), u.expr.address)
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
			ss.inst = a.findMatchingInstruction(ss.opcode, ss.operand)
			if ss.inst == nil {
				a.addError(ss.opcode, "invalid addressing mode for opcode '%s'", ss.opcode.str)
				return errParse
			}

			l := SourceLine{
				Address:   ss.addr,
				FileIndex: ss.fileIndex,
				Line:      ss.line,
			}
			a.sourceLines = append(a.sourceLines, l)

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
			a.log("%04X  .BIN Len:%d", ss.addr, len(ss.b))
			a.pc += len(ss.b)

		case *alignment:
			ss.addr = a.pc
			ss.pad = ss.align*((a.pc+ss.align-1)/ss.align) - a.pc
			a.log("%04X  .ALIGN Len:%d", ss.addr, ss.pad)
			a.pc += ss.pad

		case *padding:
			ss.addr = a.pc
			if !ss.valExpr.evaluated || !ss.lenExpr.evaluated {
				a.resolveLabels()
				a.evaluateExpressions()
				if !ss.valExpr.evaluated {
					a.addError(ss.valExpr.line, "padding value expression could not be evaluated")
					return errParse
				}
				if !ss.lenExpr.evaluated {
					a.addError(ss.lenExpr.line, "padding length expression could not be evaluated")
					return errParse
				}
			}
			ss.value = byte(ss.valExpr.value)
			ss.pad = maxInt(0, ss.lenExpr.value)
			a.log("%04X  .PAD Len:%d Val:%d", ss.addr, ss.pad, ss.value)
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
		if _, ok := a.constants[label]; ok {
			continue
		}
		addr := a.segaddr(segno)
		if addr != -1 {
			a.log("%-15s Seg:%-3d Addr:$%04X", label, segno, addr)
			a.constants[label] = &expr{op: opNumber, value: addr, evaluated: true}
		}
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
				a.log("%04X-   %-8s    %s", ss.addr, ss.codeString(), ss.opcode.str)
			case ss.inst.Mode == cpu.REL:
				offset, err := relOffset(ss.operand.getValue(), ss.addr+int(ss.inst.Length))
				if err != nil {
					a.addError(ss.opcode, "branch offset out of bounds")
				}
				a.code = append(a.code, offset)
				a.log("%04X-   %-8s    %s   %s", ss.addr, ss.codeString(), ss.opcode.str, ss.operandString())
			case ss.inst.Length == 2:
				a.code = append(a.code, byte(ss.operand.getValue()))
				a.log("%04X-   %-8s    %s   %s", ss.addr, ss.codeString(), ss.opcode.str, ss.operandString())
			case ss.inst.Length == 3:
				a.code = append(a.code, toBytes(2, ss.operand.getValue())...)
				a.log("%04X-   %-8s    %s   %s", ss.addr, ss.codeString(), ss.opcode.str, ss.operandString())
			default:
				panic("invalid operand")
			}

		case *data:
			start := len(a.code)
			for _, e := range ss.exprs {
				switch {
				case e.isString:
					s := []byte(e.stringLiteral.str)
					if ss.hiBitTerm && len(s) > 0 {
						s[len(s)-1] = s[len(s)-1] | 0x80
					}
					a.code = append(a.code, s...)
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

		case *padding:
			pad := make([]byte, ss.pad)
			for i := 0; i < ss.pad; i++ {
				pad[i] = ss.value
			}
			a.code = append(a.code, pad...)
			a.logBytes(ss.addr, pad)

		case *export:
			if ss.expr.op != opIdentifier || !ss.expr.address {
				a.addError(ss.expr.line, "export is not an address label")
			}
			export := Export{
				Label:   ss.expr.identifier.str,
				Address: uint16(ss.expr.value),
			}
			a.exports = append(a.exports, export)
		}
	}
	return nil
}

// Parse a single line of assembly code.
func (a *assembler) parseLine(line fstring) error {
	// Skip empty (or comment-only) lines
	if line.isEmpty() || line.startsWithChar('*') {
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
	word, line := line.consumeWhile(wordChar)
	if op, ok := pseudoOps[strings.ToLower(word.str)]; ok {
		return op.fn(a, line.consumeWhitespace(), fstring{}, op.param)
	}

	return a.parseInstruction(word, line)
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
	word, line := line.consumeWhile(wordChar)
	if op, ok := pseudoOps[strings.ToLower(word.str)]; ok {
		return op.fn(a, line.consumeWhitespace(), label, op.param)
	}

	// Store the label.
	err = a.storeLabel(label)
	if err != nil {
		return err
	}

	// Parse any instruction following the label
	if !word.isEmpty() {
		return a.parseInstruction(word, line)
	}
	return nil
}

// Store a label into the assembler's label list.
func (a *assembler) storeLabel(label fstring) error {
	// If the label starts with '.' or '@', it is a local label. So append it
	// to the active scope label.
	if label.startsWithChar('.') || label.startsWithChar('@') {
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

// Parse an architecture pseudo-op.
func (a *assembler) parseArch(line, label fstring, param any) error {
	archl, _ := line.consumeWhile(labelChar)
	arch := strings.ToLower(archl.str)

	switch {
	case arch == "6502" || arch == "nmos":
		a.arch = cpu.NMOS
	case arch == "65c02" || arch == "cmos":
		a.arch = cpu.CMOS
	default:
		a.addError(line, "invalid architecture '%s'", archl.str)
		return errParse
	}

	a.instSet = cpu.GetInstructionSet(a.arch)
	return nil
}

// Parse an ".EQU" constant definition.
func (a *assembler) parseEquate(line, label fstring, param any) error {
	if label.str == "" {
		a.addError(line, "equate declaration must begin with a label")
		return errParse
	}

	a.logLine(line, "equate=%s", label.str)

	// Parse the constant expression.
	e, _, err := a.exprParser.parse(line, a.scopeLabel, allowParentheses)
	if err != nil {
		a.addExprErrors()
		return err
	}

	// Attempt to evaluate the expression immediately. If not possible, add it
	// to a list of unevaluated expressions.
	if !e.eval(-1, a.constants, a.labels) {
		a.pushUnevaluated(e)
	}

	a.logLine(line, "expr=%s", e.String())
	switch e.evaluated {
	case true:
		a.logLine(line, "val=$%X", e.value)
	case false:
		a.logLine(line, "val=(uneval)")
	}

	// Track the constants for later substitution.
	a.constants[label.str] = e
	return nil
}

// Parse an ".ORG" origin definition
func (a *assembler) parseOrigin(line, label fstring, param any) error {
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

	if !e.eval(-1, a.constants, a.labels) {
		a.addError(e.identifier, "unable to evaluate expression")
		return errParse
	}

	a.logLine(line, "expr=%s", e.String())
	a.logLine(line, "val=$%04X", e.value)

	a.origin = e.value
	return nil
}

// Parse a data pseudo-op.
func (a *assembler) parseData(line, label fstring, param any) error {
	a.logLine(line, "bytes=")

	seg := &data{
		unit:      param.(int) & 7,
		hiBitTerm: (param.(int) & hiBitTerm) != 0,
		addr:      -1,
	}

	remain := line
	for !remain.isEmpty() {
		var expr fstring
		expr, remain = remain.consumeUntilUnquotedChar(',')

		if !remain.isEmpty() {
			remain = remain.consume(1).consumeWhitespace()
		}

		e, _, err := a.exprParser.parse(expr, a.scopeLabel, allowParentheses|allowStrings)
		if err != nil {
			a.addExprErrors()
			return err
		}

		if !e.eval(-1, a.constants, a.labels) {
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
func (a *assembler) parseHexString(line, label fstring, param any) error {
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

	if !label.isEmpty() {
		err := a.storeLabel(label)
		if err != nil {
			return err
		}
	}

	a.segments = append(a.segments, seg)
	return nil
}

// Parse an align pseudo-op
func (a *assembler) parseAlign(line, label fstring, param any) error {
	a.logLine(line, "align=")

	s, remain := line.consumeWhile(decimal)
	if s.isEmpty() || !remain.isEmpty() {
		a.addError(remain, "invalid alignment")
		return errParse
	}

	v, _ := strconv.ParseInt(s.str, 10, 32)
	if v == 0 || (v&(v-1)) != 0 || v > 0x100 {
		a.addError(s, "alignment must be a power of 2")
		return errParse
	}

	seg := &alignment{addr: -1, align: int(v)}

	a.segments = append(a.segments, seg)
	return nil
}

// Parse a padding pseudo-op
func (a *assembler) parsePadding(line, label fstring, param any) error {
	a.logLine(line, "pad=")

	s, remain := line.consumeUntilChar(',')
	if remain.isEmpty() {
		a.addError(s, "invalid padding")
		return errParse
	}

	valExpr, _, err := a.exprParser.parse(s, a.scopeLabel, allowParentheses)
	if err != nil {
		a.addExprErrors()
		return err
	}

	// Attempt to evaluate the pad value expression immediately.
	if !valExpr.eval(-1, a.constants, a.labels) {
		a.pushUnevaluated(valExpr)
	}

	a.logLine(line, "padexpr=%s", valExpr.String())
	switch valExpr.evaluated {
	case true:
		a.logLine(line, "val=$%X", valExpr.value)
	case false:
		a.logLine(line, "val=(uneval)")
	}

	s = remain.consume(1).consumeWhitespace()
	lenExpr, _, err := a.exprParser.parse(s, a.scopeLabel, allowParentheses)
	if err != nil {
		a.addExprErrors()
		return err
	}

	// Attempt to evaluate the length expression immediately.
	if !lenExpr.eval(-1, a.constants, a.labels) {
		a.pushUnevaluated(lenExpr)
	}

	a.logLine(line, "lenexpr=%s", lenExpr.String())
	switch lenExpr.evaluated {
	case true:
		a.logLine(line, "len=$%X", lenExpr.value)
	case false:
		a.logLine(line, "len=(uneval)")
	}

	seg := &padding{addr: -1, valExpr: valExpr, lenExpr: lenExpr}
	a.segments = append(a.segments, seg)
	return nil
}

// Parse an export pseudo-op
func (a *assembler) parseExport(line, label fstring, param any) error {
	a.logLine(line, "export=")

	// Parse the export expression.
	e, _, err := a.exprParser.parse(line, a.scopeLabel, allowParentheses)
	if err != nil {
		a.addExprErrors()
		return err
	}

	// Attempt to evaluate the expression immediately.
	if !e.eval(-1, a.constants, a.labels) {
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

// Parse an include pseudo-op
func (a *assembler) parseInclude(line, label fstring, param any) error {
	a.logLine(line, "include")

	filename, _ := line.consumeUntil(whitespace)
	if filename.isEmpty() {
		a.addError(filename, "invalid filename")
		return errParse
	}

	file, err := os.Open(filename.str)
	if err != nil {
		a.addError(filename, "unable to open '%s'", filename.str)
		return err
	}
	defer file.Close()

	fileIndex := len(a.files)
	a.files = append(a.files, filename.str)

	return a.parseFile(bufio.NewScanner(file), fileIndex)
}

// Parse a binary include pseudo-op
func (a *assembler) parseBinaryInclude(line, label fstring, param any) error {
	a.logLine(line, "binary_include")

	filename, _ := line.consumeUntil(whitespace)
	if filename.isEmpty() {
		a.addError(filename, "invalid filename")
		return errParse
	}

	file, err := os.Open(filename.str)
	if err != nil {
		a.addError(filename, "unable to open '%s'", filename.str)
		return err
	}
	defer file.Close()

	seg := &bytedata{addr: -1}

	data, err := io.ReadAll(file)
	if err != nil {
		a.addError(filename, "unable to read '%s'", filename.str)
		return err
	}

	seg.b = data

	if !label.isEmpty() {
		err := a.storeLabel(label)
		if err != nil {
			return err
		}
	}

	a.segments = append(a.segments, seg)
	return nil
}

// Parse a 6502 assembly opcode + operand.
func (a *assembler) parseInstruction(opcode, remain fstring) error {
	// No opcode characters? Or opcode has invalid suffix?
	if opcode.isEmpty() || (!remain.isEmpty() && !remain.startsWith(whitespace)) {
		a.addError(remain, "invalid opcode '%s'", remain.str)
		return errParse
	}

	// Validate the opcode
	instructions := a.instSet.GetInstructions(opcode.str)
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
	seg := &instruction{
		addr:      -1,
		fileIndex: remain.fileIndex,
		line:      remain.row,
		opcode:    opcode,
		operand:   operand,
	}
	a.segments = append(a.segments, seg)
	return nil
}

// Parse the operand expression following an opcode.
func (a *assembler) parseOperand(line fstring) (o operand, remain fstring, err error) {
	switch {
	case line.isEmpty():
		o.modeGuess, remain = cpu.IMP, line
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
		o.modeGuess = cpu.IMM
		o.forceImmediate = true
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

	if !o.expr.eval(-1, a.constants, a.labels) {
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
func (a *assembler) addError(l fstring, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	a.errors = append(a.errors, asmerror{l, msg})
	if a.verbose {
		filename := a.files[l.fileIndex]
		fmt.Fprintf(a.out, "Syntax error in '%s' line %d, col %d: %s\n", filename, l.row, l.column+1, msg)
		fmt.Fprintln(a.out, l.full)
		for i := 0; i < l.column; i++ {
			fmt.Fprintf(a.out, "-")
		}
		fmt.Fprintln(a.out, "^")
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
func (a *assembler) log(format string, args ...any) {
	if a.verbose {
		fmt.Fprintf(a.out, format, args...)
		fmt.Fprintf(a.out, "\n")
	}
}

// In verbose mode, log a string and its associated line
// of assembly code.
func (a *assembler) logLine(line fstring, format string, args ...any) {
	if a.verbose {
		detail := fmt.Sprintf(format, args...)
		fmt.Fprintf(a.out, "%-3d %-3d | %-20s | %s\n", line.row, line.column+1, detail, line.str)
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
			a.log("%04X-*  %s", addr+i, byteString(b[i:j]))
		}
	}
}

// In verbose mode, log a section header to the standard output.
func (a *assembler) logSection(name string) {
	if a.verbose {
		fmt.Fprintln(a.out, strings.Repeat("-", len(name)+6))
		fmt.Fprintf(a.out, "-- %s --\n", name)
		fmt.Fprintln(a.out, strings.Repeat("-", len(name)+6))
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
func (a *assembler) findMatchingInstruction(opcode fstring, operand operand) *cpu.Instruction {
	bestqual := 3
	var found *cpu.Instruction
	for _, inst := range a.instSet.GetInstructions(opcode.str) {
		match, qual := false, 0
		switch {
		case inst.Mode == cpu.IMP || inst.Mode == cpu.ACC:
			match, qual = (operand.modeGuess == cpu.IMP) && (operand.size() == 0), 0
		case operand.size() == 0:
			match = false
		case inst.Mode == cpu.IMM:
			match, qual = (operand.modeGuess == cpu.IMM) && (operand.size() == 1), 1
		case inst.Mode == cpu.REL:
			match, qual = (operand.modeGuess == cpu.ABS), 1
		case inst.Mode == cpu.ZPG:
			match, qual = (operand.modeGuess == cpu.ABS) && (operand.size() == 1), 1
		case inst.Mode == cpu.ZPX:
			match, qual = (operand.modeGuess == cpu.ABX) && (operand.size() == 1), 1
		case inst.Mode == cpu.ZPY:
			match, qual = (operand.modeGuess == cpu.ABY) && (operand.size() == 1), 1
		case inst.Mode == cpu.ABS:
			match, qual = (operand.modeGuess == cpu.ABS), 2
		case inst.Mode == cpu.ABX:
			match, qual = (operand.modeGuess == cpu.ABX), 2
		case inst.Mode == cpu.ABY:
			match, qual = (operand.modeGuess == cpu.ABY), 2
		case inst.Mode == cpu.IND && inst.Length == 3:
			match, qual = (operand.modeGuess == cpu.IND), 2
		case inst.Mode == cpu.IND && inst.Length == 2:
			match, qual = (operand.modeGuess == cpu.IND) && (operand.size() == 1), 1
		case inst.Mode == cpu.IDX:
			match, qual = (operand.modeGuess == cpu.IDX) && (operand.size() == 1), 1
		case inst.Mode == cpu.IDY:
			match, qual = (operand.modeGuess == cpu.IDY) && (operand.size() == 1), 1
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
func (l fstring) consumeIndirect() (mode cpu.Mode, expr fstring, remain fstring, err error) {
	expr, remain = l.consumeUntil(func(c byte) bool { return c == ',' || c == ')' })

	switch {
	case remain.startsWithString(",X)") || remain.startsWithString(",x)"):
		mode, remain = cpu.IDX, remain.consume(3)
	case remain.startsWithString("),Y") || remain.startsWithString("),y"):
		mode, remain = cpu.IDY, remain.consume(3)
	case remain.startsWithChar(')'):
		mode, remain = cpu.IND, remain.consume(1)
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
func (l fstring) consumeAbsolute() (mode cpu.Mode, expr fstring, remain fstring, err error) {
	expr, remain = l.consumeUntilChar(',')

	switch {
	case remain.startsWithString(",X") || remain.startsWithString(",x"):
		mode, remain = cpu.ABX, remain.consume(2)
	case remain.startsWithString(",Y") || remain.startsWithString(",y"):
		mode, remain = cpu.ABY, remain.consume(2)
	default:
		mode = cpu.ABS
	}

	remain = remain.consumeWhitespace()
	if !remain.isEmpty() {
		err = errParse
	}

	return mode, expr, remain, err
}
