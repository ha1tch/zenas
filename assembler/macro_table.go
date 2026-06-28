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
	nextUnique  int // For generating unique IDs
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
