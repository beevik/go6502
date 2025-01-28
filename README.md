[![GoDoc](https://godoc.org/github.com/beevik/go6502?status.svg)](https://godoc.org/github.com/beevik/go6502)

go6502
======

go6502 is a collection of go packages that emulate a 6502 or 65C02 CPU. It
includes a CPU emulator, a cross-assembler, a disassembler, a debugger, and a
host that wraps them all together.

The interactive go6502 console application in the root directory provides
access to all of these features.


# Building the application

Install go 1.21 or later, and then run `go build` to build the application.


# Tutorial

Start by considering the go6502 `sample.cmd` script:

```
load monitor.bin $F800
assemble file sample.asm
load sample.bin
reg PC START
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
4. Set the program counter register to value of the `START` address, which
   was exported during assembly into the `sample.map` file.
5. Disassemble the first few lines of machine code starting from the program
   counter address.

To run this script, type the following on the command line:

```
go6502 sample.cmd
```

You should then see:

```
Loaded 'monitor.bin' to $F800..$FFFF.
Assembled 'sample.asm' to 'sample.bin'.
Loaded source map from 'sample.bin'.
Loaded 'sample.bin' to $1000..$10FF.
Register PC set to $1000.
Breakpoint added at $1020.
1000-   A2 EE       LDX   #$EE
1002-   A9 05       LDA   #$05
1004-   20 19 10    JSR   $1019
1007-   20 1C 10    JSR   $101C
100A-   20 36 10    JSR   $1036
100D-   20 46 10    JSR   $1046
1010-   F0 06       BEQ   $1018
1012-   A0 3B       LDY   #$3B
1014-   A9 10       LDA   #$10
1016-   A2 56       LDX   #$56

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
go6502 commands:
    annotate         Annotate an address
    assemble         Assemble commands
    breakpoint       Breakpoint commands
    databreakpoint   Data breakpoint commands
    disassemble      Disassemble code
    evaluate         Evaluate an expression
    execute          Execute a go6502 script file
    exports          List exported addresses
    load             Load a binary file
    memory           Memory commands
    quit             Quit the program
    register         View or change register values
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
at address `100A` after 52 additional CPU cycles have elapsed.

## Another shortcut: Hit Enter!

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

## Annotating code

It's often useful to annotate a line of code with a comment. I use annotations
to leave notes to myself when I'm trying to understand how some piece of
machine code works.

Let's consider again the code loaded by the sample script.

```
* d $1000
1000-   A2 EE       LDX   #$EE
1002-   A9 05       LDA   #$05
1004-   20 19 10    JSR   $1019
1007-   20 1C 10    JSR   $101C
100A-   20 36 10    JSR   $1036
100D-   20 46 10    JSR   $1046
1010-   F0 06       BEQ   $1018
1012-   A0 3B       LDY   #$3B
1014-   A9 10       LDA   #$10
1016-   A2 56       LDX   #$56
*
```

The `JSR` instruction at address `1007` calls a subroutine that uses all
the addressing mode variants of the `LDA` command.  Let's add an annotation
to that line of code to remind ourselves later what its purpose is.

```
* annotate $1007 Use different forms of the LDA command
*
```

Now whenever we disassemble code that includes the instruction at address
`1007`, we will see our annotation.

```
* d $1000
1000-   A2 EE       LDX   #$EE
1002-   A9 05       LDA   #$05
1004-   20 19 10    JSR   $1019
1007-   20 1C 10    JSR   $101C     ; Use different forms of the LDA command
100A-   20 36 10    JSR   $1036
100D-   20 46 10    JSR   $1046
1010-   F0 06       BEQ   $1018
1012-   A0 3B       LDY   #$3B
1014-   A9 10       LDA   #$10
1016-   A2 56       LDX   #$56
*
```

To remove an annotation, use the `annotate` command with an address but
without a description.

## Dumping memory

Another common task is dumping the contents of memory.  To do this, use the
`memory dump` command, or `m` for short.

```
* m $1000
1000- A2 EE A9 05 20 19 10 20   "n). ..
1008- 1C 10 20 36 10 20 46 10   .. 6. F.
1010- F0 06 A0 3B A9 10 A2 56   p. ;)."V
1018- 00 A9 FF 60 A9 20 A5 20   .).`) %
1020- B5 20 A1 20 B1 20 AD 00   5 ! 1 -.
1028- 02 AD 20 00 BD 00 02 B9   .- .=..9
1030- 00 02 8D 00 03 60 A2 20   .....`"
1038- A6 20 B6 20 AE 00 02 AE   & 6 ....
*
```

Memory dumps include hexadecimal and ASCII representations of the dumped
memory, starting from the address you specified. By default, the memory
dump shows 64 bytes, but you can specify a different number of bytes to
dump with a second argument.

```
* m $1000 16
1000- A2 EE A9 05 20 19 10 20   "n). ..
1008- 1C 10 20 36 10 20 46 10   .. 6. F.
*
```

As with the `disassemble` command, you can enter a blank line to continue
dumping memory from where you left off:

```
*
1010- F0 06 A0 3B A9 10 A2 56   p. ;)."V
1018- 00 A9 FF 60 A9 20 A5 20   .).`) %
*
1020- B5 20 A1 20 B1 20 AD 00   5 ! 1 -.
1028- 02 AD 20 00 BD 00 02 B9   .- .=..9
*
```

To change the default number of bytes that are dumped by a `memory dump`
command, use the `set` command:

```
* set MemDumpBytes 128
```

## Modifying memory

To change the contents of memory, use the `memory set` command, or `ms` for
short.

```
* ms 0x800 $5A $59 $58 $57
* m 0x800 4
0800- 5A 59 58 57               ZYXW
*
```

A sequence of memory values must be separated by spaces and may include
simple hexadecimal values like shown in the example above, or mathematical
expressions like in the following:

```
* ms 0x800 12*2 'A' 1<<4 $0F^$05
* m 0x800 4
0800- 18 41 10 0A               .A..
*
```

## Aside: Number formats

go6502 accepts numbers in multiple formats. In most of the examples we've seen
so far, addresses and byte values have been specified in base-16 hexadecimal
format using the `$` prefix.

The following table lists the number-formatting options understood by go6502:

 Prefix   | Format      | Base | Example     | Comment
----------|-------------|:----:|-------------|-------------------------
 _(none)_ | Decimal     | 10   | -151        | See note about hex mode.
 `$`      | Hexadecimal | 16   | `$FDED`     |
 `0x`     | Hexadecimal | 16   | `0xfded`    |
 `%`      | Binary      | 2    | `%01011010` |
 `0b`     | Binary      | 2    | `0b01011010`|
 `0d`     | Decimal     | 10   | `0d128`     | Useful in hex mode.

If you prefer to work primarily with hexadecimal numbers, you can change the
"hex mode" setting using the `set` command.

```
* set HexMode true
```

In hex mode, numeric values entered without a prefix are interpreted as
hexadecimal values. However, because hexadecimal numbers include the letters
`A` through `F`, the interpreter is unable to distinguish between a number and
an identifier. So identifiers are not allowed when interpreting expressions in
hex mode.


## Inspecting and changing registers

The 6502 registers can be inspected using the `register` command, or `r`
for short.

```
* r
1000-   A2 EE       LDX   #$EE      A=00 X=00 Y=00 PS=[------] SP=FF PC=1000 C=0
*
```

If you wish to change a register value, simply add additional arguments.

```
* r A $80
Register A set to $80.
1000-   A2 EE       LDX   #$EE      A=80 X=00 Y=00 PS=[------] SP=FF PC=1000 C=0
*
```

Registers you can change this way include `A`, `X`, `Y`, `PC` and `SP`.

You can also change the CPU's status flags. Simply provide one of the flag
names (`N`, `Z`, `C`, `I`, `D` or `V`) instead of a register name.

```
* r Z 1
Status flag ZERO set to true.
1000-   A2 EE       LDX   #$EE      A=80 X=00 Y=00 PS=[-Z----] SP=FF PC=1000 C=0
* r Z 0
Status flag ZERO set to false.
1000-   A2 EE       LDX   #$EE      A=80 X=00 Y=00 PS=[------] SP=FF PC=1000 C=0
*
```

Further info about the `register` command can be found by typing
`help register`.


## Evaluating expressions

Sometimes it's useful to have a calculator on hand to compute the result of a
simple math expression. go6502 has a built-in expression evaluator in the form
of the `evaluate` command, or `e` for short. The evaluator understands most C
expression operators.

```
* e 1<<4
$0010
* e ($FF ^ $AA) | $0100
$0155
* e ('A' + 0x20) | 0x80
$00E1
* e 0b11100101
$00E5
* e -151
$FF69
```

Because go6502 is written for an 8-bit CPU with a 16-bit address space, the
results of all evaluations are displayed as 16-bit values.


## Assembling source code

go6502 has a built-in cross-assembler. To assemble a file on disk into a raw
binary file containing 6502 machine code, use the `assemble file` command (or
`a` for short).

```
* a sample.asm
Assembled 'sample.asm' to 'sample.bin'.
```

The `assemble file` command loads the specified source file, assembles it, and
if successful outputs a raw `.bin` file containing the machine code into the
same directory.  It also produces a `.map` source map file, which is used to
store (1) the "origin" memory address the machine code should be loaded at,
(2) a list of exported address identifiers, and (3) a mapping between source
code lines and memory addresses.

Once assembled, the binary file and its associated source map can be loaded
into memory using the `load` command.

```
* load sample.bin
Loaded source map from 'sample.map'.
Loaded 'sample.bin' to $1000..$10FF.
```


_To be continued..._
