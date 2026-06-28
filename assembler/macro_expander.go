package assembler

import (
	"fmt"
	"strings"
)

// MacroExpander handles the expansion of macro calls into assembly code
type MacroExpander struct {
	table       *MacroTable
	symbols     *SymbolTable
	uniqueID    int
	expansion   map[string]int // Track expansion depth to prevent infinite recursion
}

// NewMacroExpander creates a new macro expander
func NewMacroExpander(table *MacroTable, symbols *SymbolTable) *MacroExpander {
	return &MacroExpander{
		table:     table,
		symbols:   symbols,
		uniqueID:  1,
		expansion: make(map[string]int),
	}
}

// ExpandMacro expands a macro call into a list of assembly lines (FIXED)
func (me *MacroExpander) ExpandMacro(call *MacroCall) ([]ParsedLine, error) {
	// Validate the call
	macro, err := me.table.ValidateCall(call)
	if err != nil {
		return nil, err
	}
	
	// Check for recursive expansion
	expansionKey := fmt.Sprintf("%s_%d", call.Name, call.LineNumber)
	if depth, exists := me.expansion[expansionKey]; exists && depth > 10 {
		return nil, fmt.Errorf("macro expansion depth limit exceeded for %s", call.Name)
	}
	me.expansion[expansionKey]++
	defer func() {
		me.expansion[expansionKey]--
		if me.expansion[expansionKey] == 0 {
			delete(me.expansion, expansionKey)
		}
	}()
	
	// Create parameter substitution map
	substitution, err := me.createParameterSubstitution(macro, call)
	if err != nil {
		return nil, err
	}
	
	// Generate unique labels for this expansion
	labelMap := me.generateUniqueLabelMap(macro)

	// Note: the unique labels are NOT pre-defined here. expandMacroBody rewrites
	// both the label definitions and the references to them, so each renamed
	// label appears as a real label in the expanded output and the normal
	// two-pass resolver assigns its true address. Pre-defining them (formerly to
	// 0) corrupted relative-jump arithmetic - a DJNZ to such a label resolved
	// against 0 instead of the label's real address.

	// Expand the macro body
	expandedLines, err := me.expandMacroBody(macro.Body, substitution, labelMap)
	if err != nil {
		return nil, fmt.Errorf("error expanding macro %s: %v", call.Name, err)
	}
	
	return expandedLines, nil
}

// createParameterSubstitution creates a map of parameter names to their values
func (me *MacroExpander) createParameterSubstitution(macro *MacroDefinition, call *MacroCall) (map[string]*Expression, error) {
	substitution := make(map[string]*Expression)
	
	// Map parameters to arguments
	for i, param := range macro.Parameters {
		if i >= len(call.Arguments) {
			return nil, fmt.Errorf("macro %s parameter %s has no corresponding argument", macro.Name, param.Name)
		}
		
		arg := call.Arguments[i]
		
		// Validate argument range if it's a literal value.
		if arg.Type == ExpressionNumber {
			if err := param.Type.ValidateValue(arg.Value); err != nil {
				return nil, fmt.Errorf("macro %s parameter %s: %v", macro.Name, param.Name, err)
			}
		}

		// Width-signature check. When a parameter declares a width (uint8_t /
		// uint16_t), the argument's width must match it exactly - a narrower
		// argument would lose the high bits the signature promises, a wider one
		// would be silently truncated. Arguments whose width cannot be determined
		// (symbols, expressions) are trusted and not checked.
		if declaredBits, checkable := param.Type.DeclaredWidthBits(); checkable {
			if argBits, known := argumentWidthBits(arg); known && argBits != declaredBits {
				return nil, fmt.Errorf(
					"macro %s parameter %s declares %d-bit width but argument is %d-bit; "+
						"widths must match (no implicit narrowing or widening across a signature)",
					macro.Name, param.Name, declaredBits, argBits)
			}
		}
		
		substitution[strings.ToUpper(param.Name)] = arg
	}
	
	return substitution, nil
}

