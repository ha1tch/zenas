; Z80 Serial Communication Driver with Interrupt Handling
; Demonstrates practical macro usage with hardware I/O
; Uses only syntax features that actually work in our macro system

; =============================================================================
; TRADITIONAL MACROS - Hardware abstraction
; =============================================================================

.MACRO_STYLE TRADITIONAL
.CALLING_CONVENTION REGISTER_FAST

; Simplified macros that work like example9
MACRO UART_OUT(value)
    LD A, value
    OUT ($80), A        ; UART_DATA port
ENDMACRO

MACRO UART_SET_CONTROL(value)
    LD A, value
    OUT ($82), A        ; UART_CONTROL port
ENDMACRO

MACRO SIMPLE_DELAY(count)
    LD B, count
DELAY_LOOP:
    DJNZ DELAY_LOOP
ENDMACRO

; Status checking macros
MACRO WAIT_TX_READY
    PUSH AF
    PUSH BC
TX_WAIT:
    IN A, ($81)         ; UART_STATUS port
    BIT 0, A
    JR Z, TX_WAIT
    POP BC
    POP AF
ENDMACRO

MACRO WAIT_RX_READY
    PUSH AF
    PUSH BC
RX_WAIT:
    IN A, ($81)         ; UART_STATUS port
    BIT 1, A
    JR Z, RX_WAIT
    POP BC
    POP AF
ENDMACRO

; Buffer management macros
MACRO BUFFER_PUT(buffer_start, buffer_end, write_ptr, data_value)
    PUSH DE
    LD DE, write_ptr
    LD A, (DE)
    LD L, A
    INC DE
    LD A, (DE)
    LD H, A             ; HL now contains the write pointer value
    
    LD A, data_value
    LD (HL), A
    INC HL
    
    ; Check for wrap-around using direct comparison
    LD A, H
    CP buffer_end_high
    JR NZ, NO_WRAP_PUT
    LD A, L
    CP buffer_end_low
    JR NZ, NO_WRAP_PUT
    
    ; Wrap to start
    LD HL, buffer_start
NO_WRAP_PUT:
    ; Store HL back to write_ptr
    LD DE, write_ptr
    LD A, L
    LD (DE), A
    INC DE
    LD A, H
    LD (DE), A
    POP DE
ENDMACRO

MACRO BUFFER_GET(buffer_start, buffer_end, read_ptr)
    PUSH DE
    LD DE, read_ptr
    LD A, (DE)
    LD L, A
    INC DE
    LD A, (DE)
    LD H, A             ; HL now contains the read pointer value
    
    LD A, (HL)          ; Get the data
    PUSH AF             ; Save data
    INC HL
    
    ; Check for wrap-around using direct comparison
    LD B, A             ; (B not used here, this was wrong in original)
    LD A, H
    CP buffer_end_high
    JR NZ, NO_WRAP_GET
    LD A, L
    CP buffer_end_low
    JR NZ, NO_WRAP_GET
    
    ; Wrap to start
    LD HL, buffer_start
NO_WRAP_GET:
    ; Store HL back to read_ptr
    LD DE, read_ptr
    LD A, L
    LD (DE), A
    INC DE
    LD A, H
    LD (DE), A
    POP AF              ; Restore data to A
    POP DE
ENDMACRO

; Interrupt handling macros
MACRO SAVE_CONTEXT
    PUSH AF
    PUSH BC
    PUSH DE
    PUSH HL
ENDMACRO

MACRO RESTORE_CONTEXT
    POP HL
    POP DE
    POP BC
    POP AF
ENDMACRO

; =============================================================================
; MAIN DRIVER CODE - Using traditional macros
; =============================================================================

UART_INIT:
    ; Initialize UART - 9600 baud, 8N1
    UART_SET_CONTROL(UART_INIT_VALUE)
    
    LD A, BAUD_9600
    OUT ($83), A        ; UART_BAUD port
    
    ; Enable TX and RX  
    UART_SET_CONTROL(UART_TX_RX_ENABLE)
    
    ; Initialize buffer pointers (direct code)
    LD HL, TX_BUFFER_START
    LD (TX_WRITE_PTR), HL
    LD (TX_READ_PTR), HL
    
    LD HL, RX_BUFFER_START
    LD (RX_WRITE_PTR), HL
    LD (RX_READ_PTR), HL
    
    ; Enable interrupts
    LD A, UART_INT_ENABLE
    OUT ($84), A        ; UART_INT_MASK port
    EI
    
    RET

; Send a byte via UART (blocking)
UART_SEND_BYTE:
    ; Input: A = byte to send
    PUSH AF
    ; Wait for TX ready (expanded)
    PUSH AF
    PUSH BC
TX_WAIT_SEND:
    IN A, ($81)         ; UART_STATUS port
    BIT 0, A
    JR Z, TX_WAIT_SEND
    POP BC
    POP AF
    ; End wait for TX ready
    POP AF
    OUT ($80), A        ; UART_DATA port
    RET

