package main

import (
	"os"
	"os/signal"
)

func main() {
	h := NewHost()

	// Create a goroutine to handle ctrl-C.
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for {
			<-c
			h.println()
			if h.state == stateProcessingCommands {
				h.prompt()
			}
			h.state = stateProcessingCommands
		}
	}()

	// Run commands contained in the command-line files.
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
		h.println()
	}

	// Start the interactive debugger.
	h.RunCommands(os.Stdin, os.Stdout, true)
}