// argumentWidthBits returns the width in bits of a macro-call argument, when it
// can be determined. Only literal numbers have a knowable width here: a value
// that fits in a byte is 8-bit, otherwise 16-bit. Symbols and compound
// expressions return known=false and are not width-checked.
func argumentWidthBits(arg *Expression) (bits int, known bool) {
	if arg == nil || arg.Type != ExpressionNumber {
		return 0, false
	}
	v := arg.Value
	if v < 0 {
		v = -v
	}
	if v <= 0xFF {
		return 8, true
	}
	return 16, true
}

// generateUniqueLabelMap creates unique labels for the macro expansion
func (me *MacroExpander) generateUniqueLabelMap(macro *MacroDefinition) map[string]string {
	labelMap := make(map[string]string)
	
	// Find all labels in the macro body
	labels := me.findLabelsInMacro(macro.Body)
	
	// Generate unique replacements
	for _, label := range labels {
		uniqueLabel := fmt.Sprintf("%s_%d_%d", label, macro.UniqueID, me.uniqueID)
		labelMap[strings.ToUpper(label)] = uniqueLabel
	}
	
	me.uniqueID++
	return labelMap
}

// findLabelsInMacro finds all labels defined in a macro body
func (me *MacroExpander) findLabelsInMacro(body []ParsedLine) []string {
	var labels []string
	
	for _, line := range body {
		if line.Label != "" {
			labels = append(labels, line.Label)
		}
	}
	
	return labels
}

// expandMacroBody expands the macro body with parameter substitution and unique labels
func (me *MacroExpander) expandMacroBody(body []ParsedLine, substitution map[string]*Expression, labelMap map[string]string) ([]ParsedLine, error) {
	var expandedLines []ParsedLine
	
	for _, line := range body {
		expandedLine := line // Copy the line
		
		// Substitute label if present
		if expandedLine.Label != "" {
			if uniqueLabel, exists := labelMap[strings.ToUpper(expandedLine.Label)]; exists {
				expandedLine.Label = uniqueLabel
			}
		}
		
		// Substitute in instruction operands
		if expandedLine.Instruction != nil {
			newInstruction := *expandedLine.Instruction // Copy instruction
			
			// Substitute operands
			var newOperands []*Operand
			for _, operand := range newInstruction.Operands {
				newOperand, err := me.substituteOperand(operand, substitution, labelMap)
				if err != nil {
					return nil, err
				}
				newOperands = append(newOperands, newOperand)
			}
			newInstruction.Operands = newOperands
			expandedLine.Instruction = &newInstruction
		}
		
		// Substitute in directive arguments
		if expandedLine.Directive != nil {
			newDirective := *expandedLine.Directive // Copy directive
			
			// Substitute arguments
			var newArguments []*Expression
			for _, arg := range newDirective.Arguments {
				newArg, err := me.substituteExpression(arg, substitution, labelMap)
				if err != nil {
					return nil, err
				}
				newArguments = append(newArguments, newArg)
			}
			newDirective.Arguments = newArguments
			expandedLine.Directive = &newDirective
		}
		
		expandedLines = append(expandedLines, expandedLine)
	}
	
	return expandedLines, nil
}

// substituteOperand performs parameter substitution in an operand
func (me *MacroExpander) substituteOperand(operand *Operand, substitution map[string]*Expression, labelMap map[string]string) (*Operand, error) {
	newOperand := *operand // Copy operand
	
	// Substitute expression if present
	if newOperand.Expression != nil {
		newExpr, err := me.substituteExpression(newOperand.Expression, substitution, labelMap)
		if err != nil {
			return nil, err
		}
		newOperand.Expression = newExpr
	}
	
	return &newOperand, nil
}

