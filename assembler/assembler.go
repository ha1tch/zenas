// Package assembler provides a Z80 assembler built on patterns extracted from the zen80 emulator
package assembler

import (
	"fmt"
	"strconv"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Character set mappings for international support
var characterSets = map[string]map[rune]uint8{
	// ASCII (default)
	"ascii": {},

	// ZX Spectrum variants
	"spectrum-uk": {
		'£': 96,  // Pound sign
		'©': 127, // Copyright symbol
	},

	"spectrum-tk90x": {
		'á': 144, 'à': 145, 'â': 146, 'ã': 147, // Portuguese vowels
		'ç': 148,           // Cedilla
		'é': 149, 'ê': 150, // E variants
		'í': 151,                     // I accent
		'ó': 152, 'ô': 153, 'õ': 154, // O variants
		'ú': 155, 'ü': 156, // U variants
		'ñ': 157, // Spanish N (for loan words)
	},

	"spectrum-inves": {
		'ñ': 144, 'Ñ': 145, // Spanish N
		'á': 146, 'é': 147, 'í': 148, 'ó': 149, 'ú': 150, // Spanish vowels
		'ü': 151,           // German U (loan words)
		'¿': 152, '¡': 153, // Spanish punctuation
		'ç': 154, // Catalan cedilla
	},

	"spectrum-czech": {
		'á': 144, 'č': 145, 'ď': 146, 'é': 147,
		'ě': 148, 'í': 149, 'ľ': 150, 'ĺ': 151,
		'ň': 152, 'ó': 153, 'ô': 154, 'ř': 155,
		'š': 156, 'ť': 157, 'ú': 158, 'ů': 159,
		'ý': 160, 'ž': 161,
	},

	"spectrum-polish": {
		'ą': 144, 'ć': 145, 'ę': 146, 'ł': 147,
		'ń': 148, 'ó': 149, 'ś': 150, 'ź': 151,
		'ż': 152,
	},

	// MSX variants
	"msx-jp": {
		// Half-width Katakana (essential characters)
		'ア': 161, 'イ': 162, 'ウ': 163, 'エ': 164, 'オ': 165,
		'カ': 166, 'キ': 167, 'ク': 168, 'ケ': 169, 'コ': 170,
		'サ': 171, 'シ': 172, 'ス': 173, 'セ': 174, 'ソ': 175,
		'タ': 176, 'チ': 177, 'ツ': 178, 'テ': 179, 'ト': 180,
		'ナ': 181, 'ニ': 182, 'ヌ': 183, 'ネ': 184, 'ノ': 185,
		'ハ': 186, 'ヒ': 187, 'フ': 188, 'ヘ': 189, 'ホ': 190,
		'マ': 191, 'ミ': 192, 'ム': 193, 'メ': 194, 'モ': 195,
		'ヤ': 196, 'ユ': 197, 'ヨ': 198,
		'ラ': 199, 'リ': 200, 'ル': 201, 'レ': 202, 'ロ': 203,
		'ワ': 204, 'ヲ': 205, 'ン': 206,
		'。': 207, '「': 208, '」': 209, '･': 210,
	},

	"msx-eu": {
		// Western European characters
		'á': 160, 'à': 161, 'â': 162, 'ä': 163,
		'é': 164, 'è': 165, 'ê': 166, 'ë': 167,
		'í': 168, 'ì': 169, 'î': 170, 'ï': 171,
		'ó': 172, 'ò': 173, 'ô': 174, 'ö': 175,
		'ú': 176, 'ù': 177, 'û': 178, 'ü': 179,
		'ç': 180, 'ñ': 181, '¿': 182, '¡': 183,
		'£': 184, '¥': 185,
	},

	// Amstrad CPC variants
	"cpc-uk": {
		'£': 163, // Different position than Spectrum
		'©': 169, // Copyright
		'®': 174, // Registered trademark
		'°': 176, // Degree symbol
		'±': 177, // Plus-minus
	},

	"cpc-fr": {
		'à': 133, 'ç': 135, 'é': 130, 'è': 138,
		'ê': 136, 'ë': 137, 'î': 140, 'ï': 139,
		'ô': 147, 'ù': 151, 'û': 150, 'ü': 129,
		'â': 131, 'ä': 132, 'ö': 148,
	},
}

// Assembler represents the main Z80 assembler with macro support
type Assembler struct {
	lexer    *Lexer
	parser   *Parser
	encoder  *Encoder
	symbols  *SymbolTable
	output   []uint8
	address  uint16
	pass     int
	charset  string            // Character set for string encoding
	warnings []AssemblyWarning // Character replacement warnings
	baseDir  string            // Directory for resolving relative INCLUDE paths

	// Macro-specific components (ADDED)
	macroTable    *MacroTable
	macroParser   *MacroParserManager
	macroExpander *MacroExpander
	extendedLexer *ExtendedLexer

	// Macro settings (ADDED)
	macroStyle        MacroStyle
	callingConvention CallingConvention
	currentPackage    string // affiliation set by .PACKAGE for subsequent macro defs
	inPasmo           bool   // active pasmo dialect, tracked across the parse loop
	origin            uint16 // address of the first ORG (load address for `run`)
	originSet         bool   // whether a first ORG has been seen this assembly
	inTestFile        bool   // top-level input is a *_test.asm file; gates .EXPECT
	testSpecs         []TestSpec // .EXPECT directives collected this assembly
	lastLabel         string // most recent label seen, for binding .EXPECT

	// Expansion tracking (ADDED)
	expansionLevel    int
	maxExpansionLevel int

	// Conditional assembly (IF/ELSE/ENDIF). condStack holds one frame per open
	// IF block; a line is emitted only when every frame is active. Reset at the
	// start of each pass.
	condStack []condFrame
}

// condFrame tracks one open conditional block.
//
//	active   - emit lines in the current branch?
//	taken    - has any branch in this IF/ELSE chain already been taken? (so a
//	           later ELSE does not re-activate)
//	parentOn - was the enclosing context active when this IF was opened? (an IF
//	           nested inside a skipped block stays off regardless of its own cond)
type condFrame struct {
	active   bool
	taken    bool
	parentOn bool
}

// AssemblyResult contains the results of assembly
// TestSpec records one .EXPECT or .MATCH directive bound to the nearest preceding
// label. Expect is the raw .EXPECT assertion text (empty if this is a match).
// Match, when set, is a raw ".MATCH location, data" assertion. Only collected
// when assembling a *_test.asm file; the `assert` command runs these.
type TestSpec struct {
	Label  string
	Expect string
	Match  string
}

type AssemblyResult struct {
	MachineCode []uint8
	Symbols     map[string]uint16
	Origin      uint16
	Tests       []TestSpec
	Listing     string
	Errors      []AssemblyError
	Warnings    []AssemblyWarning
}

// AssemblyError represents an error during assembly
type AssemblyError struct {
	Line    int
	Column  int
	Message string
	Type    ErrorType
}

// AssemblyWarning represents a warning during assembly
type AssemblyWarning struct {
	Line    int
	Column  int
	Message string
	Type    WarningType
}

type ErrorType int
type WarningType int

const (
	SyntaxError ErrorType = iota
	UndefinedSymbol
	InvalidOperand
	OutOfRange
)

const (
	CharacterReplacement WarningType = iota
	CharsetCompatibility
)

// New creates a new Z80 assembler instance with macro support (UPDATED)
func New() *Assembler {
	lexer := NewLexer()
	parser := NewParser()
	encoder := NewEncoder()
	symbols := NewSymbolTable()
	macroTable := NewMacroTable()

	assembler := &Assembler{
		lexer:    lexer,
		parser:   parser,
		encoder:  encoder,
		symbols:  symbols,
		output:   []uint8{},
		address:  0,
		pass:     1,
		charset:  "ascii",
		warnings: []AssemblyWarning{},

		// Macro-specific components (NEW)
		macroTable:        macroTable,
		macroParser:       NewMacroParserManager(),
		macroExpander:     nil, // Will be initialized below
		extendedLexer:     NewExtendedLexer(),
		macroStyle:        MacroStyleTraditional,
		callingConvention: RegisterFastConvention,
		expansionLevel:    0,
		maxExpansionLevel: 10,
	}

	// Initialize macro expander (NEW)
	assembler.macroExpander = NewMacroExpander(macroTable, symbols)

	return assembler
}

// SetCharset sets the character set for string encoding
// EnableZ80N turns on the Z80N (ZX Spectrum Next) extended instruction set.
// Off by default; enabled via the --next / --cpu=Z80N command-line flag.
func (a *Assembler) EnableZ80N() {
	a.encoder.EnableZ80N()
}

// SetTestFile marks the top-level input as a *_test.asm file, which permits the
// .EXPECT directive. When false (the default), .EXPECT is an assembly error, so
// test expectations cannot appear in production builds.
func (a *Assembler) SetTestFile(isTest bool) {
	a.inTestFile = isTest
}

func (a *Assembler) SetCharset(charset string) error {
	if _, exists := characterSets[charset]; !exists {
		return fmt.Errorf("unsupported character set: %s", charset)
	}
	a.charset = charset
	return nil
}

// GetSupportedCharsets returns a list of supported character sets
func (a *Assembler) GetSupportedCharsets() []string {
	charsets := make([]string, 0, len(characterSets))
	for name := range characterSets {
		charsets = append(charsets, name)
	}
	return charsets
}

// SetMacroStyle sets the macro notation style (NEW)
func (a *Assembler) SetMacroStyle(style MacroStyle) error {
	err := a.macroParser.SetStyle(style)
	if err != nil {
		return err
	}

	a.macroStyle = style
	a.macroTable.SetStyle(style)

	return nil
}

// SetCallingConvention sets the parameter passing convention (NEW)
func (a *Assembler) SetCallingConvention(convention CallingConvention) {
	a.callingConvention = convention
	a.macroTable.SetCallingConvention(convention)
}

// AssembleString assembles Z80 assembly source code from a string (UPDATED for macros)
// SetBaseDir sets the directory against which relative INCLUDE paths are
// resolved. Typically the directory of the top-level source file.
func (a *Assembler) SetBaseDir(dir string) {
	a.baseDir = dir
}

// Define pre-seeds a symbol before assembly. Used for command-line --define
// values so conditional assembly (IF/IFDEF) can be driven from the build, the
// way Go build tags select variants. Defines persist across both passes.
func (a *Assembler) Define(name string, value uint16) {
	a.symbols.Define(name, value)
}

func (a *Assembler) AssembleString(source string) (*AssemblyResult, error) {
	base := a.baseDir
	if base == "" {
		base = "."
	}
	expanded, err := expandIncludes(source, base, nil, 0)
	if err != nil {
		return nil, err
	}
	return a.assemble(strings.NewReader(expanded))
}

// AssembleFile assembles Z80 assembly source code from a file
func (a *Assembler) AssembleFile(filename string) (*AssemblyResult, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read input file: %v", err)
	}
	a.SetBaseDir(filepath.Dir(filename))
	return a.AssembleString(string(data))
}

