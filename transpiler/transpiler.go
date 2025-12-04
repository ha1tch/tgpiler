// Package transpiler converts T-SQL source code to Go.
package transpiler

import (
	"fmt"
	"strings"

	"github.com/ha1tch/tsqlparser"
	"github.com/ha1tch/tsqlparser/ast"
)

// Transpile converts T-SQL source code to Go source code.
func Transpile(source string, packageName string) (string, error) {
	program, errors := tsqlparser.Parse(source)
	if len(errors) > 0 {
		return "", fmt.Errorf("parse errors:\n%s", strings.Join(errors, "\n"))
	}

	t := newTranspiler()
	t.packageName = packageName
	t.comments = buildCommentIndex(source)
	return t.transpile(program)
}

type transpiler struct {
	imports       map[string]bool
	output        strings.Builder
	indent        int
	inProcBody    bool
	symbols       *symbolTable
	outputParams  []*ast.ParameterDef
	hasReturnCode bool
	packageName   string
	comments      *commentIndex
}

func newTranspiler() *transpiler {
	return &transpiler{
		imports: make(map[string]bool),
		symbols: newSymbolTable(),
	}
}

// emitComments returns Go comment lines for the given signature.
func (t *transpiler) emitComments(sig string) string {
	comments := t.comments.lookup(sig)
	if len(comments) == 0 {
		return ""
	}

	var lines []string
	for _, c := range comments {
		lines = append(lines, "// "+c)
	}
	return strings.Join(lines, "\n"+t.indentStr()) + "\n" + t.indentStr()
}

// emitTrailingComment returns a trailing comment if one exists.
func (t *transpiler) emitTrailingComment(sig string) string {
	comment := t.comments.lookupTrailing(sig)
	if comment == "" {
		return ""
	}
	return " // " + comment
}

func (t *transpiler) transpile(program *ast.Program) (string, error) {
	// First pass: transpile all statements to determine imports
	var bodies []string

	for _, stmt := range program.Statements {
		body, err := t.transpileStatement(stmt)
		if err != nil {
			return "", err
		}
		if body != "" {
			bodies = append(bodies, body)
		}
	}

	// Build final output with imports
	var out strings.Builder
	out.WriteString(fmt.Sprintf("package %s\n\n", t.packageName))

	if len(t.imports) > 0 {
		out.WriteString("import (\n")
		for imp := range t.imports {
			out.WriteString(fmt.Sprintf("\t%q\n", imp))
		}
		out.WriteString(")\n\n")
	}

	out.WriteString(strings.Join(bodies, "\n\n"))
	out.WriteString("\n")

	return out.String(), nil
}

func (t *transpiler) transpileStatement(stmt ast.Statement) (string, error) {
	switch s := stmt.(type) {
	case *ast.CreateProcedureStatement:
		return t.transpileCreateProcedure(s)
	case *ast.DeclareStatement:
		return t.transpileDeclare(s)
	case *ast.SetStatement:
		return t.transpileSet(s)
	case *ast.IfStatement:
		return t.transpileIf(s)
	case *ast.WhileStatement:
		return t.transpileWhile(s)
	case *ast.BeginEndBlock:
		return t.transpileBlock(s)
	case *ast.TryCatchStatement:
		return t.transpileTryCatch(s)
	case *ast.ReturnStatement:
		return t.transpileReturn(s)
	case *ast.BreakStatement:
		return "break", nil
	case *ast.ContinueStatement:
		return "continue", nil
	case *ast.PrintStatement:
		return t.transpilePrint(s)
	default:
		return "", fmt.Errorf("unsupported statement type: %T", stmt)
	}
}

