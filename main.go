// Copyright 2018 Brett Vickers. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"

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

	h := host.New()

	// Do command-line assemble if requested.
	if assemble != "" {
		err := h.AssembleFile(assemble)
		if err != nil {
			fmt.Printf("Failed to assemble file '%s'.\n", assemble)
		}
		os.Exit(0)
	}

	// Run commands contained in command-line files.
	args := flag.Args()
	if len(args) > 0 {
		for _, filename := range args {
			file, err := os.Open(filename)
			if err != nil {
				exitOnError(err)
			}
			h.RunCommands(file, os.Stdout, false)
			file.Close()
		}
	}

	// Break on Ctrl-C.
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go handleInterrupt(h, c)

	// Run commands interactively.
	h.RunCommands(os.Stdin, os.Stdout, true)
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
