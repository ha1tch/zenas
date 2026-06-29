package assembler

import (
	"fmt"
	"strings"
)

// MacroTable manages macro definitions and their validation
type MacroTable struct {
	macros      map[string]*MacroDefinition
	convention  CallingConvention
	style       MacroStyle
	nextUnique  int             // For generating unique IDs
	mnemonics   map[string]bool // instruction mnemonics (upper-case); a macro
	// whose bare name is a mnemonic can never be reached unqualified, so it must
	// be placed in a package. Populated by SetMnemonics; empty disables the check.
}

// reservedParamNames is the set of names a macro parameter may not take, because
// they would be silently misread as registers or condition codes inside the
// macro body. For example a parameter named "b" in `LD (HL), b` assembles as the
// register B, not the argument - a failure that produces wrong code with no
// error. Forbidding these names at definition time makes that mistake loud.
// Matching is case-insensitive (names are upper-cased before lookup).
var reservedParamNames = map[string]bool{
	// 8-bit registers
	"A": true, "B": true, "C": true, "D": true, "E": true, "H": true, "L": true,
	"I": true, "R": true,
	"IXH": true, "IXL": true, "IYH": true, "IYL": true,
	// 16-bit register pairs
	"BC": true, "DE": true, "HL": true, "SP": true, "AF": true,
	"IX": true, "IY": true,
	// condition codes (NZ, Z, NC, C, PO, PE, P, M); C, Z, P also overlap above
	"NZ": true, "Z": true, "NC": true, "PO": true, "PE": true, "P": true, "M": true,
}

// NewMacroTable creates a new macro table with default settings
func NewMacroTable() *MacroTable {
	return &MacroTable{
		macros:     make(map[string]*MacroDefinition),
		convention: RegisterFastConvention,
		style:      MacroStyleTraditional,
		nextUnique: 1,
	}
}

// SetStyle sets the macro notation style for this table
func (mt *MacroTable) SetStyle(style MacroStyle) {
	mt.style = style
}

// SetMnemonics records the instruction mnemonic set (upper-case) used to reject
// a macro definition whose bare name would collide with an instruction and could
// therefore only be reached when placed in a package.
func (mt *MacroTable) SetMnemonics(names map[string]bool) {
	mt.mnemonics = names
}

// SetCallingConvention sets the calling convention for this table
func (mt *MacroTable) SetCallingConvention(convention CallingConvention) {
	mt.convention = convention
}

// macroKey builds the internal map key for a (package, name) pair. The NUL
// separator cannot appear in an identifier, so the package and name remain
// distinct components rather than a parseable substring - which matters for the
// later step where --tag selects between same-named packages across tiers.
func macroKey(pkg, name string) string {
	return strings.ToUpper(pkg) + "\x00" + strings.ToUpper(name)
}

// Define adds a macro definition to the table
func (mt *MacroTable) Define(macro *MacroDefinition) error {
	if macro == nil {
		return fmt.Errorf("cannot define nil macro")
	}
	
	// Validate macro name
	if macro.Name == "" {
		return fmt.Errorf("macro name cannot be empty")
	}

	// A macro whose bare name is an instruction mnemonic can never be reached
	// unqualified - a bare `add` is always the ADD instruction. Such a macro is
	// only usable when placed in a package and called qualified (`math.add`).
	// Requiring the package at definition time makes the unreachable case
	// impossible rather than a confusing encoding error at the call site.
	if mt.mnemonics != nil && macro.Package == "" && mt.mnemonics[strings.ToUpper(macro.Name)] {
		return fmt.Errorf("macro '%s' has the same name as an instruction mnemonic; "+
			"a bare call would assemble as the instruction, so it must be placed in a "+
			"package (add a '.PACKAGE name' before it) and called qualified, e.g. 'name.%s(...)'",
			macro.Name, macro.Name)
	}

	key := macroKey(macro.Package, macro.Name)
	
	// Check for duplicate definition (same package and name)
	if existing, exists := mt.macros[key]; exists {
		return fmt.Errorf("macro '%s' already defined at line %d", macro.Name, existing.LineNumber)
	}
	
	// Validate parameter count against calling convention
	if len(macro.Parameters) > len(mt.convention.ParamRegs) {
		return fmt.Errorf("macro '%s' has %d parameters, but calling convention only supports %d",
			macro.Name, len(macro.Parameters), len(mt.convention.ParamRegs))
	}
	
	// Validate parameter names are unique
	paramNames := make(map[string]bool)
	for _, param := range macro.Parameters {
		if param.Name == "" {
			return fmt.Errorf("macro '%s' has parameter with empty name", macro.Name)
		}
		upperName := strings.ToUpper(param.Name)
		if reservedParamNames[upperName] {
			return fmt.Errorf("macro '%s' parameter '%s' is a reserved register or condition name; "+
				"a parameter with this name would be silently assembled as the %s register/condition "+
				"inside the macro body - rename it (for example to '%s_')",
				macro.Name, param.Name, upperName, param.Name)
		}
		if paramNames[upperName] {
			return fmt.Errorf("macro '%s' has duplicate parameter name: %s", macro.Name, param.Name)
		}
		paramNames[upperName] = true
	}
	
	// Assign unique ID for label generation
	macro.UniqueID = mt.nextUnique
	mt.nextUnique++
	
	// Store macro under its (package, name) key (case-insensitive)
	mt.macros[key] = macro
	
	return nil
}

