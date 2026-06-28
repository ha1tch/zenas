; Example 12: the smallest C-style program
;
; A single main function whose body is one asm { } block. This is the minimal
; shape of a C-style source: .MACRO_STYLE C, then a void main() containing
; inline assembly.

.MACRO_STYLE C

void main() {
    uint8_t result = 42;
    asm {
        HALT;
    }
}
