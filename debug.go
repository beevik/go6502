package go6502

// The Debugger interface may be implemented to intercept instructions before
// and after they are executed on the emulated CPU.
type Debugger struct {
	Handler         DebuggerHandler
	breakpoints     map[uint16]Breakpoint
	dataBreakpoints map[uint16]DataBreakpoint
}

// The DebuggerHandler interface should be implemented by any object that
// wishes to receive debugger notifications.
type DebuggerHandler interface {
	OnBreakpoint(cpu *CPU, b Breakpoint)
	OnDataBreakpoint(cpu *CPU, b DataBreakpoint)
}

// A Breakpoint represents an address that will cause the debugger to stop
// code execution.
type Breakpoint struct {
	Address  uint16
	Disabled bool
}

// A DataBreakpoint repesents an address that will cause the debugger to
// stop executing code when data is written to it.
type DataBreakpoint struct {
	Address     uint16
	Disabled    bool
	Conditional bool
	Value       byte // if conditional == true
}

// NewDebugger creates a new CPU debugger.
func NewDebugger(handler DebuggerHandler) *Debugger {
	return &Debugger{
		Handler:         handler,
		breakpoints:     make(map[uint16]Breakpoint),
		dataBreakpoints: make(map[uint16]DataBreakpoint),
	}
}

// GetBreakpoints returns all breakpoints currently set in the debugger.
func (d *Debugger) GetBreakpoints() []Breakpoint {
	var breakpoints []Breakpoint
	for _, b := range d.breakpoints {
		breakpoints = append(breakpoints, b)
	}
	return breakpoints
}

// HasBreakpoint returns true if there is a breakpoint set on the
// requested address.
func (d *Debugger) HasBreakpoint(addr uint16) bool {
	_, ok := d.breakpoints[addr]
	return ok
}

// AddBreakpoint adds a new breakpoint address to the debugger. If the
// breakpoint was already set, the request is ignored.
func (d *Debugger) AddBreakpoint(addr uint16) {
	d.breakpoints[addr] = Breakpoint{Address: addr}
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
		b.Disabled = false
	}
}

// DisableBreakpoint disables a breakpoint.
func (d *Debugger) DisableBreakpoint(addr uint16) {
	if b, ok := d.breakpoints[addr]; ok {
		b.Disabled = true
	}
}

// GetDataBreakpoints returns all data breakpoints currently set in the
// debugger.
func (d *Debugger) GetDataBreakpoints() []DataBreakpoint {
	var breakpoints []DataBreakpoint
	for _, b := range d.dataBreakpoints {
		breakpoints = append(breakpoints, b)
	}
	return breakpoints
}

// HasDataBreakpoint returns true if the debugger has a data breakpoint
// set on the requested address.
func (d *Debugger) HasDataBreakpoint(addr uint16) bool {
	_, ok := d.dataBreakpoints[addr]
	return ok
}

// AddDataBreakpoint adds an unconditional data breakpoint on the requested
// address.
func (d *Debugger) AddDataBreakpoint(addr uint16) {
	d.dataBreakpoints[addr] = DataBreakpoint{Address: addr}
}

// AddConditionalDataBreakpoint adds a conditional data breakpoint on the
// requested address.
func (d *Debugger) AddConditionalDataBreakpoint(addr uint16, value byte) {
	d.dataBreakpoints[addr] = DataBreakpoint{Address: addr, Conditional: true, Value: value}
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
		b.Disabled = false
	}
}

// DisableDataBreakpoint disables a (conditional or unconditional) data
// breakpoint at the requested address.
func (d *Debugger) DisableDataBreakpoint(addr uint16) {
	if b, ok := d.dataBreakpoints[addr]; ok {
		b.Disabled = true
	}
}

func (d *Debugger) onUpdatePC(cpu *CPU, addr uint16) {
	if d.Handler != nil {
		if b, ok := d.breakpoints[addr]; ok && !b.Disabled {
			d.Handler.OnBreakpoint(cpu, b)
		}
	}
}

func (d *Debugger) onDataStore(cpu *CPU, addr uint16, v byte) {
	if d.Handler != nil {
		if b, ok := d.dataBreakpoints[addr]; ok && !b.Disabled {
			if !b.Conditional || b.Value == v {
				d.Handler.OnDataBreakpoint(cpu, b)
			}
		}
	}
}
