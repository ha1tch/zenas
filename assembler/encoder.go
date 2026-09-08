package assembler

import (
	"fmt"
	"strings"

	"github.com/ha1tch/zenas/pkg/z80enc"
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
	// CyclesFunc, when set, computes the real T-state count from the
	// actual resolved operands, overriding Cycles. Needed wherever a
	// single (mnemonic, operand-types) template covers forms whose
	// real timing differs by which specific register is used -- (HL)
	// vs (IX+d)/(IY+d) is the largest such family (indexed addressing
	// costs the extra DD/FD prefix and displacement fetch), but it
	// also covers plain vs half-index 8-bit registers, BC/DE/HL/SP vs
	// IX/IY for 16-bit loads, and JP (HL) vs JP (IX)/(IY). Cycles
	// alone cannot represent these correctly since one template can
	// match several real instructions with different true timings;
	// use CyclesFor to read the right value regardless of which field
	// is actually populated. Found and fixed as part of a full
	// cross-check against a reference Z80 timing table (25 wrong
	// values from this exact class of sharing), not written in
	// anticipation -- most templates never needed this and still
	// don't.
	CyclesFunc func([]*ResolvedOperand) int
}

// CyclesFor returns the correct T-state count for this template given
// the actual operands: CyclesFunc's result if set, otherwise the
// fixed Cycles value. This is the field to read, not Cycles directly,
// since Cycles alone is wrong for every template that has a
// CyclesFunc.
func (t *InstructionTemplate) CyclesFor(operands []*ResolvedOperand) int {
	if t.CyclesFunc != nil {
		return t.CyclesFunc(operands)
	}
	return t.Cycles
}

// isHalfIndexReg reports whether name is one of the undocumented
// half-index registers (IXH/IXL/IYH/IYL), which every 8-bit-register
// timing table entry treats as a separate, slower case from the plain
// 8-bit registers -- shared by several CyclesFunc implementations
// below rather than repeated in each.
func isHalfIndexReg(name string) bool {
	switch name {
	case "IXH", "IXL", "IYH", "IYL":
		return true
	}
	return false
}

// isIndexReg16 reports whether name is IX or IY, the two 16-bit index
// registers that cost extra T-states relative to the general-purpose
// pairs (BC/DE/HL/SP) in most 16-bit-register instruction shapes --
// shared by several CyclesFunc implementations below.
func isIndexReg16(name string) bool {
	return name == "IX" || name == "IY"
}

// aluRegCycles is the shared CyclesFunc for the 8 ALU-on-register
// registrations (ADD/ADC/SBC A,r and SUB/AND/XOR/OR/CP r): 4 for a
// plain register, 8 for a half-index register. The last operand is
// always the register operand, regardless of whether A was written
// explicitly (2-operand ADD/ADC/SBC form) or implied (1-operand
// SUB/AND/XOR/OR/CP form).
func aluRegCycles(ops []*ResolvedOperand) int {
	if isHalfIndexReg(ops[len(ops)-1].Register) {
		return 8
	}
	return 4
}

// aluIndirectCycles is the shared CyclesFunc for the 8 ALU-on-indirect
// registrations: 7 for (HL), 19 for (IX+d)/(IY+d). The last operand
// is always the indirect operand.
func aluIndirectCycles(ops []*ResolvedOperand) int {
	if isIndexReg16(ops[len(ops)-1].Register) {
		return 19
	}
	return 7
}

// incDecIndirectCycles: 11 for INC/DEC (HL), 23 for INC/DEC (IX+d)/(IY+d).
func incDecIndirectCycles(ops []*ResolvedOperand) int {
	if isIndexReg16(ops[0].Register) {
		return 23
	}
	return 11
}

// incDecRegCycles: 4 for a plain register, 8 for a half-index register.
func incDecRegCycles(ops []*ResolvedOperand) int {
	if isHalfIndexReg(ops[0].Register) {
		return 8
	}
	return 4
}

// incDecRp16Cycles: 6 for BC/DE/HL/SP, 10 for IX/IY.
func incDecRp16Cycles(ops []*ResolvedOperand) int {
	if isIndexReg16(ops[0].Register) {
		return 10
	}
	return 6
}

// cbIndirectCycles: 15 for (HL), 23 for (IX+d)/(IY+d) -- shared by
// RES/SET b,(...) and the rotate/shift-on-indirect family, which have
// the same timing. The indirect operand is always last, whether the
// template has one operand (rotate/shift) or two (RES/SET, with the
// bit number first).
func cbIndirectCycles(ops []*ResolvedOperand) int {
	if isIndexReg16(ops[len(ops)-1].Register) {
		return 23
	}
	return 15
}

