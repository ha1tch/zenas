package assembler

import (
	"fmt"
	"strings"
)

// Encoder converts parsed instructions into Z80 machine code
// This mirrors the instruction patterns from the zen80 emulator
type Encoder struct {
	instructions map[string][]*InstructionTemplate
}

// InstructionTemplate defines how to encode a specific instruction variant
type InstructionTemplate struct {
	Mnemonic     string
	OperandTypes []OperandType
	Encode       func(*InstructionTemplate, []*ResolvedOperand) ([]uint8, error)
	BaseOpcode   uint8
	Prefix       uint8
	Cycles       int
}

// Register mappings extracted from zen80 emulator's getRegister8 function
var (
	register8Map = map[string]uint8{
		"B": 0, "C": 1, "D": 2, "E": 3,
		"H": 4, "L": 5, "A": 7,
		// Undocumented: IXH/IXL and IYH/IYL are handled in prefix encoding
		"IXH": 4, "IXL": 5, "IYH": 4, "IYL": 5,
	}
	
	register16Map = map[string]uint8{
		"BC": 0, "DE": 1, "HL": 2, "SP": 3,
		"IX": 2, "IY": 2, // Will use DD/FD prefix
	}
	
	// Stack register mappings (PUSH/POP use different encoding)
	stackRegisterMap = map[string]uint8{
		"BC": 0, "DE": 1, "HL": 2, "AF": 3,
	}
	
	conditionMap = map[string]uint8{
		"NZ": 0, "Z": 1, "NC": 2, "C": 3,
		"PO": 4, "PE": 5, "P": 6, "M": 7,
	}
)

// NewEncoder creates a new instruction encoder
func NewEncoder() *Encoder {
	encoder := &Encoder{
		instructions: make(map[string][]*InstructionTemplate),
	}
	encoder.initializeInstructions()
	return encoder
}

// EnableZ80N registers the Z80N (ZX Spectrum Next) extended instructions on top
// of the base Z80 set. Opcodes are from the official SpecNext "Extended Z80
// instruction set" reference (see docs/Z80N_REFERENCE.md); all are ED-prefixed.
// This is off by default and enabled via --next / --cpu=Z80N.
func (e *Encoder) EnableZ80N() {
	// No-operand forms (ED prefix + single opcode byte).
	simpleN := []struct {
		mnemonic string
		opcode   uint8
	}{
		{"SWAPNIB", 0x23}, {"OUTINB", 0x90},
		{"PIXELDN", 0x93}, {"PIXELAD", 0x94}, {"SETAE", 0x95},
		{"LDIX", 0xA4}, {"LDWS", 0xA5}, {"LDDX", 0xAC},
		{"LDIRX", 0xB4}, {"LDPIRX", 0xB7}, {"LDDRX", 0xBC},
	}
	for _, s := range simpleN {
		e.addInstruction(s.mnemonic, []OperandType{}, s.opcode, 0xED, 8, encodeSimple)
	}

	// Fixed-operand forms: the operand is hard-wired, so accept both the bare
	// mnemonic and the canonical operand spelling, but reject wrong operands.
	// MIRROR A
	e.addInstruction("MIRROR", []OperandType{}, 0x24, 0xED, 8, encodeSimple)
	e.addInstruction("MIRROR", []OperandType{OperandRegister8}, 0x24, 0xED, 8, encodeZ80NFixedReg("A"))
	// MUL D,E
	e.addInstruction("MUL", []OperandType{}, 0x30, 0xED, 8, encodeSimple)
	e.addInstruction("MUL", []OperandType{OperandRegister8, OperandRegister8}, 0x30, 0xED, 8, encodeZ80NFixedRegPair("D", "E"))
	// JP (C)
	e.addInstruction("JP", []OperandType{OperandIndirect}, 0x98, 0xED, 13, encodeZ80NJPc)
	// Barrel shifts: BSxx DE,B and BRLC DE,B
	for _, bs := range []struct {
		mnemonic string
		opcode   uint8
	}{{"BSLA", 0x28}, {"BSRA", 0x29}, {"BSRL", 0x2A}, {"BSRF", 0x2B}, {"BRLC", 0x2C}} {
		e.addInstruction(bs.mnemonic, []OperandType{OperandRegister16, OperandRegister8}, bs.opcode, 0xED, 8, encodeZ80NFixedRegPair("DE", "B"))
	}

	// TEST n  (ED 27 n)
	e.addInstruction("TEST", []OperandType{OperandImmediate8}, 0x27, 0xED, 11, encodeZ80NTEST)

	// ADD rr,A  (ED 31/32/33) and ADD rr,nn (ED 34/35/36)
	e.addInstruction("ADD", []OperandType{OperandRegister16, OperandRegister8}, 0x31, 0xED, 8, encodeZ80NADDrrA)
	e.addInstruction("ADD", []OperandType{OperandRegister16, OperandImmediate16}, 0x34, 0xED, 16, encodeZ80NADDrrnn)

	// PUSH nn  (ED 8A hi lo) - BIG-ENDIAN operand
	e.addInstruction("PUSH", []OperandType{OperandImmediate16}, 0x8A, 0xED, 23, encodeZ80NPUSHnn)

	// NEXTREG n,n (ED 91 reg val) and NEXTREG n,A (ED 92 reg)
	e.addInstruction("NEXTREG", []OperandType{OperandImmediate8, OperandImmediate8}, 0x91, 0xED, 20, encodeZ80NNEXTREGnn)
	e.addInstruction("NEXTREG", []OperandType{OperandImmediate8, OperandRegister8}, 0x92, 0xED, 17, encodeZ80NNEXTREGnA)
}

// Encode converts an instruction with resolved operands to machine code
func (e *Encoder) Encode(mnemonic string, operands []*ResolvedOperand) ([]uint8, error) {
	templates, exists := e.instructions[strings.ToUpper(mnemonic)]
	if !exists {
		return nil, fmt.Errorf("unknown instruction: %s", mnemonic)
	}
	
	// Find matching template based on operand types. Several templates can match
	// the same operand *types* (e.g. JP (HL) and the Z80N JP (C) both match
	// JP [indirect]); a template may still reject the specific operands at encode
	// time. In that case, try the next matching template and only surface the
	// error if none succeed.
	var firstErr error
	for _, template := range templates {
		if e.matchesTemplate(template, operands) {
			code, err := template.Encode(template, operands)
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			return applyHalfIndexPrefix(code, operands)
		}
	}
	if firstErr != nil {
		return nil, firstErr
	}
	
	// Create better error message showing what operands we have
	var operandDesc []string
	for _, op := range operands {
		switch op.Type {
		case OperandRegister8:
			operandDesc = append(operandDesc, "reg8:"+op.Register)
		case OperandRegister16:
			operandDesc = append(operandDesc, "reg16:"+op.Register)
		case OperandCondition:
			operandDesc = append(operandDesc, "cond:"+op.Condition)
		case OperandImmediate8:
			operandDesc = append(operandDesc, fmt.Sprintf("imm8:%d", op.Value))
		case OperandImmediate16:
			operandDesc = append(operandDesc, fmt.Sprintf("imm16:%d", op.Value))
		case OperandIndirect:
			if op.Register != "" {
				operandDesc = append(operandDesc, "indirect:"+op.Register)
			} else {
				operandDesc = append(operandDesc, fmt.Sprintf("indirect:$%04X", op.Value))
			}
		default:
			operandDesc = append(operandDesc, fmt.Sprintf("unknown:%v", op.Type))
		}
	}
	
	return nil, fmt.Errorf("no matching encoding for %s with operands: [%s]", mnemonic, strings.Join(operandDesc, ", "))
}

