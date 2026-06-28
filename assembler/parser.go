package assembler

import (
	"fmt"
	"strings"
)

// Parser converts tokens into an Abstract Syntax Tree
type Parser struct {
	tokens   []Token
	position int
	current  Token
	errors   []string
	// macroNames, when set, lets the parser recognise NAME(args) as a macro call
	// with a comma-separated argument list rather than as an instruction with a
	// single indirect operand. Used when parsing macro bodies so that nested
	// calls (including multi-argument and zero-argument ones) parse correctly.
	macroNames map[string]bool
	// instructionNames, when set, lets the parser tell a mnemonic from a label in
	// pasmo dialect: a column-1 identifier that is not an instruction (nor a
	// directive) is a label even without a trailing colon.
	instructionNames map[string]bool
	// inPasmo tracks the active dialect as the parser walks the token stream; it
	// flips on .PASMO / .ZENAS directive tokens so no-colon labels are accepted
	// only within pasmo regions.
	inPasmo bool
}

// ParsedProgram represents the complete parsed assembly program
type ParsedProgram struct {
	Lines []ParsedLine
}

// ParsedLine represents a single line of assembly
type ParsedLine struct {
	LineNumber  int
	Label       string
	Instruction *Instruction
	Directive   *Directive
	Comment     string
}

// Instruction represents a Z80 instruction
type Instruction struct {
	Mnemonic string
	Operands []*Operand
}

// Directive represents an assembler directive
type Directive struct {
	Name      string
	Arguments []*Expression
	RawArgs   string // raw trailing text, used by .EXPECT (an assertion DSL, not Z80 expressions)
}

// OperandType represents the type of an operand
type OperandType int

const (
	OperandRegister8 OperandType = iota
	OperandRegister16
	OperandImmediate8
	OperandImmediate16
	OperandIndirect
	OperandCondition
	OperandRelative
)

// Operand represents an instruction operand
type Operand struct {
	Type             OperandType
	Register         string
	Expression       *Expression
	Displacement     int
	DisplacementExpr *Expression // IX/IY displacement, evaluated at assembly time
	DisplacementSign int         // +1 or -1; the sign before the displacement
	Condition        string
}

// ResolvedOperand represents an operand with resolved values
type ResolvedOperand struct {
	Type         OperandType
	Register     string
	Value        int
	Displacement int
	Condition    string
}

// ExpressionType represents the type of an expression
type ExpressionType int

const (
	ExpressionNumber ExpressionType = iota
	ExpressionSymbol
	ExpressionBinary
	ExpressionUnary
	ExpressionString  // String literal for character mapping
)

// Expression represents a mathematical expression
type Expression struct {
	Type        ExpressionType
	Value       int
	Symbol      string
	StringValue string       // For string literals
	Operator    string
	Left        *Expression
	Right       *Expression
	RawDigits   string // original digit string of a 0x/0d literal, for multi-byte hexdump chopping in data directives
	RawRadix    int    // 16 or 10 when RawDigits or RadixGroups is set
	RadixGroups []string // spaced 0x/0d hexdump groups (e.g. ["DE","AD"]) for data directives
}

// Register sets for validation
var (
	register8Set = map[string]bool{
		"A": true, "B": true, "C": true, "D": true,
		"E": true, "H": true, "L": true,
		"IXH": true, "IXL": true, "IYH": true, "IYL": true,
		"I": true, "R": true,
	}
	
	register16Set = map[string]bool{
		"BC": true, "DE": true, "HL": true, "SP": true,
		"IX": true, "IY": true, "AF": true,
		"AF'": true,
	}
	
	conditionSet = map[string]bool{
		"NZ": true, "Z": true, "NC": true, "C": true,
		"PO": true, "PE": true, "P": true, "M": true,
	}
)

// NewParser creates a new parser instance
// SetMacroNames tells the parser which identifiers are macros, so that
// NAME(args) is parsed as a macro call with an argument list rather than as an
// instruction with a single indirect operand. Names are matched case-insensitively.
func (p *Parser) SetMacroNames(names map[string]bool) {
	p.macroNames = names
}

// SetInstructionNames gives the parser the set of valid instruction mnemonics,
// used in pasmo dialect to distinguish a column-1 label from an instruction.
func (p *Parser) SetInstructionNames(names map[string]bool) {
	p.instructionNames = names
}

func NewParser() *Parser {
	return &Parser{}
}

