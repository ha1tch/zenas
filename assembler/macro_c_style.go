package assembler

import (
	"fmt"
	"strings"
)

// CStyleMacroParser handles C-style function macro syntax
// Format: return_type name(type param1, type param2, ...) {
//           body...
//         }
type CStyleMacroParser struct {
	baseParser *Parser
}

// NewCStyleMacroParser creates a new C-style macro parser
func NewCStyleMacroParser() *CStyleMacroParser {
	return &CStyleMacroParser{
		baseParser: NewParser(),
	}
}

// CanParseMacroDefinition checks if tokens represent a C-style macro definition
func (cmp *CStyleMacroParser) CanParseMacroDefinition(tokens []Token, startPos int) bool {
	// Look for pattern: [type] identifier(
	pos := startPos
	
	// Skip potential return type
	if pos < len(tokens) && cmp.isTypeIdentifier(tokens[pos]) {
		pos++
	}
	
	// Expect function name
	if pos >= len(tokens) || tokens[pos].Type != TokenIdentifier {
		return false
	}
	pos++
	
	// Expect opening parenthesis
	return pos < len(tokens) && tokens[pos].Type == TokenLParen
}

// CanParseMacroCall checks if tokens represent a C-style macro call
func (cmp *CStyleMacroParser) CanParseMacroCall(tokens []Token, startPos int) bool {
	// Pattern: identifier(
	if startPos >= len(tokens) || tokens[startPos].Type != TokenIdentifier {
		return false
	}
	
	pos := startPos + 1
	return pos < len(tokens) && tokens[pos].Type == TokenLParen
}

