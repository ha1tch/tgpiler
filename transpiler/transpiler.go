// Package transpiler converts T-SQL source code to Go.
package transpiler

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/ha1tch/tsqlparser"
	"github.com/ha1tch/tsqlparser/ast"
)

// goStatementPattern matches GO batch separator lines.
// GO is a client tool directive (SSMS, sqlcmd), not T-SQL itself.
var goStatementPattern = regexp.MustCompile(`(?im)^\s*GO\s*$`)

// stripGoStatements removes GO batch separators from T-SQL source.
// GO has no semantic meaning for transpilation - it's a client tool artifact.
func stripGoStatements(source string) string {
	return goStatementPattern.ReplaceAllString(source, "")
}

// Transpile converts T-SQL source code to Go source code.
// This version only handles procedural code (no DML statements).
// GO statements are stripped by default as they have no semantic meaning.
func Transpile(source string, packageName string) (string, error) {
	source = stripGoStatements(source)
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
// GO statements are stripped by default unless PreserveGo is set in config.
func TranspileWithDML(source string, packageName string, dmlConfig DMLConfig) (string, error) {
	result, err := TranspileWithDMLEx(source, packageName, dmlConfig)
	if err != nil {
		return "", err
	}
	return result.Code, nil
}

// TranspileResult contains the transpilation output and metadata
type TranspileResult struct {
	Code              string   // Generated Go code
	DDLWarnings       []string // Warnings about skipped DDL statements
	ExtractedDDL      []string // DDL statements collected for extraction
	TempTablesUsed    []string // Temp tables encountered (for fallback backend info)
	TempTableWarnings []string // Warnings about temp tables with non-SQL backends
}

// TranspileWithDMLEx is like TranspileWithDML but returns extended results
func TranspileWithDMLEx(source string, packageName string, dmlConfig DMLConfig) (*TranspileResult, error) {
	if !dmlConfig.PreserveGo {
		source = stripGoStatements(source)
	}
	program, errors := tsqlparser.Parse(source)
	if len(errors) > 0 {
		return nil, fmt.Errorf("parse errors:\n%s", strings.Join(errors, "\n"))
	}

	t := newTranspiler()
	t.packageName = packageName
	t.comments = buildCommentIndex(source)
	t.dmlConfig = dmlConfig
	t.dmlEnabled = true
	t.annotateLevel = dmlConfig.AnnotateLevel
	
	code, err := t.transpile(program)
	if err != nil {
		return nil, err
	}
	
	// Generate temp table warnings if needed
	var tempTableWarnings []string
	if len(t.tempTablesUsed) > 0 && (dmlConfig.Backend == BackendGRPC || dmlConfig.Backend == BackendMock) {
		if !dmlConfig.FallbackExplicit {
			tempTableWarnings = append(tempTableWarnings,
				fmt.Sprintf("Temp tables detected (%s) with --%s backend. "+
					"Using --fallback-backend=%s (default). "+
					"Use --fallback-backend to specify explicitly.",
					strings.Join(t.tempTablesUsed, ", "),
					dmlConfig.Backend,
					dmlConfig.FallbackBackend))
		}
	}
	
	return &TranspileResult{
		Code:              code,
		DDLWarnings:       t.ddlWarnings,
		ExtractedDDL:      t.extractedDDL,
		TempTablesUsed:    t.tempTablesUsed,
		TempTableWarnings: tempTableWarnings,
	}, nil
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
	usesRowCount    bool // Track if procedure uses @@ROWCOUNT
	
	// Annotation level: none, minimal, standard, verbose
	annotateLevel string
	
	// Cursor handling
	cursors       map[string]*cursorInfo // name -> cursor info
	activeCursor  string                 // currently open cursor (for FETCH detection)
	
	// User-defined function tracking
	userFunctions map[string]*userFuncInfo // function name (lowercase) -> info
	
	// DDL handling
	ddlWarnings  []string // Collect DDL skip warnings
	extractedDDL []string // Collect DDL statements for extraction
	
	// Temp table tracking for fallback backend warnings
	tempTablesUsed []string // Names of temp tables encountered
	
	// Track if any procedures/functions were transpiled
	hasProcedures bool
}

// userFuncInfo tracks user-defined functions for call resolution
type userFuncInfo struct {
	name       string           // Original SQL function name
	goName     string           // Generated Go function name
	params     []*ast.ParameterDef
	returnType string           // Go return type
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
		imports:       make(map[string]bool),
		symbols:       newSymbolTable(),
		dmlConfig:     DefaultDMLConfig(),
		cursors:       make(map[string]*cursorInfo),
		userFunctions: make(map[string]*userFuncInfo),
		annotateLevel: "none",
	}
}

// Annotation level helpers
// Levels: none < minimal < standard < verbose

// emitTODOs returns true if TODO markers should be emitted (minimal+)
func (t *transpiler) emitTODOs() bool {
	return t.annotateLevel == "minimal" || t.annotateLevel == "standard" || t.annotateLevel == "verbose"
}

