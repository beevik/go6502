; Sample program illustrating some of the features of the go6502 macro
; assembler.

		.ARCH		65c02	; select CMOS 65c02 chip
		.ORG		$1000	; origin address for machine code

; -------
; Exports
; -------

		; Exported labels are reported by the assembler with
		; their assigned addresses.

		.EX		START
		.EX		DATA
		.EX		END
		.EX 		LDA_TEST
		.EX		LDX_TEST
		.EX		LDY_TEST
		.EX		JSR_TEST

; ---------
; Constants
; ---------
		; A constant may be a literal (numeric or character). Or
		; it may be an expression including literals, address labels,
		; and other constants. Such constants may appear in
		; expressions anywhere in the source code.

STORE		.EQ		$0200	; Constant defined with .EQ
X		EQU		$EE	; Alternative: defined with EQU
Y		=		$FE	; Alternative: defined with =


; -------
; Program
; -------

START:				; Labels may end in ':', which is ignored.
		LDA #X
		LDA #Y
		LDA #128
		LDA #$7F
		LDA #%01011010
		JSR JSR_TEST
		JSR LDA_TEST
		JSR LDX_TEST
		JSR LDY_TEST
		BEQ @1		; Branch to a local label ('@' or '.' prefix)
		LDY #';'	; Immediate character ASCII value
		LDX #DATA	; Lower byte of DATA
		LDX #<DATA	; Also: Lower byte of DATA
		LDA #>DATA	; Upper byte of DATA
@1		BRK		; @1 label is valid only within START scope.

JSR_TEST	LDA #$FF
		RTS

LDA_TEST	LDA #$20	; Immediate
		LDA $20		; Zero page
		LDA $20,X	; Zero page + X
		LDA ($20,X)	; Indirect + X
		LDA ($20),Y	; Indirect + Y
		LDA $0200	; Absolute
		LDA ABS:$20	; Absolute (forced)
		LDA $0200,X	; Absolute + X
		LDA $0200,Y	; Absolute + Y
		STA $0300
		RTS

LDX_TEST	LDX #$20	; Immediate
		LDX $20		; Zero page
		LDX $20,Y	; Zero page + Y
		LDX $0200	; Absolute
		LDX ABS:$20	; Absolute (forced)
		LDX $0200,Y	; Absolute + Y
		RTS

LDY_TEST	LDY #$20	; Immediate
		LDY $20		; Zero page
		LDY $20,X	; Zero page + X
		LDY $0200	; Absolute
		LDY ABS:$20	; Absolute (forced)
		LDY $0200,X	; Absolute + X
		RTS

; ----
; Data
; ----

DATA:

		.ALIGN 		16	; align next addr on 16-byte boundary

.BYTES		; .DB data can include literals (string, character and
		; numeric) and math expressions using labels and constants.
		; For numeric values, only the least significant byte is
		; stored.

		.DB		"AB,", $00		; 41 42 2C 00
		.DB		'F', ','		; 46 2C
		.DB		1			; 01
		.DB		$ABCD			; CD
		.DB		$ABCD >> 8		; AB
		.DB		$0102			; 02
		.DB		0x03040506		; 06
		.DB		1+2+3+4, 5+6+7+8	; 0A 1A
		.DB		LDA_TEST, LDA_TEST>>8	; addr of LDA_TEST
		.DB		-1, -129		; FF 7F
		.DB		$12345678		; 78
		.DB		0b01010101		; 55
		.DB 		$ - .BYTES		; 12

		.ALIGN		2

.WORDS		; .DW data works like .DB, except all numeric values are
		; stored as 2-byte words. String literals are still stored
		; with one byte per character.

		.DW		"AB", $00		; 41 42 00 00
		.DW		'F', 'F'		; 46 00 46 00
		.DW		1			; 01 00
		.DW		$ABCD			; CD AB
		.DW		$ABCD >> 8		; AB 00
		.DW		$0102			; 02 01
		.DW		0x03040506		; 06 05
		.DW		1+2+3+4, 5+6+7+8	; 0A 00 1A 00
		.DW		LDA_TEST		; 2-byte addr of LDA_TEST
		.DW		-1, -129		; FF FF 7F FF
		.DW		$12345678		; 78 56
		.DW		0b11110101		; F5 00
		.DW		$ - .WORDS		; 20 00

		.ALIGN		4

.DWORDS		; .DD data works like .DB and .DW, except all numeric values
		; are stored as 4-byte double-words. String literals are still
		; stored with one byte per character.

		.DD		"AB", $00		; 41 42 00 00 00 00
		.DD		'F', 'F'		; 46 00 00 00 46 00 00 00
		.DD		1			; 01 00 00 00
		.DD		$ABCD			; CD AB 00 00
		.DD		$ABCD >> 8		; AB 00 00 00
		.DD		$0102			; 02 01 00 00
		.DD		0x03040506		; 06 05 04 03
		.DD		1+2+3+4, 5+6+7+8	; 0A 00 00 00 1A 00 00 00
		.DD		LDA_TEST		; 4-byte addr of LDA_TEST
		.DD		-1, -129		; FF FF FF FF 7F FF FF FF
		.DD		$12345678		; 78 56 34 12
		.DD		0b11110101		; F5 00 00 00
		.DD		$ - .DWORDS		; 3E 00 00 00

		.ALIGN		4

.HEXSTRINGS	; .DH data is expressed as a chain of hexadecimal values,
		; which are stored directly into the assembled data.

		.DH		414200			; 41 42 00
		.DH		4646			; 46 46
		.DH		01			; 01
		.DH		12345678		; 12 34 56 78
		.DH		0123456789abcdef	; 01 23 45 67 89 AB CD EF
		.DB		$ - .HEXSTRINGS		; 12

		.ALIGN		4

.TSTRINGS	; .DS data works the same way as .DB, except the last byte
		; in a string literal has its most significant bit set.

		.DS		"AAA"			; 41 41 C1
		.DS		"A", 0			; C1 00
		.DB		$ - .TSTRINGS		; 05

		.ALIGN		4

		; Pad the file to a length of 256 bytes. Use FF for padding.
.PADDING	.PAD		$FF, 256-($-START)

END
