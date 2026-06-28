package assembler

import (
	"fmt"
	"strings"
)

// CStyleConverter handles the conversion of C-style macro tokens to traditional assembly tokens
// This is the critical missing piece identified in the development journey analysis
type CStyleConverter struct {
	macroTable  *MacroTable
	symbolTable *SymbolTable
	uniqueID    int
	debug       bool // NEW: Debug flag
}

// ConversionState tracks the position during token conversion
type ConversionState struct {
	tokens   []Token
	position int
}

// NewCStyleConverter creates a new C-style token converter
func NewCStyleConverter(macroTable *MacroTable, symbolTable *SymbolTable) *CStyleConverter {
	return &CStyleConverter{
		macroTable:  macroTable,
		symbolTable: symbolTable,
		uniqueID:    1,
		debug:       false, // debug tracing off; flip to true only when diagnosing
	}
}

// ConvertToTraditional converts C-style tokens to traditional assembly tokens
// This is the main entry point that fixes the broken token conversion pipeline
func (c *CStyleConverter) ConvertToTraditional(tokens []Token) ([]Token, error) {
	if c.debug {
		fmt.Printf("[DEBUG] ConvertToTraditional called with %d tokens\n", len(tokens))
	}
	
	var result []Token
	state := &ConversionState{
		tokens:   tokens,
		position: 0,
	}

	for state.position < len(tokens) {
		token := state.current()
		if c.debug {
			fmt.Printf("[DEBUG] Processing token %d: %s(%q)\n", state.position, token.Type, token.Value)
		}
		
		// Skip whitespace and comments
		if c.isWhitespaceOrComment(state) {
			if c.debug {
				fmt.Printf("[DEBUG] Skipping whitespace/comment\n")
			}
			result = append(result, state.current())
			state.advance()
			continue
		}

		// Handle different C-style constructs
		switch {
		case c.isVariableDeclaration(state):
			if c.debug {
				fmt.Printf("[DEBUG] Detected variable declaration\n")
			}
			converted, err := c.convertVariableDeclaration(state)
			if err != nil {
				return nil, err
			}
			result = append(result, converted...)

		case c.isAssignment(state):
			if c.debug {
				fmt.Printf("[DEBUG] Detected assignment\n")
			}
			converted, err := c.convertAssignment(state)
			if err != nil {
				return nil, err
			}
			result = append(result, converted...)

		case c.isFunctionDefinition(state):
			if c.debug {
				fmt.Printf("[DEBUG] Detected function definition: %s\n", token.Value)
			}
			converted, err := c.convertFunctionDefinition(state)
			if err != nil {
				return nil, err
			}
			result = append(result, converted...)

		case c.isAsmBlock(state):
			if c.debug {
				fmt.Printf("[DEBUG] Detected asm block\n")
			}
			converted, err := c.convertAsmBlock(state)
			if err != nil {
				return nil, err
			}
			result = append(result, converted...)

		case c.isStandaloneCall(state):
			if c.debug {
				fmt.Printf("[DEBUG] Detected standalone call: %s\n", token.Value)
			}
			converted, err := c.convertStandaloneCall(state)
			if err != nil {
				return nil, err
			}
			result = append(result, converted...)

		default:
			if c.debug {
				fmt.Printf("[DEBUG] Passing through token: %s(%q)\n", token.Type, token.Value)
			}
			// Pass through tokens that don't need conversion
			result = append(result, state.current())
			state.advance()
		}
	}

	if c.debug {
		fmt.Printf("[DEBUG] ConvertToTraditional completed, returning %d tokens\n", len(result))
		for i, token := range result {
			fmt.Printf("[DEBUG] Result[%d]: %s(%q)\n", i, token.Type, token.Value)
		}
	}

	return result, nil
}

// Helper methods for ConversionState
func (s *ConversionState) current() Token {
	if s.position >= len(s.tokens) {
		return Token{Type: TokenEOF}
	}
	return s.tokens[s.position]
}

func (s *ConversionState) peek(offset int) Token {
	pos := s.position + offset
	if pos >= len(s.tokens) {
		return Token{Type: TokenEOF}
	}
	return s.tokens[pos]
}

func (s *ConversionState) advance() {
	if s.position < len(s.tokens) {
		s.position++
	}
}

func (s *ConversionState) advanceBy(count int) {
	s.position += count
	if s.position > len(s.tokens) {
		s.position = len(s.tokens)
	}
}