// emitOriginal returns true if original SQL comments should be emitted (standard+)
func (t *transpiler) emitOriginal() bool {
	return t.annotateLevel == "standard" || t.annotateLevel == "verbose"
}

// emitTypeAnnotations returns true if type annotations should be emitted (verbose only)
func (t *transpiler) emitTypeAnnotations() bool {
	return t.annotateLevel == "verbose"
}

// emitSections returns true if section markers should be emitted (verbose only)
func (t *transpiler) emitSections() bool {
	return t.annotateLevel == "verbose"
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

	// Check for DDL-only files (no procedures/functions)
	if !t.hasProcedures && len(bodies) > 0 {
		// File contains statements but no procedures - likely a DDL/schema file
		hint := "This file appears to contain only DDL statements (CREATE TABLE, etc.) without any stored procedures.\n" +
			"      tgpiler transpiles stored procedures to Go functions.\n\n" +
			"      For DDL/schema files, consider:\n" +
			"        - Keep them as SQL migration scripts\n" +
			"        - Use --extract-ddl=FILE to collect DDL from mixed files\n" +
			"        - Use a migration tool like golang-migrate, goose, or atlas"
		return "", fmt.Errorf("no stored procedures found in input\n\n      Hint: %s", hint)
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
	case *ast.CreateFunctionStatement:
		return t.transpileCreateFunction(s)
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
		// Check if this is a DDL statement that should be skipped
		if t.dmlEnabled && t.dmlConfig.SkipDDL && !t.dmlConfig.StrictDDL {
			if skipped, comment := t.trySkipDDL(stmt); skipped {
				return comment, nil
			}
		}
		return "", unsupportedStatementError(stmt)
	}
}

// unsupportedStatementError returns a helpful error message for unsupported statements.
func unsupportedStatementError(stmt ast.Statement) error {
	typeName := fmt.Sprintf("%T", stmt)
	
	// Provide specific hints based on type name
	switch {
	case strings.Contains(typeName, "GoStatement"):
		return fmt.Errorf("unsupported statement type: %s\n"+
			"      Hint: GO is a batch separator with no semantic meaning.\n"+
			"      GO statements are stripped by default. If you see this error,\n"+
			"      use --preserve-go=false or check your tgpiler version.", typeName)
	
	case strings.Contains(typeName, "CreateFunction"):
		return fmt.Errorf("unsupported statement type: %s\n"+
			"      Hint: Table-valued functions are not yet supported.\n"+
			"      Scalar functions with a BEGIN/END body are supported.", typeName)
	
	case strings.Contains(typeName, "CreateView"):
		return fmt.Errorf("unsupported statement type: %s\n"+
			"      Hint: CREATE VIEW is a DDL statement, not procedural code.\n"+
			"      Views should remain in your database; tgpiler transpiles procedures.", typeName)
	
	case strings.Contains(typeName, "CreateTable"):
		return fmt.Errorf("unsupported statement type: %s\n"+
			"      Hint: CREATE TABLE is a DDL statement.\n"+
			"      For temp tables inside procedures, use --dml mode.\n"+
			"      For permanent tables, keep them in your database schema.", typeName)
	
	case strings.Contains(typeName, "CreateIndex"):
		return fmt.Errorf("unsupported statement type: %s\n"+
			"      Hint: CREATE INDEX is a DDL statement.\n"+
			"      Indexes should remain in your database schema.", typeName)
	
	case strings.Contains(typeName, "Alter"):
		return fmt.Errorf("unsupported statement type: %s\n"+
			"      Hint: ALTER statements are DDL and not transpiled.\n"+
			"      These should remain as database migrations.", typeName)
	
	case strings.Contains(typeName, "Drop"):
		return fmt.Errorf("unsupported statement type: %s\n"+
			"      Hint: DROP statements are DDL and not transpiled.\n"+
			"      These should remain as database migrations.", typeName)
	
	case strings.Contains(typeName, "Use"):
		return fmt.Errorf("unsupported statement type: %s\n"+
			"      Hint: USE <database> is a client directive.\n"+
			"      Database selection is handled by your connection string.", typeName)
	
	case strings.Contains(typeName, "CreateSequence"):
		return fmt.Errorf("unsupported statement type: %s\n"+
			"      Hint: CREATE SEQUENCE is a DDL statement.\n"+
			"      Sequences should remain in your database schema.\n"+
			"      Use result.LastInsertId() or uuid.New() in Go.", typeName)
	
	default:
		return fmt.Errorf("unsupported statement type: %s\n"+
			"      Hint: This statement type is not yet implemented.\n"+
			"      Please file an issue at github.com/ha1tch/tgpiler if you need it.", typeName)
	}
}