// matchesTemplate checks if operands match a template
func (e *Encoder) matchesTemplate(template *InstructionTemplate, operands []*ResolvedOperand) bool {
	if len(template.OperandTypes) != len(operands) {
		return false
	}
	
	for i, expectedType := range template.OperandTypes {
		if !e.operandTypeMatches(expectedType, operands[i]) {
			return false
		}
	}
	
	return true
}

// operandTypeMatches checks if an operand matches the expected type
func (e *Encoder) operandTypeMatches(expected OperandType, operand *ResolvedOperand) bool {
	switch expected {
	case OperandRegister8:
		return operand.Type == OperandRegister8
	case OperandRegister16:
		return operand.Type == OperandRegister16
	case OperandImmediate8:
		return operand.Type == OperandImmediate8 || 
		       (operand.Type == OperandImmediate16 && operand.Value >= -128 && operand.Value <= 255)
	case OperandImmediate16:
		return operand.Type == OperandImmediate16 || operand.Type == OperandImmediate8
	case OperandIndirect:
		return operand.Type == OperandIndirect
	case OperandCondition:
		return operand.Type == OperandCondition
	case OperandRelative:
		return operand.Type == OperandRelative || operand.Type == OperandImmediate16 || operand.Type == OperandImmediate8
	default:
		return false
	}
}

