		.ORG 		$600

X 		.EQ 		$100

START		
		LDA #$44
		STA $00
		LDA #$55
		STA X
		LDY #1
		LDX ABS:X-1,Y
		LDX X-1,Y
		SED
		CLC
		LDA #$84
		ADC #$25
		.BYTE 		$18 $69 $25	; CLC, ADC #$25
		BRK

TABLE		.BYTE		0x02001516
		.AT 		START
		
