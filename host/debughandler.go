package host

import "github.com/beevik/go6502"

// The debugHandler receives notifications from the cpu debugger and
// forwards them to the host.
type debugHandler struct {
	host *Host
}

func newDebugHandler(h *Host) *debugHandler {
	return &debugHandler{host: h}
}

func (h *debugHandler) OnBreakpoint(cpu *go6502.CPU, b *go6502.Breakpoint) {
	h.host.onBreakpoint(cpu, b)
}

func (h *debugHandler) OnDataBreakpoint(cpu *go6502.CPU, b *go6502.DataBreakpoint) {
	h.host.onDataBreakpoint(cpu, b)
}
