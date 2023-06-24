// Copyright 2018 Brett Vickers. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"

	"github.com/beevik/go6502/asm"
	"github.com/beevik/go6502/host"
)

var (
	assemble string
)

func init() {
	flag.StringVar(&assemble, "a", "", "assemble file")
	flag.CommandLine.Usage = func() {
		fmt.Println("Usage: go6502 [script] ..\nOptions:")
		flag.PrintDefaults()
	}
}

func main() {
	flag.Parse()

	// Initiate assembly from the command line if requested.
	if assemble != "" {
		err := asm.AssembleFile(assemble, 0, os.Stdout)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to assemble (%v).\n", err)
		}
		os.Exit(0)
	}

	// Create the host
	h := host.New()
	defer h.Cleanup()

	// Run commands contained in command-line files.
	args := flag.Args()
	if len(args) > 0 {
		for _, filename := range args {
			file, err := os.Open(filename)
			if err != nil {
				exitOnError(err)
			}
			ioState := h.EnableProcessedMode(file, os.Stdout)
			h.RunCommands(false)
			h.RestoreIoState(ioState)
			file.Close()
		}
	}

	// Break on Ctrl-C.
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go handleInterrupt(h, c)

	// Run commands interactively. Disable raw mode on Windows because the
	// golang.org/x/term package is buggy on Windows.
	if runtime.GOOS == "windows" {
		h.EnableProcessedMode(os.Stdin, os.Stdout)
	} else {
		h.EnableRawMode()
	}

	// Interactively run commands entered by the user.
	h.RunCommands(true)
}

func handleInterrupt(h *host.Host, c chan os.Signal) {
	for {
		<-c
		h.Break()
	}
}

func exitOnError(err error) {
	fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
	os.Exit(1)
}
