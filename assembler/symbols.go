package assembler

// SymbolKind says what a symbol is: symbol files that tell a label from a
// constant (a CSpect map) need to know, and so does a debugger.
type SymbolKind int

const (
	// SymbolLabel is an address in the program: a label on a line.
	SymbolLabel SymbolKind = iota
	// SymbolConstant is a value given with EQU or on the command line
	// (--define, --tag).
	SymbolConstant
	// SymbolImported came from another tool's symbol file (--symbols). The
	// source can use it; it is not part of what the source defines, so it
	// is left out of GetAll and of every symbol file written.
	SymbolImported
)

// SymbolTable manages symbols (labels and constants) during assembly
type SymbolTable struct {
	symbols map[string]uint16
	kinds   map[string]SymbolKind
}

// NewSymbolTable creates a new symbol table
func NewSymbolTable() *SymbolTable {
	return &SymbolTable{
		symbols: make(map[string]uint16),
		kinds:   make(map[string]SymbolKind),
	}
}

// Define adds or updates a label in the table
func (st *SymbolTable) Define(name string, value uint16) {
	st.DefineKind(name, value, SymbolLabel)
}

// DefineKind adds or updates a symbol of a given kind. A later definition
// replaces an earlier one, kind and all: a name the source defines takes
// the place of an imported one.
func (st *SymbolTable) DefineKind(name string, value uint16, kind SymbolKind) {
	st.symbols[name] = value
	st.kinds[name] = kind
}

// Kinds returns the kind of every symbol GetAll returns.
func (st *SymbolTable) Kinds() map[string]SymbolKind {
	result := make(map[string]SymbolKind)
	for name, kind := range st.kinds {
		if kind != SymbolImported {
			result[name] = kind
		}
	}
	return result
}

// Lookup retrieves a symbol's value
func (st *SymbolTable) Lookup(name string) (uint16, bool) {
	value, exists := st.symbols[name]
	return value, exists
}

// GetAll returns the symbols the program defines: every one in the table
// but those imported from a symbol file.
func (st *SymbolTable) GetAll() map[string]uint16 {
	result := make(map[string]uint16)
	for name, value := range st.symbols {
		if st.kinds[name] != SymbolImported {
			result[name] = value
		}
	}
	return result
}

// Clear removes all symbols from the table
func (st *SymbolTable) Clear() {
	st.symbols = make(map[string]uint16)
	st.kinds = make(map[string]SymbolKind)
}

// IsDefined checks if a symbol is defined
func (st *SymbolTable) IsDefined(name string) bool {
	_, exists := st.symbols[name]
	return exists
}

// Remove removes a symbol from the table
func (st *SymbolTable) Remove(name string) {
	delete(st.symbols, name)
	delete(st.kinds, name)
}

// Count returns the number of symbols in the table
func (st *SymbolTable) Count() int {
	return len(st.symbols)
}