// Detection methods

func (c *CStyleConverter) isWhitespaceOrComment(state *ConversionState) bool {
	token := state.current()
	return token.Type == TokenComment || token.Type == TokenNewline
}

// isReturnStatement reports whether the cursor is at a C-style `return`.
func (c *CStyleConverter) isReturnStatement(state *ConversionState) bool {
	return state.current().Type == TokenIdentifier &&
		strings.ToLower(state.current().Value) == "return"
}

// convertReturn handles a `return [expr];` statement. The return width is fixed
// by the function's signature: a value's width must match it exactly, a void
// function must not return a value, and a non-void function must return one.
// On success it emits a RET (the control-flow half); placing the value in a
// result location is the primitive tier's responsibility, not zenas's.
func (c *CStyleConverter) convertReturn(state *ConversionState, returnType string) ([]Token, error) {
	line := state.current().Line
	state.advance() // consume 'return'

	declaredBits, returnsValue := returnWidthBits(returnType)

	// Collect the return expression tokens (if any) up to the terminating ';'.
	var exprTokens []Token
	for state.current().Type != TokenSemicolon &&
		state.current().Type != TokenEOF &&
		state.current().Type != TokenRBrace &&
		state.current().Type != TokenNewline {
		exprTokens = append(exprTokens, state.current())
		state.advance()
	}
	if state.current().Type == TokenSemicolon {
		state.advance() // consume ';'
	}

	hasValue := len(exprTokens) > 0

	// Enforce the signature contract.
	if hasValue && !returnsValue {
		return nil, fmt.Errorf("line %d: void function returns no value", line)
	}
	if !hasValue && returnsValue {
		return nil, fmt.Errorf("line %d: function declares a %d-bit return but returns no value", line, declaredBits)
	}

	// Width check: only a single literal number has a knowable width here.
	if hasValue && returnsValue && len(exprTokens) == 1 && exprTokens[0].Type == TokenNumber {
		if v, err := ParseNumber(exprTokens[0]); err == nil {
			argBits := 8
			if v < 0 {
				v = -v
			}
			if v > 0xFF {
				argBits = 16
			}
			if argBits != declaredBits {
				return nil, fmt.Errorf(
					"line %d: function declares a %d-bit return but returns a %d-bit value; "+
						"widths must match", line, declaredBits, argBits)
			}
		}
	}

	// Emit RET. (The return value, if any, is left wherever the body placed it;
	// the calling/return convention is the primitive tier's concern.)
	return []Token{
		{Type: TokenIdentifier, Value: "RET", Line: line},
		{Type: TokenNewline, Value: "\n", Line: line},
	}, nil
}

func (c *CStyleConverter) isVariableDeclaration(state *ConversionState) bool {
	// Pattern: type_name variable_name;
	// Example: uint8_t result;
	token := state.current()
	if token.Type != TokenIdentifier {
		return false
	}

	// Check if it's a type keyword
	if !c.isTypeKeyword(token.Value) {
		return false
	}

	// Look ahead for variable name
	next := state.peek(1)
	if next.Type != TokenIdentifier {
		return false
	}

	// Look ahead for semicolon
	semicolon := state.peek(2)
	return semicolon.Type == TokenSemicolon
}

func (c *CStyleConverter) isAssignment(state *ConversionState) bool {
	// Pattern: variable = expression;
	// Example: result = add_two(5);
	if state.current().Type != TokenIdentifier {
		return false
	}

	equals := state.peek(1)
	result := equals.Type == TokenEquals
	
	if c.debug && result {
		fmt.Printf("[DEBUG] isAssignment: %s = ...\n", state.current().Value)
	}
	
	return result
}

func (c *CStyleConverter) isFunctionDefinition(state *ConversionState) bool {
	// Pattern: return_type function_name(params) { ... }
	// Example: uint8_t add_two(uint8_t a) { ... }
	
	// Look for return type (optional)
	pos := 0
	if c.isTypeKeyword(state.peek(pos).Value) {
		pos++
	}

	// Function name
	if state.peek(pos).Type != TokenIdentifier {
		return false
	}
	pos++

	// Opening parenthesis
	result := state.peek(pos).Type == TokenLParen
	
	if c.debug && result {
		fmt.Printf("[DEBUG] isFunctionDefinition: detected function starting with %s\n", state.current().Value)
	}
	
	return result
}

