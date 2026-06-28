; Example 11: C-style macros with a register calling convention
;
; Demonstrates .MACRO_STYLE C together with .CALLING_CONVENTION REGISTER_FAST,
; where arguments and return values travel in registers. Each function becomes
; a Z80 routine; assignments from a call store the returned A into a variable.

.MACRO_STYLE C
.CALLING_CONVENTION REGISTER_FAST

// Simple C-style macro that adds 2 to a value
uint8_t add_two(uint8_t value) {
    asm {
        ADD A, 2;
    }
    return value;
}

// Another simple macro to set an LED
void set_led() {
    asm {
        LD A, 1;
        OUT (144), A;
    }
}

// Main program using C-style syntax
void main() {
    uint8_t result;

    // Call add_two(5) and store its result. The argument goes in A, the
    // routine runs, and the returned A is stored into result.
    result = add_two(5);

    // A call with no return value.
    set_led();
    
    asm {
        HALT;
    }
}

.END