func (t *transpiler) transpileCreateProcedure(proc *ast.CreateProcedureStatement) (string, error) {
	var out strings.Builder

	// Reset symbol table for new procedure scope
	t.symbols = newSymbolTable()

	// Get procedure name for comment lookup
	procName := proc.Name.Parts[len(proc.Name.Parts)-1].Value
	sig := "PROC:" + strings.ToLower(procName)

	// Emit leading comments for the procedure
	if comments := t.comments.lookup(sig); len(comments) > 0 {
		for _, c := range comments {
			out.WriteString("// " + c + "\n")
		}
	}

	// Separate input and output parameters
	var inputParams []string
	var outputParams []*ast.ParameterDef
	
	for _, p := range proc.Parameters {
		goType, err := t.mapDataType(p.DataType)
		if err != nil {
			return "", fmt.Errorf("parameter %s: %w", p.Name, err)
		}
		paramName := goIdentifier(strings.TrimPrefix(p.Name, "@"))
		
		// Record parameter type in symbol table
		t.symbols.define(paramName, typeInfoFromDataType(p.DataType))
		
		if p.Output {
			outputParams = append(outputParams, p)
		} else {
			inputParams = append(inputParams, fmt.Sprintf("%s %s", paramName, goType))
		}
	}

	// Function signature
	funcName := goIdentifier(procName)
	out.WriteString(fmt.Sprintf("func %s(", funcName))
	out.WriteString(strings.Join(inputParams, ", "))
	out.WriteString(")")

	// Return type(s)
	hasReturn := t.procedureHasReturn(proc)
	if len(outputParams) > 0 || hasReturn {
		out.WriteString(" (")
		var returns []string
		for _, p := range outputParams {
			goType, _ := t.mapDataType(p.DataType)
			paramName := goIdentifier(strings.TrimPrefix(p.Name, "@"))
			returns = append(returns, fmt.Sprintf("%s %s", paramName, goType))
		}
		if hasReturn {
			returns = append(returns, "returnCode int32")
		}
		out.WriteString(strings.Join(returns, ", "))
		out.WriteString(")")
	}

	out.WriteString(" {\n")

	// Note: OUTPUT parameters become named return values in Go,
	// so they don't need separate var declarations
	t.indent = 1

	// Track output params and return for use in RETURN statements
	t.outputParams = outputParams
	t.hasReturnCode = hasReturn

	// Body
	t.inProcBody = true
	if proc.Body != nil {
		for _, stmt := range proc.Body.Statements {
			body, err := t.transpileStatement(stmt)
			if err != nil {
				return "", err
			}
			if body != "" {
				out.WriteString(t.indentStr())
				out.WriteString(body)
				out.WriteString("\n")
			}
		}
	}
	t.inProcBody = false

	// Final return if we have output params or return code, 
	// but only if the block doesn't already end with a return
	if (len(outputParams) > 0 || hasReturn) && !t.blockEndsWithReturn(proc.Body) {
		out.WriteString(t.indentStr())
		out.WriteString(t.buildReturnStatement(nil))
		out.WriteString("\n")
	}

	t.indent = 0
	out.WriteString("}")

	// Clear procedure-specific state
	t.outputParams = nil
	t.hasReturnCode = false

	return out.String(), nil
}

// procedureHasReturn checks if a procedure has any RETURN statements with values.
func (t *transpiler) procedureHasReturn(proc *ast.CreateProcedureStatement) bool {
	if proc.Body == nil {
		return false
	}
	return t.blockHasReturn(proc.Body)
}

func (t *transpiler) blockHasReturn(block *ast.BeginEndBlock) bool {
	for _, stmt := range block.Statements {
		if t.statementHasReturn(stmt) {
			return true
		}
	}
	return false
}

func (t *transpiler) statementHasReturn(stmt ast.Statement) bool {
	switch s := stmt.(type) {
	case *ast.ReturnStatement:
		return s.Value != nil
	case *ast.BeginEndBlock:
		return t.blockHasReturn(s)
	case *ast.IfStatement:
		if t.statementHasReturn(s.Consequence) {
			return true
		}
		if s.Alternative != nil && t.statementHasReturn(s.Alternative) {
			return true
		}
	case *ast.WhileStatement:
		return t.statementHasReturn(s.Body)
	case *ast.TryCatchStatement:
		if s.TryBlock != nil && t.blockHasReturn(s.TryBlock) {
			return true
		}
		if s.CatchBlock != nil && t.blockHasReturn(s.CatchBlock) {
			return true
		}
	}
	return false
}

// blockEndsWithReturn checks if a block's last statement is a return.
func (t *transpiler) blockEndsWithReturn(block *ast.BeginEndBlock) bool {
	if block == nil || len(block.Statements) == 0 {
		return false
	}
	lastStmt := block.Statements[len(block.Statements)-1]
	switch s := lastStmt.(type) {
	case *ast.ReturnStatement:
		return true
	case *ast.BeginEndBlock:
		return t.blockEndsWithReturn(s)
	}
	return false
}

// buildReturnStatement generates a return statement with output params and optional return code.
func (t *transpiler) buildReturnStatement(returnValue ast.Expression) string {
	var parts []string
	
	for _, p := range t.outputParams {
		paramName := goIdentifier(strings.TrimPrefix(p.Name, "@"))
		parts = append(parts, paramName)
	}
	
	if t.hasReturnCode {
		if returnValue != nil {
			val, err := t.transpileExpression(returnValue)
			if err != nil {
				parts = append(parts, "0")
			} else {
				parts = append(parts, val)
			}
		} else {
			parts = append(parts, "0")
		}
	}
	
	if len(parts) == 0 {
		return "return"
	}
	return "return " + strings.Join(parts, ", ")
}