// trySkipDDL checks if a statement is a DDL statement that should be skipped.
// Returns (true, comment) if skipped, (false, "") if not a skippable DDL.
func (t *transpiler) trySkipDDL(stmt ast.Statement) (bool, string) {
	typeName := fmt.Sprintf("%T", stmt)
	
	var ddlType, ddlName string
	
	switch {
	case strings.Contains(typeName, "CreateSequence"):
		ddlType = "CREATE SEQUENCE"
		// Try to extract sequence name
		if s := stmt.String(); s != "" {
			ddlName = extractDDLName(s, "SEQUENCE")
		}
	case strings.Contains(typeName, "CreateView"):
		ddlType = "CREATE VIEW"
		if s := stmt.String(); s != "" {
			ddlName = extractDDLName(s, "VIEW")
		}
	case strings.Contains(typeName, "CreateIndex"):
		ddlType = "CREATE INDEX"
		if s := stmt.String(); s != "" {
			ddlName = extractDDLName(s, "INDEX")
		}
	case strings.Contains(typeName, "AlterTable"):
		ddlType = "ALTER TABLE"
		if s := stmt.String(); s != "" {
			ddlName = extractDDLName(s, "TABLE")
		}
	case strings.Contains(typeName, "AlterIndex"):
		ddlType = "ALTER INDEX"
	case strings.Contains(typeName, "AlterView"):
		ddlType = "ALTER VIEW"
	case strings.Contains(typeName, "DropIndex"):
		ddlType = "DROP INDEX"
	case strings.Contains(typeName, "DropView"):
		ddlType = "DROP VIEW"
	case strings.Contains(typeName, "Use"):
		ddlType = "USE"
	default:
		return false, ""
	}
	
	// Record warning
	warning := fmt.Sprintf("Skipped %s", ddlType)
	if ddlName != "" {
		warning = fmt.Sprintf("Skipped %s %s", ddlType, ddlName)
	}
	t.ddlWarnings = append(t.ddlWarnings, warning)
	
	// Collect DDL for extraction if configured
	if t.dmlConfig.ExtractDDL != "" {
		t.extractedDDL = append(t.extractedDDL, stmt.String())
	}
	
	// Return comment
	comment := fmt.Sprintf("// %s (DDL - keep in database schema)\n", warning)
	return true, comment
}

// extractDDLName tries to extract the object name from a DDL statement string.
func extractDDLName(sql, keyword string) string {
	upper := strings.ToUpper(sql)
	idx := strings.Index(upper, keyword)
	if idx < 0 {
		return ""
	}
	rest := strings.TrimSpace(sql[idx+len(keyword):])
	// Take the first word (the name)
	fields := strings.Fields(rest)
	if len(fields) > 0 {
		return fields[0]
	}
	return ""
}

// isIfAroundDDL checks if an IF statement wraps DDL statements (CREATE, ALTER, DROP).
// This is common for patterns like: IF NOT EXISTS (...) CREATE SEQUENCE ...
func (t *transpiler) isIfAroundDDL(ifStmt *ast.IfStatement) bool {
	// Check consequence
	if ifStmt.Consequence != nil && t.statementContainsDDL(ifStmt.Consequence) {
		return true
	}
	// Check alternative
	if ifStmt.Alternative != nil && t.statementContainsDDL(ifStmt.Alternative) {
		return true
	}
	return false
}

// statementContainsDDL checks if a statement is or contains DDL.
func (t *transpiler) statementContainsDDL(stmt ast.Statement) bool {
	if stmt == nil {
		return false
	}
	
	// Check the statement type name for DDL patterns
	typeName := fmt.Sprintf("%T", stmt)
	ddlPatterns := []string{
		"CreateSequence", "CreateView", "CreateIndex", "CreateTable",
		"AlterTable", "AlterIndex", "AlterView",
		"DropIndex", "DropView", "DropTable", "DropSequence",
	}
	for _, pattern := range ddlPatterns {
		if strings.Contains(typeName, pattern) {
			return true
		}
	}
	
	// For BEGIN/END blocks, check all contained statements
	if block, ok := stmt.(*ast.BeginEndBlock); ok {
		for _, s := range block.Statements {
			if t.statementContainsDDL(s) {
				return true
			}
		}
	}
	
	return false
}

// skipIfAroundDDL returns a comment for a skipped top-level IF around DDL.
func (t *transpiler) skipIfAroundDDL(ifStmt *ast.IfStatement) string {
	// Try to describe what's being skipped
	ddlDesc := t.describeDDLInStatement(ifStmt.Consequence)
	if ddlDesc == "" && ifStmt.Alternative != nil {
		ddlDesc = t.describeDDLInStatement(ifStmt.Alternative)
	}
	if ddlDesc == "" {
		ddlDesc = "DDL statement"
	}
	
	// Record warning
	warning := fmt.Sprintf("Skipped conditional %s (top-level IF around DDL)", ddlDesc)
	t.ddlWarnings = append(t.ddlWarnings, warning)
	
	// Collect DDL for extraction if configured
	if t.dmlConfig.ExtractDDL != "" {
		t.extractedDDL = append(t.extractedDDL, ifStmt.String())
	}
	
	return fmt.Sprintf("// %s\n// Hint: Keep this in your database migration scripts\n// Original: %s",
		warning, summarizeStatement(ifStmt.String(), 80))
}

