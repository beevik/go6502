package asm

import (
	"fmt"
	"strconv"
)

//
// exprOp
//

type exprOp byte

const (
	// operators in descending order of precedence

	// unary operations
	op_uminus exprOp = iota
	op_uplus
	op_bitneg

	// binary operations
	op_multiply
	op_divide
	op_modulo
	op_add
	op_subtract
	op_shift_left
	op_shift_right
	op_and
	op_xor
	op_or

	// value "operations"
	op_number
	op_identifier

	// pseudo-operations (used only during parsing but not stored in expr's)
	op_lparen
	op_rparen
)

type opdata_t struct {
	precedence      byte
	binary          bool
	leftAssociative bool
	symbol          string
	eval            func(a, b int) int
}

var opdata = []opdata_t{
	// unary and binary operations
	{7, false, false, "-", func(a, b int) int { return -a }},             // op_uminus
	{7, false, false, "+", func(a, b int) int { return -a }},             // op_uplus
	{7, false, false, "~", func(a, b int) int { return 0xffffffff ^ a }}, // op_bitneg
	{6, true, true, "*", func(a, b int) int { return a * b }},            // op_multiply
	{6, true, true, "/", func(a, b int) int { return a / b }},            // op_divide
	{6, true, true, "%", func(a, b int) int { return a % b }},            // op_modulo
	{5, true, true, "+", func(a, b int) int { return a + b }},            // op_add
	{5, true, true, "-", func(a, b int) int { return a - b }},            // op_subtract
	{4, true, true, "<<", func(a, b int) int { return a << uint32(b) }},  // op_shift_left
	{4, true, true, ">>", func(a, b int) int { return a >> uint32(b) }},  // op_shift_right
	{3, true, true, "&", func(a, b int) int { return a & b }},            // op_and
	{2, true, true, "^", func(a, b int) int { return a ^ b }},            // op_xor
	{1, true, true, "|", func(a, b int) int { return a | b }},            // op_or

	// value operations
	{0, false, false, "", nil}, // op_number
	{0, false, false, "", nil}, // op_identifier

	// pseudo-operations
	{0, false, false, "", nil}, // op_lparen
	{0, false, false, "", nil}, // op_rparen
}

func (op exprOp) isBinary() bool {
	return opdata[op].binary
}

func (op exprOp) eval(a, b int) int {
	return opdata[op].eval(a, b)
}

func (op exprOp) symbol() string {
	return opdata[op].symbol
}

func (op exprOp) isCollapsible() bool {
	return opdata[op].precedence > 0
}

// Compare the precendence and associativity of 'op' to 'other'.
// Return true if the shunting yard algorithm should cause an
// expression node collapse.
func (op exprOp) collapses(other exprOp) bool {
	if opdata[op].leftAssociative {
		return opdata[op].precedence <= opdata[other].precedence
	} else {
		return opdata[op].precedence < opdata[other].precedence
	}
}

//
// expr
//

// An expr represents a single node in a binary expression tree.
// The root node represents an entire expression.
type expr struct {
	number     int
	identifier fstring
	op         exprOp
	evaluated  bool
	address    bool
	child0     *expr
	child1     *expr
}

// Return the expression as a postfix notation string.
func (e *expr) String() string {
	switch {
	case e.op == op_number:
		return fmt.Sprintf("%d", e.number)
	case e.op == op_identifier:
		return e.identifier.str
	case e.op.isBinary():
		return fmt.Sprintf("%s %s %s", e.child0.String(), e.child1.String(), e.op.symbol())
	case !e.op.isBinary():
		return fmt.Sprintf("%s [%s]", e.child0.String(), e.op.symbol())
	default:
		return ""
	}
}