// Parse converts tokens into a parsed program
func (p *Parser) Parse(tokens []Token) (*ParsedProgram, error) {
	p.tokens = tokens
	p.position = 0
	p.errors = []string{}
	
	if len(tokens) > 0 {
		p.current = tokens[0]
	}
	
	program := &ParsedProgram{
		Lines: []ParsedLine{},
	}
	
	for p.current.Type != TokenEOF {
		line, err := p.parseLine()
		if err != nil {
			p.errors = append(p.errors, err.Error())
			p.skipToNextLine()
			continue
		}
		
		if line != nil {
			program.Lines = append(program.Lines, *line)
		}
	}
	
	if len(p.errors) > 0 {
		return program, fmt.Errorf("parse errors:\n%s", strings.Join(p.errors, "\n"))
	}
	
	return program, nil
}

// parseLine parses a single line of assembly
func (p *Parser) parseLine() (*ParsedLine, error) {
	lineNumber := p.current.Line
	line := &ParsedLine{LineNumber: lineNumber}
	
	// Skip empty lines
	if p.current.Type == TokenNewline {
		p.advance()
		return nil, nil
	}
	
	// Handle comments at start of line
	if p.current.Type == TokenComment {
		line.Comment = p.current.Value
		p.advance()
		p.skipNewlines()
		return line, nil
	}
	
	// Track dialect as we walk the stream: .PASMO / .ZENAS flip the mode so the
	// no-colon-label rule below applies only inside pasmo regions.
	if p.current.Type == TokenDirective {
		switch strings.ToUpper(p.current.Value) {
		case ".PASMO":
			p.inPasmo = true
		case ".ZENAS":
			p.inPasmo = false
		}
	}
	
	// Check for label
	if p.current.Type == TokenIdentifier && p.peek().Type == TokenColon {
		line.Label = p.current.Value
		p.advance() // Skip identifier
		p.advance() // Skip colon
	} else if p.current.Type == TokenIdentifier && p.peek().Type == TokenDirective {
		// Handle "LABEL .EQU value" syntax
		line.Label = p.current.Value
		p.advance() // Skip label identifier
		// Now current token should be the directive
	} else if p.current.Type == TokenIdentifier && p.peek().Type == TokenIdentifier && p.isDirectiveIdentifier(p.peek().Value) {
		// Handle "LABEL EQU value" syntax (bare, non-dotted directive)
		line.Label = p.current.Value
		p.advance() // Skip label identifier
		// Now current token should be the bare directive identifier
	} else if p.inPasmo && p.isPasmoNoColonLabel() {
		// pasmo dialect: a column-1 identifier that is neither an instruction nor
		// a directive is a label, even without a trailing colon.
		line.Label = p.current.Value
		p.advance() // Skip label identifier
	}
	
	// Check for directive
	if p.current.Type == TokenDirective {
		directive, err := p.parseDirective()
		if err != nil {
			return nil, fmt.Errorf("line %d: %v", lineNumber, err)
		}
		line.Directive = directive
	} else if p.current.Type == TokenIdentifier {
		// Check if this identifier is actually a directive without dot prefix
		if p.isDirectiveIdentifier(p.current.Value) {
			directive, err := p.parseDirectiveFromIdentifier()
			if err != nil {
				return nil, fmt.Errorf("line %d: %v", lineNumber, err)
			}
			line.Directive = directive
		} else {
			// Parse instruction
			instruction, err := p.parseInstruction()
			if err != nil {
				return nil, fmt.Errorf("line %d: %v", lineNumber, err)
			}
			line.Instruction = instruction
		}
	}
	
	// Handle end-of-line comment
	if p.current.Type == TokenComment {
		line.Comment = p.current.Value
		p.advance()
	}
	
	// Expect newline or EOF
	if p.current.Type == TokenNewline {
		p.advance()
	} else if p.current.Type != TokenEOF {
		return nil, fmt.Errorf("line %d: expected newline at end of line, got %s", lineNumber, p.current.Type)
	}
	
	return line, nil
}

// stringStartsExpression reports whether the current TokenString should be parsed
// as an arithmetic term rather than a standalone string literal. This is true
// only for a single-character literal immediately followed by an arithmetic
// operator (e.g. 'A'+1), so ordinary string data like "Hello" is unaffected.
func (p *Parser) stringStartsExpression() bool {
	if p.current.Type != TokenString {
		return false
	}
	if len([]rune(p.current.Value)) != 1 {
		return false // multi-character string is always a string literal
	}
	switch p.peek().Type {
	case TokenPlus, TokenMinus, TokenStar, TokenSlash:
		return true
	}
	return false
}

