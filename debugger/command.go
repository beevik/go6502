package main

import (
	"errors"
	"strings"

	"github.com/beevik/prefixtree"
)

// A Command represents either a single named command or a group of
// subcommands.
type Command struct {
	Name        string      // command string
	Shortcut    string      // optional shortcut for command
	Description string      // description shown in help text
	Param       interface{} // user-defined parameter for this command
	Tree        *Tree       // the command tree this command belongs to
	Subcommands *Tree       // child command tree
}

// A Tree contains one or more commands which may be looked up by
// a shortest unambiguous prefix match.
type Tree struct {
	Title    string    // Description of all commands in tree
	Commands []Command // All commands in the tree
	tree     *prefixtree.Tree
}

// A Selection represents the result of looking up a command in a
// hierarchical command tree. It includes the whitespace-delimited arguments
// following the discovered command, if any.
type Selection struct {
	Command *Command // The selected command
	Args    []string // the command's white-space delimited arguments
}

// Errors returned by the cmd package.
var (
	ErrAmbiguous = errors.New("Command is ambiguous")
	ErrNotFound  = errors.New("Command not found")
)

// NewCommands creates a new Command tree.
func NewCommands(title string, list []Command) *Tree {
	c := &Tree{
		Title:    title,
		Commands: list,
		tree:     prefixtree.New(),
	}
	for i, cc := range c.Commands {
		c.Commands[i].Tree = c
		c.tree.Add(cc.Name, &c.Commands[i])
		if cc.Shortcut != "" {
			c.tree.Add(cc.Shortcut, &c.Commands[i])
		}
	}
	return c
}

// Lookup performs a hierarchical search on a command tree for a matching
// command.
func (c *Tree) Lookup(line string) (Selection, error) {
	ss := strings.SplitN(stripLeadingWhitespace(line), " ", 2)

	var args string
	cmdStr := ss[0]
	if len(ss) > 1 {
		args = stripLeadingWhitespace(ss[1])
	}

	if cmdStr == "" {
		return Selection{}, nil
	}

	ci, err := c.tree.Find(cmdStr)
	switch err {
	case prefixtree.ErrPrefixAmbiguous:
		return Selection{}, ErrAmbiguous
	case prefixtree.ErrPrefixNotFound:
		return Selection{}, ErrNotFound
	}

	cmd := ci.(*Command)
	switch {
	case cmd.Subcommands != nil:
		if args == "" {
			h, err := cmd.Subcommands.Lookup("help")
			if err == nil {
				return h, nil
			}
			return Selection{}, nil
		}
		return cmd.Subcommands.Lookup(args)
	case cmd.Param != nil:
		return Selection{Command: cmd, Args: splitArgs(args)}, nil
	}

	return Selection{}, errors.New("command not found")
}

func stripLeadingWhitespace(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] != ' ' && s[i] != '\t' {
			return s[i:]
		}
	}
	return ""
}

func splitArgs(args string) []string {
	ss := make([]string, 0)
	for len(args) > 0 {
		i := strings.IndexAny(args, " \t")
		if i == -1 {
			if len(args) > 0 {
				ss = append(ss, args)
			}
			break
		}

		if i > 0 {
			arg := args[:i]
			ss = append(ss, arg)
		}

		for i < len(args) && (args[i] == ' ' || args[i] == '\t') {
			i++
		}
		args = args[i:]
	}
	return ss
}
