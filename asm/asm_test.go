// Copyright 2014-2018 Brett Vickers. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package asm

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func assemble(code string) ([]byte, error) {
	r := bytes.NewReader([]byte(code))
	assembly, _, err := Assemble(r, "test", 0x1000, os.Stdout, 0)
	if err != nil {
		return []byte{}, err
	}
	return assembly.Code, nil
}

func checkASM(t *testing.T, asm string, expected string) {
	code, err := assemble(asm)
	if err != nil {
		t.Error(err)
		return
	}

	b := make([]byte, len(code)*2)
	for i, j := 0, 0; i < len(code); i, j = i+1, j+2 {
		v := code[i]
		b[j+0] = hex[v>>4]
		b[j+1] = hex[v&0x0f]
	}
	s := string(b)

	if s != expected {
		t.Error("code doesn't match expected")
		t.Errorf("got: %s\n", s)
		t.Errorf("exp: %s\n", expected)
	}
}

func checkASMError(t *testing.T, asm string, errString string) {
	_, err := assemble(asm)
	if err == nil {
		t.Errorf("Expected error on %s, didn't get one\n", asm)
		return
	}
	if errString != err.Error() {
		t.Errorf("Expected '%s', got '%v'\n", errString, err)
	}
}

func TestAddressingIMM(t *testing.T) {
	asm := `
	LDA #$20
	LDX #$20
	LDY #$20
	ADC #$20
	SBC #$20
	CMP #$20
	CPX #$20
	CPY #$20
	AND #$20
	ORA #$20
	EOR #$20`

	checkASM(t, asm, "A920A220A0206920E920C920E020C020292009204920")
}

func TestAddressingABS(t *testing.T) {
	asm := `
	LDA $2000
	LDX $2000
	LDY $2000
	STA $2000
	STX $2000
	STY $2000
	ADC $2000
	SBC $2000
	CMP $2000
	CPX $2000
	CPY $2000
	BIT $2000
	AND $2000
	ORA $2000
	EOR $2000
	INC $2000
	DEC $2000
	JMP $2000
	JSR $2000
	ASL $2000
	LSR $2000
	ROL $2000
	ROR $2000
	LDA A:$20
	LDA ABS:$20`

	checkASM(t, asm, "AD0020AE0020AC00208D00208E00208C00206D0020ED0020CD0020"+
		"EC0020CC00202C00202D00200D00204D0020EE0020CE00204C00202000200E0020"+
		"4E00202E00206E0020AD2000AD2000")
}

func TestAddressingABX(t *testing.T) {
	asm := `
	LDA $2000,X
	LDY $2000,X
	STA $2000,X
	ADC $2000,X
	SBC $2000,X
	CMP $2000,X
	AND $2000,X
	ORA $2000,X
	EOR $2000,X
	INC $2000,X
	DEC $2000,X
	ASL $2000,X
	LSR $2000,X
	ROL $2000,X
	ROR $2000,X`

	checkASM(t, asm, "BD0020BC00209D00207D0020FD0020DD00203D00201D00205D0020"+
		"FE0020DE00201E00205E00203E00207E0020")
}

func TestAddressingABY(t *testing.T) {
	asm := `
	LDA $2000,Y
	LDX $2000,Y
	STA $2000,Y
	ADC $2000,Y
	SBC $2000,Y
	CMP $2000,Y
	AND $2000,Y
	ORA $2000,Y
	EOR $2000,Y`

	checkASM(t, asm, "B90020BE0020990020790020F90020D90020390020190020590020")
}

func TestAddressingZPG(t *testing.T) {
	asm := `
	LDA $20
	LDX $20
	LDY $20
	STA $20
	STX $20
	STY $20
	ADC $20
	SBC $20
	CMP $20
	CPX $20
	CPY $20
	BIT $20
	AND $20
	ORA $20
	EOR $20
	INC $20
	DEC $20
	ASL $20
	LSR $20
	ROL $20
	ROR $20`

	checkASM(t, asm, "A520A620A4208520862084206520E520C520E420C42024202520"+
		"05204520E620C6200620462026206620")
}

