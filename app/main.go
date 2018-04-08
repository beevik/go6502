package main

import (
	"fmt"
	"os"

	"github.com/beevik/go6502/host"
)

func main() {
	h := host.New()

	// Run commands contained in command-line files.
	args := os.Args[1:]
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

	// Run commands interactively.
	h.RunCommands(os.Stdin, os.Stdout, true)
}

func exitOnError(err error) {
	fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
	os.Exit(1)
}