// initializeInstructions sets up the instruction encoding templates
// These patterns are extracted from the zen80 emulator's decode.go
func (e *Encoder) initializeInstructions() {
	// Block 1: 8-bit register loads (from executeBlock1)
	e.addInstruction("LD", []OperandType{OperandRegister8, OperandRegister8}, 0x40, 0x00, 4, encodeLDrr)
	e.addInstruction("LD", []OperandType{OperandRegister8, OperandImmediate8}, 0x06, 0x00, 7, encodeLDrn)
	e.addInstruction("LD", []OperandType{OperandRegister8, OperandIndirect}, 0x46, 0x00, 7, encodeLDrHL)
	e.addInstruction("LD", []OperandType{OperandIndirect, OperandRegister8}, 0x70, 0x00, 7, encodeLDHLr)
	e.addInstruction("LD", []OperandType{OperandIndirect, OperandImmediate8}, 0x36, 0x00, 10, encodeLDHLn)
	
	// Extended 16-bit loads
	e.addInstruction("LD", []OperandType{OperandRegister16, OperandImmediate16}, 0x01, 0x00, 10, encodeLDrpnn)
	e.addInstruction("LD", []OperandType{OperandRegister16, OperandIndirect}, 0x2A, 0x00, 16, encodeLDrpaddr)
	e.addInstruction("LD", []OperandType{OperandIndirect, OperandRegister16}, 0x22, 0x00, 16, encodeLDaddrrp)
	e.addInstruction("LD", []OperandType{OperandRegister16, OperandRegister16}, 0xF9, 0x00, 6, encodeLDSPHL)
	
	// Block 2: ALU operations (from executeBlock2)
	e.addInstruction("ADD", []OperandType{OperandRegister8, OperandRegister8}, 0x80, 0x00, 4, encodeALUr)
	e.addInstruction("ADC", []OperandType{OperandRegister8, OperandRegister8}, 0x88, 0x00, 4, encodeALUr)
	e.addInstruction("SUB", []OperandType{OperandRegister8}, 0x90, 0x00, 4, encodeALUr)
	e.addInstruction("SBC", []OperandType{OperandRegister8, OperandRegister8}, 0x98, 0x00, 4, encodeALUr)
	e.addInstruction("AND", []OperandType{OperandRegister8}, 0xA0, 0x00, 4, encodeALUr)
	e.addInstruction("XOR", []OperandType{OperandRegister8}, 0xA8, 0x00, 4, encodeALUr)
	e.addInstruction("OR", []OperandType{OperandRegister8}, 0xB0, 0x00, 4, encodeALUr)
	e.addInstruction("CP", []OperandType{OperandRegister8}, 0xB8, 0x00, 4, encodeALUr)
	
	// ALU with immediate values
	e.addInstruction("ADD", []OperandType{OperandRegister8, OperandImmediate8}, 0xC6, 0x00, 7, encodeALUn)
	e.addInstruction("ADC", []OperandType{OperandRegister8, OperandImmediate8}, 0xCE, 0x00, 7, encodeALUn)
	e.addInstruction("SUB", []OperandType{OperandImmediate8}, 0xD6, 0x00, 7, encodeALUn)
	e.addInstruction("SBC", []OperandType{OperandRegister8, OperandImmediate8}, 0xDE, 0x00, 7, encodeALUn)
	e.addInstruction("AND", []OperandType{OperandImmediate8}, 0xE6, 0x00, 7, encodeALUn)
	e.addInstruction("XOR", []OperandType{OperandImmediate8}, 0xEE, 0x00, 7, encodeALUn)
	e.addInstruction("OR", []OperandType{OperandImmediate8}, 0xF6, 0x00, 7, encodeALUn)
	e.addInstruction("CP", []OperandType{OperandImmediate8}, 0xFE, 0x00, 7, encodeALUn)

	// ALU operations on (HL) / (IX+d) / (IY+d). BaseOpcode is the register-form
	// base; the indirect encoder ORs in reg code 6 and adds DD/FD + displacement
	// for index registers. Two-operand forms (ADD/ADC/SBC A,(HL)) and one-operand
	// forms (SUB/AND/XOR/OR/CP (HL)) are both registered.
	e.addInstruction("ADD", []OperandType{OperandRegister8, OperandIndirect}, 0x80, 0x00, 7, encodeALUIndirect)
	e.addInstruction("ADC", []OperandType{OperandRegister8, OperandIndirect}, 0x88, 0x00, 7, encodeALUIndirect)
	e.addInstruction("SBC", []OperandType{OperandRegister8, OperandIndirect}, 0x98, 0x00, 7, encodeALUIndirect)
	e.addInstruction("SUB", []OperandType{OperandIndirect}, 0x90, 0x00, 7, encodeALUIndirect)
	e.addInstruction("AND", []OperandType{OperandIndirect}, 0xA0, 0x00, 7, encodeALUIndirect)
	e.addInstruction("XOR", []OperandType{OperandIndirect}, 0xA8, 0x00, 7, encodeALUIndirect)
	e.addInstruction("OR", []OperandType{OperandIndirect}, 0xB0, 0x00, 7, encodeALUIndirect)
	e.addInstruction("CP", []OperandType{OperandIndirect}, 0xB8, 0x00, 7, encodeALUIndirect)

	// INC/DEC on (HL) / (IX+d) / (IY+d).
	e.addInstruction("INC", []OperandType{OperandIndirect}, 0x34, 0x00, 11, encodeINCDECIndirect)
	e.addInstruction("DEC", []OperandType{OperandIndirect}, 0x35, 0x00, 11, encodeINCDECIndirect)
	
	// Block 0: Miscellaneous (from executeBlock0)
	e.addInstruction("NOP", []OperandType{}, 0x00, 0x00, 4, encodeSimple)
	e.addInstruction("HALT", []OperandType{}, 0x76, 0x00, 4, encodeSimple)
	
	// Flag and interrupt control instructions
	e.addInstruction("CCF", []OperandType{}, 0x3F, 0x00, 4, encodeSimple)
	e.addInstruction("SCF", []OperandType{}, 0x37, 0x00, 4, encodeSimple)
	e.addInstruction("CPL", []OperandType{}, 0x2F, 0x00, 4, encodeSimple)
	e.addInstruction("DAA", []OperandType{}, 0x27, 0x00, 4, encodeSimple)
	e.addInstruction("EI", []OperandType{}, 0xFB, 0x00, 4, encodeSimple)
	e.addInstruction("DI", []OperandType{}, 0xF3, 0x00, 4, encodeSimple)
	
	e.addInstruction("INC", []OperandType{OperandRegister8}, 0x04, 0x00, 4, encodeINCr)
	e.addInstruction("DEC", []OperandType{OperandRegister8}, 0x05, 0x00, 4, encodeDECr)
	
	// 16-bit operations
	e.addInstruction("LD", []OperandType{OperandRegister16, OperandImmediate16}, 0x01, 0x00, 10, encodeLDrpnn)
	e.addInstruction("LD", []OperandType{OperandRegister16, OperandIndirect}, 0x2A, 0x00, 16, encodeLDrpaddr)
	e.addInstruction("LD", []OperandType{OperandIndirect, OperandRegister16}, 0x22, 0x00, 16, encodeLDaddrrp)
	e.addInstruction("LD", []OperandType{OperandRegister16, OperandRegister16}, 0xF9, 0x00, 6, encodeLDSPHL)
	e.addInstruction("ADD", []OperandType{OperandRegister16, OperandRegister16}, 0x09, 0x00, 11, encodeADDHLrp)
	e.addInstruction("INC", []OperandType{OperandRegister16}, 0x03, 0x00, 6, encodeINCrp)
	e.addInstruction("DEC", []OperandType{OperandRegister16}, 0x0B, 0x00, 6, encodeDECrp)
	
	// Stack operations (PUSH/POP) - ADDED
	e.addInstruction("PUSH", []OperandType{OperandRegister16}, 0xC5, 0x00, 11, encodePUSHrp)
	e.addInstruction("POP", []OperandType{OperandRegister16}, 0xC1, 0x00, 10, encodePOPrp)
	
	// Control flow (from executeBlock3)
	e.addInstruction("JP", []OperandType{OperandImmediate16}, 0xC3, 0x00, 10, encodeJPnn)
	e.addInstruction("JP", []OperandType{OperandCondition, OperandImmediate16}, 0xC2, 0x00, 10, encodeJPccnn)
	e.addInstruction("JR", []OperandType{OperandRelative}, 0x18, 0x00, 12, encodeJRd)
	e.addInstruction("JR", []OperandType{OperandCondition, OperandRelative}, 0x20, 0x00, 12, encodeJRccd)
	e.addInstruction("DJNZ", []OperandType{OperandRelative}, 0x10, 0x00, 13, encodeDJNZ)
	e.addInstruction("CALL", []OperandType{OperandImmediate16}, 0xCD, 0x00, 17, encodeCALLnn)
	e.addInstruction("CALL", []OperandType{OperandCondition, OperandImmediate16}, 0xC4, 0x00, 17, encodeCALLccnn)
	e.addInstruction("RET", []OperandType{}, 0xC9, 0x00, 10, encodeSimple)
	e.addInstruction("RET", []OperandType{OperandCondition}, 0xC0, 0x00, 5, encodeRETcc)

	// JP (HL) / JP (IX) / JP (IY) - register-indirect jump
	e.addInstruction("JP", []OperandType{OperandIndirect}, 0xE9, 0x00, 4, encodeJPindirect)

	// LD (IX+d),n / LD (IY+d),n - store immediate to indexed; the (HL) form is
	// already registered above (encodeLDHLn), which now also handles IX/IY.

	// Interrupt/refresh register loads (ED-prefixed) are handled inside
	// encodeLDrr, which already matches the LD r,r template.

	// Block I/O (ED-prefixed, no operands)
	e.addInstruction("INI", []OperandType{}, 0xA2, 0xED, 16, encodeSimple)
	e.addInstruction("IND", []OperandType{}, 0xAA, 0xED, 16, encodeSimple)
	e.addInstruction("INIR", []OperandType{}, 0xB2, 0xED, 16, encodeSimple)
	e.addInstruction("INDR", []OperandType{}, 0xBA, 0xED, 16, encodeSimple)
	e.addInstruction("OUTI", []OperandType{}, 0xA3, 0xED, 16, encodeSimple)
	e.addInstruction("OUTD", []OperandType{}, 0xAB, 0xED, 16, encodeSimple)
	e.addInstruction("OTIR", []OperandType{}, 0xB3, 0xED, 16, encodeSimple)
	e.addInstruction("OTDR", []OperandType{}, 0xBB, 0xED, 16, encodeSimple)
	
	// I/O instructions
	e.addInstruction("OUT", []OperandType{OperandIndirect, OperandRegister8}, 0xD3, 0x00, 11, encodeOUTnA)
	e.addInstruction("IN", []OperandType{OperandRegister8, OperandIndirect}, 0xDB, 0x00, 11, encodeINAn)
	
	// CB-prefixed instructions (from executeCB)
	e.addInstruction("RLC", []OperandType{OperandRegister8}, 0x00, 0xCB, 8, encodeCBr)
	e.addInstruction("RRC", []OperandType{OperandRegister8}, 0x08, 0xCB, 8, encodeCBr)
	e.addInstruction("RL", []OperandType{OperandRegister8}, 0x10, 0xCB, 8, encodeCBr)
	e.addInstruction("RR", []OperandType{OperandRegister8}, 0x18, 0xCB, 8, encodeCBr)
	e.addInstruction("SLA", []OperandType{OperandRegister8}, 0x20, 0xCB, 8, encodeCBr)
	e.addInstruction("SRA", []OperandType{OperandRegister8}, 0x28, 0xCB, 8, encodeCBr)
	e.addInstruction("SRL", []OperandType{OperandRegister8}, 0x38, 0xCB, 8, encodeCBr)
	e.addInstruction("BIT", []OperandType{OperandImmediate8, OperandRegister8}, 0x40, 0xCB, 8, encodeBITbr)
	e.addInstruction("RES", []OperandType{OperandImmediate8, OperandRegister8}, 0x80, 0xCB, 8, encodeRESbr)
	e.addInstruction("SET", []OperandType{OperandImmediate8, OperandRegister8}, 0xC0, 0xCB, 8, encodeSETbr)

	// CB-prefixed bit ops on (HL): reg code 6. base | (bit<<3) | 6
	e.addInstruction("BIT", []OperandType{OperandImmediate8, OperandIndirect}, 0x40, 0xCB, 12, encodeCBbHL)
	e.addInstruction("RES", []OperandType{OperandImmediate8, OperandIndirect}, 0x80, 0xCB, 15, encodeCBbHL)
	e.addInstruction("SET", []OperandType{OperandImmediate8, OperandIndirect}, 0xC0, 0xCB, 15, encodeCBbHL)
	// CB-prefixed rotates/shifts on (HL): base | 6
	e.addInstruction("RLC", []OperandType{OperandIndirect}, 0x00, 0xCB, 15, encodeCBHL)
	e.addInstruction("RRC", []OperandType{OperandIndirect}, 0x08, 0xCB, 15, encodeCBHL)
	e.addInstruction("RL", []OperandType{OperandIndirect}, 0x10, 0xCB, 15, encodeCBHL)
	e.addInstruction("RR", []OperandType{OperandIndirect}, 0x18, 0xCB, 15, encodeCBHL)
	e.addInstruction("SLA", []OperandType{OperandIndirect}, 0x20, 0xCB, 15, encodeCBHL)
	e.addInstruction("SRA", []OperandType{OperandIndirect}, 0x28, 0xCB, 15, encodeCBHL)
	e.addInstruction("SRL", []OperandType{OperandIndirect}, 0x38, 0xCB, 15, encodeCBHL)

	// Block transfer / search (ED-prefixed, no operands)
	e.addInstruction("LDIR", []OperandType{}, 0xB0, 0xED, 16, encodeSimple)
	e.addInstruction("LDDR", []OperandType{}, 0xB8, 0xED, 16, encodeSimple)
	e.addInstruction("LDI", []OperandType{}, 0xA0, 0xED, 16, encodeSimple)
	e.addInstruction("LDD", []OperandType{}, 0xA8, 0xED, 16, encodeSimple)
	e.addInstruction("CPIR", []OperandType{}, 0xB1, 0xED, 16, encodeSimple)
	e.addInstruction("CPDR", []OperandType{}, 0xB9, 0xED, 16, encodeSimple)
	e.addInstruction("CPI", []OperandType{}, 0xA1, 0xED, 16, encodeSimple)
	e.addInstruction("CPD", []OperandType{}, 0xA9, 0xED, 16, encodeSimple)

	// Misc ED-prefixed, no operands
	e.addInstruction("NEG", []OperandType{}, 0x44, 0xED, 8, encodeSimple)
	e.addInstruction("RETI", []OperandType{}, 0x4D, 0xED, 14, encodeSimple)
	e.addInstruction("RETN", []OperandType{}, 0x45, 0xED, 14, encodeSimple)
	e.addInstruction("RRD", []OperandType{}, 0x67, 0xED, 18, encodeSimple)
	e.addInstruction("RLD", []OperandType{}, 0x6F, 0xED, 18, encodeSimple)

	// Exchange / exchange-set (single unprefixed opcodes)
	e.addInstruction("EXX", []OperandType{}, 0xD9, 0x00, 4, encodeSimple)

	// Accumulator rotates (single-byte, no operands)
	e.addInstruction("RLCA", []OperandType{}, 0x07, 0x00, 4, encodeSimple)
	e.addInstruction("RRCA", []OperandType{}, 0x0F, 0x00, 4, encodeSimple)
	e.addInstruction("RLA", []OperandType{}, 0x17, 0x00, 4, encodeSimple)
	e.addInstruction("RRA", []OperandType{}, 0x1F, 0x00, 4, encodeSimple)

	// 16-bit ADC/SBC HL,rp (ED-prefixed): 0x4A|(p<<4) for ADC, 0x42|(p<<4) for SBC
	e.addInstruction("ADC", []OperandType{OperandRegister16, OperandRegister16}, 0x4A, 0xED, 15, encodeADC16)
	e.addInstruction("SBC", []OperandType{OperandRegister16, OperandRegister16}, 0x42, 0xED, 15, encodeSBC16)

	// EX DE,HL / EX AF,AF' / EX (SP),HL
	e.addInstruction("EX", []OperandType{OperandRegister16, OperandRegister16}, 0x00, 0x00, 4, encodeEX)
	e.addInstruction("EX", []OperandType{OperandIndirect, OperandRegister16}, 0xE3, 0x00, 19, encodeEXSPHL)

	// IM 0/1/2 (ED-prefixed)
	e.addInstruction("IM", []OperandType{OperandImmediate8}, 0x46, 0xED, 8, encodeIM)

	// RST n
	e.addInstruction("RST", []OperandType{OperandImmediate8}, 0xC7, 0x00, 11, encodeRST)
}

