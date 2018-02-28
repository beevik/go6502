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
	opUnaryMinus exprOp = iota
	opUnaryPlus
	opBitwiseNEG

	// binary operations
	opMultiply
	opDivide
	opModulo
	opAdd
	opSubstract
	opShiftLeft
	opShiftRight
	opBitwiseAND
	opBitwiseXOR
	opBitwiseOR

	// value "operations"
	opNumber
	opIdentifier

	// pseudo-operations (used only during parsing but not stored in expr's)
	opLeftParen
	opRightParen
)

type opdata struct {
	precedence      byte
	binary          bool
	leftAssociative bool
	symbol          string
	eval            func(a, b int) int
}

var ops = []opdata{
	// unary and binary operations
	{7, false, false, "-", func(a, b int) int { return -a }},             // uminus
	{7, false, false, "+", func(a, b int) int { return -a }},             // uplus
	{7, false, false, "~", func(a, b int) int { return 0xffffffff ^ a }}, // bitneg
	{6, true, true, "*", func(a, b int) int { return a * b }},            // multiply
	{6, true, true, "/", func(a, b int) int { return a / b }},            // divide
	{6, true, true, "%", func(a, b int) int { return a % b }},            // modulo
	{5, true, true, "+", func(a, b int) int { return a + b }},            // add
	{5, true, true, "-", func(a, b int) int { return a - b }},            // subtract
	{4, true, true, "<<", func(a, b int) int { return a << uint32(b) }},  // shift_left
	{4, true, true, ">>", func(a, b int) int { return a >> uint32(b) }},  // shift_right
	{3, true, true, "&", func(a, b int) int { return a & b }},            // and
	{2, true, true, "^", func(a, b int) int { return a ^ b }},            // xor
	{1, true, true, "|", func(a, b int) int { return a | b }},            // or

	// value operations
	{0, false, false, "", nil}, // number
	{0, false, false, "", nil}, // identifier

	// pseudo-operations
	{0, false, false, "", nil}, // lparen
	{0, false, false, "", nil}, // rparen
}

func (op exprOp) isBinary() bool {
	return ops[op].binary
}

func (op exprOp) eval(a, b int) int {
	return ops[op].eval(a, b)
}

func (op exprOp) symbol() string {
	return ops[op].symbol
}

func (op exprOp) isCollapsible() bool {
	return ops[op].precedence > 0
}

// Compare the precendence and associativity of 'op' to 'other'.
// Return true if the shunting yard algorithm should cause an
// expression node collapse.
func (op exprOp) collapses(other exprOp) bool {
	if ops[op].leftAssociative {
		return ops[op].precedence <= ops[other].precedence
	}
	return ops[op].precedence < ops[other].precedence
}

//
// expr
//

// An expr represents a single node in a binary expression tree.
// The root node represents an entire expression.
type expr struct {
	number     int
	identifier fstring
	scopeLabel fstring
	op         exprOp
	evaluated  bool
	address    bool
	child0     *expr
	child1     *expr
}

