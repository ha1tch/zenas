; Example Z80 Assembly Program
; This demonstrates the features of the zen80-based assembler

        .ORG $8000              ; Start at address $8000

; Program entry point
START:
        LD A, 0                 ; Clear accumulator
        LD B, 10                ; Load counter with 10
        LD HL, DATA             ; Point HL to data area

; Main loop - add numbers 1 through 10
LOOP:
        ADD A, B                ; Add B to A
        DEC B                   ; Decrement counter
        JR NZ, LOOP            ; Continue if not zero
        
        ; Store result
        LD (RESULT), A          ; Store sum in RESULT
        
        ; Display result (simplified - just output to port)
        OUT ($01), A            ; Output result to port 1
        
        ; Test some other instructions
        LD C, A                 ; Copy A to C
        XOR A                   ; Clear A using XOR
        CP C                    ; Compare A with C
        JP Z, ZERO              ; Jump if zero
        
        ; Test bit operations
        LD A, %10101010         ; Load binary pattern
        BIT 7, A                ; Test bit 7
        JR NZ, BIT_SET          ; Jump if bit is set
        
BIT_SET:
        SET 0, A                ; Set bit 0
        RES 1, A                ; Reset bit 1
        RLC A                   ; Rotate left circular
        
        ; Test 16-bit operations
        LD BC, $1234            ; Load BC with hex value
        LD DE, BUFFER           ; Load DE with address
        ADD HL, BC              ; Add BC to HL
        INC DE                  ; Increment DE
        
        ; Call a subroutine
        CALL MULTIPLY           ; Call multiply routine
        
        ; End program
        HALT                    ; Stop execution

ZERO:
        LD A, $FF               ; Load $FF if we got zero
        JP START                ; Jump back to start

; Subroutine: Multiply A by 2
MULTIPLY:
        SLA A                   ; Shift left (multiply by 2)
        RET                     ; Return

; Data area
DATA:
        .DB 1, 2, 3, 4, 5       ; Some test data
        .DB "Hello", 0          ; String with null terminator
        
BUFFER: .DB 0, 0, 0, 0, 0       ; Buffer space

RESULT: .DW 0                   ; 16-bit result storage

; Constants
MAX_COUNT .EQU 255              ; Define a constant
EOF_CHAR  .EQU $1A              ; End of file character

; Test various number formats
NUMBERS:
        .DB 42                  ; Decimal
        .DB $2A                 ; Hexadecimal with $
        .DB 2AH                 ; Hexadecimal with H suffix
        .DB %00101010           ; Binary with %
        .DB 101010B             ; Binary with B suffix

; Test indirect addressing
INDIRECT_TEST:
        LD A, (HL)              ; Load from address in HL
        LD (BC), A              ; Store to address in BC
        LD A, ($8000)           ; Load from absolute address
        LD ($8001), A           ; Store to absolute address

; Test conditions
CONDITION_TEST:
        CP 0                    ; Compare with zero
        JP Z, IS_ZERO           ; Jump if zero
        JP NZ, NOT_ZERO         ; Jump if not zero
        JP C, IS_CARRY          ; Jump if carry
        JP NC, NO_CARRY         ; Jump if no carry

IS_ZERO:
NOT_ZERO:
IS_CARRY:
NO_CARRY:
        RET                     ; Return from test

        .END START              ; End of program, entry point is START