package go6502

// An opsym is an internal symbol used to associate opcode data
// with its implementation.
type opsym byte

const (
	symADC opsym = iota
	symAND
	symASL
	symBCC
	symBCS
	symBEQ
	symBIT
	symBMI
	symBNE
	symBPL
	symBRK
	symBVC
	symBVS
	symCLC
	symCLD
	symCLI
	symCLV
	symCMP
	symCPX
	symCPY
	symDEC
	symDEX
	symDEY
	symEOR
	symINC
	symINX
	symINY
	symJMP
	symJSR
	symLDA
	symLDX
	symLDY
	symLSR
	symNOP
	symORA
	symPHA
	symPHP
	symPLA
	symPLP
	symROL
	symROR
	symRTI
	symRTS
	symSBC
	symSEC
	symSED
	symSEI
	symSTA
	symSTX
	symSTY
	symTAX
	symTAY
	symTSX
	symTXA
	symTXS
	symTYA
)

type instfunc func(c *CPU, inst *Instruction, operand []byte)

// Opcode name and function implementation
type opcodeImpl struct {
	sym    opsym
	name   string
	fnCMOS instfunc
	fnNMOS instfunc
}

var impl = []opcodeImpl{
	{symADC, "ADC", (*CPU).adcc, (*CPU).adcn},
	{symAND, "AND", (*CPU).and, (*CPU).and},
	{symASL, "ASL", (*CPU).asl, (*CPU).asl},
	{symBCC, "BCC", (*CPU).bcc, (*CPU).bcc},
	{symBCS, "BCS", (*CPU).bcs, (*CPU).bcs},
	{symBEQ, "BEQ", (*CPU).beq, (*CPU).beq},
	{symBIT, "BIT", (*CPU).bit, (*CPU).bit},
	{symBMI, "BMI", (*CPU).bmi, (*CPU).bmi},
	{symBNE, "BNE", (*CPU).bne, (*CPU).bne},
	{symBPL, "BPL", (*CPU).bpl, (*CPU).bpl},
	{symBRK, "BRK", (*CPU).brk, (*CPU).brk},
	{symBVC, "BVC", (*CPU).bvc, (*CPU).bvc},
	{symBVS, "BVS", (*CPU).bvs, (*CPU).bvs},
	{symCLC, "CLC", (*CPU).clc, (*CPU).clc},
	{symCLD, "CLD", (*CPU).cld, (*CPU).cld},
	{symCLI, "CLI", (*CPU).cli, (*CPU).cli},
	{symCLV, "CLV", (*CPU).clv, (*CPU).clv},
	{symCMP, "CMP", (*CPU).cmp, (*CPU).cmp},
	{symCPX, "CPX", (*CPU).cpx, (*CPU).cpx},
	{symCPY, "CPY", (*CPU).cpy, (*CPU).cpy},
	{symDEC, "DEC", (*CPU).dec, (*CPU).dec},
	{symDEX, "DEX", (*CPU).dex, (*CPU).dex},
	{symDEY, "DEY", (*CPU).dey, (*CPU).dey},
	{symEOR, "EOR", (*CPU).eor, (*CPU).eor},
	{symINC, "INC", (*CPU).inc, (*CPU).inc},
	{symINX, "INX", (*CPU).inx, (*CPU).inx},
	{symINY, "INY", (*CPU).iny, (*CPU).iny},
	{symJMP, "JMP", (*CPU).jmp, (*CPU).jmp},
	{symJSR, "JSR", (*CPU).jsr, (*CPU).jsr},
	{symLDA, "LDA", (*CPU).lda, (*CPU).lda},
	{symLDX, "LDX", (*CPU).ldx, (*CPU).ldx},
	{symLDY, "LDY", (*CPU).ldy, (*CPU).ldy},
	{symLSR, "LSR", (*CPU).lsr, (*CPU).lsr},
	{symNOP, "NOP", (*CPU).nop, (*CPU).nop},
	{symORA, "ORA", (*CPU).ora, (*CPU).ora},
	{symPHA, "PHA", (*CPU).pha, (*CPU).pha},
	{symPHP, "PHP", (*CPU).php, (*CPU).php},
	{symPLA, "PLA", (*CPU).pla, (*CPU).pla},
	{symPLP, "PLP", (*CPU).plp, (*CPU).plp},
	{symROL, "ROL", (*CPU).rol, (*CPU).rol},
	{symROR, "ROR", (*CPU).ror, (*CPU).ror},
	{symRTI, "RTI", (*CPU).rti, (*CPU).rti},
	{symRTS, "RTS", (*CPU).rts, (*CPU).rts},
	{symSBC, "SBC", (*CPU).sbcc, (*CPU).sbcn},
	{symSEC, "SEC", (*CPU).sec, (*CPU).sec},
	{symSED, "SED", (*CPU).sed, (*CPU).sed},
	{symSEI, "SEI", (*CPU).sei, (*CPU).sei},
	{symSTA, "STA", (*CPU).sta, (*CPU).sta},
	{symSTX, "STX", (*CPU).stx, (*CPU).stx},
	{symSTY, "STY", (*CPU).sty, (*CPU).sty},
	{symTAX, "TAX", (*CPU).tax, (*CPU).tax},
	{symTAY, "TAY", (*CPU).tay, (*CPU).tay},
	{symTSX, "TSX", (*CPU).tsx, (*CPU).tsx},
	{symTXA, "TXA", (*CPU).txa, (*CPU).txa},
	{symTXS, "TXS", (*CPU).txs, (*CPU).txs},
	{symTYA, "TYA", (*CPU).tya, (*CPU).tya},
}