// addInstruction adds an instruction template to the encoder
func (e *Encoder) addInstruction(mnemonic string, operandTypes []OperandType, baseOpcode, prefix uint8, cycles int, encodeFunc func(*InstructionTemplate, []*ResolvedOperand) ([]uint8, error)) {
	template := &InstructionTemplate{
		Mnemonic:     mnemonic,
		OperandTypes: operandTypes,
		Encode:       encodeFunc,
		BaseOpcode:   baseOpcode,
		Prefix:       prefix,
		Cycles:       cycles,
	}
	
	e.instructions[mnemonic] = append(e.instructions[mnemonic], template)
}

// Encoding functions - these mirror the zen80 emulator's decoding patterns

// encodeSimple encodes instructions with no operands
func encodeSimple(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	if template.Prefix != 0 {
		return []uint8{template.Prefix, template.BaseOpcode}, nil
	}
	return []uint8{template.BaseOpcode}, nil
}

// encodeLDrr encodes LD r,r' - mirrors executeBlock1 pattern 01yyyzzz
func encodeLDrr(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	d, s := operands[0].Register, operands[1].Register

	// Interrupt-vector (I) and memory-refresh (R) register transfers are
	// ED-prefixed special cases: LD A,I / LD A,R / LD I,A / LD R,A. They are the
	// only valid uses of I and R as 8-bit operands.
	switch {
	case d == "A" && s == "I":
		return []uint8{0xED, 0x57}, nil
	case d == "A" && s == "R":
		return []uint8{0xED, 0x5F}, nil
	case d == "I" && s == "A":
		return []uint8{0xED, 0x47}, nil
	case d == "R" && s == "A":
		return []uint8{0xED, 0x4F}, nil
	}
	if d == "I" || d == "R" || s == "I" || s == "R" {
		return nil, fmt.Errorf("registers I and R may only be used as LD A,I / LD A,R / LD I,A / LD R,A")
	}

	dst := register8Map[d]
	src := register8Map[s]
	opcode := 0x40 | (dst << 3) | src
	return []uint8{opcode}, nil
}

// encodeLDrn encodes LD r,n - mirrors executeBlock0 pattern 00yyy110
func encodeLDrn(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	reg := register8Map[operands[0].Register]
	value := uint8(operands[1].Value)
	
	opcode := 0x06 | (reg << 3)
	return []uint8{opcode, value}, nil
}

