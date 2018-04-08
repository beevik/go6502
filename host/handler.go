package host

import "github.com/beevik/go6502"

// The debugger handler receives notifications from the cpu debugger and
// dispatches them to the debugger host.
type handler struct {
	host *Host
}

func (h *handler) OnBreakpoint(cpu *go6502.CPU, b *go6502.Breakpoint) {
	h.host.onBreakpoint(cpu, b)
}

func (h *handler) OnDataBreakpoint(cpu *go6502.CPU, b *go6502.DataBreakpoint) {
	h.host.onDataBreakpoint(cpu, b)
}
