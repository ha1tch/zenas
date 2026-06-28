package assembler

import (
	"fmt"
	"strings"
)

// MacroNotationParser defines the interface for parsing different macro styles
type MacroNotationParser interface {
	ParseMacroDefinition(tokens []Token, startPos int) (*MacroDefinition, int, error)
	ParseMacroCall(tokens []Token, startPos int) (*MacroCall, int, error)
	CanParseMacroDefinition(tokens []Token, startPos int) bool
	CanParseMacroCall(tokens []Token, startPos int) bool
}

// MacroParserManager manages different macro notation parsers
type MacroParserManager struct {
	parsers    map[MacroStyle]MacroNotationParser
	currentStyle MacroStyle
}

// SetKnownMacros forwards the set of currently-defined macro names to the
// traditional parser, so nested calls inside a macro body parse correctly.
func (mpm *MacroParserManager) SetKnownMacros(names map[string]bool) {
	if tmp, ok := mpm.parsers[MacroStyleTraditional].(*TraditionalMacroParser); ok {
		tmp.SetKnownMacros(names)
	}
}

// NewMacroParserManager creates a new macro parser manager
func NewMacroParserManager() *MacroParserManager {
	manager := &MacroParserManager{
		parsers:      make(map[MacroStyle]MacroNotationParser),
		currentStyle: MacroStyleTraditional,
	}
	
	// Register built-in parsers
	manager.parsers[MacroStyleTraditional] = NewTraditionalMacroParser()
	manager.parsers[MacroStyleC] = NewCStyleMacroParser()
	
	return manager
}

// SetStyle sets the current macro style
func (mpm *MacroParserManager) SetStyle(style MacroStyle) error {
	if _, exists := mpm.parsers[style]; !exists {
		return fmt.Errorf("unsupported macro style: %v", style)
	}
	mpm.currentStyle = style
	return nil
}

// GetCurrentStyle returns the current macro style
func (mpm *MacroParserManager) GetCurrentStyle() MacroStyle {
	return mpm.currentStyle
}

// ParseMacroDefinition parses a macro definition using the current style
func (mpm *MacroParserManager) ParseMacroDefinition(tokens []Token, startPos int) (*MacroDefinition, int, error) {
	parser, exists := mpm.parsers[mpm.currentStyle]
	if !exists {
		return nil, startPos, fmt.Errorf("no parser available for style: %v", mpm.currentStyle)
	}
	
	if !parser.CanParseMacroDefinition(tokens, startPos) {
		return nil, startPos, fmt.Errorf("tokens do not represent a macro definition for style: %v", mpm.currentStyle)
	}
	
	macro, newPos, err := parser.ParseMacroDefinition(tokens, startPos)
	if err != nil {
		return nil, startPos, err
	}
	
	// Set the style in the macro definition
	macro.Style = mpm.currentStyle
	
	return macro, newPos, nil
}

// ParseMacroCall parses a macro call using the current style
func (mpm *MacroParserManager) ParseMacroCall(tokens []Token, startPos int) (*MacroCall, int, error) {
	parser, exists := mpm.parsers[mpm.currentStyle]
	if !exists {
		return nil, startPos, fmt.Errorf("no parser available for style: %v", mpm.currentStyle)
	}
	
	if !parser.CanParseMacroCall(tokens, startPos) {
		return nil, startPos, fmt.Errorf("tokens do not represent a macro call for style: %v", mpm.currentStyle)
	}
	
	call, newPos, err := parser.ParseMacroCall(tokens, startPos)
	if err != nil {
		return nil, startPos, err
	}
	
	// Set the style in the macro call
	call.Style = mpm.currentStyle
	
	return call, newPos, nil
}

// CanParseMacroDefinition checks if current style can parse a macro definition
func (mpm *MacroParserManager) CanParseMacroDefinition(tokens []Token, startPos int) bool {
	parser, exists := mpm.parsers[mpm.currentStyle]
	if !exists {
		return false
	}
	return parser.CanParseMacroDefinition(tokens, startPos)
}

// CanParseMacroCall checks if current style can parse a macro call
func (mpm *MacroParserManager) CanParseMacroCall(tokens []Token, startPos int) bool {
	parser, exists := mpm.parsers[mpm.currentStyle]
	if !exists {
		return false
	}
	return parser.CanParseMacroCall(tokens, startPos)
}

// RegisterParser adds a parser for a specific macro style
func (mpm *MacroParserManager) RegisterParser(style MacroStyle, parser MacroNotationParser) {
	mpm.parsers[style] = parser
}

// GetSupportedStyles returns all supported macro styles
func (mpm *MacroParserManager) GetSupportedStyles() []MacroStyle {
	var styles []MacroStyle
	for style := range mpm.parsers {
		styles = append(styles, style)
	}
	return styles
}

// Helper functions for token analysis

// TokenSequenceMatches checks if tokens starting at pos match the expected sequence
func TokenSequenceMatches(tokens []Token, startPos int, expected ...TokenType) bool {
	if startPos < 0 || startPos >= len(tokens) {
		return false
	}
	
	if startPos + len(expected) > len(tokens) {
		return false
	}
	
	for i, expectedType := range expected {
		if tokens[startPos + i].Type != expectedType {
			return false
		}
	}
	
	return true
}

// TokenValueMatches checks if token at pos has the expected type and value
func TokenValueMatches(tokens []Token, pos int, expectedType TokenType, expectedValue string) bool {
	if pos < 0 || pos >= len(tokens) {
		return false
	}
	
	token := tokens[pos]
	return token.Type == expectedType && strings.ToUpper(token.Value) == strings.ToUpper(expectedValue)
}

// FindMatchingToken finds the matching closing token for an opening token
func FindMatchingToken(tokens []Token, startPos int, openType, closeType TokenType) int {
	if startPos < 0 || startPos >= len(tokens) || tokens[startPos].Type != openType {
		return -1
	}
	
	depth := 1
	for i := startPos + 1; i < len(tokens); i++ {
		if tokens[i].Type == openType {
			depth++
		} else if tokens[i].Type == closeType {
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	
	return -1 // No matching closing token found
}

// SkipWhitespaceAndComments advances position past whitespace and comments
func SkipWhitespaceAndComments(tokens []Token, startPos int) int {
	pos := startPos
	for pos < len(tokens) {
		token := tokens[pos]
		if token.Type == TokenComment || token.Type == TokenNewline {
			pos++
		} else {
			break
		}
	}
	return pos
}

// FindEndOfStatement finds the end of a statement (newline or semicolon)
func FindEndOfStatement(tokens []Token, startPos int) int {
	for i := startPos; i < len(tokens); i++ {
		token := tokens[i]
		if token.Type == TokenNewline || (token.Type == TokenIdentifier && token.Value == ";") {
			return i
		}
	}
	return len(tokens) // End of input
}
