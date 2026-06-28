package assembler

import (
	"fmt"
	"strings"
)

// TraditionalMacroParser handles traditional assembly macro syntax
// Format: MACRO name(param1, param2, ...)
//           body...
//         ENDMACRO
type TraditionalMacroParser struct {
	baseParser *Parser
	// knownMacros lets the body parser recognise nested macro calls. Set before
	// each ParseMacroDefinition from the macros defined so far.
	knownMacros map[string]bool
}

// SetKnownMacros records which macro names are currently defined, so that nested
// calls inside a subsequently-parsed macro body are recognised as calls.
func (tmp *TraditionalMacroParser) SetKnownMacros(names map[string]bool) {
	tmp.knownMacros = names
}

// NewTraditionalMacroParser creates a new traditional macro parser
func NewTraditionalMacroParser() *TraditionalMacroParser {
	return &TraditionalMacroParser{
		baseParser: NewParser(),
	}
}

// CanParseMacroDefinition checks if tokens represent a traditional macro definition
func (tmp *TraditionalMacroParser) CanParseMacroDefinition(tokens []Token, startPos int) bool {
	return TokenValueMatches(tokens, startPos, TokenIdentifier, "MACRO")
}

// CanParseMacroCall checks if tokens represent a traditional macro call
func (tmp *TraditionalMacroParser) CanParseMacroCall(tokens []Token, startPos int) bool {
	// For traditional style, any identifier followed by optional parameters
	// could be a macro call. We'll need the macro table to definitively identify it.
	// For now, we'll use a simple heuristic: identifier followed by optional parentheses
	if startPos >= len(tokens) || tokens[startPos].Type != TokenIdentifier {
		return false
	}
	
	// Skip the identifier
	pos := startPos + 1
	if pos < len(tokens) && tokens[pos].Type == TokenLParen {
		// Has parentheses, likely a macro call
		return true
	}
	
	// Could be a macro call without parentheses (depends on macro table)
	return true
}

// ParseMacroDefinition parses a traditional macro definition
func (tmp *TraditionalMacroParser) ParseMacroDefinition(tokens []Token, startPos int) (*MacroDefinition, int, error) {
	pos := startPos
	
	// Expect MACRO keyword
	if !TokenValueMatches(tokens, pos, TokenIdentifier, "MACRO") {
		return nil, pos, fmt.Errorf("expected MACRO keyword")
	}
	pos++
	
	// Expect macro name
	if pos >= len(tokens) || tokens[pos].Type != TokenIdentifier {
		return nil, pos, fmt.Errorf("expected macro name after MACRO")
	}
	
	macroName := tokens[pos].Value
	pos++
	
	// Parse parameters if present
	var parameters []*MacroParameter
	
	if pos < len(tokens) && tokens[pos].Type == TokenLParen {
		pos++ // Skip opening parenthesis
		
		// Parse parameter list
		for pos < len(tokens) && tokens[pos].Type != TokenRParen {
			// Skip whitespace and comments
			pos = SkipWhitespaceAndComments(tokens, pos)
			
			if pos >= len(tokens) {
				return nil, pos, fmt.Errorf("unterminated parameter list")
			}
			
			if tokens[pos].Type == TokenRParen {
				break
			}
			
			// Optional width marker before the name: MACRO f(uint8_t a, word b).
			// Absent means untyped (not width-checked). The marker is one of the
			// type keywords recognised by ParseParameterType.
			paramType := TypeUntyped
			if pos+1 < len(tokens) && tokens[pos].Type == TokenIdentifier &&
				isWidthMarker(tokens[pos].Value) &&
				tokens[pos+1].Type == TokenIdentifier {
				if t, err := ParseParameterType(tokens[pos].Value); err == nil {
					paramType = t
				}
				pos++ // consume the marker; the name follows
			}

			// Expect parameter name
			if tokens[pos].Type != TokenIdentifier {
				return nil, pos, fmt.Errorf("expected parameter name")
			}
			
			paramName := tokens[pos].Value
			pos++
			
			param := &MacroParameter{
				Name: paramName,
				Type: paramType,
			}
			
			parameters = append(parameters, param)
			
			// Skip whitespace
			pos = SkipWhitespaceAndComments(tokens, pos)
			
			// Check for comma or closing parenthesis
			if pos < len(tokens) && tokens[pos].Type == TokenComma {
				pos++ // Skip comma
			} else if pos < len(tokens) && tokens[pos].Type == TokenRParen {
				// Will be handled by loop condition
			} else if pos >= len(tokens) {
				return nil, pos, fmt.Errorf("unterminated parameter list")
			}
		}
		
		if pos >= len(tokens) || tokens[pos].Type != TokenRParen {
			return nil, pos, fmt.Errorf("expected closing parenthesis")
		}
		pos++ // Skip closing parenthesis
	}
	
	// Skip to end of line
	pos = FindEndOfStatement(tokens, pos)
	if pos < len(tokens) && tokens[pos].Type == TokenNewline {
		pos++
	}
	
	// Parse macro body until ENDMACRO
	var bodyTokens []Token
	bodyStart := pos
	
	for pos < len(tokens) {
		if TokenValueMatches(tokens, pos, TokenIdentifier, "ENDMACRO") {
			break
		}
		pos++
	}
	
	if pos >= len(tokens) {
		return nil, pos, fmt.Errorf("missing ENDMACRO for macro %s", macroName)
	}
	
	// Extract body tokens
	bodyTokens = tokens[bodyStart:pos]
	
	// Skip ENDMACRO
	pos++
	
	// Parse the body tokens into ParsedLines
	bodyParser := NewParser()
	if tmp.knownMacros != nil {
		bodyParser.SetMacroNames(tmp.knownMacros)
	}
	bodyProgram, err := bodyParser.Parse(bodyTokens)
	if err != nil {
		return nil, pos, fmt.Errorf("error parsing macro body: %v", err)
	}
	
	macro := &MacroDefinition{
		Name:       macroName,
		Parameters: parameters,
		ReturnType: TypeUint8, // Traditional macros don't specify return type
		Body:       bodyProgram.Lines,
		Style:      MacroStyleTraditional,
		LineNumber: tokens[startPos].Line,
	}
	
	return macro, pos, nil
}

