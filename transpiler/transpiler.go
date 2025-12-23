// Package transpiler converts T-SQL source code to Go.
package transpiler

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/ha1tch/tsqlparser"
	"github.com/ha1tch/tsqlparser/ast"
)

// Transpile converts T-SQL source code to Go source code.
// This version only handles procedural code (no DML statements).
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

// TranspileWithDML converts T-SQL source code to Go, including DML statements.
// DML statements (SELECT, INSERT, UPDATE, DELETE, EXEC) are converted to
// the appropriate backend calls based on the DMLConfig.
func TranspileWithDML(source string, packageName string, dmlConfig DMLConfig) (string, error) {
	program, errors := tsqlparser.Parse(source)
	if len(errors) > 0 {
		return "", fmt.Errorf("parse errors:\n%s", strings.Join(errors, "\n"))
	}

	t := newTranspiler()
	t.packageName = packageName
	t.comments = buildCommentIndex(source)
	t.dmlConfig = dmlConfig
	t.dmlEnabled = true
	return t.transpile(program)
}

type transpiler struct {
	imports       map[string]bool
	output        strings.Builder
	indent        int
	inProcBody    bool
	inTryBlock    bool   // Track if we're inside a TRY block (anonymous function)
	inCatchBlock  bool   // Track if we're inside a CATCH block
	currentProcName string // Current procedure name for ERROR_PROCEDURE()
	symbols       *symbolTable
	outputParams  []*ast.ParameterDef
	hasReturnCode bool
	packageName   string
	comments      *commentIndex
	
	// DML handling
	dmlEnabled      bool
	dmlConfig       DMLConfig
	inTransaction   bool // Track if we're inside a transaction block
	hasDMLStatements bool // Track if procedure has DML requiring error return
	
	// Cursor handling
	cursors       map[string]*cursorInfo // name -> cursor info
	activeCursor  string                 // currently open cursor (for FETCH detection)
}

// cursorInfo tracks declared cursors for conversion to rows iteration
type cursorInfo struct {
	name       string
	query      *ast.SelectStatement
	fetchVars  []*ast.Variable // Variables from FETCH INTO
	rowsVar    string          // Generated Go variable name for rows
	isOpen     bool
}