// describeDDLInStatement returns a description of DDL found in a statement.
func (t *transpiler) describeDDLInStatement(stmt ast.Statement) string {
	if stmt == nil {
		return ""
	}
	
	typeName := fmt.Sprintf("%T", stmt)
	
	switch {
	case strings.Contains(typeName, "CreateSequence"):
		name := extractDDLName(stmt.String(), "SEQUENCE")
		if name != "" {
			return "CREATE SEQUENCE " + name
		}
		return "CREATE SEQUENCE"
	case strings.Contains(typeName, "CreateView"):
		return "CREATE VIEW"
	case strings.Contains(typeName, "CreateIndex"):
		return "CREATE INDEX"
	case strings.Contains(typeName, "CreateTable"):
		return "CREATE TABLE"
	case strings.Contains(typeName, "AlterTable"):
		return "ALTER TABLE"
	case strings.Contains(typeName, "DropTable"):
		return "DROP TABLE"
	case strings.Contains(typeName, "DropSequence"):
		return "DROP SEQUENCE"
	}
	
	// For blocks, check contents
	if block, ok := stmt.(*ast.BeginEndBlock); ok {
		for _, s := range block.Statements {
			if desc := t.describeDDLInStatement(s); desc != "" {
				return desc
			}
		}
	}
	
	return ""
}

// summarizeStatement returns a truncated version of a statement for comments.
func summarizeStatement(s string, maxLen int) string {
	// Normalize whitespace
	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
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
	t.hasProcedures = true       // Mark that we found a procedure
	sig := "PROC:" + strings.ToLower(procName)

	// Emit section header for verbose mode
	if t.emitSections() {
		out.WriteString("// ============================================================\n")
		out.WriteString(fmt.Sprintf("// PROCEDURE: %s\n", procName))
		if len(proc.Parameters) > 0 {
			var inputNames, outputNames []string
			for _, p := range proc.Parameters {
				name := strings.TrimPrefix(p.Name, "@")
				if p.Output {
					outputNames = append(outputNames, name)
				} else {
					inputNames = append(inputNames, name)
				}
			}
			if len(inputNames) > 0 {
				out.WriteString(fmt.Sprintf("// Inputs: %s\n", strings.Join(inputNames, ", ")))
			}
			if len(outputNames) > 0 {
				out.WriteString(fmt.Sprintf("// Outputs: %s\n", strings.Join(outputNames, ", ")))
			}
		}
		out.WriteString("// ============================================================\n")
	}

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
	
	// Add receiver if configured (DML mode with receiver)
	if t.dmlEnabled && t.dmlConfig.Receiver != "" && t.dmlConfig.ReceiverType != "" {
		out.WriteString(fmt.Sprintf("func (%s %s) %s(", t.dmlConfig.Receiver, t.dmlConfig.ReceiverType, funcName))
		// Always add ctx as first parameter in DML mode with receiver
		out.WriteString("ctx context.Context")
		if len(inputParams) > 0 {
			out.WriteString(", ")
			out.WriteString(strings.Join(inputParams, ", "))
		}
		t.imports["context"] = true
	} else {
		out.WriteString(fmt.Sprintf("func %s(", funcName))
		out.WriteString(strings.Join(inputParams, ", "))
	}
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

	// Pre-scan for @@ROWCOUNT usage
	t.usesRowCount = t.blockUsesRowCount(proc.Body)
	if t.usesRowCount {
		out.WriteString(t.indentStr())
		out.WriteString("var rowsAffected int32\n")
	}

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

	// Emit blank assignments for genuinely unused local variables
	// Skip this when the body is wrapped in TRY/CATCH (IIFE) since variables are scoped to the IIFE
	endsWithReturn := t.blockEndsWithReturn(proc.Body)
	unusedVars := t.symbols.getUnusedVars()
	bodyHasTryCatch := proc.Body != nil && len(proc.Body.Statements) > 0 && t.bodyStartsWithTryCatch(proc.Body)
	
	if len(unusedVars) > 0 && !bodyHasTryCatch {
		if endsWithReturn {
			// Block ends with return - insert suppress statements before the final return
			// Find the last return statement in the output and insert before it
			content := out.String()
			lastReturnIdx := strings.LastIndex(content, "\treturn ")
			if lastReturnIdx == -1 {
				lastReturnIdx = strings.LastIndex(content, "return ")
			}
			if lastReturnIdx != -1 {
				// Build suppress statements
				var suppressBuilder strings.Builder
				suppressBuilder.WriteString("\n")
				suppressBuilder.WriteString(t.indentStr())
				suppressBuilder.WriteString("// Suppress unused variable warnings\n")
				for _, varName := range unusedVars {
					suppressBuilder.WriteString(t.indentStr())
					suppressBuilder.WriteString(fmt.Sprintf("_ = %s\n", varName))
				}
				// Insert before the return
				newContent := content[:lastReturnIdx] + suppressBuilder.String() + content[lastReturnIdx:]
				out.Reset()
				out.WriteString(newContent)
			}
		} else {
			// Block doesn't end with return - emit at end as before
			out.WriteString("\n")
			out.WriteString(t.indentStr())
			out.WriteString("// Suppress unused variable warnings\n")
			for _, varName := range unusedVars {
				out.WriteString(t.indentStr())
				out.WriteString(fmt.Sprintf("_ = %s\n", varName))
			}
		}
	}

	// Final return if we have output params or return code, 
	// but only if the block doesn't already end with a return
	if (len(outputParams) > 0 || hasReturn || needsErrorReturn) && !endsWithReturn {
		out.WriteString(t.indentStr())
		out.WriteString(t.buildReturnStatement(nil))
		out.WriteString("\n")
	}

	t.indent = 0
	out.WriteString("}")

	// Clear procedure-specific state
	t.outputParams = nil
	t.hasReturnCode = false
	t.currentProcName = "" // Reset so top-level statements are detected

	return out.String(), nil
}

