; This program causes a bug in the assember, because the value X is
; not resolved to an address before addresses are assigned.

		.ORG		$0600

START
		CLC
X		.EQ		$ - 1

		BIT		X		; Should be ABS mode, not ZPG
		
		BRK