func (c *CStyleConverter) isAsmBlock(state *ConversionState) bool {
	// Pattern: asm { ... }
	token := state.current()
	if token.Type != TokenIdentifier || strings.ToUpper(token.Value) != "ASM" {
		return false
	}

	next := state.peek(1)
	result := next.Type == TokenLBrace
	
	if c.debug && result {
		fmt.Printf("[DEBUG] isAsmBlock: detected asm block\n")
	}
	
	return result
}

func (c *CStyleConverter) isStandaloneCall(state *ConversionState) bool {
	// Pattern: function_name(args);
	// Example: set_led(); or test_func();
	
	// Must start with identifier
	if state.current().Type != TokenIdentifier {
		if c.debug {
			fmt.Printf("[DEBUG] isStandaloneCall: not identifier (%s)\n", state.current().Type)
		}
		return false
	}
	
	// Check that it's not a type keyword (avoid false positives)
	if c.isTypeKeyword(state.current().Value) {
		if c.debug {
			fmt.Printf("[DEBUG] isStandaloneCall: %s is type keyword\n", state.current().Value)
		}
		return false
	}
	
	// Next token must be opening parenthesis  
	paren := state.peek(1)
	result := paren.Type == TokenLParen
	
	if c.debug {
		fmt.Printf("[DEBUG] isStandaloneCall: %s + %s = %v\n", state.current().Value, paren.Type, result)
	}
	
	return result
}

func (c *CStyleConverter) isTypeKeyword(value string) bool {
	types := map[string]bool{
		"void": true, "uint8_t": true, "uint16_t": true,
		"uint8": true, "uint16": true, "byte": true, "word": true,
		"register8_t": true, "register16_t": true,
		"reg8": true, "reg16": true, "address_t": true, "addr": true,
	}
	return types[strings.ToLower(value)]
}

// Conversion methods

func (c *CStyleConverter) convertVariableDeclaration(state *ConversionState) ([]Token, error) {
	// Convert: uint8_t result; → result: DS 1
	//          uint16_t addr;  → addr: DS 2

	typeName := state.current().Value
	state.advance()

	varName := state.current().Value
	state.advance()

	// Skip semicolon
	if state.current().Type == TokenSemicolon {
		state.advance()
	}

	// Determine size based on type
	size := 1
	if strings.Contains(strings.ToLower(typeName), "16") || 
	   strings.ToLower(typeName) == "word" || 
	   strings.ToLower(typeName) == "address_t" {
		size = 2
	}

	return []Token{
		{Type: TokenIdentifier, Value: varName, Line: state.current().Line},
		{Type: TokenColon, Value: ":", Line: state.current().Line},
		{Type: TokenDirective, Value: ".DS", Line: state.current().Line},
		{Type: TokenNumber, Value: fmt.Sprintf("%d", size), Line: state.current().Line},
		{Type: TokenNewline, Value: "\n", Line: state.current().Line},
	}, nil
}

func (c *CStyleConverter) convertAssignment(state *ConversionState) ([]Token, error) {
	// Convert: result = add_two(5); → [macro call setup] + [result storage]
	//          result = 42;         → LD result, 42

	varName := state.current().Value
	state.advance() // skip variable name
	state.advance() // skip equals sign

	if c.isMacroCallAtPosition(state) {
		return c.convertMacroCallAssignment(state, varName)
	} else {
		return c.convertDirectAssignment(state, varName)
	}
}

func (c *CStyleConverter) convertDirectAssignment(state *ConversionState, varName string) ([]Token, error) {
	// Convert: result = 42; → LD HL, result; LD (HL), 42
	// This uses proper Z80 indirect addressing

	value := state.current().Value
	state.advance()

	// Skip semicolon
	if state.current().Type == TokenSemicolon {
		state.advance()
	}

	return []Token{
		// Load address of variable into HL
		{Type: TokenIdentifier, Value: "LD", Line: state.current().Line},
		{Type: TokenIdentifier, Value: "HL", Line: state.current().Line},
		{Type: TokenComma, Value: ",", Line: state.current().Line},
		{Type: TokenIdentifier, Value: varName, Line: state.current().Line},
		{Type: TokenNewline, Value: "\n", Line: state.current().Line},
		
		// Store value at (HL)
		{Type: TokenIdentifier, Value: "LD", Line: state.current().Line},
		{Type: TokenLParen, Value: "(", Line: state.current().Line},
		{Type: TokenIdentifier, Value: "HL", Line: state.current().Line},
		{Type: TokenRParen, Value: ")", Line: state.current().Line},
		{Type: TokenComma, Value: ",", Line: state.current().Line},
		{Type: TokenNumber, Value: value, Line: state.current().Line},
		{Type: TokenNewline, Value: "\n", Line: state.current().Line},
	}, nil
}