// transpileCreateFunction converts a T-SQL function to a Go function.
func (t *transpiler) transpileCreateFunction(fn *ast.CreateFunctionStatement) (string, error) {
	var out strings.Builder

	// Reset symbol table for new function scope
	t.symbols = newSymbolTable()

	// Get function name
	funcName := fn.Name.Parts[len(fn.Name.Parts)-1].Value
	goFuncName := goIdentifier(funcName)
	t.hasProcedures = true // Mark that we found a function (counts as a procedure)

	// Only support scalar functions with a body for now
	if fn.ReturnsTable || fn.TableDef != nil {
		return "", fmt.Errorf("table-valued functions not yet supported: %s", funcName)
	}
	if fn.Body == nil {
		// Inline TVF (RETURNS TABLE AS RETURN SELECT...) 
		return "", fmt.Errorf("inline table-valued functions not yet supported: %s", funcName)
	}

	// Determine return type
	returnType := "interface{}"
	if fn.ReturnType != nil {
		var err error
		returnType, err = t.mapDataType(fn.ReturnType)
		if err != nil {
			return "", fmt.Errorf("function %s return type: %w", funcName, err)
		}
	}

	// Register function for call resolution
	t.userFunctions[strings.ToLower(funcName)] = &userFuncInfo{
		name:       funcName,
		goName:     goFuncName,
		params:     fn.Parameters,
		returnType: returnType,
	}

	// Build function signature
	out.WriteString(fmt.Sprintf("func %s(", goFuncName))

	// Parameters
	var params []string
	for _, p := range fn.Parameters {
		goType, err := t.mapDataType(p.DataType)
		if err != nil {
			return "", fmt.Errorf("parameter %s: %w", p.Name, err)
		}
		paramName := goIdentifier(strings.TrimPrefix(p.Name, "@"))
		t.symbols.define(paramName, typeInfoFromDataType(p.DataType))
		params = append(params, fmt.Sprintf("%s %s", paramName, goType))
	}
	out.WriteString(strings.Join(params, ", "))
	out.WriteString(") ")

	// Return type
	out.WriteString(returnType)
	out.WriteString(" {\n")

	// Transpile body
	t.indent = 1
	t.inProcBody = true
	
	for _, stmt := range fn.Body.Statements {
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
	
	t.inProcBody = false

	// Emit blank assignments for genuinely unused local variables
	// Skip this when the body is wrapped in TRY/CATCH (IIFE) since variables are scoped to the IIFE
	endsWithReturn := t.blockEndsWithReturn(fn.Body)
	unusedVars := t.symbols.getUnusedVars()
	bodyHasTryCatch := fn.Body != nil && len(fn.Body.Statements) > 0 && t.bodyStartsWithTryCatch(fn.Body)
	
	if len(unusedVars) > 0 && !bodyHasTryCatch {
		if endsWithReturn {
			// Block ends with return - insert suppress statements before the final return
			content := out.String()
			lastReturnIdx := strings.LastIndex(content, "\treturn ")
			if lastReturnIdx == -1 {
				lastReturnIdx = strings.LastIndex(content, "return ")
			}
			if lastReturnIdx != -1 {
				var suppressBuilder strings.Builder
				suppressBuilder.WriteString("\n")
				suppressBuilder.WriteString(t.indentStr())
				suppressBuilder.WriteString("// Suppress unused variable warnings\n")
				for _, varName := range unusedVars {
					suppressBuilder.WriteString(t.indentStr())
					suppressBuilder.WriteString(fmt.Sprintf("_ = %s\n", varName))
				}
				newContent := content[:lastReturnIdx] + suppressBuilder.String() + content[lastReturnIdx:]
				out.Reset()
				out.WriteString(newContent)
			}
		} else {
			out.WriteString("\n")
			out.WriteString(t.indentStr())
			out.WriteString("// Suppress unused variable warnings\n")
			for _, varName := range unusedVars {
				out.WriteString(t.indentStr())
				out.WriteString(fmt.Sprintf("_ = %s\n", varName))
			}
		}
	}

	t.indent = 0
	out.WriteString("}")

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

// blockUsesRowCount checks if a block contains @@ROWCOUNT references
func (t *transpiler) blockUsesRowCount(block *ast.BeginEndBlock) bool {
	if block == nil {
		return false
	}
	for _, stmt := range block.Statements {
		if t.statementUsesRowCount(stmt) {
			return true
		}
	}
	return false
}

// statementUsesRowCount checks if a statement uses @@ROWCOUNT
func (t *transpiler) statementUsesRowCount(stmt ast.Statement) bool {
	switch s := stmt.(type) {
	case *ast.SetStatement:
		return t.expressionUsesRowCount(s.Variable) || t.expressionUsesRowCount(s.Value)
	case *ast.IfStatement:
		if t.expressionUsesRowCount(s.Condition) {
			return true
		}
		if t.statementUsesRowCount(s.Consequence) {
			return true
		}
		if s.Alternative != nil && t.statementUsesRowCount(s.Alternative) {
			return true
		}
		return false
	case *ast.WhileStatement:
		if t.expressionUsesRowCount(s.Condition) {
			return true
		}
		return t.statementUsesRowCount(s.Body)
	case *ast.BeginEndBlock:
		return t.blockUsesRowCount(s)
	case *ast.TryCatchStatement:
		if s.TryBlock != nil && t.blockUsesRowCount(s.TryBlock) {
			return true
		}
		if s.CatchBlock != nil && t.blockUsesRowCount(s.CatchBlock) {
			return true
		}
		return false
	case *ast.DeclareStatement:
		for _, v := range s.Variables {
			if v.Value != nil && t.expressionUsesRowCount(v.Value) {
				return true
			}
		}
		return false
	default:
		return false
	}
}

// expressionUsesRowCount checks if an expression contains @@ROWCOUNT
func (t *transpiler) expressionUsesRowCount(expr ast.Expression) bool {
	if expr == nil {
		return false
	}
	switch e := expr.(type) {
	case *ast.Variable:
		return strings.ToUpper(e.Name) == "@@ROWCOUNT"
	case *ast.InfixExpression:
		return t.expressionUsesRowCount(e.Left) || t.expressionUsesRowCount(e.Right)
	case *ast.PrefixExpression:
		return t.expressionUsesRowCount(e.Right)
	case *ast.FunctionCall:
		for _, arg := range e.Arguments {
			if t.expressionUsesRowCount(arg) {
				return true
			}
		}
		return false
	case *ast.CaseExpression:
		for _, when := range e.WhenClauses {
			if t.expressionUsesRowCount(when.Condition) || t.expressionUsesRowCount(when.Result) {
				return true
			}
		}
		if e.ElseClause != nil && t.expressionUsesRowCount(e.ElseClause) {
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

// bodyStartsWithTryCatch checks if a block has a TRY/CATCH statement at the top level.
// This is used to skip unused variable suppression when variables are scoped to an IIFE.
func (t *transpiler) bodyStartsWithTryCatch(block *ast.BeginEndBlock) bool {
	if block == nil || len(block.Statements) == 0 {
		return false
	}
	// Check if any top-level statement is TRY/CATCH
	for _, stmt := range block.Statements {
		if _, isTryCatch := stmt.(*ast.TryCatchStatement); isTryCatch {
			return true
		}
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

// buildErrorReturn generates a return statement with error for error handling.
// Handles TRY/CATCH blocks specially since they're inside anonymous functions.
func (t *transpiler) buildErrorReturn() string {
	// In TRY block, we're inside an anonymous func() - cannot return values
	// Panic with error to let defer/recover catch it
	if t.inTryBlock {
		return "panic(err)"
	}
	
	// In CATCH block, we're inside a defer func - cannot return values
	// Use _ = err to acknowledge error but continue
	if t.inCatchBlock {
		return "_ = err // Operation failed in error handler"
	}

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
		// Mark as declared for unused variable tracking
		t.symbols.markDeclared(varName)

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

		// Type annotation for verbose mode
		var typeComment string
		if t.emitTypeAnnotations() && v.DataType != nil {
			typeComment = fmt.Sprintf(" // T-SQL: %s", strings.ToUpper(v.DataType.String()))
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
			parts = append(parts, fmt.Sprintf("%svar %s %s = %s%s", prefix, varName, goType, valExpr, typeComment))
		} else {
			parts = append(parts, fmt.Sprintf("%svar %s %s%s", prefix, varName, goType, typeComment))
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

	// For variable assignment, get the variable name directly without marking as "used"
	// (writing to a variable is not "using" it for unused variable detection)
	var varExpr string
	if v, ok := set.Variable.(*ast.Variable); ok {
		varExpr = goIdentifier(v.Name)
	} else {
		// For complex expressions (method calls etc), use transpileExpression
		var err error
		varExpr, err = t.transpileExpression(set.Variable)
		if err != nil {
			return "", err
		}
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
	if varType != nil && varType.isDecimal && !isNull {
		valExpr = t.ensureDecimal(set.Value, valExpr)
	}
	if varType != nil && varType.isBool && !isNull {
		valExpr = t.ensureBool(set.Value, valExpr)
	}

	// Skip self-assignments (e.g., SET @x = ISNULL(@x, 0) for value types becomes x = x)
	if varExpr == valExpr {
		return fmt.Sprintf("%s// SET %s = %s (no-op, skipped)", prefix, varExpr, valExpr), nil
	}

	return fmt.Sprintf("%s%s = %s", prefix, varExpr, valExpr), nil
}

func (t *transpiler) transpileIf(ifStmt *ast.IfStatement) (string, error) {
	// Check for top-level IF around DDL (common pattern: IF NOT EXISTS ... CREATE ...)
	// At top level (not inside a procedure), IF statements containing DDL should be skipped
	if t.currentProcName == "" && t.isIfAroundDDL(ifStmt) {
		return t.skipIfAroundDDL(ifStmt), nil
	}
	
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
	// Push scope for if block - variables declared here are local to this block
	savedSymbols := t.symbols
	t.symbols = t.symbols.pushScope()
	conseq, err := t.transpileStatementBlock(ifStmt.Consequence)
	if err != nil {
		return "", err
	}
	out.WriteString(conseq)
	// Emit unused variable suppression for this scope before popping
	unusedVars := t.symbols.getUnusedVars()
	for _, v := range unusedVars {
		out.WriteString(t.indentStr() + "_ = " + v + "\n")
	}
	t.symbols = savedSymbols // Pop scope
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
		// Push scope for else block
		savedSymbols = t.symbols
		t.symbols = t.symbols.pushScope()
		alt, err := t.transpileStatementBlock(ifStmt.Alternative)
		if err != nil {
			return "", err
		}
		out.WriteString(alt)
		// Emit unused variable suppression for else scope before popping
		unusedVars = t.symbols.getUnusedVars()
		for _, v := range unusedVars {
			out.WriteString(t.indentStr() + "_ = " + v + "\n")
		}
		t.symbols = savedSymbols // Pop scope
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
	// Push scope for loop body
	savedSymbols := t.symbols
	t.symbols = t.symbols.pushScope()
	body, err := t.transpileStatementBlock(whileStmt.Body)
	if err != nil {
		return "", err
	}
	out.WriteString(body)
	// Emit unused variable suppression for loop scope before popping
	unusedVars := t.symbols.getUnusedVars()
	for _, v := range unusedVars {
		out.WriteString(t.indentStr() + "_ = " + v + "\n")
	}
	t.symbols = savedSymbols // Pop scope
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

	// Add TODO marker if requested
	if t.emitTODOs() {
		out.WriteString("// TODO(tgpiler): TRY/CATCH converted to defer/recover IIFE - verify error semantics\n")
		out.WriteString(t.indentStr())
	}

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
	
	// Push a new scope for the defer func - variables declared here are local
	savedSymbols := t.symbols
	t.symbols = t.symbols.pushScope()

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
	
	// Pop the CATCH block scope
	t.symbols = savedSymbols
	t.inCatchBlock = wasInCatchBlock

	t.indent--
	out.WriteString(t.indentStr())
	out.WriteString("}\n")
	t.indent--
	out.WriteString(t.indentStr())
	out.WriteString("}()\n")

	// TRY block - set flag to handle RETURN statements correctly
	// Push a new scope for the IIFE - variables declared here are in the IIFE scope
	wasInTryBlock := t.inTryBlock
	t.inTryBlock = true
	savedTrySymbols := t.symbols
	t.symbols = t.symbols.pushScope()
	
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
	
	// Emit suppression for unused variables declared in this scope (inside the IIFE)
	unusedVars := t.symbols.getUnusedVars()
	if len(unusedVars) > 0 {
		out.WriteString(t.indentStr())
		out.WriteString("// Suppress unused variable warnings\n")
		for _, varName := range unusedVars {
			out.WriteString(t.indentStr())
			out.WriteString(fmt.Sprintf("_ = %s\n", varName))
		}
	}
	
	// Pop the TRY block scope
	t.symbols = savedTrySymbols
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
	
	// Inside a CATCH block (defer function), just return to exit the defer
	// Cannot return values from a defer - values are set via named return params
	if t.inCatchBlock {
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
	out.WriteString("\t" + t.buildErrorReturn() + "\n")
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
	out.WriteString("\t" + t.buildErrorReturn() + "\n")
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
	
	// Extract table name to check if it's a temp table
	tableName := ""
	if exists.Subquery.From != nil && len(exists.Subquery.From.Tables) > 0 {
		if tn, ok := exists.Subquery.From.Tables[0].(*ast.TableName); ok && tn.Name != nil && len(tn.Name.Parts) > 0 {
			tableName = tn.Name.Parts[len(tn.Name.Parts)-1].Value
		}
	}
	
	// Track temp table usage
	if isTempTable(tableName) {
		t.recordTempTableUsed(tableName)
	}
	
	// For gRPC backend, try to convert to a gRPC call (but not for temp tables)
	if t.dmlEnabled && t.dmlConfig.Backend == BackendGRPC && !isTempTable(tableName) {
		if result, ok := t.tryExistsAsGRPC(exists); ok {
			return result, nil
		}
		// Fall through to SQL if gRPC conversion fails
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

// recordTempTableUsed adds a temp table to the tracking list (deduped).
func (t *transpiler) recordTempTableUsed(name string) {
	for _, existing := range t.tempTablesUsed {
		if existing == name {
			return
		}
	}
	t.tempTablesUsed = append(t.tempTablesUsed, name)
}

// tryExistsAsGRPC attempts to convert EXISTS to a gRPC call.
// Returns the code and true if successful, empty and false otherwise.
func (t *transpiler) tryExistsAsGRPC(exists *ast.ExistsExpression) (string, bool) {
	subquery := exists.Subquery
	if subquery == nil {
		return "", false
	}
	
	// Extract table name from subquery
	tableName := ""
	if subquery.From != nil && len(subquery.From.Tables) > 0 {
		if tn, ok := subquery.From.Tables[0].(*ast.TableName); ok && tn.Name != nil && len(tn.Name.Parts) > 0 {
			tableName = tn.Name.Parts[len(tn.Name.Parts)-1].Value
		}
	}
	if tableName == "" {
		return "", false
	}
	
	// Extract WHERE fields
	whereFields := t.extractExistsWhereFields(subquery.Where)
	if len(whereFields) == 0 {
		return "", false
	}
	
	// Build method name: Get{Table}By{Column} (singularize table name like inferGRPCMethod does)
	entityName := toPascalCase(singularize(tableName))
	methodName := "Get" + entityName + "By" + toPascalCase(whereFields[0].column)
	
	// Get client variable - same logic as getGRPCClientForTable
	clientVar := t.dmlConfig.StoreVar
	if t.dmlConfig.GRPCClientVar != "" && t.dmlConfig.GRPCClientVar != "client" {
		clientVar = t.dmlConfig.GRPCClientVar
	}
	if clientVar == "" {
		clientVar = "client"
	}
	
	// Get proto package
	protoPackage := t.dmlConfig.ProtoPackage
	
	// Build request fields
	var reqFields []string
	for _, wf := range whereFields {
		reqFields = append(reqFields, fmt.Sprintf("\t\t\t%s: %s,", goExportedIdentifier(wf.column), wf.variable))
	}
	
	// Generate the gRPC existence check
	var out strings.Builder
	out.WriteString("func() bool {\n")
	if protoPackage != "" {
		out.WriteString(fmt.Sprintf("\t\tresp, err := %s.%s(ctx, &%s.%sRequest{\n", clientVar, methodName, protoPackage, methodName))
	} else {
		out.WriteString(fmt.Sprintf("\t\tresp, err := %s.%s(ctx, &%sRequest{\n", clientVar, methodName, methodName))
	}
	for _, rf := range reqFields {
		out.WriteString(rf + "\n")
	}
	out.WriteString("\t\t})\n")
	out.WriteString("\t\treturn err == nil && resp != nil\n")
	out.WriteString("\t}()")
	
	return out.String(), true
}

// extractExistsWhereFields extracts column=variable pairs from a WHERE expression
func (t *transpiler) extractExistsWhereFields(expr ast.Expression) []struct{ column, variable string } {
	var fields []struct{ column, variable string }
	if expr == nil {
		return fields
	}
	
	switch e := expr.(type) {
	case *ast.InfixExpression:
		op := strings.ToUpper(e.Operator)
		if op == "AND" || op == "OR" {
			fields = append(fields, t.extractExistsWhereFields(e.Left)...)
			fields = append(fields, t.extractExistsWhereFields(e.Right)...)
			return fields
		}
		
		// Extract column name from left side
		var colName string
		if id, ok := e.Left.(*ast.Identifier); ok {
			colName = id.Value
		} else if qid, ok := e.Left.(*ast.QualifiedIdentifier); ok && len(qid.Parts) > 0 {
			colName = qid.Parts[len(qid.Parts)-1].Value
		}
		
		if colName == "" {
			return fields
		}
		
		// Extract value from right side - could be variable or literal
		var value string
		switch v := e.Right.(type) {
		case *ast.Variable:
			value = goIdentifier(strings.TrimPrefix(v.Name, "@"))
		case *ast.StringLiteral:
			value = fmt.Sprintf("%q", v.Value)
		case *ast.IntegerLiteral:
			value = fmt.Sprintf("%d", v.Value)
		case *ast.FloatLiteral:
			value = fmt.Sprintf("%v", v.Value)
		case *ast.NullLiteral:
			value = "nil"
		case *ast.Identifier:
			// Could be TRUE/FALSE
			upper := strings.ToUpper(v.Value)
			if upper == "TRUE" {
				value = "true"
			} else if upper == "FALSE" {
				value = "false"
			}
		}
		
		if value != "" {
			fields = append(fields, struct{ column, variable string }{colName, value})
		}
	}
	
	return fields
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

// truncateSQL truncates a SQL string for display in comments
func truncateSQL(sql string, maxLen int) string {
	// Normalize whitespace
	sql = strings.ReplaceAll(sql, "\n", " ")
	sql = strings.ReplaceAll(sql, "\r", "")
	sql = strings.ReplaceAll(sql, "\t", " ")
	for strings.Contains(sql, "  ") {
		sql = strings.ReplaceAll(sql, "  ", " ")
	}
	sql = strings.TrimSpace(sql)
	
	if len(sql) <= maxLen {
		return sql
	}
	return sql[:maxLen-3] + "..."
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