package go6502

// The Debugger interface may be implemented to intercept instructions before
// and after they are executed on the emulated CPU.
type Debugger struct {
	Handler         DebuggerHandler
	breakpoints     map[uint16]*Breakpoint
	dataBreakpoints map[uint16]*DataBreakpoint
}

// The DebuggerHandler interface should be implemented by any object that
// wishes to receive debugger notifications.
type DebuggerHandler interface {
	OnBreakpoint(cpu *CPU, addr uint16)
	OnDataBreakpoint(cpu *CPU, addr uint16, v byte)
}

// A Breakpoint represents an address that will cause the debugger to stop
// code execution.
type Breakpoint struct {
	Address uint16
	Enabled bool
}

// A DataBreakpoint repesents an address that will cause the debugger to
// stop executing code when data is written to it.
type DataBreakpoint struct {
	Address     uint16
	Enabled     bool
	Conditional bool
	Value       byte // if conditional == true
}

// NewDebugger creates a new CPU debugger.
func NewDebugger(handler DebuggerHandler) *Debugger {
	return &Debugger{
		Handler:         handler,
		breakpoints:     make(map[uint16]*Breakpoint),
		dataBreakpoints: make(map[uint16]*DataBreakpoint),
	}
}

// GetBreakpoints returns all breakpoints currently set in the debugger.
func (d *Debugger) GetBreakpoints() []*Breakpoint {
	var breakpoints []*Breakpoint
	for _, b := range d.breakpoints {
		breakpoints = append(breakpoints, b)
	}
	return breakpoints
}

// GetDataBreakpoints returns all data breakpoints currently set in the
// debugger.
func (d *Debugger) GetDataBreakpoints() []*DataBreakpoint {
	var breakpoints []*DataBreakpoint
	for _, b := range d.dataBreakpoints {
		breakpoints = append(breakpoints, b)
	}
	return breakpoints
}

// AddBreakpoint adds a new breakpoint address to the debugger. If the
// breakpoint was already set, the request is ignored.
func (d *Debugger) AddBreakpoint(addr uint16) {
	b := &Breakpoint{Address: addr, Enabled: true}
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
		b.Enabled = true
	}
}

// DisableBreakpoint disables a breakpoint.
func (d *Debugger) DisableBreakpoint(addr uint16) {
	if b, ok := d.breakpoints[addr]; ok {
		b.Enabled = false
	}
}

// AddDataBreakpoint adds an unconditional data breakpoint on the requested
// address.
func (d *Debugger) AddDataBreakpoint(addr uint16) {
	b := &DataBreakpoint{Address: addr, Enabled: true, Conditional: false}
	d.dataBreakpoints[addr] = b
}

// AddConditionalDataBreakpoint adds a conditional data breakpoint on the
// requested address.
func (d *Debugger) AddConditionalDataBreakpoint(addr uint16, v byte) {
	b := &DataBreakpoint{Address: addr, Enabled: true, Conditional: true, Value: v}
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
		b.Enabled = true
	}
}

// DisableDataBreakpoint disables a (conditional or unconditional) data
// breakpoint at the requested address.
func (d *Debugger) DisableDataBreakpoint(addr uint16) {
	if b, ok := d.dataBreakpoints[addr]; ok {
		b.Enabled = false
	}
}

func (d *Debugger) onCPUExecute(cpu *CPU, addr uint16) {
	if d.Handler != nil {
		if b, ok := d.breakpoints[addr]; ok && b.Enabled {
			d.Handler.OnBreakpoint(cpu, addr)
		}
	}
}

func (d *Debugger) onDataStore(cpu *CPU, addr uint16, v byte) {
	if d.Handler != nil {
		if b, ok := d.dataBreakpoints[addr]; ok && b.Enabled {
			if !b.Conditional || b.Value == v {
				d.Handler.OnDataBreakpoint(cpu, addr, v)
			}
		}
	}
}
