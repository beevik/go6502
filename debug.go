package go6502

// The Debugger interface may be implemented to intercept instructions before
// and after they are executed on the emulated CPU.
type Debugger struct {
	Handler         DebuggerHandler
	breakpoints     map[uint16]*breakpoint
	dataBreakpoints map[uint16]*dataBreakpoint
}

// The DebuggerHandler interface should be implemented by any object that
// wishes to receive debugger notifications.
type DebuggerHandler interface {
	onBreakpoint(cpu *CPU, addr uint16)
	onDataBreakpoint(cpu *CPU, addr uint16, v byte)
}

type breakpoint struct {
	addr    uint16
	enabled bool
}

type dataBreakpoint struct {
	addr        uint16
	enabled     bool
	conditional bool
	value       byte // if conditional == true
}

// NewDebugger creates a new CPU debugger.
func NewDebugger(handler DebuggerHandler) *Debugger {
	return &Debugger{
		Handler:         handler,
		breakpoints:     make(map[uint16]*breakpoint),
		dataBreakpoints: make(map[uint16]*dataBreakpoint),
	}
}

// AddBreakpoint adds a new breakpoint address to the debugger. If the
// breakpoint was already set, the request is ignored.
func (d *Debugger) AddBreakpoint(addr uint16) {
	b := &breakpoint{addr: addr, enabled: true}
	d.breakpoints[addr] = b
}

// RemoveBreakpoint removes a breakpoint from the debugger.
func (d *Debugger) RemoveBreakpoint(addr uint16) {
	if _, ok := d.breakpoints[addr]; ok {
		delete(d.breakpoints, addr)
	}
}

// EnableBreakpoint enables a breakpoint.
func (d *Debugger) EnableBreakpoint(addr uint16) {
	if b, ok := d.breakpoints[addr]; ok {
		b.enabled = true
	}
}

// DisableBreakpoint disables a breakpoint.
func (d *Debugger) DisableBreakpoint(addr uint16) {
	if b, ok := d.breakpoints[addr]; ok {
		b.enabled = false
	}
}

// AddDataBreakpoint adds an unconditional data breakpoint on the requested
// address.
func (d *Debugger) AddDataBreakpoint(addr uint16) {
	b := &dataBreakpoint{addr: addr, enabled: true, conditional: false}
	d.dataBreakpoints[addr] = b
}

// AddConditionalDataBreakpoint adds a conditional data breakpoint on the
// requested address.
func (d *Debugger) AddConditionalDataBreakpoint(addr uint16, v byte) {
	b := &dataBreakpoint{addr: addr, enabled: true, conditional: true, value: v}
	d.dataBreakpoints[addr] = b
}

// RemoveDataBreakpoint removes a (conditional or unconditional) data
// breakpoint at the requested address.
func (d *Debugger) RemoveDataBreakpoint(addr uint16) {
	if _, ok := d.dataBreakpoints[addr]; ok {
		delete(d.dataBreakpoints, addr)
	}
}

// EnableDataBreakpoint enables a (conditional or unconditional) data
// breakpoint at the requested address.
func (d *Debugger) EnableDataBreakpoint(addr uint16) {
	if b, ok := d.dataBreakpoints[addr]; ok {
		b.enabled = true
	}
}

// DisableDataBreakpoint disables a (conditional or unconditional) data
// breakpoint at the requested address.
func (d *Debugger) DisableDataBreakpoint(addr uint16) {
	if b, ok := d.dataBreakpoints[addr]; ok {
		b.enabled = false
	}
}

func (d *Debugger) onCPUExecute(cpu *CPU, addr uint16) {
	if d.Handler != nil {
		if b, ok := d.breakpoints[addr]; ok && b.enabled {
			d.Handler.onBreakpoint(cpu, addr)
		}
	}
}

func (d *Debugger) onDataStore(cpu *CPU, addr uint16, v byte) {
	if d.Handler != nil {
		if b, ok := d.dataBreakpoints[addr]; ok && b.enabled {
			if !b.conditional || b.value == v {
				d.Handler.onDataBreakpoint(cpu, addr, v)
			}
		}
	}
}