// assemble performs the actual assembly process (UPDATED for macros)
func (a *Assembler) assemble(source io.Reader) (*AssemblyResult, error) {
	result := &AssemblyResult{
		MachineCode: []uint8{},
		Symbols:     make(map[string]uint16),
		Errors:      []AssemblyError{},
		Warnings:    []AssemblyWarning{},
	}

	// Read source
	sourceBytes, err := io.ReadAll(source)
	if err != nil {
		return nil, err
	}
	sourceText := string(sourceBytes)

	a.origin = 0
	a.originSet = false
	a.testSpecs = nil

	// Two-pass assembly with macro support
	for pass := 1; pass <= 2; pass++ {
		a.pass = pass
		a.address = 0
		a.currentPackage = "" // reset affiliation; .PACKAGE directives re-set it each pass
		a.inPasmo = false     // dialect resets per pass; .pasmo/.zenas re-set it
		// Give the line parser the current instruction set (including Z80N if
		// enabled), so pasmo no-colon-label detection can tell a label from a
		// mnemonic.
		a.parser.SetInstructionNames(a.instructionNameSet())
		a.condStack = a.condStack[:0]
		// Reset the macro expander's unique-label counter so each pass generates
		// identical unique label names. Otherwise pass 2 would mint different
		// names than pass 1 (e.g. LP_1_3 vs LP_1_1) and references would resolve
		// against symbols that only pass 1 defined.
		a.macroExpander.Reset()
		if pass == 2 {
			a.output = []uint8{}
			a.warnings = []AssemblyWarning{}
		}

		err := a.processPass(sourceText, result)
		if err != nil {
			return result, err
		}

		if len(result.Errors) > 0 && pass == 1 {
			// Don't continue to pass 2 if there are errors
			return result, fmt.Errorf("assembly failed with %d errors", len(result.Errors))
		}
	}

	result.MachineCode = a.output
	result.Origin = a.origin
	result.Tests = a.testSpecs
	result.Symbols = a.symbols.GetAll()
	result.Warnings = a.warnings

	return result, nil
}

// processPass handles a single assembly pass (UPDATED for macros)
func (a *Assembler) processPass(source string, result *AssemblyResult) error {
	// Check if we should use macro-aware parsing
	if a.hasMacroDirectives(source) {
		return a.processPassWithMacros(source, result)
	}

	// Original logic for files without macros
	tokens, err := a.lexer.Tokenize(source)
	if err != nil {
		return err
	}

	program, err := a.parser.Parse(tokens)
	if err != nil {
		return err
	}

	for _, line := range program.Lines {
		err := a.processLine(&line, result)
		if err != nil {
			result.Errors = append(result.Errors, AssemblyError{
				Line:    line.LineNumber,
				Message: err.Error(),
				Type:    SyntaxError,
			})
		}
	}

	if len(a.condStack) > 0 {
		result.Errors = append(result.Errors, AssemblyError{
			Message: fmt.Sprintf("%d unterminated IF block(s): missing ENDIF", len(a.condStack)),
			Type:    SyntaxError,
		})
	}

	return nil
}