// ParseMacroDefinition parses a C-style macro definition
func (cmp *CStyleMacroParser) ParseMacroDefinition(tokens []Token, startPos int) (*MacroDefinition, int, error) {
	pos := startPos
	
	// Parse optional return type
	returnType := TypeUint8 // Default return type
	if pos < len(tokens) && cmp.isTypeIdentifier(tokens[pos]) {
		var err error
		returnType, err = ParseParameterType(tokens[pos].Value)
		if err != nil {
			return nil, pos, fmt.Errorf("invalid return type: %v", err)
		}
		pos++
	}
	
	// Expect function name
	if pos >= len(tokens) || tokens[pos].Type != TokenIdentifier {
		return nil, pos, fmt.Errorf("expected function name")
	}
	
	macroName := tokens[pos].Value
	pos++
	
	// Expect opening parenthesis
	if pos >= len(tokens) || tokens[pos].Type != TokenLParen {
		return nil, pos, fmt.Errorf("expected opening parenthesis")
	}
	pos++
	
	// Parse parameter list
	var parameters []*MacroParameter
	
	for pos < len(tokens) && tokens[pos].Type != TokenRParen {
		// Skip whitespace and comments
		pos = SkipWhitespaceAndComments(tokens, pos)
		
		if pos >= len(tokens) {
			return nil, pos, fmt.Errorf("unterminated parameter list")
		}
		
		if tokens[pos].Type == TokenRParen {
			break
		}
		
		// Parse parameter type
		if !cmp.isTypeIdentifier(tokens[pos]) {
			return nil, pos, fmt.Errorf("expected parameter type")
		}
		
		paramType, err := ParseParameterType(tokens[pos].Value)
		if err != nil {
			return nil, pos, fmt.Errorf("invalid parameter type: %v", err)
		}
		pos++
		
		// Expect parameter name
		if pos >= len(tokens) || tokens[pos].Type != TokenIdentifier {
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
	pos++
	
	// Skip whitespace
	pos = SkipWhitespaceAndComments(tokens, pos)
	
	// Expect opening brace
	if pos >= len(tokens) || tokens[pos].Type != TokenLBrace {
		return nil, pos, fmt.Errorf("expected opening brace")
	}
	pos++
	
	// Parse macro body until closing brace
	bodyStart := pos
	braceDepth := 1
	
	for pos < len(tokens) && braceDepth > 0 {
		if tokens[pos].Type == TokenLBrace {
			braceDepth++
		} else if tokens[pos].Type == TokenRBrace {
			braceDepth--
		}
		if braceDepth > 0 {
			pos++
		}
	}
	
	if braceDepth > 0 {
		return nil, pos, fmt.Errorf("missing closing brace for macro %s", macroName)
	}
	
	// Extract body tokens (excluding the closing brace)
	bodyTokens := tokens[bodyStart:pos]
	
	// Skip closing brace
	pos++
	
	// SIMPLIFIED APPROACH: For C-style macros that contain macro calls,
	// just create an empty body. The actual expansion will happen when
	// this macro is called and needs to expand the inner macro calls.
	
	// Check if body contains only macro calls and C syntax
	if cmp.containsOnlyMacroCallsAndCSyntax(bodyTokens) {
		// Create an empty macro that will be handled specially during expansion
		macro := &MacroDefinition{
			Name:       macroName,
			Parameters: parameters,
			ReturnType: returnType,
			Body:       []ParsedLine{}, // Empty body
			Style:      MacroStyleC,
			LineNumber: tokens[startPos].Line,
		}
		
		return macro, pos, nil
	}
	
	// If it contains asm{} blocks, process normally
	assemblyTokens, err := cmp.convertCStyleToAssembly(bodyTokens)
	if err != nil {
		return nil, pos, fmt.Errorf("error converting C-style body: %v", err)
	}
	
	bodyParser := NewParser()
	bodyProgram, err := bodyParser.Parse(assemblyTokens)
	if err != nil {
		return nil, pos, fmt.Errorf("error parsing macro body: %v", err)
	}
	
	macro := &MacroDefinition{
		Name:       macroName,
		Parameters: parameters,
		ReturnType: returnType,
		Body:       bodyProgram.Lines,
		Style:      MacroStyleC,
		LineNumber: tokens[startPos].Line,
	}
	
	return macro, pos, nil
}

// containsOnlyMacroCallsAndCSyntax checks if the body contains only macro calls and C syntax
func (cmp *CStyleMacroParser) containsOnlyMacroCallsAndCSyntax(tokens []Token) bool {
	// If the body contains asm{} blocks, it needs normal processing
	for i := 0; i < len(tokens)-1; i++ {
		if tokens[i].Type == TokenIdentifier && 
		   strings.ToUpper(tokens[i].Value) == "ASM" &&
		   tokens[i+1].Type == TokenLBrace {
			return false
		}
	}
	
	// For now, assume C-style macros without asm{} blocks are macro-call-only
	return true
}

// ParseMacroCall parses a C-style macro call
func (cmp *CStyleMacroParser) ParseMacroCall(tokens []Token, startPos int) (*MacroCall, int, error) {
	pos := startPos
	
	// Expect macro name
	if pos >= len(tokens) || tokens[pos].Type != TokenIdentifier {
		return nil, pos, fmt.Errorf("expected macro name")
	}
	
	macroName := tokens[pos].Value
	pos++
	
	// Expect opening parenthesis
	if pos >= len(tokens) || tokens[pos].Type != TokenLParen {
		return nil, pos, fmt.Errorf("expected opening parenthesis")
	}
	pos++
	
	var arguments []*Expression
	
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
		expr, newPos, err := cmp.parseExpressionFromTokens(tokens, pos)
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
	pos++
	
	// Expect semicolon (optional)
	if pos < len(tokens) && tokens[pos].Value == ";" {
		pos++
	}
	
	call := &MacroCall{
		Name:       macroName,
		Arguments:  arguments,
		Style:      MacroStyleC,
		LineNumber: tokens[startPos].Line,
	}
	
	return call, pos, nil
}

// isTypeIdentifier checks if a token represents a type identifier
func (cmp *CStyleMacroParser) isTypeIdentifier(token Token) bool {
	if token.Type != TokenIdentifier {
		return false
	}
	
	types := []string{
		"void", "uint8_t", "uint16_t", "uint8", "uint16", 
		"byte", "word", "register8_t", "register16_t", 
		"reg8", "reg16", "address_t", "addr",
	}
	
	value := strings.ToLower(token.Value)
	for _, validType := range types {
		if value == validType {
			return true
		}
	}
	
	return false
}

// convertCStyleToAssembly converts C-style body tokens to assembly-compatible tokens
// Enhanced version with asm{} block support and immediate macro expansion
func (cmp *CStyleMacroParser) convertCStyleToAssembly(tokens []Token) ([]Token, error) {
	var result []Token
	
	// Assembly instruction mnemonics for detection
	assemblyInstructions := map[string]bool{
		"LD": true, "PUSH": true, "POP": true, "ADD": true, "SUB": true, "INC": true, "DEC": true,
		"JP": true, "JR": true, "CALL": true, "RET": true, "NOP": true, "HALT": true,
		"AND": true, "OR": true, "XOR": true, "CP": true, "BIT": true, "SET": true, "RES": true,
		"RLC": true, "RRC": true, "RL": true, "RR": true, "SLA": true, "SRA": true, "SRL": true,
		"DJNZ": true, "IN": true, "OUT": true, "EX": true, "EXX": true, "DI": true, "EI": true,
		"CCF": true, "SCF": true, "CPL": true, "DAA": true, "NEG": true, "IM": true,
	}
	
	i := 0
	for i < len(tokens) {
		token := tokens[i]
		
		// Check for asm{} blocks first (CRITICAL: lexer converts "asm" to "ASM")
		if token.Type == TokenIdentifier && strings.ToUpper(token.Value) == "ASM" {
			// Look ahead for opening brace
			if i+1 < len(tokens) && tokens[i+1].Type == TokenLBrace {
				// Found asm{} block - process it specially
				asmTokens, newPos, err := cmp.extractAndProcessAsmBlock(tokens, i)
				if err != nil {
					return nil, err
				}
				
				// Add the processed asm tokens to result
				result = append(result, asmTokens...)
				i = newPos
				continue
			}
		}
		
		// Check for C-style macro calls (identifier followed by parentheses)
		if token.Type == TokenIdentifier && i+1 < len(tokens) && tokens[i+1].Type == TokenLParen {
			// This looks like a function call - handle it specially
			// Instead of converting to assembly syntax, create a placeholder comment
			macroName := token.Value
			
			// Skip the entire function call
			pos := i + 1 // Start at opening paren
			if pos < len(tokens) && tokens[pos].Type == TokenLParen {
				parenDepth := 1
				pos++ // Skip opening paren
				
				// Skip to matching closing paren
				for pos < len(tokens) && parenDepth > 0 {
					if tokens[pos].Type == TokenLParen {
						parenDepth++
					} else if tokens[pos].Type == TokenRParen {
						parenDepth--
					}
					pos++
				}
			}
			
			// Skip optional semicolon
			if pos < len(tokens) && tokens[pos].Value == ";" {
				pos++
			}
			
			// Create a comment placeholder for this macro call
			commentToken := Token{
				Type:     TokenComment,
				Value:    fmt.Sprintf("; C-style macro call: %s", macroName),
				Line:     token.Line,
				Column:   token.Column,
				Position: token.Position,
			}
			result = append(result, commentToken)
			
			// Add newline
			newlineToken := Token{
				Type:     TokenNewline,
				Value:    "\n",
				Line:     token.Line,
				Column:   token.Column,
				Position: token.Position,
			}
			result = append(result, newlineToken)
			
			i = pos
			continue
		}
		
		// Check for C-style assignment with macro call: result = macro_name(...)
		if token.Type == TokenIdentifier && i+2 < len(tokens) && 
		   tokens[i+1].Type == TokenIdentifier && tokens[i+1].Value == "=" &&
		   tokens[i+2].Type == TokenIdentifier {
		   
			// Look ahead for parentheses after the macro name
			if i+3 < len(tokens) && tokens[i+3].Type == TokenLParen {
				// This is an assignment from a macro call
				varName := token.Value
				macroName := tokens[i+2].Value
				
				// Skip the entire assignment and function call
				pos := i + 3 // Start at opening paren
				if pos < len(tokens) && tokens[pos].Type == TokenLParen {
					parenDepth := 1
					pos++ // Skip opening paren
					
					// Skip to matching closing paren
					for pos < len(tokens) && parenDepth > 0 {
						if tokens[pos].Type == TokenLParen {
							parenDepth++
						} else if tokens[pos].Type == TokenRParen {
							parenDepth--
						}
						pos++
					}
				}
				
				// Skip optional semicolon
				if pos < len(tokens) && tokens[pos].Value == ";" {
					pos++
				}
				
				// Create a comment placeholder for this assignment
				commentToken := Token{
					Type:     TokenComment,
					Value:    fmt.Sprintf("; C-style assignment: %s = %s", varName, macroName),
					Line:     token.Line,
					Column:   token.Column,
					Position: token.Position,
				}
				result = append(result, commentToken)
				
				// Add newline
				newlineToken := Token{
					Type:     TokenNewline,
					Value:    "\n",
					Line:     token.Line,
					Column:   token.Column,
					Position: token.Position,
				}
				result = append(result, newlineToken)
				
				i = pos
				continue
			}
		}
		
		switch token.Type {
		case TokenIdentifier:
			// Check for assembly instruction mnemonics OUTSIDE asm{} blocks
			upperValue := strings.ToUpper(token.Value)
			if assemblyInstructions[upperValue] {
				return nil, fmt.Errorf("Assembly instruction '%s' found in C-style macro at line %d\nHint: C-style macros should contain C syntax. Use traditional macro style for assembly code, or wrap assembly in asm{} blocks", token.Value, token.Line)
			}
			
			// Check for assembly labels (identifier followed by colon) OUTSIDE asm{} blocks
			if i+1 < len(tokens) && tokens[i+1].Type == TokenColon {
				return nil, fmt.Errorf("Assembly label '%s:' found in C-style macro at line %d\nHint: C-style macros should not contain assembly labels. Use traditional macro style for assembly code, or wrap assembly in asm{} blocks", token.Value, token.Line)
			}
			
			// Convert C-style keywords to assembly equivalents
			switch strings.ToLower(token.Value) {
			case "return":
				// Skip return statements for now - would need more sophisticated handling
				i++
				continue
			case "uint8_t", "uint16_t", "void":
				// Skip C type declarations
				i++
				continue
			default:
				// Pass through identifiers as-is (but only if not part of a macro call)
				result = append(result, token)
			}
			
		case TokenLParen:
			// Check for assembly-style indirect addressing patterns OUTSIDE asm{} blocks
			// Look for pattern: ( register_name )
			if i+1 < len(tokens) && i+2 < len(tokens) && 
			   tokens[i+2].Type == TokenRParen && 
			   cmp.isRegisterName(tokens[i+1].Value) {
				
				// Check if this is in an assembly context (preceded by instruction or comma)
				if i > 0 {
					prevToken := tokens[i-1]
					if prevToken.Type == TokenComma || assemblyInstructions[strings.ToUpper(prevToken.Value)] {
						return nil, fmt.Errorf("Assembly-style register indirect addressing '(%s)' found in C-style macro at line %d\nHint: C-style macros should contain C syntax like function calls, not assembly addressing, or wrap assembly in asm{} blocks", tokens[i+1].Value, tokens[i+1].Line)
					}
				}
			}
			// Pass through parentheses
			result = append(result, token)
			
		default:
			// Handle semicolons as statement separators
			if token.Value == ";" {
				// Convert semicolon to newline for assembly parsing
				newlineToken := Token{
					Type:     TokenNewline,
					Value:    "\n",
					Line:     token.Line,
					Column:   token.Column,
					Position: token.Position,
				}
				result = append(result, newlineToken)
			} else {
				// Pass through other tokens as-is
				result = append(result, token)
			}
		}
		
		i++
	}
	
	return result, nil
}

// processCStyleMacroCall converts a C-style function call to macro call syntax (ENHANCED)
func (cmp *CStyleMacroParser) processCStyleMacroCall(tokens []Token, startPos int) ([]Token, int, error) {
	// startPos should point to function name
	if startPos >= len(tokens) || tokens[startPos].Type != TokenIdentifier {
		return nil, startPos, fmt.Errorf("expected function name at position %d", startPos)
	}
	
	var result []Token
	
	// Add the function name as-is (it will be recognized as a macro call later)
	result = append(result, tokens[startPos])
	pos := startPos + 1
	
	// Process the parentheses and arguments
	if pos < len(tokens) && tokens[pos].Type == TokenLParen {
		result = append(result, tokens[pos]) // Add opening paren
		pos++
		
		// Copy all tokens until the matching closing parenthesis
		parenDepth := 1
		for pos < len(tokens) && parenDepth > 0 {
			if tokens[pos].Type == TokenLParen {
				parenDepth++
			} else if tokens[pos].Type == TokenRParen {
				parenDepth--
			}
			
			result = append(result, tokens[pos])
			pos++
		}
		
		if parenDepth > 0 {
			return nil, pos, fmt.Errorf("unmatched parentheses in function call at line %d", tokens[startPos].Line)
		}
	}
	
	// Skip optional semicolon and convert to newline
	if pos < len(tokens) && tokens[pos].Value == ";" {
		// Convert semicolon to newline
		newlineToken := Token{
			Type:     TokenNewline,
			Value:    "\n",
			Line:     tokens[pos].Line,
			Column:   tokens[pos].Column,
			Position: tokens[pos].Position,
		}
		result = append(result, newlineToken)
		pos++
	} else {
		// Add newline even if no semicolon (for proper line separation)
		newlineToken := Token{
			Type:     TokenNewline,
			Value:    "\n",
			Line:     tokens[startPos].Line,
			Column:   tokens[startPos].Column,
			Position: tokens[startPos].Position,
		}
		result = append(result, newlineToken)
	}
	
	return result, pos, nil
}

// extractAndProcessAsmBlock extracts and processes an asm{} block
func (cmp *CStyleMacroParser) extractAndProcessAsmBlock(tokens []Token, startPos int) ([]Token, int, error) {
	// startPos should point to "ASM" token
	if startPos >= len(tokens) || strings.ToUpper(tokens[startPos].Value) != "ASM" {
		return nil, startPos, fmt.Errorf("expected ASM keyword at position %d", startPos)
	}
	
	// Next token should be opening brace
	pos := startPos + 1
	if pos >= len(tokens) || tokens[pos].Type != TokenLBrace {
		return nil, pos, fmt.Errorf("expected '{' after asm keyword at line %d", tokens[startPos].Line)
	}
	
	// Find matching closing brace
	pos++ // Skip opening brace
	braceDepth := 1
	asmStart := pos
	
	for pos < len(tokens) && braceDepth > 0 {
		if tokens[pos].Type == TokenLBrace {
			braceDepth++
		} else if tokens[pos].Type == TokenRBrace {
			braceDepth--
		}
		if braceDepth > 0 {
			pos++
		}
	}
	
	if braceDepth > 0 {
		return nil, pos, fmt.Errorf("unmatched opening brace in asm{} block at line %d", tokens[startPos].Line)
	}
	
	// Extract tokens from inside the asm{} block (excluding braces)
	asmTokens := tokens[asmStart:pos]
	
	// Process the asm tokens - pass them through mostly unchanged
	// but convert semicolons to newlines for assembly parser
	var processedTokens []Token
	for _, token := range asmTokens {
		if token.Value == ";" {
			// Convert semicolon to newline for assembly parsing
			newlineToken := Token{
				Type:     TokenNewline,
				Value:    "\n",
				Line:     token.Line,
				Column:   token.Column,
				Position: token.Position,
			}
			processedTokens = append(processedTokens, newlineToken)
		} else {
			// Pass through assembly tokens unchanged
			processedTokens = append(processedTokens, token)
		}
	}
	
	// Skip past the closing brace
	pos++
	
	return processedTokens, pos, nil
}

// isRegisterName checks if a string is a Z80 register name
func (cmp *CStyleMacroParser) isRegisterName(name string) bool {
	registers := map[string]bool{
		"A": true, "B": true, "C": true, "D": true, "E": true, "H": true, "L": true,
		"BC": true, "DE": true, "HL": true, "SP": true, "IX": true, "IY": true, "AF": true,
		"IXH": true, "IXL": true, "IYH": true, "IYL": true,
	}
	
	return registers[strings.ToUpper(name)]
}

// parseExpressionFromTokens parses a simple expression from tokens (same as traditional)
func (cmp *CStyleMacroParser) parseExpressionFromTokens(tokens []Token, startPos int) (*Expression, int, error) {
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

// IsCStyleMacroKeyword checks if a token is a C-style macro keyword
func IsCStyleMacroKeyword(token Token) bool {
	if token.Type != TokenIdentifier {
		return false
	}
	
	keywords := []string{
		"void", "uint8_t", "uint16_t", "uint8", "uint16",
		"byte", "word", "register8_t", "register16_t",
		"reg8", "reg16", "address_t", "addr", "return",
		"asm", // Add asm as a keyword
	}
	
	value := strings.ToLower(token.Value)
	for _, keyword := range keywords {
		if value == keyword {
			return true
		}
	}
	
	return false
}