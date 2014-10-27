package go6502

// An opsym is an internal symbol used to associate opcode data
// with its implementation.
type opsym byte

const (
	sym_ADC opsym = iota
	sym_AND
	sym_ASL
	sym_BCC
	sym_BCS
	sym_BEQ
	sym_BIT
	sym_BMI
	sym_BNE
	sym_BPL
	sym_BRK
	sym_BVC
	sym_BVS
	sym_CLC
	sym_CLD
	sym_CLI
	sym_CLV
	sym_CMP
	sym_CPX
	sym_CPY
	sym_DEC
	sym_DEX
	sym_DEY
	sym_EOR
	sym_INC
	sym_INX
	sym_INY
	sym_JMP
	sym_JSR
	sym_LDA
	sym_LDX
	sym_LDY
	sym_LSR
	sym_NOP
	sym_ORA
	sym_PHA
	sym_PHP
	sym_PLA
	sym_PLP
	sym_ROL
	sym_ROR
	sym_RTI
	sym_RTS
	sym_SBC
	sym_SEC
	sym_SED
	sym_SEI
	sym_STA
	sym_STX
	sym_STY
	sym_TAX
	sym_TAY
	sym_TSX
	sym_TXA
	sym_TXS
	sym_TYA
)

// Opcode name and function implementation
type opcodeImpl struct {
	sym  opsym
	name string
	fn   func(c *Cpu, inst *Instruction, operand []byte)
}

var impl = []opcodeImpl{
	{sym_ADC, "ADC", (*Cpu).opADC},
	{sym_AND, "AND", (*Cpu).opAND},
	{sym_ASL, "ASL", (*Cpu).opASL},
	{sym_BCC, "BCC", (*Cpu).opBCC},
	{sym_BCS, "BCS", (*Cpu).opBCS},
	{sym_BEQ, "BEQ", (*Cpu).opBEQ},
	{sym_BIT, "BIT", (*Cpu).opBIT},
	{sym_BMI, "BMI", (*Cpu).opBMI},
	{sym_BNE, "BNE", (*Cpu).opBNE},
	{sym_BPL, "BPL", (*Cpu).opBPL},
	{sym_BRK, "BRK", (*Cpu).opBRK},
	{sym_BVC, "BVC", (*Cpu).opBVC},
	{sym_BVS, "BVS", (*Cpu).opBVS},
	{sym_CLC, "CLC", (*Cpu).opCLC},
	{sym_CLD, "CLD", (*Cpu).opCLD},
	{sym_CLI, "CLI", (*Cpu).opCLI},
	{sym_CLV, "CLV", (*Cpu).opCLV},
	{sym_CMP, "CMP", (*Cpu).opCMP},
	{sym_CPX, "CPX", (*Cpu).opCPX},
	{sym_CPY, "CPY", (*Cpu).opCPY},
	{sym_DEC, "DEC", (*Cpu).opDEC},
	{sym_DEX, "DEX", (*Cpu).opDEX},
	{sym_DEY, "DEY", (*Cpu).opDEY},
	{sym_EOR, "EOR", (*Cpu).opEOR},
	{sym_INC, "INC", (*Cpu).opINC},
	{sym_INX, "INX", (*Cpu).opINX},
	{sym_INY, "INY", (*Cpu).opINY},
	{sym_JMP, "JMP", (*Cpu).opJMP},
	{sym_JSR, "JSR", (*Cpu).opJSR},
	{sym_LDA, "LDA", (*Cpu).opLDA},
	{sym_LDX, "LDX", (*Cpu).opLDX},
	{sym_LDY, "LDY", (*Cpu).opLDY},
	{sym_LSR, "LSR", (*Cpu).opLSR},
	{sym_NOP, "NOP", (*Cpu).opNOP},
	{sym_ORA, "ORA", (*Cpu).opORA},
	{sym_PHA, "PHA", (*Cpu).opPHA},
	{sym_PHP, "PHP", (*Cpu).opPHP},
	{sym_PLA, "PLA", (*Cpu).opPLA},
	{sym_PLP, "PLP", (*Cpu).opPLP},
	{sym_ROL, "ROL", (*Cpu).opROL},
	{sym_ROR, "ROR", (*Cpu).opROR},
	{sym_RTI, "RTI", (*Cpu).opRTI},
	{sym_RTS, "RTS", (*Cpu).opRTS},
	{sym_SBC, "SBC", (*Cpu).opSBC},
	{sym_SEC, "SEC", (*Cpu).opSEC},
	{sym_SED, "SED", (*Cpu).opSED},
	{sym_SEI, "SEI", (*Cpu).opSEI},
	{sym_STA, "STA", (*Cpu).opSTA},
	{sym_STX, "STX", (*Cpu).opSTX},
	{sym_STY, "STY", (*Cpu).opSTY},
	{sym_TAX, "TAX", (*Cpu).opTAX},
	{sym_TAY, "TAY", (*Cpu).opTAY},
	{sym_TSX, "TSX", (*Cpu).opTSX},
	{sym_TXA, "TXA", (*Cpu).opTXA},
	{sym_TXS, "TXS", (*Cpu).opTXS},
	{sym_TYA, "TYA", (*Cpu).opTYA},
}

