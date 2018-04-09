package main

import (
	"fmt"
	"os"
	"os/signal"

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
