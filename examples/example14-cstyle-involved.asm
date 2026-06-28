; Example 14: C-style functions with parameters and return values
;
; Shows how the C-style macro front-end maps functions to Z80 routines.
; Each function becomes a labelled routine; the body's asm { } block is
; emitted verbatim, and a return becomes a RET. Parameters follow a simple
; register calling convention (the first argument arrives in A).

.MACRO_STYLE C

; A function taking one parameter. The argument is passed in A, so the body
; operates on A directly and returns it.
uint8_t add_five(uint8_t value) {
    asm {
        ADD A, 5;
    }
    return value;
}

; A function taking two parameters. By convention the first argument is in A
; and the second in B, so adding them is a single ADD A, B.
uint8_t add_two_numbers(uint8_t a, uint8_t b) {
    asm {
        ADD A, B;
    }
    return a;
}

; A function with no parameters that loads a constant into A and returns it.
uint8_t get_constant() {
    asm {
        LD A, 42;
    }
    return 42;
}

; main calls each function in turn, then halts. Each call assembles to a CALL
; of the corresponding routine.
void main() {
    get_constant();
    add_five(10);
    add_two_numbers(15, 25);

    asm {
        HALT;
    }
}

.END