// cbBitIndirectCycles: 12 for BIT b,(HL), 20 for BIT b,(IX+d)/(IY+d).
func cbBitIndirectCycles(ops []*ResolvedOperand) int {
	if isIndexReg16(ops[len(ops)-1].Register) {
		return 20
	}
	return 12
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
		cycles   int
	}{
		{"SWAPNIB", 0x23, 8}, {"OUTINB", 0x90, 16},
		{"PIXELDN", 0x93, 8}, {"PIXELAD", 0x94, 8}, {"SETAE", 0x95, 8},
		{"LDIX", 0xA4, 16}, {"LDWS", 0xA5, 14}, {"LDDX", 0xAC, 16},
		// LDIRX/LDIRSCALE/LDPIRX/LDDRX repeat; 21 is the repeat count,
		// matching the same convention LDIR/CPIR/INIR/OTIR use (the
		// last-iteration value, 16, is shorter but not what a single
		// Cycles field reports here). LDIRSCALE (ED B6) was entirely
		// missing before this fix -- confirmed absent from this
		// codebase, not a design decision -- despite decoding as a
		// real, documented instruction (a plain repeating copy; the
		// scaling the name suggests is not actually present in the
		// silicon this assembler targets).
		{"LDIRX", 0xB4, 21}, {"LDIRSCALE", 0xB6, 21}, {"LDPIRX", 0xB7, 21}, {"LDDRX", 0xBC, 21},
	}
	for _, s := range simpleN {
		e.addInstruction(s.mnemonic, []OperandType{}, s.opcode, 0xED, s.cycles, encodeSimple)
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
// EncodeWithCycles behaves exactly like Encode, but also returns the
// real T-state count for the specific operands given -- whichever
// value CyclesFor returns for the template that actually matched.
// This is the only way to observe accurate cycle data from outside
// this package: Encode itself only returns bytes, and which template
// matched (needed to call CyclesFor at all) isn't otherwise exposed.
func (e *Encoder) EncodeWithCycles(mnemonic string, operands []*ResolvedOperand) ([]uint8, int, error) {
	templates, exists := e.instructions[strings.ToUpper(mnemonic)]
	if !exists {
		return nil, 0, fmt.Errorf("unknown instruction: %s", mnemonic)
	}

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
			finalCode, err := applyHalfIndexPrefix(code, operands)
			if err != nil {
				return nil, 0, err
			}
			return finalCode, template.CyclesFor(operands), nil
		}
	}
	// No template matched (or all matched-but-failed) -- reuse Encode's
	// own detailed error-message construction rather than duplicating it.
	_, err := e.Encode(mnemonic, operands)
	return nil, 0, err
}

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
	e.addInstructionVarCycles("LD", []OperandType{OperandRegister8, OperandRegister8}, 0x40, 0x00, func(ops []*ResolvedOperand) int {
		a, b := ops[0].Register, ops[1].Register
		if a == "I" || a == "R" || b == "I" || b == "R" {
			return 9 // LD A,I / LD A,R / LD I,A / LD R,A -- ED-prefixed
		}
		if isHalfIndexReg(a) || isHalfIndexReg(b) {
			return 8 // LD IXh,p / LD p,IXh -- the prefix costs 4 extra T-states
		}
		return 4
	}, encodeLDrr)
	e.addInstructionVarCycles("LD", []OperandType{OperandRegister8, OperandImmediate8}, 0x06, 0x00, func(ops []*ResolvedOperand) int {
		if isHalfIndexReg(ops[0].Register) {
			return 11 // LD IXh,n
		}
		return 7
	}, encodeLDrn)
	e.addInstructionVarCycles("LD", []OperandType{OperandRegister8, OperandIndirect}, 0x46, 0x00, func(ops []*ResolvedOperand) int {
		switch ops[1].Register {
		case "IX", "IY":
			return 19 // LD r,(IX+d)
		case "":
			return 13 // LD A,(nn)
		default:
			return 7 // LD r,(HL) / LD A,(BC) / LD A,(DE)
		}
	}, encodeLDrHL)
	e.addInstructionVarCycles("LD", []OperandType{OperandIndirect, OperandRegister8}, 0x70, 0x00, func(ops []*ResolvedOperand) int {
		switch ops[0].Register {
		case "IX", "IY":
			return 19 // LD (IX+d),r
		case "":
			return 13 // LD (nn),A
		default:
			return 7 // LD (HL),r / LD (BC),A / LD (DE),A
		}
	}, encodeLDHLr)
	e.addInstructionVarCycles("LD", []OperandType{OperandIndirect, OperandImmediate8}, 0x36, 0x00, func(ops []*ResolvedOperand) int {
		if isIndexReg16(ops[0].Register) {
			return 19 // LD (IX+d),n
		}
		return 10 // LD (HL),n
	}, encodeLDHLn)

	// Extended 16-bit loads
	e.addInstructionVarCycles("LD", []OperandType{OperandRegister16, OperandImmediate16}, 0x01, 0x00, func(ops []*ResolvedOperand) int {
		if isIndexReg16(ops[0].Register) {
			return 14 // LD IX/IY,nn
		}
		return 10 // LD BC/DE/HL/SP,nn
	}, encodeLDrpnn)
	e.addInstructionVarCycles("LD", []OperandType{OperandRegister16, OperandIndirect}, 0x2A, 0x00, func(ops []*ResolvedOperand) int {
		if ops[0].Register == "HL" {
			return 16 // LD HL,(nn) -- unprefixed, shorter
		}
		return 20 // LD BC/DE/SP,(nn) (ED-prefixed) / LD IX/IY,(nn)
	}, encodeLDrpaddr)
	e.addInstructionVarCycles("LD", []OperandType{OperandIndirect, OperandRegister16}, 0x22, 0x00, func(ops []*ResolvedOperand) int {
		if ops[1].Register == "HL" {
			return 16 // LD (nn),HL -- unprefixed, shorter
		}
		return 20 // LD (nn),BC/DE/SP (ED-prefixed) / LD (nn),IX/IY
	}, encodeLDaddrrp)
	e.addInstructionVarCycles("LD", []OperandType{OperandRegister16, OperandRegister16}, 0xF9, 0x00, func(ops []*ResolvedOperand) int {
		if isIndexReg16(ops[1].Register) {
			return 10 // LD SP,IX/IY
		}
		return 6 // LD SP,HL
	}, encodeLDSPHL)

	// Block 2: ALU operations (from executeBlock2)
	e.addInstructionVarCycles("ADD", []OperandType{OperandRegister8, OperandRegister8}, 0x80, 0x00, aluRegCycles, encodeALUr)
	e.addInstructionVarCycles("ADC", []OperandType{OperandRegister8, OperandRegister8}, 0x88, 0x00, aluRegCycles, encodeALUr)
	e.addInstructionVarCycles("SUB", []OperandType{OperandRegister8}, 0x90, 0x00, aluRegCycles, encodeALUr)
	e.addInstructionVarCycles("SBC", []OperandType{OperandRegister8, OperandRegister8}, 0x98, 0x00, aluRegCycles, encodeALUr)
	e.addInstructionVarCycles("AND", []OperandType{OperandRegister8}, 0xA0, 0x00, aluRegCycles, encodeALUr)
	e.addInstructionVarCycles("XOR", []OperandType{OperandRegister8}, 0xA8, 0x00, aluRegCycles, encodeALUr)
	e.addInstructionVarCycles("OR", []OperandType{OperandRegister8}, 0xB0, 0x00, aluRegCycles, encodeALUr)
	e.addInstructionVarCycles("CP", []OperandType{OperandRegister8}, 0xB8, 0x00, aluRegCycles, encodeALUr)

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
	e.addInstructionVarCycles("ADD", []OperandType{OperandRegister8, OperandIndirect}, 0x80, 0x00, aluIndirectCycles, encodeALUIndirect)
	e.addInstructionVarCycles("ADC", []OperandType{OperandRegister8, OperandIndirect}, 0x88, 0x00, aluIndirectCycles, encodeALUIndirect)
	e.addInstructionVarCycles("SBC", []OperandType{OperandRegister8, OperandIndirect}, 0x98, 0x00, aluIndirectCycles, encodeALUIndirect)
	e.addInstructionVarCycles("SUB", []OperandType{OperandIndirect}, 0x90, 0x00, aluIndirectCycles, encodeALUIndirect)
	e.addInstructionVarCycles("AND", []OperandType{OperandIndirect}, 0xA0, 0x00, aluIndirectCycles, encodeALUIndirect)
	e.addInstructionVarCycles("XOR", []OperandType{OperandIndirect}, 0xA8, 0x00, aluIndirectCycles, encodeALUIndirect)
	e.addInstructionVarCycles("OR", []OperandType{OperandIndirect}, 0xB0, 0x00, aluIndirectCycles, encodeALUIndirect)
	e.addInstructionVarCycles("CP", []OperandType{OperandIndirect}, 0xB8, 0x00, aluIndirectCycles, encodeALUIndirect)

	// INC/DEC on (HL) / (IX+d) / (IY+d).
	e.addInstructionVarCycles("INC", []OperandType{OperandIndirect}, 0x34, 0x00, incDecIndirectCycles, encodeINCDECIndirect)
	e.addInstructionVarCycles("DEC", []OperandType{OperandIndirect}, 0x35, 0x00, incDecIndirectCycles, encodeINCDECIndirect)

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

	e.addInstructionVarCycles("INC", []OperandType{OperandRegister8}, 0x04, 0x00, incDecRegCycles, encodeINCr)
	e.addInstructionVarCycles("DEC", []OperandType{OperandRegister8}, 0x05, 0x00, incDecRegCycles, encodeDECr)

	e.addInstructionVarCycles("ADD", []OperandType{OperandRegister16, OperandRegister16}, 0x09, 0x00, func(ops []*ResolvedOperand) int {
		if isIndexReg16(ops[0].Register) {
			return 15 // ADD IX,rr / ADD IY,rr
		}
		return 11 // ADD HL,rr
	}, encodeADDHLrp)
	e.addInstructionVarCycles("INC", []OperandType{OperandRegister16}, 0x03, 0x00, incDecRp16Cycles, encodeINCrp)
	e.addInstructionVarCycles("DEC", []OperandType{OperandRegister16}, 0x0B, 0x00, incDecRp16Cycles, encodeDECrp)

	// Stack operations (PUSH/POP) - ADDED
	e.addInstructionVarCycles("PUSH", []OperandType{OperandRegister16}, 0xC5, 0x00, func(ops []*ResolvedOperand) int {
		if isIndexReg16(ops[0].Register) {
			return 15 // PUSH IX/IY
		}
		return 11 // PUSH BC/DE/HL/AF
	}, encodePUSHrp)
	e.addInstructionVarCycles("POP", []OperandType{OperandRegister16}, 0xC1, 0x00, func(ops []*ResolvedOperand) int {
		if isIndexReg16(ops[0].Register) {
			return 14 // POP IX/IY
		}
		return 10 // POP BC/DE/HL/AF
	}, encodePOPrp)

	// Control flow (from executeBlock3)
	e.addInstruction("JP", []OperandType{OperandImmediate16}, 0xC3, 0x00, 10, encodeJPnn)
	e.addInstruction("JP", []OperandType{OperandCondition, OperandImmediate16}, 0xC2, 0x00, 10, encodeJPccnn)
	e.addInstruction("JR", []OperandType{OperandRelative}, 0x18, 0x00, 12, encodeJRd)
	e.addInstruction("JR", []OperandType{OperandCondition, OperandRelative}, 0x20, 0x00, 12, encodeJRccd)
	e.addInstruction("DJNZ", []OperandType{OperandRelative}, 0x10, 0x00, 13, encodeDJNZ)
	e.addInstruction("CALL", []OperandType{OperandImmediate16}, 0xCD, 0x00, 17, encodeCALLnn)
	e.addInstruction("CALL", []OperandType{OperandCondition, OperandImmediate16}, 0xC4, 0x00, 17, encodeCALLccnn)
	e.addInstruction("RET", []OperandType{}, 0xC9, 0x00, 10, encodeSimple)
	e.addInstruction("RET", []OperandType{OperandCondition}, 0xC0, 0x00, 11, encodeRETcc) // taken; not-taken is 5, matching JR cc/CALL cc/DJNZ's own convention of reporting the longer case

	// JP (HL) / JP (IX) / JP (IY) - register-indirect jump
	e.addInstructionVarCycles("JP", []OperandType{OperandIndirect}, 0xE9, 0x00, func(ops []*ResolvedOperand) int {
		if isIndexReg16(ops[0].Register) {
			return 8 // JP (IX) / JP (IY)
		}
		return 4 // JP (HL)
	}, encodeJPindirect)

	// LD (IX+d),n / LD (IY+d),n - store immediate to indexed; the (HL) form is
	// already registered above (encodeLDHLn), which now also handles IX/IY.

	// Interrupt/refresh register loads (ED-prefixed) are handled inside
	// encodeLDrr, which already matches the LD r,r template.

	// Block I/O (ED-prefixed, no operands)
	e.addInstruction("INI", []OperandType{}, 0xA2, 0xED, 16, encodeSimple)
	e.addInstruction("IND", []OperandType{}, 0xAA, 0xED, 16, encodeSimple)
	e.addInstruction("INIR", []OperandType{}, 0xB2, 0xED, 21, encodeSimple) // repeat; last iteration is 16
	e.addInstruction("INDR", []OperandType{}, 0xBA, 0xED, 21, encodeSimple) // repeat; last iteration is 16
	e.addInstruction("OUTI", []OperandType{}, 0xA3, 0xED, 16, encodeSimple)
	e.addInstruction("OUTD", []OperandType{}, 0xAB, 0xED, 16, encodeSimple)
	e.addInstruction("OTIR", []OperandType{}, 0xB3, 0xED, 21, encodeSimple) // repeat; last iteration is 16
	e.addInstruction("OTDR", []OperandType{}, 0xBB, 0xED, 21, encodeSimple) // repeat; last iteration is 16

	// I/O instructions
	e.addInstructionVarCycles("OUT", []OperandType{OperandIndirect, OperandRegister8}, 0xD3, 0x00, func(ops []*ResolvedOperand) int {
		if ops[0].Register == "C" {
			return 12 // OUT (C),r
		}
		return 11 // OUT (n),A
	}, encodeOUTnA)
	e.addInstructionVarCycles("IN", []OperandType{OperandRegister8, OperandIndirect}, 0xDB, 0x00, func(ops []*ResolvedOperand) int {
		if ops[1].Register == "C" {
			return 12 // IN r,(C)
		}
		return 11 // IN A,(n)
	}, encodeINAn)

	// Undocumented ED70/ED71: bare IN (C) (reads the port, sets flags,
	// discards the byte -- also widely known as "IN F,(C)", confirmed
	// the same instruction under both names, not two different
	// opcodes; only the register-less spelling is implemented here)
	// and OUT (C),0 (outputs a literal zero, no source register).
	e.addInstruction("IN", []OperandType{OperandIndirect}, 0x70, 0xED, 12, encodeINFC)
	e.addInstruction("OUT", []OperandType{OperandIndirect, OperandImmediate8}, 0x71, 0xED, 12, encodeOUTC0)

	// CB-prefixed instructions (from executeCB)
	e.addInstruction("RLC", []OperandType{OperandRegister8}, 0x00, 0xCB, 8, encodeCBr)
	e.addInstruction("RRC", []OperandType{OperandRegister8}, 0x08, 0xCB, 8, encodeCBr)
	e.addInstruction("RL", []OperandType{OperandRegister8}, 0x10, 0xCB, 8, encodeCBr)
	e.addInstruction("RR", []OperandType{OperandRegister8}, 0x18, 0xCB, 8, encodeCBr)
	e.addInstruction("SLA", []OperandType{OperandRegister8}, 0x20, 0xCB, 8, encodeCBr)
	e.addInstruction("SRA", []OperandType{OperandRegister8}, 0x28, 0xCB, 8, encodeCBr)
	e.addInstruction("SRL", []OperandType{OperandRegister8}, 0x38, 0xCB, 8, encodeCBr)
	// Undocumented 8th shift (CB 30-37), commonly called SLL, with SLI
	// and SL1 as widely-recognised synonyms for the same opcode --
	// confirmed "quite commonly used" despite never being officially
	// documented. Shifts left like SLA, but sets bit 0 rather than
	// clearing it.
	e.addInstruction("SLL", []OperandType{OperandRegister8}, 0x30, 0xCB, 8, encodeCBr)
	e.addInstruction("SLI", []OperandType{OperandRegister8}, 0x30, 0xCB, 8, encodeCBr)
	e.addInstruction("SL1", []OperandType{OperandRegister8}, 0x30, 0xCB, 8, encodeCBr)
	e.addInstruction("BIT", []OperandType{OperandImmediate8, OperandRegister8}, 0x40, 0xCB, 8, encodeBITbr)
	e.addInstruction("RES", []OperandType{OperandImmediate8, OperandRegister8}, 0x80, 0xCB, 8, encodeRESbr)
	e.addInstruction("SET", []OperandType{OperandImmediate8, OperandRegister8}, 0xC0, 0xCB, 8, encodeSETbr)

	// CB-prefixed bit ops on (HL): reg code 6. base | (bit<<3) | 6
	e.addInstructionVarCycles("BIT", []OperandType{OperandImmediate8, OperandIndirect}, 0x40, 0xCB, cbBitIndirectCycles, encodeCBbHL)
	e.addInstructionVarCycles("RES", []OperandType{OperandImmediate8, OperandIndirect}, 0x80, 0xCB, cbIndirectCycles, encodeCBbHL)
	e.addInstructionVarCycles("SET", []OperandType{OperandImmediate8, OperandIndirect}, 0xC0, 0xCB, cbIndirectCycles, encodeCBbHL)
	// CB-prefixed rotates/shifts on (HL): base | 6
	e.addInstructionVarCycles("RLC", []OperandType{OperandIndirect}, 0x00, 0xCB, cbIndirectCycles, encodeCBHL)
	e.addInstructionVarCycles("RRC", []OperandType{OperandIndirect}, 0x08, 0xCB, cbIndirectCycles, encodeCBHL)
	e.addInstructionVarCycles("RL", []OperandType{OperandIndirect}, 0x10, 0xCB, cbIndirectCycles, encodeCBHL)
	e.addInstructionVarCycles("RR", []OperandType{OperandIndirect}, 0x18, 0xCB, cbIndirectCycles, encodeCBHL)
	e.addInstructionVarCycles("SLA", []OperandType{OperandIndirect}, 0x20, 0xCB, cbIndirectCycles, encodeCBHL)
	e.addInstructionVarCycles("SRA", []OperandType{OperandIndirect}, 0x28, 0xCB, cbIndirectCycles, encodeCBHL)
	e.addInstructionVarCycles("SRL", []OperandType{OperandIndirect}, 0x38, 0xCB, cbIndirectCycles, encodeCBHL)
	// SLL/SLI/SL1 on (HL)/(IX+d)/(IY+d): encodeCBHL already handles all
	// three via the switch added for the DDCB/FDCB fix, so no new
	// encode function is needed here either.
	e.addInstructionVarCycles("SLL", []OperandType{OperandIndirect}, 0x30, 0xCB, cbIndirectCycles, encodeCBHL)
	e.addInstructionVarCycles("SLI", []OperandType{OperandIndirect}, 0x30, 0xCB, cbIndirectCycles, encodeCBHL)
	e.addInstructionVarCycles("SL1", []OperandType{OperandIndirect}, 0x30, 0xCB, cbIndirectCycles, encodeCBHL)

	// DDCB/FDCB undocumented copy-to-register forms. Conventional
	// syntax "RLC (IX+d),B" / "SET n,(IX+d),B" (confirmed against GNU
	// as's own documented "op (ix+d), r" convention). BIT has no
	// equivalent (see encodeDDFDCBbCopy's own comment) and is
	// deliberately not registered here.
	e.addInstruction("RLC", []OperandType{OperandIndirect, OperandRegister8}, 0x00, 0xCB, 23, encodeDDFDCBCopy)
	e.addInstruction("RRC", []OperandType{OperandIndirect, OperandRegister8}, 0x08, 0xCB, 23, encodeDDFDCBCopy)
	e.addInstruction("RL", []OperandType{OperandIndirect, OperandRegister8}, 0x10, 0xCB, 23, encodeDDFDCBCopy)
	e.addInstruction("RR", []OperandType{OperandIndirect, OperandRegister8}, 0x18, 0xCB, 23, encodeDDFDCBCopy)
	e.addInstruction("SLA", []OperandType{OperandIndirect, OperandRegister8}, 0x20, 0xCB, 23, encodeDDFDCBCopy)
	e.addInstruction("SRA", []OperandType{OperandIndirect, OperandRegister8}, 0x28, 0xCB, 23, encodeDDFDCBCopy)
	e.addInstruction("SLL", []OperandType{OperandIndirect, OperandRegister8}, 0x30, 0xCB, 23, encodeDDFDCBCopy)
	e.addInstruction("SLI", []OperandType{OperandIndirect, OperandRegister8}, 0x30, 0xCB, 23, encodeDDFDCBCopy)
	e.addInstruction("SL1", []OperandType{OperandIndirect, OperandRegister8}, 0x30, 0xCB, 23, encodeDDFDCBCopy)
	e.addInstruction("SRL", []OperandType{OperandIndirect, OperandRegister8}, 0x38, 0xCB, 23, encodeDDFDCBCopy)
	e.addInstruction("RES", []OperandType{OperandImmediate8, OperandIndirect, OperandRegister8}, 0x80, 0xCB, 23, encodeDDFDCBbCopy)
	e.addInstruction("SET", []OperandType{OperandImmediate8, OperandIndirect, OperandRegister8}, 0xC0, 0xCB, 23, encodeDDFDCBbCopy)

	// Block transfer / search (ED-prefixed, no operands)
	e.addInstruction("LDIR", []OperandType{}, 0xB0, 0xED, 21, encodeSimple) // repeat; last iteration is 16
	e.addInstruction("LDDR", []OperandType{}, 0xB8, 0xED, 21, encodeSimple) // repeat; last iteration is 16
	e.addInstruction("LDI", []OperandType{}, 0xA0, 0xED, 16, encodeSimple)
	e.addInstruction("LDD", []OperandType{}, 0xA8, 0xED, 16, encodeSimple)
	e.addInstruction("CPIR", []OperandType{}, 0xB1, 0xED, 21, encodeSimple) // repeat; last iteration is 16
	e.addInstruction("CPDR", []OperandType{}, 0xB9, 0xED, 21, encodeSimple) // repeat; last iteration is 16
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
	e.addInstructionVarCycles("EX", []OperandType{OperandIndirect, OperandRegister16}, 0xE3, 0x00, func(ops []*ResolvedOperand) int {
		if isIndexReg16(ops[1].Register) {
			return 23 // EX (SP),IX / EX (SP),IY
		}
		return 19 // EX (SP),HL
	}, encodeEXSPHL)

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

