// Copyright 2014-2018 Brett Vickers. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cpu

import "sort"

// The Debugger interface may be implemented to intercept instructions before
// and after they are executed on the emulated CPU.
type Debugger struct {
	breakpointHandler BreakpointHandler
	breakpoints       map[uint16]*Breakpoint
	dataBreakpoints   map[uint16]*DataBreakpoint
}

// The BreakpointHandler interface should be implemented by any object that
// wishes to receive debugger breakpoint notifications.
type BreakpointHandler interface {
	OnBreakpoint(cpu *CPU, b *Breakpoint)
	OnDataBreakpoint(cpu *CPU, b *DataBreakpoint)
}

// A Breakpoint represents an address that will cause the debugger to stop
// code execution when the program counter reaches it.
type Breakpoint struct {
	Address  uint16 // address of execution breakpoint
	Disabled bool   // this breakpoint is currently disabled
}

// A DataBreakpoint represents an address that will cause the debugger to
// stop executing code when a byte is stored to it.
type DataBreakpoint struct {
	Address     uint16 // breakpoint triggered by stores to this address
	Disabled    bool   // this breakpoint is currently disabled
	Conditional bool   // this breakpoint is conditional on a certain Value being stored
	Value       byte   // the value that must be stored if the breakpoint is conditional
}

// NewDebugger creates a new CPU debugger.
func NewDebugger(breakpointHandler BreakpointHandler) *Debugger {
	return &Debugger{
		breakpointHandler: breakpointHandler,
		breakpoints:       make(map[uint16]*Breakpoint),
		dataBreakpoints:   make(map[uint16]*DataBreakpoint),
	}
}

type byBPAddr []*Breakpoint

func (a byBPAddr) Len() int           { return len(a) }
func (a byBPAddr) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byBPAddr) Less(i, j int) bool { return a[i].Address < a[j].Address }

// GetBreakpoint looks up a breakpoint by address and returns it if found.
// Otherwise it returns nil.
func (d *Debugger) GetBreakpoint(addr uint16) *Breakpoint {
	if b, ok := d.breakpoints[addr]; ok {
		return b
	}
	return nil
}

// GetBreakpoints returns all breakpoints currently set in the debugger.
func (d *Debugger) GetBreakpoints() []*Breakpoint {
	var breakpoints []*Breakpoint
	for _, b := range d.breakpoints {
		breakpoints = append(breakpoints, b)
	}
	sort.Sort(byBPAddr(breakpoints))
	return breakpoints
}

// AddBreakpoint adds a new breakpoint address to the debugger. If the
// breakpoint was already set, the request is ignored.
func (d *Debugger) AddBreakpoint(addr uint16) *Breakpoint {
	b := &Breakpoint{Address: addr}
	d.breakpoints[addr] = b
	return b
}

// RemoveBreakpoint removes a breakpoint from the debugger.
func (d *Debugger) RemoveBreakpoint(addr uint16) {
	delete(d.breakpoints, addr)
}

type byDBPAddr []*DataBreakpoint

func (a byDBPAddr) Len() int           { return len(a) }
func (a byDBPAddr) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byDBPAddr) Less(i, j int) bool { return a[i].Address < a[j].Address }

// GetDataBreakpoint looks up a data breakpoint on the provided address
// and returns it if found. Otherwise it returns nil.
func (d *Debugger) GetDataBreakpoint(addr uint16) *DataBreakpoint {
	if b, ok := d.dataBreakpoints[addr]; ok {
		return b
	}
	return nil
}

// GetDataBreakpoints returns all data breakpoints currently set in the
// debugger.
func (d *Debugger) GetDataBreakpoints() []*DataBreakpoint {
	var breakpoints []*DataBreakpoint
	for _, b := range d.dataBreakpoints {
		breakpoints = append(breakpoints, b)
	}
	sort.Sort(byDBPAddr(breakpoints))
	return breakpoints
}

// AddDataBreakpoint adds an unconditional data breakpoint on the requested
// address.
func (d *Debugger) AddDataBreakpoint(addr uint16) *DataBreakpoint {
	b := &DataBreakpoint{Address: addr}
	d.dataBreakpoints[addr] = b
	return b
}

// AddConditionalDataBreakpoint adds a conditional data breakpoint on the
// requested address.
func (d *Debugger) AddConditionalDataBreakpoint(addr uint16, value byte) {
	d.dataBreakpoints[addr] = &DataBreakpoint{
		Address:     addr,
		Conditional: true,
		Value:       value,
	}
}

// RemoveDataBreakpoint removes a (conditional or unconditional) data
// breakpoint at the requested address.
func (d *Debugger) RemoveDataBreakpoint(addr uint16) {
	delete(d.dataBreakpoints, addr)
}

func (d *Debugger) onUpdatePC(cpu *CPU, addr uint16) {
	if d.breakpointHandler != nil {
		if b, ok := d.breakpoints[addr]; ok && !b.Disabled {
			d.breakpointHandler.OnBreakpoint(cpu, b)
		}
	}
}

func (d *Debugger) onDataStore(cpu *CPU, addr uint16, v byte) {
	if d.breakpointHandler != nil {
		if b, ok := d.dataBreakpoints[addr]; ok && !b.Disabled {
			if !b.Conditional || b.Value == v {
				d.breakpointHandler.OnDataBreakpoint(cpu, b)
			}
		}
	}
}
