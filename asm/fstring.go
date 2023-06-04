// Copyright 2014-2018 Brett Vickers. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package asm

// An fstring is a string that keeps track of its position within the
// file from which it was read.
type fstring struct {
	fileIndex int    // index of file in the assembly
	row       int    // 1-based line number of substring
	column    int    // 0-based column of start of substring
	str       string // the actual substring of interest
	full      string // the full line as originally read from the file
}

func newFstring(fileIndex, row int, str string) fstring {
	return fstring{fileIndex, row, 0, str, str}
}

func (l *fstring) String() string {
	return l.str
}

func (l *fstring) advanceColumn(n int) int {
	c := l.column
	for i := 0; i < n; i++ {
		if l.str[i] == '\t' {
			c += 8 - (c % 8)
		} else {
			c++
		}
	}
	return c
}

func (l fstring) consume(n int) fstring {
	col := l.advanceColumn(n)
	return fstring{l.fileIndex, l.row, col, l.str[n:], l.full}
}

func (l fstring) trunc(n int) fstring {
	return fstring{l.fileIndex, l.row, l.column, l.str[:n], l.full}
}

func (l *fstring) isEmpty() bool {
	return len(l.str) == 0
}

func (l *fstring) startsWith(fn func(c byte) bool) bool {
	return len(l.str) > 0 && fn(l.str[0])
}

func (l *fstring) startsWithChar(c byte) bool {
	return len(l.str) > 0 && l.str[0] == c
}

func (l *fstring) startsWithString(s string) bool {
	return len(l.str) >= len(s) && l.str[:len(s)] == s
}

func (l fstring) consumeWhitespace() fstring {
	return l.consume(l.scanWhile(whitespace))
}

func (l *fstring) scanWhile(fn func(c byte) bool) int {
	i := 0
	for ; i < len(l.str) && fn(l.str[i]); i++ {
	}
	return i
}

func (l *fstring) scanUntil(fn func(c byte) bool) int {
	i := 0
	for ; i < len(l.str) && !fn(l.str[i]); i++ {
	}
	return i
}

func (l *fstring) scanUntilChar(c byte) int {
	i := 0
	for ; i < len(l.str) && l.str[i] != c; i++ {
	}
	return i
}

func (l *fstring) consumeWhile(fn func(c byte) bool) (consumed, remain fstring) {
	i := l.scanWhile(fn)
	consumed, remain = l.trunc(i), l.consume(i)
	return
}

func (l *fstring) consumeUntil(fn func(c byte) bool) (consumed, remain fstring) {
	i := l.scanUntil(fn)
	consumed, remain = l.trunc(i), l.consume(i)
	return
}

func (l *fstring) consumeUntilChar(c byte) (consumed, remain fstring) {
	i := l.scanUntilChar(c)
	consumed, remain = l.trunc(i), l.consume(i)
	return
}

func (l *fstring) consumeUntilUnquotedChar(c byte) (consumed, remain fstring) {
	var quote byte
	i := 0
	for ; i < len(l.str); i++ {
		if quote == 0 {
			if l.str[i] == c {
				break
			}
			if l.str[i] == '\'' || l.str[i] == '"' {
				quote = l.str[i]
			}
		} else {
			if l.str[i] == quote {
				quote = 0
			}
		}
	}
	consumed, remain = l.trunc(i), l.consume(i)
	return
}

func (l fstring) stripTrailingComment() fstring {
	lastNonWS := 0
	for i := 0; i < len(l.str); i++ {
		if comment(l.str[i]) {
			break
		}
		if stringQuote(l.str[i]) {
			q := l.str[i]
			i++
			for ; i < len(l.str) && l.str[i] != q; i++ {
			}
			lastNonWS = i
			if i == len(l.str) {
				break
			}
		}
		if !whitespace(l.str[i]) {
			lastNonWS = i + 1
		}
	}
	return l.trunc(lastNonWS)
}

//
// character helper functions
//

func whitespace(c byte) bool {
	return c == ' ' || c == '\t'
}

func wordChar(c byte) bool {
	return c != ' ' && c != '\t'
}

func alpha(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func decimal(c byte) bool {
	return (c >= '0' && c <= '9')
}

func comment(c byte) bool {
	return c == ';'
}

func hexadecimal(c byte) bool {
	return decimal(c) || (c >= 'A' && c <= 'F') || (c >= 'a' && c <= 'f')
}

func binarynum(c byte) bool {
	return c == '0' || c == '1'
}

func labelStartChar(c byte) bool {
	return alpha(c) || c == '_' || c == '.' || c == '@'
}

func labelChar(c byte) bool {
	return alpha(c) || decimal(c) || c == '_' || c == '.' || c == '@'
}

func identifierStartChar(c byte) bool {
	return alpha(c) || c == '_' || c == '.' || c == '@'
}

func identifierChar(c byte) bool {
	return alpha(c) || decimal(c) || c == '_' || c == '.' || c == '@' || c == ':'
}

func stringQuote(c byte) bool {
	return c == '"' || c == '\''
}