// Memory addressing mode.
type Mode byte

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
	{sym_LDA, IMM, 0xa9, 2, 2, 0},
	{sym_LDA, ZPG, 0xa5, 2, 3, 0},
	{sym_LDA, ZPX, 0xb5, 2, 4, 0},
	{sym_LDA, ABS, 0xad, 3, 4, 0},
	{sym_LDA, ABX, 0xbd, 3, 4, 1},
	{sym_LDA, ABY, 0xb9, 3, 4, 1},
	{sym_LDA, IDX, 0xa1, 2, 6, 0},
	{sym_LDA, IDY, 0xb1, 2, 5, 1},

	{sym_LDX, IMM, 0xa2, 2, 2, 0},
	{sym_LDX, ZPG, 0xa6, 2, 3, 0},
	{sym_LDX, ZPY, 0xb6, 2, 4, 0},
	{sym_LDX, ABS, 0xae, 3, 4, 0},
	{sym_LDX, ABY, 0xbe, 3, 4, 1},

	{sym_LDY, IMM, 0xa0, 2, 2, 0},
	{sym_LDY, ZPG, 0xa4, 2, 3, 0},
	{sym_LDY, ZPX, 0xb4, 2, 4, 0},
	{sym_LDY, ABS, 0xac, 3, 4, 0},
	{sym_LDY, ABX, 0xbc, 3, 4, 1},

	{sym_STA, ZPG, 0x85, 2, 3, 0},
	{sym_STA, ZPX, 0x95, 2, 4, 0},
	{sym_STA, ABS, 0x8d, 3, 4, 0},
	{sym_STA, ABX, 0x9d, 3, 5, 0},
	{sym_STA, ABY, 0x99, 3, 5, 0},
	{sym_STA, IDX, 0x81, 2, 6, 0},
	{sym_STA, IDY, 0x91, 2, 6, 0},

	{sym_STX, ZPG, 0x86, 2, 3, 0},
	{sym_STX, ZPY, 0x97, 2, 4, 0},
	{sym_STX, ABS, 0x8e, 3, 4, 0},

	{sym_STY, ZPG, 0x84, 2, 3, 0},
	{sym_STY, ZPX, 0x94, 2, 4, 0},
	{sym_STY, ABS, 0x8c, 3, 4, 0},

	{sym_ADC, IMM, 0x69, 2, 2, 0},
	{sym_ADC, ZPG, 0x65, 2, 3, 0},
	{sym_ADC, ZPX, 0x75, 2, 4, 0},
	{sym_ADC, ABS, 0x6d, 3, 4, 0},
	{sym_ADC, ABX, 0x7d, 3, 4, 1},
	{sym_ADC, ABY, 0x79, 3, 4, 1},
	{sym_ADC, IDX, 0x61, 2, 6, 0},
	{sym_ADC, IDY, 0x71, 2, 5, 1},

	{sym_SBC, IMM, 0xe9, 2, 2, 0},
	{sym_SBC, ZPG, 0xe5, 2, 3, 0},
	{sym_SBC, ZPX, 0xf5, 2, 4, 0},
	{sym_SBC, ABS, 0xed, 3, 4, 0},
	{sym_SBC, ABX, 0xfd, 3, 4, 1},
	{sym_SBC, ABY, 0xf9, 3, 4, 1},
	{sym_SBC, IDX, 0xe1, 2, 6, 0},
	{sym_SBC, IDY, 0xf1, 2, 5, 1},

	{sym_CMP, IMM, 0xc9, 2, 2, 0},
	{sym_CMP, ZPG, 0xc5, 2, 3, 0},
	{sym_CMP, ZPX, 0xd5, 2, 4, 0},
	{sym_CMP, ABS, 0xcd, 3, 4, 0},
	{sym_CMP, ABX, 0xdd, 3, 4, 1},
	{sym_CMP, ABY, 0xd9, 3, 4, 1},
	{sym_CMP, IDX, 0xc1, 2, 6, 0},
	{sym_CMP, IDY, 0xd1, 2, 5, 1},

	{sym_CPX, IMM, 0xe0, 2, 2, 0},
	{sym_CPX, ZPG, 0xe4, 2, 3, 0},
	{sym_CPX, ABS, 0xec, 3, 4, 0},

	{sym_CPY, IMM, 0xc0, 2, 2, 0},
	{sym_CPY, ZPG, 0xc4, 2, 3, 0},
	{sym_CPY, ABS, 0xcc, 3, 4, 0},

	{sym_BIT, ZPG, 0x24, 2, 3, 0},
	{sym_BIT, ABS, 0x2c, 3, 4, 0},

	{sym_CLC, IMP, 0x18, 1, 2, 0},
	{sym_SEC, IMP, 0x38, 1, 2, 0},
	{sym_CLI, IMP, 0x58, 1, 2, 0},
	{sym_SEI, IMP, 0x78, 1, 2, 0},
	{sym_CLD, IMP, 0xd8, 1, 2, 0},
	{sym_SED, IMP, 0xf8, 1, 2, 0},
	{sym_CLV, IMP, 0xb8, 1, 2, 0},

	{sym_BCC, REL, 0x90, 2, 2, 1},
	{sym_BCS, REL, 0xb0, 2, 2, 1},
	{sym_BEQ, REL, 0xf0, 2, 2, 1},
	{sym_BNE, REL, 0xd0, 2, 2, 1},
	{sym_BMI, REL, 0x30, 2, 2, 1},
	{sym_BPL, REL, 0x10, 2, 2, 1},
	{sym_BVC, REL, 0x50, 2, 2, 1},
	{sym_BVS, REL, 0x70, 2, 2, 1},

	{sym_BRK, IMP, 0x00, 1, 7, 0},

	{sym_AND, IMM, 0x29, 2, 2, 0},
	{sym_AND, ZPG, 0x25, 2, 3, 0},
	{sym_AND, ZPX, 0x35, 2, 4, 0},
	{sym_AND, ABS, 0x2d, 3, 4, 0},
	{sym_AND, ABX, 0x3d, 3, 4, 1},
	{sym_AND, ABY, 0x39, 3, 4, 1},
	{sym_AND, IDX, 0x21, 2, 6, 0},
	{sym_AND, IDY, 0x31, 2, 5, 1},

	{sym_ORA, IMM, 0x09, 2, 2, 0},
	{sym_ORA, ZPG, 0x05, 2, 3, 0},
	{sym_ORA, ZPX, 0x15, 2, 4, 0},
	{sym_ORA, ABS, 0x0d, 3, 4, 0},
	{sym_ORA, ABX, 0x1d, 3, 4, 1},
	{sym_ORA, ABY, 0x19, 3, 4, 1},
	{sym_ORA, IDX, 0x01, 2, 6, 0},
	{sym_ORA, IDY, 0x11, 2, 5, 1},

	{sym_EOR, IMM, 0x49, 2, 2, 0},
	{sym_EOR, ZPG, 0x45, 2, 3, 0},
	{sym_EOR, ZPX, 0x55, 2, 4, 0},
	{sym_EOR, ABS, 0x4d, 3, 4, 0},
	{sym_EOR, ABX, 0x5d, 3, 4, 1},
	{sym_EOR, ABY, 0x59, 3, 4, 1},
	{sym_EOR, IDX, 0x41, 2, 6, 0},
	{sym_EOR, IDY, 0x51, 2, 5, 1},

	{sym_INC, ZPG, 0xe6, 2, 5, 0},
	{sym_INC, ZPX, 0xf6, 2, 6, 0},
	{sym_INC, ABS, 0xee, 3, 6, 0},
	{sym_INC, ABX, 0xfe, 3, 7, 0},

	{sym_DEC, ZPG, 0xc6, 2, 5, 0},
	{sym_DEC, ZPX, 0xd6, 2, 6, 0},
	{sym_DEC, ABS, 0xce, 3, 6, 0},
	{sym_DEC, ABX, 0xde, 3, 7, 0},

	{sym_INX, IMP, 0xe8, 1, 2, 0},
	{sym_INY, IMP, 0xc8, 1, 2, 0},

	{sym_DEX, IMP, 0xca, 1, 2, 0},
	{sym_DEY, IMP, 0x88, 1, 2, 0},

	{sym_JMP, ABS, 0x4c, 3, 3, 0},
	{sym_JMP, IND, 0x6c, 3, 5, 0},

	{sym_JSR, ABS, 0x20, 3, 6, 0},
	{sym_RTS, IMP, 0x60, 1, 6, 0},

	{sym_RTI, IMP, 0x40, 1, 6, 0},

	{sym_NOP, IMM, 0xea, 1, 2, 0},

	{sym_TAX, IMP, 0xaa, 1, 2, 0},
	{sym_TXA, IMP, 0x8a, 1, 2, 0},
	{sym_TAY, IMP, 0xa8, 1, 2, 0},
	{sym_TYA, IMP, 0x98, 1, 2, 0},
	{sym_TXS, IMP, 0x9a, 1, 2, 0},
	{sym_TSX, IMP, 0xba, 1, 2, 0},

	{sym_PHA, IMP, 0x48, 1, 3, 0},
	{sym_PLA, IMP, 0x68, 1, 4, 0},
	{sym_PHP, IMP, 0x08, 1, 3, 0},
	{sym_PLP, IMP, 0x28, 1, 4, 0},

	{sym_ASL, ACC, 0x0a, 1, 2, 0},
	{sym_ASL, ZPG, 0x06, 2, 5, 0},
	{sym_ASL, ZPX, 0x16, 2, 6, 0},
	{sym_ASL, ABS, 0x0e, 3, 6, 0},
	{sym_ASL, ABX, 0x1e, 3, 7, 0},

	{sym_LSR, ACC, 0x4a, 1, 2, 0},
	{sym_LSR, ZPG, 0x46, 2, 5, 0},
	{sym_LSR, ZPX, 0x56, 2, 6, 0},
	{sym_LSR, ABS, 0x4e, 3, 6, 0},
	{sym_LSR, ABX, 0x5e, 3, 7, 0},

	{sym_ROL, ACC, 0x2a, 1, 2, 0},
	{sym_ROL, ZPG, 0x26, 2, 5, 0},
	{sym_ROL, ZPX, 0x36, 2, 6, 0},
	{sym_ROL, ABS, 0x2e, 3, 6, 0},
	{sym_ROL, ABX, 0x3e, 3, 7, 0},

	{sym_ROR, ACC, 0x6a, 1, 2, 0},
	{sym_ROR, ZPG, 0x66, 2, 5, 0},
	{sym_ROR, ZPX, 0x76, 2, 6, 0},
	{sym_ROR, ABS, 0x6e, 3, 6, 0},
	{sym_ROR, ABX, 0x7e, 3, 7, 0},
}

// A single instruction composed by joining the data in
// data and impl.
type Instruction struct {
	Name     string // string representation of the opcode
	Mode     Mode   // addressing mode
	Opcode   byte   // hexadecimal opcode value
	Length   byte   // number of machine code bytes required, including opcode
	Cycles   byte   // number of CPU cycles to execute the instruction
	BPCycles byte   // additional cycles required by instruction if a boundary page is crossed
	fn       func(c *Cpu, inst *Instruction, operand []byte)
}

// All 6502 instructions indexed by opcode value.
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
		inst.fn = impl.fn

		variants[inst.Name] = append(variants[inst.Name], inst)
	}
}

// Return all instructions matching the opcode name.
func GetInstructions(opcode string) []*Instruction {
	return variants[opcode]
}