// encodeLDrHL encodes LD r,(HL) - mirrors executeBlock1 pattern 01yyy110
func encodeLDrHL(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	if operands[1].Type != OperandIndirect {
		return nil, fmt.Errorf("expected indirect operand as source")
	}
	
	// Handle different indirect addressing modes
	if operands[1].Register == "HL" {
		// LD r,(HL)
		reg := register8Map[operands[0].Register]
		opcode := 0x46 | (reg << 3)
		return []uint8{opcode}, nil
	} else if operands[1].Register == "BC" && operands[0].Register == "A" {
		// LD A,(BC)
		return []uint8{0x0A}, nil
	} else if operands[1].Register == "DE" && operands[0].Register == "A" {
		// LD A,(DE)
		return []uint8{0x1A}, nil
	} else if operands[1].Register == "" {
		// LD A,(nn) - absolute address (no register means it's an address)
		if operands[0].Register != "A" {
			return nil, fmt.Errorf("only LD A,(nn) supported for absolute addressing")
		}
		addr := uint16(operands[1].Value)
		return []uint8{0x3A, uint8(addr), uint8(addr >> 8)}, nil
	} else if operands[1].Register == "IX" || operands[1].Register == "IY" {
		// LD r,(IX+d) / LD r,(IY+d): DD/FD prefix + (HL)-form opcode + displacement
		prefix := indexPrefix(operands[1].Register)
		reg := register8Map[operands[0].Register]
		opcode := 0x46 | (reg << 3)
		return []uint8{prefix, opcode, byte(int8(operands[1].Displacement))}, nil
	}
	
	return nil, fmt.Errorf("unsupported indirect addressing mode: (%s)", operands[1].Register)
}

// indexPrefix returns the DD/FD prefix byte for an IX/IY indexed operand.
func indexPrefix(reg string) uint8 {
	if reg == "IY" {
		return 0xFD
	}
	return 0xDD
}

// applyHalfIndexPrefix prepends the DD/FD prefix for the undocumented half-index
// registers (IXH/IXL/IYH/IYL). These encode as the corresponding H/L register
// instruction (register8Map already maps them to codes 4/5) plus a DD (IX) or FD
// (IY) prefix.
//
// The prefix governs the entire instruction, which constrains what is legal:
//   - All index halves in one instruction must be the same index register; you
//     cannot mix an IX half and an IY half (LD IXH,IYL is impossible - it would
//     need two prefixes).
//   - A half register cannot coexist with an indexed memory operand (IX+d)/(IY+d)
//     in the same instruction, since the prefix can only mean one thing.
// Both illegal cases are rejected rather than silently producing a different
// instruction.
func applyHalfIndexPrefix(code []uint8, operands []*ResolvedOperand) ([]uint8, error) {
	var prefix uint8
	var sawHalf, sawIndexedMem bool
	for _, op := range operands {
		if op.Type == OperandIndirect && (op.Register == "IX" || op.Register == "IY") {
			sawIndexedMem = true
			continue
		}
		if op.Type != OperandRegister8 {
			continue
		}
		var p uint8
		switch op.Register {
		case "IXH", "IXL":
			p = 0xDD
		case "IYH", "IYL":
			p = 0xFD
		default:
			continue
		}
		if prefix != 0 && prefix != p {
			return nil, fmt.Errorf("cannot mix IX and IY half registers in one instruction")
		}
		prefix = p
		sawHalf = true
	}
	if prefix == 0 {
		return code, nil
	}
	if sawHalf && sawIndexedMem {
		return nil, fmt.Errorf("cannot combine an index half register with an (IX+d)/(IY+d) operand")
	}
	if len(code) > 0 && (code[0] == 0xDD || code[0] == 0xFD) {
		return code, nil // already prefixed (the indexed-operand encoder added it)
	}
	return append([]uint8{prefix}, code...), nil
}
func encodeLDHLr(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	if operands[0].Type != OperandIndirect {
		return nil, fmt.Errorf("expected indirect operand as destination")
	}
	
	// Handle different indirect addressing modes
	if operands[0].Register == "HL" {
		// LD (HL),r
		reg := register8Map[operands[1].Register]
		opcode := 0x70 | reg
		return []uint8{opcode}, nil
	} else if operands[0].Register == "BC" && operands[1].Register == "A" {
		// LD (BC),A
		return []uint8{0x02}, nil
	} else if operands[0].Register == "DE" && operands[1].Register == "A" {
		// LD (DE),A
		return []uint8{0x12}, nil
	} else if operands[0].Register == "" {
		// LD (nn),A - absolute address (no register means it's an address)
		if operands[1].Register != "A" {
			return nil, fmt.Errorf("only LD (nn),A supported for absolute addressing")
		}
		addr := uint16(operands[0].Value)
		return []uint8{0x32, uint8(addr), uint8(addr >> 8)}, nil
	} else if operands[0].Register == "IX" || operands[0].Register == "IY" {
		// LD (IX+d),r / LD (IY+d),r: DD/FD prefix + (HL)-form opcode + displacement
		prefix := indexPrefix(operands[0].Register)
		reg := register8Map[operands[1].Register]
		opcode := 0x70 | reg
		return []uint8{prefix, opcode, byte(int8(operands[0].Displacement))}, nil
	}
	
	return nil, fmt.Errorf("unsupported indirect addressing mode: (%s)", operands[0].Register)
}
func encodeLDHLn(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	dst := operands[0]
	value := uint8(operands[1].Value)
	switch dst.Register {
	case "HL":
		return []uint8{0x36, value}, nil
	case "IX", "IY":
		return []uint8{indexPrefix(dst.Register), 0x36, byte(int8(dst.Displacement)), value}, nil
	}
	return nil, fmt.Errorf("expected (HL), (IX+d), or (IY+d) as destination")
}

// encodeALUr encodes ALU operations with register - mirrors executeBlock2 pattern 10yyyzzz
func encodeALUr(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	var reg uint8
	
	if len(operands) == 1 {
		// SUB r, AND r, XOR r, OR r, CP r
		reg = register8Map[operands[0].Register]
	} else if len(operands) == 2 {
		// ADD A,r; ADC A,r; SBC A,r
		if operands[0].Register != "A" {
			return nil, fmt.Errorf("first operand must be A for %s", template.Mnemonic)
		}
		reg = register8Map[operands[1].Register]
	} else {
		return nil, fmt.Errorf("invalid number of operands for ALU instruction")
	}
	
	opcode := template.BaseOpcode | reg
	return []uint8{opcode}, nil
}

// encodeALUn encodes ALU operations with immediate value
func encodeALUn(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	var value uint8
	
	if len(operands) == 1 {
		// SUB n, AND n, XOR n, OR n, CP n
		value = uint8(operands[0].Value)
	} else if len(operands) == 2 {
		// ADD A,n; ADC A,n; SBC A,n
		if operands[0].Register != "A" {
			return nil, fmt.Errorf("first operand must be A for %s", template.Mnemonic)
		}
		value = uint8(operands[1].Value)
	} else {
		return nil, fmt.Errorf("invalid number of operands for ALU instruction")
	}
	
	return []uint8{template.BaseOpcode, value}, nil
}

// encodeALUIndirect encodes ALU ops on (HL)/(IX+d)/(IY+d). The indirect operand
// is the last one (CP (HL): one operand; ADD A,(HL): two). reg code 6 selects
// the (HL) form; for IX/IY a DD/FD prefix and displacement byte are added.
func encodeALUIndirect(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	ind := operands[len(operands)-1]
	if ind.Type != OperandIndirect {
		return nil, fmt.Errorf("expected indirect operand for %s", template.Mnemonic)
	}
	if len(operands) == 2 && operands[0].Register != "A" {
		return nil, fmt.Errorf("first operand must be A for %s", template.Mnemonic)
	}
	opcode := template.BaseOpcode | 6
	switch ind.Register {
	case "HL":
		return []uint8{opcode}, nil
	case "IX", "IY":
		return []uint8{indexPrefix(ind.Register), opcode, byte(int8(ind.Displacement))}, nil
	}
	return nil, fmt.Errorf("unsupported indirect operand for %s: (%s)", template.Mnemonic, ind.Register)
}

