; Example 6: miscellaneous core instructions
.ORG $8000

START:
        ; interrupt control
        DI              ; Disable interrupts (0xF3)
        LD A, $42       ; Some work while interrupts disabled
        EI              ; Enable interrupts (0xFB)
        
        ; carry flag operations
        SCF             ; Set carry flag (0x37)
        CCF             ; Complement carry flag (0x3F) - should clear it
        
        ; accumulator operations
        LD A, $F0       ; Load test value
        CPL             ; Complement A (0x2F) - A becomes $0F
        
        ; decimal adjust
        LD A, $09       ; Load 9
        ADD A, $01      ; Add 1 = $0A (not valid BCD)
        DAA             ; Decimal adjust A (0x27) - should make it $10
        
        ; Another DAA test
        LD A, $89       ; Load 89 (valid BCD)
        ADD A, $23      ; Add 23 = $AC (not valid BCD)  
        DAA             ; Should adjust to $12 with carry set
        
        ; flag manipulation sequence
        SCF             ; Set carry
        LD A, $55
        CPL             ; Complement A
        CCF             ; Complement carry
        
        ; in interrupt context simulation
        DI              ; Disable interrupts
        LD SP, $8FFF    ; Set up stack
        ; ... critical section ...
        EI              ; Re-enable interrupts
        
        ; Final test - all instructions together
        SCF             ; 0x37
        CCF             ; 0x3F  
        CPL             ; 0x2F
        DAA             ; 0x27
        DI              ; 0xF3
        EI              ; 0xFB
        
        HALT

.END START