// Mode describes a memory addressing mode.
type Mode byte

// All possible memory addressing modes
const (
	IMM Mode = iota // Immediate
	IMP             // Implied
	REL             // Relative
	ZPG             // Zero Page
	ZPX             // Zero Page,X
	ZPY             // Zero Page,Y
	ABS             // Absolute
	ABX             // Absolute,X
	ABY             // Absolute,Y
	IND             // (Indirect)
	IDX             // (Indirect,X)
	IDY             // (Indirect),Y
	ACC             // Accumulator
)

// Opcode data for an (opcode, mode) pair
type opcodeData struct {
	sym      opsym // internal opcode key value
	mode     Mode  // addressing mode
	opcode   byte  // opcode hex value
	length   byte  // length of opcode + operand in bytes
	cycles   byte  // number of CPU cycles to execute command
	bpcycles byte  // additional CPU cycles if command crosses page boundary
}

// All valid (opcode, mode) pairs
var data = []opcodeData{
	{symLDA, IMM, 0xa9, 2, 2, 0},
	{symLDA, ZPG, 0xa5, 2, 3, 0},
	{symLDA, ZPX, 0xb5, 2, 4, 0},
	{symLDA, ABS, 0xad, 3, 4, 0},
	{symLDA, ABX, 0xbd, 3, 4, 1},
	{symLDA, ABY, 0xb9, 3, 4, 1},
	{symLDA, IDX, 0xa1, 2, 6, 0},
	{symLDA, IDY, 0xb1, 2, 5, 1},

	{symLDX, IMM, 0xa2, 2, 2, 0},
	{symLDX, ZPG, 0xa6, 2, 3, 0},
	{symLDX, ZPY, 0xb6, 2, 4, 0},
	{symLDX, ABS, 0xae, 3, 4, 0},
	{symLDX, ABY, 0xbe, 3, 4, 1},

	{symLDY, IMM, 0xa0, 2, 2, 0},
	{symLDY, ZPG, 0xa4, 2, 3, 0},
	{symLDY, ZPX, 0xb4, 2, 4, 0},
	{symLDY, ABS, 0xac, 3, 4, 0},
	{symLDY, ABX, 0xbc, 3, 4, 1},

	{symSTA, ZPG, 0x85, 2, 3, 0},
	{symSTA, ZPX, 0x95, 2, 4, 0},
	{symSTA, ABS, 0x8d, 3, 4, 0},
	{symSTA, ABX, 0x9d, 3, 5, 0},
	{symSTA, ABY, 0x99, 3, 5, 0},
	{symSTA, IDX, 0x81, 2, 6, 0},
	{symSTA, IDY, 0x91, 2, 6, 0},

	{symSTX, ZPG, 0x86, 2, 3, 0},
	{symSTX, ZPY, 0x97, 2, 4, 0},
	{symSTX, ABS, 0x8e, 3, 4, 0},

	{symSTY, ZPG, 0x84, 2, 3, 0},
	{symSTY, ZPX, 0x94, 2, 4, 0},
	{symSTY, ABS, 0x8c, 3, 4, 0},

	{symADC, IMM, 0x69, 2, 2, 0},
	{symADC, ZPG, 0x65, 2, 3, 0},
	{symADC, ZPX, 0x75, 2, 4, 0},
	{symADC, ABS, 0x6d, 3, 4, 0},
	{symADC, ABX, 0x7d, 3, 4, 1},
	{symADC, ABY, 0x79, 3, 4, 1},
	{symADC, IDX, 0x61, 2, 6, 0},
	{symADC, IDY, 0x71, 2, 5, 1},

	{symSBC, IMM, 0xe9, 2, 2, 0},
	{symSBC, ZPG, 0xe5, 2, 3, 0},
	{symSBC, ZPX, 0xf5, 2, 4, 0},
	{symSBC, ABS, 0xed, 3, 4, 0},
	{symSBC, ABX, 0xfd, 3, 4, 1},
	{symSBC, ABY, 0xf9, 3, 4, 1},
	{symSBC, IDX, 0xe1, 2, 6, 0},
	{symSBC, IDY, 0xf1, 2, 5, 1},

	{symCMP, IMM, 0xc9, 2, 2, 0},
	{symCMP, ZPG, 0xc5, 2, 3, 0},
	{symCMP, ZPX, 0xd5, 2, 4, 0},
	{symCMP, ABS, 0xcd, 3, 4, 0},
	{symCMP, ABX, 0xdd, 3, 4, 1},
	{symCMP, ABY, 0xd9, 3, 4, 1},
	{symCMP, IDX, 0xc1, 2, 6, 0},
	{symCMP, IDY, 0xd1, 2, 5, 1},

	{symCPX, IMM, 0xe0, 2, 2, 0},
	{symCPX, ZPG, 0xe4, 2, 3, 0},
	{symCPX, ABS, 0xec, 3, 4, 0},

	{symCPY, IMM, 0xc0, 2, 2, 0},
	{symCPY, ZPG, 0xc4, 2, 3, 0},
	{symCPY, ABS, 0xcc, 3, 4, 0},

	{symBIT, ZPG, 0x24, 2, 3, 0},
	{symBIT, ABS, 0x2c, 3, 4, 0},

	{symCLC, IMP, 0x18, 1, 2, 0},
	{symSEC, IMP, 0x38, 1, 2, 0},
	{symCLI, IMP, 0x58, 1, 2, 0},
	{symSEI, IMP, 0x78, 1, 2, 0},
	{symCLD, IMP, 0xd8, 1, 2, 0},
	{symSED, IMP, 0xf8, 1, 2, 0},
	{symCLV, IMP, 0xb8, 1, 2, 0},

	{symBCC, REL, 0x90, 2, 2, 1},
	{symBCS, REL, 0xb0, 2, 2, 1},
	{symBEQ, REL, 0xf0, 2, 2, 1},
	{symBNE, REL, 0xd0, 2, 2, 1},
	{symBMI, REL, 0x30, 2, 2, 1},
	{symBPL, REL, 0x10, 2, 2, 1},
	{symBVC, REL, 0x50, 2, 2, 1},
	{symBVS, REL, 0x70, 2, 2, 1},

	{symBRK, IMP, 0x00, 1, 7, 0},

	{symAND, IMM, 0x29, 2, 2, 0},
	{symAND, ZPG, 0x25, 2, 3, 0},
	{symAND, ZPX, 0x35, 2, 4, 0},
	{symAND, ABS, 0x2d, 3, 4, 0},
	{symAND, ABX, 0x3d, 3, 4, 1},
	{symAND, ABY, 0x39, 3, 4, 1},
	{symAND, IDX, 0x21, 2, 6, 0},
	{symAND, IDY, 0x31, 2, 5, 1},

	{symORA, IMM, 0x09, 2, 2, 0},
	{symORA, ZPG, 0x05, 2, 3, 0},
	{symORA, ZPX, 0x15, 2, 4, 0},
	{symORA, ABS, 0x0d, 3, 4, 0},
	{symORA, ABX, 0x1d, 3, 4, 1},
	{symORA, ABY, 0x19, 3, 4, 1},
	{symORA, IDX, 0x01, 2, 6, 0},
	{symORA, IDY, 0x11, 2, 5, 1},

	{symEOR, IMM, 0x49, 2, 2, 0},
	{symEOR, ZPG, 0x45, 2, 3, 0},
	{symEOR, ZPX, 0x55, 2, 4, 0},
	{symEOR, ABS, 0x4d, 3, 4, 0},
	{symEOR, ABX, 0x5d, 3, 4, 1},
	{symEOR, ABY, 0x59, 3, 4, 1},
	{symEOR, IDX, 0x41, 2, 6, 0},
	{symEOR, IDY, 0x51, 2, 5, 1},

	{symINC, ZPG, 0xe6, 2, 5, 0},
	{symINC, ZPX, 0xf6, 2, 6, 0},
	{symINC, ABS, 0xee, 3, 6, 0},
	{symINC, ABX, 0xfe, 3, 7, 0},

	{symDEC, ZPG, 0xc6, 2, 5, 0},
	{symDEC, ZPX, 0xd6, 2, 6, 0},
	{symDEC, ABS, 0xce, 3, 6, 0},
	{symDEC, ABX, 0xde, 3, 7, 0},

	{symINX, IMP, 0xe8, 1, 2, 0},
	{symINY, IMP, 0xc8, 1, 2, 0},

	{symDEX, IMP, 0xca, 1, 2, 0},
	{symDEY, IMP, 0x88, 1, 2, 0},

	{symJMP, ABS, 0x4c, 3, 3, 0},
	{symJMP, IND, 0x6c, 3, 5, 0},

	{symJSR, ABS, 0x20, 3, 6, 0},
	{symRTS, IMP, 0x60, 1, 6, 0},

	{symRTI, IMP, 0x40, 1, 6, 0},

	{symNOP, IMM, 0xea, 1, 2, 0},

	{symTAX, IMP, 0xaa, 1, 2, 0},
	{symTXA, IMP, 0x8a, 1, 2, 0},
	{symTAY, IMP, 0xa8, 1, 2, 0},
	{symTYA, IMP, 0x98, 1, 2, 0},
	{symTXS, IMP, 0x9a, 1, 2, 0},
	{symTSX, IMP, 0xba, 1, 2, 0},

	{symPHA, IMP, 0x48, 1, 3, 0},
	{symPLA, IMP, 0x68, 1, 4, 0},
	{symPHP, IMP, 0x08, 1, 3, 0},
	{symPLP, IMP, 0x28, 1, 4, 0},

	{symASL, ACC, 0x0a, 1, 2, 0},
	{symASL, ZPG, 0x06, 2, 5, 0},
	{symASL, ZPX, 0x16, 2, 6, 0},
	{symASL, ABS, 0x0e, 3, 6, 0},
	{symASL, ABX, 0x1e, 3, 7, 0},

	{symLSR, ACC, 0x4a, 1, 2, 0},
	{symLSR, ZPG, 0x46, 2, 5, 0},
	{symLSR, ZPX, 0x56, 2, 6, 0},
	{symLSR, ABS, 0x4e, 3, 6, 0},
	{symLSR, ABX, 0x5e, 3, 7, 0},

	{symROL, ACC, 0x2a, 1, 2, 0},
	{symROL, ZPG, 0x26, 2, 5, 0},
	{symROL, ZPX, 0x36, 2, 6, 0},
	{symROL, ABS, 0x2e, 3, 6, 0},
	{symROL, ABX, 0x3e, 3, 7, 0},

	{symROR, ACC, 0x6a, 1, 2, 0},
	{symROR, ZPG, 0x66, 2, 5, 0},
	{symROR, ZPX, 0x76, 2, 6, 0},
	{symROR, ABS, 0x6e, 3, 6, 0},
	{symROR, ABX, 0x7e, 3, 7, 0},
}

