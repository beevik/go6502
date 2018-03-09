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
	symBRA
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
	symPHX
	symPHY
	symPLA
	symPLP
	symPLX
	symPLY
	symROL
	symROR
	symRTI
	symRTS
	symSBC
	symSEC
	symSED
	symSEI
	symSTA
	symSTZ
	symSTX
	symSTY
	symTAX
	symTAY
	symTRB
	symTSB
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
	{symBRA, "BRA", (*CPU).bra, nil},
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
	{symPHX, "PHX", (*CPU).phx, nil},
	{symPHY, "PHY", (*CPU).phy, nil},
	{symPLA, "PLA", (*CPU).pla, (*CPU).pla},
	{symPLP, "PLP", (*CPU).plp, (*CPU).plp},
	{symPLX, "PLX", (*CPU).plx, nil},
	{symPLY, "PLY", (*CPU).ply, nil},
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
	{symSTZ, "STZ", (*CPU).stz, nil},
	{symTAX, "TAX", (*CPU).tax, (*CPU).tax},
	{symTAY, "TAY", (*CPU).tay, (*CPU).tay},
	{symTRB, "TRB", (*CPU).trb, nil},
	{symTSB, "TSB", (*CPU).tsb, nil},
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
	cmos     bool  // whether the opcode/mode pair is valid only on 65C02
}