func (c *CStyleConverter) convertMacroCallAssignment(state *ConversionState, varName string) ([]Token, error) {
	// Convert: result = add_two(5); → [parameter setup] + add_two + LD (result), A

	macroCall, err := c.extractMacroCall(state)
	if err != nil {
		return nil, err
	}

	var tokens []Token

	// Generate parameter setup according to calling convention
	convention := c.macroTable.GetCallingConvention()
	for i, arg := range macroCall.Arguments {
		if i < len(convention.ParamRegs) {
			reg := convention.ParamRegs[i]
			
			tokens = append(tokens,
				Token{Type: TokenIdentifier, Value: "LD", Line: state.current().Line},
				Token{Type: TokenIdentifier, Value: reg, Line: state.current().Line},
				Token{Type: TokenComma, Value: ",", Line: state.current().Line},
				Token{Type: TokenNumber, Value: fmt.Sprintf("%d", arg.Value), Line: state.current().Line},
				Token{Type: TokenNewline, Value: "\n", Line: state.current().Line},
			)
		}
	}

	// Generate macro call (traditional style)
	tokens = append(tokens,
		Token{Type: TokenIdentifier, Value: macroCall.Name, Line: state.current().Line},
	)

	// Add arguments in traditional format
	if len(macroCall.Arguments) > 0 {
		tokens = append(tokens, Token{Type: TokenLParen, Value: "(", Line: state.current().Line})
		
		for i, arg := range macroCall.Arguments {
			if i > 0 {
				tokens = append(tokens, Token{Type: TokenComma, Value: ",", Line: state.current().Line})
			}
			tokens = append(tokens, Token{Type: TokenNumber, Value: fmt.Sprintf("%d", arg.Value), Line: state.current().Line})
		}
		
		tokens = append(tokens, Token{Type: TokenRParen, Value: ")", Line: state.current().Line})
	}

	tokens = append(tokens, Token{Type: TokenNewline, Value: "\n", Line: state.current().Line})

	// Store result in variable (assume result comes back in A register)
	if varName != "" {
		tokens = append(tokens,
			// Load address of variable into HL
			Token{Type: TokenIdentifier, Value: "LD", Line: state.current().Line},
			Token{Type: TokenIdentifier, Value: "HL", Line: state.current().Line},
			Token{Type: TokenComma, Value: ",", Line: state.current().Line},
			Token{Type: TokenIdentifier, Value: varName, Line: state.current().Line},
			Token{Type: TokenNewline, Value: "\n", Line: state.current().Line},
			
			// Store A register at (HL)
			Token{Type: TokenIdentifier, Value: "LD", Line: state.current().Line},
			Token{Type: TokenLParen, Value: "(", Line: state.current().Line},
			Token{Type: TokenIdentifier, Value: "HL", Line: state.current().Line},
			Token{Type: TokenRParen, Value: ")", Line: state.current().Line},
			Token{Type: TokenComma, Value: ",", Line: state.current().Line},
			Token{Type: TokenIdentifier, Value: "A", Line: state.current().Line},
			Token{Type: TokenNewline, Value: "\n", Line: state.current().Line},
		)
	}

	return tokens, nil
}