// preprocessMacroDirectives extracts and processes macro style directives before conversion (NEW)
func (a *Assembler) preprocessMacroDirectives(tokens []Token) error {
	pos := 0
	
	for pos < len(tokens) {
		// Skip whitespace and comments
		pos = SkipWhitespaceAndComments(tokens, pos)
		if pos >= len(tokens) {
			break
		}

		// Check for macro style directive
		if a.isDirective(tokens, pos, ".MACRO_STYLE") {
			_, err := a.processMacroStyleDirective(tokens, pos)
			if err != nil {
				return err
			}
			// Don't advance pos - let normal parsing handle it later
			break // Only process the first .MACRO_STYLE directive
		}

		// Check for calling convention directive
		if a.isDirective(tokens, pos, ".CALLING_CONVENTION") {
			_, err := a.processCallingConventionDirective(tokens, pos)
			if err != nil {
				return err
			}
			break // Only process the first .CALLING_CONVENTION directive
		}

		pos++
	}
	
	return nil
}

// hasMacroDirectives checks if source contains macro-related directives (NEW)
func (a *Assembler) hasMacroDirectives(source string) bool {
	return strings.Contains(source, ".MACRO_STYLE") ||
		strings.Contains(source, "MACRO ") ||
		strings.Contains(source, "ENDMACRO") ||
		strings.Contains(source, "uint8_t") ||
		strings.Contains(source, "void ")
}

// cStyleSelected reports whether the source selects the C macro style, by
// looking for a `.MACRO_STYLE C` directive. Used to put the lexer into C-style
// mode (semicolon as terminator) before tokenising.
func cStyleSelected(source string) bool {
	for _, line := range strings.Split(source, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && strings.EqualFold(fields[0], ".MACRO_STYLE") &&
			strings.EqualFold(fields[1], "C") {
			return true
		}
	}
	return false
}

// processPassWithMacros handles assembly pass with macro support (ENHANCED WITH CONVERTER)
func (a *Assembler) processPassWithMacros(source string, result *AssemblyResult) error {
	// In C-style source, ';' is a statement terminator rather than a comment
	// start. Detect the style directive before tokenising so the lexer treats
	// semicolons correctly even when a whole function is on one line.
	if cStyleSelected(source) {
		a.extendedLexer.SetCStyleMode(true)
	} else {
		a.extendedLexer.SetCStyleMode(false)
	}

	// Tokenize with macro support
	tokens, err := a.extendedLexer.TokenizeWithMacroSupport(source)
	if err != nil {
		return err
	}

	// PRE-PROCESS: Extract macro style directives BEFORE conversion
	err = a.preprocessMacroDirectives(tokens)
	if err != nil {
		return fmt.Errorf("macro directive preprocessing failed: %v", err)
	}

	// CRITICAL FIX: Convert C-style tokens to traditional assembly tokens
	// Now that macro style is properly set
	if a.macroStyle == MacroStyleC {
		converter := NewCStyleConverter(a.macroTable, a.symbols)
		tokens, err = converter.ConvertToTraditional(tokens)
		if err != nil {
			return fmt.Errorf("C-style conversion failed: %v", err)
		}
	}

	// Parse with macro awareness (now using converted tokens)
	program, err := a.parseWithMacroSupport(tokens)
	if err != nil {
		return err
	}

	// Process each line with macro expansion
	for _, line := range program.Lines {
		err := a.processLine(&line, result)
		if err != nil {
			result.Errors = append(result.Errors, AssemblyError{
				Line:    line.LineNumber,
				Message: err.Error(),
				Type:    SyntaxError,
			})
		}
	}

	if len(a.condStack) > 0 {
		result.Errors = append(result.Errors, AssemblyError{
			Message: fmt.Sprintf("%d unterminated IF block(s): missing ENDIF", len(a.condStack)),
			Type:    SyntaxError,
		})
	}

	return nil
}

// parseWithMacroSupport parses tokens with macro definition and call recognition (FIXED)
func (a *Assembler) parseWithMacroSupport(tokens []Token) (*ParsedProgram, error) {
	program := &ParsedProgram{Lines: []ParsedLine{}}
	pos := 0

	for pos < len(tokens) {
		// Skip whitespace and comments
		pos = SkipWhitespaceAndComments(tokens, pos)
		if pos >= len(tokens) {
			break
		}

		// Check for macro style directive
		if a.isDirective(tokens, pos, ".MACRO_STYLE") {
			newPos, err := a.processMacroStyleDirective(tokens, pos)
			if err != nil {
				return nil, err
			}
			pos = newPos
			continue
		}

		// Check for calling convention directive
		if a.isDirective(tokens, pos, ".CALLING_CONVENTION") {
			newPos, err := a.processCallingConventionDirective(tokens, pos)
			if err != nil {
				return nil, err
			}
			pos = newPos
			continue
		}

		// Check for package affiliation directive: .PACKAGE name
		if a.isDirective(tokens, pos, ".PACKAGE") {
			newPos, err := a.processPackageDirective(tokens, pos)
			if err != nil {
				return nil, err
			}
			pos = newPos
			continue
		}

		// CRITICAL FIX: Check for traditional MACRO definitions (from converter or original)
		if pos < len(tokens) && tokens[pos].Type == TokenIdentifier && 
		   strings.ToUpper(tokens[pos].Value) == "MACRO" {
			// Directly use traditional macro parser for converted tokens
			traditionalParser := NewTraditionalMacroParser()
			traditionalParser.SetKnownMacros(a.macroNameSet())
			
			macro, newPos, err := traditionalParser.ParseMacroDefinition(tokens, pos)
			if err != nil {
				return nil, fmt.Errorf("error parsing converted macro definition at line %d: %v", 
					tokens[pos].Line, err)
			}

			// Define macro in table ONLY on pass 1 to prevent duplicates
			if a.pass == 1 {
				macro.Package = a.currentPackage
				err = a.macroTable.Define(macro)
				if err != nil {
					return nil, fmt.Errorf("error defining macro '%s' at line %d: %v", 
						macro.Name, tokens[pos].Line, err)
				}
			}

			pos = newPos
			continue
		}

		// Check for macro definition using current style (original detection)
		if a.macroParser.CanParseMacroDefinition(tokens, pos) {
			a.macroParser.SetKnownMacros(a.macroNameSet())
			macro, newPos, err := a.macroParser.ParseMacroDefinition(tokens, pos)
			if err != nil {
				return nil, fmt.Errorf("error parsing macro definition at line %d: %v", 
					tokens[pos].Line, err)
			}

			// Define macro in table ONLY on pass 1 to prevent duplicates
			if a.pass == 1 {
				macro.Package = a.currentPackage
				err = a.macroTable.Define(macro)
				if err != nil {
					return nil, fmt.Errorf("error defining macro '%s' at line %d: %v", 
						macro.Name, tokens[pos].Line, err)
				}
			}

			pos = newPos
			continue
		}

		// ENHANCED: Better C-style macro call detection
		if a.macroStyle == MacroStyleC && a.couldBeCStyleMacroCall(tokens, pos) {
			// For C-style, check if this identifier is a defined macro
			if pos < len(tokens) && tokens[pos].Type == TokenIdentifier {
				macroName := tokens[pos].Value
				if a.macroTable.IsDefined(macroName) {
					// This is a macro call disguised as a C-style function call
					call, newPos, err := a.macroParser.ParseMacroCall(tokens, pos)
					if err != nil {
						return nil, fmt.Errorf("error parsing macro call at line %d: %v", 
							tokens[pos].Line, err)
					}
					
					// Expand the macro call immediately
					expandedLines, err := a.macroExpander.ExpandMacro(call)
					if err != nil {
						return nil, fmt.Errorf("error expanding macro '%s' at line %d: %v", 
							call.Name, tokens[pos].Line, err)
					}
					
					// Add expanded lines to program
					program.Lines = append(program.Lines, expandedLines...)
					pos = newPos
					continue
				}
			}
		}

		// Traditional macro call detection: an identifier that names a defined
		// macro, optionally followed by a parenthesised argument list. Routing the
		// call through the token-level macro parser (rather than parsing it as an
		// instruction operand) is what lets multi-argument and zero-argument calls
		// work - (a, b) and () are not valid indirect operands.
		if a.macroStyle == MacroStyleTraditional &&
			pos < len(tokens) && tokens[pos].Type == TokenIdentifier &&
			a.shouldInterceptAsMacro(tokens[pos].Value) {
			call, newPos, err := a.macroParser.ParseMacroCall(tokens, pos)
			if err != nil {
				return nil, fmt.Errorf("error parsing macro call '%s' at line %d: %v",
					tokens[pos].Value, tokens[pos].Line, err)
			}
			expandedLines, err := a.macroExpander.ExpandMacro(call)
			if err != nil {
				return nil, fmt.Errorf("error expanding macro '%s' at line %d: %v",
					call.Name, tokens[pos].Line, err)
			}
			program.Lines = append(program.Lines, expandedLines...)
			pos = newPos
			continue
		}

		// Parse regular line
		line, newPos, err := a.parseRegularLine(tokens, pos)
		if err != nil {
			return nil, err
		}

		if line != nil {
			program.Lines = append(program.Lines, *line)
		}

		pos = newPos
	}

	return program, nil
}