// isPasmoNoColonLabel reports whether the current token is a pasmo label written
// without a colon. The pasmo rule (verified against the pasmo binary): the first
// identifier on a line is a label unless it is a recognised instruction mnemonic
// or a directive - regardless of indentation. So both `loop  NOP` in column 1
// and an indented `  loop  NOP` treat `loop` as a label. An identifier already
// handled as "LABEL:" or "LABEL EQU" is matched by the earlier branches.
func (p *Parser) isPasmoNoColonLabel() bool {
	if p.current.Type != TokenIdentifier {
		return false
	}
	name := strings.ToUpper(p.current.Value)
	if name == "$" {
		return false
	}
	if p.instructionNames != nil && p.instructionNames[name] {
		return false // a mnemonic is an instruction, not a label
	}
	if p.macroNames != nil && p.macroNames[name] {
		return false // a known macro name is a call, not a label
	}
	if p.isDirectiveIdentifier(p.current.Value) {
		return false // a directive is a directive, not a label
	}
	return true
}

// isRadixMarker reports whether the current token is a bare 0x/0d hexdump marker.
func (p *Parser) isRadixMarker() bool {
	return p.current.Type == TokenIdentifier && (p.current.Value == "0x" || p.current.Value == "0d")
}

// parseRadixMarker consumes a 0x/0d marker and the following group tokens, and
// returns a single expression carrying the raw groups for the data directive to
// emit. After 0x the groups are hex (lexed as identifiers or numbers); after 0d
// they are decimal numbers. Consumption stops at a comma, newline, EOF, or
// comment.
func (p *Parser) parseRadixMarker() (*Expression, error) {
	radix := 16
	if p.current.Value == "0d" {
		radix = 10
	}
	p.advance() // consume the marker

	var groups []string
	for p.current.Type != TokenNewline && p.current.Type != TokenEOF &&
		p.current.Type != TokenComment && p.current.Type != TokenComma {
		if p.current.Type != TokenIdentifier && p.current.Type != TokenNumber {
			return nil, fmt.Errorf("unexpected %q in 0%c hex/dec data", p.current.Value, map[int]byte{16: 'x', 10: 'd'}[radix])
		}
		groups = append(groups, p.current.Value)
		p.advance()
	}
	if len(groups) == 0 {
		return nil, fmt.Errorf("0%c marker has no values", map[int]byte{16: 'x', 10: 'd'}[radix])
	}
	return &Expression{
		Type:       ExpressionNumber,
		RadixGroups: groups,
		RawRadix:    radix,
	}, nil
}

// parseDirective parses an assembler directive
func (p *Parser) parseDirective() (*Directive, error) {
	directiveName := p.current.Value
	directive := &Directive{
		Name:      directiveName,
		Arguments: []*Expression{},
	}
	
	p.advance() // Skip directive name
	
	// Handle .END directive specially (it can have optional argument)
	if directiveName == ".END" {
		if p.current.Type == TokenIdentifier {
			// .END START
			directive.Arguments = append(directive.Arguments, &Expression{
				Type:   ExpressionSymbol,
				Symbol: p.current.Value,
			})
			p.advance()
		}
		return directive, nil
	}
	
	// Handle .EXPECT specially: its arguments are an assertion DSL (A=0x0E,CF=1),
	// not Z80 expressions, so capture the rest of the line verbatim.
	if strings.ToUpper(directiveName) == ".EXPECT" {
		var parts []string
		for p.current.Type != TokenNewline && p.current.Type != TokenEOF && p.current.Type != TokenComment {
			parts = append(parts, p.current.Value)
			p.advance()
		}
		directive.RawArgs = strings.Join(parts, "")
		return directive, nil
	}
	
	// Handle .MATCH specially: "location, data-directive" — capture verbatim so
	// the data part can be re-assembled through the real data path at check time.
	// Preserve spaces between tokens so the spaced radix form (0x DE AD) survives.
	if strings.ToUpper(directiveName) == ".MATCH" {
		var parts []string
		for p.current.Type != TokenNewline && p.current.Type != TokenEOF && p.current.Type != TokenComment {
			parts = append(parts, p.current.Value)
			p.advance()
		}
		directive.RawArgs = strings.Join(parts, " ")
		return directive, nil
	}
	
	// Parse arguments for other directives
	for p.current.Type != TokenNewline && p.current.Type != TokenEOF && p.current.Type != TokenComment {
		if p.isRadixMarker() {
			expr, err := p.parseRadixMarker()
			if err != nil {
				return nil, err
			}
			directive.Arguments = append(directive.Arguments, expr)
		} else if p.current.Type == TokenString && !p.stringStartsExpression() {
			// Handle string literals in directives like .DB "Hello"
			str := p.current.Value
			p.advance()
			// Store as string expression for character mapping during assembly
			directive.Arguments = append(directive.Arguments, &Expression{
				Type:        ExpressionString,
				StringValue: str,
			})
		} else {
			expr, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			directive.Arguments = append(directive.Arguments, expr)
		}
		
		if p.current.Type == TokenComma {
			p.advance()
		} else {
			break
		}
	}
	
	return directive, nil
}

