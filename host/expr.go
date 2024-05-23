// Copyright 2018 Brett Vickers. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package host

import (
	"errors"
	"strconv"
)

var errExprParse = errors.New("expression syntax error")

type tokenType byte

const (
	tokenNil tokenType = iota
	tokenIdentifier
	tokenNumber
	tokenOp
	tokenLParen
	tokenRParen
)

type token struct {
	Type  tokenType
	Value any // nil, string, int64 or *op (depends on Type)
}

type opType byte

const (
	opNil opType = 0 + iota
	opMultiply
	opDivide
	opModulo
	opAdd
	opSubtract
	opShiftLeft
	opShiftRight
	opBitwiseAnd
	opBitwiseXor
	opBitwiseOr
	opBitwiseNot
	opUnaryMinus
	opUnaryPlus
	opUnaryBinary
)

type associativity byte

const (
	left associativity = iota
	right
)

type op struct {
	Symbol     string
	Type       opType
	Precedence byte
	Assoc      associativity
	Args       byte
	UnaryOp    opType
	Eval       func(a, b int64) int64
}

var ops = []op{
	{"", opNil, 0, right, 2, opNil, nil},
	{"*", opMultiply, 6, right, 2, opNil, func(a, b int64) int64 { return a * b }},
	{"/", opDivide, 6, right, 2, opNil, func(a, b int64) int64 { return a / b }},
	{"%", opModulo, 6, right, 2, opUnaryBinary, func(a, b int64) int64 { return a % b }},
	{"+", opAdd, 5, right, 2, opUnaryPlus, func(a, b int64) int64 { return a + b }},
	{"-", opSubtract, 5, right, 2, opUnaryMinus, func(a, b int64) int64 { return a - b }},
	{"<<", opShiftLeft, 4, right, 2, opNil, func(a, b int64) int64 { return a << uint32(b) }},
	{">>", opShiftRight, 4, right, 2, opNil, func(a, b int64) int64 { return a >> uint32(b) }},
	{"&", opBitwiseAnd, 3, right, 2, opNil, func(a, b int64) int64 { return a & b }},
	{"^", opBitwiseXor, 2, right, 2, opNil, func(a, b int64) int64 { return a ^ b }},
	{"|", opBitwiseOr, 1, right, 2, opNil, func(a, b int64) int64 { return a | b }},
	{"~", opBitwiseNot, 7, left, 1, opNil, func(a, b int64) int64 { return ^a }},
	{"-", opUnaryMinus, 7, left, 1, opNil, func(a, b int64) int64 { return -a }},
	{"+", opUnaryPlus, 7, left, 1, opNil, func(a, b int64) int64 { return a }},
	{"%", opUnaryBinary, 7, left, 1, opNil, func(a, b int64) int64 { return fromBinary(a) }},
}

// lexeme identifiers
const (
	lNil byte = iota
	lNum
	lCha
	lIde
	lLPa
	lRPa
	lMul
	lDiv
	lMod
	lAdd
	lSub
	lShl
	lShr
	lAnd
	lXor
	lOra
	lNot
)

// A table mapping lexeme identifiers to token data and parsers.
var lexeme = []struct {
	TokenType tokenType
	OpType    opType
	Parse     func(p *exprParser, t tstring) (tok token, remain tstring, err error)
}{
	/*lNil*/ {TokenType: tokenNil, OpType: opNil},
	/*lNum*/ {TokenType: tokenNumber, OpType: opNil, Parse: (*exprParser).parseNumber},
	/*lCha*/ {TokenType: tokenNumber, OpType: opNil, Parse: (*exprParser).parseChar},
	/*lIde*/ {TokenType: tokenIdentifier, OpType: opNil, Parse: (*exprParser).parseIdentifier},
	/*lLPa*/ {TokenType: tokenLParen, OpType: opNil},
	/*lRPa*/ {TokenType: tokenRParen, OpType: opNil},
	/*lMul*/ {TokenType: tokenOp, OpType: opMultiply},
	/*lDiv*/ {TokenType: tokenOp, OpType: opDivide},
	/*lMod*/ {TokenType: tokenOp, OpType: opModulo},
	/*lAdd*/ {TokenType: tokenOp, OpType: opAdd},
	/*lSub*/ {TokenType: tokenOp, OpType: opSubtract},
	/*lShl*/ {TokenType: tokenOp, OpType: opNil, Parse: (*exprParser).parseShiftOp},
	/*lShr*/ {TokenType: tokenOp, OpType: opNil, Parse: (*exprParser).parseShiftOp},
	/*lAnd*/ {TokenType: tokenOp, OpType: opBitwiseAnd},
	/*lXor*/ {TokenType: tokenOp, OpType: opBitwiseXor},
	/*lOra*/ {TokenType: tokenOp, OpType: opBitwiseOr},
	/*lNot*/ {TokenType: tokenOp, OpType: opBitwiseNot},
}

