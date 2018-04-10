// Copyright 2018 Brett Vickers. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package host

import "github.com/beevik/go6502/cpu"

// The debugHandler receives notifications from the cpu debugger and
// forwards them to the host.
type debugHandler struct {
	host *Host
}

func newDebugHandler(h *Host) *debugHandler {
	return &debugHandler{host: h}
}

func (h *debugHandler) OnBreakpoint(cpu *cpu.CPU, b *cpu.Breakpoint) {
	h.host.onBreakpoint(cpu, b)
}

func (h *debugHandler) OnDataBreakpoint(cpu *cpu.CPU, b *cpu.DataBreakpoint) {
	h.host.onDataBreakpoint(cpu, b)
}
