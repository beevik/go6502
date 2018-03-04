		.ORG	$0600

; -------
; Exports
; -------

		.EX	START
		.EX	LDA_TEST
		.EX	LDX_TEST
		.EX	END
		.EX	DATA
		.EX	DATA_END


; ---------
; Constants
; ---------

STORE		.EQ	$0200

; -------
; Program
; -------

START:
		JSR LDA_TEST
		JSR LDX_TEST
		JSR LDY_TEST
		BEQ .1
		LDX /('$'+LDY_TEST)
.1		BRK

START.1:

LDA_TEST	LDA #$20	; Immediate
		LDA $20		; Zero page
		LDA $20,X	; Zero page + X
		LDA ($20,X)	; Indirect + X
		LDA ($20),Y	; Indirect + Y
		LDA ABS:$20	; Absolute
		LDA $0200	; Absolute
		LDA $0200,X	; Absolute + X
		LDA $0200,Y	; Absolute + Y
		RTS

LDX_TEST	LDX #$20	; Immediate    
		LDX $20		; Zero page
		LDX $20,Y	; Zero page + Y
		LDX ABS:$20	; Absolute
		LDX $0200	; Absolute
		LDX $0200,Y	; Absolute + Y
		RTS
		
LDY_TEST	LDY #$20	; Immediate
		LDY $20		; Zero page
		LDY $20,X	; Zero page + X
		LDY ABS:$20
		LDY $0200	; Absolute
		LDY $0200,X	; Absolute + X
		RTS
END

DATA:
		.DB		"String  ", $00
		.DB		$0102, $03040506
		.DB		'<, '<'
		.DB 		-$01, -$0001
		.DB		-1, -129
		.DB		0b01010101, -0b01010101
DATA_END