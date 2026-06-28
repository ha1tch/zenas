; Z80 Assembler Macro System Test Examples
; This file demonstrates both Traditional and C-style macro syntax

; =============================================================================
; TRADITIONAL STYLE MACROS
; =============================================================================

.MACRO_STYLE TRADITIONAL
.CALLING_CONVENTION REGISTER_FAST

; Simple delay macro
MACRO DELAY_CYCLES(cycles)
    PUSH BC
    LD B, cycles
DELAY_LOOP:
    DJNZ DELAY_LOOP
    POP BC
ENDMACRO

; Memory copy macro
MACRO COPY_BYTES(count)
    PUSH BC
    LD BC, count
COPY_LOOP:
    LD A, (HL)
    LD (DE), A
    INC HL
    INC DE
    DEC BC
    LD A, B
    OR C
    JR NZ, COPY_LOOP
    POP BC
ENDMACRO

; Multiply by 8 (shift left 3 times)
MACRO MUL8(value)
    LD A, value
    ADD A, A    ; * 2
    ADD A, A    ; * 4
    ADD A, A    ; * 8
ENDMACRO

; Usage of traditional macros
TRADITIONAL_DEMO:
    LD HL, SOURCE_DATA
    LD DE, DEST_BUFFER
    
    DELAY_CYCLES(50)        ; Wait 50 cycles
    COPY_BYTES(16)          ; Copy 16 bytes
    MUL8(42)                ; Multiply 42 by 8
    
    HALT

; =============================================================================
; C-STYLE MACROS
; =============================================================================

.MACRO_STYLE C
.CALLING_CONVENTION REGISTER_FAST

// Simple arithmetic function
uint8_t add_numbers(uint8_t a, uint8_t b) {
    // Parameters: a in A register, b in B register
    // Return: result in A register
    ADD A, B;
}

// 16-bit addition
uint16_t add16(uint16_t val1, uint16_t val2) {
    // Parameters: val1 in HL, val2 in DE
    // Return: result in HL
    ADD HL, DE;
}

// Memory fill function
void fill_memory(uint16_t address, uint8_t value, uint8_t count) {
    // Parameters: address in HL, value in A, count in B
    PUSH AF;
FILL_LOOP:
    LD (HL), A;
    INC HL;
    DEC B;
    JR NZ, FILL_LOOP;
    POP AF;
}

// Conditional operation
uint8_t max_value(uint8_t a, uint8_t b) {
    // Parameters: a in A, b in B
    // Return: maximum value in A
    CP B;
    JR NC, A_IS_LARGER;
    LD A, B;    // B is larger
A_IS_LARGER:
    // A already contains the larger value
}

// Stack manipulation helper
void save_registers() {
    PUSH AF;
    PUSH BC;
    PUSH DE;
    PUSH HL;
}

void restore_registers() {
    POP HL;
    POP DE;
    POP BC;
    POP AF;
}

// Usage of C-style macros
void c_demo() {
    uint8_t result;
    
    save_registers();
    
    result = add_numbers(10, 20);       // Add 10 + 20
    result = max_value(result, 50);     // Get max of result and 50
    
    fill_memory(0x8000, 0xFF, 16);     // Fill 16 bytes with 0xFF
    
    restore_registers();
}

// Mixed usage demonstration
void main() {
    // Initialize data
    LD HL, WORK_AREA;
    LD DE, BUFFER;
    
    // Use C-style macros
    c_demo();
    
    // The assembler can mix styles in different files
    // but each file must declare its style
    
    HALT;
}

; =============================================================================
; DATA AREA
; =============================================================================

SOURCE_DATA:
    .DB "Hello, Z80 Macros!", 0

DEST_BUFFER:
    .DB 0, 0, 0, 0, 0, 0, 0, 0
    .DB 0, 0, 0, 0, 0, 0, 0, 0

WORK_AREA:
    .DW 0x0000

BUFFER:
    .DS 64      ; Reserve 64 bytes

; =============================================================================
; EXPECTED BEHAVIOR
; =============================================================================

; Traditional macro calls:
; DELAY_CYCLES(50) expands to:
;   PUSH BC
;   LD B, 50
; DELAY_LOOP_1_1:
;   DJNZ DELAY_LOOP_1_1
;   POP BC

; C-style macro calls:
; add_numbers(10, 20) expands to:
;   LD A, 10        ; Load first parameter
;   LD B, 20        ; Load second parameter
;   ADD A, B        ; Execute macro body

; The macro system automatically:
; 1. Generates unique labels to prevent conflicts
; 2. Maps parameters to registers according to calling convention
; 3. Validates parameter types and counts
; 4. Provides clear error messages for macro-related issues