// encodeINCDECIndirect encodes INC/DEC on (HL)/(IX+d)/(IY+d).
func encodeINCDECIndirect(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	ind := operands[0]
	if ind.Type != OperandIndirect {
		return nil, fmt.Errorf("expected indirect operand for %s", template.Mnemonic)
	}
	switch ind.Register {
	case "HL":
		return []uint8{template.BaseOpcode}, nil
	case "IX", "IY":
		return []uint8{indexPrefix(ind.Register), template.BaseOpcode, byte(int8(ind.Displacement))}, nil
	}
	return nil, fmt.Errorf("unsupported indirect operand for %s: (%s)", template.Mnemonic, ind.Register)
}

// encodeINCr encodes INC r - mirrors executeBlock0 pattern 00yyy100
func encodeINCr(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	reg := register8Map[operands[0].Register]
	opcode := 0x04 | (reg << 3)
	return []uint8{opcode}, nil
}

// encodeDECr encodes DEC r - mirrors executeBlock0 pattern 00yyy101
func encodeDECr(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	reg := register8Map[operands[0].Register]
	opcode := 0x05 | (reg << 3)
	return []uint8{opcode}, nil
}

// encodeLDrpnn encodes LD rp,nn - mirrors executeBlock0 pattern 00pp0001
func encodeLDrpnn(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	dst := operands[0].Register
	value := uint16(operands[1].Value)
	rp := register16Map[dst]
	opcode := 0x01 | (rp << 4)
	if dst == "IX" || dst == "IY" {
		// LD IX,nn / LD IY,nn: DD/FD prefix + the LD HL,nn opcode (0x21).
		return []uint8{indexPrefix(dst), opcode, uint8(value), uint8(value >> 8)}, nil
	}
	return []uint8{opcode, uint8(value), uint8(value >> 8)}, nil
}

// encodeADDHLrp encodes ADD HL,rp - mirrors executeBlock0 pattern 00pp1001
func encodeADDHLrp(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	dst := operands[0].Register
	if dst != "HL" && dst != "IX" && dst != "IY" {
		return nil, fmt.Errorf("first operand must be HL, IX, or IY")
	}
	src := operands[1].Register
	if dst == "IX" || dst == "IY" {
		// ADD IX,rp: DD/FD prefix + 0x09|(p<<4). The source pair uses HL's code
		// (2) when it is the index register itself (ADD IX,IX), otherwise its
		// normal code. ZX Opal uses ADD IX,BC/DE/SP.
		var rp uint8
		if src == dst {
			rp = 2
		} else if src == "IX" || src == "IY" {
			return nil, fmt.Errorf("cannot mix IX and IY in ADD")
		} else {
			rp = register16Map[src]
		}
		return []uint8{indexPrefix(dst), 0x09 | (rp << 4)}, nil
	}
	rp := register16Map[src]
	return []uint8{0x09 | (rp << 4)}, nil
}

// encodeINCrp encodes INC rp - mirrors executeBlock0 pattern 00pp0011
func encodeINCrp(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	r := operands[0].Register
	rp := register16Map[r]
	if r == "IX" || r == "IY" {
		return []uint8{indexPrefix(r), 0x03 | (rp << 4)}, nil
	}
	opcode := 0x03 | (rp << 4)
	return []uint8{opcode}, nil
}

// encodeDECrp encodes DEC rp - mirrors executeBlock0 pattern 00pp1011
func encodeDECrp(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	r := operands[0].Register
	rp := register16Map[r]
	if r == "IX" || r == "IY" {
		return []uint8{indexPrefix(r), 0x0B | (rp << 4)}, nil
	}
	opcode := 0x0B | (rp << 4)
	return []uint8{opcode}, nil
}

// encodePUSHrp encodes PUSH rp - mirrors executeBlock0 pattern 11pp0101 (ADDED)
func encodePUSHrp(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	if r := operands[0].Register; r == "IX" || r == "IY" {
		return []uint8{indexPrefix(r), 0xE5}, nil
	}
	rp, exists := stackRegisterMap[operands[0].Register]
	if !exists {
		return nil, fmt.Errorf("invalid register for PUSH: %s", operands[0].Register)
	}
	
	opcode := 0xC5 | (rp << 4)
	return []uint8{opcode}, nil
}

// encodePOPrp encodes POP rp - mirrors executeBlock0 pattern 11pp0001 (ADDED)
func encodePOPrp(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	if r := operands[0].Register; r == "IX" || r == "IY" {
		return []uint8{indexPrefix(r), 0xE1}, nil
	}
	rp, exists := stackRegisterMap[operands[0].Register]
	if !exists {
		return nil, fmt.Errorf("invalid register for POP: %s", operands[0].Register)
	}
	
	opcode := 0xC1 | (rp << 4)
	return []uint8{opcode}, nil
}

// encodeLDrpaddr encodes LD rp,(nn) - load 16-bit register from memory
func encodeLDrpaddr(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	reg := operands[0].Register
	addr := uint16(operands[1].Value)
	
	if reg == "HL" {
		// LD HL,(nn) - 0x2A
		return []uint8{0x2A, uint8(addr), uint8(addr >> 8)}, nil
	} else {
		// LD BC,(nn), LD DE,(nn), LD SP,(nn) - need ED prefix
		var subOpcode uint8
		switch reg {
		case "BC":
			subOpcode = 0x4B
		case "DE":
			subOpcode = 0x5B
		case "SP":
			subOpcode = 0x7B
		default:
			return nil, fmt.Errorf("unsupported register for LD rp,(nn): %s", reg)
		}
		return []uint8{0xED, subOpcode, uint8(addr), uint8(addr >> 8)}, nil
	}
}

// encodeLDaddrrp encodes LD (nn),rp - store 16-bit register to memory  
func encodeLDaddrrp(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	addr := uint16(operands[0].Value)
	reg := operands[1].Register
	
	if reg == "HL" {
		// LD (nn),HL - 0x22
		return []uint8{0x22, uint8(addr), uint8(addr >> 8)}, nil
	} else {
		// LD (nn),BC, LD (nn),DE, LD (nn),SP - need ED prefix
		var subOpcode uint8
		switch reg {
		case "BC":
			subOpcode = 0x43
		case "DE":
			subOpcode = 0x53
		case "SP":
			subOpcode = 0x73
		default:
			return nil, fmt.Errorf("unsupported register for LD (nn),rp: %s", reg)
		}
		return []uint8{0xED, subOpcode, uint8(addr), uint8(addr >> 8)}, nil
	}
}

// encodeLDSPHL encodes LD SP,HL - special case for loading SP from HL
func encodeLDSPHL(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	if operands[0].Register != "SP" {
		return nil, fmt.Errorf("LD rp,rp only supports SP as destination here")
	}
	switch operands[1].Register {
	case "HL":
		return []uint8{0xF9}, nil
	case "IX", "IY":
		// LD SP,IX / LD SP,IY
		return []uint8{indexPrefix(operands[1].Register), 0xF9}, nil
	}
	return nil, fmt.Errorf("unsupported LD SP,%s", operands[1].Register)
}

