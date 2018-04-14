[![Build Status](https://travis-ci.org/beevik/go6502.svg?branch=master)](https://travis-ci.org/beevik/go6502)
[![GoDoc](https://godoc.org/github.com/beevik/go6502?status.svg)](https://godoc.org/github.com/beevik/go6502)

go6502
======

_This project is currently under construction and is changing frequently._

go6502 is a collection of go packages that emulate a 6502 or 65C02 CPU. It
includes a CPU emulator, a cross-assembler, a disassembler, a debugger, and a
host that wraps them all together.

The interactive go6502 console application in the root directory provides
access to all of these features.


# Building the application

Make sure you have all the application's dependencies.

```
go get -u github.com/beevik/go6502
```

Then build go6502.

```
go build
```


# Tutorial

Let's start by considering the go6502 `sample.cmd` script:

```
load monitor.bin $F800
assemble file sample.asm
load sample.bin
set PC START
d .
```

We'll describe what each of these commands does in greater detail later, but
for now know that they do the following things:
1. Load the `monitor.bin` binary file at memory address `F800`.
2. Assemble the `sample.asm` file using the go6502 cross-assembler, generating
   a `sample.bin` binary file and a `sample.map` source map file.
3. Load the `sample.bin` binary file and its corresponding `sample.map` source
   map file. The binary data is loaded into memory at the origin address
   exported during assembly into the `sample.map` file.
4. Set the program counter to the `START` address, which is an address
   exported during assembly into the `sample.map` file.
5. Disassemble the first few lines of machine code starting from the program
   counter address.

To run this script, build the go6502 application and type the following on the
command line:

```
go6502 sample.cmd
```

You should then see:

```
Loaded 'monitor.bin' to $F800..$FFFF
Assembled 'sample.asm' to 'sample.bin'.
Loaded 'sample.bin' to $1000..$10FF
Loaded 'sample.map' source map
Register PC set to $1000.
1000-   A2 EE       LDX   #$EE
1002-   48          PHA
1003-   20 18 10    JSR   $1018
1006-   20 1B 10    JSR   $101B
1009-   20 35 10    JSR   $1035
100C-   20 45 10    JSR   $1045
100F-   F0 06       BEQ   $1017
1011-   A0 3B       LDY   #$3B
1013-   A9 10       LDA   #$10
1015-   A2 55       LDX   #$55

1000-   A2 EE       LDX   #$EE      A=00 X=00 Y=00 PS=[------] SP=FF PC=1000 C=0
*
```

The output shows the result of running each sample script command. Once the
script has finished running, go6502 enters interactive mode and displays a `*`
prompt for further input.

Just before the prompt is a line starting with `1000-`. This line displays the
disassembly of the instruction at the current program counter address and the
state of the CPU registers. The `C` value indicates the number of CPU cycles
that have elapsed since the application started.

## Getting help

Let's enter our first interactive command. Type `help` to see a list of all
commands.

```
* help
go6502 commands:
    annotate         Annotate an address
    assemble         Assemble commands
    breakpoint       Breakpoint commands
    databreakpoint   Data breakpoint commands
    disassemble      Disassemble code
    evaluate         Evaluate an expression
    exports          List exported addresses
    load             Load a binary file
    memory           Memory commands
    quit             Quit the program
    registers        Display register contents
    run              Run the CPU
    set              Set a configuration variable
    step             Step the debugger

*
```

To get more information about a command, type `help` followed by the command
name. In some cases, you will be shown a list of subcommands that must be used
with the command. Let's try `help step`.

```
* help step
Step commands:
    in               Step into next instruction
    over             Step over next instruction

*
```

This response indicates that the `step` command has two possible subcommands.
For example, if you wanted to step the CPU into the next instruction, you
would type `step in`.

Now let's get help on the `step in` command.

```
* help step in
Usage: step in [<count>]

Description:
   Step the CPU by a single instruction. If the instruction is a subroutine
   call, step into the subroutine. The number of steps may be specified as an
   option.

Shortcut: si

*
```

Every command has help text like this. Included in the help text is a
description of the command, a list of shortcuts that can be used to invoke the
command, and a usage hint indicating the arguments accepted by the command.
Usage arguments appear inside `<angle-brackets>`. Optional usage arguments
appear inside square `[<brackets>]`.

## Abbreviating commands

The go6502 application uses a "shortest unambiguous match" parser to process
commands. This means that when entering a command, you need only type the
smallest number of characters that uniquely identify it.  For instance,
instead of typing `quit`, you can type `q` since no other commands start with
the letter Q.

Most commands also have shortcuts. To discover a command's shortcuts, use
`help`.

## Stepping the CPU

Let's use one of the `step` commands to step the CPU by a single instruction.
Type `step in`.

```
1000-   A2 EE       LDX   #$EE      A=00 X=00 Y=00 PS=[------] SP=FF PC=1000 C=0
* step in
1002-   A9 05       LDA   #$05      A=00 X=EE Y=00 PS=[N-----] SP=FF PC=1002 C=2
*
```

By typing `step in`, you are telling the emulated CPU to execute the `LDX #$EE`
instruction at address `1000`. This advances the program counter to `1002`,
loads the value `EE` into the X register, and increases the CPU cycle counter
by 2 cycles.

Each time go6502 advances the program counter interactively, it disassembles
and displays the instruction to be executed next. It also displays the current
values of the CPU registers and cycle counter.

The shortcut for the `step in` command is `si`. Let's type `si 4` to step the
CPU by 4 instructions:

```
1002-   A9 05       LDA   #$05      A=00 X=EE Y=00 PS=[N-----] SP=FF PC=1002 C=2
* si 4
1004-   20 19 10    JSR   $1019     A=05 X=EE Y=00 PS=[------] SP=FF PC=1004 C=4
1019-   A9 FF       LDA   #$FF      A=05 X=EE Y=00 PS=[------] SP=FD PC=1019 C=10
101B-   60          RTS             A=FF X=EE Y=00 PS=[N-----] SP=FD PC=101B C=12
1007-   20 1C 10    JSR   $101C     A=FF X=EE Y=00 PS=[N-----] SP=FF PC=1007 C=18
*
```

This output shows that the CPU has stepped the next 4 instructions starting at
address `1002`.  Each executed instruction is disassembled and displayed along
with the CPU's register values at the start of each instruction. In this
example, a total of 18 CPU cycles have elapsed, and the program counter ends
at address `1007`.  The instruction at `1007` is waiting to be executed.

Note that the `step in` command stepped _into_ the `JSR $1019` subroutine call
rather than stepping _over_ it.  If you weren't interested in stepping through
all the code inside the subroutine, you could have used the `step over`
command instead. This would have caused the debugger to invisibly execute all
instructions inside the subroutine, returning the prompt only after the `RTS`
instruction has executed.

Since the CPU is about to execute another `JSR` instruction, let's try the
`step over` command (or `s` for short).

```
1007-   20 1C 10    JSR   $101C     A=FF X=EE Y=00 PS=[N-----] SP=FF PC=1007 C=18
* s
100A-   20 36 10    JSR   $1036     A=00 X=EE Y=00 PS=[-Z----] SP=FF PC=100A C=70
*
```

After stepping over the `JSR` call at address `1007`, all of the instructions
inside the subroutine at `101C` have been executed, and control has returned
at address `100A` after 52 CPU cycles have elapsed.

## Another shortcut: hit Enter!

One shortcut you will probably use frequently is the blank-line short cut.
Whenever you hit the Enter key instead of typing a command, the go6502
application repeats the previously entered command.

Let's try hitting enter twice to repeat the `step over` command two more
times.

```
100A-   20 36 10    JSR   $1036     A=00 X=EE Y=00 PS=[-Z----] SP=FF PC=100A C=70
*
100D-   20 46 10    JSR   $1046     A=00 X=00 Y=00 PS=[-Z----] SP=FF PC=100D C=103
*
1010-   F0 06       BEQ   $1018     A=00 X=00 Y=00 PS=[-Z----] SP=FF PC=1010 C=136
*
```

go6502 has stepped over two more `JSR` instructions, elapsing another 66 CPU
cycles and leaving the program counter at `1010`.

## Disassembling code

Now let's disassemble some code at the current program counter address to get
a preview of the code about to be executed.  To do this, use the `disassemble`
command or its shortcut `d`.

```
* d .
1010-   F0 06       BEQ   $1018
1012-   A0 3B       LDY   #$3B
1014-   A9 10       LDA   #$10
1016-   A2 56       LDX   #$56
1018-   00          BRK
1019-   A9 FF       LDA   #$FF
101B-   60          RTS
101C-   A9 20       LDA   #$20
101E-   A5 20       LDA   $20
1020-   B5 20       LDA   $20,X
*
```

Note the `.` after the `d` command.  This is shorthand for the current program
counter address.  You may also pass an address or mathematical expression to
disassemble code starting from any address:

```
* d START+2
1002-   A9 05       LDA   #$05
1004-   20 19 10    JSR   $1019
1007-   20 1C 10    JSR   $101C
100A-   20 36 10    JSR   $1036
100D-   20 46 10    JSR   $1046
1010-   F0 06       BEQ   $1018
1012-   A0 3B       LDY   #$3B
1014-   A9 10       LDA   #$10
1016-   A2 56       LDX   #$56
1018-   00          BRK
*
```

By default, go6502 disassembles 10 instructions, but you can disassemble a
different number of instructions by specifying a second argument to the
command.

```
* d . 3
1010-   F0 06       BEQ   $1018    
1012-   A0 3B       LDY   #$3B     
1014-   A9 10       LDA   #$10     
*
```

If you hit the Enter key after using a disassemble command, go6502 will
continue disassembling code from where it left off.

```
*
1016-   A2 56       LDX   #$56     
1018-   00          BRK            
1019-   A9 FF       LDA   #$FF
*
```

If you don't like the number of instructions that go6502 is configured to
disassemble by default, you can change it with the `set` command:

```
* set DisasmLines 20
```


_To be continued..._
