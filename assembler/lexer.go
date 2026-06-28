package assembler

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// TokenType represents the different types of tokens in Z80 assembly
type TokenType int

const (
	// End of input
	TokenEOF TokenType = iota
	
	// Literals
	TokenIdentifier  // Labels, mnemonics, registers
	TokenNumber      // Numeric literals
	TokenString      // String literals
	
	// Operators and punctuation
	TokenComma       // ,
	TokenColon       // :
	TokenPlus        // +
	TokenMinus       // -
	TokenStar        // *
	TokenSlash       // /
	TokenLParen      // (
	TokenRParen      // )
	TokenLBracket    // [
	TokenRBracket    // ]
	
	// Macro-specific tokens (ADDED)
	TokenLBrace      // {
	TokenRBrace      // }
	TokenSemicolon   // ;
	TokenArrow       // ->
	TokenEquals      // =
	
	// Special
	TokenNewline     // \n
	TokenComment     // ; comment
	TokenDirective   // .directive
	
	// Error token
	TokenError
)

// Token represents a lexical token
type Token struct {
	Type     TokenType
	Value    string
	Line     int
	Column   int
	Position int
}

// Lexer tokenizes Z80 assembly source code
// Dialect selects a source-compatibility dialect for the lexer. It is distinct
// from cStyleMode (a macro-style concern): a dialect changes how base syntax
// like the location counter and labels is read, so zenas can ingest another
// assembler's source within a scoped region. DialectNative is zenas's own
// syntax; further dialects (e.g. a future sjasmplus mode) are added as values
// here and switched by their directives, mirroring how .pasmo works.
type Dialect int

const (
	DialectNative Dialect = iota
	DialectPasmo
)

type Lexer struct {
	input    []rune
	position int
	line     int
	column   int
	errors   []string
	// cStyleMode makes ';' a statement terminator (TokenSemicolon) rather than
	// the start of a comment, but only inside a brace-delimited block (depth > 0);
	// at file level ';' remains a comment so header comments keep working. Set when
	// assembling .MACRO_STYLE C source. '//' is the comment form in C-style code.
	cStyleMode bool
	braceDepth int
	// dialect is the active source-compatibility dialect; .pasmo / .zenas
	// directives switch it as the stream is lexed. Reset to native per tokenise.
	dialect Dialect
}

// SetDialect sets the active source dialect (used by tests and by the assembler
// before tokenising when the source declares a dialect at the top).
func (l *Lexer) SetDialect(d Dialect) {
	l.dialect = d
}

// SetCStyleMode controls whether ';' is a statement terminator (C-style) or a
// comment start (traditional Z80).
func (l *Lexer) SetCStyleMode(enabled bool) {
	l.cStyleMode = enabled
}

// NewLexer creates a new lexer instance
func NewLexer() *Lexer {
	return &Lexer{}
}

// Tokenize converts source code into tokens
func (l *Lexer) Tokenize(source string) ([]Token, error) {
	l.input = []rune(source)
	l.position = 0
	l.line = 1
	l.column = 1
	l.errors = []string{}
	l.braceDepth = 0
	l.dialect = DialectNative
	
	var tokens []Token
	
	for l.position < len(l.input) {
		token := l.nextToken()
		if token.Type == TokenError {
			l.errors = append(l.errors, fmt.Sprintf("line %d:%d: %s", token.Line, token.Column, token.Value))
			continue
		}
		if token.Type != TokenEOF {
			tokens = append(tokens, token)
		}
		if token.Type == TokenEOF {
			break
		}
	}
	
	if len(l.errors) > 0 {
		return nil, fmt.Errorf("lexical errors:\n%s", strings.Join(l.errors, "\n"))
	}
	
	// Add final EOF token
	tokens = append(tokens, Token{
		Type:     TokenEOF,
		Line:     l.line,
		Column:   l.column,
		Position: l.position,
	})
	
	return tokens, nil
}