// encodeJPnn encodes JP nn
func encodeJPnn(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	addr := uint16(operands[0].Value)
	return []uint8{0xC3, uint8(addr), uint8(addr >> 8)}, nil
}

// encodeJPccnn encodes JP cc,nn - mirrors executeBlock3 pattern 11ccc010
func encodeJPccnn(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	if len(operands) != 2 {
		return nil, fmt.Errorf("JP cc,nn requires exactly 2 operands")
	}
	
	if operands[0].Type != OperandCondition {
		return nil, fmt.Errorf("first operand must be a condition code")
	}
	
	cc := conditionMap[operands[0].Condition]
	addr := uint16(operands[1].Value)
	
	opcode := 0xC2 | (cc << 3)
	return []uint8{opcode, uint8(addr), uint8(addr >> 8)}, nil
}

// encodeJRd encodes JR d
func encodeJRd(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	disp := int8(operands[0].Value)
	return []uint8{0x18, uint8(disp)}, nil
}

// encodeJRccd encodes JR cc,d - mirrors executeBlock0 pattern 001cc000
func encodeJRccd(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	// JR only supports NZ, Z, NC, C conditions
	condMap := map[string]uint8{"NZ": 0, "Z": 1, "NC": 2, "C": 3}
	cc, exists := condMap[operands[0].Condition]
	if !exists {
		return nil, fmt.Errorf("invalid condition for JR: %s", operands[0].Condition)
	}
	
	disp := int8(operands[1].Value)
	opcode := 0x20 | (cc << 3)
	return []uint8{opcode, uint8(disp)}, nil
}

// encodeDJNZ encodes DJNZ d - decrement B and jump if not zero
func encodeDJNZ(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	disp := int8(operands[0].Value)
	return []uint8{0x10, uint8(disp)}, nil
}

// encodeCALLnn encodes CALL nn
func encodeCALLnn(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	addr := uint16(operands[0].Value)
	return []uint8{0xCD, uint8(addr), uint8(addr >> 8)}, nil
}

// encodeCALLccnn encodes CALL cc,nn - 11ccc100
func encodeCALLccnn(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	cc := conditionMap[operands[0].Condition]
	addr := uint16(operands[1].Value)
	opcode := 0xC4 | (cc << 3)
	return []uint8{opcode, uint8(addr), uint8(addr >> 8)}, nil
}

// encodeJPindirect encodes JP (HL) / JP (IX) / JP (IY)
func encodeJPindirect(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	switch operands[0].Register {
	case "HL":
		return []uint8{0xE9}, nil
	case "IX", "IY":
		return []uint8{indexPrefix(operands[0].Register), 0xE9}, nil
	}
	return nil, fmt.Errorf("JP only supports (HL), (IX), (IY) as indirect targets")
}

// encodeRETcc encodes RET cc - mirrors executeBlock3 pattern 11ccc000
func encodeRETcc(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	cc := conditionMap[operands[0].Condition]
	opcode := 0xC0 | (cc << 3)
	return []uint8{opcode}, nil
}

// CB-prefixed instruction encoders

// encodeCBr encodes CB r instructions
func encodeCBr(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	reg := register8Map[operands[0].Register]
	cbOpcode := template.BaseOpcode | reg
	return []uint8{0xCB, cbOpcode}, nil
}

// encodeBITbr encodes BIT b,r
func encodeBITbr(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	bit := uint8(operands[0].Value)
	if bit > 7 {
		return nil, fmt.Errorf("bit number must be 0-7")
	}
	
	reg := register8Map[operands[1].Register]
	cbOpcode := 0x40 | (bit << 3) | reg
	return []uint8{0xCB, cbOpcode}, nil
}

// encodeRESbr encodes RES b,r
func encodeRESbr(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	bit := uint8(operands[0].Value)
	if bit > 7 {
		return nil, fmt.Errorf("bit number must be 0-7")
	}
	
	reg := register8Map[operands[1].Register]
	cbOpcode := 0x80 | (bit << 3) | reg
	return []uint8{0xCB, cbOpcode}, nil
}

// encodeOUTnA encodes OUT (n),A
func encodeOUTnA(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	if operands[0].Type != OperandIndirect {
		return nil, fmt.Errorf("expected indirect operand as port")
	}
	// OUT (C),r: ED-prefixed, 0x41 | (r<<3). The register here is the source.
	if operands[0].Register == "C" {
		reg := register8Map[operands[1].Register]
		return []uint8{0xED, 0x41 | (reg << 3)}, nil
	}
	// OUT (n),A: only A is valid for the immediate-port form.
	if operands[1].Register != "A" {
		return nil, fmt.Errorf("second operand must be A for OUT (n),A")
	}
	port := uint8(operands[0].Value)
	return []uint8{0xD3, port}, nil
}

// encodeINAn encodes IN A,(n)
func encodeINAn(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	if operands[1].Type != OperandIndirect {
		return nil, fmt.Errorf("expected indirect operand as port")
	}
	// IN r,(C): ED-prefixed, 0x40 | (r<<3). The register here is the destination.
	if operands[1].Register == "C" {
		reg := register8Map[operands[0].Register]
		return []uint8{0xED, 0x40 | (reg << 3)}, nil
	}
	// IN A,(n): only A is valid for the immediate-port form.
	if operands[0].Register != "A" {
		return nil, fmt.Errorf("first operand must be A for IN A,(n)")
	}
	port := uint8(operands[1].Value)
	return []uint8{0xDB, port}, nil
}

// encodeSETbr encodes SET b,r
func encodeSETbr(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	bit := uint8(operands[0].Value)
	if bit > 7 {
		return nil, fmt.Errorf("bit number must be 0-7")
	}
	
	reg := register8Map[operands[1].Register]
	cbOpcode := 0xC0 | (bit << 3) | reg
	return []uint8{0xCB, cbOpcode}, nil
}

// encodeCBHL encodes CB-prefixed rotate/shift on (HL): 0xCB, base|6
func encodeCBHL(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	if operands[0].Register != "HL" {
		return nil, fmt.Errorf("expected (HL)")
	}
	return []uint8{0xCB, template.BaseOpcode | 6}, nil
}

// encodeCBbHL encodes CB-prefixed BIT/RES/SET n,(HL): 0xCB, base|(bit<<3)|6
func encodeCBbHL(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	bit := uint8(operands[0].Value)
	if bit > 7 {
		return nil, fmt.Errorf("bit number must be 0-7")
	}
	if operands[1].Register != "HL" {
		return nil, fmt.Errorf("expected (HL)")
	}
	return []uint8{0xCB, template.BaseOpcode | (bit << 3) | 6}, nil
}

// encodeADC16 encodes ADC HL,rp (ED-prefixed): 0xED, 0x4A|(p<<4)
func encodeADC16(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	if operands[0].Register != "HL" {
		return nil, fmt.Errorf("16-bit ADC requires HL as the destination")
	}
	rp, ok := register16Map[operands[1].Register]
	if !ok {
		return nil, fmt.Errorf("invalid source register for ADC HL,rp")
	}
	return []uint8{0xED, 0x4A | (rp << 4)}, nil
}

