load monitor.bin $F800
assemble file sample.asm
load sample.bin
set compact true
reg PC START
d .
