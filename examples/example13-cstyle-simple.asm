; Example 13: combining several C-style functions
;
; A slightly larger C-style program: a few small functions, each wrapping a
; short asm { } block, called in sequence from main. Shows how function
; definitions, calls, and inline assembly fit together.

.MACRO_STYLE C

// Simple utility function - adds 1 to accumulator
void increment() {
    asm {
        ADD A, 1;
    }
}

// Function that does multiple operations
void complex_math() {
    asm {
        LD A, 10;       // Load initial value
        ADD A, 5;       // Add 5 (now 15)
        ADD A, A;       // Double it (now 30)
        LD B, A;        // Store in B register
    }
}

// I/O operation function
void output_value() {
    asm {
        LD A, 0x42;     // Load ASCII 'B'
        OUT (0x90), A;  // Output to port 144
    }
}

// Main program using multiple functions
void main() {
    // Call each function in sequence
    complex_math();
    increment();
    output_value();
    
    // Final operations
    asm {
        LD A, 0xFF;     // Load final value
        HALT;           // Stop execution
    }
}

.END