func (c *CStyleConverter) convertFunctionDefinition(state *ConversionState) ([]Token, error) {
	// Convert: uint8_t add_two(uint8_t a) { asm { ADD A, 2; } }
	// To: MACRO add_two(a); ADD A, 2; ENDMACRO

	if c.debug {
		fmt.Printf("[DEBUG] convertFunctionDefinition starting\n")
	}

	var tokens []Token

	// Capture the return type if present. It is the function's return-width
	// contract: a `return <expr>` must produce a value of this width, and a
	// `void` function must not return a value. Default to void when absent.
	returnType := "void"
	if c.isTypeKeyword(state.current().Value) {
		returnType = strings.ToLower(state.current().Value)
		if c.debug {
			fmt.Printf("[DEBUG] Return type: %s\n", returnType)
		}
		state.advance()
	}

	// Get function name
	funcName := state.current().Value
	isMainFunction := strings.ToLower(funcName) == "main"
	if c.debug {
		fmt.Printf("[DEBUG] Function name: %s (isMain: %v)\n", funcName, isMainFunction)
	}
	state.advance()

	// Parse parameters
	if state.current().Type != TokenLParen {
		return nil, fmt.Errorf("expected '(' after function name")
	}
	state.advance() // skip opening paren

	// Each parameter is captured as an optional width marker plus its name, so
	// the marker survives into the generated traditional MACRO header and the
	// width-signature check can see it. An absent marker yields an empty type.
	type cparam struct{ typ, name string }
	var params []cparam
	for state.current().Type != TokenRParen {
		if state.current().Type == TokenEOF {
			return nil, fmt.Errorf("unterminated parameter list")
		}

		// Capture the parameter width marker, if present.
		typ := ""
		if c.isTypeKeyword(state.current().Value) {
			typ = state.current().Value
			state.advance()
		}

		// Get parameter name
		if state.current().Type == TokenIdentifier {
			params = append(params, cparam{typ: typ, name: state.current().Value})
			state.advance()
		}

		// Skip comma
		if state.current().Type == TokenComma {
			state.advance()
		}
	}
	state.advance() // skip closing paren

	if c.debug {
		fmt.Printf("[DEBUG] Function parameters: %v\n", params)
	}

	// Generate MACRO header
	tokens = append(tokens,
		Token{Type: TokenIdentifier, Value: "MACRO", Line: state.current().Line},
		Token{Type: TokenIdentifier, Value: funcName, Line: state.current().Line},
	)

	// Add parameters, preserving any width marker as "type name".
	if len(params) > 0 {
		tokens = append(tokens, Token{Type: TokenLParen, Value: "(", Line: state.current().Line})
		
		for i, param := range params {
			if i > 0 {
				tokens = append(tokens, Token{Type: TokenComma, Value: ",", Line: state.current().Line})
			}
			// "void" is a return/placeholder marker, not a value width; drop it.
			if param.typ != "" && strings.ToLower(param.typ) != "void" {
				tokens = append(tokens, Token{Type: TokenIdentifier, Value: param.typ, Line: state.current().Line})
			}
			tokens = append(tokens, Token{Type: TokenIdentifier, Value: param.name, Line: state.current().Line})
		}
		
		tokens = append(tokens, Token{Type: TokenRParen, Value: ")", Line: state.current().Line})
	}

	tokens = append(tokens, Token{Type: TokenNewline, Value: "\n", Line: state.current().Line})

	// Skip opening brace
	if state.current().Type == TokenLBrace {
		if c.debug {
			fmt.Printf("[DEBUG] Skipping opening brace\n")
		}
		state.advance()
	}

	// CRITICAL FIX: Process function body with proper brace tracking
	if c.debug {
		fmt.Printf("[DEBUG] Processing function body...\n")
	}
	
	bodyTokenCount := 0
	braceDepth := 1 // We already consumed the opening brace
	sawReturn := false
	
	for braceDepth > 0 && state.current().Type != TokenEOF {
		token := state.current()
		if c.debug {
			fmt.Printf("[DEBUG] Body token %d: %s(%q) [depth=%d]\n", bodyTokenCount, token.Type, token.Value, braceDepth)
		}
		bodyTokenCount++
		
		// Track brace depth FIRST
		if token.Type == TokenLBrace {
			braceDepth++
			if c.debug {
				fmt.Printf("[DEBUG] Found opening brace, depth now %d\n", braceDepth)
			}
		} else if token.Type == TokenRBrace {
			braceDepth--
			if c.debug {
				fmt.Printf("[DEBUG] Found closing brace, depth now %d\n", braceDepth)
			}
			if braceDepth == 0 {
				// This is the closing brace for the function
				if c.debug {
					fmt.Printf("[DEBUG] Function body complete\n")
				}
				state.advance() // consume the closing brace
				break
			}
		}
		
		if c.isReturnStatement(state) {
			retTokens, err := c.convertReturn(state, returnType)
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, retTokens...)
			sawReturn = true
		} else if c.isAsmBlock(state) {
			if c.debug {
				fmt.Printf("[DEBUG] Converting asm block\n")
			}
			asmTokens, err := c.convertAsmBlock(state)
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, asmTokens...)
		} else if c.isVariableDeclaration(state) {
			if c.debug {
				fmt.Printf("[DEBUG] Skipping variable declaration\n")
			}
			c.skipVariableDeclaration(state)
		} else if c.isAssignment(state) {
			if c.debug {
				fmt.Printf("[DEBUG] Skipping assignment\n")
			}
			c.skipAssignment(state)
		} else if c.isStandaloneCall(state) {
			if c.debug {
				fmt.Printf("[DEBUG] Converting standalone call: %s\n", token.Value)
			}
			callTokens, err := c.convertStandaloneCall(state)
			if err != nil {
				return nil, err
			}
			if c.debug {
				fmt.Printf("[DEBUG] Standalone call converted to %d tokens\n", len(callTokens))
			}
			tokens = append(tokens, callTokens...)
		} else if state.current().Type == TokenIdentifier && state.peek(1).Type == TokenLParen {
			if c.debug {
				fmt.Printf("[DEBUG] FALLBACK: Converting function call: %s\n", token.Value)
			}
			callTokens, err := c.convertStandaloneCall(state)
			if err != nil {
				return nil, fmt.Errorf("failed to convert function call at line %d: %v", state.current().Line, err)
			}
			tokens = append(tokens, callTokens...)
		} else {
			if c.debug {
				fmt.Printf("[DEBUG] Skipping other token: %s(%q)\n", token.Type, token.Value)
			}
			// Skip whitespace and comments
			if token.Type == TokenComment || token.Type == TokenNewline {
				tokens = append(tokens, token)
			}
			// Skip other tokens but advance
			state.advance()
		}
	}

	if c.debug {
		fmt.Printf("[DEBUG] Function body processing complete, processed %d tokens\n", bodyTokenCount)
	}

	// A function that declares a return width must actually return it. Falling
	// off the end of a typed function leaves the width contract undelivered, the
	// same failure as a bare `return;` in a typed function.
	if _, returnsValue := returnWidthBits(returnType); returnsValue && !sawReturn {
		return nil, fmt.Errorf("function %s declares a %s return but has no return statement",
			funcName, returnType)
	}

	// Add ENDMACRO
	tokens = append(tokens,
		Token{Type: TokenIdentifier, Value: "ENDMACRO", Line: state.current().Line},
		Token{Type: TokenNewline, Value: "\n", Line: state.current().Line},
	)

	// CRITICAL FIX: For main() function, automatically generate a call
	if isMainFunction {
		if c.debug {
			fmt.Printf("[DEBUG] Generating main() call\n")
		}
		tokens = append(tokens,
			Token{Type: TokenNewline, Value: "\n", Line: state.current().Line},
			Token{Type: TokenIdentifier, Value: funcName, Line: state.current().Line},
		)
		
		// Add parameters if any
		if len(params) > 0 {
			tokens = append(tokens, Token{Type: TokenLParen, Value: "(", Line: state.current().Line})
			
			for i := range params {
				if i > 0 {
					tokens = append(tokens, Token{Type: TokenComma, Value: ",", Line: state.current().Line})
				}
				// For main(), parameters typically default to 0
				tokens = append(tokens, Token{Type: TokenNumber, Value: "0", Line: state.current().Line})
			}
			
			tokens = append(tokens, Token{Type: TokenRParen, Value: ")", Line: state.current().Line})
		}
		
		tokens = append(tokens, Token{Type: TokenNewline, Value: "\n", Line: state.current().Line})
	}

	if c.debug {
		fmt.Printf("[DEBUG] convertFunctionDefinition returning %d tokens\n", len(tokens))
	}

	return tokens, nil
}

