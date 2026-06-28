; Example 2: arithmetic, increment and decrement
.ORG $8000

START:
        ; Immediate arithmetic
        LD A, 10
        ADD A, 5        ; A = 15
        SUB 3           ; A = 12  
        AND $0F         ; A = 12
        OR $F0          ; A = 252
        XOR $FF         ; A = 3
        CP 3            ; Compare with 3
        
        ; Register increment/decrement (test if working)
        LD B, 50
        INC A           ; Increment A 
        INC B           ; Increment B
        DEC A           ; Decrement A
        DEC B           ; Decrement B
        
        ; other registers
        LD C, 100
        LD D, 200
        INC C
        DEC D
        
        ; 16-bit increment/decrement (test if working)
        LD BC, $1000
        LD DE, $2000
        LD HL, $3000
        
        INC BC          ; Increment BC pair
        DEC DE          ; Decrement DE pair  
        INC HL          ; Increment HL pair
        
        ; stack operations
        ; PUSH AF
        ; POP BC
        
        HALT

.END START