// Lookup retrieves a macro definition by a possibly-qualified name. A name
// containing a '.' is treated as qualified (package.name) and resolved exactly;
// resolveBodyCall resolves a name used inside a macro body to a defined macro,
// honoring the calling macro's package for bare names: a bare name prefers a
// macro in the same package, then falls back to a unique cross-package match.
func (mt *MacroTable) resolveBodyCall(name, fromPkg string) (*MacroDefinition, bool) {
	if _, _, ok := splitQualified(name); ok {
		return mt.Lookup(name)
	}
	if fromPkg != "" {
		if m, ok := mt.macros[macroKey(fromPkg, name)]; ok {
			return m, true
		}
	}
	return mt.Lookup(name)
}

// IsRecursive reports whether the given macro can reach itself through calls in
// its body (directly or through a chain of other macros). It is used to refuse
// SINGLETON mode for recursive macros, whose fixed-slot argument passing is not
// re-entrant.
func (mt *MacroTable) IsRecursive(macro *MacroDefinition) bool {
	start := macroKey(macro.Package, macro.Name)
	visited := make(map[string]bool)

	var reaches func(m *MacroDefinition) bool
	reaches = func(m *MacroDefinition) bool {
		for _, line := range m.Body {
			if line.Instruction == nil {
				continue
			}
			callee, ok := mt.resolveBodyCall(line.Instruction.Mnemonic, m.Package)
			if !ok {
				continue
			}
			calleeKey := macroKey(callee.Package, callee.Name)
			if calleeKey == start {
				return true // reached the original macro: a cycle
			}
			if visited[calleeKey] {
				continue
			}
			visited[calleeKey] = true
			if reaches(callee) {
				return true
			}
		}
		return false
	}
	return reaches(macro)
}

// a bare name is resolved across packages, succeeding only when exactly one
// package defines it (see LookupBare for the ambiguous case).
func (mt *MacroTable) Lookup(name string) (*MacroDefinition, bool) {
	if pkg, bare, ok := splitQualified(name); ok {
		macro, exists := mt.macros[macroKey(pkg, bare)]
		return macro, exists
	}
	macro, ambiguous, exists := mt.LookupBare(name)
	if ambiguous {
		// Ambiguous bare name: report not-found rather than a nil match, so
		// callers do not dereference nil. Callers that want to give a good
		// "qualify this" error use LookupBare directly.
		return nil, false
	}
	return macro, exists
}

// LookupBare resolves an unqualified name across all packages. It returns the
// single matching macro when exactly one package defines the name; when more
// than one does, it returns ambiguous=true and the macro is nil, so the caller
// can require qualification. A name containing a '.' is resolved exactly instead.
func (mt *MacroTable) LookupBare(name string) (macro *MacroDefinition, ambiguous bool, exists bool) {
	if pkg, bare, ok := splitQualified(name); ok {
		m, e := mt.macros[macroKey(pkg, bare)]
		return m, false, e
	}
	upper := strings.ToUpper(name)
	var found *MacroDefinition
	count := 0
	var pkgs []string
	for _, m := range mt.macros {
		if strings.ToUpper(m.Name) == upper {
			found = m
			count++
			pkgs = append(pkgs, m.Package)
		}
	}
	if count == 0 {
		return nil, false, false
	}
	if count > 1 {
		return nil, true, true
	}
	return found, false, true
}