; Receive a byte via UART (blocking)
UART_RECV_BYTE:
    ; Output: A = received byte
    ; Wait for RX ready (expanded)
    PUSH AF
    PUSH BC
RX_WAIT_RECV:
    IN A, ($81)         ; UART_STATUS port
    BIT 1, A
    JR Z, RX_WAIT_RECV
    POP BC
    POP AF
    ; End wait for RX ready
    IN A, ($80)         ; UART_DATA port
    RET

; Send a string via UART
UART_SEND_STRING:
    ; Input: HL = pointer to null-terminated string
    LD A, (HL)
    OR A
    RET Z               ; Return if null terminator
    
    CALL UART_SEND_BYTE
    INC HL
    JR UART_SEND_STRING

; Add byte to TX buffer (simplified)
UART_QUEUE_BYTE:
    ; Input: A = byte to queue
    ; For demo purposes, just send directly
    UART_OUT(42)        ; Simple macro demo
    SIMPLE_DELAY(10)    ; Another simple macro demo
    RET

START_TX_INT:
    LD A, UART_INT_TX_ENABLE
    OUT ($84), A        ; UART_INT_MASK port
    RET

; =============================================================================
; C-STYLE MACROS - Protocol handlers
; =============================================================================



; =============================================================================
; MORE TRADITIONAL MACROS - Protocol helpers
; =============================================================================

; Send a command byte
MACRO SEND_CMD(cmd_value)
    LD A, PACKET_HEADER
    CALL UART_SEND_BYTE
    LD A, cmd_value
    CALL UART_SEND_BYTE
ENDMACRO

; Simple LED control macros
MACRO LED_ON
    LD A, 1
    OUT ($90), A        ; LED_PORT
ENDMACRO

MACRO LED_OFF
    LD A, 0
    OUT ($90), A        ; LED_PORT
ENDMACRO

; =============================================================================
; MAIN PROGRAM - Using only traditional macros
; =============================================================================

MAIN_LOOP:
    ; Demonstrate macro usage
    SEND_CMD(CMD_LED_ON)
    LED_ON
    SIMPLE_DELAY(100)
    
    SEND_CMD(CMD_LED_OFF)
    LED_OFF
    SIMPLE_DELAY(100)
    
    JP MAIN_LOOP

; =============================================================================
; INTERRUPT HANDLERS
; =============================================================================

; =============================================================================
; SIMPLIFIED INTERRUPT HANDLERS
; =============================================================================

UART_TX_INTERRUPT:
    SAVE_CONTEXT
    
    ; Simple TX handling - just send a test byte
    LD A, 65            ; ASCII 'A'
    OUT ($80), A        ; UART_DATA port
    
    RESTORE_CONTEXT
    EI
    RET

UART_RX_INTERRUPT:
    SAVE_CONTEXT
    
    ; Simple RX handling - just read and discard
    IN A, ($80)         ; UART_DATA port
    
    RESTORE_CONTEXT
    EI
    RET

; =============================================================================
; MAIN PROGRAM
; =============================================================================

START:
    ; Initialize system
    CALL UART_INIT
    
    ; Send startup message
    LD HL, STARTUP_MSG
    CALL UART_SEND_STRING
    
    ; Start main communication loop
    CALL MAIN_LOOP
    
    HALT                ; Should never reach here

; =============================================================================
; DATA AND CONSTANTS
; =============================================================================

; Hardware addresses
UART_DATA       .EQU $80
UART_STATUS     .EQU $81
UART_CONTROL    .EQU $82
UART_BAUD       .EQU $83
UART_INT_MASK   .EQU $84

LED_PORT        .EQU $90
SENSOR_PORT     .EQU $91

; UART configuration values
UART_INIT_VALUE     .EQU $03
BAUD_9600          .EQU $60
UART_TX_RX_ENABLE  .EQU $0C
UART_INT_ENABLE    .EQU $03
UART_INT_TX_ENABLE .EQU $01
UART_INT_RX_ONLY   .EQU $02

; Protocol constants
PACKET_HEADER   .EQU $AA
CMD_LED_ON      .EQU $01
CMD_LED_OFF     .EQU $02
CMD_READ_SENSOR .EQU $03
ERROR_RESPONSE  .EQU $FF

; Buffer definitions
TX_BUFFER_START .EQU $2000
TX_BUFFER_END   .EQU $2080
RX_BUFFER_START .EQU $2080
RX_BUFFER_END   .EQU $2100

; Buffer boundary constants (for macro use)
buffer_end_high .EQU $20    ; High byte of $2080 and $2100
buffer_end_low  .EQU $80    ; Low byte of $2080
rx_end_low      .EQU $00    ; Low byte of $2100

; Buffer pointers
TX_WRITE_PTR:   .DW TX_BUFFER_START
TX_READ_PTR:    .DW TX_BUFFER_START
RX_WRITE_PTR:   .DW RX_BUFFER_START
RX_READ_PTR:    .DW RX_BUFFER_START

; Messages
STARTUP_MSG:    .DB "Z80 Driver Ready", 13, 10, 0

.END START