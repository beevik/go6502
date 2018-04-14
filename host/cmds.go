package host

import "github.com/beevik/cmd"

var cmds *cmd.Tree

func init() {
	root := cmd.NewTree("go6502")
	root.AddCommand(cmd.Command{
		Name:        "help",
		Description: "Display help for a command.",
		Usage:       "help [<command>]",
		Data:        (*Host).cmdHelp,
	})
	root.AddCommand(cmd.Command{
		Name:  "annotate",
		Brief: "Annotate an address",
		Description: "Provide a code annotation at a memory address." +
			" When disassembling code at this address, the annotation will" +
			" be displayed.",
		Usage: "annotate <address> <string>",
		Data:  (*Host).cmdAnnotate,
	})

	// Assemble commands
	ass := cmd.NewTree("Assemble")
	root.AddCommand(cmd.Command{
		Name:    "assemble",
		Brief:   "Assemble commands",
		Subtree: ass,
	})
	ass.AddCommand(cmd.Command{
		Name:  "file",
		Brief: "Assemble a file from disk and save the binary to disk",
		Description: "Run the cross-assembler on the specified file," +
			" producing a binary file and source map file if successful.",
		Usage: "assemble file <filename>",
		Data:  (*Host).cmdAssembleFile,
	})
	ass.AddCommand(cmd.Command{
		Name:  "interactive",
		Brief: "Start interactive assembly mode",
		Description: "Start interactive assembler mode. A new prompt will" +
			" appear, allowing you to enter assembly language instructions" +
			" interactively.  Once you type END, the instructions will be" +
			" assembled and stored in memory at the specified address.",
		Usage: "assemble interactive <address>",
		Data:  (*Host).cmdAssembleInteractive,
	})

	// Breakpoint commands
	bp := cmd.NewTree("Breakpoint")
	root.AddCommand(cmd.Command{
		Name:    "breakpoint",
		Brief:   "Breakpoint commands",
		Subtree: bp,
	})
	bp.AddCommand(cmd.Command{
		Name:        "list",
		Brief:       "List breakpoints",
		Description: "List all current breakpoints.",
		Usage:       "breakpoint list",
		Data:        (*Host).cmdBreakpointList,
	})
	bp.AddCommand(cmd.Command{
		Name:  "add",
		Brief: "Add a breakpoint",
		Description: "Add a breakpoint at the specified address." +
			" The breakpoints starts enabled.",
		Usage: "breakpoint add <address>",
		Data:  (*Host).cmdBreakpointAdd,
	})
	bp.AddCommand(cmd.Command{
		Name:        "remove",
		Brief:       "Remove a breakpoint",
		Description: "Remove a breakpoint at the specified address.",
		Usage:       "breakpoint remove <address>",
		Data:        (*Host).cmdBreakpointRemove,
	})
	bp.AddCommand(cmd.Command{
		Name:        "enable",
		Brief:       "Enable a breakpoint",
		Description: "Enable a previously added breakpoint.",
		Usage:       "breakpoint enable <address>",
		Data:        (*Host).cmdBreakpointEnable,
	})
	bp.AddCommand(cmd.Command{
		Name:  "disable",
		Brief: "Disable a breakpoint",
		Description: "Disable a previously added breakpoint. This" +
			" prevents the breakpoint from being hit when running the" +
			" CPU",
		Usage: "breakpoint disable <address>",
		Data:  (*Host).cmdBreakpointDisable,
	})

	// Data breakpoint commands
	dbp := cmd.NewTree("Data breakpoint")
	root.AddCommand(cmd.Command{
		Name:    "databreakpoint",
		Brief:   "Data breakpoint commands",
		Subtree: dbp,
	})
	dbp.AddCommand(cmd.Command{
		Name:        "list",
		Brief:       "List data breakpoints",
		Description: "List all current data breakpoints.",
		Usage:       "databreakpoint list",
		Data:        (*Host).cmdDataBreakpointList,
	})
	dbp.AddCommand(cmd.Command{
		Name:  "add",
		Brief: "Add a data breakpoint",
		Description: "Add a new data breakpoint at the specified" +
			" memory address. When the CPU stores data at this address, the " +
			" breakpoint will stop the CPU. Optionally, a byte " +
			" value may be specified, and the CPU will stop only " +
			" when this value is stored. The data breakpoint starts" +
			" enabled.",
		Usage: "databreakpoint add <address> [<value>]",
		Data:  (*Host).cmdDataBreakpointAdd,
	})
	dbp.AddCommand(cmd.Command{
		Name:  "remove",
		Brief: "Remove a data breakpoint",
		Description: "Remove a previously added data breakpoint at" +
			" the specified memory address.",
		Usage: "databreakpoint remove <address>",
		Data:  (*Host).cmdDataBreakpointRemove,
	})
	dbp.AddCommand(cmd.Command{
		Name:        "enable",
		Brief:       "Enable a data breakpoint",
		Description: "Enable a previously added breakpoint.",
		Usage:       "databreakpoint enable <address>",
		Data:        (*Host).cmdDataBreakpointEnable,
	})
	dbp.AddCommand(cmd.Command{
		Name:        "disable",
		Brief:       "Disable a data breakpoint",
		Description: "Disable a previously added breakpoint.",
		Usage:       "databreakpoint disable <address>",
		Data:        (*Host).cmdDataBreakpointDisable,
	})

	root.AddCommand(cmd.Command{
		Name:  "disassemble",
		Brief: "Disassemble code",
		Description: "Disassemble machine code starting at the requested" +
			" address. The number of instructions to disassemble may be" +
			" specified as an option. If no address is specified, the" +
			" disassembly continues from where the last disassembly left off.",
		Usage: "disassemble [<address>] [<count>]",
		Data:  (*Host).cmdDisassemble,
	})
	root.AddCommand(cmd.Command{
		Name:        "evaluate",
		Brief:       "Evaluate an expression",
		Description: "Evaluate a mathemetical expression.",
		Usage:       "evaluate <expression>",
		Data:        (*Host).cmdEval,
	})
	root.AddCommand(cmd.Command{
		Name:  "exports",
		Brief: "List exported addresses",
		Description: "Display a list of all memory addresses exported by" +
			" loaded binary files. Exported addresses are stored in a binary" +
			" file's associated source map file.",
		Usage: "exports",
		Data:  (*Host).cmdExports,
	})
	root.AddCommand(cmd.Command{
		Name:  "load",
		Brief: "Load a binary file",
		Description: "Load the contents of a binary file into the emulated" +
			" system's memory. If the file has an associated source map, it" +
			" will be loaded too. If the file contains raw binary data, you must" +
			" specify the address where the data will be loaded.",
		Usage: "load <filename> [<address>]",
		Data:  (*Host).cmdLoad,
	})

	// Memory commands
	mem := cmd.NewTree("Memory")
	root.AddCommand(cmd.Command{
		Name:    "memory",
		Brief:   "Memory commands",
		Subtree: mem,
	})
	mem.AddCommand(cmd.Command{
		Name:  "dump",
		Brief: "Dump memory at address",
		Description: "Dump the contents of memory starting from the" +
			" specified address. The number of bytes to dump may be" +
			" specified as an option. If no address is specified, the" +
			" memory dump continues from where the last dump left off.",
		Usage: "memory dump [<address>] [<bytes>]",
		Data:  (*Host).cmdMemoryDump,
	})
	mem.AddCommand(cmd.Command{
		Name:  "set",
		Brief: "Set memory at address",
		Description: "Set the contents of memory starting from the specified" +
			" address. The values to assign should be a series of" +
			" space-separated byte values. You may use an expression for each" +
			" byte value.",
		Usage: "memory set <address> <byte> [<byte> ...]",
		Data:  (*Host).cmdMemorySet,
	})

	root.AddCommand(cmd.Command{
		Name:        "quit",
		Brief:       "Quit the program",
		Description: "Quit the program.",
		Usage:       "quit",
		Data:        (*Host).cmdQuit,
	})
	root.AddCommand(cmd.Command{
		Name:  "registers",
		Brief: "Display register contents",
		Description: "Display the current contents of all CPU registers, and" +
			" disassemble the instruction at the current program counter address.",
		Usage: "registers",
		Data:  (*Host).cmdRegisters,
	})
	root.AddCommand(cmd.Command{
		Name:  "run",
		Brief: "Run the CPU",
		Description: "Run the CPU until a breakpoint is hit or until the" +
			" user types Ctrl-C.",
		Usage: "run",
		Data:  (*Host).cmdRun,
	})
	root.AddCommand(cmd.Command{
		Name:  "set",
		Brief: "Set a configuration variable",
		Description: "Set the value of a configuration variable. To see the" +
			" current values of all configuration variables, type set" +
			" without any arguments.",
		Usage: "set [<var> <value>]",
		Data:  (*Host).cmdSet,
	})

	// Step commands
	step := cmd.NewTree("Step")
	root.AddCommand(cmd.Command{
		Name:    "step",
		Brief:   "Step the debugger",
		Subtree: step,
	})
	step.AddCommand(cmd.Command{
		Name:  "in",
		Brief: "Step into next instruction",
		Description: "Step the CPU by a single instruction. If the" +
			" instruction is a subroutine call, step into the subroutine." +
			" The number of steps may be specified as an option.",
		Usage: "step in [<count>]",
		Data:  (*Host).cmdStepIn,
	})
	step.AddCommand(cmd.Command{
		Name:  "over",
		Brief: "Step over next instruction",
		Description: "Step the CPU by a single instruction. If the" +
			" instruction is a subroutine call, step over the subroutine." +
			" The number of steps may be specified as an option.",
		Usage: "step over [<count>]",
		Data:  (*Host).cmdStepOver,
	})

	// Add command shortcuts.
	root.AddShortcut("a", "assemble file")
	root.AddShortcut("ai", "assemble interactive")
	root.AddShortcut("b", "breakpoint")
	root.AddShortcut("bp", "breakpoint")
	root.AddShortcut("ba", "breakpoint add")
	root.AddShortcut("br", "breakpoint remove")
	root.AddShortcut("bl", "breakpoint list")
	root.AddShortcut("be", "breakpoint enable")
	root.AddShortcut("bd", "breakpoint disable")
	root.AddShortcut("d", "disassemble")
	root.AddShortcut("db", "databreakpoint")
	root.AddShortcut("dbp", "databreakpoint")
	root.AddShortcut("dbl", "databreakpoint list")
	root.AddShortcut("dba", "databreakpoint add")
	root.AddShortcut("dbr", "databreakpoint remove")
	root.AddShortcut("dbe", "databreakpoint enable")
	root.AddShortcut("dbd", "databreakpoint disable")
	root.AddShortcut("e", "evaluate")
	root.AddShortcut("m", "memory dump")
	root.AddShortcut("ms", "memory set")
	root.AddShortcut("r", "registers")
	root.AddShortcut("s", "step over")
	root.AddShortcut("si", "step in")
	root.AddShortcut("?", "help")
	root.AddShortcut(".", "registers")

	cmds = root
}
