go6502/cpu
==========

Initialize a 64KB memory space and load some machine code from a byte
slice.
```go
mem := go6502.NewFlatMemory()

// Load a byte slice of machine code at address 0x600
mem.StoreBytes(0x600, []byte{0xa2, 0x05, 0xa1, 0x02, 0xa9, 0x08, 0x8d,
    0x01, 0x02, 0x69, 0xfe, 0x8d, 0x00, 0x02, 0xa9, 0xff, 0xad, 0x00,
    0x02, 0xa2, 0xee, 0x4c, 0x00, 0x06})
```

Create an emulated CMOS 65c02 CPU and initialize its program counter.
```go
cpu := go6502.NewCPU(go6502.CMOS, mem)
cpu.SetPC(0x600)
```

Use the `Step()` function to manually step the CPU one instruction at a time.
```go
for i := 0; i < 20; i++ {
    cpu.Step()
}
```

Use the `go6502/disasm` package to disassemble machine code while stepping
the CPU.
```go
for i := 0; i < 20; i++ {
    pc := cpu.Reg.PC
    line, _, _ := disasm.Disassemble(cpu.Mem, pc)
    cpu.Step()
    fmt.Printf("%04X-   %-12s  A=%02X X=%02X Y=%02X PS=[%s] SP=%02X PC=%04X Cycles=%d\n",
        pc, line,
        cpu.Reg.A, cpu.Reg.X, cpu.Reg.Y, psString(&cpu.Reg),
        cpu.Reg.SP, cpu.Reg.PC,
        cpu.Cycles)
}
```

Output:
```
0600-   LDX #$05      A=00 X=05 Y=00 PS=[------] SP=FF PC=0602 Cycles=2
0602-   LDA ($02,X)   A=00 X=05 Y=00 PS=[-Z----] SP=FF PC=0604 Cycles=8
0604-   LDA #$08      A=08 X=05 Y=00 PS=[------] SP=FF PC=0606 Cycles=10
0606-   STA $0201     A=08 X=05 Y=00 PS=[------] SP=FF PC=0609 Cycles=14
0609-   ADC #$FE      A=06 X=05 Y=00 PS=[C-----] SP=FF PC=060B Cycles=16
060B-   STA $0200     A=06 X=05 Y=00 PS=[C-----] SP=FF PC=060E Cycles=20
060E-   LDA #$FF      A=FF X=05 Y=00 PS=[C----N] SP=FF PC=0610 Cycles=22
0610-   LDA $0200     A=06 X=05 Y=00 PS=[C-----] SP=FF PC=0613 Cycles=26
0613-   LDX #$EE      A=06 X=EE Y=00 PS=[C----N] SP=FF PC=0615 Cycles=28
0615-   JMP $0600     A=06 X=EE Y=00 PS=[C----N] SP=FF PC=0600 Cycles=31
0600-   LDX #$05      A=06 X=05 Y=00 PS=[C-----] SP=FF PC=0602 Cycles=33
0602-   LDA ($02,X)   A=00 X=05 Y=00 PS=[CZ----] SP=FF PC=0604 Cycles=39
0604-   LDA #$08      A=08 X=05 Y=00 PS=[C-----] SP=FF PC=0606 Cycles=41
0606-   STA $0201     A=08 X=05 Y=00 PS=[C-----] SP=FF PC=0609 Cycles=45
0609-   ADC #$FE      A=07 X=05 Y=00 PS=[C-----] SP=FF PC=060B Cycles=47
060B-   STA $0200     A=07 X=05 Y=00 PS=[C-----] SP=FF PC=060E Cycles=51
060E-   LDA #$FF      A=FF X=05 Y=00 PS=[C----N] SP=FF PC=0610 Cycles=53
0610-   LDA $0200     A=07 X=05 Y=00 PS=[C-----] SP=FF PC=0613 Cycles=57
0613-   LDX #$EE      A=07 X=EE Y=00 PS=[C----N] SP=FF PC=0615 Cycles=59
0615-   JMP $0600     A=07 X=EE Y=00 PS=[C----N] SP=FF PC=0600 Cycles=62
```

Here is the implementation of the helper function `psString` used in the 
example:
```go
func psString(r *go6502.Registers) string {
	v := func(bit bool, ch byte) byte {
		if bit {
			return ch
		}
		return '-'
	}
	b := []byte{
		v(r.Carry, 'C'),
		v(r.Zero, 'Z'),
		v(r.InterruptDisable, 'I'),
		v(r.Decimal, 'D'),
		v(r.Overflow, 'O'),
		v(r.Negative, 'N'),
	}
	return string(b)
}
```