// A table mapping the first char of a lexeme to a lexeme identifier.
var lex0 = [96]byte{
	lNil, lNil, lNil, lNil, lNum, lMod, lAnd, lCha, // 32..39
	lLPa, lRPa, lMul, lAdd, lNil, lSub, lIde, lDiv, // 40..47
	lNum, lNum, lNum, lNum, lNum, lNum, lNum, lNum, // 48..55
	lNum, lNum, lNil, lNil, lShl, lNil, lShr, lNil, // 56..63
	lNil, lIde, lIde, lIde, lIde, lIde, lIde, lIde, // 64..71
	lIde, lIde, lIde, lIde, lIde, lIde, lIde, lIde, // 72..79
	lIde, lIde, lIde, lIde, lIde, lIde, lIde, lIde, // 80..87
	lIde, lIde, lIde, lNil, lNil, lNil, lXor, lIde, // 88..95
	lNil, lIde, lIde, lIde, lIde, lIde, lIde, lIde, // 96..103
	lIde, lIde, lIde, lIde, lIde, lIde, lIde, lIde, // 104..111
	lIde, lIde, lIde, lIde, lIde, lIde, lIde, lIde, // 112..119
	lIde, lIde, lIde, lNil, lOra, lNil, lNot, lNil, // 120..127
}

type resolver interface {
	resolveIdentifier(s string) (int64, error)
}

//
// exprParser
//

type exprParser struct {
	output        tokenStack
	operatorStack tokenStack
	prevTokenType tokenType
	hexMode       bool
}

func newExprParser() *exprParser {
	return &exprParser{}
}

func (p *exprParser) Reset() {
	p.output.reset()
	p.operatorStack.reset()
	p.prevTokenType = tokenNil
}

func (p *exprParser) Parse(expr string, r resolver) (int64, error) {
	defer p.Reset()

	t := tstring(expr)

	for {
		tok, remain, err := p.parseToken(t)
		if err != nil {
			return 0, err
		}
		if tok.Type == tokenNil {
			break
		}
		t = remain

		switch tok.Type {
		case tokenNumber:
			p.output.push(tok)

		case tokenIdentifier:
			v, err := r.resolveIdentifier(tok.Value.(string))
			if err != nil {
				return 0, err
			}
			tok.Type, tok.Value = tokenNumber, v
			p.output.push(tok)

		case tokenLParen:
			p.operatorStack.push(tok)

		case tokenRParen:
			foundLParen := false
			for !p.operatorStack.isEmpty() {
				tmp := p.operatorStack.pop()
				if tmp.Type == tokenLParen {
					foundLParen = true
					break
				}
				p.output.push(tmp)
			}
			if !foundLParen {
				return 0, errExprParse
			}

		case tokenOp:
			if err := p.checkForUnaryOp(&tok); err != nil {
				return 0, err
			}
			for p.isCollapsible(&tok) {
				p.output.push(p.operatorStack.pop())
			}
			p.operatorStack.push(tok)
		}

		p.prevTokenType = tok.Type
	}

	for !p.operatorStack.isEmpty() {
		tok := p.operatorStack.pop()
		if tok.Type == tokenLParen {
			return 0, errExprParse
		}
		p.output.push(tok)
	}

	result, err := p.evalOutput()
	if err != nil {
		return 0, err
	}
	if !p.output.isEmpty() {
		return 0, errExprParse
	}

	return result.Value.(int64), nil
}

func (p *exprParser) parseToken(t tstring) (tok token, remain tstring, err error) {
	t = t.consumeWhitespace()

	// Return the nil token when there are no more tokens to parse.
	if len(t) == 0 {
		return token{}, t, nil
	}

	// Use the first character of the token string to look up lexeme
	// parser data.
	if t[0] < 32 || t[0] > 127 {
		return token{}, t, errExprParse
	}
	lex := lexeme[lex0[t[0]-32]]

	// One-character lexemes require no additional parsing to generate the
	// token.
	if lex.Parse == nil {
		tok = token{lex.TokenType, nil}
		if lex.OpType != opNil {
			tok.Value = &ops[lex.OpType]
		}
		return tok, t.consume(1), nil
	}

	// Lexemes that are more than one character in length require custom
	// parsing to generate the token.
	return lex.Parse(p, t)
}

func (p *exprParser) parseNumber(t tstring) (tok token, remain tstring, err error) {
	base, fn, num := 10, decimal, t

	if p.hexMode {
		base, fn = 16, hexadecimal
	}

	switch num[0] {
	case '$':
		if len(num) < 2 {
			return token{}, t, errExprParse
		}
		base, fn, num = 16, hexadecimal, num.consume(1)

	case '0':
		if len(num) > 1 && (num[1] == 'x' || num[1] == 'b' || num[1] == 'd') {
			if len(num) < 3 {
				return token{}, t, errExprParse
			}
			switch num[1] {
			case 'x':
				base, fn = 16, hexadecimal
			case 'b':
				base, fn = 2, binary
			case 'd':
				base, fn = 10, decimal
			}
			num = num.consume(2)
		}
	}

	num, remain = num.consumeWhile(fn)
	if num == "" {
		return token{}, t, errExprParse
	}

	v, err := strconv.ParseInt(string(num), base, 64)
	if err != nil {
		return token{}, t, errExprParse
	}

	tok = token{tokenNumber, v}
	return tok, remain, nil
}

