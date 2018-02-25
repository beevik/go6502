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
mem.CopyBytes(0x600, []byte{0xa2, 0x05, 0xa1, 0x02, 0xa9, 0x08, 0x8d,
    0x01, 0x02, 0x69, 0xfe, 0x8d, 0x00, 0x02, 0xa9, 0xff, 0xad, 0x00,
    0x02, 0xa2, 0xee, 0x4c, 0x00, 0x06})
```

### Example: Creating and initializing the emulated CPU

Create and initialize the program counter of a 6502 processor emulator.
```go
cpu := go6502.NewCPU(mem)
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

Output:
```
0600-   LDX #$05     [A=00 X=05 Y=00 PS=00 SP=FF PC=0602] Cycles=2
0602-   LDA ($02,X)  [A=00 X=05 Y=00 PS=02 SP=FF PC=0604] Cycles=8
0604-   LDA #$08     [A=08 X=05 Y=00 PS=00 SP=FF PC=0606] Cycles=10
0606-   STA $0201    [A=08 X=05 Y=00 PS=00 SP=FF PC=0609] Cycles=14
0609-   ADC #$FE     [A=06 X=05 Y=00 PS=01 SP=FF PC=060B] Cycles=16
060B-   STA $0200    [A=06 X=05 Y=00 PS=01 SP=FF PC=060E] Cycles=20
060E-   LDA #$FF     [A=FF X=05 Y=00 PS=81 SP=FF PC=0610] Cycles=22
0610-   LDA $0200    [A=06 X=05 Y=00 PS=01 SP=FF PC=0613] Cycles=26
0613-   LDX #$EE     [A=06 X=EE Y=00 PS=81 SP=FF PC=0615] Cycles=28
0615-   JMP $0600    [A=06 X=EE Y=00 PS=81 SP=FF PC=0600] Cycles=31
0600-   LDX #$05     [A=06 X=05 Y=00 PS=01 SP=FF PC=0602] Cycles=33
0602-   LDA ($02,X)  [A=00 X=05 Y=00 PS=03 SP=FF PC=0604] Cycles=39
0604-   LDA #$08     [A=08 X=05 Y=00 PS=01 SP=FF PC=0606] Cycles=41
0606-   STA $0201    [A=08 X=05 Y=00 PS=01 SP=FF PC=0609] Cycles=45
0609-   ADC #$FE     [A=07 X=05 Y=00 PS=01 SP=FF PC=060B] Cycles=47
060B-   STA $0200    [A=07 X=05 Y=00 PS=01 SP=FF PC=060E] Cycles=51
060E-   LDA #$FF     [A=FF X=05 Y=00 PS=81 SP=FF PC=0610] Cycles=53
0610-   LDA $0200    [A=07 X=05 Y=00 PS=01 SP=FF PC=0613] Cycles=57
0613-   LDX #$EE     [A=07 X=EE Y=00 PS=81 SP=FF PC=0615] Cycles=59
0615-   JMP $0600    [A=07 X=EE Y=00 PS=81 SP=FF PC=0600] Cycles=62
```