// couldBeCStyleMacroCall checks if tokens could represent a C-style macro call (NEW)
func (a *Assembler) couldBeCStyleMacroCall(tokens []Token, pos int) bool {
	// Look for pattern: identifier ( ... ) ;
	if pos >= len(tokens) || tokens[pos].Type != TokenIdentifier {
		return false
	}
	
	// Check for parentheses (function call syntax)
	if pos+1 < len(tokens) && tokens[pos+1].Type == TokenLParen {
		return true
	}
	
	// Check for assignment syntax: identifier = macro_call(...)
	if pos+2 < len(tokens) && 
	   tokens[pos+1].Type == TokenIdentifier && tokens[pos+1].Value == "=" &&
	   tokens[pos+2].Type == TokenIdentifier {
		// Look ahead for parentheses after the macro name
		if pos+3 < len(tokens) && tokens[pos+3].Type == TokenLParen {
			return true
		}
	}
	
	return false
}

// processLine processes a single line of the program (UPDATED for macros)
func (a *Assembler) processLine(line *ParsedLine, result *AssemblyResult) error {
	// Conditional assembly: IF/IFDEF/IFNDEF/ELSE/ENDIF directives update the
	// condition stack and are handled here regardless of the current active
	// state (so the stack stays balanced even inside skipped blocks). All other
	// lines are processed only when every open conditional frame is active.
	if name, ok := conditionalDirectiveName(line); ok {
		return a.processConditional(name, line)
	}
	if !a.condActive() {
		return nil // inside an inactive conditional branch - skip
	}

	// Check if this line contains a macro call
	if line.Instruction != nil {
		macroCall, err := a.detectMacroCall(line.Instruction)
		if err != nil {
			return err
		}
		if macroCall != nil {
			return a.expandAndProcessMacroCall(macroCall, result)
		}
	}

	// Process as regular line (original assembler logic)
	return a.processLineOriginal(line, result)
}

// condActive reports whether the current line should be emitted (all open
// conditional frames active, or no open frames).
func (a *Assembler) condActive() bool {
	for i := range a.condStack {
		if !a.condStack[i].active {
			return false
		}
	}
	return true
}

// conditionalDirectiveName returns the uppercased conditional directive name on
// a line (IF/IFDEF/IFNDEF/ELSE/ENDIF), if any.
func conditionalDirectiveName(line *ParsedLine) (string, bool) {
	if line.Directive == nil {
		return "", false
	}
	switch strings.ToUpper(line.Directive.Name) {
	case "IF", "IFDEF", "IFNDEF", "ELSE", "ENDIF":
		return strings.ToUpper(line.Directive.Name), true
	}
	return "", false
}

// processConditional updates the condition stack for a conditional directive.
func (a *Assembler) processConditional(name string, line *ParsedLine) error {
	switch name {
	case "IF", "IFDEF", "IFNDEF":
		parentOn := a.condActive()
		cond := false
		if parentOn {
			// Only evaluate the condition when the enclosing context is active.
			var err error
			cond, err = a.evalCondition(name, line.Directive)
			if err != nil {
				return err
			}
		}
		a.condStack = append(a.condStack, condFrame{
			active:   parentOn && cond,
			taken:    parentOn && cond,
			parentOn: parentOn,
		})
		return nil

	case "ELSE":
		if len(a.condStack) == 0 {
			return fmt.Errorf("ELSE without matching IF")
		}
		top := &a.condStack[len(a.condStack)-1]
		// Activate the ELSE branch only if the parent was on and no prior branch
		// in this chain was taken.
		top.active = top.parentOn && !top.taken
		if top.active {
			top.taken = true
		}
		return nil

	case "ENDIF":
		if len(a.condStack) == 0 {
			return fmt.Errorf("ENDIF without matching IF")
		}
		a.condStack = a.condStack[:len(a.condStack)-1]
		return nil
	}
	return nil
}

// evalCondition evaluates the truth of an IF/IFDEF/IFNDEF directive.
func (a *Assembler) evalCondition(name string, directive *Directive) (bool, error) {
	switch name {
	case "IF":
		if len(directive.Arguments) != 1 {
			return false, fmt.Errorf("IF requires exactly one expression")
		}
		v, err := a.evalConditionExpr(directive.Arguments[0])
		if err != nil {
			return false, fmt.Errorf("IF condition: %v", err)
		}
		return v != 0, nil
	case "IFDEF", "IFNDEF":
		if len(directive.Arguments) != 1 || directive.Arguments[0].Type != ExpressionSymbol {
			return false, fmt.Errorf("%s requires a single symbol name", name)
		}
		defined := a.symbols.IsDefined(directive.Arguments[0].Symbol)
		if name == "IFNDEF" {
			return !defined, nil
		}
		return defined, nil
	}
	return false, nil
}

