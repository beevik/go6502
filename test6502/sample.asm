		.ORG		$0600

; -------
; Exports
; -------

		.EX		START
		.EX		DATA
		.EX		END


; ---------
; Constants
; ---------

STORE		.EQ		$0200

; -------
; Program
; -------

START:				; Labels can end with ':'
		JSR LDA_TEST
		JSR LDX_TEST
		JSR LDY_TEST
		BEQ .1		; Branch to local label ('.' prefix)
		LDA /DATA	; Upper byte of DATA
                LDX #DATA	; Lower byte of DATA
.1		BRK

LDA_TEST	LDA #$20	; Immediate
		LDA $20		; Zero page
		LDA $20,X	; Zero page + X
		LDA ($20,X)	; Indirect + X
		LDA ($20),Y	; Indirect + Y
		LDA $0200	; Absolute
		LDA ABS:$20	; Absolute (forced)
		LDA $0200,X	; Absolute + X
		LDA $0200,Y	; Absolute + Y
.1		RTS

LDX_TEST	LDX #$20	; Immediate
		LDX $20		; Zero page
		LDX $20,Y	; Zero page + Y
		LDX $0200	; Absolute
		LDX ABS:$20	; Absolute (forced)
		LDX $0200,Y	; Absolute + Y
.1		RTS

START.1:			; Shouldn't conflict with .1 under START

LDY_TEST	LDY #$20	; Immediate
		LDY $20		; Zero page
		LDY $20,X	; Zero page + X
		LDY $0200	; Absolute
		LDY ABS:$20	; Absolute (forced)
		LDY $0200,X	; Absolute + X
.1		RTS

; ----
; Data
; ----

DATA:
		; .DB data can include literals (string, character and
		; numeric) and math expressions using labels and macros. For
		; numeric values, only the least significant byte is stored.

.BYTES		.DB		"AB", $00		; $41, $42, $00
		.DB		$0102, 0x03040506	; $02, $06
		.DB		1+2+3+4, 5+6+7+8	; $0A, $1A
		.DB		LDA_TEST, LDA_TEST>>8	; addr of LDA_TEST
		.DB		'<, '<'			; $3C, $3C
		.DB 		-$01, -$0001		; $FF, $FF
		.DB		$ABCD >> 8		; $AB
		.DB		-1, -129		; $FF, $7F
		.DB		0b01010101, -0b01010101 ; $55, $AB
		.DB 		$ - .BYTES		; $ = curr line addr

.WORDS		; .DW data works like .DB, except all numeric values are
		; stored as 2-byte words. String characters are still stored
		; with only one byte each.

		.DW		"AB"			; $41, $42
		.DW		'F'			; $46, $00
		.DW		$01			; $01, $00
		.DW		$ABCD			; $CD, $AB
		.DW		$ABCD >> 8		; $AB, $00
		.DW		LDA_TEST		; addr of LDA_TEST
		.DW		$12345678		; $78, $56
		.DW		0b11110101		; $F5, $00
		.DW		-1			; $FF, $FF
		.DW		$ - .WORDS		; $ = curr line addr

.DWORDS		; .DD data works like .DB and .DW, except all numeric values
		; are stored as 4-byte double-words. String characters are
		; still stored with only one byte each.

		.DD		"AB"			; $41, $42
		.DD		'F'			; $46, $00, $00, $00
		.DD		-1			; $FF, $FF, $FF, $FF
		.DD		$12345678		; $78, $56, $34, $12
		.DD		$ - .DWORDS		; $ = curr line addr

END