func (t *transpiler) transpileDeclare(decl *ast.DeclareStatement) (string, error) {
	var parts []string

	for i, v := range decl.Variables {
		if v.TableType != nil {
			return "", fmt.Errorf("table variables not supported")
		}

		goType, err := t.mapDataType(v.DataType)
		if err != nil {
			return "", fmt.Errorf("variable %s: %w", v.Name, err)
		}

		varName := goIdentifier(strings.TrimPrefix(v.Name, "@"))

		// Record variable type in symbol table
		t.symbols.define(varName, typeInfoFromDataType(v.DataType))

		// Look up comments for first variable in declaration
		var prefix string
		if i == 0 {
			sig := "DECLARE:" + strings.ToLower(strings.TrimPrefix(v.Name, "@"))
			if comments := t.comments.lookup(sig); len(comments) > 0 {
				for _, c := range comments {
					prefix += "// " + c + "\n" + t.indentStr()
				}
			}
		}

		if v.Value != nil {
			valExpr, err := t.transpileExpression(v.Value)
			if err != nil {
				return "", err
			}
			// Check if we need to convert the initialiser to decimal
			ti := t.symbols.lookup(varName)
			if ti != nil && ti.isDecimal {
				valExpr = t.ensureDecimal(v.Value, valExpr)
			}
			parts = append(parts, fmt.Sprintf("%svar %s %s = %s", prefix, varName, goType, valExpr))
		} else {
			parts = append(parts, fmt.Sprintf("%svar %s %s", prefix, varName, goType))
		}
	}

	return strings.Join(parts, "\n"+t.indentStr()), nil
}

func (t *transpiler) transpileSet(set *ast.SetStatement) (string, error) {
	// Handle SET options like NOCOUNT
	if set.Option != "" {
		// Ignore SET options - they're SQL Server specific
		return fmt.Sprintf("// SET %s %s (ignored)", set.Option, set.OnOff), nil
	}

	varExpr, err := t.transpileExpression(set.Variable)
	if err != nil {
		return "", err
	}

	// Look up comments for this SET statement
	var prefix string
	if v, ok := set.Variable.(*ast.Variable); ok {
		sig := "SET:" + strings.ToLower(strings.TrimPrefix(v.Name, "@"))
		if comments := t.comments.lookup(sig); len(comments) > 0 {
			for _, c := range comments {
				prefix += "// " + c + "\n" + t.indentStr()
			}
		}
	}

	if set.Value == nil {
		// Method call like @xml.modify(...)
		return prefix + varExpr, nil
	}

	valExpr, err := t.transpileExpression(set.Value)
	if err != nil {
		return "", err
	}

	// Check if we need to convert the value to match the variable's type
	varType := t.inferType(set.Variable)
	if varType.isDecimal {
		valExpr = t.ensureDecimal(set.Value, valExpr)
	}

	return fmt.Sprintf("%s%s = %s", prefix, varExpr, valExpr), nil
}

func (t *transpiler) transpileIf(ifStmt *ast.IfStatement) (string, error) {
	var out strings.Builder

	cond, err := t.transpileExpression(ifStmt.Condition)
	if err != nil {
		return "", err
	}

	// Look up comments for this IF statement
	sig := t.extractConditionSignature("IF", ifStmt.Condition)
	if comments := t.comments.lookup(sig); len(comments) > 0 {
		for _, c := range comments {
			out.WriteString("// " + c + "\n" + t.indentStr())
		}
	}

	out.WriteString(fmt.Sprintf("if %s {\n", cond))

	t.indent++
	conseq, err := t.transpileStatementBlock(ifStmt.Consequence)
	if err != nil {
		return "", err
	}
	out.WriteString(conseq)
	t.indent--

	if ifStmt.Alternative != nil {
		// Check if Alternative is another IF (ELSE IF chain)
		if elseIf, ok := ifStmt.Alternative.(*ast.IfStatement); ok {
			out.WriteString(t.indentStr())
			out.WriteString("} else ")
			// Recursively transpile without adding indent - it will add its own "if"
			elseIfCode, err := t.transpileIf(elseIf)
			if err != nil {
				return "", err
			}
			out.WriteString(elseIfCode)
			// Don't add closing brace - the recursive call handles it
			return out.String(), nil
		}

		// Regular ELSE block
		out.WriteString(t.indentStr())
		out.WriteString("} else {\n")
		t.indent++
		alt, err := t.transpileStatementBlock(ifStmt.Alternative)
		if err != nil {
			return "", err
		}
		out.WriteString(alt)
		t.indent--
	}

	out.WriteString(t.indentStr())
	out.WriteString("}")

	return out.String(), nil
}

