go6502
======

The go6502 package implements a 6502 CPU instruction set, emulator,
assembler, and disassembler.

See http://godoc.org/github.com/beevik/go6502 for the godoc-formatted API
documentation.

The assembler and disassembler are currently under construction, so they
may have a significant number of bugs.

### Example: Loading memory

Initialize a 64KB memory space and load some machine code from a byte
slice.
```go
mem := go6502.NewMemory()

// Load a byte slice of machine code at address 0x600
mem.LoadBytes(0x600, []byte{0xa2, 0x05, 0xa1, 0x02, 0xa9, 0x08, 0x8d,
    0x01, 0x02, 0x69, 0xfe, 0x8d, 0x00, 0x02, 0xa9, 0xff, 0xad, 0x00,
    0x02, 0xa2, 0xee, 0x4c, 0x00, 0x06})
```

### Example: Creating and initializing the emulated CPU

Create and initialize the program counter of a 6502 processor emulator.
```go
cpu := go6502.NewCpu(mem)
cpu.SetPC(0x600)
```

### Example: Stepping the CPU manually

To manually step the CPU one instruction at a time, use the Step function.
```go
for i := 0; i < 20; i++ {
    cpu.Step()
}
```

### Example: Disassembling machine code

To disassemble machine code while stepping the CPU, use the go6502.disasm
package.
```go
for i := 0; i < 20; i++ {
    pc := cpu.Reg.PC
    line, _ := disasm.Disassemble(cpu.Mem, pc)
    cpu.Step()
    fmt.Printf("%04X-   %-12s [A=%02X X=%02X Y=%02X PS=%02X SP=%02X PC=%04X] Cycles=%d\n",
        pc, line,
        cpu.Reg.A, cpu.Reg.X, cpu.Reg.Y, cpu.Reg.PS
        cpu.Reg.SP, cpu.Reg.PC,
        cpu.Cycles)
}
```
