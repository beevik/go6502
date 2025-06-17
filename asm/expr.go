// Copyright 2014-2018 Brett Vickers. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

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

	// unary operations (0..5)
	// opUnaryMinus
	// opUnaryPlus
	// opUnaryLessThan
	// opUnaryGreaterThan
	// opUnarySlash
	// opBitwiseNEG

	// binary operations (6..15)
	// opMultiply
	// opDivide
	// opModulo
	// opAdd
	// opSubstract
	// opShiftLeft
	// opShiftRight
	// opBitwiseAND
	// opBitwiseXOR
	// opBitwiseOR

	// value "operations" (16..19)
	opNumber exprOp = iota + 16
	opString
	opIdentifier
	opHere

	// pseudo-ops (20..21) (used only during parsing but not stored in expr's)
	opLeftParen
	opRightParen
)

type opdata struct {
	precedence      byte
	children        int
	leftAssociative bool
	symbol          string
	eval            func(a, b int) int
}

func (o *opdata) isBinary() bool {
	return o.children == 2
}

func (o *opdata) isUnary() bool {
	return o.children == 1
}

// One entry per exprOp value (order must match)
var ops = []opdata{
	// unary operations
	{7, 1, false, "-", func(a, b int) int { return -a }},              // uminus
	{7, 1, false, "+", func(a, b int) int { return a }},               // uplus
	{7, 1, false, "<", func(a, b int) int { return a & 0xff }},        // ulessthan
	{7, 1, false, ">", func(a, b int) int { return (a >> 8) & 0xff }}, // ugreaterthan
	{7, 1, false, "/", func(a, b int) int { return (a >> 8) & 0xff }}, // uslash
	{7, 1, false, "~", func(a, b int) int { return ^a }},              // bitneg

	// binary operations
	{6, 2, true, "*", func(a, b int) int { return a * b }},           // multiply
	{6, 2, true, "/", func(a, b int) int { return a / b }},           // divide
	{6, 2, true, "%", func(a, b int) int { return a % b }},           // modulo
	{5, 2, true, "+", func(a, b int) int { return a + b }},           // add
	{5, 2, true, "-", func(a, b int) int { return a - b }},           // subtract
	{4, 2, true, "<<", func(a, b int) int { return a << uint32(b) }}, // shift_left
	{4, 2, true, ">>", func(a, b int) int { return a >> uint32(b) }}, // shift_right
	{3, 2, true, "&", func(a, b int) int { return a & b }},           // and
	{2, 2, true, "^", func(a, b int) int { return a ^ b }},           // xor
	{1, 2, true, "|", func(a, b int) int { return a | b }},           // or

	// value "operations"
	{0, 0, false, "", nil}, // numeric literal
	{0, 0, false, "", nil}, // string literal
	{0, 0, false, "", nil}, // identifier
	{0, 0, false, "", nil}, // here

	// pseudo-operations
	{0, 0, false, "", nil}, // lparen
	{0, 0, false, "", nil}, // rparen
}

