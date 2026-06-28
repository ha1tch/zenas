; Example 3: relative jumps (JR, DJNZ, conditional JR)
.ORG $8000

START:
        LD A, 10
        LD B, 5

        ; unconditional relative jump
        JR FORWARD      ; Jump forward

BACKWARD:
        ; We'll jump here from FORWARD
        DEC A
        CP 0
        JR Z, DONE      ; Jump to DONE if A is zero
        JR BACKWARD     ; Jump back to BACKWARD

FORWARD:
        INC A           ; A = 11
        JR BACKWARD     ; Jump backward

LOOP:
        ; DJNZ (decrement B and jump if not zero)
        DEC A
        DJNZ LOOP       ; Decrement B, jump to LOOP if B != 0

        ; conditional relative jumps
        LD A, 100
        CP 50
        JR C, LESS      ; Jump if carry (A < 50) - won't jump
        JR NC, GREATER  ; Jump if no carry (A >= 50) - will jump

LESS:
        LD A, 1         ; Shouldn't reach here
        JR DONE

GREATER:
        LD A, 2         ; Should reach here

DONE:
        HALT

.END START