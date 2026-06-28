; Example 5: CB-prefixed bit operations (BIT, SET, RES, shifts)
.ORG $8000

START:
        LD A, $55       ; Load test pattern 01010101
        LD B, $AA       ; Load test pattern 10101010
        
        ; bit testing
        BIT 0, A        ; bit 0 of A (should be 1)
        BIT 1, A        ; bit 1 of A (should be 0)
        BIT 7, B        ; bit 7 of B (should be 1)
        
        ; bit setting  
        SET 1, A        ; Set bit 1 of A (A becomes $57)
        SET 0, B        ; Set bit 0 of B (B becomes $AB)
        
        ; bit clearing
        RES 0, A        ; Clear bit 0 of A (A becomes $56)
        RES 7, B        ; Clear bit 7 of B (B becomes $2B)
        
        ; rotations
        LD C, $81       ; Load 10000001
        RLC C          ; Rotate left circular: 00000011
        RRC C          ; Rotate right circular: 10000001
        
        ; rotations through carry
        LD D, $80       ; Load 10000000
        RL D           ; Rotate left through carry
        RR D           ; Rotate right through carry
        
        ; shifts
        LD E, $FF       ; Load 11111111
        SLA E          ; Shift left arithmetic: 11111110
        SRA E          ; Shift right arithmetic: 11111111 (sign extend)
        SRL E          ; Shift right logical: 01111111
        
        ; with different bit positions
        LD H, 0
        SET 3, H       ; Set bit 3: H = $08
        SET 5, H       ; Set bit 5: H = $28
        BIT 3, H       ; bit 3 (should be set)
        RES 3, H       ; Clear bit 3: H = $20
        
        ; all 8 bits
        LD L, 0
        SET 0, L       ; L = $01
        SET 1, L       ; L = $03
        SET 2, L       ; L = $07
        SET 3, L       ; L = $0F
        SET 4, L       ; L = $1F
        SET 5, L       ; L = $3F
        SET 6, L       ; L = $7F
        SET 7, L       ; L = $FF
        
        HALT

.END START