func (op exprOp) isBinary() bool {
	return ops[op].isBinary()
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

type parseFlags uint32

const (
	allowParentheses parseFlags = 1 << iota
	allowStrings
)

// An expr represents a single node in a binary expression tree.
// The root node represents an entire expression.
type expr struct {
	line          fstring // start of expression line
	op            exprOp  // type of expression
	value         int     // resolved value
	bytes         int     // number of bytes to hold the value
	address       bool    // true if value is an address
	evaluated     bool    // true if value has been evaluated
	isString      bool    // true if expr is a string literal (not a value)
	stringLiteral fstring // if op == opString
	identifier    fstring // if op == opIdentifier
	scopeLabel    fstring // active scope label when parsing began
	child0        *expr   // first child in expression tree
	child1        *expr   // second child in expression tree (parent must be binary op)
}

// Return the expression as a postfix notation string.
func (e *expr) String() string {
	switch {
	case e.op == opNumber:
		return fmt.Sprintf("%d", e.value)
	case e.op == opString:
		return e.stringLiteral.str
	case e.op == opIdentifier:
		if e.address && (e.identifier.startsWithChar('.') || e.identifier.startsWithChar('@')) {
			return "~" + e.scopeLabel.str + e.identifier.str
		}
		return e.identifier.str
	case e.op == opHere:
		return "$"
	case e.op.isBinary():
		return fmt.Sprintf("%s %s %s", e.child0.String(), e.child1.String(), e.op.symbol())
	case !e.op.isBinary():
		return fmt.Sprintf("%s [%s]", e.child0.String(), e.op.symbol())
	default:
		return ""
	}
}

// Evaluate the expression tree.
func (e *expr) eval(addr int, constants map[string]*expr, labels map[string]int) bool {
	if !e.evaluated {
		switch {
		case e.op == opNumber:
			e.evaluated = true

		case e.op == opString:
			e.evaluated = true

		case e.op == opIdentifier:
			var ident string
			switch {
			case e.identifier.startsWithChar('.') || e.identifier.startsWithChar('@'):
				ident = "~" + e.scopeLabel.str + e.identifier.str
			default:
				ident = e.identifier.str
			}
			if m, ok := constants[ident]; ok {
				e.bytes = maxInt(e.bytes, m.bytes)
				if m.address {
					e.address = true
				}
				if m.evaluated {
					e.value, e.bytes, e.evaluated = m.value, m.bytes, true
				}
			}
			if _, ok := labels[ident]; ok {
				e.address, e.bytes = true, 2
			}

		case e.op == opHere:
			if addr != -1 {
				e.value, e.bytes, e.address, e.evaluated = addr, 2, true, true
			}

		case e.op.isBinary():
			e.child0.eval(addr, constants, labels)
			e.child1.eval(addr, constants, labels)
			if e.child0.evaluated && e.child1.evaluated {
				e.value = e.op.eval(e.child0.value, e.child1.value)
				e.bytes = maxInt(e.child0.bytes, e.child1.bytes)
				e.evaluated = true
			}
			if e.child0.address || e.child1.address {
				e.address, e.bytes = true, 2
			}

		default:
			e.child0.eval(addr, constants, labels)
			if e.child0.evaluated {
				e.value = e.op.eval(e.child0.value, 0)
				e.bytes = e.child0.bytes
				e.evaluated = true
			}
			if e.child0.address {
				e.address, e.bytes = true, 2
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
	tokenString
	tokenIdentifier
	tokenHere
	tokenLeftParen
	tokenRightParen
)

func (tt tokentype) isValue() bool {
	return tt == tokenNumber || tt == tokenIdentifier || tt == tokenHere
}

func (tt tokentype) canPrecedeUnaryOp() bool {
	return tt == tokenOp || tt == tokenLeftParen || tt == tokenNil
}

type token struct {
	typ           tokentype
	value         int
	bytes         int
	stringLiteral fstring
	identifier    fstring
	op            exprOp
}

//
// exprParser
//

type exprParser struct {
	operandStack  stack[*expr]
	operatorStack stack[exprOp]
	parenCounter  int
	flags         parseFlags
	prevTokenType tokentype
	errors        []asmerror
}

// Parse an expression from the line until it is exhausted.
func (p *exprParser) parse(line, scopeLabel fstring, flags parseFlags) (e *expr, remain fstring, err error) {
	p.errors = nil
	p.flags = flags
	p.prevTokenType = tokenNil

	orig := line

	// Process expression using Dijkstra's shunting-yard algorithm
	for err == nil {

		// Parse the next expression token
		var token token
		token, remain, err = p.parseToken(line)
		if err != nil {
			break
		}

		// We're done when the token parser returns the nil token
		if token.typ == tokenNil {
			break
		}

		// Handle each possible token type
		switch token.typ {

		case tokenNumber:
			e := &expr{
				op:        opNumber,
				value:     token.value,
				bytes:     token.bytes,
				evaluated: true,
			}
			p.operandStack.push(e)

		case tokenString:
			e := &expr{
				op:            opString,
				stringLiteral: token.stringLiteral,
				isString:      true,
				bytes:         token.bytes,
				evaluated:     true,
			}
			p.operandStack.push(e)

		case tokenIdentifier:
			e := &expr{
				op:         opIdentifier,
				identifier: token.identifier,
				scopeLabel: scopeLabel,
			}
			p.operandStack.push(e)

		case tokenHere:
			e := &expr{
				op:      opHere,
				bytes:   2,
				address: true,
			}
			p.operandStack.push(e)

		case tokenOp:
			for err == nil && !p.operatorStack.empty() && token.op.collapses(p.operatorStack.peek()) {
				err = collapse(&p.operandStack, p.operatorStack.pop())
				if err != nil {
					p.addError(line, "invalid expression")
				}
			}
			p.operatorStack.push(token.op)

		case tokenLeftParen:
			p.operatorStack.push(opLeftParen)

		case tokenRightParen:
			for err == nil {
				if p.operatorStack.empty() {
					p.addError(line, "mismatched parentheses")
					err = errParse
					break
				}
				op := p.operatorStack.pop()
				if op == opLeftParen {
					break
				}
				err = collapse(&p.operandStack, op)
				if err != nil {
					p.addError(line, "invalid expression")
				}
			}

		}
		line = remain
	}

	// Collapse any operators (and operands) remaining on the stack
	for err == nil && !p.operatorStack.empty() {
		err = collapse(&p.operandStack, p.operatorStack.pop())
		if err != nil {
			p.addError(line, "invalid expression")
			err = errParse
		}
	}

	if err == nil {
		e = p.operandStack.peek()
		e.line = orig
	}

	p.reset()
	return e, remain, err
}

// Collapse one or more expression nodes on the top of the
// stack into a combined expression node, and push the combined
// node back onto the stack.
func collapse(s *stack[*expr], op exprOp) error {
	switch {
	case !op.isCollapsible():
		return errParse

	case op.isBinary():
		if len(s.data) < 2 {
			return errParse
		}
		e := &expr{
			op:     op,
			child1: s.pop(),
			child0: s.pop(),
		}
		s.push(e)
		return nil

	default:
		if s.empty() {
			return errParse
		}
		e := &expr{
			op:     op,
			child0: s.pop(),
		}
		s.push(e)
		return nil
	}
}

// Attempt to parse the next token from the line.
func (p *exprParser) parseToken(line fstring) (t token, remain fstring, err error) {
	if line.isEmpty() {
		return token{typ: tokenNil}, line, nil
	}

	switch {
	case line.startsWithChar('$') && (len(line.str) == 1 || !hexadecimal(line.str[1])):
		remain = line.consume(1)
		t.typ = tokenHere
		t.bytes = 2

	case line.startsWith(decimal) || line.startsWithChar('$') || line.startsWithChar('%'):
		t.value, t.bytes, remain, err = p.parseNumber(line)
		t.typ = tokenNumber
		if p.prevTokenType.isValue() || p.prevTokenType == tokenRightParen {
			p.addError(line, "invalid numeric literal")
			err = errParse
		}

	case line.startsWithChar('\''):
		t.value, remain, err = p.parseCharLiteral(line)
		t.bytes = 1
		t.typ = tokenNumber
		if p.prevTokenType.isValue() || p.prevTokenType == tokenRightParen {
			p.addError(line, "invalid character literal")
			err = errParse
		}

	case line.startsWith(stringQuote) && (p.flags&allowStrings) != 0:
		t.stringLiteral, remain, err = p.parseStringLiteral(line)
		t.bytes = len(t.stringLiteral.str)
		t.typ = tokenString

	case line.startsWithChar('(') && (p.flags&allowParentheses) != 0:
		p.parenCounter++
		t.typ, t.op = tokenLeftParen, opLeftParen
		remain = line.consume(1)

	case line.startsWithChar(')') && (p.flags&allowParentheses) != 0:
		if p.parenCounter == 0 {
			p.addError(line, "mismatched parentheses")
			err = errParse
			remain = line.consume(1)
		} else {
			p.parenCounter--
			t.typ, t.op, remain = tokenRightParen, opRightParen, line.consume(1)
		}

	case line.startsWith(identifierStartChar):
		t.typ = tokenIdentifier
		t.identifier, remain = line.consumeWhile(identifierChar)
		if p.prevTokenType.isValue() || p.prevTokenType == tokenRightParen {
			p.addError(line, "invalid identifier")
			err = errParse
		}

	default:
		for i, o := range ops {
			if o.children > 0 && line.startsWithString(o.symbol) {
				if o.isBinary() || (o.isUnary() && p.prevTokenType.canPrecedeUnaryOp()) {
					t.typ, t.op, remain = tokenOp, exprOp(i), line.consume(len(o.symbol))
					break
				}
			}
		}
		if t.typ != tokenOp {
			p.addError(line, "invalid token")
			err = errParse
		}
	}

	p.prevTokenType = t.typ
	remain = remain.consumeWhitespace()
	return t, remain, err
}

// Parse a number from the line. The following numeric formats are allowed:
//
//	[0-9]+           Decimal number
//	$[0-9a-fA-F]+    Hexadecimal number
//	0x[0-9a-fA-F]+   Hexadecimal number
//	0b[01]+          Binary number
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
	// Select decimal, hexadecimal or binary depending on the prefix.
	base, fn, bitsPerChar, negative := 10, decimal, 0, false
	if line.startsWithChar('-') {
		negative = true
		line = line.consume(1)
	}

	switch {
	case line.startsWithChar('$'):
		line = line.consume(1)
		base, fn, bitsPerChar = 16, hexadecimal, 4
	case line.startsWithString("0x"):
		line = line.consume(2)
		base, fn, bitsPerChar = 16, hexadecimal, 4
	case line.startsWithChar('%'):
		line = line.consume(1)
		base, fn, bitsPerChar = 2, binarynum, 1
	case line.startsWithString("0b"):
		line = line.consume(2)
		base, fn, bitsPerChar = 2, binarynum, 1
	}

	numstr, remain := line.consumeWhile(fn)

	num64, converr := strconv.ParseInt(numstr.str, base, 32)
	if converr != nil {
		p.addError(numstr, "invalid numeric literal")
		err = errParse
	}

	value = int(num64)

	if base == 10 {
		switch negative {
		case true:
			switch {
			case value <= 0x80:
				return 0x100 - value, 1, remain, err
			case value <= 0x8000:
				return 0x10000 - value, 2, remain, err
			default:
				return int(int64(0x100000000) - int64(value)), 4, remain, err
			}
		case false:
			switch {
			case value <= 0xff:
				return value, 1, remain, err
			case value <= 0xffff:
				return value, 2, remain, err
			default:
				return value, 4, remain, err
			}
		}
	}

	bytes = (len(numstr.str)*bitsPerChar + 7) / 8
	if bytes > 2 {
		bytes = 4
	}

	if negative {
		value = -value
	}

	return value, bytes, remain, err
}

func (p *exprParser) parseStringLiteral(line fstring) (s, remain fstring, err error) {
	quote := line.str[0]
	remain = line.consume(1)

	s, remain = remain.consumeUntilChar(quote)
	if remain.isEmpty() {
		p.addError(remain, "string literal missing closing quote")
		return fstring{}, remain, errParse
	}

	remain = remain.consume(1)
	return s, remain, nil
}

func (p *exprParser) parseCharLiteral(line fstring) (value int, remain fstring, err error) {
	if len(line.str) < 3 || line.str[2] != '\'' {
		p.addError(line, "invalid character literal")
		return 0, fstring{}, errParse
	}

	value = int(line.str[1])
	remain = line.consume(3)
	return value, remain, nil
}

func (p *exprParser) addError(line fstring, msg string) {
	p.errors = append(p.errors, asmerror{line, msg})
}

func (p *exprParser) reset() {
	p.operandStack.data, p.operatorStack.data = nil, nil
	p.parenCounter = 0
}

//
// stack
//

type stack[T any] struct {
	data []T
}

func (s *stack[T]) push(value T) {
	s.data = append(s.data, value)
}

func (s *stack[T]) pop() T {
	i := len(s.data) - 1
	value := s.data[i]
	s.data = s.data[:i]
	return value
}

func (s *stack[T]) empty() bool {
	return len(s.data) == 0
}

func (s *stack[T]) peek() T {
	return s.data[len(s.data)-1]
}
