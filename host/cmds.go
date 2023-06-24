package host

import "github.com/beevik/cmd"

var cmds *cmd.Tree

func init() {
	root := cmd.NewTree(cmd.TreeDescriptor{Name: "go6502"})
	root.AddCommand(cmd.CommandDescriptor{
		Name:        "help",
		Description: "Display help for a command.",
		Usage:       "help [<command>]",
		Data:        (*Host).cmdHelp,
	})
	root.AddCommand(cmd.CommandDescriptor{
		Name:  "annotate",
		Brief: "Annotate an address",
		Description: "Provide a code annotation at a memory address." +
			" When disassembling code at this address, the annotation will" +
			" be displayed.",
		Usage: "annotate <address> <string>",
		Data:  (*Host).cmdAnnotate,
	})

	// Assemble commands
	as := root.AddSubtree(cmd.TreeDescriptor{Name: "assemble", Brief: "Assemble commands"})
	as.AddCommand(cmd.CommandDescriptor{
		Name:  "file",
		Brief: "Assemble a file from disk and save the binary to disk",
		Description: "Run the cross-assembler on the specified file," +
			" producing a binary file and source map file if successful." +
			" If you want verbose output, specify true as a second parameter.",
		Usage: "assemble file <filename> [<verbose>]",
		Data:  (*Host).cmdAssembleFile,
	})
	as.AddCommand(cmd.CommandDescriptor{
		Name:  "interactive",
		Brief: "Start interactive assembly mode",
		Description: "Start interactive assembler mode. A new prompt will" +
			" appear, allowing you to enter assembly language instructions" +
			" interactively.  Once you type END, the instructions will be" +
			" assembled and stored in memory at the specified address.",
		Usage: "assemble interactive <address>",
		Data:  (*Host).cmdAssembleInteractive,
	})
	as.AddCommand(cmd.CommandDescriptor{
		Name:  "map",
		Brief: "Create a source map file",
		Description: "Create an empty source map file for an existing binary" +
			" file. Pass the name of the binary file and the origin address it" +
			" should load at.",
		Usage: "assemble map <filename> <origin>",
		Data:  (*Host).cmdAssembleMap,
	})

	// Breakpoint commands
	bp := root.AddSubtree(cmd.TreeDescriptor{Name: "breakpoint", Brief: "Breakpoint commands"})
	bp.AddCommand(cmd.CommandDescriptor{
		Name:        "list",
		Brief:       "List breakpoints",
		Description: "List all current breakpoints.",
		Usage:       "breakpoint list",
		Data:        (*Host).cmdBreakpointList,
	})
	bp.AddCommand(cmd.CommandDescriptor{
		Name:  "add",
		Brief: "Add a breakpoint",
		Description: "Add a breakpoint at the specified address." +
			" The breakpoints starts enabled.",
		Usage: "breakpoint add <address>",
		Data:  (*Host).cmdBreakpointAdd,
	})
	bp.AddCommand(cmd.CommandDescriptor{
		Name:        "remove",
		Brief:       "Remove a breakpoint",
		Description: "Remove a breakpoint at the specified address.",
		Usage:       "breakpoint remove <address>",
		Data:        (*Host).cmdBreakpointRemove,
	})
	bp.AddCommand(cmd.CommandDescriptor{
		Name:        "enable",
		Brief:       "Enable a breakpoint",
		Description: "Enable a previously added breakpoint.",
		Usage:       "breakpoint enable <address>",
		Data:        (*Host).cmdBreakpointEnable,
	})
	bp.AddCommand(cmd.CommandDescriptor{
		Name:  "disable",
		Brief: "Disable a breakpoint",
		Description: "Disable a previously added breakpoint. This" +
			" prevents the breakpoint from being hit when running the" +
			" CPU",
		Usage: "breakpoint disable <address>",
		Data:  (*Host).cmdBreakpointDisable,
	})

	// Data breakpoint commands
	db := root.AddSubtree(cmd.TreeDescriptor{Name: "databreakpoint", Brief: "Data Breakpoint commands"})
	db.AddCommand(cmd.CommandDescriptor{
		Name:        "list",
		Brief:       "List data breakpoints",
		Description: "List all current data breakpoints.",
		Usage:       "databreakpoint list",
		Data:        (*Host).cmdDataBreakpointList,
	})
	db.AddCommand(cmd.CommandDescriptor{
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
	db.AddCommand(cmd.CommandDescriptor{
		Name:  "remove",
		Brief: "Remove a data breakpoint",
		Description: "Remove a previously added data breakpoint at" +
			" the specified memory address.",
		Usage: "databreakpoint remove <address>",
		Data:  (*Host).cmdDataBreakpointRemove,
	})
	db.AddCommand(cmd.CommandDescriptor{
		Name:        "enable",
		Brief:       "Enable a data breakpoint",
		Description: "Enable a previously added breakpoint.",
		Usage:       "databreakpoint enable <address>",
		Data:        (*Host).cmdDataBreakpointEnable,
	})
	db.AddCommand(cmd.CommandDescriptor{
		Name:        "disable",
		Brief:       "Disable a data breakpoint",
		Description: "Disable a previously added breakpoint.",
		Usage:       "databreakpoint disable <address>",
		Data:        (*Host).cmdDataBreakpointDisable,
	})

	root.AddCommand(cmd.CommandDescriptor{
		Name:  "disassemble",
		Brief: "Disassemble code",
		Description: "Disassemble machine code starting at the requested" +
			" address. The number of instruction lines to disassemble may be" +
			" specified as an option. If no address is specified, the" +
			" disassembly continues from where the last disassembly left off.",
		Usage: "disassemble [<address>] [<lines>]",
		Data:  (*Host).cmdDisassemble,
	})
	root.AddCommand(cmd.CommandDescriptor{
		Name:        "evaluate",
		Brief:       "Evaluate an expression",
		Description: "Evaluate a mathemetical expression.",
		Usage:       "evaluate <expression>",
		Data:        (*Host).cmdEvaluate,
	})
	root.AddCommand(cmd.CommandDescriptor{
		Name:  "execute",
		Brief: "Execute a go6502 script file",
		Description: "Load a go6502 script file from disk and execute the" +
			" commands it contains.",
		Usage: "execute <filename>",
		Data:  (*Host).cmdExecute,
	})
	root.AddCommand(cmd.CommandDescriptor{
		Name:  "exports",
		Brief: "List exported addresses",
		Description: "Display a list of all memory addresses exported by" +
			" loaded binary files. Exported addresses are stored in a binary" +
			" file's associated source map file.",
		Usage: "exports",
		Data:  (*Host).cmdExports,
	})
	root.AddCommand(cmd.CommandDescriptor{
		Name:  "list",
		Brief: "List source code lines",
		Description: "List the source code corresponding to the machine code" +
			" at the specified address. A source map containing the address must" +
			" have been previously loaded.",
		Usage: "list <address> [<lines>]",
		Data:  (*Host).cmdList,
	})
	root.AddCommand(cmd.CommandDescriptor{
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
	me := root.AddSubtree(cmd.TreeDescriptor{Name: "memory", Brief: "Memory commands"})
	me.AddCommand(cmd.CommandDescriptor{
		Name:  "dump",
		Brief: "Dump memory at address",
		Description: "Dump the contents of memory starting from the" +
			" specified address. The number of bytes to dump may be" +
			" specified as an option. If no address is specified, the" +
			" memory dump continues from where the last dump left off.",
		Usage: "memory dump [<address>] [<bytes>]",
		Data:  (*Host).cmdMemoryDump,
	})
	me.AddCommand(cmd.CommandDescriptor{
		Name:  "set",
		Brief: "Set memory at address",
		Description: "Set the contents of memory starting from the specified" +
			" address. The values to assign should be a series of" +
			" space-separated byte values. You may use an expression for each" +
			" byte value.",
		Usage: "memory set <address> <byte> [<byte> ...]",
		Data:  (*Host).cmdMemorySet,
	})
	me.AddCommand(cmd.CommandDescriptor{
		Name:  "copy",
		Brief: "Copy memory",
		Description: "Copy memory from one range of addresses to another. You" +
			" must specify the destination address, the first byte of the source" +
			" address, and the last byte of the source address.",
		Usage: "memory copy <dst addr> <src addr begin> <src addr end>",
		Data:  (*Host).cmdMemoryCopy,
	})

	root.AddCommand(cmd.CommandDescriptor{
		Name:        "quit",
		Brief:       "Quit the program",
		Description: "Quit the program.",
		Usage:       "quit",
		Data:        (*Host).cmdQuit,
	})
	root.AddCommand(cmd.CommandDescriptor{
		Name:  "register",
		Brief: "View or change register values",
		Description: "When used without arguments, this command displays the current" +
			" contents of the CPU registers.  When used with arguments, this" +
			" command changes the value of a register or one of the CPU's status" +
			" flags. Allowed register names include A, X, Y, PC and SP. Allowed status" +
			" flag names include N (Sign), Z (Zero), C (Carry), I (InterruptDisable)," +
			" D (Decimal) and V (Overflow).",
		Usage: "register [<name> <value>]",
		Data:  (*Host).cmdRegister,
	})
	root.AddCommand(cmd.CommandDescriptor{
		Name:  "run",
		Brief: "Run the CPU",
		Description: "Run the CPU until a breakpoint is hit or until the" +
			" user types Ctrl-C.",
		Usage: "run",
		Data:  (*Host).cmdRun,
	})
	root.AddCommand(cmd.CommandDescriptor{
		Name:  "set",
		Brief: "Set a configuration variable",
		Description: "Set the value of a configuration variable. To see the" +
			" current values of all configuration variables, type set" +
			" without any arguments.",
		Usage: "set [<var> <value>]",
		Data:  (*Host).cmdSet,
	})

	// Step commands
	st := root.AddSubtree(cmd.TreeDescriptor{Name: "step", Brief: "Step the debugger"})
	st.AddCommand(cmd.CommandDescriptor{
		Name:  "in",
		Brief: "Step into next instruction",
		Description: "Step the CPU by a single instruction. If the" +
			" instruction is a subroutine call, step into the subroutine." +
			" The number of steps may be specified as an option.",
		Usage: "step in [<count>]",
		Data:  (*Host).cmdStepIn,
	})
	st.AddCommand(cmd.CommandDescriptor{
		Name:  "over",
		Brief: "Step over next instruction",
		Description: "Step the CPU by a single instruction. If the" +
			" instruction is a subroutine call, step over the subroutine." +
			" The number of steps may be specified as an option.",
		Usage: "step over [<count>]",
		Data:  (*Host).cmdStepOver,
	})
	st.AddCommand(cmd.CommandDescriptor{
		Name:  "out",
		Brief: "Step out of the current subroutine",
		Description: "Step the CPU until it executes an RTS or RTI" +
			" instruction. This has the effect of stepping until the " +
			" currently running subroutine has returned.",
		Usage: "step out",
		Data:  (*Host).cmdStepOut,
	})

	// Add command shortcuts.
	root.AddShortcut("a", "assemble file")
	root.AddShortcut("ai", "assemble interactive")
	root.AddShortcut("am", "assemble map")
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
	root.AddShortcut("l", "list")
	root.AddShortcut("m", "memory dump")
	root.AddShortcut("mc", "memory copy")
	root.AddShortcut("ms", "memory set")
	root.AddShortcut("r", "register")
	root.AddShortcut("s", "step over")
	root.AddShortcut("si", "step in")
	root.AddShortcut("so", "step out")
	root.AddShortcut("?", "help")
	root.AddShortcut(".", "register")

	cmds = root
}