// isWidthMarker reports whether a token value is a value-width type keyword
// (uint8_t/uint16_t and their aliases). These are the markers that carry a
// checkable argument width; register/address types are not treated as markers
// here.
func isWidthMarker(value string) bool {
	switch strings.ToLower(value) {
	case "uint8_t", "uint8", "byte", "uint16_t", "uint16", "word":
		return true
	}
	return false
}

// returnWidthBits maps a function return-type keyword to its width in bits and
// whether it returns a value at all. void returns nothing (0, false); the uintN
// markers return their width.
func returnWidthBits(returnType string) (bits int, returnsValue bool) {
	switch strings.ToLower(returnType) {
	case "uint8_t", "uint8", "byte":
		return 8, true
	case "uint16_t", "uint16", "word", "address_t", "addr":
		return 16, true
	default: // "void" and anything else
		return 0, false
	}
}

// ParseMacroCall parses a traditional macro call
func (tmp *TraditionalMacroParser) ParseMacroCall(tokens []Token, startPos int) (*MacroCall, int, error) {
	pos := startPos
	
	// Expect macro name
	if pos >= len(tokens) || tokens[pos].Type != TokenIdentifier {
		return nil, pos, fmt.Errorf("expected macro name")
	}
	
	macroName := tokens[pos].Value
	pos++
	
	var arguments []*Expression
	
	// Check for parameters
	if pos < len(tokens) && tokens[pos].Type == TokenLParen {
		pos++ // Skip opening parenthesis
		
		// Parse argument list
		for pos < len(tokens) && tokens[pos].Type != TokenRParen {
			// Skip whitespace and comments
			pos = SkipWhitespaceAndComments(tokens, pos)
			
			if pos >= len(tokens) {
				return nil, pos, fmt.Errorf("unterminated argument list")
			}
			
			if tokens[pos].Type == TokenRParen {
				break
			}
			
			// Parse expression argument
			expr, newPos, err := tmp.parseExpressionFromTokens(tokens, pos)
			if err != nil {
				return nil, pos, fmt.Errorf("error parsing argument: %v", err)
			}
			
			arguments = append(arguments, expr)
			pos = newPos
			
			// Skip whitespace
			pos = SkipWhitespaceAndComments(tokens, pos)
			
			// Check for comma or closing parenthesis
			if pos < len(tokens) && tokens[pos].Type == TokenComma {
				pos++ // Skip comma
			} else if pos < len(tokens) && tokens[pos].Type == TokenRParen {
				// Will be handled by loop condition
			} else if pos >= len(tokens) {
				return nil, pos, fmt.Errorf("unterminated argument list")
			}
		}
		
		if pos >= len(tokens) || tokens[pos].Type != TokenRParen {
			return nil, pos, fmt.Errorf("expected closing parenthesis")
		}
		pos++ // Skip closing parenthesis
	}
	
	call := &MacroCall{
		Name:       macroName,
		Arguments:  arguments,
		Style:      MacroStyleTraditional,
		LineNumber: tokens[startPos].Line,
	}
	
	return call, pos, nil
}

// parseExpressionFromTokens parses a simple expression from tokens
func (tmp *TraditionalMacroParser) parseExpressionFromTokens(tokens []Token, startPos int) (*Expression, int, error) {
	pos := startPos
	
	if pos >= len(tokens) {
		return nil, pos, fmt.Errorf("unexpected end of input")
	}
	
	token := tokens[pos]
	
	switch token.Type {
	case TokenNumber:
		value, err := ParseNumber(token)
		if err != nil {
			return nil, pos, fmt.Errorf("invalid number: %v", err)
		}
		pos++
		return &Expression{
			Type:  ExpressionNumber,
			Value: value,
		}, pos, nil
		
	case TokenIdentifier:
		pos++
		return &Expression{
			Type:   ExpressionSymbol,
			Symbol: token.Value,
		}, pos, nil
		
	case TokenString:
		pos++
		return &Expression{
			Type:        ExpressionString,
			StringValue: token.Value,
		}, pos, nil
		
	case TokenMinus:
		// Handle negative numbers
		pos++
		if pos >= len(tokens) || tokens[pos].Type != TokenNumber {
			return nil, pos, fmt.Errorf("expected number after minus sign")
		}
		
		value, err := ParseNumber(tokens[pos])
		if err != nil {
			return nil, pos, fmt.Errorf("invalid number: %v", err)
		}
		pos++
		
		return &Expression{
			Type:  ExpressionNumber,
			Value: -value,
		}, pos, nil
		
	default:
		return nil, pos, fmt.Errorf("unexpected token in expression: %s", token.Type)
	}
}

// IsTraditionalMacroKeyword checks if a token is a traditional macro keyword
func IsTraditionalMacroKeyword(token Token) bool {
	if token.Type != TokenIdentifier {
		return false
	}
	
	keywords := []string{"MACRO", "ENDMACRO"}
	value := strings.ToUpper(token.Value)
	
	for _, keyword := range keywords {
		if value == keyword {
			return true
		}
	}
	
	return false
}
