		.ORG	$0600

; -------
; Exports
; -------

		.EX	START
		.EX	START.1
		.EX	LDA_TEST
		.EX	LDX_TEST
		.EX	END


; ---------
; Constants
; ---------

DOLLAR 		.EQ	'$'+$80
STORE		.EQ	$0200

; -------
; Program
; -------

START:
		JSR LDA_TEST
		JSR LDX_TEST
		JSR LDY_TEST
		LDX #$05
		LDA ($02,X)
		LDA #$08
		STA STORE+$1
		ADC #$FE
		STA STORE
		LDA #DOLLAR
		LDA STORE
		LDA #$00
		BEQ .1
		LDX #'$'+$80
.1		BRK

LDA_TEST	LDA #$20	; Immediate
		LDA $20		; Zero page
		LDA $20,X	; Zero page + X
		LDA ($20,X)	; Indirect + X
		LDA ($20),Y	; Indirect + Y
		LDA $0200	; Absolute
		LDA $0200,X	; Absolute + X
		LDA $0200,Y	; Absolute + Y
		RTS

LDX_TEST	LDX #$20	; Immediate    
		LDX $20		; Zero page
		LDX $20,Y	; Zero page + Y
		LDX $0200	; Absolute
		LDX $0200,Y	; Absolute + Y
		RTS
		
LDY_TEST	LDY #$20	; Immediate
		LDY $20		; Zero page
		LDY $20,X	; Zero page + X
		LDY $0200	; Absolute
		LDY $0200,X	; Absolute + X
		RTS

END