// All valid (opcode, mode) pairs
var data = []opcodeData{
	{symLDA, IMM, 0xa9, 2, 2, 0, false},
	{symLDA, ZPG, 0xa5, 2, 3, 0, false},
	{symLDA, ZPX, 0xb5, 2, 4, 0, false},
	{symLDA, ABS, 0xad, 3, 4, 0, false},
	{symLDA, ABX, 0xbd, 3, 4, 1, false},
	{symLDA, ABY, 0xb9, 3, 4, 1, false},
	{symLDA, IDX, 0xa1, 2, 6, 0, false},
	{symLDA, IDY, 0xb1, 2, 5, 1, false},
	{symLDA, IND, 0xb2, 2, 5, 0, true},

	{symLDX, IMM, 0xa2, 2, 2, 0, false},
	{symLDX, ZPG, 0xa6, 2, 3, 0, false},
	{symLDX, ZPY, 0xb6, 2, 4, 0, false},
	{symLDX, ABS, 0xae, 3, 4, 0, false},
	{symLDX, ABY, 0xbe, 3, 4, 1, false},

	{symLDY, IMM, 0xa0, 2, 2, 0, false},
	{symLDY, ZPG, 0xa4, 2, 3, 0, false},
	{symLDY, ZPX, 0xb4, 2, 4, 0, false},
	{symLDY, ABS, 0xac, 3, 4, 0, false},
	{symLDY, ABX, 0xbc, 3, 4, 1, false},

	{symSTA, ZPG, 0x85, 2, 3, 0, false},
	{symSTA, ZPX, 0x95, 2, 4, 0, false},
	{symSTA, ABS, 0x8d, 3, 4, 0, false},
	{symSTA, ABX, 0x9d, 3, 5, 0, false},
	{symSTA, ABY, 0x99, 3, 5, 0, false},
	{symSTA, IDX, 0x81, 2, 6, 0, false},
	{symSTA, IDY, 0x91, 2, 6, 0, false},
	{symSTA, IND, 0x92, 2, 5, 0, true},

	{symSTX, ZPG, 0x86, 2, 3, 0, false},
	{symSTX, ZPY, 0x97, 2, 4, 0, false},
	{symSTX, ABS, 0x8e, 3, 4, 0, false},

	{symSTY, ZPG, 0x84, 2, 3, 0, false},
	{symSTY, ZPX, 0x94, 2, 4, 0, false},
	{symSTY, ABS, 0x8c, 3, 4, 0, false},

	{symSTZ, ZPG, 0x64, 2, 3, 0, true},
	{symSTZ, ZPX, 0x74, 2, 4, 0, true},
	{symSTZ, ABS, 0x9c, 3, 4, 0, true},
	{symSTZ, ABX, 0x9e, 3, 5, 0, true},

	{symADC, IMM, 0x69, 2, 2, 0, false},
	{symADC, ZPG, 0x65, 2, 3, 0, false},
	{symADC, ZPX, 0x75, 2, 4, 0, false},
	{symADC, ABS, 0x6d, 3, 4, 0, false},
	{symADC, ABX, 0x7d, 3, 4, 1, false},
	{symADC, ABY, 0x79, 3, 4, 1, false},
	{symADC, IDX, 0x61, 2, 6, 0, false},
	{symADC, IDY, 0x71, 2, 5, 1, false},
	{symADC, IND, 0x72, 2, 5, 1, true},

	{symSBC, IMM, 0xe9, 2, 2, 0, false},
	{symSBC, ZPG, 0xe5, 2, 3, 0, false},
	{symSBC, ZPX, 0xf5, 2, 4, 0, false},
	{symSBC, ABS, 0xed, 3, 4, 0, false},
	{symSBC, ABX, 0xfd, 3, 4, 1, false},
	{symSBC, ABY, 0xf9, 3, 4, 1, false},
	{symSBC, IDX, 0xe1, 2, 6, 0, false},
	{symSBC, IDY, 0xf1, 2, 5, 1, false},
	{symSBC, IND, 0xf2, 2, 5, 1, true},

	{symCMP, IMM, 0xc9, 2, 2, 0, false},
	{symCMP, ZPG, 0xc5, 2, 3, 0, false},
	{symCMP, ZPX, 0xd5, 2, 4, 0, false},
	{symCMP, ABS, 0xcd, 3, 4, 0, false},
	{symCMP, ABX, 0xdd, 3, 4, 1, false},
	{symCMP, ABY, 0xd9, 3, 4, 1, false},
	{symCMP, IDX, 0xc1, 2, 6, 0, false},
	{symCMP, IDY, 0xd1, 2, 5, 1, false},
	{symCMP, IND, 0xd2, 2, 5, 0, true},

	{symCPX, IMM, 0xe0, 2, 2, 0, false},
	{symCPX, ZPG, 0xe4, 2, 3, 0, false},
	{symCPX, ABS, 0xec, 3, 4, 0, false},

	{symCPY, IMM, 0xc0, 2, 2, 0, false},
	{symCPY, ZPG, 0xc4, 2, 3, 0, false},
	{symCPY, ABS, 0xcc, 3, 4, 0, false},

	{symBIT, IMM, 0x89, 2, 2, 0, true},
	{symBIT, ZPG, 0x24, 2, 3, 0, false},
	{symBIT, ZPX, 0x34, 2, 4, 0, true},
	{symBIT, ABS, 0x2c, 3, 4, 0, false},
	{symBIT, ABX, 0x3c, 3, 4, 1, true},

	{symCLC, IMP, 0x18, 1, 2, 0, false},
	{symSEC, IMP, 0x38, 1, 2, 0, false},
	{symCLI, IMP, 0x58, 1, 2, 0, false},
	{symSEI, IMP, 0x78, 1, 2, 0, false},
	{symCLD, IMP, 0xd8, 1, 2, 0, false},
	{symSED, IMP, 0xf8, 1, 2, 0, false},
	{symCLV, IMP, 0xb8, 1, 2, 0, false},

	{symBCC, REL, 0x90, 2, 2, 1, false},
	{symBCS, REL, 0xb0, 2, 2, 1, false},
	{symBEQ, REL, 0xf0, 2, 2, 1, false},
	{symBNE, REL, 0xd0, 2, 2, 1, false},
	{symBMI, REL, 0x30, 2, 2, 1, false},
	{symBPL, REL, 0x10, 2, 2, 1, false},
	{symBVC, REL, 0x50, 2, 2, 1, false},
	{symBVS, REL, 0x70, 2, 2, 1, false},
	{symBRA, REL, 0x80, 2, 2, 1, true},

	{symBRK, IMP, 0x00, 1, 7, 0, false},

	{symAND, IMM, 0x29, 2, 2, 0, false},
	{symAND, ZPG, 0x25, 2, 3, 0, false},
	{symAND, ZPX, 0x35, 2, 4, 0, false},
	{symAND, ABS, 0x2d, 3, 4, 0, false},
	{symAND, ABX, 0x3d, 3, 4, 1, false},
	{symAND, ABY, 0x39, 3, 4, 1, false},
	{symAND, IDX, 0x21, 2, 6, 0, false},
	{symAND, IDY, 0x31, 2, 5, 1, false},
	{symAND, IND, 0x32, 2, 5, 0, true},

	{symORA, IMM, 0x09, 2, 2, 0, false},
	{symORA, ZPG, 0x05, 2, 3, 0, false},
	{symORA, ZPX, 0x15, 2, 4, 0, false},
	{symORA, ABS, 0x0d, 3, 4, 0, false},
	{symORA, ABX, 0x1d, 3, 4, 1, false},
	{symORA, ABY, 0x19, 3, 4, 1, false},
	{symORA, IDX, 0x01, 2, 6, 0, false},
	{symORA, IDY, 0x11, 2, 5, 1, false},
	{symORA, IND, 0x12, 2, 5, 0, true},

	{symEOR, IMM, 0x49, 2, 2, 0, false},
	{symEOR, ZPG, 0x45, 2, 3, 0, false},
	{symEOR, ZPX, 0x55, 2, 4, 0, false},
	{symEOR, ABS, 0x4d, 3, 4, 0, false},
	{symEOR, ABX, 0x5d, 3, 4, 1, false},
	{symEOR, ABY, 0x59, 3, 4, 1, false},
	{symEOR, IDX, 0x41, 2, 6, 0, false},
	{symEOR, IDY, 0x51, 2, 5, 1, false},
	{symEOR, IND, 0x52, 2, 5, 0, true},

	{symINC, ZPG, 0xe6, 2, 5, 0, false},
	{symINC, ZPX, 0xf6, 2, 6, 0, false},
	{symINC, ABS, 0xee, 3, 6, 0, false},
	{symINC, ABX, 0xfe, 3, 7, 0, false},
	{symINC, ACC, 0x1a, 1, 2, 0, true},

	{symDEC, ZPG, 0xc6, 2, 5, 0, false},
	{symDEC, ZPX, 0xd6, 2, 6, 0, false},
	{symDEC, ABS, 0xce, 3, 6, 0, false},
	{symDEC, ABX, 0xde, 3, 7, 0, false},
	{symDEC, ACC, 0x3a, 1, 2, 0, true},

	{symINX, IMP, 0xe8, 1, 2, 0, false},
	{symINY, IMP, 0xc8, 1, 2, 0, false},

	{symDEX, IMP, 0xca, 1, 2, 0, false},
	{symDEY, IMP, 0x88, 1, 2, 0, false},

	{symJMP, ABS, 0x4c, 3, 3, 0, false},
	{symJMP, ABX, 0x7c, 3, 6, 0, true},
	{symJMP, IND, 0x6c, 3, 5, 0, false},

	{symJSR, ABS, 0x20, 3, 6, 0, false},
	{symRTS, IMP, 0x60, 1, 6, 0, false},

	{symRTI, IMP, 0x40, 1, 6, 0, false},

	{symNOP, IMP, 0xea, 1, 2, 0, false},

	{symTAX, IMP, 0xaa, 1, 2, 0, false},
	{symTXA, IMP, 0x8a, 1, 2, 0, false},
	{symTAY, IMP, 0xa8, 1, 2, 0, false},
	{symTYA, IMP, 0x98, 1, 2, 0, false},
	{symTXS, IMP, 0x9a, 1, 2, 0, false},
	{symTSX, IMP, 0xba, 1, 2, 0, false},

	{symTRB, ZPG, 0x14, 2, 5, 0, true},
	{symTRB, ABS, 0x1c, 3, 6, 0, true},
	{symTSB, ZPG, 0x04, 2, 5, 0, true},
	{symTSB, ABS, 0x0c, 3, 6, 0, true},

	{symPHA, IMP, 0x48, 1, 3, 0, false},
	{symPLA, IMP, 0x68, 1, 4, 0, false},
	{symPHP, IMP, 0x08, 1, 3, 0, false},
	{symPLP, IMP, 0x28, 1, 4, 0, false},
	{symPHX, IMP, 0xda, 1, 3, 0, true},
	{symPLX, IMP, 0xfa, 1, 4, 0, true},
	{symPHY, IMP, 0x5a, 1, 3, 0, true},
	{symPLY, IMP, 0x7a, 1, 4, 0, true},

	{symASL, ACC, 0x0a, 1, 2, 0, false},
	{symASL, ZPG, 0x06, 2, 5, 0, false},
	{symASL, ZPX, 0x16, 2, 6, 0, false},
	{symASL, ABS, 0x0e, 3, 6, 0, false},
	{symASL, ABX, 0x1e, 3, 7, 0, false},

	{symLSR, ACC, 0x4a, 1, 2, 0, false},
	{symLSR, ZPG, 0x46, 2, 5, 0, false},
	{symLSR, ZPX, 0x56, 2, 6, 0, false},
	{symLSR, ABS, 0x4e, 3, 6, 0, false},
	{symLSR, ABX, 0x5e, 3, 7, 0, false},

	{symROL, ACC, 0x2a, 1, 2, 0, false},
	{symROL, ZPG, 0x26, 2, 5, 0, false},
	{symROL, ZPX, 0x36, 2, 6, 0, false},
	{symROL, ABS, 0x2e, 3, 6, 0, false},
	{symROL, ABX, 0x3e, 3, 7, 0, false},

	{symROR, ACC, 0x6a, 1, 2, 0, false},
	{symROR, ZPG, 0x66, 2, 5, 0, false},
	{symROR, ZPX, 0x76, 2, 6, 0, false},
	{symROR, ABS, 0x6e, 3, 6, 0, false},
	{symROR, ABX, 0x7e, 3, 7, 0, false},
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
