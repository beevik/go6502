// Copyright 2014-2018 Brett Vickers. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package asm

var hex = "0123456789ABCDEF"

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func hexchar(c byte) byte {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10
	default:
		return 0
	}
}

func hexToByte(s string) byte {
	return hexchar(s[0])<<4 | hexchar(s[1])
}

// Return a little-endian representation of the value using the requested
// number of bytes.
func toBytes(bytes, value int) []byte {
	switch bytes {
	case 1:
		return []byte{byte(value)}
	case 2:
		return []byte{byte(value), byte(value >> 8)}
	default:
		return []byte{byte(value), byte(value >> 8), byte(value >> 16), byte(value >> 24)}
	}
}

// Return a hexadecimal string representation of a byte slice.
func byteString(b []byte) string {
	if len(b) < 1 {
		return ""
	}

	s := make([]byte, len(b)*3-1)
	i, j := 0, 0
	for n := len(b) - 1; i < n; i, j = i+1, j+3 {
		s[j+0] = hex[(b[i] >> 4)]
		s[j+1] = hex[(b[i] & 0x0f)]
		s[j+2] = ' '
	}
	s[j+0] = hex[(b[i] >> 4)]
	s[j+1] = hex[(b[i] & 0x0f)]
	return string(s)
}
