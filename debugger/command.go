package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/beevik/prefixtree"
)

type handlerFunc func(h *host, args []string) error

type command struct {
	name        string
	shortcut    string
	description string
	handler     handlerFunc
	commands    *commands
}

type commands struct {
	context string
	list    []command
	tree    *prefixtree.Tree
}

type commandResult struct {
	cmd      *command
	args     string
	helpText string
}

func newCommands(context string, list []command) *commands {
	c := &commands{
		context: context,
		list:    list,
		tree:    prefixtree.New(),
	}
	for i, cc := range c.list {
		c.tree.Add(cc.name, &c.list[i])
		if cc.shortcut != "" {
			c.tree.Add(cc.shortcut, &c.list[i])
		}
	}
	return c
}

func (c *commands) find(line string) (commandResult, error) {
	ss := strings.SplitN(stripLeadingWhitespace(line), " ", 2)

	var args string
	cmd := ss[0]
	if len(ss) > 1 {
		args = stripLeadingWhitespace(ss[1])
	}

	if cmd == "" {
		return commandResult{}, nil
	}

	if cmd == "help" || cmd == "?" {
		return c.getHelp()
	}

	ci, err := c.tree.Find(cmd)
	if err != nil {
		return commandResult{}, err
	}

	cc := ci.(*command)
	switch {
	case cc.handler != nil:
		return commandResult{cmd: cc, args: args}, nil
	case cc.commands != nil:
		if args == "" {
			return cc.commands.getHelp()
		}
		return cc.commands.find(args)
	}

	return commandResult{}, errors.New("command not found")
}

func (c *commands) getHelp() (commandResult, error) {
	lines := []string{fmt.Sprintf("%s commands:\n", c.context)}
	for _, c := range c.list {
		if c.description != "" {
			line := fmt.Sprintf("  %-15s  %s\n", c.name, c.description)
			lines = append(lines, line)
		}
	}
	return commandResult{helpText: strings.Join(lines, "")}, nil
}

func stripLeadingWhitespace(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] != ' ' && s[i] != '\t' {
			return s[i:]
		}
	}
	return ""
}