func newTranspiler() *transpiler {
	return &transpiler{
		imports:   make(map[string]bool),
		symbols:   newSymbolTable(),
		dmlConfig: DefaultDMLConfig(),
		cursors:   make(map[string]*cursorInfo),
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

	// Generate SPLogger initialization if requested
	if t.dmlEnabled && t.dmlConfig.UseSPLogger && t.dmlConfig.GenLoggerInit {
		initCode := t.generateSPLoggerInit()
		if initCode != "" {
			out.WriteString(initCode)
			out.WriteString("\n\n")
		}
	}

	out.WriteString(strings.Join(bodies, "\n\n"))
	out.WriteString("\n")

	return out.String(), nil
}

// generateSPLoggerInit generates initialization code for SPLogger based on config
func (t *transpiler) generateSPLoggerInit() string {
	var out strings.Builder

	out.WriteString("// SPLogger initialization - configure based on your environment\n")
	out.WriteString("var " + t.dmlConfig.SPLoggerVar + " tsqlruntime.SPLogger\n\n")
	out.WriteString("func init() {\n")

	switch t.dmlConfig.SPLoggerType {
	case "db":
		out.WriteString("\t// Database logger - logs to a table like the original T-SQL pattern\n")
		out.WriteString("\t// Requires: db *sql.DB to be initialised\n")
		out.WriteString(fmt.Sprintf("\t// %s = tsqlruntime.NewDatabaseSPLogger(db, %q, %q)\n",
			t.dmlConfig.SPLoggerVar, t.dmlConfig.SPLoggerTable, t.dmlConfig.SQLDialect))
		out.WriteString("\t\n")
		out.WriteString("\t// For now, use slog as fallback\n")
		out.WriteString(fmt.Sprintf("\t%s = tsqlruntime.NewSlogSPLogger(nil)\n", t.dmlConfig.SPLoggerVar))

	case "file":
		if t.dmlConfig.SPLoggerFile != "" {
			out.WriteString(fmt.Sprintf("\t// File logger - logs to %s\n", t.dmlConfig.SPLoggerFile))
			out.WriteString(fmt.Sprintf("\tvar err error\n"))
			out.WriteString(fmt.Sprintf("\t%s, err = tsqlruntime.NewFileSPLogger(%q, %q)\n",
				t.dmlConfig.SPLoggerVar, t.dmlConfig.SPLoggerFile, t.dmlConfig.SPLoggerFormat))
			out.WriteString("\tif err != nil {\n")
			out.WriteString(fmt.Sprintf("\t\t%s = tsqlruntime.NewSlogSPLogger(nil) // Fallback to slog\n", t.dmlConfig.SPLoggerVar))
			out.WriteString("\t}\n")
		} else {
			out.WriteString("\t// File logger - specify path with --logger-file\n")
			out.WriteString(fmt.Sprintf("\t%s = tsqlruntime.NewSlogSPLogger(nil)\n", t.dmlConfig.SPLoggerVar))
		}

	case "multi":
		out.WriteString("\t// Multi logger - logs to multiple destinations\n")
		out.WriteString("\t// Customise the loggers as needed:\n")
		out.WriteString("\tslogLogger := tsqlruntime.NewSlogSPLogger(nil)\n")
		out.WriteString("\t// dbLogger := tsqlruntime.NewDatabaseSPLogger(db, \"Error.Log\", \"postgres\")\n")
		out.WriteString("\t// fileLogger, _ := tsqlruntime.NewFileSPLogger(\"/var/log/sp.json\", \"json\")\n")
		out.WriteString(fmt.Sprintf("\t%s = tsqlruntime.NewMultiSPLogger(slogLogger)\n", t.dmlConfig.SPLoggerVar))

	case "nop":
		out.WriteString("\t// No-op logger - discards all logs (for testing)\n")
		out.WriteString(fmt.Sprintf("\t%s = tsqlruntime.NewNopSPLogger()\n", t.dmlConfig.SPLoggerVar))

	default: // "slog" or anything else
		out.WriteString("\t// Slog logger - uses Go's structured logging\n")
		out.WriteString(fmt.Sprintf("\t%s = tsqlruntime.NewSlogSPLogger(nil) // Uses slog.Default()\n", t.dmlConfig.SPLoggerVar))
	}

	out.WriteString("}\n")

	return out.String()
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
	
	// DML statements - only handled if DML is enabled
	case *ast.SelectStatement:
		if t.dmlEnabled {
			return t.transpileSelect(s)
		}
		return "", fmt.Errorf("SELECT statements require DML mode (use TranspileWithDML)")
	case *ast.InsertStatement:
		if t.dmlEnabled {
			return t.transpileInsert(s)
		}
		return "", fmt.Errorf("INSERT statements require DML mode (use TranspileWithDML)")
	case *ast.UpdateStatement:
		if t.dmlEnabled {
			return t.transpileUpdate(s)
		}
		return "", fmt.Errorf("UPDATE statements require DML mode (use TranspileWithDML)")
	case *ast.DeleteStatement:
		if t.dmlEnabled {
			return t.transpileDelete(s)
		}
		return "", fmt.Errorf("DELETE statements require DML mode (use TranspileWithDML)")
	case *ast.ExecStatement:
		if t.dmlEnabled {
			return t.transpileExec(s)
		}
		return "", fmt.Errorf("EXEC statements require DML mode (use TranspileWithDML)")
	
	// Transaction statements
	case *ast.BeginTransactionStatement:
		if t.dmlEnabled {
			return t.transpileBeginTransaction(s)
		}
		return "", fmt.Errorf("BEGIN TRANSACTION requires DML mode (use TranspileWithDML)")
	case *ast.CommitTransactionStatement:
		if t.dmlEnabled {
			return t.transpileCommitTransaction(s)
		}
		return "", fmt.Errorf("COMMIT TRANSACTION requires DML mode (use TranspileWithDML)")
	case *ast.RollbackTransactionStatement:
		if t.dmlEnabled {
			return t.transpileRollbackTransaction(s)
		}
		return "", fmt.Errorf("ROLLBACK TRANSACTION requires DML mode (use TranspileWithDML)")
	
	// DDL statements for temp tables
	case *ast.CreateTableStatement:
		if t.dmlEnabled {
			return t.transpileCreateTable(s)
		}
		return "", fmt.Errorf("CREATE TABLE requires DML mode (use TranspileWithDML)")
	case *ast.DropTableStatement:
		if t.dmlEnabled {
			return t.transpileDropTable(s)
		}
		return "", fmt.Errorf("DROP TABLE requires DML mode (use TranspileWithDML)")
	case *ast.TruncateTableStatement:
		if t.dmlEnabled {
			return t.transpileTruncateTable(s)
		}
		return "", fmt.Errorf("TRUNCATE TABLE requires DML mode (use TranspileWithDML)")
	
	// Cursor statements
	case *ast.DeclareCursorStatement:
		if t.dmlEnabled {
			return t.transpileDeclareCursor(s)
		}
		return "", fmt.Errorf("DECLARE CURSOR requires DML mode (use TranspileWithDML)")
	case *ast.OpenCursorStatement:
		if t.dmlEnabled {
			return t.transpileOpenCursor(s)
		}
		return "", fmt.Errorf("OPEN cursor requires DML mode (use TranspileWithDML)")
	case *ast.FetchStatement:
		if t.dmlEnabled {
			return t.transpileFetch(s)
		}
		return "", fmt.Errorf("FETCH requires DML mode (use TranspileWithDML)")
	case *ast.CloseCursorStatement:
		if t.dmlEnabled {
			return t.transpileCloseCursor(s)
		}
		return "", fmt.Errorf("CLOSE cursor requires DML mode (use TranspileWithDML)")
	case *ast.DeallocateCursorStatement:
		if t.dmlEnabled {
			return t.transpileDeallocateCursor(s)
		}
		return "", fmt.Errorf("DEALLOCATE cursor requires DML mode (use TranspileWithDML)")
	
	// Error handling statements
	case *ast.RaiserrorStatement:
		return t.transpileRaiserror(s)
	case *ast.ThrowStatement:
		return t.transpileThrow(s)
	
	// CTE (Common Table Expression) statements
	case *ast.WithStatement:
		if t.dmlEnabled {
			return t.transpileWithStatement(s)
		}
		return "", fmt.Errorf("WITH/CTE statements require DML mode (use TranspileWithDML)")
	
	default:
		return "", fmt.Errorf("unsupported statement type: %T", stmt)
	}
}