func (c *CStyleConverter) convertAsmBlock(state *ConversionState) ([]Token, error) {
	// Convert: asm { ADD A, 2; } → ADD A, 2\n

	if c.debug {
		fmt.Printf("[DEBUG] convertAsmBlock starting at token: %s\n", state.current().Value)
	}

	// Skip "asm" keyword
	state.advance()

	// Skip opening brace
	if state.current().Type == TokenLBrace {
		if c.debug {
			fmt.Printf("[DEBUG] Skipping ASM opening brace\n")
		}
		state.advance()
	}

	var tokens []Token
	asmBraceDepth := 1 // We consumed the opening brace

	// Copy tokens until closing brace, converting semicolons to newlines
	for asmBraceDepth > 0 && state.current().Type != TokenEOF {
		token := state.current()
		if c.debug {
			fmt.Printf("[DEBUG] ASM token: %s(%q) [asmDepth=%d]\n", token.Type, token.Value, asmBraceDepth)
		}

		if token.Type == TokenLBrace {
			asmBraceDepth++
			if c.debug {
				fmt.Printf("[DEBUG] ASM opening brace, depth now %d\n", asmBraceDepth)
			}
		} else if token.Type == TokenRBrace {
			asmBraceDepth--
			if c.debug {
				fmt.Printf("[DEBUG] ASM closing brace, depth now %d\n", asmBraceDepth)
			}
			if asmBraceDepth == 0 {
				// This is the closing brace for the asm block
				state.advance() // consume it
				break
			}
		}
		
		if token.Type == TokenSemicolon {
			// Convert semicolon to newline for assembly
			tokens = append(tokens, Token{
				Type:  TokenNewline,
				Value: "\n",
				Line:  token.Line,
			})
		} else if token.Type == TokenComment {
			// Check if comment contains a closing brace that we should handle
			if strings.Contains(token.Value, "}") && asmBraceDepth == 1 {
				// This comment contains the closing brace for the asm block
				if c.debug {
					fmt.Printf("[DEBUG] Found closing brace in comment: %s\n", token.Value)
				}
				asmBraceDepth = 0
				state.advance()
				break
			} else {
				// Regular comment, pass through
				tokens = append(tokens, token)
			}
		} else if asmBraceDepth > 0 {
			// Pass through assembly tokens unchanged (but only if we're still inside the asm block)
			tokens = append(tokens, token)
		}
		
		state.advance()
	}

	if c.debug {
		fmt.Printf("[DEBUG] convertAsmBlock completed, generated %d tokens\n", len(tokens))
	}

	return tokens, nil
}