// parseInstruction parses a Z80 instruction
func (p *Parser) parseInstruction() (*Instruction, error) {
	instruction := &Instruction{
		Mnemonic: p.current.Value,
		Operands: []*Operand{},
	}

	// Macro call with a parenthesised argument list: NAME(a, b, ...). Only when
	// NAME is a known macro - otherwise NAME(...) is an ordinary instruction with
	// an indirect operand. This is what lets nested macro calls inside a macro
	// body carry multiple (or zero) arguments; a bare "(a, b)" is not a valid
	// indirect operand.
	if p.macroNames != nil && p.macroNames[strings.ToUpper(p.current.Value)] &&
		p.peek().Type == TokenLParen {
		p.advance() // skip macro name
		p.advance() // skip '('
		for p.current.Type != TokenRParen && p.current.Type != TokenNewline &&
			p.current.Type != TokenEOF {
			expr, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			instruction.Operands = append(instruction.Operands, &Operand{
				Type:       OperandImmediate16,
				Expression: expr,
			})
			if p.current.Type == TokenComma {
				p.advance()
			} else {
				break
			}
		}
		if p.current.Type != TokenRParen {
			return nil, fmt.Errorf("expected ')' to close macro call arguments for %s", instruction.Mnemonic)
		}
		p.advance() // skip ')'
		return instruction, nil
	}

	p.advance() // Skip mnemonic
	
	// Parse operands
	for p.current.Type != TokenNewline && p.current.Type != TokenEOF && p.current.Type != TokenComment {
		operand, err := p.parseOperand()
		if err != nil {
			return nil, err
		}
		instruction.Operands = append(instruction.Operands, operand)
		
		if p.current.Type == TokenComma {
			p.advance()
		} else {
			break
		}
	}
	
	return instruction, nil
}

// parseOperand parses an instruction operand
func (p *Parser) parseOperand() (*Operand, error) {
	switch p.current.Type {
	case TokenIdentifier:
		return p.parseIdentifierOperand()
		
	case TokenLParen:
		return p.parseIndirectOperand()
		
	case TokenNumber, TokenMinus, TokenPlus:
		return p.parseImmediateOperand()
		
	case TokenString:
		// Handle string literals in data contexts
		str := p.current.Value
		p.advance()
		// Convert string to byte values
		return &Operand{
			Type: OperandImmediate8,
			Expression: &Expression{
				Type:  ExpressionNumber,
				Value: int(str[0]), // Just first character for now
			},
		}, nil
		
	default:
		return nil, fmt.Errorf("unexpected token in operand: %s (expected identifier, number, or '(')", p.current.Type)
	}
}