// An Instruction describes a 6502 CPU instruction, including its name,
// its function implementation, and other metadata.
type Instruction struct {
	Name     string   // string representation of the opcode
	Mode     Mode     // addressing mode
	Opcode   byte     // hexadecimal opcode value
	Length   byte     // number of machine code bytes required, including opcode
	Cycles   byte     // number of CPU cycles to execute the instruction
	BPCycles byte     // additional cycles required by instruction if a boundary page is crossed
	fnCMOS   instfunc // implementing function for CMOS (65C02)
	fnNMOS   instfunc // implementing function for NMOS (6502)
}

// Instructions is an array of all possible 6502 instructions indexed by
// opcode value.
var Instructions [256]Instruction

var variants map[string][]*Instruction

// Build the Instructions table.
func init() {

	// Create a map from symbol to implementation
	symToImpl := make(map[opsym]*opcodeImpl, len(impl))
	for i := range impl {
		symToImpl[impl[i].sym] = &impl[i]
	}

	// Create a map from instruction name string to the list of
	// all instruction variants matching that name
	variants = make(map[string][]*Instruction)

	// Build a full array impl comprehensive instruction data
	for _, d := range data {
		impl := symToImpl[d.sym]
		inst := &Instructions[d.opcode]
		inst.Name = impl.name
		inst.Mode = d.mode
		inst.Opcode = d.opcode
		inst.Length = d.length
		inst.Cycles = d.cycles
		inst.BPCycles = d.bpcycles
		inst.fnCMOS = impl.fnCMOS
		inst.fnNMOS = impl.fnNMOS

		variants[inst.Name] = append(variants[inst.Name], inst)
	}
}

// GetInstructions returns all instructions matching the opcode name.
func GetInstructions(opcode string) []*Instruction {
	return variants[opcode]
}