// Evaluate the expression tree.
func (e *expr) eval(macros map[string]*expr, labels map[string]int) bool {
	if !e.evaluated {
		switch {
		case e.op == op_number:
			e.evaluated = true
		case e.op == op_identifier:
			if m, ok := macros[e.identifier.str]; ok && m.evaluated {
				e.number = m.number
				e.evaluated = true
			}
			if _, ok := labels[e.identifier.str]; ok {
				e.address = true
			}
		case e.op.isBinary():
			e.child0.eval(macros, labels)
			e.child1.eval(macros, labels)
			if e.child0.evaluated && e.child1.evaluated {
				e.number = e.op.eval(e.child0.number, e.child1.number)
				e.evaluated = true
			}
			if e.child0.address || e.child1.address {
				e.address = true
			}
		default:
			e.child0.eval(macros, labels)
			if e.child0.evaluated {
				e.number = e.op.eval(e.child0.number, 0)
				e.evaluated = true
			}
			if e.child0.address {
				e.address = true
			}
		}
	}
	return e.evaluated
}

//
// token
//

type tokentype byte

const (
	tt_nil tokentype = iota
	tt_op
	tt_number
	tt_identifier
	tt_lparen
	tt_rparen
)

func (tt tokentype) isValue() bool {
	return tt == tt_number || tt == tt_identifier
}

type token struct {
	tt         tokentype
	number     int
	identifier fstring
	op         exprOp
}

//
// exprParser
//

type exprParser struct {
	operandStack  exprStack
	operatorStack opStack
	parenCounter  int
	allowParens   bool
	prevToken     token
	errors        []asmerror
}

// Parse an expression from the line until it is exhausted.
func (p *exprParser) parse(line fstring, allowParens bool) (e *expr, out fstring, err error) {
	p.errors = nil
	p.allowParens = allowParens
	p.prevToken = token{}

	// Process expression using Dijkstra's shunting-yard algorithm
	for err == nil {

		// Parse the next expression token
		var token token
		token, out, err = p.parseToken(line)
		if err != nil {
			break
		}

		// We're done when the token parser returns the nil token
		if token.tt == tt_nil {
			break
		}

		// Handle each possible token type
		switch token.tt {

		case tt_number:
			p.operandStack.push(&expr{op: op_number, number: token.number, evaluated: true})

		case tt_identifier:
			p.operandStack.push(&expr{op: op_identifier, identifier: token.identifier})

		case tt_op:
			for err == nil && !p.operatorStack.empty() && token.op.collapses(p.operatorStack.peek()) {
				err = p.operandStack.collapse(p.operatorStack.pop())
				if err != nil {
					p.addError(line, "Expression syntax error 1")
				}
			}
			p.operatorStack.push(token.op)

		case tt_lparen:
			p.operatorStack.push(op_lparen)

		case tt_rparen:
			for err == nil {
				if p.operatorStack.empty() {
					p.addError(line, "Mismatched parentheses")
					err = parseError
					break
				}
				op := p.operatorStack.pop()
				if op == op_lparen {
					break
				}
				err = p.operandStack.collapse(op)
				if err != nil {
					p.addError(line, "Expression syntax error 2")
				}
			}

		}
		line = out
	}

	// Collapse any operators (and operands) remaining on the stack
	for err == nil && !p.operatorStack.empty() {
		err = p.operandStack.collapse(p.operatorStack.pop())
		if err != nil {
			p.addError(line, "Expression syntax error 3")
			err = parseError
		}
	}

	if err == nil {
		e = p.operandStack.peek()
	}
	p.reset()
	return
}