// parseIdentifierOperand parses an identifier-based operand (register, condition, or symbol)
func (p *Parser) parseIdentifierOperand() (*Operand, error) {
	identifier := p.current.Value
	p.advance()

	// Registers and condition codes are matched case-insensitively (LD/ld are the
	// same instruction, A/a the same register), and stored in canonical uppercase
	// so the encoder's register maps resolve. User symbols keep their original
	// case (the assembler is case-sensitive for labels and constants), so the
	// raw identifier is used only on the symbol path below.
	upper := strings.ToUpper(identifier)
	
	// Special case: for certain instructions, some identifiers should be treated as conditions first
	// This handles cases like "JP C, addr" where C should be condition, not register
	if p.isConditionContext() && conditionSet[upper] {
		return &Operand{
			Type:      OperandCondition,
			Condition: upper,
		}, nil
	}
	
	// Check if it's a register
	if register8Set[upper] {
		return &Operand{
			Type:     OperandRegister8,
			Register: upper,
		}, nil
	}
	
	if register16Set[upper] {
		return &Operand{
			Type:     OperandRegister16,
			Register: upper,
		}, nil
	}
	
	// Check if it's a condition (for cases not caught above)
	if conditionSet[upper] {
		return &Operand{
			Type:      OperandCondition,
			Condition: upper,
		}, nil
	}
	
	// Otherwise, it's a symbol reference. If an arithmetic operator follows, the
	// operand is an expression seeded by this symbol (e.g. kernel_end+255,
	// CELLMAP_SIZE-1, vtable+N*3). Otherwise it's a bare symbol. Symbol case is
	// preserved either way.
	left := &Expression{Type: ExpressionSymbol, Symbol: identifier}
	if p.isExpressionOperator(p.current.Type) {
		expr, err := p.parseExpressionFromLeft(left)
		if err != nil {
			return nil, err
		}
		return &Operand{
			Type:       OperandImmediate16,
			Expression: expr,
		}, nil
	}
	return &Operand{
		Type:       OperandImmediate16, // Assume 16-bit, will be resolved later
		Expression: left,
	}, nil
}

// isExpressionOperator reports whether a token begins a binary arithmetic
// continuation of an expression.
func (p *Parser) isExpressionOperator(t TokenType) bool {
	return t == TokenPlus || t == TokenMinus || t == TokenStar || t == TokenSlash
}

// parseExpressionFromLeft continues parsing a binary expression given an already
// parsed left operand. It mirrors the precedence of parseExpression: the seed is
// treated as the first multiplicative term, so a following * or / binds tighter
// than a following + or -. (Seed is a primary, which is the common case for
// symbol-led operands like vtable+N*3.)
func (p *Parser) parseExpressionFromLeft(left *Expression) (*Expression, error) {
	// First, absorb any higher-precedence (* /) operators directly attached to
	// the seed, forming the first additive term.
	term := left
	for p.current.Type == TokenStar || p.current.Type == TokenSlash {
		operator := p.current.Value
		p.advance()
		right, err := p.parsePrimaryExpression()
		if err != nil {
			return nil, err
		}
		term = &Expression{Type: ExpressionBinary, Operator: operator, Left: term, Right: right}
	}
	// Then additive (+ -), with each subsequent term parsed at full mul/div
	// precedence via parseMulDivExpression.
	result := term
	for p.current.Type == TokenPlus || p.current.Type == TokenMinus {
		operator := p.current.Value
		p.advance()
		right, err := p.parseMulDivExpression()
		if err != nil {
			return nil, err
		}
		result = &Expression{Type: ExpressionBinary, Operator: operator, Left: result, Right: right}
	}
	return result, nil
}

// isConditionContext checks if we're parsing in a context where conditions are likely
func (p *Parser) isConditionContext() bool {
	// Look back to see if we're parsing operands for a conditional instruction
	// This is a simple heuristic - in a real parser you'd track more context
	return p.currentInstructionMnemonic() == "JP" || 
		   p.currentInstructionMnemonic() == "JR" || 
		   p.currentInstructionMnemonic() == "CALL" || 
		   p.currentInstructionMnemonic() == "RET"
}

// currentInstructionMnemonic returns the mnemonic of the instruction currently being parsed
func (p *Parser) currentInstructionMnemonic() string {
	// Simple implementation: look back in tokens to find the instruction mnemonic
	// In practice, you'd track this in the parser state
	pos := p.position - 1
	for pos >= 0 && p.tokens[pos].Type != TokenNewline {
		if p.tokens[pos].Type == TokenIdentifier {
			// Check if this looks like an instruction mnemonic
			mnemonic := strings.ToUpper(p.tokens[pos].Value)
			if mnemonic == "JP" || mnemonic == "JR" || mnemonic == "CALL" || mnemonic == "RET" {
				return mnemonic
			}
		}
		pos--
	}
	return ""
}