// evalConditionExpr evaluates an IF condition expression. Unlike
// evaluateExpression, an undefined symbol is treated as 0 (false) consistently
// in BOTH passes rather than being a pass-1 dummy and a pass-2 error. This gives
// build-tag semantics (an unset symbol is false) and, crucially, guarantees both
// passes take the same branch so addresses cannot diverge. Conditions are
// therefore evaluated against symbols known at that point; forward references in
// conditions resolve to 0 (the easy-regime contract).
func (a *Assembler) evalConditionExpr(expr *Expression) (int, error) {
	switch expr.Type {
	case ExpressionNumber:
		return expr.Value, nil
	case ExpressionSymbol:
		if expr.Symbol == "$" {
			return int(a.address), nil // location counter (pasmo dialect)
		}
		value, exists := a.symbols.Lookup(expr.Symbol)
		if !exists {
			return 0, nil
		}
		return int(value), nil
	case ExpressionBinary:
		left, err := a.evalConditionExpr(expr.Left)
		if err != nil {
			return 0, err
		}
		// Short-circuit boolean operators. A value is true when non-zero.
		switch expr.Operator {
		case "AND":
			if left == 0 {
				return 0, nil
			}
			right, err := a.evalConditionExpr(expr.Right)
			if err != nil {
				return 0, err
			}
			if right != 0 {
				return 1, nil
			}
			return 0, nil
		case "OR":
			if left != 0 {
				return 1, nil
			}
			right, err := a.evalConditionExpr(expr.Right)
			if err != nil {
				return 0, err
			}
			if right != 0 {
				return 1, nil
			}
			return 0, nil
		}
		right, err := a.evalConditionExpr(expr.Right)
		if err != nil {
			return 0, err
		}
		switch expr.Operator {
		case "+":
			return left + right, nil
		case "-":
			return left - right, nil
		case "*":
			return left * right, nil
		case "/":
			if right == 0 {
				return 0, fmt.Errorf("division by zero")
			}
			return left / right, nil
		default:
			return 0, fmt.Errorf("unsupported operator: %s", expr.Operator)
		}
	case ExpressionUnary:
		v, err := a.evalConditionExpr(expr.Left)
		if err != nil {
			return 0, err
		}
		switch expr.Operator {
		case "NOT":
			if v == 0 {
				return 1, nil
			}
			return 0, nil
		case "-":
			return -v, nil
		case "+":
			return v, nil
		default:
			return 0, fmt.Errorf("unsupported unary operator: %s", expr.Operator)
		}
	default:
		return 0, fmt.Errorf("unsupported expression type: %v", expr.Type)
	}
}

// processLineOriginal contains the original processLine logic
func (a *Assembler) processLineOriginal(line *ParsedLine, result *AssemblyResult) error {
	// Handle label
	if line.Label != "" {
		a.lastLabel = line.Label
		if a.pass == 1 {
			// For EQU (dotted or bare), the value comes from the directive argument
			if line.Directive != nil && (strings.ToUpper(line.Directive.Name) == ".EQU" || strings.ToUpper(line.Directive.Name) == "EQU") {
				if len(line.Directive.Arguments) != 1 {
					return fmt.Errorf("EQU directive requires exactly one argument")
				}
				value, err := a.evaluateExpression(line.Directive.Arguments[0])
				if err != nil {
					return err
				}
				a.symbols.Define(line.Label, uint16(value))
			} else {
				// Regular label - use current address
				a.symbols.Define(line.Label, a.address)
			}
		}
	}

	// Handle directive
	if line.Directive != nil {
		return a.processDirective(line.Directive, result, line.LineNumber)
	}

	// Handle instruction
	if line.Instruction != nil {
		return a.processInstruction(line.Instruction, result)
	}

	return nil
}

// macroNameSet returns the set of currently-defined macro names, uppercased, for
// the body parser so it can recognise nested macro calls. A nested call can only
// reference a macro defined before it, which is exactly what the table holds at
// the point a later definition is parsed.
func (a *Assembler) macroNameSet() map[string]bool {
	names := make(map[string]bool)
	for _, m := range a.macroTable.GetAll() {
		// Recognise a nested call by its bare leaf name and, when the macro is in
		// a named package, by its qualified form too, so both `add(...)` and
		// `math.add(...)` parse as calls inside a macro body.
		names[strings.ToUpper(m.Name)] = true
		if m.Package != "" {
			names[strings.ToUpper(m.Package+"."+m.Name)] = true
		}
	}
	return names
}

// instructionNameSet returns the set of instruction mnemonics (uppercased) for
// the line parser, so pasmo no-colon-label detection can distinguish a label
// from an instruction.
func (a *Assembler) instructionNameSet() map[string]bool {
	names := make(map[string]bool)
	for _, m := range a.encoder.GetAllInstructions() {
		names[strings.ToUpper(m)] = true
	}
	return names
}

// shouldInterceptAsMacro reports whether a name at a call position should be
// routed through the macro parser rather than parsed as an instruction. A
// qualified name (package.name) that is defined is always a macro. A bare name
// is a macro only when it is defined, unambiguous across packages, and is not a
// real instruction mnemonic (instructions are never shadowed by a bare macro
// name; the macro must be qualified to be reached). An ambiguous bare name is
// left to detectMacroCall, which reports the qualify-it error.
func (a *Assembler) shouldInterceptAsMacro(name string) bool {
	if _, _, qualified := splitQualified(name); qualified {
		return a.macroTable.IsDefined(name)
	}
	macro, ambiguous, exists := a.macroTable.LookupBare(name)
	if !exists || ambiguous || macro == nil {
		return false
	}
	return !a.encoder.IsInstruction(name)
}

// detectMacroCall decides whether an instruction line is really a macro call,
// honouring package resolution:
//   - a qualified name (package.name) is always a macro call, resolved exactly;
//   - a bare name is a macro call only when it is unambiguous across packages
//     AND is not a real instruction mnemonic (instructions are never shadowed);
//   - a bare name defined in more than one package is ambiguous and reported as
//     an error telling the user to qualify it.
// It returns (call, nil) for a macro call, (nil, nil) for a non-macro line, and
// (nil, err) for an ambiguous bare name.
func (a *Assembler) detectMacroCall(instruction *Instruction) (*MacroCall, error) {
	name := instruction.Mnemonic

	if _, _, qualified := splitQualified(name); qualified {
		if !a.macroTable.IsDefined(name) {
			return nil, nil // not a known macro; let normal handling error if needed
		}
		return a.buildMacroCall(instruction), nil
	}

	// Bare name. A real instruction mnemonic is always the instruction and is
	// never shadowed by a macro - check this before ambiguity, so an instruction
	// like ADD is not reported as an ambiguous macro just because packages define
	// a macro of the same name.
	if a.encoder.IsInstruction(name) {
		return nil, nil
	}

	// Not an instruction: resolve as a macro, reporting ambiguity.
	if _, ambiguous, exists := a.macroTable.LookupBare(name); exists {
		if ambiguous {
			pkgs := a.macroTable.AmbiguousPackages(name)
			return nil, fmt.Errorf("macro '%s' is ambiguous: defined in %s; qualify the call (e.g. %s.%s)",
				name, strings.Join(pkgs, ", "), pkgs[0], name)
		}
		return a.buildMacroCall(instruction), nil
	}

	return nil, nil
}

// buildMacroCall converts an instruction-shaped token into a MacroCall.
func (a *Assembler) buildMacroCall(instruction *Instruction) *MacroCall {
	var arguments []*Expression
	for _, operand := range instruction.Operands {
		if operand.Expression != nil {
			arguments = append(arguments, operand.Expression)
		}
	}
	return &MacroCall{
		Name:      instruction.Mnemonic,
		Arguments: arguments,
		Style:     a.macroStyle,
	}
}