// addInstructionVarCycles registers a template whose true T-state count
// depends on which specific register the resolved operands use (e.g.
// (HL) vs (IX+d)/(IY+d)) -- cyclesFunc computes it from the actual
// operands; Cycles itself is left at 0 and must not be read directly
// for these templates (use CyclesFor).
func (e *Encoder) addInstructionVarCycles(mnemonic string, operandTypes []OperandType, baseOpcode, prefix uint8, cyclesFunc func([]*ResolvedOperand) int, encodeFunc func(*InstructionTemplate, []*ResolvedOperand) ([]uint8, error)) {
	template := &InstructionTemplate{
		Mnemonic:     mnemonic,
		OperandTypes: operandTypes,
		Encode:       encodeFunc,
		BaseOpcode:   baseOpcode,
		Prefix:       prefix,
		CyclesFunc:   cyclesFunc,
	}

	e.instructions[mnemonic] = append(e.instructions[mnemonic], template)
}

// Encoding functions - these mirror the zen80 emulator's decoding patterns

// encodeSimple encodes instructions with no operands
func encodeSimple(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	return z80enc.EncodeSimple(template.Prefix, template.BaseOpcode), nil
}

// encodeLDrr encodes LD r,r' - mirrors executeBlock1 pattern 01yyyzzz
func encodeLDrr(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	b, err := z80enc.EncodeLDrr(operands[0].Register, operands[1].Register)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// encodeLDrn encodes LD r,n - mirrors executeBlock0 pattern 00yyy110
func encodeLDrn(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	return z80enc.EncodeReg8Imm(0x06, operands[0].Register, uint8(operands[1].Value))
}

// encodeLDrHL encodes LD r,(HL) - mirrors executeBlock1 pattern 01yyy110
func encodeLDrHL(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	if operands[1].Type != OperandIndirect {
		return nil, fmt.Errorf("expected indirect operand as source")
	}
	return z80enc.EncodeLDRegFromIndirect(operands[0].Register, operands[1].Register, uint16(operands[1].Value), int8(operands[1].Displacement))
}

// indexPrefix returns the DD/FD prefix byte for an IX/IY indexed operand.
func indexPrefix(reg string) uint8 {
	if reg == "IY" {
		return 0xFD
	}
	return 0xDD
}

// encodeDDFDCBCopy encodes the undocumented DDCB/FDCB copy-to-register
// rotate/shift forms: DD/FD, 0xCB, displacement, base|reg. Confirmed
// real, "quite commonly used" behaviour, not a theoretical opcode
// (multiple independent sources agree): the operation is performed on
// the indexed memory operand exactly as the documented (IX+d) form
// does, and the same result is *also* stored in the named register --
// equivalent to the operation followed by LD r,(IX+d). Conventional
// syntax confirmed against GNU as's own documentation: "op (ix+d), r".
// The actual byte arithmetic lives in pkg/z80enc (shareable, primitive-
// typed) -- this is a thin adapter extracting plain strings/ints from
// zenas's own ResolvedOperand.
func encodeDDFDCBCopy(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	ind := operands[0]
	b, err := z80enc.EncodeDDFDCBCopy(template.BaseOpcode, ind.Register, int8(ind.Displacement), operands[1].Register)
	if err != nil {
		if ind.Register != "IX" && ind.Register != "IY" {
			return nil, fmt.Errorf("expected (IX+d) or (IY+d)")
		}
		return nil, fmt.Errorf("%s: %w", template.Mnemonic, err)
	}
	return b, nil
}

// encodeDDFDCBbCopy encodes the undocumented DDCB/FDCB copy-to-register
// RES/SET forms: DD/FD, 0xCB, displacement, base|(bit<<3)|reg. BIT has
// no equivalent and is deliberately not registered with this function:
// confirmed (multiple independent sources) that all 8 register-field
// values of a DDCB/FDCB BIT instruction are identical, since BIT never
// writes a result anywhere but the flags -- there is nothing to copy.
func encodeDDFDCBbCopy(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	bit := uint8(operands[0].Value)
	ind := operands[1]
	b, err := z80enc.EncodeDDFDCBBitCopy(template.BaseOpcode, bit, ind.Register, int8(ind.Displacement), operands[2].Register)
	if err != nil {
		if bit > 7 {
			return nil, fmt.Errorf("bit number must be 0-7")
		}
		if ind.Register != "IX" && ind.Register != "IY" {
			return nil, fmt.Errorf("expected (IX+d) or (IY+d)")
		}
		return nil, fmt.Errorf("%s: %w", template.Mnemonic, err)
	}
	return b, nil
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
//
// Both illegal cases are rejected rather than silently producing a different
// instruction.
func applyHalfIndexPrefix(code []uint8, operands []*ResolvedOperand) ([]uint8, error) {
	var reg8Regs, indirectRegs []string
	for _, op := range operands {
		switch op.Type {
		case OperandIndirect:
			indirectRegs = append(indirectRegs, op.Register)
		case OperandRegister8:
			reg8Regs = append(reg8Regs, op.Register)
		}
	}
	return z80enc.ApplyHalfIndexPrefix(code, reg8Regs, indirectRegs)
}
func encodeLDHLr(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	if operands[0].Type != OperandIndirect {
		return nil, fmt.Errorf("expected indirect operand as destination")
	}
	return z80enc.EncodeLDIndirectFromReg(operands[0].Register, uint16(operands[0].Value), operands[1].Register, int8(operands[0].Displacement))
}
func encodeLDHLn(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	dst := operands[0]
	b, err := z80enc.EncodeIndirectImm8(dst.Register, int8(dst.Displacement), uint8(operands[1].Value))
	if err != nil {
		return nil, fmt.Errorf("expected (HL), (IX+d), or (IY+d) as destination")
	}
	return b, nil
}

// encodeALUr encodes ALU operations with register - mirrors executeBlock2 pattern 10yyyzzz
func encodeALUr(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	var reg string

	if len(operands) == 1 {
		// SUB r, AND r, XOR r, OR r, CP r
		reg = operands[0].Register
	} else if len(operands) == 2 {
		// ADD A,r; ADC A,r; SBC A,r
		if operands[0].Register != "A" {
			return nil, fmt.Errorf("first operand must be A for %s", template.Mnemonic)
		}
		reg = operands[1].Register
	} else {
		return nil, fmt.Errorf("invalid number of operands for ALU instruction")
	}

	return z80enc.EncodeReg8(template.BaseOpcode, reg)
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

	return z80enc.EncodeImm8(template.BaseOpcode, value), nil
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
	b, err := z80enc.EncodeIndirectByte(template.BaseOpcode|6, ind.Register, int8(ind.Displacement))
	if err != nil {
		return nil, fmt.Errorf("unsupported indirect operand for %s: (%s)", template.Mnemonic, ind.Register)
	}
	return b, nil
}

// encodeINCDECIndirect encodes INC/DEC on (HL)/(IX+d)/(IY+d).
func encodeINCDECIndirect(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	ind := operands[0]
	if ind.Type != OperandIndirect {
		return nil, fmt.Errorf("expected indirect operand for %s", template.Mnemonic)
	}
	b, err := z80enc.EncodeIndirectByte(template.BaseOpcode, ind.Register, int8(ind.Displacement))
	if err != nil {
		return nil, fmt.Errorf("unsupported indirect operand for %s: (%s)", template.Mnemonic, ind.Register)
	}
	return b, nil
}

// encodeINCr encodes INC r - mirrors executeBlock0 pattern 00yyy100
func encodeINCr(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	return z80enc.EncodeReg8Shift3(0x04, operands[0].Register)
}

// encodeDECr encodes DEC r - mirrors executeBlock0 pattern 00yyy101
func encodeDECr(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	return z80enc.EncodeReg8Shift3(0x05, operands[0].Register)
}

// encodeLDrpnn encodes LD rp,nn - mirrors executeBlock0 pattern 00pp0001
func encodeLDrpnn(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	return z80enc.EncodeRp16Imm(0x01, operands[0].Register, uint16(operands[1].Value))
}

// encodeADDHLrp encodes ADD HL,rp - mirrors executeBlock0 pattern 00pp1001
func encodeADDHLrp(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	return z80enc.EncodeADDHLrp(operands[0].Register, operands[1].Register)
}

// encodeINCrp encodes INC rp - mirrors executeBlock0 pattern 00pp0011
func encodeINCrp(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	return z80enc.EncodeRp16Shift4(0x03, operands[0].Register)
}

// encodeDECrp encodes DEC rp - mirrors executeBlock0 pattern 00pp1011
func encodeDECrp(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	return z80enc.EncodeRp16Shift4(0x0B, operands[0].Register)
}

// encodePUSHrp encodes PUSH rp - mirrors executeBlock0 pattern 11pp0101 (ADDED)
func encodePUSHrp(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	b, err := z80enc.EncodeStackOp(0xC5, operands[0].Register)
	if err != nil {
		return nil, fmt.Errorf("invalid register for PUSH: %s", operands[0].Register)
	}
	return b, nil
}

// encodePOPrp encodes POP rp - mirrors executeBlock0 pattern 11pp0001 (ADDED)
func encodePOPrp(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	b, err := z80enc.EncodeStackOp(0xC1, operands[0].Register)
	if err != nil {
		return nil, fmt.Errorf("invalid register for POP: %s", operands[0].Register)
	}
	return b, nil
}

// encodeLDrpaddr encodes LD rp,(nn) - load 16-bit register from memory
func encodeLDrpaddr(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	b, err := z80enc.EncodeLDrpAddr(operands[0].Register, uint16(operands[1].Value))
	if err != nil {
		return nil, fmt.Errorf("unsupported register for LD rp,(nn): %s", operands[0].Register)
	}
	return b, nil
}

// encodeLDaddrrp encodes LD (nn),rp - store 16-bit register to memory
func encodeLDaddrrp(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	b, err := z80enc.EncodeLDAddrRp(operands[1].Register, uint16(operands[0].Value))
	if err != nil {
		return nil, fmt.Errorf("unsupported register for LD (nn),rp: %s", operands[1].Register)
	}
	return b, nil
}

// encodeLDSPHL encodes LD SP,HL - special case for loading SP from HL
func encodeLDSPHL(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	if operands[0].Register != "SP" {
		return nil, fmt.Errorf("LD rp,rp only supports SP as destination here")
	}
	return z80enc.EncodeLDSPHL(operands[1].Register)
}

// encodeJPnn encodes JP nn
func encodeJPnn(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	return z80enc.EncodeAddr16(0xC3, uint16(operands[0].Value)), nil
}

// encodeJPccnn encodes JP cc,nn - mirrors executeBlock3 pattern 11ccc010
func encodeJPccnn(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	if len(operands) != 2 {
		return nil, fmt.Errorf("JP cc,nn requires exactly 2 operands")
	}

	if operands[0].Type != OperandCondition {
		return nil, fmt.Errorf("first operand must be a condition code")
	}

	return z80enc.EncodeCondAddr16(0xC2, operands[0].Condition, uint16(operands[1].Value))
}

// encodeJRd encodes JR d
func encodeJRd(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	return z80enc.EncodeRelBranch(0x18, int8(operands[0].Value)), nil
}

// encodeJRccd encodes JR cc,d - mirrors executeBlock0 pattern 001cc000
func encodeJRccd(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	b, err := z80enc.EncodeCondRelBranch(0x20, operands[0].Condition, int8(operands[1].Value))
	if err != nil {
		return nil, fmt.Errorf("invalid condition for JR: %s", operands[0].Condition)
	}
	return b, nil
}

// encodeDJNZ encodes DJNZ d - decrement B and jump if not zero
func encodeDJNZ(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	return z80enc.EncodeRelBranch(0x10, int8(operands[0].Value)), nil
}

// encodeCALLnn encodes CALL nn
func encodeCALLnn(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	return z80enc.EncodeAddr16(0xCD, uint16(operands[0].Value)), nil
}

// encodeCALLccnn encodes CALL cc,nn - 11ccc100
func encodeCALLccnn(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	return z80enc.EncodeCondAddr16(0xC4, operands[0].Condition, uint16(operands[1].Value))
}

// encodeJPindirect encodes JP (HL) / JP (IX) / JP (IY)
func encodeJPindirect(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	b, err := z80enc.EncodeJPIndirect(operands[0].Register)
	if err != nil {
		return nil, fmt.Errorf("JP only supports (HL), (IX), (IY) as indirect targets")
	}
	return b, nil
}

// encodeRETcc encodes RET cc - mirrors executeBlock3 pattern 11ccc000
func encodeRETcc(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	return z80enc.EncodeCondOnly(0xC0, operands[0].Condition)
}

// CB-prefixed instruction encoders

// encodeCBr encodes CB r instructions
func encodeCBr(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	return z80enc.EncodeCBReg8(template.BaseOpcode, operands[0].Register)
}

// encodeBITbr encodes BIT b,r
func encodeBITbr(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	b, err := z80enc.EncodeCBBitReg8(0x40, uint8(operands[0].Value), operands[1].Register)
	if err != nil {
		if v := operands[0].Value; v < 0 || v > 7 {
			return nil, fmt.Errorf("bit number must be 0-7")
		}
		return nil, err
	}
	return b, nil
}

// encodeRESbr encodes RES b,r
func encodeRESbr(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	b, err := z80enc.EncodeCBBitReg8(0x80, uint8(operands[0].Value), operands[1].Register)
	if err != nil {
		if v := operands[0].Value; v < 0 || v > 7 {
			return nil, fmt.Errorf("bit number must be 0-7")
		}
		return nil, err
	}
	return b, nil
}

// encodeOUTnA encodes OUT (n),A
func encodeOUTnA(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	if operands[0].Type != OperandIndirect {
		return nil, fmt.Errorf("expected indirect operand as port")
	}
	b, err := z80enc.EncodeOUT(operands[0].Register, operands[1].Register, uint8(operands[0].Value))
	if err != nil {
		return nil, fmt.Errorf("second operand must be A for OUT (n),A")
	}
	return b, nil
}

// encodeINAn encodes IN A,(n)
func encodeINAn(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	if operands[1].Type != OperandIndirect {
		return nil, fmt.Errorf("expected indirect operand as port")
	}
	b, err := z80enc.EncodeIN(operands[0].Register, operands[1].Register, uint8(operands[1].Value))
	if err != nil {
		return nil, fmt.Errorf("first operand must be A for IN A,(n)")
	}
	return b, nil
}

// encodeINFC encodes the undocumented "IN (C)": ED 70. Reads port C,
// sets flags exactly like the documented IN r,(C) forms, discards the
// byte read.
func encodeINFC(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	return z80enc.EncodeINFC(operands[0].Register)
}

// encodeOUTC0 encodes the undocumented "OUT (C),0": ED 71. Outputs a
// literal zero byte to the port in C, no source register.
func encodeOUTC0(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	return z80enc.EncodeOUTC0(operands[0].Register, operands[1].Value)
}

// encodeSETbr encodes SET b,r
func encodeSETbr(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	b, err := z80enc.EncodeCBBitReg8(0xC0, uint8(operands[0].Value), operands[1].Register)
	if err != nil {
		if v := operands[0].Value; v < 0 || v > 7 {
			return nil, fmt.Errorf("bit number must be 0-7")
		}
		return nil, err
	}
	return b, nil
}

// encodeCBHL encodes CB-prefixed rotate/shift on (HL)/(IX+d)/(IY+d): (HL) is
// 0xCB, base|6; the indexed forms are DD/FD, 0xCB, displacement, base|6 --
// the same opcode byte as the (HL) form, with the index prefix and
// displacement inserted before it. zen80 already executes this DDCB/FDCB
// group; zenas previously had no encoding for it at all (confirmed: no
// DDCB/FDCB handling anywhere in this file before this fix), forcing an
// LD-preserves-flags workaround at every call site that needed it.
func encodeCBHL(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	ind := operands[0]
	b, err := z80enc.EncodeCBIndirect(template.BaseOpcode, ind.Register, int8(ind.Displacement))
	if err != nil {
		return nil, fmt.Errorf("expected (HL), (IX+d), or (IY+d)")
	}
	return b, nil
}

// encodeCBbHL encodes CB-prefixed BIT/RES/SET n,(HL)/(IX+d)/(IY+d): (HL) is
// 0xCB, base|(bit<<3)|6; the indexed forms are DD/FD, 0xCB, displacement,
// base|(bit<<3)|6 -- the same opcode byte as the (HL) form, with the index
// prefix and displacement inserted before it. This is exactly the group a
// real zenpoint call site needed (flag-preserving BIT/SET/RES on (IY+d)),
// previously entirely unencodable, forcing a byte- and cycle-costly
// LD-preserves-flags workaround at all 21 sites that needed it.
func encodeCBbHL(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	bit := uint8(operands[0].Value)
	ind := operands[1]
	b, err := z80enc.EncodeCBBitIndirect(template.BaseOpcode, bit, ind.Register, int8(ind.Displacement))
	if err != nil {
		if bit > 7 {
			return nil, fmt.Errorf("bit number must be 0-7")
		}
		return nil, fmt.Errorf("expected (HL), (IX+d), or (IY+d)")
	}
	return b, nil
}

// encodeADC16 encodes ADC HL,rp (ED-prefixed): 0xED, 0x4A|(p<<4)
func encodeADC16(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	if operands[0].Register != "HL" {
		return nil, fmt.Errorf("16-bit ADC requires HL as the destination")
	}
	b, err := z80enc.EncodeED16(0x4A, operands[1].Register)
	if err != nil {
		return nil, fmt.Errorf("invalid source register for ADC HL,rp")
	}
	return b, nil
}

// encodeSBC16 encodes SBC HL,rp (ED-prefixed): 0xED, 0x42|(p<<4)
func encodeSBC16(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	if operands[0].Register != "HL" {
		return nil, fmt.Errorf("16-bit SBC requires HL as the destination")
	}
	b, err := z80enc.EncodeED16(0x42, operands[1].Register)
	if err != nil {
		return nil, fmt.Errorf("invalid source register for SBC HL,rp")
	}
	return b, nil
}

// encodeEX encodes EX DE,HL (0xEB) and EX AF,AF' (0x08)
func encodeEX(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	return z80enc.EncodeEX(operands[0].Register, operands[1].Register)
}

// encodeEXSPHL encodes EX (SP),HL (0xE3)
func encodeEXSPHL(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	if operands[0].Register != "SP" {
		return nil, fmt.Errorf("EX (SP),... requires SP as the first operand")
	}
	return z80enc.EncodeEXSPHL(operands[1].Register)
}

// encodeIM encodes IM 0/1/2 (ED-prefixed): 0xED then 0x46/0x56/0x5E
func encodeIM(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	return z80enc.EncodeIM(operands[0].Value)
}

// encodeRST encodes RST n: 0xC7 | n, where n is one of 0x00,0x08,...,0x38
func encodeRST(template *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	return z80enc.EncodeRST(operands[0].Value)
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
		if len(operands) != 1 {
			return nil, fmt.Errorf("%s takes only the operand %s", t.Mnemonic, want)
		}
		return z80enc.EncodeFixedOperand(t.Prefix, t.BaseOpcode, t.Mnemonic, operands[0].Register, want)
	}
}

// encodeZ80NFixedRegPair returns an encoder for a two-operand instruction whose
// operands are hard-wired (e.g. MUL D,E or BSLA DE,B).
func encodeZ80NFixedRegPair(a, b string) func(*InstructionTemplate, []*ResolvedOperand) ([]uint8, error) {
	return func(t *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
		if len(operands) != 2 {
			return nil, fmt.Errorf("%s takes only the operands %s,%s", t.Mnemonic, a, b)
		}
		return z80enc.EncodeFixedOperandPair(t.Prefix, t.BaseOpcode, t.Mnemonic, operands[0].Register, a, operands[1].Register, b)
	}
}

// encodeZ80NJPc encodes the Z80N JP (C) (ED 98). The operand parses as an
// indirect with register "C"; any other indirect is rejected so JP (HL)/(IX)/(IY)
// continue to be handled by the base encoder.
func encodeZ80NJPc(t *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	b, err := z80enc.EncodeZ80NJPc(operands[0].Register)
	if err != nil {
		return nil, fmt.Errorf("expected JP (C)")
	}
	return b, nil
}

// encodeZ80NTEST encodes TEST n (ED 27 n).
func encodeZ80NTEST(t *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	return z80enc.EncodeZ80NTEST(uint8(operands[0].Value)), nil
}

// encodeZ80NADDrrA encodes ADD HL,A / ADD DE,A / ADD BC,A (ED 31/32/33).
func encodeZ80NADDrrA(t *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	return z80enc.EncodeZ80NADDrrA(operands[0].Register, operands[1].Register)
}

// encodeZ80NADDrrnn encodes ADD HL,nn / ADD DE,nn / ADD BC,nn (ED 34/35/36).
// The 16-bit immediate is little-endian (unlike PUSH nn).
func encodeZ80NADDrrnn(t *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	return z80enc.EncodeZ80NADDrrnn(operands[0].Register, uint16(operands[1].Value))
}

// encodeZ80NPUSHnn encodes PUSH nn (ED 8A hi lo). This is the only Z80 operand
// encoded BIG-endian: the high byte precedes the low byte.
func encodeZ80NPUSHnn(t *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	return z80enc.EncodeZ80NPUSHnn(uint16(operands[0].Value)), nil
}

// encodeZ80NNEXTREGnn encodes NEXTREG n,n (ED 91 reg val).
func encodeZ80NNEXTREGnn(t *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	return z80enc.EncodeZ80NNEXTREGnn(uint8(operands[0].Value), uint8(operands[1].Value)), nil
}

// encodeZ80NNEXTREGnA encodes NEXTREG n,A (ED 92 reg); the value comes from A.
func encodeZ80NNEXTREGnA(t *InstructionTemplate, operands []*ResolvedOperand) ([]uint8, error) {
	return z80enc.EncodeZ80NNEXTREGnA(uint8(operands[0].Value), operands[1].Register)
}