// parseIndirectOperand parses an indirect addressing operand
func (p *Parser) parseIndirectOperand() (*Operand, error) {
	p.advance() // Skip '('
	
	operand := &Operand{Type: OperandIndirect}
	
	if p.current.Type == TokenIdentifier {
		identifier := p.current.Value
		upper := strings.ToUpper(identifier)
		p.advance()
		
		// Check for displacement (IX+d, IY+d)
		if upper == "IX" || upper == "IY" {
			operand.Register = upper
			
			if p.current.Type == TokenPlus || p.current.Type == TokenMinus {
				sign := 1
				if p.current.Type == TokenMinus {
					sign = -1
				}
				p.advance()
				
				// The displacement is a compile-time constant but may reference
				// symbols (e.g. (IX+OFFSET)), so we keep the expression and
				// evaluate it at assembly time, where symbols are known. The
				// value is range-checked then. Both number and symbol terms with
				// +/-/* are accepted (e.g. (IX+1+3), (IX+N)).
				dispExpr, err := p.parseMulDivExpression()
				if err != nil {
					return nil, fmt.Errorf("invalid displacement: %v", err)
				}
				if p.isExpressionOperator(p.current.Type) {
					dispExpr, err = p.parseExpressionFromLeft(dispExpr)
					if err != nil {
						return nil, fmt.Errorf("invalid displacement: %v", err)
					}
				}
				operand.DisplacementExpr = dispExpr
				operand.DisplacementSign = sign
			}
		} else if register16Set[upper] {
			operand.Register = upper
		} else if upper == "C" {
			// (C) - the Z80 port-C addressing mode used by IN r,(C) / OUT (C),r.
			// Keep it as a register-bearing indirect so the encoder can emit the
			// ED-prefixed form rather than treating C as an undefined symbol.
			operand.Register = "C"
		} else {
			// It's a symbol/label reference - treat as absolute address.
			// Preserve original case (symbols are case-sensitive). If an
			// arithmetic operator follows, parse the full expression (e.g. the
			// self-modifying-code pattern LD (rowcount+1),A).
			left := &Expression{Type: ExpressionSymbol, Symbol: identifier}
			if p.isExpressionOperator(p.current.Type) {
				expr, err := p.parseExpressionFromLeft(left)
				if err != nil {
					return nil, err
				}
				operand.Expression = expr
			} else {
				operand.Expression = left
			}
		}
	} else if p.current.Type == TokenNumber {
		// Absolute address
		expr, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		operand.Expression = expr
	} else {
		return nil, fmt.Errorf("expected register, symbol, or number in indirect operand")
	}
	
	if p.current.Type != TokenRParen {
		return nil, fmt.Errorf("expected ')' to close indirect operand")
	}
	p.advance() // Skip ')'
	
	return operand, nil
}

// parseImmediateOperand parses an immediate value operand
func (p *Parser) parseImmediateOperand() (*Operand, error) {
	expr, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	
	return &Operand{
		Type:       OperandImmediate16, // Will be determined during encoding
		Expression: expr,
	}, nil
}

// parseExpression parses a mathematical expression
func (p *Parser) parseExpression() (*Expression, error) {
	return p.parseAddSubExpression()
}

// isKeyword reports whether the current token is the given bareword operator
// (AND/OR/NOT), which lex as identifiers. Matched case-insensitively.
func (p *Parser) isKeyword(word string) bool {
	return p.current.Type == TokenIdentifier && strings.ToUpper(p.current.Value) == word
}

// parseConditionExpression parses an IF condition with boolean operators.
// Precedence, lowest to highest: OR, AND, NOT, then arithmetic/comparison
// operands, then parenthesised sub-conditions. AND/OR/NOT are scoped to IF.
func (p *Parser) parseConditionExpression() (*Expression, error) {
	return p.parseOrExpression()
}

func (p *Parser) parseOrExpression() (*Expression, error) {
	left, err := p.parseAndExpression()
	if err != nil {
		return nil, err
	}
	for p.isKeyword("OR") {
		p.advance()
		right, err := p.parseAndExpression()
		if err != nil {
			return nil, err
		}
		left = &Expression{Type: ExpressionBinary, Operator: "OR", Left: left, Right: right}
	}
	return left, nil
}

func (p *Parser) parseAndExpression() (*Expression, error) {
	left, err := p.parseNotExpression()
	if err != nil {
		return nil, err
	}
	for p.isKeyword("AND") {
		p.advance()
		right, err := p.parseNotExpression()
		if err != nil {
			return nil, err
		}
		left = &Expression{Type: ExpressionBinary, Operator: "AND", Left: left, Right: right}
	}
	return left, nil
}

func (p *Parser) parseNotExpression() (*Expression, error) {
	if p.isKeyword("NOT") {
		p.advance()
		operand, err := p.parseNotExpression()
		if err != nil {
			return nil, err
		}
		return &Expression{Type: ExpressionUnary, Operator: "NOT", Left: operand}, nil
	}
	return p.parseConditionPrimary()
}

