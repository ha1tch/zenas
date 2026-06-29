package assembler

import (
	"fmt"
	"strings"
)

// MacroStyle represents the macro notation style
type MacroStyle int

const (
	MacroStyleTraditional MacroStyle = iota
	MacroStyleC
)

func (ms MacroStyle) String() string {
	switch ms {
	case MacroStyleTraditional:
		return "TRADITIONAL"
	case MacroStyleC:
		return "C"
	default:
		return "UNKNOWN"
	}
}

// MacroMode controls how repeated instantiations of a macro are emitted.
//
//	INLINE    - every instantiation emits the full body (the default).
//	SINGLETON - the body is emitted once as a callable routine; every
//	            instantiation emits argument setup followed by a CALL to it.
type MacroMode int

const (
	MacroModeInline MacroMode = iota
	MacroModeSingleton
)

func (mm MacroMode) String() string {
	switch mm {
	case MacroModeInline:
		return "INLINE"
	case MacroModeSingleton:
		return "SINGLETON"
	default:
		return "UNKNOWN"
	}
}

// CallingConvention defines how parameters are passed between macros
type CallingConvention struct {
	Name         string
	ParamRegs    []string // A, B, C, D, E, H, L
	ReturnRegs   []string // HL, DE, A
	CallerSaved  []string // Registers caller must preserve
	CalleeSaved  []string // Registers callee must preserve
	StackCleanup string   // "CALLER" or "CALLEE"
}

// ParameterType represents the type of a macro parameter
type ParameterType int

const (
	TypeUint8 ParameterType = iota
	TypeUint16
	TypeRegister8
	TypeRegister16
	TypeAddress
	// TypeUntyped marks a parameter with no declared width. Arguments bound to an
	// untyped parameter are not width-checked (the programmer is trusted). Kept
	// last so the existing iota values are unchanged.
	TypeUntyped
)

// DeclaredWidthBits returns the declared width of a parameter type in bits, and
// whether the type carries a checkable width at all. Untyped and register types
// have no fixed value-width to check an argument against.
func (pt ParameterType) DeclaredWidthBits() (bits int, checkable bool) {
	switch pt {
	case TypeUint8:
		return 8, true
	case TypeUint16, TypeAddress:
		return 16, true
	default:
		// TypeUntyped, TypeRegister8, TypeRegister16: no value-width contract.
		return 0, false
	}
}

func (pt ParameterType) String() string {
	switch pt {
	case TypeUint8:
		return "uint8_t"
	case TypeUint16:
		return "uint16_t"
	case TypeRegister8:
		return "register8_t"
	case TypeRegister16:
		return "register16_t"
	case TypeAddress:
		return "address_t"
	default:
		return "unknown"
	}
}

// MacroParameter represents a parameter in a macro definition
type MacroParameter struct {
	Name string
	Type ParameterType
}

// MacroDefinition represents a complete macro definition
type MacroDefinition struct {
	Name        string
	Package     string // affiliation set by .PACKAGE; empty = default package
	Parameters  []*MacroParameter
	ReturnType  ParameterType
	Body        []ParsedLine
	Style       MacroStyle
	LineNumber  int
	UniqueID    int // For generating unique labels
}

// MacroCall represents a call to a macro
type MacroCall struct {
	Name       string
	Arguments  []*Expression
	Style      MacroStyle
	LineNumber int
}

// Default calling conventions
var (
	RegisterFastConvention = CallingConvention{
		Name:         "REGISTER_FAST",
		ParamRegs:    []string{"A", "B", "C", "D", "E", "H", "L"},
		ReturnRegs:   []string{"A", "HL", "DE"},
		CallerSaved:  []string{"A", "BC", "DE"},
		CalleeSaved:  []string{"HL", "IX", "IY"},
		StackCleanup: "CALLER",
	}
)

// ParseParameterType converts a string type to ParameterType
func ParseParameterType(typeStr string) (ParameterType, error) {
	// Convert to lowercase for case-insensitive comparison
	lowerType := strings.ToLower(typeStr)
	
	switch lowerType {
	case "uint8_t", "uint8", "byte":
		return TypeUint8, nil
	case "uint16_t", "uint16", "word":
		return TypeUint16, nil
	case "register8_t", "reg8":
		return TypeRegister8, nil
	case "register16_t", "reg16":
		return TypeRegister16, nil
	case "address_t", "addr":
		return TypeAddress, nil
	case "void":  // Add support for void return type
		return TypeUint8, nil  // Use uint8 as default, could add TypeVoid if needed
	default:
		return TypeUint8, fmt.Errorf("unknown parameter type: %s", typeStr)
	}
}

// ValidateParameterType checks if a parameter type is valid for the given value
func (pt ParameterType) ValidateValue(value int) error {
	switch pt {
	case TypeUint8:
		if value < 0 || value > 255 {
			return fmt.Errorf("uint8_t value %d out of range [0, 255]", value)
		}
	case TypeUint16, TypeAddress:
		if value < 0 || value > 65535 {
			return fmt.Errorf("uint16_t value %d out of range [0, 65535]", value)
		}
	case TypeRegister8:
		// Register validation would be done elsewhere
		return nil
	case TypeRegister16:
		// Register validation would be done elsewhere
		return nil
	}
	return nil
}