// nextToken scans and returns the next token
func (l *Lexer) nextToken() Token {
	l.skipWhitespace()
	
	if l.position >= len(l.input) {
		return Token{Type: TokenEOF, Line: l.line, Column: l.column, Position: l.position}
	}
	
	ch := l.current()
	startLine := l.line
	startColumn := l.column
	startPosition := l.position
	
	switch ch {
	case '\n':
		l.advance()
		return Token{
			Type:     TokenNewline,
			Value:    "\n",
			Line:     startLine,
			Column:   startColumn,
			Position: startPosition,
		}
		
	case ',':
		l.advance()
		return Token{
			Type:     TokenComma,
			Value:    ",",
			Line:     startLine,
			Column:   startColumn,
			Position: startPosition,
		}
		
	case ':':
		l.advance()
		return Token{
			Type:     TokenColon,
			Value:    ":",
			Line:     startLine,
			Column:   startColumn,
			Position: startPosition,
		}
		
	case '+':
		l.advance()
		return Token{
			Type:     TokenPlus,
			Value:    "+",
			Line:     startLine,
			Column:   startColumn,
			Position: startPosition,
		}
		
	case '-':
		l.advance()
		return Token{
			Type:     TokenMinus,
			Value:    "-",
			Line:     startLine,
			Column:   startColumn,
			Position: startPosition,
		}
		
	case '*':
		l.advance()
		return Token{
			Type:     TokenStar,
			Value:    "*",
			Line:     startLine,
			Column:   startColumn,
			Position: startPosition,
		}
		
	case '/':
		// Check for C-style comment //
		if l.peek() == '/' {
			return l.scanCStyleComment(startLine, startColumn, startPosition)
		} else {
			l.advance()
			return Token{
				Type:     TokenSlash,
				Value:    "/",
				Line:     startLine,
				Column:   startColumn,
				Position: startPosition,
			}
		}
		
	case '(':
		l.advance()
		return Token{
			Type:     TokenLParen,
			Value:    "(",
			Line:     startLine,
			Column:   startColumn,
			Position: startPosition,
		}
		
	case ')':
		l.advance()
		return Token{
			Type:     TokenRParen,
			Value:    ")",
			Line:     startLine,
			Column:   startColumn,
			Position: startPosition,
		}
		
	case '[':
		l.advance()
		return Token{
			Type:     TokenLBracket,
			Value:    "[",
			Line:     startLine,
			Column:   startColumn,
			Position: startPosition,
		}
		
	case ']':
		l.advance()
		return Token{
			Type:     TokenRBracket,
			Value:    "]",
			Line:     startLine,
			Column:   startColumn,
			Position: startPosition,
		}
		
	case '{':  // NEW for macros
		l.advance()
		l.braceDepth++
		return Token{
			Type:     TokenLBrace,
			Value:    "{",
			Line:     startLine,
			Column:   startColumn,
			Position: startPosition,
		}
		
	case '}':  // NEW for macros
		l.advance()
		if l.braceDepth > 0 {
			l.braceDepth--
		}
		return Token{
			Type:     TokenRBrace,
			Value:    "}",
			Line:     startLine,
			Column:   startColumn,
			Position: startPosition,
		}
		
	case '=':  // NEW for C-style assignments
		l.advance()
		return Token{
			Type:     TokenEquals,
			Value:    "=",
			Line:     startLine,
			Column:   startColumn,
			Position: startPosition,
		}
		
	case ';':
		// In traditional Z80 assembly a semicolon begins a comment to end of
		// line. In C-style mode it is a statement terminator instead, so that
		// inline asm{} statements and the surrounding braces tokenise correctly
		// even on a single physical line. (C-style comments use '//'.)
		if l.cStyleMode && l.braceDepth > 0 {
			l.advance()
			return Token{
				Type:     TokenSemicolon,
				Value:    ";",
				Line:     startLine,
				Column:   startColumn,
				Position: startPosition,
			}
		}
		return l.scanComment(startLine, startColumn, startPosition)
		
	case '.':
		return l.scanDirective(startLine, startColumn, startPosition)
		
	case '"', '\'':
		return l.scanString(ch, startLine, startColumn, startPosition)
		
	case '0':
		// Check for 0x (hex) or 0b (binary) prefixes
		if l.peek() == 'x' || l.peek() == 'X' {
			// A bare "0x" not followed by a hex digit is a radix marker (the
			// spaced hexdump form, e.g. .DB 0x DE AD). Emit it as an identifier
			// token "0x" for the data-directive parser to interpret.
			if !isHexDigit(l.peekAt(2)) {
				l.advance() // '0'
				l.advance() // 'x'
				return Token{Type: TokenIdentifier, Value: "0x", Line: startLine, Column: startColumn, Position: startPosition}
			}
			return l.scanHexNumber0x(startLine, startColumn, startPosition)
		} else if (l.peek() == 'd' || l.peek() == 'D') && !isDecimalDigitRune(l.peekAt(2)) {
			// "0d" radix marker (decimal hexdump form, e.g. .DB 0d 222 173).
			l.advance() // '0'
			l.advance() // 'd'
			return Token{Type: TokenIdentifier, Value: "0d", Line: startLine, Column: startColumn, Position: startPosition}
		} else if l.peek() == 'b' || l.peek() == 'B' {
			return l.scanBinaryNumber0b(startLine, startColumn, startPosition)
		} else {
			// Regular number starting with 0
			return l.scanNumber(startLine, startColumn, startPosition)
		}
		
	case '$':
		// In pasmo dialect, a '$' that is not immediately followed by a hex
		// digit is the location counter (current address). A '$' followed by a
		// hex digit is still a pasmo hex literal ($1234). In native dialect, '$'
		// is always the hex prefix.
		if l.dialect == DialectPasmo && !isHexDigit(l.peek()) {
			l.advance() // consume '$'
			return Token{
				Type:     TokenIdentifier,
				Value:    "$",
				Line:     startLine,
				Column:   startColumn,
				Position: startPosition,
			}
		}
		return l.scanHexNumber(startLine, startColumn, startPosition)
		
	case '&':
		return l.scanHexNumberAmp(startLine, startColumn, startPosition)

	case '#':
		// In pasmo dialect, '#' is a hexadecimal prefix (#80 == 0x80), like '&'.
		// In native dialect '#' is not a number prefix and is an unexpected
		// character, consistent with the default handling below.
		if l.dialect == DialectPasmo {
			return l.scanHexNumberHash(startLine, startColumn, startPosition)
		}
		l.advance()
		return Token{
			Type:     TokenError,
			Value:    fmt.Sprintf("unexpected character: %c", ch),
			Line:     startLine,
			Column:   startColumn,
			Position: startPosition,
		}
		
	case '%':
		return l.scanBinaryNumber(startLine, startColumn, startPosition)
		
	default:
		if unicode.IsDigit(ch) {
			return l.scanNumber(startLine, startColumn, startPosition)
		} else if unicode.IsLetter(ch) || ch == '_' {
			return l.scanIdentifier(startLine, startColumn, startPosition)
		} else {
			l.advance()
			return Token{
				Type:     TokenError,
				Value:    fmt.Sprintf("unexpected character: %c", ch),
				Line:     startLine,
				Column:   startColumn,
				Position: startPosition,
			}
		}
	}
}