func (p *Parser) parseConditionPrimary() (*Expression, error) {
	// Parenthesised sub-condition.
	if p.current.Type == TokenLParen {
		p.advance()
		inner, err := p.parseOrExpression()
		if err != nil {
			return nil, err
		}
		if p.current.Type != TokenRParen {
			return nil, fmt.Errorf("expected ')' to close condition")
		}
		p.advance()
		return inner, nil
	}
	// Otherwise an arithmetic/value operand (a symbol, number, or arithmetic
	// expression). A bare value is true when non-zero.
	return p.parseExpression()
}

// parseAddSubExpression parses addition and subtraction
func (p *Parser) parseAddSubExpression() (*Expression, error) {
	left, err := p.parseMulDivExpression()
	if err != nil {
		return nil, err
	}
	
	for p.current.Type == TokenPlus || p.current.Type == TokenMinus {
		operator := p.current.Value
		p.advance()
		
		right, err := p.parseMulDivExpression()
		if err != nil {
			return nil, err
		}
		
		left = &Expression{
			Type:     ExpressionBinary,
			Operator: operator,
			Left:     left,
			Right:    right,
		}
	}
	
	return left, nil
}

// parseMulDivExpression parses multiplication and division
func (p *Parser) parseMulDivExpression() (*Expression, error) {
	left, err := p.parsePrimaryExpression()
	if err != nil {
		return nil, err
	}
	
	for p.current.Type == TokenStar || p.current.Type == TokenSlash {
		operator := p.current.Value
		p.advance()
		
		right, err := p.parsePrimaryExpression()
		if err != nil {
			return nil, err
		}
		
		left = &Expression{
			Type:     ExpressionBinary,
			Operator: operator,
			Left:     left,
			Right:    right,
		}
	}
	
	return left, nil
}

// parsePrimaryExpression parses primary expressions (numbers, symbols, parenthesized expressions)
func (p *Parser) parsePrimaryExpression() (*Expression, error) {
	switch p.current.Type {
	case TokenNumber:
		tv := p.current.Value
		// A 0x literal with more than two hex digits may be a hexdump (e.g.
		// .DB 0xDEADBEEF). Capture the raw digits first: such literals can exceed
		// the integer range, so we must not fail on ParseNumber overflow before a
		// data directive has a chance to chop them into bytes/words.
		isLongHex := len(tv) > 4 && (tv[:2] == "0x" || tv[:2] == "0X")
		value, err := ParseNumber(p.current)
		if err != nil && !isLongHex {
			return nil, err
		}
		expr := &Expression{
			Type:  ExpressionNumber,
			Value: int(value),
		}
		if len(tv) > 2 && (tv[:2] == "0x" || tv[:2] == "0X") {
			expr.RawDigits = tv[2:]
			expr.RawRadix = 16
		}
		p.advance()
		return expr, nil

	case TokenString:
		// A single-character literal ('A') is usable as a numeric term: its value
		// is the character code. Multi-character strings are not valid arithmetic
		// operands. This matches pasmo/standard assembler behaviour and works in
		// both native and pasmo dialects.
		runes := []rune(p.current.Value)
		if len(runes) != 1 {
			return nil, fmt.Errorf("only a single-character literal may be used in an expression, got %q", p.current.Value)
		}
		ch := int(runes[0])
		p.advance()
		return &Expression{
			Type:  ExpressionNumber,
			Value: ch,
		}, nil
		
	case TokenIdentifier:
		symbol := p.current.Value
		p.advance()
		return &Expression{
			Type:   ExpressionSymbol,
			Symbol: symbol,
		}, nil
		
	case TokenMinus:
		p.advance()
		expr, err := p.parsePrimaryExpression()
		if err != nil {
			return nil, err
		}
		return &Expression{
			Type:     ExpressionUnary,
			Operator: "-",
			Left:     expr,
		}, nil
		
	case TokenPlus:
		p.advance()
		return p.parsePrimaryExpression()
		
	case TokenLParen:
		p.advance() // Skip '('
		expr, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		if p.current.Type != TokenRParen {
			return nil, fmt.Errorf("expected ')' after expression")
		}
		p.advance() // Skip ')'
		return expr, nil
		
	default:
		return nil, fmt.Errorf("unexpected token in expression: %s", p.current.Type)
	}
}

// Helper methods