func TestAddressingIND(t *testing.T) {
	asm := `
	JMP ($20)
	JMP ($2000)`

	checkASM(t, asm, "6C20006C0020")
}

func TestDataBytes(t *testing.T) {
	asm := `
	.DB "AB", $00
	.DB 'f', 'f'
	.DB $ABCD
	.DB $ABCD >> 8
	.DB $0102
	.DB $03040506
	.DB 1+2+3+4
	.DB -1
	.DB -129
	.DB 0b0101010101010101
	.DB 0b01010101`

	checkASM(t, asm, "4142006666CDAB02060AFF7F5555")
}

func TestDataWords(t *testing.T) {
	asm := `
	.DW "AB", $00
	.DW 'f', 'f'
	.DW $ABCD
	.DW $ABCD >> 8
	.DW $0102
	.DW $03040506
	.DW 1+2+3+4
	.DW -1
	.DW -129
	.DW 0b01010101
	.DW 0b0101010101010101`

	checkASM(t, asm, "4142000066006600CDABAB00020106050A00FFFF7FFF55005555")
}

func TestDataDwords(t *testing.T) {
	asm := `
	.DD "AB", $00
	.DD 'f', 'f'
	.DD $ABCD
	.DD $ABCD >> 8
	.DD $0102
	.DD $03040506
	.DD 1+2+3+4
	.DD -1
	.DD -129
	.DD 0b01010101
	.DD 0b0101010101010101`

	checkASM(t, asm, "4142000000006600000066000000CDAB0000AB000000020100000"+
		"60504030A000000FFFFFFFF7FFFFFFF5500000055550000")
}

func TestDataHexStrings(t *testing.T) {
	asm := `
	.DH 0102030405060708
	.DH aabbcc
	.DH dd
	.DH ee
	.DH ff`

	checkASM(t, asm, "0102030405060708AABBCCDDEEFF")
}

func TestDataTermStrings(t *testing.T) {
	asm := `
	.DS "AAA"
	.DS "a", 0
	.DS ""`

	checkASM(t, asm, "4141C1E100")
}

func TestAlign(t *testing.T) {
	asm := `
	.ALIGN 4
	.DB $ff
	.ALIGN 2
	.DB $ff
	.ALIGN 8
	.DB $ff
	.ALIGN 1
	.DB $ff`

	checkASM(t, asm, "FF00FF0000000000FFFF")
}

func TestHereExpression1(t *testing.T) {
	asm := `
	.OR $0600
X	.EQ	FOO
	BIT X
FOO	.EQ $`

	checkASM(t, asm, "2C0306")
}

func TestHereExpression2(t *testing.T) {
	asm := `
	.OR $0600
X	.EQ	$ - 1
	BIT X`

	checkASM(t, asm, "2CFF05")
}

func TestHereExpression3(t *testing.T) {
	asm := `
	.OR $0600
	BIT X
X	.EQ	$ - 1`

	checkASM(t, asm, "2C0206")
}

var asm65c02 = `	PHX
	PHY
	PLX
	PLY
	BRA $1000
	STZ $01
	STZ $1234
	STZ ABS:$01
	STZ $01,X
	STZ $1234,X
	INC
	DEC
	JMP $1234,X
	BIT #$12
	BIT $12,X
	BIT $1234,X
	TRB $01
	TRB $1234
	TSB $01
	TSB $1234
	ADC ($01)
	SBC ($01)
	CMP ($01)
	AND ($01)
	ORA ($01)
	EOR ($01)
	LDA ($01)
	STA ($01)`

func Test65c02(t *testing.T) {
	prefix := `
	.ARCH 65c02
	.ORG $1000
`
	checkASM(t, prefix+asm65c02, "DA5AFA7A80FA64019C34129C010074019E3412"+
		"1A3A7C3412891234123C341214011C341204010C34127201F201D201320112015201B2019201")
}

func Test65c02FailOn6502(t *testing.T) {
	lines := strings.Split(asm65c02, "\n")
	prefix := `
	.ARCH 6502
	.ORG $1000
`
	for _, line := range lines {
		checkASMError(t, prefix+line, "parse error")
	}
}