// Helper methods

// isHexDigit reports whether r is a hexadecimal digit.
func isHexDigit(r rune) bool {
	return (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
}

func (l *Lexer) current() rune {
	if l.position >= len(l.input) {
		return 0
	}
	return l.input[l.position]
}

func (l *Lexer) peek() rune {
	if l.position+1 >= len(l.input) {
		return 0
	}
	return l.input[l.position+1]
}

// peekAt returns the rune n positions ahead of the current one (peekAt(1) == peek()).
func (l *Lexer) peekAt(n int) rune {
	if l.position+n >= len(l.input) {
		return 0
	}
	return l.input[l.position+n]
}

func isDecimalDigitRune(r rune) bool {
	return r >= '0' && r <= '9'
}

func (l *Lexer) advance() {
	if l.position < len(l.input) {
		if l.input[l.position] == '\n' {
			l.line++
			l.column = 1
		} else {
			l.column++
		}
		l.position++
	}
}

func (l *Lexer) skipWhitespace() {
	for l.position < len(l.input) {
		ch := l.current()
		if ch == ' ' || ch == '\t' || ch == '\r' {
			l.advance()
		} else {
			break
		}
	}
}

func (l *Lexer) scanComment(startLine, startColumn, startPosition int) Token {
	value := ""
	for l.position < len(l.input) && l.current() != '\n' {
		value += string(l.current())
		l.advance()
	}
	
	return Token{
		Type:     TokenComment,
		Value:    value,
		Line:     startLine,
		Column:   startColumn,
		Position: startPosition,
	}
}

func (l *Lexer) scanCStyleComment(startLine, startColumn, startPosition int) Token {
	value := "//"
	l.advance() // Skip first '/'
	l.advance() // Skip second '/'
	
	// Read the rest of the line
	for l.position < len(l.input) && l.current() != '\n' {
		value += string(l.current())
		l.advance()
	}
	
	return Token{
		Type:     TokenComment,
		Value:    value,
		Line:     startLine,
		Column:   startColumn,
		Position: startPosition,
	}
}

func (l *Lexer) scanDirective(startLine, startColumn, startPosition int) Token {
	value := ""
	l.advance() // Skip '.'
	
	for l.position < len(l.input) {
		ch := l.current()
		if unicode.IsLetter(ch) || unicode.IsDigit(ch) || ch == '_' {
			value += string(ch)
			l.advance()
		} else {
			break
		}
	}
	
	directiveName := "." + strings.ToUpper(value)

	// Dialect switches take effect immediately as they are lexed, so subsequent
	// tokens - including the contents of a textually-included file spliced in
	// after the directive - are read in the selected dialect.
	switch directiveName {
	case ".PASMO":
		l.dialect = DialectPasmo
	case ".ZENAS":
		l.dialect = DialectNative
	}

	return Token{
		Type:     TokenDirective,
		Value:    directiveName,
		Line:     startLine,
		Column:   startColumn,
		Position: startPosition,
	}
}

func (l *Lexer) scanString(quote rune, startLine, startColumn, startPosition int) Token {
	value := ""
	l.advance() // Skip opening quote
	
	for l.position < len(l.input) {
		ch := l.current()
		if ch == quote {
			l.advance() // Skip closing quote
			break
		} else if ch == '\\' && l.peek() != 0 {
			l.advance() // Skip backslash
			escaped := l.current()
			l.advance()
			switch escaped {
			case 'n':
				value += "\n"
			case 't':
				value += "\t"
			case 'r':
				value += "\r"
			case '\\':
				value += "\\"
			case '"':
				value += "\""
			case '\'':
				value += "'"
			default:
				value += string(escaped)
			}
		} else if ch == '\n' {
			return Token{
				Type:     TokenError,
				Value:    "unterminated string literal",
				Line:     startLine,
				Column:   startColumn,
				Position: startPosition,
			}
		} else {
			value += string(ch)
			l.advance()
		}
	}
	
	return Token{
		Type:     TokenString,
		Value:    value,
		Line:     startLine,
		Column:   startColumn,
		Position: startPosition,
	}
}

func (l *Lexer) scanNumber(startLine, startColumn, startPosition int) Token {
	value := ""
	
	// Scan digits and letters for hex/binary suffixes
	for l.position < len(l.input) {
		ch := l.current()
		if unicode.IsDigit(ch) || (ch >= 'A' && ch <= 'F') || (ch >= 'a' && ch <= 'f') {
			value += string(ch)
			l.advance()
		} else {
			break
		}
	}
	
	// Check for hex suffix (like 1234H) or binary suffix (like 101010B)
	if l.position < len(l.input) {
		ch := l.current()
		if ch == 'H' || ch == 'h' || ch == 'B' || ch == 'b' {
			value += string(ch)
			l.advance()
		}
	}
	
	return Token{
		Type:     TokenNumber,
		Value:    value,
		Line:     startLine,
		Column:   startColumn,
		Position: startPosition,
	}
}

func (l *Lexer) scanHexNumber0x(startLine, startColumn, startPosition int) Token {
	value := "0x"
	l.advance() // Skip '0'
	l.advance() // Skip 'x' or 'X'
	
	digitCount := 0
	for l.position < len(l.input) {
		ch := l.current()
		if (ch >= '0' && ch <= '9') || (ch >= 'A' && ch <= 'F') || (ch >= 'a' && ch <= 'f') {
			value += string(ch)
			l.advance()
			digitCount++
		} else if ch == '_' {
			// Skip underscores for readability (0x0fff_0fff)
			l.advance()
		} else {
			break
		}
	}
	
	if digitCount == 0 {
		return Token{
			Type:     TokenError,
			Value:    "invalid hex number: missing digits after 0x",
			Line:     startLine,
			Column:   startColumn,
			Position: startPosition,
		}
	}
	
	return Token{
		Type:     TokenNumber,
		Value:    value,
		Line:     startLine,
		Column:   startColumn,
		Position: startPosition,
	}
}

func (l *Lexer) scanBinaryNumber0b(startLine, startColumn, startPosition int) Token {
	value := "0b"
	l.advance() // Skip '0'
	l.advance() // Skip 'b' or 'B'
	
	digitCount := 0
	for l.position < len(l.input) {
		ch := l.current()
		if ch == '0' || ch == '1' {
			value += string(ch)
			l.advance()
			digitCount++
		} else if ch == '_' {
			// Skip underscores for readability (0b1111_0000)
			l.advance()
		} else {
			break
		}
	}
	
	if digitCount == 0 {
		return Token{
			Type:     TokenError,
			Value:    "invalid binary number: missing digits after 0b",
			Line:     startLine,
			Column:   startColumn,
			Position: startPosition,
		}
	}
	
	return Token{
		Type:     TokenNumber,
		Value:    value,
		Line:     startLine,
		Column:   startColumn,
		Position: startPosition,
	}
}

func (l *Lexer) scanHexNumber(startLine, startColumn, startPosition int) Token {
	value := "$"
	l.advance() // Skip '$'
	
	digitCount := 0
	for l.position < len(l.input) {
		ch := l.current()
		if (ch >= '0' && ch <= '9') || (ch >= 'A' && ch <= 'F') || (ch >= 'a' && ch <= 'f') {
			value += string(ch)
			l.advance()
			digitCount++
		} else {
			break
		}
	}
	
	if digitCount == 0 {
		return Token{
			Type:     TokenError,
			Value:    "invalid hex number: missing digits after $",
			Line:     startLine,
			Column:   startColumn,
			Position: startPosition,
		}
	}
	
	return Token{
		Type:     TokenNumber,
		Value:    value,
		Line:     startLine,
		Column:   startColumn,
		Position: startPosition,
	}
}

func (l *Lexer) scanBinaryNumber(startLine, startColumn, startPosition int) Token {
	value := "%"
	l.advance() // Skip '%'
	
	digitCount := 0
	for l.position < len(l.input) {
		ch := l.current()
		if ch == '0' || ch == '1' {
			value += string(ch)
			l.advance()
			digitCount++
		} else {
			break
		}
	}
	
	if digitCount == 0 {
		return Token{
			Type:     TokenError,
			Value:    "invalid binary number: missing digits after %",
			Line:     startLine,
			Column:   startColumn,
			Position: startPosition,
		}
	}
	
	return Token{
		Type:     TokenNumber,
		Value:    value,
		Line:     startLine,
		Column:   startColumn,
		Position: startPosition,
	}
}

func (l *Lexer) scanHexNumberAmp(startLine, startColumn, startPosition int) Token {
	value := "&"
	l.advance() // Skip '&'
	
	digitCount := 0
	for l.position < len(l.input) {
		ch := l.current()
		if (ch >= '0' && ch <= '9') || (ch >= 'A' && ch <= 'F') || (ch >= 'a' && ch <= 'f') {
			value += string(ch)
			l.advance()
			digitCount++
		} else {
			break
		}
	}
	
	if digitCount == 0 {
		return Token{
			Type:     TokenError,
			Value:    "invalid hex number: missing digits after &",
			Line:     startLine,
			Column:   startColumn,
			Position: startPosition,
		}
	}
	
	return Token{
		Type:     TokenNumber,
		Value:    value,
		Line:     startLine,
		Column:   startColumn,
		Position: startPosition,
	}
}

func (l *Lexer) scanHexNumberHash(startLine, startColumn, startPosition int) Token {
	value := "#"
	l.advance() // Skip '#'

	digitCount := 0
	for l.position < len(l.input) {
		ch := l.current()
		if (ch >= '0' && ch <= '9') || (ch >= 'A' && ch <= 'F') || (ch >= 'a' && ch <= 'f') {
			value += string(ch)
			l.advance()
			digitCount++
		} else {
			break
		}
	}

	if digitCount == 0 {
		return Token{
			Type:     TokenError,
			Value:    "invalid hex number: missing digits after #",
			Line:     startLine,
			Column:   startColumn,
			Position: startPosition,
		}
	}

	return Token{
		Type:     TokenNumber,
		Value:    value,
		Line:     startLine,
		Column:   startColumn,
		Position: startPosition,
	}
}

func (l *Lexer) scanIdentifier(startLine, startColumn, startPosition int) Token {
	value := ""
	
	for l.position < len(l.input) {
		ch := l.current()
		if unicode.IsLetter(ch) || unicode.IsDigit(ch) || ch == '_' || ch == '.' ||
			(l.dialect == DialectPasmo && ch == '@') {
			// pasmo permits '@' within identifiers; native dialect does not.
			// (Note: pasmo's '?' is the ternary conditional operator, not an
			// identifier character, so it is deliberately not accepted here.)
			value += string(ch)
			l.advance()
		} else {
			break
		}
	}

	// Allow a trailing apostrophe as part of a prime (shadow) register name,
	// e.g. the AF' in "EX AF,AF'". This is narrow: only when the identifier
	// scanned so far is a register that has a prime form, so ordinary char
	// literals like 'A' (which begin with the apostrophe) are unaffected.
	if l.position < len(l.input) && l.current() == '\'' {
		switch strings.ToUpper(value) {
		case "AF", "BC", "DE", "HL":
			value += "'"
			l.advance()
		}
	}
	
	return Token{
		Type:     TokenIdentifier,
		Value:    value,
		Line:     startLine,
		Column:   startColumn,
		Position: startPosition,
	}
}

// ParseNumber converts a number token to an integer value
func ParseNumber(token Token) (int, error) {
	if token.Type != TokenNumber {
		return 0, fmt.Errorf("not a number token")
	}
	
	value := token.Value
	
	if strings.HasPrefix(value, "$") {
		// Hexadecimal
		val, err := strconv.ParseInt(value[1:], 16, 32)
		return int(val), err
	} else if strings.HasPrefix(value, "&") {
		// Hexadecimal with & prefix  
		val, err := strconv.ParseInt(value[1:], 16, 32)
		return int(val), err
	} else if strings.HasPrefix(value, "#") {
		// Hexadecimal with # prefix (pasmo dialect)
		val, err := strconv.ParseInt(value[1:], 16, 32)
		return int(val), err
	} else if strings.HasPrefix(value, "%") {
		// Binary
		val, err := strconv.ParseInt(value[1:], 2, 32)
		return int(val), err
	} else if strings.HasPrefix(strings.ToLower(value), "0x") {
		// Hexadecimal with 0x prefix (remove underscores first)
		cleanValue := strings.ReplaceAll(value[2:], "_", "")
		val, err := strconv.ParseInt(cleanValue, 16, 32)
		return int(val), err
	} else if strings.HasPrefix(strings.ToLower(value), "0b") {
		// Binary with 0b prefix (remove underscores first)
		cleanValue := strings.ReplaceAll(value[2:], "_", "")
		val, err := strconv.ParseInt(cleanValue, 2, 32)
		return int(val), err
	} else if strings.HasSuffix(strings.ToUpper(value), "H") {
		// Hexadecimal with H suffix
		val, err := strconv.ParseInt(value[:len(value)-1], 16, 32)
		return int(val), err
	} else if strings.HasSuffix(strings.ToUpper(value), "B") {
		// Binary with B suffix
		val, err := strconv.ParseInt(value[:len(value)-1], 2, 32)
		return int(val), err
	} else {
		// Decimal
		val, err := strconv.ParseInt(value, 10, 32)
		return int(val), err
	}
}

// TokenTypeString returns a string representation of a token type
func (t TokenType) String() string {
	switch t {
	case TokenEOF:
		return "EOF"
	case TokenIdentifier:
		return "IDENTIFIER"
	case TokenNumber:
		return "NUMBER"
	case TokenString:
		return "STRING"
	case TokenComma:
		return "COMMA"
	case TokenColon:
		return "COLON"
	case TokenPlus:
		return "PLUS"
	case TokenMinus:
		return "MINUS"
	case TokenStar:
		return "STAR"
	case TokenSlash:
		return "SLASH"
	case TokenLParen:
		return "LPAREN"
	case TokenRParen:
		return "RPAREN"
	case TokenLBracket:
		return "LBRACKET"
	case TokenRBracket:
		return "RBRACKET"
	case TokenLBrace:
		return "LBRACE"
	case TokenRBrace:
		return "RBRACE"
	case TokenSemicolon:
		return "SEMICOLON"
	case TokenEquals:
		return "EQUALS"
	case TokenArrow:
		return "ARROW"
	case TokenNewline:
		return "NEWLINE"
	case TokenComment:
		return "COMMENT"
	case TokenDirective:
		return "DIRECTIVE"
	case TokenError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// String returns a string representation of a token
func (t Token) String() string {
	return fmt.Sprintf("%s(%q) at %d:%d", t.Type, t.Value, t.Line, t.Column)
}

// ExtendedLexer wraps the existing lexer with macro-specific functionality
type ExtendedLexer struct {
	*Lexer
	macroMode bool
}

// NewExtendedLexer creates a new extended lexer for macro support
func NewExtendedLexer() *ExtendedLexer {
	return &ExtendedLexer{
		Lexer:     NewLexer(),
		macroMode: false,
	}
}

// TokenizeWithMacroSupport tokenizes source code with macro token support
func (el *ExtendedLexer) TokenizeWithMacroSupport(source string) ([]Token, error) {
	// Use the base lexer tokenization
	tokens, err := el.Lexer.Tokenize(source)
	if err != nil {
		return nil, err
	}
	
	// Post-process tokens to add macro-specific token types
	return el.postProcessMacroTokens(tokens), nil
}

// postProcessMacroTokens converts generic tokens to macro-specific ones where appropriate
func (el *ExtendedLexer) postProcessMacroTokens(tokens []Token) []Token {
	// The base lexer already handles braces and semicolons correctly
	// This function can be used for additional macro-specific processing if needed
	return tokens
}

// SetMacroMode enables or disables macro-specific token recognition
func (el *ExtendedLexer) SetMacroMode(enabled bool) {
	el.macroMode = enabled
}

// isMacroKeyword checks if a string is a macro-related keyword
func (el *ExtendedLexer) isMacroKeyword(value string) bool {
	keywords := map[string]bool{
		// Traditional macro keywords
		"MACRO":    true,
		"ENDMACRO": true,
		
		// C-style keywords
		"void":        true,
		"uint8_t":     true,
		"uint16_t":    true,
		"uint8":       true,
		"uint16":      true,
		"byte":        true,
		"word":        true,
		"register8_t": true,
		"register16_t": true,
		"reg8":        true,
		"reg16":       true,
		"address_t":   true,
		"addr":        true,
		"return":      true,
	}
	
	return keywords[strings.ToUpper(value)]
}

// ValidateBraceMatching validates that braces are properly matched in macro definitions
func (el *ExtendedLexer) ValidateBraceMatching(tokens []Token) error {
	var braceStack []Token
	
	for _, token := range tokens {
		switch token.Type {
		case TokenLBrace:
			braceStack = append(braceStack, token)
			
		case TokenRBrace:
			if len(braceStack) == 0 {
				return fmt.Errorf("unmatched closing brace at line %d:%d", token.Line, token.Column)
			}
			// Pop from stack
			braceStack = braceStack[:len(braceStack)-1]
		}
	}
	
	if len(braceStack) > 0 {
		unmatched := braceStack[0]
		return fmt.Errorf("unmatched opening brace at line %d:%d", unmatched.Line, unmatched.Column)
	}
	
	return nil
}