func (p *Parser) advance() {
	p.position++
	if p.position < len(p.tokens) {
		p.current = p.tokens[p.position]
	} else {
		p.current = Token{Type: TokenEOF}
	}
}

func (p *Parser) peek() Token {
	if p.position+1 < len(p.tokens) {
		return p.tokens[p.position+1]
	}
	return Token{Type: TokenEOF}
}

func (p *Parser) skipNewlines() {
	for p.current.Type == TokenNewline {
		p.advance()
	}
}

func (p *Parser) skipToNextLine() {
	for p.current.Type != TokenNewline && p.current.Type != TokenEOF {
		p.advance()
	}
	if p.current.Type == TokenNewline {
		p.advance()
	}
}

// String methods for debugging

func (o *Operand) String() string {
	switch o.Type {
	case OperandRegister8, OperandRegister16:
		return o.Register
	case OperandCondition:
		return o.Condition
	case OperandIndirect:
		if o.Register != "" {
			if o.Displacement != 0 {
				return fmt.Sprintf("(%s%+d)", o.Register, o.Displacement)
			}
			return fmt.Sprintf("(%s)", o.Register)
		}
		return fmt.Sprintf("(%s)", o.Expression.String())
	case OperandImmediate8, OperandImmediate16:
		return o.Expression.String()
	default:
		return "<?>"
	}
}

func (e *Expression) String() string {
	switch e.Type {
	case ExpressionNumber:
		return fmt.Sprintf("%d", e.Value)
	case ExpressionSymbol:
		return e.Symbol
	case ExpressionString:
		return fmt.Sprintf("\"%s\"", e.StringValue)
	case ExpressionBinary:
		return fmt.Sprintf("(%s %s %s)", e.Left.String(), e.Operator, e.Right.String())
	case ExpressionUnary:
		return fmt.Sprintf("%s%s", e.Operator, e.Left.String())
	default:
		return "<?>"
	}
}

// isDirectiveIdentifier checks if an identifier is actually a directive without dot prefix
func (p *Parser) isDirectiveIdentifier(name string) bool {
	directiveNames := map[string]bool{
		"DEFB": true, "DB": true, "DEFM": true,
		"DEFW": true, "DW": true,
		"EQU": true,
		"ORG": true,
		"END": true,
		"DS": true, "DEFS": true,
		"INCBIN": true,
		"IF": true, "IFDEF": true, "IFNDEF": true,
		"ELSE": true, "ENDIF": true,
	}
	return directiveNames[strings.ToUpper(name)]
}

// parseDirectiveFromIdentifier parses a directive from a bare identifier (no dot prefix)
func (p *Parser) parseDirectiveFromIdentifier() (*Directive, error) {
	directiveName := strings.ToUpper(p.current.Value)
	directive := &Directive{
		Name:      directiveName,
		Arguments: []*Expression{},
	}
	
	p.advance() // Skip directive name
	
	// Handle END directive specially (it can have optional argument)
	if directiveName == "END" {
		if p.current.Type == TokenIdentifier {
			// END START
			directive.Arguments = append(directive.Arguments, &Expression{
				Type:   ExpressionSymbol,
				Symbol: p.current.Value,
			})
			p.advance()
		}
		return directive, nil
	}

	// IF takes a single boolean condition that may use AND/OR/NOT and
	// parentheses. These operators are scoped to IF; they are not part of
	// ordinary operand or DB expressions.
	if directiveName == "IF" {
		expr, err := p.parseConditionExpression()
		if err != nil {
			return nil, err
		}
		directive.Arguments = append(directive.Arguments, expr)
		return directive, nil
	}

	// Parse arguments for other directives
	for p.current.Type != TokenNewline && p.current.Type != TokenEOF && p.current.Type != TokenComment {
		if p.isRadixMarker() {
			expr, err := p.parseRadixMarker()
			if err != nil {
				return nil, err
			}
			directive.Arguments = append(directive.Arguments, expr)
		} else if p.current.Type == TokenString && !p.stringStartsExpression() {
			// Handle string literals in directives like DEFB "Hello"
			str := p.current.Value
			p.advance()
			// Store as string expression for character mapping during assembly
			directive.Arguments = append(directive.Arguments, &Expression{
				Type:        ExpressionString,
				StringValue: str,
			})
		} else {
			expr, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			directive.Arguments = append(directive.Arguments, expr)
		}
		
		if p.current.Type == TokenComma {
			p.advance()
		} else {
			break
		}
	}
	
	return directive, nil
}