func (c *CStyleConverter) convertStandaloneCall(state *ConversionState) ([]Token, error) {
	// Convert: add_five(10); → add_five(10) (traditional macro call with parameters)
	// Convert: get_constant(); → get_constant (parameterless call)

	if c.debug {
		fmt.Printf("[DEBUG] convertStandaloneCall starting with: %s\n", state.current().Value)
	}

	// Capture the line number at the start
	startLine := state.current().Line

	// First, check if this looks like a function call
	if state.current().Type != TokenIdentifier {
		return nil, fmt.Errorf("expected function name")
	}
	
	functionName := state.current().Value
	
	// Check if next token is opening parenthesis
	if state.peek(1).Type != TokenLParen {
		return nil, fmt.Errorf("expected '(' after function name")
	}

	// Extract the macro call
	macroCall, err := c.extractMacroCall(state)
	if err != nil {
		return nil, fmt.Errorf("failed to parse function call %s: %v", functionName, err)
	}

	if c.debug {
		fmt.Printf("[DEBUG] Extracted macro call: %s with %d arguments\n", macroCall.Name, len(macroCall.Arguments))
	}

	var tokens []Token

	// Generate traditional macro call with parameters
	tokens = append(tokens,
		Token{Type: TokenIdentifier, Value: macroCall.Name, Line: startLine},
	)

	// Add parameters in traditional format if any exist
	if len(macroCall.Arguments) > 0 {
		tokens = append(tokens, Token{Type: TokenLParen, Value: "(", Line: startLine})
		
		for i, arg := range macroCall.Arguments {
			if i > 0 {
				tokens = append(tokens, Token{Type: TokenComma, Value: ",", Line: startLine})
			}
			
			// Convert argument based on type
			if arg.Type == ExpressionNumber {
				tokens = append(tokens, Token{Type: TokenNumber, Value: fmt.Sprintf("%d", arg.Value), Line: startLine})
			} else if arg.Type == ExpressionSymbol {
				tokens = append(tokens, Token{Type: TokenIdentifier, Value: arg.Symbol, Line: startLine})
			}
		}
		
		tokens = append(tokens, Token{Type: TokenRParen, Value: ")", Line: startLine})
	}

	tokens = append(tokens, Token{Type: TokenNewline, Value: "\n", Line: startLine})

	if c.debug {
		fmt.Printf("[DEBUG] convertStandaloneCall returning %d tokens\n", len(tokens))
		for i, token := range tokens {
			fmt.Printf("[DEBUG] Call token[%d]: %s(%q)\n", i, token.Type, token.Value)
		}
	}

	return tokens, nil
}