// encodeSBC16 encodes SBC HL,rp (ED-prefixed): 0xED, 0x42|(p<<4)
func encodeSBC16(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	if operands[0].Register != "HL" {
		return nil, fmt.Errorf("16-bit SBC requires HL as the destination")
	}
	rp, ok := register16Map[operands[1].Register]
	if !ok {
		return nil, fmt.Errorf("invalid source register for SBC HL,rp")
	}
	return []uint8{0xED, 0x42 | (rp << 4)}, nil
}

// encodeEX encodes EX DE,HL (0xEB) and EX AF,AF' (0x08)
func encodeEX(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	a, b := operands[0].Register, operands[1].Register
	switch {
	case a == "DE" && b == "HL":
		return []uint8{0xEB}, nil
	case a == "AF" && (b == "AF" || b == "AF'"):
		return []uint8{0x08}, nil
	default:
		return nil, fmt.Errorf("unsupported EX operands: %s,%s", a, b)
	}
}

// encodeEXSPHL encodes EX (SP),HL (0xE3)
func encodeEXSPHL(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	if operands[0].Register != "SP" || operands[1].Register != "HL" {
		return nil, fmt.Errorf("only EX (SP),HL is supported in this form")
	}
	return []uint8{0xE3}, nil
}

// encodeIM encodes IM 0/1/2 (ED-prefixed): 0xED then 0x46/0x56/0x5E
func encodeIM(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	switch operands[0].Value {
	case 0:
		return []uint8{0xED, 0x46}, nil
	case 1:
		return []uint8{0xED, 0x56}, nil
	case 2:
		return []uint8{0xED, 0x5E}, nil
	default:
		return nil, fmt.Errorf("IM mode must be 0, 1, or 2")
	}
}

// encodeRST encodes RST n: 0xC7 | n, where n is one of 0x00,0x08,...,0x38
func encodeRST(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	n := operands[0].Value
	if n < 0 || n > 0x38 || (n&0x07) != 0 {
		return nil, fmt.Errorf("RST target must be one of 00h,08h,10h,18h,20h,28h,30h,38h")
	}
	return []uint8{0xC7 | uint8(n)}, nil
}

// GetInstructionInfo returns information about available instructions
func (e *Encoder) GetInstructionInfo(mnemonic string) []*InstructionTemplate {
	return e.instructions[strings.ToUpper(mnemonic)]
}

// IsInstruction reports whether a mnemonic is a real Z80/Z80N instruction.
// Used by macro resolution so a bare name that is an instruction is never
// shadowed by a packaged macro of the same name (the macro must be qualified).
func (e *Encoder) IsInstruction(mnemonic string) bool {
	_, exists := e.instructions[strings.ToUpper(mnemonic)]
	return exists
}

// GetAllInstructions returns all available instruction mnemonics
func (e *Encoder) GetAllInstructions() []string {
	var mnemonics []string
	for mnemonic := range e.instructions {
		mnemonics = append(mnemonics, mnemonic)
	}
	return mnemonics
}
// ---------------------------------------------------------------------------
// Z80N (ZX Spectrum Next) encoders. All opcodes verified against the official
// SpecNext reference (docs/Z80N_REFERENCE.md). Every form is ED-prefixed.
// ---------------------------------------------------------------------------

// encodeZ80NFixedReg returns an encoder for a one-operand instruction whose
// operand is hard-wired to a specific 8-bit register (e.g. MIRROR A).
func encodeZ80NFixedReg(want string) func(*InstructionTemplate, []*ResolvedOperand) ([]uint8, error) {
	return func(t *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
		if len(operands) != 1 || operands[0].Register != want {
			return nil, fmt.Errorf("%s takes only the operand %s", t.Mnemonic, want)
		}
		return []uint8{t.Prefix, t.BaseOpcode}, nil
	}
}

// encodeZ80NFixedRegPair returns an encoder for a two-operand instruction whose
// operands are hard-wired (e.g. MUL D,E or BSLA DE,B).
func encodeZ80NFixedRegPair(a, b string) func(*InstructionTemplate, []*ResolvedOperand) ([]uint8, error) {
	return func(t *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
		if len(operands) != 2 || operands[0].Register != a || operands[1].Register != b {
			return nil, fmt.Errorf("%s takes only the operands %s,%s", t.Mnemonic, a, b)
		}
		return []uint8{t.Prefix, t.BaseOpcode}, nil
	}
}

// encodeZ80NJPc encodes the Z80N JP (C) (ED 98). The operand parses as an
// indirect with register "C"; any other indirect is rejected so JP (HL)/(IX)/(IY)
// continue to be handled by the base encoder.
func encodeZ80NJPc(t *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	if len(operands) != 1 || operands[0].Register != "C" {
		return nil, fmt.Errorf("expected JP (C)")
	}
	return []uint8{0xED, 0x98}, nil
}

// encodeZ80NTEST encodes TEST n (ED 27 n).
func encodeZ80NTEST(t *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	return []uint8{0xED, 0x27, uint8(operands[0].Value)}, nil
}

// encodeZ80NADDrrA encodes ADD HL,A / ADD DE,A / ADD BC,A (ED 31/32/33).
func encodeZ80NADDrrA(t *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	if operands[1].Register != "A" {
		return nil, fmt.Errorf("ADD %s,%s is not a Z80N instruction", operands[0].Register, operands[1].Register)
	}
	var op uint8
	switch operands[0].Register {
	case "HL":
		op = 0x31
	case "DE":
		op = 0x32
	case "BC":
		op = 0x33
	default:
		return nil, fmt.Errorf("ADD %s,A: only HL, DE, BC are valid", operands[0].Register)
	}
	return []uint8{0xED, op}, nil
}

// encodeZ80NADDrrnn encodes ADD HL,nn / ADD DE,nn / ADD BC,nn (ED 34/35/36).
// The 16-bit immediate is little-endian (unlike PUSH nn).
func encodeZ80NADDrrnn(t *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	var op uint8
	switch operands[0].Register {
	case "HL":
		op = 0x34
	case "DE":
		op = 0x35
	case "BC":
		op = 0x36
	default:
		return nil, fmt.Errorf("ADD %s,nn: only HL, DE, BC are valid", operands[0].Register)
	}
	v := uint16(operands[1].Value)
	return []uint8{0xED, op, uint8(v), uint8(v >> 8)}, nil
}

// encodeZ80NPUSHnn encodes PUSH nn (ED 8A hi lo). This is the only Z80 operand
// encoded BIG-endian: the high byte precedes the low byte.
func encodeZ80NPUSHnn(t *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	v := uint16(operands[0].Value)
	return []uint8{0xED, 0x8A, uint8(v >> 8), uint8(v)}, nil
}

// encodeZ80NNEXTREGnn encodes NEXTREG n,n (ED 91 reg val).
func encodeZ80NNEXTREGnn(t *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	return []uint8{0xED, 0x91, uint8(operands[0].Value), uint8(operands[1].Value)}, nil
}

// encodeZ80NNEXTREGnA encodes NEXTREG n,A (ED 92 reg); the value comes from A.
func encodeZ80NNEXTREGnA(t *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	if operands[1].Register != "A" {
		return nil, fmt.Errorf("NEXTREG n,%s: second operand must be an immediate or A", operands[1].Register)
	}
	return []uint8{0xED, 0x92, uint8(operands[0].Value)}, nil
}
