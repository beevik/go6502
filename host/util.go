// Copyright 2018 Brett Vickers. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package host

import (
	"fmt"
	"strings"
)

func codeString(b []byte) string {
	switch len(b) {
	case 1:
		return fmt.Sprintf("%02X", b[0])
	case 2:
		return fmt.Sprintf("%02X %02X", b[0], b[1])
	case 3:
		return fmt.Sprintf("%02X %02X %02X", b[0], b[1], b[2])
	default:
		return ""
	}
}

func startsWith(s, m string) bool {
	if len(s) < len(m) {
		return false
	}
	return s[:len(m)] == m
}

func endsWith(s, m string) bool {
	if len(s) < len(m) {
		return false
	}
	return s[len(s)-len(m):] == m
}

func stringToBool(s string) (bool, error) {
	s = strings.ToLower(s)
	switch s {
	case "0", "false":
		return false, nil
	case "1", "true":
		return true, nil
	default:
		return false, fmt.Errorf("invalid bool value '%s'", s)
	}
}

func intToBool(v int) bool {
	return v != 0
}

func indentWrap(indent int, s string) string {
	ss := strings.Fields(s)
	if len(ss) == 0 {
		return ""
	}

	counts := make([]int, 0)
	count := 1
	l := indent + len(ss[0])
	for i := 1; i < len(ss); i++ {
		if l+1+len(ss[i]) < 80 {
			count++
			l += 1 + len(ss[i])
			continue
		}

		counts = append(counts, count)
		count = 1
		l = indent + len(ss[i])
	}
	counts = append(counts, count)

	var lines []string
	i := 0
	for _, c := range counts {
		line := strings.Repeat(" ", indent) + strings.Join(ss[i:i+c], " ")
		lines = append(lines, line)
		i += c
	}

	return strings.Join(lines, "\n")
}

var hexString = "0123456789ABCDEF"

func addrToBuf(addr uint16, b []byte) {
	b[0] = hexString[(addr>>12)&0xf]
	b[1] = hexString[(addr>>8)&0xf]
	b[2] = hexString[(addr>>4)&0xf]
	b[3] = hexString[addr&0xf]
}

func byteToBuf(v byte, b []byte) {
	b[0] = hexString[(v>>4)&0xf]
	b[1] = hexString[v&0xf]
}

func toPrintableChar(v byte) byte {
	switch {
	case v >= 32 && v < 127:
		return v
	case v >= 160 && v < 255:
		return v - 128
	default:
		return '.'
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