// AmbiguousPackages returns the package names that define a given bare macro
// name, for building a helpful "qualify this call" error.
func (mt *MacroTable) AmbiguousPackages(name string) []string {
	upper := strings.ToUpper(name)
	var pkgs []string
	for _, m := range mt.macros {
		if strings.ToUpper(m.Name) == upper {
			p := m.Package
			if p == "" {
				p = "(default)"
			}
			pkgs = append(pkgs, p)
		}
	}
	return pkgs
}

// splitQualified splits "package.name" into its parts. Returns ok=false for a
// bare name (no '.'). Only the last '.' separates package from name, so dotted
// labels inside a name are tolerated, though packages are normally single-level.
func splitQualified(name string) (pkg, bare string, ok bool) {
	idx := strings.LastIndex(name, ".")
	if idx <= 0 || idx == len(name)-1 {
		return "", "", false
	}
	return name[:idx], name[idx+1:], true
}

// ValidateCall validates a macro call against its definition
func (mt *MacroTable) ValidateCall(call *MacroCall) (*MacroDefinition, error) {
	if call == nil {
		return nil, fmt.Errorf("cannot validate nil macro call")
	}
	
	// Find macro definition
	macro, exists := mt.Lookup(call.Name)
	if !exists {
		return nil, fmt.Errorf("undefined macro: %s", call.Name)
	}
	
	// Check parameter count
	if len(call.Arguments) != len(macro.Parameters) {
		return nil, fmt.Errorf("macro '%s' expects %d parameters, got %d",
			call.Name, len(macro.Parameters), len(call.Arguments))
	}
	
	// Note: Type validation would happen during expansion when expressions are evaluated
	
	return macro, nil
}

// GetAll returns all macro definitions
func (mt *MacroTable) GetAll() map[string]*MacroDefinition {
	result := make(map[string]*MacroDefinition)
	for name, macro := range mt.macros {
		result[name] = macro
	}
	return result
}

// Clear removes all macro definitions
func (mt *MacroTable) Clear() {
	mt.macros = make(map[string]*MacroDefinition)
	mt.nextUnique = 1
}

// Count returns the number of defined macros
func (mt *MacroTable) Count() int {
	return len(mt.macros)
}

// IsDefined checks if a macro is defined
func (mt *MacroTable) IsDefined(name string) bool {
	_, exists := mt.Lookup(name)
	return exists
}

// Remove removes a macro definition
func (mt *MacroTable) Remove(name string) bool {
	upperName := strings.ToUpper(name)
	if _, exists := mt.macros[upperName]; exists {
		delete(mt.macros, upperName)
		return true
	}
	return false
}

// GetCallingConvention returns the current calling convention
func (mt *MacroTable) GetCallingConvention() CallingConvention {
	return mt.convention
}

// GetStyle returns the current macro style
func (mt *MacroTable) GetStyle() MacroStyle {
	return mt.style
}

// GenerateUniqueLabelSuffix generates a unique suffix for labels within macro expansion
func (mt *MacroTable) GenerateUniqueLabelSuffix(macroID int, labelBase string) string {
	return fmt.Sprintf("%s_%d", strings.ToUpper(labelBase), macroID)
}

// ListMacros returns a formatted list of all defined macros
func (mt *MacroTable) ListMacros() []string {
	var macros []string
	for name, macro := range mt.macros {
		paramList := ""
		if len(macro.Parameters) > 0 {
			var params []string
			for _, param := range macro.Parameters {
				params = append(params, fmt.Sprintf("%s %s", param.Type.String(), param.Name))
			}
			paramList = strings.Join(params, ", ")
		}
		
		macroDesc := fmt.Sprintf("%s(%s) -> %s [%s style]",
			name, paramList, macro.ReturnType.String(), macro.Style.String())
		macros = append(macros, macroDesc)
	}
	return macros
}