// expandAndProcessMacroCall expands a macro call and processes the result (NEW)
func (a *Assembler) expandAndProcessMacroCall(call *MacroCall, result *AssemblyResult) error {
	// Prevent excessive recursion
	if a.expansionLevel >= a.maxExpansionLevel {
		return fmt.Errorf("macro expansion depth limit exceeded")
	}

	a.expansionLevel++
	defer func() { a.expansionLevel-- }()

	// Expand the macro
	expandedLines, err := a.macroExpander.ExpandMacro(call)
	if err != nil {
		return err
	}

	// Process each expanded line
	for _, expandedLine := range expandedLines {
		err := a.processLine(&expandedLine, result)
		if err != nil {
			return err
		}
	}

	return nil
}

// Helper methods for directive processing (NEW)
func (a *Assembler) isDirective(tokens []Token, pos int, directiveName string) bool {
	if pos >= len(tokens) {
		return false
	}

	return TokenValueMatches(tokens, pos, TokenDirective, directiveName)
}

// processMacroStyleDirective handles .MACRO_STYLE directive (FIXED)
func (a *Assembler) processMacroStyleDirective(tokens []Token, pos int) (int, error) {
	if !TokenValueMatches(tokens, pos, TokenDirective, ".MACRO_STYLE") {
		return pos, fmt.Errorf("expected .MACRO_STYLE directive")
	}
	pos++

	if pos >= len(tokens) || tokens[pos].Type != TokenIdentifier {
		return pos, fmt.Errorf("expected macro style after .MACRO_STYLE")
	}

	styleStr := strings.ToUpper(tokens[pos].Value)
	pos++

	var style MacroStyle
	switch styleStr {
	case "TRADITIONAL":
		style = MacroStyleTraditional
	case "C":
		style = MacroStyleC
	default:
		return pos, fmt.Errorf("unknown macro style: %s", styleStr)
	}

	err := a.SetMacroStyle(style)
	if err != nil {
		return pos, err
	}

	// Skip to end of line
	pos = FindEndOfStatement(tokens, pos)
	if pos < len(tokens) && tokens[pos].Type == TokenNewline {
		pos++
	}

	return pos, nil
}

// processPackageDirective handles `.PACKAGE name`, setting the affiliation for
// macros defined after it (until the next .PACKAGE or end of source). The name
// is an identifier; it does not emit code.
func (a *Assembler) processPackageDirective(tokens []Token, pos int) (int, error) {
	if !TokenValueMatches(tokens, pos, TokenDirective, ".PACKAGE") {
		return pos, fmt.Errorf("expected .PACKAGE directive")
	}
	pos++

	if pos >= len(tokens) || tokens[pos].Type != TokenIdentifier {
		return pos, fmt.Errorf("expected a package name after .PACKAGE")
	}

	name := tokens[pos].Value
	if strings.Contains(name, ".") {
		return pos, fmt.Errorf("package name %q must not contain '.'", name)
	}
	a.currentPackage = name
	pos++

	// Skip to end of line
	pos = FindEndOfStatement(tokens, pos)
	if pos < len(tokens) && tokens[pos].Type == TokenNewline {
		pos++
	}

	return pos, nil
}

// processCallingConventionDirective handles .CALLING_CONVENTION directive (FIXED)
func (a *Assembler) processCallingConventionDirective(tokens []Token, pos int) (int, error) {
	if !TokenValueMatches(tokens, pos, TokenDirective, ".CALLING_CONVENTION") {
		return pos, fmt.Errorf("expected .CALLING_CONVENTION directive")
	}
	pos++

	if pos >= len(tokens) || tokens[pos].Type != TokenIdentifier {
		return pos, fmt.Errorf("expected calling convention after .CALLING_CONVENTION")
	}

	conventionStr := strings.ToUpper(tokens[pos].Value)
	pos++

	switch conventionStr {
	case "REGISTER_FAST":
		a.SetCallingConvention(RegisterFastConvention)
	default:
		return pos, fmt.Errorf("unknown calling convention: %s", conventionStr)
	}

	// Skip to end of line
	pos = FindEndOfStatement(tokens, pos)
	if pos < len(tokens) && tokens[pos].Type == TokenNewline {
		pos++
	}

	return pos, nil
}

// parseRegularLine parses a regular assembly line (ENHANCED)
func (a *Assembler) parseRegularLine(tokens []Token, pos int) (*ParsedLine, int, error) {
	// Collect tokens until newline or EOF
	lineTokens := []Token{}
	
	for pos < len(tokens) && tokens[pos].Type != TokenNewline {
		lineTokens = append(lineTokens, tokens[pos])
		pos++
	}

	if pos < len(tokens) && tokens[pos].Type == TokenNewline {
		lineTokens = append(lineTokens, tokens[pos])
		pos++
	}

	if len(lineTokens) == 0 {
		return nil, pos, nil
	}

	// Use the base parser to parse a regular assembly line
	program, err := a.parser.Parse(lineTokens)
	if err != nil {
		return nil, pos, err
	}

	if len(program.Lines) > 0 {
		return &program.Lines[0], pos, nil
	}

	return nil, pos, nil
}

// emitHexdumpBytes handles the multi-digit 0x/0d data form for .DB/.DM (unit=1)
// and .DW (unit=2). The raw digit string is chopped into groups of unitBytes*2
// hex digits (or decimal values fitting the unit), each emitted in the
// directive's order. Returns true if the argument was a hexdump and was handled.
func (a *Assembler) emitHexdumpBytes(arg *Expression, unitBytes int) (bool, error) {
	// Spaced form: 0x DE AD  /  0d 222 173 — each group is one unit.
	if len(arg.RadixGroups) > 0 {
		for _, g := range arg.RadixGroups {
			var val int64
			var err error
			if arg.RawRadix == 16 {
				if len(g) != unitBytes*2 {
					return true, fmt.Errorf("hex group %q is not %d digit(s) for this directive", g, unitBytes*2)
				}
				val, err = strconv.ParseInt(g, 16, 32)
			} else {
				val, err = strconv.ParseInt(g, 10, 32)
				maxv := int64(1)<<(unitBytes*8) - 1
				if err == nil && (val < 0 || val > maxv) {
					return true, fmt.Errorf("decimal value %s does not fit %d byte(s)", g, unitBytes)
				}
			}
			if err != nil {
				return true, fmt.Errorf("invalid value %q in radix data", g)
			}
			if a.pass == 2 {
				if unitBytes == 1 {
					a.output = append(a.output, uint8(val))
				} else {
					a.output = append(a.output, uint8(val&0xFF), uint8((val>>8)&0xFF))
				}
			}
			a.address += uint16(unitBytes)
		}
		return true, nil
	}

	if arg.RawDigits == "" || arg.RawRadix != 16 {
		return false, nil
	}
	digits := arg.RawDigits
	// A literal that fits a single unit is not a hexdump - let normal evaluation
	// handle it (e.g. .DB 0x42, .DW 0x1234).
	if len(digits) <= unitBytes*2 {
		return false, nil
	}
	if len(digits)%(unitBytes*2) != 0 {
		return true, fmt.Errorf("hex data 0x%s does not divide into %d-byte units", digits, unitBytes)
	}
	groupDigits := unitBytes * 2
	for i := 0; i < len(digits); i += groupDigits {
		group := digits[i : i+groupDigits]
		val, err := strconv.ParseInt(group, 16, 32)
		if err != nil {
			return true, fmt.Errorf("invalid hex group %q", group)
		}
		if a.pass == 2 {
			if unitBytes == 1 {
				a.output = append(a.output, uint8(val))
			} else {
				// .DW: Z80 little-endian word.
				a.output = append(a.output, uint8(val&0xFF), uint8((val>>8)&0xFF))
			}
		}
		a.address += uint16(unitBytes)
	}
	return true, nil
}