func (t *transpiler) transpileCreateProcedure(proc *ast.CreateProcedureStatement) (string, error) {
	var out strings.Builder

	// Reset symbol table for new procedure scope
	t.symbols = newSymbolTable()
	
	// Reset DML tracking
	t.hasDMLStatements = false

	// Pre-scan for DML statements if DML mode is enabled
	if t.dmlEnabled && proc.Body != nil {
		t.hasDMLStatements = t.blockHasDML(proc.Body)
	}

	// Get procedure name for comment lookup and ERROR_PROCEDURE()
	procName := proc.Name.Parts[len(proc.Name.Parts)-1].Value
	t.currentProcName = procName // Store for ERROR_PROCEDURE() in CATCH blocks
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
	funcName := goExportedIdentifier(procName)
	out.WriteString(fmt.Sprintf("func %s(", funcName))
	out.WriteString(strings.Join(inputParams, ", "))
	out.WriteString(")")

	// Return type(s)
	hasReturn := t.procedureHasReturn(proc)
	needsErrorReturn := t.hasDMLStatements
	
	if len(outputParams) > 0 || hasReturn || needsErrorReturn {
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
		if needsErrorReturn {
			returns = append(returns, "err error")
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
	if (len(outputParams) > 0 || hasReturn || needsErrorReturn) && !t.blockEndsWithReturn(proc.Body) {
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

// blockHasDML checks if a statement block contains DML statements
func (t *transpiler) blockHasDML(block *ast.BeginEndBlock) bool {
	if block == nil {
		return false
	}
	for _, stmt := range block.Statements {
		if t.statementHasDML(stmt) {
			return true
		}
	}
	return false
}

// statementHasDML checks if a statement is or contains DML
func (t *transpiler) statementHasDML(stmt ast.Statement) bool {
	switch s := stmt.(type) {
	case *ast.SelectStatement, *ast.InsertStatement, *ast.UpdateStatement, *ast.DeleteStatement:
		return true
	case *ast.CreateTableStatement, *ast.DropTableStatement, *ast.TruncateTableStatement:
		return true
	case *ast.ExecStatement:
		return true
	case *ast.BeginEndBlock:
		return t.blockHasDML(s)
	case *ast.IfStatement:
		if t.statementHasDML(s.Consequence) {
			return true
		}
		if s.Alternative != nil && t.statementHasDML(s.Alternative) {
			return true
		}
		return false
	case *ast.WhileStatement:
		return t.statementHasDML(s.Body)
	case *ast.TryCatchStatement:
		if s.TryBlock != nil && t.blockHasDML(s.TryBlock) {
			return true
		}
		if s.CatchBlock != nil && t.blockHasDML(s.CatchBlock) {
			return true
		}
		return false
	default:
		return false
	}
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
	
	// Add nil error if DML mode with error return
	if t.hasDMLStatements {
		parts = append(parts, "nil")
	}
	
	if len(parts) == 0 {
		return "return"
	}
	return "return " + strings.Join(parts, ", ")
}

// zeroValueForType returns the Go zero value for a given type.
// Used when transpiling NULL assignments to value types.
func (t *transpiler) zeroValueForType(ti *typeInfo) string {
	if ti == nil {
		return "nil"
	}
	switch ti.goType {
	case "int32", "int16", "int64", "uint8", "int":
		return "0"
	case "float64", "float32":
		return "0.0"
	case "string":
		return `""`
	case "bool":
		return "false"
	case "time.Time":
		t.imports["time"] = true
		return "time.Time{}"
	case "decimal.Decimal":
		t.imports["github.com/shopspring/decimal"] = true
		return "decimal.Zero"
	default:
		return "nil"
	}
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
			// Check if we need to convert the initialiser to match the variable's type
			ti := t.symbols.lookup(varName)

			// Handle NULL initialisation for value types
			_, isNull := v.Value.(*ast.NullLiteral)
			if isNull && ti != nil {
				valExpr = t.zeroValueForType(ti)
			}

			// Only call ensureDecimal/ensureBool if we didn't already handle NULL
			if ti != nil && ti.isDecimal && !isNull {
				valExpr = t.ensureDecimal(v.Value, valExpr)
			}
			if ti != nil && ti.isBool && !isNull {
				valExpr = t.ensureBool(v.Value, valExpr)
			}
			parts = append(parts, fmt.Sprintf("%svar %s %s = %s", prefix, varName, goType, valExpr))
		} else {
			parts = append(parts, fmt.Sprintf("%svar %s %s", prefix, varName, goType))
		}

		// Add blank assignment to prevent "declared and not used" errors
		// This is a common Go idiom for variables that may not be used in all code paths
		parts = append(parts, fmt.Sprintf("_ = %s", varName))
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

	// Check if value is a subquery (SET @var = (SELECT ...))
	if subq, ok := set.Value.(*ast.SubqueryExpression); ok && t.dmlEnabled {
		return t.transpileSetSubquery(set.Variable, subq, prefix)
	}

	valExpr, err := t.transpileExpression(set.Value)
	if err != nil {
		return "", err
	}

	// Check if we need to convert the value to match the variable's type
	varType := t.inferType(set.Variable)

	// Handle NULL assignment to value types (which can't be nil in Go)
	_, isNull := set.Value.(*ast.NullLiteral)
	if isNull {
		valExpr = t.zeroValueForType(varType)
	}

	// Only call ensureDecimal/ensureBool if we didn't already handle NULL
	if varType.isDecimal && !isNull {
		valExpr = t.ensureDecimal(set.Value, valExpr)
	}
	if varType.isBool && !isNull {
		valExpr = t.ensureBool(set.Value, valExpr)
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
	// Check for WHILE @@FETCH_STATUS = 0 cursor pattern
	if t.dmlEnabled && t.isFetchStatusCheck(whileStmt.Condition) {
		return t.transpileCursorWhile(whileStmt)
	}
	
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
	// Set inCatchBlock so we can handle ERROR_* functions and XML building specially
	wasInCatchBlock := t.inCatchBlock
	t.inCatchBlock = true

	// If SPLogger is enabled, generate a CaptureError call at the start
	if t.dmlEnabled && t.dmlConfig.UseSPLogger {
		t.imports["github.com/ha1tch/tgpiler/tsqlruntime"] = true
		out.WriteString(t.indentStr())
		out.WriteString(fmt.Sprintf("_spErr := tsqlruntime.CaptureError(%q, _recovered, %s)\n",
			t.currentProcName, t.buildParamsMap()))
	}

	if tc.CatchBlock != nil {
		for _, stmt := range tc.CatchBlock.Statements {
			// Check if SPLogger is enabled and we should skip/replace certain statements
			if t.dmlEnabled && t.dmlConfig.UseSPLogger {
				// Skip DECLARE statements that build XML parameters
				if decl, ok := stmt.(*ast.DeclareStatement); ok && t.isXMLParameterDeclare(decl) {
					continue
				}
				
				// Replace error logging INSERT with SPLogger call
				if insert, ok := stmt.(*ast.InsertStatement); ok && t.isErrorLoggingInsert(insert) {
					out.WriteString(t.indentStr())
					out.WriteString(fmt.Sprintf("_ = %s.LogError(ctx, _spErr)\n", t.dmlConfig.SPLoggerVar))
					continue
				}
			}

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
	t.inCatchBlock = wasInCatchBlock

	t.indent--
	out.WriteString(t.indentStr())
	out.WriteString("}\n")
	t.indent--
	out.WriteString(t.indentStr())
	out.WriteString("}()\n")

	// TRY block - set flag to handle RETURN statements correctly
	wasInTryBlock := t.inTryBlock
	t.inTryBlock = true
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
	t.inTryBlock = wasInTryBlock

	t.indent--
	out.WriteString(t.indentStr())
	out.WriteString("}()")

	return out.String(), nil
}

// buildParamsMap builds a Go map literal of procedure parameters for SPLogger
func (t *transpiler) buildParamsMap() string {
	if len(t.outputParams) == 0 && len(t.symbols.variables) == 0 {
		return "nil"
	}

	var parts []string

	// Add input parameters from symbol table (excluding output params)
	for name := range t.symbols.variables {
		// Skip internal variables
		if strings.HasPrefix(name, "_") {
			continue
		}
		parts = append(parts, fmt.Sprintf("%q: %s", name, name))
	}

	if len(parts) == 0 {
		return "nil"
	}

	return "map[string]interface{}{" + strings.Join(parts, ", ") + "}"
}

// isErrorLoggingInsert checks if an INSERT is an error logging pattern
func (t *transpiler) isErrorLoggingInsert(ins *ast.InsertStatement) bool {
	if ins.Table == nil {
		return false
	}

	tableName := strings.ToLower(ins.Table.String())

	// Common error log table patterns
	errorTablePatterns := []string{
		"error",
		"errorlog",
		"error_log",
		"logerror",
		"log_error",
		"logforstorep",
	}

	for _, pattern := range errorTablePatterns {
		if strings.Contains(tableName, pattern) {
			return true
		}
	}

	return false
}

// isXMLParameterDeclare checks if a DECLARE is for XML parameter building in CATCH
func (t *transpiler) isXMLParameterDeclare(decl *ast.DeclareStatement) bool {
	if decl == nil || len(decl.Variables) == 0 {
		return false
	}

	for _, v := range decl.Variables {
		// Check if variable name suggests parameters
		varName := strings.ToLower(v.Name)
		if strings.Contains(varName, "param") || strings.Contains(varName, "xml") {
			return true
		}

		// Check if it has a subquery with FOR XML
		if subq, ok := v.Value.(*ast.SubqueryExpression); ok {
			if subq.Subquery != nil && subq.Subquery.ForClause != nil {
				if strings.ToUpper(subq.Subquery.ForClause.ForType) == "XML" {
					return true
				}
			}
		}
	}

	return false
}

func (t *transpiler) transpileReturn(ret *ast.ReturnStatement) (string, error) {
	// Inside a TRY block (anonymous function), just return to exit the IIFE
	// The actual return values are set via named return parameters
	if t.inTryBlock {
		return "return", nil
	}

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

// Transaction support

func (t *transpiler) transpileBeginTransaction(s *ast.BeginTransactionStatement) (string, error) {
	t.inTransaction = true
	
	var out strings.Builder
	out.WriteString("// BEGIN TRANSACTION\n")
	out.WriteString(t.indentStr())
	out.WriteString(fmt.Sprintf("tx, err := %s.BeginTx(ctx, nil)\n", t.dmlConfig.StoreVar))
	out.WriteString(t.indentStr())
	out.WriteString("if err != nil {\n")
	out.WriteString(t.indentStr())
	out.WriteString("\treturn err\n")
	out.WriteString(t.indentStr())
	out.WriteString("}\n")
	out.WriteString(t.indentStr())
	out.WriteString("defer func() {\n")
	out.WriteString(t.indentStr())
	out.WriteString("\tif p := recover(); p != nil {\n")
	out.WriteString(t.indentStr())
	out.WriteString("\t\ttx.Rollback()\n")
	out.WriteString(t.indentStr())
	out.WriteString("\t\tpanic(p)\n")
	out.WriteString(t.indentStr())
	out.WriteString("\t}\n")
	out.WriteString(t.indentStr())
	out.WriteString("}()")
	
	return out.String(), nil
}

func (t *transpiler) transpileCommitTransaction(s *ast.CommitTransactionStatement) (string, error) {
	t.inTransaction = false
	
	var out strings.Builder
	out.WriteString("// COMMIT TRANSACTION\n")
	out.WriteString(t.indentStr())
	out.WriteString("if err := tx.Commit(); err != nil {\n")
	out.WriteString(t.indentStr())
	out.WriteString("\treturn err\n")
	out.WriteString(t.indentStr())
	out.WriteString("}")
	
	return out.String(), nil
}

func (t *transpiler) transpileRollbackTransaction(s *ast.RollbackTransactionStatement) (string, error) {
	t.inTransaction = false
	
	var out strings.Builder
	out.WriteString("// ROLLBACK TRANSACTION\n")
	out.WriteString(t.indentStr())
	out.WriteString("tx.Rollback()")
	
	return out.String(), nil
}

func (t *transpiler) indentStr() string {
	return strings.Repeat("\t", t.indent)
}

// transpileRaiserror converts RAISERROR to Go error handling
func (t *transpiler) transpileRaiserror(s *ast.RaiserrorStatement) (string, error) {
	t.imports["fmt"] = true
	
	var out strings.Builder
	
	// Get the message
	msg, err := t.transpileExpression(s.Message)
	if err != nil {
		return "", err
	}
	
	// Build error expression
	var errExpr string
	if len(s.Args) > 0 {
		var args []string
		for _, arg := range s.Args {
			a, err := t.transpileExpression(arg)
			if err != nil {
				return "", err
			}
			args = append(args, a)
		}
		errExpr = "fmt.Errorf(" + msg
		for _, a := range args {
			errExpr += ", " + a
		}
		errExpr += ")"
	} else {
		errExpr = "fmt.Errorf(" + msg + ")"
	}
	
	// Build return statement with all output params
	var parts []string
	for _, p := range t.outputParams {
		paramName := goIdentifier(strings.TrimPrefix(p.Name, "@"))
		parts = append(parts, paramName)
	}
	if t.hasReturnCode {
		parts = append(parts, "0")
	}
	parts = append(parts, errExpr)
	
	out.WriteString("return " + strings.Join(parts, ", "))
	
	return out.String(), nil
}

// transpileThrow converts THROW to Go error handling
func (t *transpiler) transpileThrow(s *ast.ThrowStatement) (string, error) {
	t.imports["fmt"] = true
	
	var out strings.Builder
	
	if s.ErrorNum == nil && s.Message == nil {
		// THROW with no arguments - rethrow current error
		out.WriteString("return err // THROW (rethrow)")
	} else {
		// THROW with arguments
		msg := "\"unknown error\""
		if s.Message != nil {
			var err error
			msg, err = t.transpileExpression(s.Message)
			if err != nil {
				return "", err
			}
		}
		
		errNum := "50000"
		if s.ErrorNum != nil {
			var err error
			errNum, err = t.transpileExpression(s.ErrorNum)
			if err != nil {
				return "", err
			}
		}
		
		out.WriteString(fmt.Sprintf("return fmt.Errorf(\"error %%d: %%s\", %s, %s)", errNum, msg))
	}
	
	return out.String(), nil
}

// transpileSetSubquery handles SET @var = (SELECT ...) assignments
func (t *transpiler) transpileSetSubquery(variable ast.Expression, subq *ast.SubqueryExpression, prefix string) (string, error) {
	t.imports["database/sql"] = true
	
	varExpr, err := t.transpileExpression(variable)
	if err != nil {
		return "", err
	}
	
	// Get variable type for proper scanning
	varType := t.inferType(variable)
	
	// Build the SQL query from the subquery
	// We need to convert the SELECT statement to a SQL string
	sql := subq.Subquery.String()
	
	var out strings.Builder
	out.WriteString(prefix)
	out.WriteString("// SET from subquery\n")
	out.WriteString(t.indentStr())
	
	// For scalar subqueries, use QueryRowContext and Scan
	out.WriteString(fmt.Sprintf("if err := %s.QueryRowContext(ctx, %q).Scan(&%s); err != nil {\n", 
		t.dmlConfig.StoreVar, sql, varExpr))
	out.WriteString(t.indentStr())
	out.WriteString("\tif err != sql.ErrNoRows {\n")
	out.WriteString(t.indentStr())
	out.WriteString("\t\t")
	out.WriteString(t.buildSubqueryErrorReturn())
	out.WriteString("\n")
	out.WriteString(t.indentStr())
	out.WriteString("\t}\n")
	
	// Set zero value if no rows
	out.WriteString(t.indentStr())
	out.WriteString(fmt.Sprintf("\t%s = %s\n", varExpr, t.zeroValueForType(varType)))
	out.WriteString(t.indentStr())
	out.WriteString("}")
	
	return out.String(), nil
}

// buildSubqueryErrorReturn generates an error return appropriate for the current function
func (t *transpiler) buildSubqueryErrorReturn() string {
	var parts []string
	
	// Add output params
	for _, p := range t.outputParams {
		paramName := goIdentifier(strings.TrimPrefix(p.Name, "@"))
		parts = append(parts, paramName)
	}
	
	// Add return code if present
	if t.hasReturnCode {
		parts = append(parts, "0")
	}
	
	// Add error
	parts = append(parts, "err")
	
	if len(parts) == 1 {
		return "return err"
	}
	return "return " + strings.Join(parts, ", ")
}

// transpileSubqueryExpression handles subqueries used as expressions (not in SET context)
func (t *transpiler) transpileSubqueryExpression(subq *ast.SubqueryExpression) (string, error) {
	sql := subq.Subquery.String()
	
	// Check if this is a FOR XML query in a CATCH block (error logging pattern)
	isForXML := strings.Contains(strings.ToUpper(sql), "FOR XML")
	
	if t.inCatchBlock && isForXML {
		// In CATCH context with FOR XML, build XML in Go instead of querying DB
		// This is safer because the DB might be the source of the error
		return t.transpileErrorLoggingXML(subq.Subquery)
	}
	
	// Standard subquery handling - substitute variables
	sql = stripTableHints(sql)
	substitutedSQL, args := t.substituteVariablesForExists(sql)
	
	var argsStr string
	if len(args) > 0 {
		argsStr = ", " + strings.Join(args, ", ")
	}
	
	// Generate an anonymous function that executes and returns the result
	return fmt.Sprintf("func() interface{} {\n"+
		"\t\tvar result interface{}\n"+
		"\t\t_ = %s.QueryRowContext(ctx, %q%s).Scan(&result)\n"+
		"\t\treturn result\n"+
		"\t}()", t.dmlConfig.StoreVar, substitutedSQL, argsStr), nil
}

// transpileErrorLoggingXML handles SELECT ... FOR XML in CATCH blocks
// Instead of querying the database, we build XML in Go
func (t *transpiler) transpileErrorLoggingXML(sel *ast.SelectStatement) (string, error) {
	t.imports["fmt"] = true
	
	// Extract column aliases and their source expressions
	var xmlParts []string
	var args []string
	
	if sel.Columns != nil {
		for _, col := range sel.Columns {
			alias := ""
			if col.Alias != nil {
				alias = col.Alias.Value
			} else if id, ok := col.Expression.(*ast.Identifier); ok {
				alias = id.Value
			} else {
				alias = "value"
			}
			
			// Extract the variable from the expression
			// Pattern: ISNULL(CONVERT(VARCHAR(MAX), @VarName), '--NULL--')
			varName := t.extractVariableFromExpression(col.Expression)
			if varName != "" {
				xmlParts = append(xmlParts, fmt.Sprintf("<%s>%%v</%s>", alias, alias))
				args = append(args, goIdentifier(varName))
			} else {
				// Can't extract variable, use placeholder
				xmlParts = append(xmlParts, fmt.Sprintf("<%s>%%v</%s>", alias, alias))
				// Try to transpile the expression
				exprStr, err := t.transpileExpression(col.Expression)
				if err != nil {
					args = append(args, "\"\"")
				} else {
					args = append(args, exprStr)
				}
			}
		}
	}
	
	// Get the root element name from FOR XML PATH
	rootElement := "RootXml"
	if sel.ForClause != nil && sel.ForClause.ElementName != "" {
		rootElement = strings.Trim(sel.ForClause.ElementName, "'\"")
	} else if sel.ForClause != nil && sel.ForClause.Root != "" {
		rootElement = strings.Trim(sel.ForClause.Root, "'\"")
	}
	
	// Build the format string
	xmlFormat := fmt.Sprintf("<%s>%s</%s>", rootElement, strings.Join(xmlParts, ""), rootElement)
	
	if len(args) > 0 {
		return fmt.Sprintf("fmt.Sprintf(`%s`, %s)", xmlFormat, strings.Join(args, ", ")), nil
	}
	return fmt.Sprintf("`%s`", xmlFormat), nil
}

// extractVariableFromExpression tries to find a @variable in an expression
// Handles patterns like ISNULL(CONVERT(type, @Var), default)
func (t *transpiler) extractVariableFromExpression(expr ast.Expression) string {
	if expr == nil {
		return ""
	}
	
	switch e := expr.(type) {
	case *ast.Variable:
		return strings.TrimPrefix(e.Name, "@")
	case *ast.FunctionCall:
		// Check arguments recursively (for ISNULL, CONVERT, etc.)
		for _, arg := range e.Arguments {
			if v := t.extractVariableFromExpression(arg); v != "" {
				return v
			}
		}
	case *ast.CastExpression:
		return t.extractVariableFromExpression(e.Expression)
	case *ast.ConvertExpression:
		return t.extractVariableFromExpression(e.Expression)
	}
	return ""
}

// transpileExistsExpression handles EXISTS(SELECT ...) expressions
func (t *transpiler) transpileExistsExpression(exists *ast.ExistsExpression) (string, error) {
	if exists.Subquery == nil {
		return "", fmt.Errorf("EXISTS expression has no subquery")
	}
	
	// Get the subquery SQL and substitute variables
	sql := exists.Subquery.String()
	
	// Strip table hints like (NOLOCK) that aren't supported by all databases
	sql = stripTableHints(sql)
	
	// Substitute variables in the query
	substitutedSQL, args := t.substituteVariablesForExists(sql)
	
	// Generate an inline function that checks if any rows exist
	// Uses COUNT with LIMIT 1 for portability across databases
	var argsStr string
	if len(args) > 0 {
		argsStr = ", " + strings.Join(args, ", ")
	}
	
	return fmt.Sprintf("func() bool {\n"+
		"\t\tvar exists int\n"+
		"\t\terr := %s.QueryRowContext(ctx, \"SELECT 1 WHERE EXISTS(%s)\"%s).Scan(&exists)\n"+
		"\t\treturn err == nil && exists == 1\n"+
		"\t}()", t.dmlConfig.StoreVar, substitutedSQL, argsStr), nil
}

// substituteVariablesForExists replaces @variables with placeholders and returns args
func (t *transpiler) substituteVariablesForExists(sql string) (string, []string) {
	var args []string
	paramIndex := 0
	
	result := make([]byte, 0, len(sql))
	i := 0
	for i < len(sql) {
		if sql[i] == '@' && i+1 < len(sql) && (isAlphaForCTE(sql[i+1]) || sql[i+1] == '@') {
			// Skip @@ system variables - leave them as-is for now
			if i+1 < len(sql) && sql[i+1] == '@' {
				result = append(result, sql[i], sql[i+1])
				i += 2
				for i < len(sql) && isAlphaNumForCTE(sql[i]) {
					result = append(result, sql[i])
					i++
				}
				continue
			}
			
			// Extract variable name
			start := i + 1
			j := start
			for j < len(sql) && isAlphaNumForCTE(sql[j]) {
				j++
			}
			varName := sql[start:j]
			
			// Add placeholder
			paramIndex++
			placeholder := getPlaceholderForDialect(t.dmlConfig.SQLDialect, paramIndex)
			result = append(result, placeholder...)
			
			// Add to args
			args = append(args, goIdentifier(varName))
			i = j
		} else {
			result = append(result, sql[i])
			i++
		}
	}
	
	return string(result), args
}

// getPlaceholderForDialect returns the appropriate placeholder for a given SQL dialect
func getPlaceholderForDialect(dialect string, n int) string {
	switch dialect {
	case "postgres":
		return fmt.Sprintf("$%d", n)
	case "sqlserver":
		return fmt.Sprintf("@p%d", n)
	case "oracle":
		return fmt.Sprintf(":p%d", n)
	default: // mysql, sqlite
		return "?"
	}
}

// stripTableHints removes SQL Server table hints like (NOLOCK), WITH (NOLOCK), WITH (NOLOCK, ROWLOCK), etc.
func stripTableHints(sql string) string {
	// Common table hints - these will be matched case-insensitively
	hintPattern := `(?i)\b(NOLOCK|READUNCOMMITTED|READCOMMITTED|REPEATABLEREAD|SERIALIZABLE|ROWLOCK|PAGLOCK|TABLOCK|TABLOCKX|UPDLOCK|XLOCK|HOLDLOCK|NOWAIT|READPAST)\b`
	
	// Pattern 1: WITH (hint) or WITH (hint1, hint2, ...)
	// Matches: WITH (NOLOCK), WITH (NOLOCK, ROWLOCK), WITH ( NOLOCK , ROWLOCK )
	withPattern := regexp.MustCompile(`(?i)\s*WITH\s*\(\s*` + hintPattern + `(\s*,\s*` + hintPattern + `)*\s*\)`)
	result := withPattern.ReplaceAllString(sql, "")
	
	// Pattern 2: Just (hint) or (hint1, hint2, ...) - legacy syntax without WITH
	// Need to be careful not to remove function calls or subqueries
	// We match (HINT) only when preceded by whitespace or identifier char (table name/alias)
	legacyPattern := regexp.MustCompile(`(?i)(\s)\(\s*` + hintPattern + `(\s*,\s*` + hintPattern + `)*\s*\)`)
	result = legacyPattern.ReplaceAllString(result, "$1")
	
	// Clean up any double spaces left behind
	for strings.Contains(result, "  ") {
		result = strings.ReplaceAll(result, "  ", " ")
	}
	
	return result
}

// replaceIgnoreCase performs case-insensitive string replacement
func replaceIgnoreCase(s, old, new string) string {
	lower := strings.ToLower(s)
	oldLower := strings.ToLower(old)
	
	var result strings.Builder
	i := 0
	for i < len(s) {
		idx := strings.Index(lower[i:], oldLower)
		if idx == -1 {
			result.WriteString(s[i:])
			break
		}
		result.WriteString(s[i : i+idx])
		result.WriteString(new)
		i = i + idx + len(old)
	}
	return result.String()
}