// Attempt to parse the next token from the line.
func (p *exprParser) parseToken(line fstring) (t token, out fstring, err error) {
	if line.isEmpty() {
		t.tt, out = tt_nil, line
		return
	}
	switch {

	case line.startsWith(decimal) || line.startsWithChar('$'):
		t.number, out, err = p.parseNumber(line)
		t.tt = tt_number
		if p.prevToken.tt.isValue() || p.prevToken.tt == tt_rparen {
			p.addError(line, "Expression syntax error 4")
			err = parseError
		}

	case p.allowParens && line.startsWithChar('('):
		p.parenCounter++
		t.tt, t.op = tt_lparen, op_lparen
		out = line.consume(1)

	case p.allowParens && line.startsWithChar(')'):
		if p.parenCounter == 0 {
			p.addError(line, "Mismatched parentheses")
			err = parseError
			out = line.consume(1)
		} else {
			p.parenCounter--
			t.tt, t.op, out = tt_rparen, op_rparen, line.consume(1)
		}

	case line.startsWith(identifierStartChar):
		t.tt = tt_identifier
		t.identifier, out = line.consumeWhile(identifierChar)
		if p.prevToken.tt.isValue() || p.prevToken.tt == tt_rparen {
			p.addError(line, "Expression syntax error 5")
			err = parseError
		}

	default:
		for i, o := range opdata {
			if o.symbol != "" && line.startsWithString(o.symbol) {
				if o.binary || (!o.binary && !p.prevToken.tt.isValue() && p.prevToken.tt != tt_rparen) {
					t.tt, t.op, out = tt_op, exprOp(i), line.consume(len(o.symbol))
					break
				}
			}
		}
		if t.tt != tt_op {
			p.addError(line, "Expression syntax error 6")
			err = parseError
		}
	}

	p.prevToken = t
	out = out.consumeWhitespace()
	return
}

// Parse a number from the line. The following numeric formats are allowed:
//   [0-9]+   			Decimal number
//   $[0-9a-fA-F]+		Hexadecimal number
//	 0x[0-9a-fA-F]+ 	Hexadecimal number
//	 0b[01]+ 			Binary number
func (p *exprParser) parseNumber(line fstring) (number int, out fstring, err error) {

	// Select decimal, hexadecimal or binary depending on the prefix
	base, fn := 10, decimal
	if line.startsWithChar('$') {
		line = line.consume(1)
		base, fn = 16, hexadecimal
	} else if line.startsWithString("0x") {
		line = line.consume(2)
		base, fn = 16, hexadecimal
	} else if line.startsWithString("0b") {
		line = line.consume(2)
		base, fn = 2, binary
	}

	// Consume the number and update the remaining line
	numstr, out := line.consumeWhile(fn)

	// Convert the string to an integer
	num64, converr := strconv.ParseInt(numstr.str, base, 32)
	if converr != nil {
		p.addError(numstr, "Failed to parse integer")
		err = parseError
	}

	number = int(num64)
	return
}

func (p *exprParser) addError(line fstring, msg string) {
	p.errors = append(p.errors, asmerror{line, msg})
}

func (p *exprParser) reset() {
	p.operandStack.data, p.operatorStack.data = nil, nil
	p.parenCounter = 0
}

//
// exprStack
//

type exprStack struct {
	data []*expr
}

func (s *exprStack) empty() bool {
	return len(s.data) == 0
}

func (s *exprStack) push(e *expr) {
	s.data = append(s.data, e)
}

func (s *exprStack) pop() *expr {
	l := len(s.data)
	e := s.data[l-1]
	s.data = s.data[:l-1]
	return e
}

func (s *exprStack) peek() *expr {
	if len(s.data) == 0 {
		return nil
	} else {
		return s.data[len(s.data)-1]
	}
}

// Collapse one or more expression nodes on the top of the
// stack into a combined expression node, and push the combined
// node back onto the stack.
func (s *exprStack) collapse(op exprOp) error {
	switch {
	case !op.isCollapsible():
		return parseError
	case op.isBinary():
		if len(s.data) < 2 {
			return parseError
		}
		s.push(&expr{op: op, child1: s.pop(), child0: s.pop()})
	default:
		if s.empty() {
			return parseError
		}
		s.push(&expr{op: op, child0: s.pop()})
	}
	return nil
}

//
// opStack
//

type opStack struct {
	data []exprOp
}

func (s *opStack) push(op exprOp) {
	s.data = append(s.data, op)
}

func (s *opStack) pop() exprOp {
	op := s.data[len(s.data)-1]
	s.data = s.data[0 : len(s.data)-1]
	return op
}

func (s *opStack) empty() bool {
	return len(s.data) == 0
}

func (s *opStack) peek() exprOp {
	return s.data[len(s.data)-1]
}
