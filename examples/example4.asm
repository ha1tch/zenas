; Example 4: extended load instructions (LD SP,HL; LD HL,(nn); ...)
.ORG $8000

DATA_AREA   .EQU $9000
BUFFER      .EQU $9010

START:
        ; LD SP,HL - load stack pointer from HL
        LD HL, $8FFF
        LD SP, HL       ; SP = $8FFF
        
        ; LD HL,(nn) - load HL from memory address
        LD HL, (DATA_AREA)      ; Load HL from address $9000
        
        ; LD (nn),HL - store HL to memory address  
        LD (BUFFER), HL         ; Store HL to address $9010
        
        ; ED-prefixed loads (BC, DE, SP)
        LD BC, (DATA_AREA)      ; Load BC from address $9000 (ED prefix)
        LD (BUFFER), BC         ; Store BC to address $9010 (ED prefix)
        
        LD DE, (DATA_AREA)      ; Load DE from address $9000 (ED prefix)
        LD (BUFFER), DE         ; Store DE to address $9010 (ED prefix)
        
        ; with different addresses
        LD HL, $1234
        LD (DATA_AREA), HL      ; Store $1234 to $9000
        
        LD BC, $5678
        LD (DATA_AREA), BC      ; Store $5678 to $9000
        
        HALT

; Data area (just for address references)
.ORG DATA_AREA
        .DW $ABCD               ; Default data at $9000

.END START