func (p *exprParser) parseChar(t tstring) (tok token, remain tstring, err error) {
	if len(t) < 3 || t[2] != '\'' {
		return tok, t, errExprParse
	}

	tok = token{tokenNumber, int64(t[1])}
	return tok, t.consume(3), nil
}

func (p *exprParser) parseIdentifier(t tstring) (tok token, remain tstring, err error) {
	if p.hexMode {
		return p.parseNumber(t)
	}

	var id tstring
	id, remain = t.consumeWhile(identifier)
	tok = token{tokenIdentifier, string(id)}
	return tok, remain, nil
}

func (p *exprParser) parseShiftOp(t tstring) (tok token, remain tstring, err error) {
	if len(t) < 2 || t[1] != t[0] {
		return token{}, t, errExprParse
	}

	var op *op
	switch t[0] {
	case '<':
		op = &ops[opShiftLeft]
	default:
		op = &ops[opShiftRight]
	}

	tok = token{tokenOp, op}
	return tok, t.consume(2), nil
}

func (p *exprParser) evalOutput() (token, error) {
	if p.output.isEmpty() {
		return token{}, errExprParse
	}

	tok := p.output.pop()
	if tok.Type == tokenNumber {
		return tok, nil
	}
	if tok.Type != tokenOp {
		return token{}, errExprParse
	}

	op := tok.Value.(*op)
	switch op.Args {
	case 1:
		child, err := p.evalOutput()
		if err != nil {
			return token{}, err
		}
		tok.Type = tokenNumber
		tok.Value = op.Eval(child.Value.(int64), 0)
		return tok, nil

	default:
		child2, err := p.evalOutput()
		if err != nil {
			return token{}, err
		}
		child1, err := p.evalOutput()
		if err != nil {
			return token{}, err
		}

		tok.Type = tokenNumber
		tok.Value = op.Eval(child1.Value.(int64), child2.Value.(int64))
		return tok, nil
	}
}

func (p *exprParser) checkForUnaryOp(tok *token) error {
	o := tok.Value.(*op)
	if o.UnaryOp == opNil {
		return nil
	}

	// If this operation follows an operation, a left parenthesis, or nothing,
	// then convert it to a unary op.
	if p.prevTokenType == tokenOp || p.prevTokenType == tokenLParen || p.prevTokenType == tokenNil {
		tok.Value = &ops[o.UnaryOp]
	}
	return nil
}

func (p *exprParser) isCollapsible(opToken *token) bool {
	if p.operatorStack.isEmpty() {
		return false
	}

	top := p.operatorStack.peek()
	if top.Type != tokenOp {
		return false
	}

	currOp := opToken.Value.(*op)
	topOp := top.Value.(*op)
	if topOp.Precedence > currOp.Precedence {
		return true
	}
	if topOp.Precedence == currOp.Precedence && topOp.Assoc == left {
		return true
	}
	return false
}

//
// tokenStack
//

type tokenStack struct {
	stack []token
}

func (s *tokenStack) reset() {
	s.stack = s.stack[:0]
}

func (s *tokenStack) isEmpty() bool {
	return len(s.stack) == 0
}

func (s *tokenStack) peek() *token {
	return &s.stack[len(s.stack)-1]
}

func (s *tokenStack) push(t token) {
	s.stack = append(s.stack, t)
}

func (s *tokenStack) pop() token {
	top := len(s.stack) - 1
	t := s.stack[top]
	s.stack = s.stack[:top]
	return t
}

//
// helpers
//

func fromBinary(a int64) int64 {
	v, err := strconv.ParseInt(strconv.FormatInt(a, 10), 2, 64)
	if err != nil {
		return 0
	}
	return v
}

//
// tstring
//

type tstring string

func (t tstring) consume(n int) tstring {
	return t[n:]
}

func (t tstring) consumeWhitespace() tstring {
	return t.consume(t.scanWhile(whitespace))
}

func (t tstring) scanWhile(fn func(c byte) bool) int {
	i := 0
	for ; i < len(t) && fn(t[i]); i++ {
	}
	return i
}

func (t tstring) consumeWhile(fn func(c byte) bool) (consumed, remain tstring) {
	i := t.scanWhile(fn)
	return t[:i], t[i:]
}

func whitespace(c byte) bool {
	return c == ' ' || c == '\t'
}

func decimal(c byte) bool {
	return (c >= '0' && c <= '9')
}

func hexadecimal(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'A' && c <= 'F') || (c >= 'a' && c <= 'f')
}

func binary(c byte) bool {
	return c == '0' || c == '1'
}

func identifier(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '.'
}
