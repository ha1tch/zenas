; Example 15: .MACRO_MODE SINGLETON
;
; By default a macro is expanded inline at every call. SINGLETON mode instead
; emits the body once as a callable routine and turns each instantiation into a
; CALL - smaller when a sizeable body is used several times.
;
; Parameters are passed through fixed memory slots: each call writes its argument
; to the macro's slot and then CALLs, and the body reads the parameter from the
; slot (so a parameter `val` is read as `(val)`). Because the slots are fixed
; locations, a parameterised singleton is not re-entrant - use INLINE for
; recursive or interrupt-reachable code.

.MACRO_MODE SINGLETON

; A parameterless helper: advance the cursor in HL by one screen line (32 bytes)
; on the ZX Spectrum bitmap.
MACRO next_line()
    PUSH DE
    LD DE, 32
    ADD HL, DE
    POP DE
ENDMACRO

; A parameterised helper: write a byte at (HL) and advance. The argument arrives
; through the slot, read here as (val).
MACRO emit(uint8_t val)
    LD A, (val)
    LD (HL), A
    INC HL
ENDMACRO

    ORG 0x8000

start:
    LD HL, 0x4000        ; top-left of the screen bitmap
    next_line()          ; each of these is a CALL to the shared routine
    next_line()
    next_line()
    next_line()
    ; HL now points 4 lines down (0x4000 + 4*32 = 0x4080)

    LD HL, 0xC000
    emit(0x11)           ; each writes its argument via the slot, then CALLs
    emit(0x22)
    emit(0x33)
    ; 0xC000..0xC002 now hold 11 22 33
    HALT

.END