// substituteExpression performs parameter substitution in an expression
func (me *MacroExpander) substituteExpression(expr *Expression, substitution map[string]*Expression, labelMap map[string]string) (*Expression, error) {
	switch expr.Type {
	case ExpressionSymbol:
		// Check for parameter substitution
		upperSymbol := strings.ToUpper(expr.Symbol)
		if paramExpr, exists := substitution[upperSymbol]; exists {
			// Return the parameter value
			return paramExpr, nil
		}
		
		// Check for label substitution
		if uniqueLabel, exists := labelMap[upperSymbol]; exists {
			return &Expression{
				Type:   ExpressionSymbol,
				Symbol: uniqueLabel,
			}, nil
		}
		
		// No substitution needed
		return expr, nil
		
	case ExpressionBinary:
		// Recursively substitute left and right expressions
		newLeft, err := me.substituteExpression(expr.Left, substitution, labelMap)
		if err != nil {
			return nil, err
		}
		
		newRight, err := me.substituteExpression(expr.Right, substitution, labelMap)
		if err != nil {
			return nil, err
		}
		
		return &Expression{
			Type:     ExpressionBinary,
			Operator: expr.Operator,
			Left:     newLeft,
			Right:    newRight,
		}, nil
		
	case ExpressionUnary:
		// Recursively substitute the inner expression
		newLeft, err := me.substituteExpression(expr.Left, substitution, labelMap)
		if err != nil {
			return nil, err
		}
		
		return &Expression{
			Type:     ExpressionUnary,
			Operator: expr.Operator,
			Left:     newLeft,
		}, nil
		
	case ExpressionNumber, ExpressionString:
		// No substitution needed for literals
		return expr, nil
		
	default:
		return nil, fmt.Errorf("unknown expression type: %v", expr.Type)
	}
}

// GenerateCallingConventionCode generates parameter passing code according to calling convention
func (me *MacroExpander) GenerateCallingConventionCode(call *MacroCall, macro *MacroDefinition) ([]ParsedLine, []ParsedLine, error) {
	convention := me.table.GetCallingConvention()
	
	var setupLines []ParsedLine
	var cleanupLines []ParsedLine
	
	// Generate parameter setup code
	for i := range macro.Parameters {
		if i >= len(call.Arguments) {
			continue
		}
		
		if i >= len(convention.ParamRegs) {
			return nil, nil, fmt.Errorf("macro %s has too many parameters for calling convention", macro.Name)
		}
		
		reg := convention.ParamRegs[i]
		arg := call.Arguments[i]
		
		// Generate LD instruction to load parameter into register
		if arg.Type == ExpressionNumber {
			// LD reg, immediate
			setupLines = append(setupLines, ParsedLine{
				Instruction: &Instruction{
					Mnemonic: "LD",
					Operands: []*Operand{
						{
							Type:     OperandRegister8,
							Register: reg,
						},
						{
							Type:       OperandImmediate8,
							Expression: arg,
						},
					},
				},
			})
		} else {
			// LD reg, symbol (more complex case)
			setupLines = append(setupLines, ParsedLine{
				Instruction: &Instruction{
					Mnemonic: "LD",
					Operands: []*Operand{
						{
							Type:     OperandRegister8,
							Register: reg,
						},
						{
							Type:       OperandImmediate16,
							Expression: arg,
						},
					},
				},
			})
		}
	}
	
	// Generate cleanup code if needed (for now, minimal cleanup)
	// In a full implementation, this would restore caller-saved registers
	
	return setupLines, cleanupLines, nil
}

// Reset clears the expander state
func (me *MacroExpander) Reset() {
	me.uniqueID = 1
	me.expansion = make(map[string]int)
}

// GetExpansionStats returns statistics about macro expansions
func (me *MacroExpander) GetExpansionStats() map[string]interface{} {
	return map[string]interface{}{
		"unique_id":        me.uniqueID,
		"active_expansions": len(me.expansion),
		"expansion_depth":   me.expansion,
	}
}