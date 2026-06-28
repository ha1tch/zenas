package assembler


// SymbolTable manages symbols (labels and constants) during assembly
type SymbolTable struct {
	symbols map[string]uint16
}

// NewSymbolTable creates a new symbol table
func NewSymbolTable() *SymbolTable {
	return &SymbolTable{
		symbols: make(map[string]uint16),
	}
}

// Define adds or updates a symbol in the table
func (st *SymbolTable) Define(name string, value uint16) {
	st.symbols[name] = value
}

// Lookup retrieves a symbol's value
func (st *SymbolTable) Lookup(name string) (uint16, bool) {
	value, exists := st.symbols[name]
	return value, exists
}

// GetAll returns all symbols in the table
func (st *SymbolTable) GetAll() map[string]uint16 {
	result := make(map[string]uint16)
	for name, value := range st.symbols {
		result[name] = value
	}
	return result
}

// Clear removes all symbols from the table
func (st *SymbolTable) Clear() {
	st.symbols = make(map[string]uint16)
}

// IsDefined checks if a symbol is defined
func (st *SymbolTable) IsDefined(name string) bool {
	_, exists := st.symbols[name]
	return exists
}

// Remove removes a symbol from the table
func (st *SymbolTable) Remove(name string) {
	delete(st.symbols, name)
}

// Count returns the number of symbols in the table
func (st *SymbolTable) Count() int {
	return len(st.symbols)
}