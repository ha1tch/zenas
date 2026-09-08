package z80enc

import "fmt"

// Reg8 returns the 3-bit register-field code for an 8-bit register name:
// B,C,D,E,H,L,A (0,1,2,3,4,5,7 -- 6 is reserved for (HL)/(IX+d)/(IY+d)
// and is never returned here), plus the undocumented half-index
// registers IXH/IXL/IYH/IYL, which share H/L's own codes (4/5) -- the
// distinguishing DD/FD prefix is a separate concern, not part of the
// register-field code itself.
func Reg8(name string) (uint8, error) {
	switch name {
	case "B":
		return 0, nil
	case "C":
		return 1, nil
	case "D":
		return 2, nil
	case "E":
		return 3, nil
	case "H", "IXH", "IYH":
		return 4, nil
	case "L", "IXL", "IYL":
		return 5, nil
	case "A":
		return 7, nil
	default:
		return 0, fmt.Errorf("z80enc: invalid 8-bit register %q", name)
	}
}

// Reg16 returns the 2-bit register-pair code for the general 00pp-style
// encoding (LD rp,nn / INC rp / DEC rp / ADD HL,rp and similar): BC, DE,
// HL/IX/IY (all three share code 2 -- IX/IY are distinguished by their
// own DD/FD prefix, not a different code), SP.
func Reg16(name string) (uint8, error) {
	switch name {
	case "BC":
		return 0, nil
	case "DE":
		return 1, nil
	case "HL", "IX", "IY":
		return 2, nil
	case "SP":
		return 3, nil
	default:
		return 0, fmt.Errorf("z80enc: invalid 16-bit register %q", name)
	}
}

// StackReg16 returns the 2-bit register-pair code PUSH/POP use, which
// differs from Reg16's own encoding at code 3: BC, DE, HL, AF (not SP --
// PUSH/POP never take SP as an operand at all).
func StackReg16(name string) (uint8, error) {
	switch name {
	case "BC":
		return 0, nil
	case "DE":
		return 1, nil
	case "HL":
		return 2, nil
	case "AF":
		return 3, nil
	default:
		return 0, fmt.Errorf("z80enc: invalid PUSH/POP register %q", name)
	}
}

// Condition returns the 3-bit condition code: NZ, Z, NC, C, PO, PE, P, M.
func Condition(name string) (uint8, error) {
	switch name {
	case "NZ":
		return 0, nil
	case "Z":
		return 1, nil
	case "NC":
		return 2, nil
	case "C":
		return 3, nil
	case "PO":
		return 4, nil
	case "PE":
		return 5, nil
	case "P":
		return 6, nil
	case "M":
		return 7, nil
	default:
		return 0, fmt.Errorf("z80enc: invalid condition %q", name)
	}
}

// JRCondition returns the 2-bit condition code JR/DJNZ's own narrower
// encoding uses -- only NZ, Z, NC, C are valid (JR has no unconditional
// -vs- PO/PE/P/M forms the way JP/CALL do).
func JRCondition(name string) (uint8, error) {
	switch name {
	case "NZ":
		return 0, nil
	case "Z":
		return 1, nil
	case "NC":
		return 2, nil
	case "C":
		return 3, nil
	default:
		return 0, fmt.Errorf("z80enc: invalid JR/DJNZ condition %q (want NZ, Z, NC, or C)", name)
	}
}

// IndexPrefix returns the DD (IX) or FD (IY) prefix byte for the named
// index register. regName must be "IX" or "IY".
func IndexPrefix(regName string) (uint8, error) {
	switch regName {
	case "IX":
		return 0xDD, nil
	case "IY":
		return 0xFD, nil
	default:
		return 0, fmt.Errorf("z80enc: expected IX or IY, got %q", regName)
	}
}