// Return the expression as a postfix notation string.
func (e *expr) String() string {
	switch {
	case e.op == opNumber:
		return fmt.Sprintf("%d", e.number)
	case e.op == opIdentifier:
		if e.address && e.identifier.startsWithChar('.') {
			return e.scopeLabel.str + e.identifier.str
		}
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
		case e.op == opNumber:
			e.evaluated = true
		case e.op == opIdentifier:
			var ident string
			if e.identifier.startsWithChar('.') {
				ident = e.scopeLabel.str + e.identifier.str
			} else {
				ident = e.identifier.str
			}
			if m, ok := macros[ident]; ok && m.evaluated {
				e.number = m.number
				e.evaluated = true
			}
			if _, ok := labels[ident]; ok {
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
	tokenNil tokentype = iota
	tokenOp
	tokenNumber
	tokenIdentifier
	tokenLeftParen
	tokenRightParen
)

func (tt tokentype) isValue() bool {
	return tt == tokenNumber || tt == tokenIdentifier
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
func (p *exprParser) parse(line, scopeLabel fstring, allowParens bool) (e *expr, out fstring, err error) {
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
		if token.tt == tokenNil {
			break
		}

		// Handle each possible token type
		switch token.tt {

		case tokenNumber:
			p.operandStack.push(&expr{op: opNumber, number: token.number, evaluated: true})

		case tokenIdentifier:
			p.operandStack.push(&expr{op: opIdentifier, identifier: token.identifier, scopeLabel: scopeLabel})

		case tokenOp:
			for err == nil && !p.operatorStack.empty() && token.op.collapses(p.operatorStack.peek()) {
				err = p.operandStack.collapse(p.operatorStack.pop())
				if err != nil {
					p.addError(line, "Expression syntax error 1")
				}
			}
			p.operatorStack.push(token.op)

		case tokenLeftParen:
			p.operatorStack.push(opLeftParen)

		case tokenRightParen:
			for err == nil {
				if p.operatorStack.empty() {
					p.addError(line, "Mismatched parentheses")
					err = errParse
					break
				}
				op := p.operatorStack.pop()
				if op == opLeftParen {
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
			err = errParse
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
		t.tt, out = tokenNil, line
		return
	}
	switch {

	case line.startsWith(decimal) || line.startsWithChar('$'):
		t.number, _, out, err = p.parseNumber(line)
		t.tt = tokenNumber
		if p.prevToken.tt.isValue() || p.prevToken.tt == tokenRightParen {
			p.addError(line, "Expression syntax error 4")
			err = errParse
		}

	case p.allowParens && line.startsWithChar('('):
		p.parenCounter++
		t.tt, t.op = tokenLeftParen, opLeftParen
		out = line.consume(1)

	case p.allowParens && line.startsWithChar(')'):
		if p.parenCounter == 0 {
			p.addError(line, "Mismatched parentheses")
			err = errParse
			out = line.consume(1)
		} else {
			p.parenCounter--
			t.tt, t.op, out = tokenRightParen, opRightParen, line.consume(1)
		}

	case line.startsWith(identifierStartChar):
		t.tt = tokenIdentifier
		t.identifier, out = line.consumeWhile(identifierChar)
		if p.prevToken.tt.isValue() || p.prevToken.tt == tokenRightParen {
			p.addError(line, "Expression syntax error 5")
			err = errParse
		}

	default:
		for i, o := range ops {
			if o.symbol != "" && line.startsWithString(o.symbol) {
				if o.binary || (!o.binary && !p.prevToken.tt.isValue() && p.prevToken.tt != tokenRightParen) {
					t.tt, t.op, out = tokenOp, exprOp(i), line.consume(len(o.symbol))
					break
				}
			}
		}
		if t.tt != tokenOp {
			p.addError(line, "Expression syntax error 6")
			err = errParse
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
//
// The function returns the parsed value, the number of bytes used to
// hold the value, the remainder of the line, and any parsing error
// encountered.  The number of bytes used to hold the value will be 1, 2
// or 4.
//
// If a hexadecimal or binary value is parsed, the length of the parsed
// string is used to determine how many bytes are required to hold the
// value.  For example, if the parsed string is "0x0020", the number of bytes
// required to hold the value is 2, while if the parse string is "0x20", the
// number of bytes required is 1.
//
// If a decimal number if parsed, the length of the parsed string is ignored,
// and the minimum number of bytes required to hold the value is returned.
func (p *exprParser) parseNumber(line fstring) (value, bytes int, remain fstring, err error) {
	// Select decimal, hexadecimal or binary depending on the prefix
	base, fn, bitsPerChar := 10, decimal, 0
	if line.startsWithChar('$') {
		line = line.consume(1)
		base, fn, bitsPerChar = 16, hexadecimal, 4
	} else if line.startsWithString("0x") {
		line = line.consume(2)
		base, fn, bitsPerChar = 16, hexadecimal, 4
	} else if line.startsWithString("0b") {
		line = line.consume(2)
		base, fn, bitsPerChar = 2, binary, 1
	}

	// Consume the number and update the remaining line
	numstr, remain := line.consumeWhile(fn)

	// Convert the string to an integer
	num64, converr := strconv.ParseInt(numstr.str, base, 32)
	if converr != nil {
		p.addError(numstr, "Failed to parse integer")
		err = errParse
	}

	value = int(num64)

	l := len(numstr.str)
	switch bitsPerChar {
	case 0:
		switch {
		case value < 0x100:
			bytes = 1
		case value < 0x10000:
			bytes = 2
		default:
			bytes = 4
		}
	default:
		bytes = (l*bitsPerChar + 7) / 8
		if bytes > 2 {
			bytes = 4
		}
	}

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
	}
	return s.data[len(s.data)-1]
}

// Collapse one or more expression nodes on the top of the
// stack into a combined expression node, and push the combined
// node back onto the stack.
func (s *exprStack) collapse(op exprOp) error {
	switch {
	case !op.isCollapsible():
		return errParse
	case op.isBinary():
		if len(s.data) < 2 {
			return errParse
		}
		s.push(&expr{op: op, child1: s.pop(), child0: s.pop()})
	default:
		if s.empty() {
			return errParse
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