// processDirective handles assembler directives (ENHANCED with .DS support)
func (a *Assembler) processDirective(directive *Directive, result *AssemblyResult, lineNumber int) error {
	switch strings.ToUpper(directive.Name) {
	case ".ORG", "ORG":
		if len(directive.Arguments) != 1 {
			return fmt.Errorf("ORG directive requires exactly one argument")
		}
		addr, err := a.evaluateExpression(directive.Arguments[0])
		if err != nil {
			return err
		}
		a.address = uint16(addr)
		if !a.originSet {
			a.origin = a.address
			a.originSet = true
		}

	case ".DB", "DEFB", "DB", "DEFM":
		for _, arg := range directive.Arguments {
			if arg.Type == ExpressionString {
				// Handle string literal with character mapping
				for _, ch := range arg.StringValue {
					mappedChar, warning, err := a.mapCharacterWithReplacement(ch, lineNumber)
					if err != nil {
						return fmt.Errorf("DB string encoding error: %v", err)
					}
					if warning != nil && a.pass == 2 {
						a.warnings = append(a.warnings, *warning)
					}
					if a.pass == 2 {
						a.output = append(a.output, mappedChar)
					}
					a.address++
				}
			} else {
				// Handle numeric expression (or multi-byte 0x hexdump form)
				if handled, err := a.emitHexdumpBytes(arg, 1); err != nil {
					return err
				} else if handled {
					continue
				}
				value, err := a.evaluateExpression(arg)
				if err != nil {
					return err
				}
				if value < 0 || value > 255 {
					return fmt.Errorf("DB value out of range: %d", value)
				}
				if a.pass == 2 {
					a.output = append(a.output, uint8(value))
				}
				a.address++
			}
		}

	case ".DW", "DEFW", "DW":
		for _, arg := range directive.Arguments {
			if handled, err := a.emitHexdumpBytes(arg, 2); err != nil {
				return err
			} else if handled {
				continue
			}
			value, err := a.evaluateExpression(arg)
			if err != nil {
				return err
			}
			if value < -32768 || value > 65535 {
				return fmt.Errorf("DW value out of range: %d", value)
			}
			if a.pass == 2 {
				// Little-endian
				a.output = append(a.output, uint8(value), uint8(value>>8))
			}
			a.address += 2
		}

	case ".INCBIN", "INCBIN":
		// INCBIN "file"[,skip[,length]] - insert the raw bytes of a binary file
		// at the current address. Optional skip (offset into the file) and length
		// (number of bytes to insert) follow the filename. Paths are resolved
		// relative to baseDir, matching INCLUDE.
		if len(directive.Arguments) == 0 || directive.Arguments[0].Type != ExpressionString {
			return fmt.Errorf("INCBIN requires a quoted filename")
		}
		name := directive.Arguments[0].StringValue
		path := name
		if !filepath.IsAbs(path) {
			base := a.baseDir
			if base == "" {
				base = "."
			}
			path = filepath.Join(base, name)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("INCBIN cannot read %q: %v", name, err)
		}

		skip := 0
		if len(directive.Arguments) > 1 {
			skip, err = a.evaluateExpression(directive.Arguments[1])
			if err != nil {
				return err
			}
			if skip < 0 {
				return fmt.Errorf("INCBIN skip cannot be negative: %d", skip)
			}
		}
		if skip > len(data) {
			return fmt.Errorf("INCBIN skip %d is past end of %q (%d bytes)", skip, name, len(data))
		}
		data = data[skip:]

		if len(directive.Arguments) > 2 {
			length, err := a.evaluateExpression(directive.Arguments[2])
			if err != nil {
				return err
			}
			if length < 0 {
				return fmt.Errorf("INCBIN length cannot be negative: %d", length)
			}
			if length > len(data) {
				return fmt.Errorf("INCBIN length %d exceeds available %d bytes in %q after skip", length, len(data), name)
			}
			data = data[:length]
		}
		if len(directive.Arguments) > 3 {
			return fmt.Errorf("INCBIN takes at most filename, skip, length")
		}

		if a.pass == 2 {
			a.output = append(a.output, data...)
		}
		a.address += uint16(len(data))

	case ".DS", "DEFS", "DS":
		// Define Storage - reserve bytes
		if len(directive.Arguments) == 0 {
			return fmt.Errorf("DS directive requires at least one argument (byte count)")
		}
		
		// Get byte count
		count, err := a.evaluateExpression(directive.Arguments[0])
		if err != nil {
			return err
		}
		if count < 0 {
			return fmt.Errorf("DS byte count cannot be negative: %d", count)
		}
		
		// Get fill value (default to 0)
		fillValue := 0
		if len(directive.Arguments) > 1 {
			fillValue, err = a.evaluateExpression(directive.Arguments[1])
			if err != nil {
				return err
			}
			if fillValue < 0 || fillValue > 255 {
				return fmt.Errorf("DS fill value out of range: %d", fillValue)
			}
		}
		
		// Reserve the bytes
		if a.pass == 2 {
			for i := 0; i < count; i++ {
				a.output = append(a.output, uint8(fillValue))
			}
		}
		a.address += uint16(count)

	case ".EQU", "EQU":
		// EQU defines a constant - handled in processLineOriginal

	case ".PASMO", "PASMO", ".ZENAS", "ZENAS":
		// Dialect switch. The effect happens in the lexer as the directive is
		// scanned; here it is a no-op that only needs to be recognised.

	case ".EXPECT", "EXPECT":
		// Test expectation. Only legal in a *_test.asm file, so test metadata can
		// never appear in a production build. Must attach to a test_* routine.
		if !a.inTestFile {
			return fmt.Errorf(".EXPECT is only allowed in a *_test.asm file")
		}
		if !strings.HasPrefix(a.lastLabel, "test_") {
			return fmt.Errorf(".EXPECT must follow a test_ routine (nearest label: %q)", a.lastLabel)
		}
		if a.pass == 2 {
			a.testSpecs = append(a.testSpecs, TestSpec{
				Label:  a.lastLabel,
				Expect: directive.RawArgs,
			})
		}

	case ".MATCH", "MATCH":
		// Memory-content assertion. Test-only, must attach to a test_* routine.
		if !a.inTestFile {
			return fmt.Errorf(".MATCH is only allowed in a *_test.asm file")
		}
		if !strings.HasPrefix(a.lastLabel, "test_") {
			return fmt.Errorf(".MATCH must follow a test_ routine (nearest label: %q)", a.lastLabel)
		}
		if a.pass == 2 {
			a.testSpecs = append(a.testSpecs, TestSpec{
				Label: a.lastLabel,
				Match: directive.RawArgs,
			})
		}

	case ".END", "END":
		// .END directive - marks end of assembly

	default:
		return fmt.Errorf("unknown directive: %s", directive.Name)
	}

	return nil
}