// extractConditionSignature extracts an identifier from a condition for comment lookup.
func (t *transpiler) extractConditionSignature(prefix string, cond ast.Expression) string {
	switch e := cond.(type) {
	case *ast.Variable:
		return prefix + ":" + strings.ToLower(strings.TrimPrefix(e.Name, "@"))
	case *ast.Identifier:
		return prefix + ":" + strings.ToLower(e.Value)
	case *ast.InfixExpression:
		// Try left side first
		if sig := t.extractConditionSignature(prefix, e.Left); sig != prefix+":" && sig != prefix {
			return sig
		}
		return t.extractConditionSignature(prefix, e.Right)
	case *ast.PrefixExpression:
		return t.extractConditionSignature(prefix, e.Right)
	}
	return prefix
}

// transpileStatementBlock handles a statement that may be a block or a single statement.
func (t *transpiler) transpileStatementBlock(stmt ast.Statement) (string, error) {
	var out strings.Builder

	// If it's a BEGIN/END block, transpile each statement
	if block, ok := stmt.(*ast.BeginEndBlock); ok {
		for _, s := range block.Statements {
			code, err := t.transpileStatement(s)
			if err != nil {
				return "", err
			}
			if code != "" {
				out.WriteString(t.indentStr())
				out.WriteString(code)
				out.WriteString("\n")
			}
		}
		return out.String(), nil
	}

	// Single statement
	code, err := t.transpileStatement(stmt)
	if err != nil {
		return "", err
	}
	if code != "" {
		out.WriteString(t.indentStr())
		out.WriteString(code)
		out.WriteString("\n")
	}
	return out.String(), nil
}

func (t *transpiler) transpileWhile(whileStmt *ast.WhileStatement) (string, error) {
	var out strings.Builder

	cond, err := t.transpileExpression(whileStmt.Condition)
	if err != nil {
		return "", err
	}

	// Look up comments for this WHILE statement
	sig := t.extractConditionSignature("WHILE", whileStmt.Condition)
	if comments := t.comments.lookup(sig); len(comments) > 0 {
		for _, c := range comments {
			out.WriteString("// " + c + "\n" + t.indentStr())
		}
	}

	out.WriteString(fmt.Sprintf("for %s {\n", cond))

	t.indent++
	body, err := t.transpileStatementBlock(whileStmt.Body)
	if err != nil {
		return "", err
	}
	out.WriteString(body)
	t.indent--

	out.WriteString(t.indentStr())
	out.WriteString("}")

	return out.String(), nil
}

func (t *transpiler) transpileBlock(block *ast.BeginEndBlock) (string, error) {
	var parts []string

	for _, stmt := range block.Statements {
		s, err := t.transpileStatement(stmt)
		if err != nil {
			return "", err
		}
		if s != "" {
			parts = append(parts, s)
		}
	}

	return strings.Join(parts, "\n"+t.indentStr()), nil
}

func (t *transpiler) transpileTryCatch(tc *ast.TryCatchStatement) (string, error) {
	var out strings.Builder

	// Use an IIFE with defer/recover to simulate TRY/CATCH
	out.WriteString("func() {\n")
	t.indent++

	out.WriteString(t.indentStr())
	out.WriteString("defer func() {\n")
	t.indent++
	out.WriteString(t.indentStr())
	out.WriteString("if _recovered := recover(); _recovered != nil {\n")
	t.indent++

	// CATCH block - _recovered contains the panic value if needed
	if tc.CatchBlock != nil {
		for _, stmt := range tc.CatchBlock.Statements {
			s, err := t.transpileStatement(stmt)
			if err != nil {
				return "", err
			}
			if s != "" {
				out.WriteString(t.indentStr())
				out.WriteString(s)
				out.WriteString("\n")
			}
		}
	}

	t.indent--
	out.WriteString(t.indentStr())
	out.WriteString("}\n")
	t.indent--
	out.WriteString(t.indentStr())
	out.WriteString("}()\n")

	// TRY block
	if tc.TryBlock != nil {
		for _, stmt := range tc.TryBlock.Statements {
			s, err := t.transpileStatement(stmt)
			if err != nil {
				return "", err
			}
			if s != "" {
				out.WriteString(t.indentStr())
				out.WriteString(s)
				out.WriteString("\n")
			}
		}
	}

	t.indent--
	out.WriteString(t.indentStr())
	out.WriteString("}()")

	return out.String(), nil
}

func (t *transpiler) transpileReturn(ret *ast.ReturnStatement) (string, error) {
	// If we have output params or return code tracking, use buildReturnStatement
	if len(t.outputParams) > 0 || t.hasReturnCode {
		return t.buildReturnStatement(ret.Value), nil
	}
	
	// Simple return
	if ret.Value != nil {
		val, err := t.transpileExpression(ret.Value)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("return %s", val), nil
	}
	return "return", nil
}

func (t *transpiler) transpilePrint(print *ast.PrintStatement) (string, error) {
	t.imports["fmt"] = true

	expr, err := t.transpileExpression(print.Expression)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("fmt.Println(%s)", expr), nil
}

func (t *transpiler) indentStr() string {
	return strings.Repeat("\t", t.indent)
}