// Helper methods

func (c *CStyleConverter) isMacroCallAtPosition(state *ConversionState) bool {
	// Check if current position is a macro call: function_name(args)
	if state.current().Type != TokenIdentifier {
		return false
	}

	macroName := state.current().Value
	if !c.macroTable.IsDefined(macroName) {
		return false
	}

	next := state.peek(1)
	return next.Type == TokenLParen
}

func (c *CStyleConverter) extractMacroCall(state *ConversionState) (*MacroCall, error) {
	// Extract macro call from current position
	if state.current().Type != TokenIdentifier {
		return nil, fmt.Errorf("expected macro name")
	}

	macroName := state.current().Value
	state.advance()

	// Expect opening parenthesis
	if state.current().Type != TokenLParen {
		return nil, fmt.Errorf("expected '(' after macro name")
	}
	state.advance()

	var arguments []*Expression

	// Parse arguments - handle empty parameter list
	if state.current().Type == TokenRParen {
		// Empty parameter list: function()
		state.advance() // skip closing paren
		if state.current().Type == TokenSemicolon {
			state.advance() // skip semicolon
		}
		
		return &MacroCall{
			Name:      macroName,
			Arguments: arguments, // empty
			Style:     MacroStyleC,
		}, nil
	}

	// Parse non-empty arguments
	for state.current().Type != TokenRParen && state.current().Type != TokenEOF {
		// Simple argument parsing (numbers and identifiers only for now)
		if state.current().Type == TokenNumber {
			value, err := ParseNumber(state.current())
			if err != nil {
				return nil, fmt.Errorf("invalid number in macro call: %v", err)
			}
			
			arguments = append(arguments, &Expression{
				Type:  ExpressionNumber,
				Value: value,
			})
			state.advance()
		} else if state.current().Type == TokenIdentifier {
			arguments = append(arguments, &Expression{
				Type:   ExpressionSymbol,
				Symbol: state.current().Value,
			})
			state.advance()
		} else {
			// Skip unexpected tokens to avoid parser errors
			state.advance()
		}

		// Handle comma separator
		if state.current().Type == TokenComma {
			state.advance()
			// After comma, expect another argument
			if state.current().Type == TokenRParen {
				return nil, fmt.Errorf("unexpected ')' after comma in argument list")
			}
		} else if state.current().Type != TokenRParen {
			// If not comma and not closing paren, something's wrong
			return nil, fmt.Errorf("expected ',' or ')' in argument list, got %s", state.current().Type)
		}
	}

	// Expect closing parenthesis
	if state.current().Type != TokenRParen {
		return nil, fmt.Errorf("expected ')' to close argument list")
	}
	state.advance() // skip closing paren

	// Skip semicolon if present
	if state.current().Type == TokenSemicolon {
		state.advance()
	}

	return &MacroCall{
		Name:      macroName,
		Arguments: arguments,
		Style:     MacroStyleC,
	}, nil
}

// GetUniqueLabel generates a unique label for conversion
func (c *CStyleConverter) GetUniqueLabel(base string) string {
	label := fmt.Sprintf("%s_%d", base, c.uniqueID)
	c.uniqueID++
	return label
}

// skipVariableDeclaration skips a variable declaration without processing it
func (c *CStyleConverter) skipVariableDeclaration(state *ConversionState) {
	// Skip: type varName;
	if c.isTypeKeyword(state.current().Value) {
		state.advance() // skip type
	}
	if state.current().Type == TokenIdentifier {
		state.advance() // skip variable name
	}
	if state.current().Type == TokenSemicolon {
		state.advance() // skip semicolon
	}
}

// skipAssignment skips an assignment without processing it
func (c *CStyleConverter) skipAssignment(state *ConversionState) {
	// Skip: varName = value;
	if state.current().Type == TokenIdentifier {
		state.advance() // skip variable name
	}
	if state.current().Type == TokenEquals {
		state.advance() // skip equals
	}
	
	// Skip value (could be number, identifier, or function call)
	for state.current().Type != TokenSemicolon && state.current().Type != TokenEOF && state.current().Type != TokenNewline {
		state.advance()
	}
	
	if state.current().Type == TokenSemicolon {
		state.advance() // skip semicolon
	}
}