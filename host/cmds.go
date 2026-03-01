package host

import (
	_ "embed"

	"github.com/beevik/cmd"
)

//go:embed cmds.json
var cmdsJSON string

var cmds *cmd.Tree

func init() {
	var err error
	cmds, err = cmd.NewTreeFromJSON(cmdsJSON)
	if err != nil {
		panic("error parsing cmds.json: " + err.Error())
	}

	cmds.SetData("help", (*Host).cmdHelp)
	cmds.SetData("annotate", (*Host).cmdAnnotate)
	cmds.SetData("assemble file", (*Host).cmdAssembleFile)
	cmds.SetData("assemble interactive", (*Host).cmdAssembleInteractive)
	cmds.SetData("assemble map", (*Host).cmdAssembleMap)
	cmds.SetData("breakpoint list", (*Host).cmdBreakpointList)
	cmds.SetData("breakpoint add", (*Host).cmdBreakpointAdd)
	cmds.SetData("breakpoint remove", (*Host).cmdBreakpointRemove)
	cmds.SetData("breakpoint enable", (*Host).cmdBreakpointEnable)
	cmds.SetData("breakpoint disable", (*Host).cmdBreakpointDisable)
	cmds.SetData("databreakpoint list", (*Host).cmdDataBreakpointList)
	cmds.SetData("databreakpoint add", (*Host).cmdDataBreakpointAdd)
	cmds.SetData("databreakpoint remove", (*Host).cmdDataBreakpointRemove)
	cmds.SetData("databreakpoint enable", (*Host).cmdDataBreakpointEnable)
	cmds.SetData("databreakpoint disable", (*Host).cmdDataBreakpointDisable)
	cmds.SetData("disassemble", (*Host).cmdDisassemble)
	cmds.SetData("evaluate", (*Host).cmdEvaluate)
	cmds.SetData("execute", (*Host).cmdExecute)
	cmds.SetData("exports", (*Host).cmdExports)
	cmds.SetData("list", (*Host).cmdList)
	cmds.SetData("load", (*Host).cmdLoad)
	cmds.SetData("memory dump", (*Host).cmdMemoryDump)
	cmds.SetData("memory set", (*Host).cmdMemorySet)
	cmds.SetData("memory copy", (*Host).cmdMemoryCopy)
	cmds.SetData("quit", (*Host).cmdQuit)
	cmds.SetData("register", (*Host).cmdRegister)
	cmds.SetData("run", (*Host).cmdRun)
	cmds.SetData("set", (*Host).cmdSet)
	cmds.SetData("step in", (*Host).cmdStepIn)
	cmds.SetData("step over", (*Host).cmdStepOver)
	cmds.SetData("step out", (*Host).cmdStepOut)
}