// processInstruction handles Z80 instructions (IMPLEMENTED)
func (a *Assembler) processInstruction(instruction *Instruction, result *AssemblyResult) error {
	// Resolve operands to concrete values
	var resolvedOperands []*ResolvedOperand
	
	for _, operand := range instruction.Operands {
		resolvedOperand, err := a.resolveOperand(operand)
		if err != nil {
			return fmt.Errorf("error resolving operand in %s: %v", instruction.Mnemonic, err)
		}
		resolvedOperands = append(resolvedOperands, resolvedOperand)
	}

	// Relative jumps (JR cc,e / JR e / DJNZ e) carry their target as an absolute
	// address after operand resolution. Convert it to a signed displacement
	// relative to the instruction *after* this one. Every relative jump is two
	// bytes, so the reference point is the current address + 2. The displacement
	// byte is what the encoder emits.
	switch strings.ToUpper(instruction.Mnemonic) {
	case "JR", "DJNZ":
		if len(resolvedOperands) > 0 {
			target := resolvedOperands[len(resolvedOperands)-1]
			disp := target.Value - (int(a.address) + 2)
			if a.pass == 2 && (disp < -128 || disp > 127) {
				return fmt.Errorf("%s target out of range (displacement %d, must be -128..127)", instruction.Mnemonic, disp)
			}
			target.Value = disp
		}
	}
	
	// Use the encoder to generate machine code
	machineCode, err := a.encoder.Encode(instruction.Mnemonic, resolvedOperands)
	if err != nil {
		return fmt.Errorf("error encoding instruction %s: %v", instruction.Mnemonic, err)
	}
	
	// Only add to output on pass 2
	if a.pass == 2 {
		a.output = append(a.output, machineCode...)
	}
	
	// Update address
	a.address += uint16(len(machineCode))
	
	return nil
}

// resolveOperand converts a parsed operand to a resolved operand with concrete values
func (a *Assembler) resolveOperand(operand *Operand) (*ResolvedOperand, error) {
	resolved := &ResolvedOperand{
		Type:         operand.Type,
		Register:     operand.Register,
		Displacement: operand.Displacement,
		Condition:    operand.Condition,
	}

	// Evaluate an IX/IY displacement expression now (symbols are resolvable at
	// assembly time). The Z80 displacement is a signed byte; range-check it so an
	// out-of-range value is a clear error rather than a silent wrap. In pass 1 a
	// forward symbol resolves to 0, so the range check is only enforced in pass 2.
	if operand.DisplacementExpr != nil {
		v, err := a.evaluateExpression(operand.DisplacementExpr)
		if err != nil {
			return nil, fmt.Errorf("invalid displacement: %v", err)
		}
		d := operand.DisplacementSign * v
		if a.pass == 2 && (d < -128 || d > 127) {
			return nil, fmt.Errorf("index displacement %d out of range (must be -128..127)", d)
		}
		resolved.Displacement = d
	}
	
	// Resolve expression if present
	if operand.Expression != nil {
		value, err := a.evaluateExpression(operand.Expression)
		if err != nil {
			return nil, err
		}
		resolved.Value = value
		
		// Determine operand type based on value range if not explicitly set
		if operand.Type == OperandImmediate16 {
			if value >= -128 && value <= 255 {
				// Could be 8-bit, but let the encoder decide
				resolved.Type = OperandImmediate16 // Keep as 16-bit, encoder will optimize
			}
		}
	}
	
	return resolved, nil
}

// evaluateExpression evaluates numeric expressions and symbol references (ORIGINAL method)
func (a *Assembler) evaluateExpression(expr *Expression) (int, error) {
	switch expr.Type {
	case ExpressionNumber:
		return expr.Value, nil

	case ExpressionSymbol:
		if expr.Symbol == "$" {
			return int(a.address), nil // location counter (pasmo dialect)
		}
		value, exists := a.symbols.Lookup(expr.Symbol)
		if !exists {
			if a.pass == 1 {
				// Forward reference: not yet defined in pass 1. Use a dummy value;
				// it resolves correctly in pass 2. (ORG and other addressing math
				// that depends on an already-defined symbol still works because
				// backward references are looked up for real, above.)
				return 0, nil
			}
			return 0, fmt.Errorf("undefined symbol: %s", expr.Symbol)
		}
		return int(value), nil

	case ExpressionBinary:
		left, err := a.evaluateExpression(expr.Left)
		if err != nil {
			return 0, err
		}
		right, err := a.evaluateExpression(expr.Right)
		if err != nil {
			return 0, err
		}

		switch expr.Operator {
		case "+":
			return left + right, nil
		case "-":
			return left - right, nil
		case "*":
			return left * right, nil
		case "/":
			if right == 0 {
				return 0, fmt.Errorf("division by zero")
			}
			return left / right, nil
		default:
			return 0, fmt.Errorf("unsupported operator: %s", expr.Operator)
		}

	case ExpressionUnary:
		v, err := a.evaluateExpression(expr.Left)
		if err != nil {
			return 0, err
		}
		switch expr.Operator {
		case "-":
			return -v, nil
		case "+":
			return v, nil
		default:
			return 0, fmt.Errorf("unsupported unary operator: %s", expr.Operator)
		}

	case ExpressionString:
		// A character literal used in a value context (e.g. FONT_FIRST EQU '!')
		// evaluates to its character code, mapped through the active charset so it
		// matches how the same character would be emitted by DB.
		runes := []rune(expr.StringValue)
		if len(runes) == 0 {
			return 0, fmt.Errorf("empty character literal")
		}
		if len(runes) != 1 {
			return 0, fmt.Errorf("multi-character literal %q is not a scalar value", expr.StringValue)
		}
		mapped, _, err := a.mapCharacterWithReplacement(runes[0], 0)
		if err != nil {
			return 0, err
		}
		return int(mapped), nil

	default:
		return 0, fmt.Errorf("unsupported expression type: %v", expr.Type)
	}
}

// mapCharacterWithReplacement maps characters with replacement fallback (ORIGINAL method)
func (a *Assembler) mapCharacterWithReplacement(ch rune, lineNumber int) (uint8, *AssemblyWarning, error) {
	// Simplified version - just return ASCII for now
	if ch <= 127 {
		return uint8(ch), nil, nil
	}

	// Simple fallback for non-ASCII
	warning := &AssemblyWarning{
		Line:    lineNumber,
		Message: fmt.Sprintf("Non-ASCII character '%c' replaced with '?'", ch),
		Type:    CharacterReplacement,
	}
	return uint8('?'), warning, nil
}

// GetMacroTable returns the macro table for inspection
func (a *Assembler) GetMacroTable() *MacroTable {
	return a.macroTable
}

// ListMacros returns a list of all defined macros
func (a *Assembler) ListMacros() []string {
	return a.macroTable.ListMacros()
}

// ClearMacros removes all macro definitions
func (a *Assembler) ClearMacros() {
	a.macroTable.Clear()
	a.macroExpander.Reset()
}