package main

import (
	"fmt"
	"os"
	"strings"
)

func codeString(bc []byte) string {
	switch len(bc) {
	case 1:
		return fmt.Sprintf("%02X", bc[0])
	case 2:
		return fmt.Sprintf("%02X %02X", bc[0], bc[1])
	case 3:
		return fmt.Sprintf("%02X %02X %02X", bc[0], bc[1], bc[2])
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

func exitOnError(err error) {
	fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
	os.Exit(1)
}

func toBool(s string) bool {
	switch {
	case s == "1" || strings.ToLower(s) == "true":
		return true
	default:
		return false
	}
}
