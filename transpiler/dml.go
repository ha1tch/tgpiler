// Package transpiler - DML statement handling
// Converts T-SQL DML operations to Go code targeting different backends:
// - SQL backends (PostgreSQL, MySQL, SQLite): generates database/sql calls
// - gRPC backend: generates gRPC client calls
// - Mock backend: generates mock store calls
package transpiler

import (
	"fmt"
	"strings"

	"github.com/ha1tch/tsqlparser/ast"
)

// BackendType identifies the target backend for DML code generation.
type BackendType string

const (
	BackendSQL    BackendType = "sql"    // Generic SQL (uses dialect)
	BackendGRPC   BackendType = "grpc"   // gRPC client calls
	BackendMock   BackendType = "mock"   // Mock store calls
	BackendInline BackendType = "inline" // Inline SQL strings (for migration)
)

// DMLConfig configures DML transpilation.
type DMLConfig struct {
	// Target backend
	Backend BackendType

	// Fallback backend for operations that don't map to primary backend
	// e.g., temp table operations when Backend=grpc should fall back to sql
	FallbackBackend  BackendType
	FallbackExplicit bool // True if user explicitly set --fallback-backend

	// SQL dialect (postgres, mysql, sqlite, sqlserver)
	SQLDialect string

	// Repository/store variable name (e.g., "r.db", "r.store", "r.client")
	StoreVar string

	// Receiver configuration for generated functions
	Receiver     string // Receiver variable name (e.g., "r") - empty means no receiver
	ReceiverType string // Receiver type (e.g., "*Repository", "*Service")

	// GO statement handling
	PreserveGo bool // If true, don't strip GO statements (default: false, strip them)

	// Sequence handling mode
	// "db" - use database features (RETURNING id for Postgres, LAST_INSERT_ID() for MySQL)
	// "uuid" - generate uuid.New() application-side
	// "stub" - generate TODO placeholder
	SequenceMode string

	// NEWID() handling mode
	// "app" - generate uuid.New() application-side (default, recommended)
	// "db" - use database-specific UUID function
	// "grpc" - call gRPC ID service
	// "mock" - generate predictable sequential UUIDs for testing
	// "stub" - generate TODO placeholder
	NewidMode string

	// gRPC client variable for --newid=grpc mode
	IDServiceVar string

	// DDL handling
	// SkipDDL: skip CREATE TABLE/VIEW/INDEX/SEQUENCE with warning (default: true)
	// StrictDDL: fail on any DDL statement
	// ExtractDDL: file path to extract skipped DDL
	SkipDDL    bool
	StrictDDL  bool
	ExtractDDL string

	// Whether to use transactions
	UseTransactions bool

	// gRPC backend options
	GRPCClientVar    string            // gRPC client variable name (e.g., "client", "svc")
	GRPCMappings     map[string]string // procedure -> service.method
	ProtoPackage     string            // Proto package for gRPC
	TableToService   map[string]string // table -> service name (e.g., "Products" -> "CatalogService")
	TableToClient    map[string]string // table -> client variable (e.g., "Products" -> "catalogClient")
	ServiceToPackage map[string]string // service -> proto package (e.g., "CatalogService" -> "catalogpb")

	// Mock backend options
	MockStoreVar string // Mock store variable name (e.g., "store", "mockDB")

	// SPLogger configuration
	UseSPLogger    bool   // Use SPLogger for CATCH blocks
	SPLoggerVar    string // Variable name for logger (e.g., "spLogger", "r.logger")
	SPLoggerType   string // Logger type: slog, db, file, multi, nop
	SPLoggerTable  string // Table name for db logger
	SPLoggerFile   string // File path for file logger
	SPLoggerFormat string // Format for file logger: json, text
	GenLoggerInit  bool   // Generate logger initialization code
	
	// Annotation level: none, minimal, standard, verbose
	// minimal: TODO markers for patterns needing attention
	// standard: TODOs + Original SQL comments
	// verbose: All of the above + type annotations + section markers
	AnnotateLevel string
}

// DefaultDMLConfig returns sensible defaults.
func DefaultDMLConfig() DMLConfig {
	return DMLConfig{
		Backend:          BackendSQL,
		FallbackBackend:  BackendSQL, // For temp tables when using grpc/mock
		SQLDialect:       "postgres",
		StoreVar:         "r.db",
		Receiver:         "r",
		ReceiverType:     "*Repository",
		SequenceMode:     "db",
		NewidMode:        "app",
		SkipDDL:          true,
		StrictDDL:        false,
		UseTransactions:  false,
		GRPCClientVar:    "client",
		GRPCMappings:     make(map[string]string),
		TableToService:   make(map[string]string),
		TableToClient:    make(map[string]string),
		ServiceToPackage: make(map[string]string),
		MockStoreVar:     "store",
		UseSPLogger:      false,
		SPLoggerVar:      "spLogger",
		SPLoggerType:     "slog",
		SPLoggerTable:    "Error.LogForStoreProcedure",
		SPLoggerFormat:   "json",
		AnnotateLevel:    "none",
	}
}

// dmlTranspiler handles DML statement conversion.
type dmlTranspiler struct {
	*transpiler
	config DMLConfig
}

// emitResultHandling generates the appropriate result handling code
// If usesRowCount is true, captures rowsAffected; otherwise discards result
func (dt *dmlTranspiler) emitResultHandling(out *strings.Builder, comment string) {
	out.WriteString(dt.indentStr())
	if dt.usesRowCount {
		out.WriteString("if ra, raErr := result.RowsAffected(); raErr == nil { rowsAffected = int32(ra) }")
	} else {
		out.WriteString("_ = result")
		if comment != "" {
			out.WriteString(" // " + comment)
		}
	}
	out.WriteString("\n")
}

// buildErrorReturn generates a return statement with error for DML operations
// In CATCH blocks (defer func), cannot return values - operations fail silently
func (dt *dmlTranspiler) buildErrorReturn() string {
	// In TRY block, we're inside an anonymous func() - cannot return values
	// Panic with error to let defer/recover catch it
	if dt.transpiler.inTryBlock {
		return "panic(err)"
	}
	
	// In CATCH block, we're inside a defer func - cannot return values
	// Use _ = err to acknowledge error but continue
	if dt.transpiler.inCatchBlock {
		return "_ = err // Operation failed in error handler"
	}

	var parts []string
	
	// Add output params
	for _, p := range dt.outputParams {
		paramName := goIdentifier(strings.TrimPrefix(p.Name, "@"))
		parts = append(parts, paramName)
	}
	
	// Add return code if present
	if dt.hasReturnCode {
		parts = append(parts, "0")
	}
	
	// Add error
	parts = append(parts, "err")
	
	return "return " + strings.Join(parts, ", ")
}

// transpileSelect converts a SELECT statement to Go code.
func (t *transpiler) transpileSelect(s *ast.SelectStatement) (string, error) {
	dt := &dmlTranspiler{transpiler: t, config: t.dmlConfig}
	return dt.transpileSelect(s)
}

func (dt *dmlTranspiler) transpileSelect(s *ast.SelectStatement) (string, error) {
	// Determine effective backend (use fallback for temp tables)
	tableName := dt.extractMainTable(s)
	backend := dt.getEffectiveBackend(tableName)
	
	switch backend {
	case BackendSQL:
		return dt.transpileSelectSQL(s)
	case BackendGRPC:
		return dt.transpileSelectGRPC(s)
	case BackendMock:
		return dt.transpileSelectMock(s)
	case BackendInline:
		return dt.transpileSelectInline(s)
	default:
		return dt.transpileSelectSQL(s)
	}
}

// transpileSelectSQL generates database/sql code for SELECT.
func (dt *dmlTranspiler) transpileSelectSQL(s *ast.SelectStatement) (string, error) {
	var out strings.Builder

	// Check if this is a SELECT INTO variable assignment
	assignments := dt.extractSelectAssignments(s)
	if len(assignments) > 0 {
		return dt.transpileSelectIntoVars(s, assignments)
	}

	// Build the query string
	query, args := dt.buildSelectQuery(s)
	
	// Post-process to catch any remaining @variable references
	query, extraArgs := dt.substituteVariablesInQuery(query)
	args = append(args, extraArgs...)
	
	// Get the database variable (tx if in transaction, StoreVar otherwise)
	dbVar := dt.getDBVar()
	
	// Extract column names for scan targets
	columns := dt.extractSelectColumns(s)
	scanDecl, scanTargets := dt.generateScanTargets(columns)

	// Generate the Go code
	out.WriteString("// SELECT query\n")
	out.WriteString(dt.indentStr())
	
	// Generate variable declarations for scan targets
	if scanDecl != "" {
		out.WriteString(scanDecl)
		out.WriteString("\n")
		out.WriteString(dt.indentStr())
	}

	if dt.isSingleRowSelect(s) {
		// Use QueryRow for single-row SELECT
		out.WriteString(fmt.Sprintf("row := %s.QueryRowContext(ctx, %q", dbVar, query))
		for _, arg := range args {
			out.WriteString(", " + arg)
		}
		out.WriteString(")\n")
		out.WriteString(dt.indentStr())
		out.WriteString(fmt.Sprintf("if err := row.Scan(%s); err != nil {\n", scanTargets))
		out.WriteString(dt.indentStr())
		out.WriteString("\t")
		out.WriteString(dt.buildErrorReturn())
		out.WriteString("\n")
		out.WriteString(dt.indentStr())
		out.WriteString("}")
	} else {
		// Use Query for multi-row SELECT - check if rows/err already declared
		rowsDeclared := dt.symbols.isDeclared("rows")
		errDeclared := dt.symbols.isDeclared("err")
		
		assignOp := ":="
		if rowsDeclared && errDeclared {
			assignOp = "="
		}
		dt.symbols.markDeclared("rows")
		dt.symbols.markDeclared("err")
		
		out.WriteString(fmt.Sprintf("rows, err %s %s.QueryContext(ctx, %q", assignOp, dbVar, query))
		for _, arg := range args {
			out.WriteString(", " + arg)
		}
		out.WriteString(")\n")
		out.WriteString(dt.indentStr())
		out.WriteString("if err != nil {\n")
		out.WriteString(dt.indentStr())
		out.WriteString("\t")
		out.WriteString(dt.buildErrorReturn())
		out.WriteString("\n")
		out.WriteString(dt.indentStr())
		out.WriteString("}\n")
		out.WriteString(dt.indentStr())
		out.WriteString("defer rows.Close()\n")
		out.WriteString(dt.indentStr())
		out.WriteString("for rows.Next() {\n")
		out.WriteString(dt.indentStr())
		out.WriteString(fmt.Sprintf("\tif err := rows.Scan(%s); err != nil {\n", scanTargets))
		out.WriteString(dt.indentStr())
		out.WriteString("\t\t")
		out.WriteString(dt.buildErrorReturn())
		out.WriteString("\n")
		out.WriteString(dt.indentStr())
		out.WriteString("\t}\n")
		out.WriteString(dt.indentStr())
		out.WriteString("}")
	}

	return out.String(), nil
}

// transpileSelectIntoVars handles SELECT @var = col pattern.
func (dt *dmlTranspiler) transpileSelectIntoVars(s *ast.SelectStatement, assignments []varAssignment) (string, error) {
	var out strings.Builder
	
	// This function uses sql.ErrNoRows
	dt.imports["database/sql"] = true

	// Emit original SQL if requested
	if dt.emitOriginal() {
		out.WriteString(fmt.Sprintf("// Original: %s\n", truncateSQL(s.String(), 100)))
		out.WriteString(dt.indentStr())
	}

	// Build query
	query, args := dt.buildSelectQuery(s)
	
	// Post-process to catch any remaining @variable references
	query, extraArgs := dt.substituteVariablesInQuery(query)
	args = append(args, extraArgs...)
	
	// Get the database variable (tx if in transaction, StoreVar otherwise)
	dbVar := dt.getDBVar()

	// Generate Scan targets from assignments
	var scanTargets []string
	for _, a := range assignments {
		scanTargets = append(scanTargets, "&"+a.varName)
	}

	// Check if err is already declared
	assignOp := ":="
	if dt.symbols.isDeclared("err") {
		assignOp = "="
	}
	dt.symbols.markDeclared("err")
	
	// Need database/sql for sql.ErrNoRows
	dt.imports["database/sql"] = true

	out.WriteString(fmt.Sprintf("err %s %s.QueryRowContext(ctx, %q", assignOp, dbVar, query))
	for _, arg := range args {
		out.WriteString(", " + arg)
	}
	out.WriteString(").Scan(" + strings.Join(scanTargets, ", ") + ")\n")
	out.WriteString(dt.indentStr())
	out.WriteString("if err != nil && err != sql.ErrNoRows {\n")
	out.WriteString(dt.indentStr())
	out.WriteString("\t")
	out.WriteString(dt.buildErrorReturn())
	out.WriteString("\n")
	out.WriteString(dt.indentStr())
	out.WriteString("}")

	return out.String(), nil
}

// transpileSelectGRPC generates gRPC client code for SELECT.
func (dt *dmlTranspiler) transpileSelectGRPC(s *ast.SelectStatement) (string, error) {
	// Check if this is a SELECT INTO variable assignment
	assignments := dt.extractSelectAssignments(s)
	
	// Extract table name to determine service
	tableName := dt.extractMainTable(s)
	
	// If no table (SELECT of local variables only), skip gRPC call
	if tableName == "" {
		// This is something like SELECT @var AS Name or SELECT @a, @b
		// Just return a comment - the variables are already in scope
		return "// SELECT of local variables (no gRPC call needed)", nil
	}
	
	methodName := dt.inferGRPCMethod(s, tableName)

	// Get client variable and proto package for this table
	clientVar := dt.getGRPCClientForTable(tableName)
	protoPackage := dt.getProtoPackageForTable(tableName)

	var out strings.Builder
	out.WriteString(fmt.Sprintf("// gRPC call: %s.%s\n", clientVar, methodName))
	out.WriteString(dt.indentStr())

	// Build the request
	if protoPackage != "" {
		out.WriteString(fmt.Sprintf("resp, err := %s.%s(ctx, &%s.%sRequest{\n",
			clientVar, methodName, protoPackage, methodName))
	} else {
		out.WriteString(fmt.Sprintf("resp, err := %s.%s(ctx, &%sRequest{\n",
			clientVar, methodName, methodName))
	}

	// Add request fields from WHERE clause (variables and literals)
	whereFields := dt.extractWhereFieldsWithLiterals(s)
	hasComplexFields := false
	var complexWarnings []string
	for _, wf := range whereFields {
		if wf.isComplex {
			hasComplexFields = true
			complexWarnings = append(complexWarnings, fmt.Sprintf("%s: %s", wf.column, wf.rawExpr))
			continue // Skip complex fields in request
		}
		out.WriteString(dt.indentStr())
		out.WriteString(fmt.Sprintf("\t%s: %s,\n", goExportedIdentifier(wf.column), wf.value))
	}
	
	// Add warning comment for complex fields that were skipped
	if hasComplexFields {
		out.WriteString(dt.indentStr())
		out.WriteString("\t// WARNING: Complex WHERE expressions skipped (require manual conversion):\n")
		for _, w := range complexWarnings {
			out.WriteString(dt.indentStr())
			out.WriteString(fmt.Sprintf("\t//   %s\n", w))
		}
	}

	out.WriteString(dt.indentStr())
	out.WriteString("})\n")
	out.WriteString(dt.indentStr())
	out.WriteString("if err != nil {\n")
	out.WriteString(dt.indentStr())
	out.WriteString("\t")
	out.WriteString(dt.buildErrorReturn())
	out.WriteString("\n")
	out.WriteString(dt.indentStr())
	out.WriteString("}\n")
	
	// If we have SELECT INTO assignments, extract values from response
	if len(assignments) > 0 {
		out.WriteString(dt.indentStr())
		out.WriteString("if resp != nil {\n")
		for _, a := range assignments {
			out.WriteString(dt.indentStr())
			// Map column name to proto field name (PascalCase)
			protoField := goExportedIdentifier(a.column)
			out.WriteString(fmt.Sprintf("\t%s = resp.%s\n", a.varName, protoField))
		}
		out.WriteString(dt.indentStr())
		out.WriteString("}")
	} else {
		// No assignments - just note the response is available
		out.WriteString(dt.indentStr())
		out.WriteString("_ = resp // TODO: use response")
	}

	return out.String(), nil
}

// transpileSelectMock generates mock store code for SELECT.
func (dt *dmlTranspiler) transpileSelectMock(s *ast.SelectStatement) (string, error) {
	tableName := dt.extractMainTable(s)
	methodName := dt.inferMockMethod(s, tableName)

	var out strings.Builder
	
	// Check if result and err are already declared
	resultDeclared := dt.symbols.isDeclared("result")
	errDeclared := dt.symbols.isDeclared("err")
	assignOp := ":="
	if resultDeclared && errDeclared {
		assignOp = "="
	}
	dt.symbols.markDeclared("result")
	dt.symbols.markDeclared("err")
	
	out.WriteString(fmt.Sprintf("result, err %s %s.%s(", assignOp, dt.config.StoreVar, methodName))

	// Add arguments from WHERE clause
	whereFields := dt.extractWhereFields(s)
	var argList []string
	for _, wf := range whereFields {
		argList = append(argList, wf.variable)
	}
	out.WriteString(strings.Join(argList, ", "))
	out.WriteString(")\n")
	out.WriteString(dt.indentStr())
	out.WriteString("if err != nil {\n")
	out.WriteString(dt.indentStr())
	out.WriteString("\t" + dt.buildErrorReturn() + "\n")
	out.WriteString(dt.indentStr())
	out.WriteString("}\n")
	dt.emitResultHandling(&out, "")
	
	// Check if this is a SELECT INTO variable assignment
	assignments := dt.extractSelectAssignments(s)
	if len(assignments) > 0 {
		// Generate assignments from result
		// For mock backend, we assume result has fields matching the column names
		for _, a := range assignments {
			out.WriteString(dt.indentStr())
			// Use the column name as the field accessor on result
			fieldName := goExportedIdentifier(a.column)
			out.WriteString(fmt.Sprintf("%s = result.%s\n", a.varName, fieldName))
		}
	}

	return out.String(), nil
}

// transpileSelectInline generates inline SQL string.
func (dt *dmlTranspiler) transpileSelectInline(s *ast.SelectStatement) (string, error) {
	query, args := dt.buildSelectQuery(s)
	
	// Post-process to catch any remaining @variable references
	query, extraArgs := dt.substituteVariablesInQuery(query)
	args = append(args, extraArgs...)

	var out strings.Builder
	out.WriteString(fmt.Sprintf("query := %q\n", query))
	out.WriteString(dt.indentStr())
	out.WriteString("args := []interface{}{")
	out.WriteString(strings.Join(args, ", "))
	out.WriteString("}\n")
	out.WriteString(dt.indentStr())
	out.WriteString("// Execute query with adapter")

	return out.String(), nil
}

// transpileInsert converts an INSERT statement to Go code.
func (t *transpiler) transpileInsert(s *ast.InsertStatement) (string, error) {
	dt := &dmlTranspiler{transpiler: t, config: t.dmlConfig}
	return dt.transpileInsert(s)
}

func (dt *dmlTranspiler) transpileInsert(s *ast.InsertStatement) (string, error) {
	// Determine effective backend (use fallback for temp tables)
	tableName := dt.extractInsertTable(s)
	backend := dt.getEffectiveBackend(tableName)
	
	switch backend {
	case BackendSQL:
		return dt.transpileInsertSQL(s)
	case BackendGRPC:
		return dt.transpileInsertGRPC(s)
	case BackendMock:
		return dt.transpileInsertMock(s)
	default:
		return dt.transpileInsertSQL(s)
	}
}

func (dt *dmlTranspiler) transpileInsertSQL(s *ast.InsertStatement) (string, error) {
	var out strings.Builder

	// Emit original SQL if requested
	if dt.emitOriginal() {
		out.WriteString(fmt.Sprintf("// Original: %s\n", truncateSQL(s.String(), 100)))
		out.WriteString(dt.indentStr())
	}

	query, args := dt.buildInsertQuery(s)
	
	// Post-process to catch any remaining @variable references
	query, extraArgs := dt.substituteVariablesInQuery(query)
	args = append(args, extraArgs...)
	
	// Get the database variable (tx if in transaction, StoreVar otherwise)
	dbVar := dt.getDBVar()

	// Check for OUTPUT clause (SQL Server) or RETURNING (PostgreSQL)
	hasOutput := s.Output != nil

	out.WriteString("// INSERT query\n")
	out.WriteString(dt.indentStr())

	if hasOutput && dt.config.SQLDialect == "postgres" {
		// PostgreSQL: use RETURNING
		if dt.emitTODOs() {
			out.WriteString("// TODO(tgpiler): OUTPUT clause converted to RETURNING - verify column mapping\n")
			out.WriteString(dt.indentStr())
		}
		out.WriteString(fmt.Sprintf("row := %s.QueryRowContext(ctx, %q", dbVar, query))
		for _, arg := range args {
			out.WriteString(", " + arg)
		}
		out.WriteString(")\n")
		out.WriteString(dt.indentStr())
		out.WriteString("if err := row.Scan(/* TODO: RETURNING columns */); err != nil {\n")
		out.WriteString(dt.indentStr())
		if dt.transpiler.inCatchBlock {
			// In CATCH block, just log and continue - don't return
			out.WriteString("\t_ = err // Error logging failed, but we're already in error handling\n")
		} else {
			out.WriteString("\t" + dt.buildErrorReturn() + "\n")
		}
		out.WriteString(dt.indentStr())
		out.WriteString("}")
	} else {
		// Standard INSERT - check if result/err already declared
		// Use := if either variable is new, = if both are already declared
		resultDeclared := dt.symbols.isDeclared("result")
		errDeclared := dt.symbols.isDeclared("err")
		
		assignOp := ":="
		if resultDeclared && errDeclared {
			assignOp = "="
		}
		dt.symbols.markDeclared("result")
		dt.symbols.markDeclared("err")
		
		out.WriteString(fmt.Sprintf("result, err %s %s.ExecContext(ctx, %q", assignOp, dbVar, query))
		for _, arg := range args {
			out.WriteString(", " + arg)
		}
		out.WriteString(")\n")
		out.WriteString(dt.indentStr())
		out.WriteString("if err != nil {\n")
		out.WriteString(dt.indentStr())
		if dt.transpiler.inCatchBlock {
			// In CATCH block, just log and continue - don't return
			// We're already in error handling, so failing to log is not critical
			out.WriteString("\t_ = err // Error logging failed, but we're already in error handling\n")
		} else {
			out.WriteString("\t")
			out.WriteString(dt.buildErrorReturn())
			out.WriteString("\n")
		}
		out.WriteString(dt.indentStr())
		out.WriteString("}\n")
		dt.emitResultHandling(&out, "Use result.LastInsertId() if needed")
	}

	return out.String(), nil
}

func (dt *dmlTranspiler) transpileInsertGRPC(s *ast.InsertStatement) (string, error) {
	tableName := dt.extractInsertTable(s)

	// Detect verb from INSERT columns/values
	insertFields := dt.extractInsertFields(s)
	methodName := dt.inferInsertGRPCMethod(tableName, insertFields)

	// Get client variable and proto package for this table
	clientVar := dt.getGRPCClientForTable(tableName)
	protoPackage := dt.getProtoPackageForTable(tableName)

	var out strings.Builder
	out.WriteString(fmt.Sprintf("// gRPC call: %s.%s\n", clientVar, methodName))
	out.WriteString(dt.indentStr())

	if protoPackage != "" {
		out.WriteString(fmt.Sprintf("resp, err := %s.%s(ctx, &%s.%sRequest{\n",
			clientVar, methodName, protoPackage, methodName))
	} else {
		out.WriteString(fmt.Sprintf("resp, err := %s.%s(ctx, &%sRequest{\n",
			clientVar, methodName, methodName))
	}

	// Add request fields from INSERT columns/values
	for _, f := range insertFields {
		out.WriteString(dt.indentStr())
		out.WriteString(fmt.Sprintf("\t%s: %s,\n", goExportedIdentifier(f.column), f.value))
	}

	out.WriteString(dt.indentStr())
	out.WriteString("})\n")
	out.WriteString(dt.indentStr())
	out.WriteString("if err != nil {\n")
	out.WriteString(dt.indentStr())
	out.WriteString("\t")
	out.WriteString(dt.buildErrorReturn())
	out.WriteString("\n")
	out.WriteString(dt.indentStr())
	out.WriteString("}\n")
	
	// Handle OUTPUT clause - extract returned values from response
	outputVars := dt.extractInsertOutputVars(s)
	if len(outputVars) > 0 {
		out.WriteString(dt.indentStr())
		out.WriteString("if resp != nil {\n")
		for _, ov := range outputVars {
			out.WriteString(dt.indentStr())
			// Map INSERTED.ColName to resp.ColName
			protoField := goExportedIdentifier(ov.column)
			out.WriteString(fmt.Sprintf("\t%s = resp.%s\n", ov.variable, protoField))
		}
		out.WriteString(dt.indentStr())
		out.WriteString("}")
	} else {
		out.WriteString(dt.indentStr())
		out.WriteString("_ = resp")
	}

	return out.String(), nil
}

// extractInsertOutputVars extracts variable assignments from OUTPUT clause.
// Handles patterns like: OUTPUT INSERTED.LogId INTO @NewId
func (dt *dmlTranspiler) extractInsertOutputVars(s *ast.InsertStatement) []struct{ column, variable string } {
	var outputs []struct{ column, variable string }
	
	if s.Output == nil {
		return outputs
	}
	
	// The Output clause contains columns like INSERTED.LogId
	// Check if Output has columns
	if s.Output.Columns != nil {
		for _, col := range s.Output.Columns {
			colName := ""
			// Try to extract column name from INSERTED.ColName pattern
			if qid, ok := col.Expression.(*ast.QualifiedIdentifier); ok && len(qid.Parts) >= 2 {
				// Last part is the column name (e.g., "LogId" from "INSERTED.LogId")
				colName = qid.Parts[len(qid.Parts)-1].Value
			} else if id, ok := col.Expression.(*ast.Identifier); ok {
				colName = id.Value
			}
			
			if colName != "" {
				// Use the column name as variable name (lowercase first letter)
				// The caller should have a variable declared with matching or similar name
				varName := goIdentifier(colName)
				
				outputs = append(outputs, struct{ column, variable string }{
					column:   colName,
					variable: varName,
				})
			}
		}
	}
	
	return outputs
}

// inferInsertGRPCMethod determines the gRPC method name for an INSERT statement.
func (dt *dmlTranspiler) inferInsertGRPCMethod(table string, fields []insertField) string {
	entityName := toPascalCase(singularize(table))
	
	// Check for verb hints in column/value names
	for _, f := range fields {
		if verb := extractActionVerb(f.column); verb != "" {
			if !verbConflictsWithEntity(verb, entityName) {
				return verb + entityName
			}
		}
		if verb := extractActionVerb(f.value); verb != "" {
			if !verbConflictsWithEntity(verb, entityName) {
				return verb + entityName
			}
		}
	}

	// Default to Create
	return "Create" + entityName
}

func (dt *dmlTranspiler) transpileInsertMock(s *ast.InsertStatement) (string, error) {
	tableName := dt.extractInsertTable(s)
	methodName := "Create" + toPascalCase(singularize(tableName))

	var out strings.Builder
	
	// Check if result and err are already declared
	resultDeclared := dt.symbols.isDeclared("result")
	errDeclared := dt.symbols.isDeclared("err")
	assignOp := ":="
	if resultDeclared && errDeclared {
		assignOp = "="
	}
	dt.symbols.markDeclared("result")
	dt.symbols.markDeclared("err")
	
	out.WriteString(fmt.Sprintf("result, err %s %s.%s(", assignOp, dt.config.StoreVar, methodName))

	insertFields := dt.extractInsertFields(s)
	var argList []string
	for _, f := range insertFields {
		argList = append(argList, f.value)
	}
	out.WriteString(strings.Join(argList, ", "))
	out.WriteString(")\n")
	out.WriteString(dt.indentStr())
	out.WriteString("if err != nil {\n")
	out.WriteString(dt.indentStr())
	out.WriteString("\t" + dt.buildErrorReturn() + "\n")
	out.WriteString(dt.indentStr())
	out.WriteString("}\n")
	dt.emitResultHandling(&out, "")

	return out.String(), nil
}

// transpileUpdate converts an UPDATE statement to Go code.
func (t *transpiler) transpileUpdate(s *ast.UpdateStatement) (string, error) {
	dt := &dmlTranspiler{transpiler: t, config: t.dmlConfig}
	return dt.transpileUpdate(s)
}

func (dt *dmlTranspiler) transpileUpdate(s *ast.UpdateStatement) (string, error) {
	// Determine effective backend (use fallback for temp tables)
	tableName := dt.extractUpdateTable(s)
	backend := dt.getEffectiveBackend(tableName)
	
	switch backend {
	case BackendSQL:
		return dt.transpileUpdateSQL(s)
	case BackendGRPC:
		return dt.transpileUpdateGRPC(s)
	case BackendMock:
		return dt.transpileUpdateMock(s)
	default:
		return dt.transpileUpdateSQL(s)
	}
}

func (dt *dmlTranspiler) transpileUpdateSQL(s *ast.UpdateStatement) (string, error) {
	var out strings.Builder

	// Emit original SQL if requested
	if dt.emitOriginal() {
		out.WriteString(fmt.Sprintf("// Original: %s\n", truncateSQL(s.String(), 100)))
		out.WriteString(dt.indentStr())
	}

	query, args := dt.buildUpdateQuery(s)
	
	// Post-process to catch any remaining @variable references
	query, extraArgs := dt.substituteVariablesInQuery(query)
	args = append(args, extraArgs...)
	
	// Get the database variable (tx if in transaction, StoreVar otherwise)
	dbVar := dt.getDBVar()

	// Check if result and err are already declared
	resultDeclared := dt.symbols.isDeclared("result")
	errDeclared := dt.symbols.isDeclared("err")
	assignOp := ":="
	if resultDeclared && errDeclared {
		assignOp = "="
	}
	dt.symbols.markDeclared("result")
	dt.symbols.markDeclared("err")

	out.WriteString("// UPDATE query\n")
	out.WriteString(dt.indentStr())
	out.WriteString(fmt.Sprintf("result, err %s %s.ExecContext(ctx, %q", assignOp, dbVar, query))
	for _, arg := range args {
		out.WriteString(", " + arg)
	}
	out.WriteString(")\n")
	out.WriteString(dt.indentStr())
	out.WriteString("if err != nil {\n")
	out.WriteString(dt.indentStr())
	out.WriteString("\t" + dt.buildErrorReturn() + "\n")
	out.WriteString(dt.indentStr())
	out.WriteString("}\n")
	dt.emitResultHandling(&out, "Use result.RowsAffected() if needed")

	return out.String(), nil
}

func (dt *dmlTranspiler) transpileUpdateGRPC(s *ast.UpdateStatement) (string, error) {
	tableName := dt.extractUpdateTable(s)

	// Extract SET and WHERE fields for verb detection
	setFields := dt.extractUpdateSetFields(s)
	whereFields := dt.extractWhereFieldsFromUpdate(s)
	methodName := dt.inferUpdateGRPCMethod(tableName, setFields, whereFields)

	// Get client variable and proto package for this table
	clientVar := dt.getGRPCClientForTable(tableName)
	protoPackage := dt.getProtoPackageForTable(tableName)

	var out strings.Builder
	out.WriteString(fmt.Sprintf("// gRPC call: %s.%s\n", clientVar, methodName))
	out.WriteString(dt.indentStr())

	if protoPackage != "" {
		out.WriteString(fmt.Sprintf("resp, err := %s.%s(ctx, &%s.%sRequest{\n",
			clientVar, methodName, protoPackage, methodName))
	} else {
		out.WriteString(fmt.Sprintf("resp, err := %s.%s(ctx, &%sRequest{\n",
			clientVar, methodName, methodName))
	}

	// Add SET fields
	for _, f := range setFields {
		out.WriteString(dt.indentStr())
		out.WriteString(fmt.Sprintf("\t%s: %s,\n", goExportedIdentifier(f.column), f.value))
	}

	// Add WHERE fields (for identifying the record)
	for _, wf := range whereFields {
		out.WriteString(dt.indentStr())
		out.WriteString(fmt.Sprintf("\t%s: %s,\n", goExportedIdentifier(wf.column), wf.variable))
	}

	out.WriteString(dt.indentStr())
	out.WriteString("})\n")
	out.WriteString(dt.indentStr())
	out.WriteString("if err != nil {\n")
	out.WriteString(dt.indentStr())
	out.WriteString("\t")
	out.WriteString(dt.buildErrorReturn())
	out.WriteString("\n")
	out.WriteString(dt.indentStr())
	out.WriteString("}\n")
	out.WriteString(dt.indentStr())
	out.WriteString("_ = resp")

	return out.String(), nil
}

// inferUpdateGRPCMethod determines the gRPC method name for an UPDATE statement.
// This is where state transition verbs (Approve, Reject, Suspend, etc.) are most important.
func (dt *dmlTranspiler) inferUpdateGRPCMethod(table string, setFields []setField, whereFields []whereField) string {
	entityName := toPascalCase(singularize(table))
	
	// Check SET columns for state transition verbs
	// e.g., UPDATE Orders SET ApprovalStatus = 'Approved' → ApproveOrder
	for _, f := range setFields {
		// Check column name
		if verb := extractActionVerb(f.column); verb != "" {
			// Skip if verb would duplicate or is a prefix of entity name
			// e.g., Transfer + Transfer, Transfer + TransferAccounting
			if !verbConflictsWithEntity(verb, entityName) {
				return verb + entityName
			}
		}
		// Check value for state indicators (e.g., 'Approved', 'Rejected', 'Suspended')
		if verb := extractActionVerb(f.value); verb != "" {
			if !verbConflictsWithEntity(verb, entityName) {
				return verb + entityName
			}
		}
	}

	// Check WHERE clause for verb hints
	for _, wf := range whereFields {
		if verb := extractActionVerb(wf.column); verb != "" {
			if !verbConflictsWithEntity(verb, entityName) {
				return verb + entityName
			}
		}
		if verb := extractActionVerb(wf.variable); verb != "" {
			if !verbConflictsWithEntity(verb, entityName) {
				return verb + entityName
			}
		}
	}

	// Default to Update
	return "Update" + entityName
}

// verbConflictsWithEntity returns true if using the verb would create a redundant
// or confusing method name. This happens when:
// - verb equals entity (Transfer + Transfer = TransferTransfer)
// - entity starts with verb (Transfer + TransferAccounting = TransferTransferAccounting)
func verbConflictsWithEntity(verb, entity string) bool {
	verbLower := strings.ToLower(verb)
	entityLower := strings.ToLower(entity)
	return verbLower == entityLower || strings.HasPrefix(entityLower, verbLower)
}

func (dt *dmlTranspiler) transpileUpdateMock(s *ast.UpdateStatement) (string, error) {
	tableName := dt.extractUpdateTable(s)
	methodName := "Update" + toPascalCase(singularize(tableName))

	var out strings.Builder
	
	// Check if err is already declared
	errDeclared := dt.symbols.isDeclared("err")
	assignOp := ":="
	if errDeclared {
		assignOp = "="
	}
	dt.symbols.markDeclared("err")
	
	out.WriteString(fmt.Sprintf("err %s %s.%s(", assignOp, dt.config.StoreVar, methodName))

	// Combine SET and WHERE fields
	var argList []string
	whereFields := dt.extractWhereFieldsFromUpdate(s)
	for _, wf := range whereFields {
		argList = append(argList, wf.variable)
	}
	setFields := dt.extractUpdateSetFields(s)
	for _, f := range setFields {
		argList = append(argList, f.value)
	}

	out.WriteString(strings.Join(argList, ", "))
	out.WriteString(")\n")
	out.WriteString(dt.indentStr())
	out.WriteString("if err != nil {\n")
	out.WriteString(dt.indentStr())
	out.WriteString("\t" + dt.buildErrorReturn() + "\n")
	out.WriteString(dt.indentStr())
	out.WriteString("}")

	return out.String(), nil
}

// transpileDelete converts a DELETE statement to Go code.
func (t *transpiler) transpileDelete(s *ast.DeleteStatement) (string, error) {
	dt := &dmlTranspiler{transpiler: t, config: t.dmlConfig}
	return dt.transpileDelete(s)
}

func (dt *dmlTranspiler) transpileDelete(s *ast.DeleteStatement) (string, error) {
	// Determine effective backend (use fallback for temp tables)
	tableName := dt.extractDeleteTable(s)
	backend := dt.getEffectiveBackend(tableName)
	
	switch backend {
	case BackendSQL:
		return dt.transpileDeleteSQL(s)
	case BackendGRPC:
		return dt.transpileDeleteGRPC(s)
	case BackendMock:
		return dt.transpileDeleteMock(s)
	default:
		return dt.transpileDeleteSQL(s)
	}
}

func (dt *dmlTranspiler) transpileDeleteSQL(s *ast.DeleteStatement) (string, error) {
	var out strings.Builder

	// Emit original SQL if requested
	if dt.emitOriginal() {
		out.WriteString(fmt.Sprintf("// Original: %s\n", truncateSQL(s.String(), 100)))
		out.WriteString(dt.indentStr())
	}

	query, args := dt.buildDeleteQuery(s)
	
	// Post-process to catch any remaining @variable references
	query, extraArgs := dt.substituteVariablesInQuery(query)
	args = append(args, extraArgs...)
	
	// Get the database variable (tx if in transaction, StoreVar otherwise)
	dbVar := dt.getDBVar()

	// Check if result and err are already declared
	resultDeclared := dt.symbols.isDeclared("result")
	errDeclared := dt.symbols.isDeclared("err")
	assignOp := ":="
	if resultDeclared && errDeclared {
		assignOp = "="
	}
	dt.symbols.markDeclared("result")
	dt.symbols.markDeclared("err")

	out.WriteString("// DELETE query\n")
	out.WriteString(dt.indentStr())
	out.WriteString(fmt.Sprintf("result, err %s %s.ExecContext(ctx, %q", assignOp, dbVar, query))
	for _, arg := range args {
		out.WriteString(", " + arg)
	}
	out.WriteString(")\n")
	out.WriteString(dt.indentStr())
	out.WriteString("if err != nil {\n")
	out.WriteString(dt.indentStr())
	out.WriteString("\t" + dt.buildErrorReturn() + "\n")
	out.WriteString(dt.indentStr())
	out.WriteString("}\n")
	dt.emitResultHandling(&out, "Use result.RowsAffected() if needed")

	return out.String(), nil
}

func (dt *dmlTranspiler) transpileDeleteGRPC(s *ast.DeleteStatement) (string, error) {
	tableName := dt.extractDeleteTable(s)

	// Extract WHERE fields for verb detection
	whereFields := dt.extractWhereFieldsFromDelete(s)
	methodName := dt.inferDeleteGRPCMethod(tableName, whereFields)

	// Get client variable and proto package for this table
	clientVar := dt.getGRPCClientForTable(tableName)
	protoPackage := dt.getProtoPackageForTable(tableName)

	var out strings.Builder
	out.WriteString(fmt.Sprintf("// gRPC call: %s.%s\n", clientVar, methodName))
	out.WriteString(dt.indentStr())

	if protoPackage != "" {
		out.WriteString(fmt.Sprintf("resp, err := %s.%s(ctx, &%s.%sRequest{\n",
			clientVar, methodName, protoPackage, methodName))
	} else {
		out.WriteString(fmt.Sprintf("resp, err := %s.%s(ctx, &%sRequest{\n",
			clientVar, methodName, methodName))
	}

	// Add WHERE fields (for identifying the record)
	for _, wf := range whereFields {
		out.WriteString(dt.indentStr())
		out.WriteString(fmt.Sprintf("\t%s: %s,\n", goExportedIdentifier(wf.column), wf.variable))
	}

	out.WriteString(dt.indentStr())
	out.WriteString("})\n")
	out.WriteString(dt.indentStr())
	out.WriteString("if err != nil {\n")
	out.WriteString(dt.indentStr())
	out.WriteString("\t")
	out.WriteString(dt.buildErrorReturn())
	out.WriteString("\n")
	out.WriteString(dt.indentStr())
	out.WriteString("}\n")
	out.WriteString(dt.indentStr())
	out.WriteString("_ = resp")

	return out.String(), nil
}

// inferDeleteGRPCMethod determines the gRPC method name for a DELETE statement.
// Detects verbs like Cancel, Revoke, Terminate, Remove, Purge, etc.
func (dt *dmlTranspiler) inferDeleteGRPCMethod(table string, whereFields []whereField) string {
	entityName := toPascalCase(singularize(table))
	
	// Check WHERE clause for verb hints
	for _, wf := range whereFields {
		if verb := extractActionVerb(wf.column); verb != "" {
			if !verbConflictsWithEntity(verb, entityName) {
				return verb + entityName
			}
		}
		if verb := extractActionVerb(wf.variable); verb != "" {
			if !verbConflictsWithEntity(verb, entityName) {
				return verb + entityName
			}
		}
	}

	// Default to Delete
	return "Delete" + entityName
}

func (dt *dmlTranspiler) transpileDeleteMock(s *ast.DeleteStatement) (string, error) {
	tableName := dt.extractDeleteTable(s)
	methodName := "Delete" + toPascalCase(singularize(tableName))

	var out strings.Builder
	out.WriteString(fmt.Sprintf("err := %s.%s(", dt.config.StoreVar, methodName))

	whereFields := dt.extractWhereFieldsFromDelete(s)
	var argList []string
	for _, wf := range whereFields {
		argList = append(argList, wf.variable)
	}

	out.WriteString(strings.Join(argList, ", "))
	out.WriteString(")\n")
	out.WriteString(dt.indentStr())
	out.WriteString("if err != nil {\n")
	out.WriteString(dt.indentStr())
	out.WriteString("\t" + dt.buildErrorReturn() + "\n")
	out.WriteString(dt.indentStr())
	out.WriteString("}")

	return out.String(), nil
}

// transpileWithStatement converts a WITH (CTE) statement to Go code.
// CTEs are passed through to the database which handles them natively.
func (t *transpiler) transpileWithStatement(s *ast.WithStatement) (string, error) {
	dt := &dmlTranspiler{transpiler: t, config: t.dmlConfig}
	return dt.transpileWithStatement(s)
}

func (dt *dmlTranspiler) transpileWithStatement(s *ast.WithStatement) (string, error) {
	// The inner query determines how we handle this
	switch inner := s.Query.(type) {
	case *ast.SelectStatement:
		return dt.transpileWithSelect(s, inner)
	case *ast.InsertStatement:
		return dt.transpileWithInsert(s, inner)
	case *ast.UpdateStatement:
		return dt.transpileWithUpdate(s, inner)
	case *ast.DeleteStatement:
		return dt.transpileWithDelete(s, inner)
	default:
		return "", fmt.Errorf("unsupported query type in WITH statement: %T", s.Query)
	}
}

// transpileWithSelect handles WITH ... SELECT
func (dt *dmlTranspiler) transpileWithSelect(ws *ast.WithStatement, sel *ast.SelectStatement) (string, error) {
	var out strings.Builder

	// Build the full CTE query and strip table hints
	query := stripTableHints(ws.String())
	
	// Convert @variable references to parameter placeholders
	query, args := dt.substituteVariablesInQuery(query)
	
	// Get the database variable
	dbVar := dt.getDBVar()
	
	// Check if this is a SELECT INTO variable assignment
	assignments := dt.extractSelectAssignments(sel)
	if len(assignments) > 0 {
		return dt.transpileWithSelectIntoVars(ws, sel, assignments, query, args)
	}
	
	// Extract column names from the main SELECT for scan targets
	columns := dt.extractSelectColumns(sel)
	scanDecl, scanTargets := dt.generateScanTargets(columns)

	// Generate CTE names for comment
	cteNames := make([]string, len(ws.CTEs))
	for i, cte := range ws.CTEs {
		if cte.Name != nil {
			cteNames[i] = cte.Name.Value
		}
	}

	// Generate the Go code
	out.WriteString(fmt.Sprintf("// WITH %s - CTE query\n", strings.Join(cteNames, ", ")))
	out.WriteString(dt.indentStr())
	
	// Generate variable declarations for scan targets
	if scanDecl != "" {
		out.WriteString(scanDecl)
		out.WriteString("\n")
		out.WriteString(dt.indentStr())
	}

	if dt.isSingleRowSelect(sel) {
		// Use QueryRow for single-row SELECT
		out.WriteString(fmt.Sprintf("row := %s.QueryRowContext(ctx, %q", dbVar, query))
		for _, arg := range args {
			out.WriteString(", " + arg)
		}
		out.WriteString(")\n")
		out.WriteString(dt.indentStr())
		out.WriteString(fmt.Sprintf("if err := row.Scan(%s); err != nil {\n", scanTargets))
		out.WriteString(dt.indentStr())
		out.WriteString("\t")
		out.WriteString(dt.buildErrorReturn())
		out.WriteString("\n")
		out.WriteString(dt.indentStr())
		out.WriteString("}")
	} else {
		// Use Query for multi-row SELECT
		rowsDeclared := dt.symbols.isDeclared("rows")
		errDeclared := dt.symbols.isDeclared("err")
		
		assignOp := ":="
		if rowsDeclared && errDeclared {
			assignOp = "="
		}
		dt.symbols.markDeclared("rows")
		dt.symbols.markDeclared("err")
		
		out.WriteString(fmt.Sprintf("rows, err %s %s.QueryContext(ctx, %q", assignOp, dbVar, query))
		for _, arg := range args {
			out.WriteString(", " + arg)
		}
		out.WriteString(")\n")
		out.WriteString(dt.indentStr())
		out.WriteString("if err != nil {\n")
		out.WriteString(dt.indentStr())
		out.WriteString("\t")
		out.WriteString(dt.buildErrorReturn())
		out.WriteString("\n")
		out.WriteString(dt.indentStr())
		out.WriteString("}\n")
		out.WriteString(dt.indentStr())
		out.WriteString("defer rows.Close()\n")
		out.WriteString(dt.indentStr())
		out.WriteString("for rows.Next() {\n")
		out.WriteString(dt.indentStr())
		out.WriteString(fmt.Sprintf("\tif err := rows.Scan(%s); err != nil {\n", scanTargets))
		out.WriteString(dt.indentStr())
		out.WriteString("\t\t")
		out.WriteString(dt.buildErrorReturn())
		out.WriteString("\n")
		out.WriteString(dt.indentStr())
		out.WriteString("\t}\n")
		out.WriteString(dt.indentStr())
		out.WriteString("}")
	}

	return out.String(), nil
}

// transpileWithSelectIntoVars handles WITH ... SELECT @var = col pattern
func (dt *dmlTranspiler) transpileWithSelectIntoVars(ws *ast.WithStatement, sel *ast.SelectStatement, assignments []varAssignment, query string, args []string) (string, error) {
	var out strings.Builder
	
	// This function uses sql.ErrNoRows
	dt.imports["database/sql"] = true
	
	// Get the database variable
	dbVar := dt.getDBVar()
	
	// Build scan targets from assignments
	var scanTargets []string
	for _, a := range assignments {
		scanTargets = append(scanTargets, "&"+a.varName)
	}

	// Generate CTE names for comment
	cteNames := make([]string, len(ws.CTEs))
	for i, cte := range ws.CTEs {
		if cte.Name != nil {
			cteNames[i] = cte.Name.Value
		}
	}

	out.WriteString(fmt.Sprintf("// WITH %s - CTE SELECT INTO variables\n", strings.Join(cteNames, ", ")))
	out.WriteString(dt.indentStr())
	out.WriteString(fmt.Sprintf("row := %s.QueryRowContext(ctx, %q", dbVar, query))
	for _, arg := range args {
		out.WriteString(", " + arg)
	}
	out.WriteString(")\n")
	out.WriteString(dt.indentStr())
	out.WriteString(fmt.Sprintf("if err := row.Scan(%s); err != nil {\n", strings.Join(scanTargets, ", ")))
	out.WriteString(dt.indentStr())
	out.WriteString("\tif err != sql.ErrNoRows {\n")
	out.WriteString(dt.indentStr())
	out.WriteString("\t\t")
	out.WriteString(dt.buildErrorReturn())
	out.WriteString("\n")
	out.WriteString(dt.indentStr())
	out.WriteString("\t}\n")
	out.WriteString(dt.indentStr())
	out.WriteString("}")

	return out.String(), nil
}

// transpileWithInsert handles WITH ... INSERT
func (dt *dmlTranspiler) transpileWithInsert(ws *ast.WithStatement, ins *ast.InsertStatement) (string, error) {
	var out strings.Builder

	// Build the full CTE query and strip table hints
	query := stripTableHints(ws.String())
	
	// Convert @variable references to parameter placeholders
	query, args := dt.substituteVariablesInQuery(query)
	
	// Get the database variable
	dbVar := dt.getDBVar()

	// Generate CTE names for comment
	cteNames := make([]string, len(ws.CTEs))
	for i, cte := range ws.CTEs {
		if cte.Name != nil {
			cteNames[i] = cte.Name.Value
		}
	}

	// Check if result/err already declared
	resultDeclared := dt.symbols.isDeclared("result")
	errDeclared := dt.symbols.isDeclared("err")
	
	assignOp := ":="
	if resultDeclared && errDeclared {
		assignOp = "="
	}
	dt.symbols.markDeclared("result")
	dt.symbols.markDeclared("err")

	out.WriteString(fmt.Sprintf("// WITH %s - CTE INSERT\n", strings.Join(cteNames, ", ")))
	out.WriteString(dt.indentStr())
	out.WriteString(fmt.Sprintf("result, err %s %s.ExecContext(ctx, %q", assignOp, dbVar, query))
	for _, arg := range args {
		out.WriteString(", " + arg)
	}
	out.WriteString(")\n")
	out.WriteString(dt.indentStr())
	out.WriteString("if err != nil {\n")
	out.WriteString(dt.indentStr())
	out.WriteString("\t")
	out.WriteString(dt.buildErrorReturn())
	out.WriteString("\n")
	out.WriteString(dt.indentStr())
	out.WriteString("}\n")
	dt.emitResultHandling(&out, "")

	return out.String(), nil
}

// transpileWithUpdate handles WITH ... UPDATE
func (dt *dmlTranspiler) transpileWithUpdate(ws *ast.WithStatement, upd *ast.UpdateStatement) (string, error) {
	var out strings.Builder

	// Build the full CTE query and strip table hints
	query := stripTableHints(ws.String())
	
	// Convert @variable references to parameter placeholders
	query, args := dt.substituteVariablesInQuery(query)
	
	// Get the database variable
	dbVar := dt.getDBVar()

	// Generate CTE names for comment
	cteNames := make([]string, len(ws.CTEs))
	for i, cte := range ws.CTEs {
		if cte.Name != nil {
			cteNames[i] = cte.Name.Value
		}
	}

	// Check if result/err already declared
	resultDeclared := dt.symbols.isDeclared("result")
	errDeclared := dt.symbols.isDeclared("err")
	
	assignOp := ":="
	if resultDeclared && errDeclared {
		assignOp = "="
	}
	dt.symbols.markDeclared("result")
	dt.symbols.markDeclared("err")

	out.WriteString(fmt.Sprintf("// WITH %s - CTE UPDATE\n", strings.Join(cteNames, ", ")))
	out.WriteString(dt.indentStr())
	out.WriteString(fmt.Sprintf("result, err %s %s.ExecContext(ctx, %q", assignOp, dbVar, query))
	for _, arg := range args {
		out.WriteString(", " + arg)
	}
	out.WriteString(")\n")
	out.WriteString(dt.indentStr())
	out.WriteString("if err != nil {\n")
	out.WriteString(dt.indentStr())
	out.WriteString("\t")
	out.WriteString(dt.buildErrorReturn())
	out.WriteString("\n")
	out.WriteString(dt.indentStr())
	out.WriteString("}\n")
	dt.emitResultHandling(&out, "")

	return out.String(), nil
}

// transpileWithDelete handles WITH ... DELETE
func (dt *dmlTranspiler) transpileWithDelete(ws *ast.WithStatement, del *ast.DeleteStatement) (string, error) {
	var out strings.Builder

	// Build the full CTE query and strip table hints
	query := stripTableHints(ws.String())
	
	// Convert @variable references to parameter placeholders
	query, args := dt.substituteVariablesInQuery(query)
	
	// Get the database variable
	dbVar := dt.getDBVar()

	// Generate CTE names for comment
	cteNames := make([]string, len(ws.CTEs))
	for i, cte := range ws.CTEs {
		if cte.Name != nil {
			cteNames[i] = cte.Name.Value
		}
	}

	// Check if result/err already declared
	resultDeclared := dt.symbols.isDeclared("result")
	errDeclared := dt.symbols.isDeclared("err")
	
	assignOp := ":="
	if resultDeclared && errDeclared {
		assignOp = "="
	}
	dt.symbols.markDeclared("result")
	dt.symbols.markDeclared("err")

	out.WriteString(fmt.Sprintf("// WITH %s - CTE DELETE\n", strings.Join(cteNames, ", ")))
	out.WriteString(dt.indentStr())
	out.WriteString(fmt.Sprintf("result, err %s %s.ExecContext(ctx, %q", assignOp, dbVar, query))
	for _, arg := range args {
		out.WriteString(", " + arg)
	}
	out.WriteString(")\n")
	out.WriteString(dt.indentStr())
	out.WriteString("if err != nil {\n")
	out.WriteString(dt.indentStr())
	out.WriteString("\t")
	out.WriteString(dt.buildErrorReturn())
	out.WriteString("\n")
	out.WriteString(dt.indentStr())
	out.WriteString("}\n")
	dt.emitResultHandling(&out, "")

	return out.String(), nil
}

// substituteVariablesInQuery replaces @variable references with parameter placeholders
func (dt *dmlTranspiler) substituteVariablesInQuery(query string) (string, []string) {
	var args []string
	var result strings.Builder
	paramIndex := 1 // Start at 1 for the existing getPlaceholder
	
	pos := 0
	for pos < len(query) {
		if query[pos] == '@' && pos+1 < len(query) {
			// Skip @@global variables
			if query[pos+1] == '@' {
				result.WriteByte(query[pos])
				pos++
				continue
			}
			
			// Check if this is a valid variable start
			if isAlphaForCTE(query[pos+1]) || query[pos+1] == '_' {
				// Find variable name
				end := pos + 1
				for end < len(query) && (isAlphaNumForCTE(query[end]) || query[end] == '_') {
					end++
				}
				
				varName := query[pos+1 : end]
				goVar := goIdentifier(varName)
				
				// Generate placeholder based on dialect
				placeholder := dt.getPlaceholder(paramIndex)
				result.WriteString(placeholder)
				args = append(args, goVar)
				paramIndex++
				
				pos = end
				continue
			}
		}
		result.WriteByte(query[pos])
		pos++
	}
	
	// Apply dialect-specific SQL normalization
	finalQuery := dt.normalizeDialectSQL(result.String())
	
	return finalQuery, args
}

// normalizeDialectSQL converts T-SQL specific syntax to target dialect
func (dt *dmlTranspiler) normalizeDialectSQL(query string) string {
	if dt.config.SQLDialect == "postgres" {
		// ISNULL(x, y) -> COALESCE(x, y)
		query = strings.ReplaceAll(query, "ISNULL(", "COALESCE(")
		query = strings.ReplaceAll(query, "isnull(", "COALESCE(")
		query = strings.ReplaceAll(query, "Isnull(", "COALESCE(")
		query = strings.ReplaceAll(query, "IsNull(", "COALESCE(")
		
		// GETDATE() -> NOW()
		query = strings.ReplaceAll(query, "GETDATE()", "NOW()")
		query = strings.ReplaceAll(query, "getdate()", "NOW()")
		query = strings.ReplaceAll(query, "GetDate()", "NOW()")
		
		// LEN(x) -> LENGTH(x)
		query = strings.ReplaceAll(query, "LEN(", "LENGTH(")
		query = strings.ReplaceAll(query, "len(", "LENGTH(")
		query = strings.ReplaceAll(query, "Len(", "LENGTH(")
	}
	return query
}

// isAlphaForCTE checks if a character is alphabetic
func isAlphaForCTE(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// isAlphaNumForCTE checks if a character is alphanumeric
func isAlphaNumForCTE(c byte) bool {
	return isAlphaForCTE(c) || (c >= '0' && c <= '9')
}

// transpileExec converts an EXEC/EXECUTE statement to a Go function call.
func (t *transpiler) transpileExec(s *ast.ExecStatement) (string, error) {
	dt := &dmlTranspiler{transpiler: t, config: t.dmlConfig}
	return dt.transpileExec(s)
}

func (dt *dmlTranspiler) transpileExec(s *ast.ExecStatement) (string, error) {
	// EXEC calls another stored procedure
	procName := ""
	if s.Procedure != nil {
		procName = s.Procedure.String()
	}

	// Clean up procedure name (remove dbo. prefix, etc.)
	procName = cleanProcedureName(procName)

	// Check if gRPC backend with explicit mapping
	if dt.config.Backend == BackendGRPC {
		if mapping, ok := dt.lookupGRPCMapping(procName); ok {
			return dt.transpileExecGRPC(s, procName, mapping)
		}
		// Even without explicit mapping, try to infer gRPC method
		if dt.config.ProtoPackage != "" || len(dt.config.TableToService) > 0 {
			return dt.transpileExecGRPCInferred(s, procName)
		}
	}

	// Default: generate Go function call
	return dt.transpileExecFunction(s, procName)
}

// lookupGRPCMapping checks GRPCMappings for a procedure name.
// Tries various name formats: exact, without prefix, normalized.
func (dt *dmlTranspiler) lookupGRPCMapping(procName string) (string, bool) {
	if dt.config.GRPCMappings == nil {
		return "", false
	}

	// Try exact match
	if mapping, ok := dt.config.GRPCMappings[procName]; ok {
		return mapping, true
	}

	// Try without common prefixes
	normalized := procName
	for _, prefix := range []string{"usp_", "sp_", "proc_", "p_", "dbo."} {
		normalized = strings.TrimPrefix(strings.ToLower(normalized), prefix)
	}

	for key, mapping := range dt.config.GRPCMappings {
		keyNorm := key
		for _, prefix := range []string{"usp_", "sp_", "proc_", "p_", "dbo."} {
			keyNorm = strings.TrimPrefix(strings.ToLower(keyNorm), prefix)
		}
		if keyNorm == normalized {
			return mapping, true
		}
	}

	return "", false
}

// transpileExecGRPC generates a gRPC call for EXEC with explicit mapping.
func (dt *dmlTranspiler) transpileExecGRPC(s *ast.ExecStatement, procName, mapping string) (string, error) {
	// Parse mapping: "ServiceName.MethodName" or just "MethodName"
	parts := strings.Split(mapping, ".")
	var serviceName, methodName string
	if len(parts) == 2 {
		serviceName = parts[0]
		methodName = parts[1]
	} else {
		methodName = mapping
	}

	// Determine client variable
	clientVar := dt.config.GRPCClientVar
	if clientVar == "" || clientVar == "client" {
		clientVar = dt.config.StoreVar
	}
	if serviceName != "" {
		// Use service-specific client
		clientVar = toLowerCamel(serviceName) + "Client"
	}

	// Determine proto package
	protoPackage := dt.config.ProtoPackage
	if serviceName != "" {
		// First check explicit service-to-package mapping
		if dt.config.ServiceToPackage != nil {
			if pkg, ok := dt.config.ServiceToPackage[serviceName]; ok {
				protoPackage = pkg
			} else {
				// Infer from service name
				protoPackage = inferProtoPackage(serviceName)
			}
		} else {
			// Infer from service name
			protoPackage = inferProtoPackage(serviceName)
		}
	}

	var out strings.Builder
	out.WriteString(fmt.Sprintf("// EXEC %s -> gRPC %s\n", procName, mapping))
	out.WriteString(dt.indentStr())

	// Build request struct
	if protoPackage != "" {
		out.WriteString(fmt.Sprintf("resp, err := %s.%s(ctx, &%s.%sRequest{\n",
			clientVar, methodName, protoPackage, methodName))
	} else {
		out.WriteString(fmt.Sprintf("resp, err := %s.%s(ctx, &%sRequest{\n",
			clientVar, methodName, methodName))
	}

	// Add parameters as request fields
	for _, p := range s.Parameters {
		fieldName := ""
		if p.Name != "" {
			fieldName = goExportedIdentifier(strings.TrimPrefix(p.Name, "@"))
		}
		argVal, err := dt.transpileExpression(p.Value)
		if err != nil {
			return "", err
		}
		if fieldName != "" {
			out.WriteString(dt.indentStr())
			out.WriteString(fmt.Sprintf("\t%s: %s,\n", fieldName, argVal))
		}
	}

	out.WriteString(dt.indentStr())
	out.WriteString("})\n")
	out.WriteString(dt.indentStr())
	out.WriteString("if err != nil {\n")
	out.WriteString(dt.indentStr())
	out.WriteString("\t")
	out.WriteString(dt.buildErrorReturn())
	out.WriteString("\n")
	out.WriteString(dt.indentStr())
	out.WriteString("}\n")
	out.WriteString(dt.indentStr())
	out.WriteString("_ = resp // TODO: use response")

	return out.String(), nil
}

// transpileExecGRPCInferred generates a gRPC call by inferring method from procedure name.
func (dt *dmlTranspiler) transpileExecGRPCInferred(s *ast.ExecStatement, procName string) (string, error) {
	// Infer method name from procedure name using verb detection
	methodName := dt.inferMethodFromProcedure(procName)

	clientVar := dt.config.GRPCClientVar
	if clientVar == "" || clientVar == "client" {
		clientVar = dt.config.StoreVar
	}

	protoPackage := dt.config.ProtoPackage

	var out strings.Builder
	out.WriteString(fmt.Sprintf("// EXEC %s -> gRPC %s (inferred)\n", procName, methodName))
	out.WriteString(dt.indentStr())

	if protoPackage != "" {
		out.WriteString(fmt.Sprintf("resp, err := %s.%s(ctx, &%s.%sRequest{\n",
			clientVar, methodName, protoPackage, methodName))
	} else {
		out.WriteString(fmt.Sprintf("resp, err := %s.%s(ctx, &%sRequest{\n",
			clientVar, methodName, methodName))
	}

	// Add parameters as request fields
	for _, p := range s.Parameters {
		fieldName := ""
		if p.Name != "" {
			fieldName = goExportedIdentifier(strings.TrimPrefix(p.Name, "@"))
		}
		argVal, err := dt.transpileExpression(p.Value)
		if err != nil {
			return "", err
		}
		if fieldName != "" {
			out.WriteString(dt.indentStr())
			out.WriteString(fmt.Sprintf("\t%s: %s,\n", fieldName, argVal))
		}
	}

	out.WriteString(dt.indentStr())
	out.WriteString("})\n")
	out.WriteString(dt.indentStr())
	out.WriteString("if err != nil {\n")
	out.WriteString(dt.indentStr())
	out.WriteString("\t")
	out.WriteString(dt.buildErrorReturn())
	out.WriteString("\n")
	out.WriteString(dt.indentStr())
	out.WriteString("}\n")
	out.WriteString(dt.indentStr())
	out.WriteString("_ = resp // TODO: use response")

	return out.String(), nil
}

// inferMethodFromProcedure infers a gRPC method name from a stored procedure name.
func (dt *dmlTranspiler) inferMethodFromProcedure(procName string) string {
	// Remove common prefixes
	name := procName
	for _, prefix := range []string{"usp_", "sp_", "proc_", "p_"} {
		if strings.HasPrefix(strings.ToLower(name), prefix) {
			name = name[len(prefix):]
			break
		}
	}

	// The procedure name likely already follows VerbEntity pattern
	// e.g., usp_GetProductById -> GetProductById -> GetProduct
	// e.g., usp_ApproveLoan -> ApproveLoan

	// Remove ById suffix if present
	if strings.HasSuffix(name, "ById") {
		name = strings.TrimSuffix(name, "ById")
	}

	return toPascalCase(name)
}

// transpileExecFunction generates a Go function call for EXEC (default behavior).
func (dt *dmlTranspiler) transpileExecFunction(s *ast.ExecStatement, procName string) (string, error) {
	funcName := goExportedIdentifier(procName)

	var out strings.Builder
	out.WriteString(fmt.Sprintf("// EXEC %s\n", procName))
	out.WriteString(dt.indentStr())

	// Check if result variable is assigned
	hasResultVar := s.ReturnVariable != nil

	// Generate the call
	if hasResultVar {
		// Build argument list (all params passed, OUTPUT ones get &)
		var args []string
		for _, p := range s.Parameters {
			argVal, err := dt.transpileExpression(p.Value)
			if err != nil {
				return "", err
			}
			if p.Output {
				argVal = "&" + argVal
			}
			args = append(args, argVal)
		}
		resultVar := goIdentifier(s.ReturnVariable.Value)
		out.WriteString(fmt.Sprintf("%s = %s(%s)", resultVar, funcName, strings.Join(args, ", ")))
	} else {
		// Check for OUTPUT params that need to capture return values
		var outputVars []string
		for _, p := range s.Parameters {
			if p.Output {
				varName := ""
				if v, ok := p.Value.(*ast.Variable); ok {
					// Don't call transpileExpression - just get the name without marking as used
					varName = goIdentifier(strings.TrimPrefix(v.Name, "@"))
				}
				if varName != "" {
					outputVars = append(outputVars, varName)
				}
			}
		}

		// Build non-output args (these ARE being read, so transpileExpression is correct)
		var callArgs []string
		for _, p := range s.Parameters {
			if !p.Output {
				argVal, _ := dt.transpileExpression(p.Value)
				callArgs = append(callArgs, argVal)
			}
		}

		if len(outputVars) > 0 {
			out.WriteString(strings.Join(outputVars, ", "))
			out.WriteString(" = ")
		}
		out.WriteString(fmt.Sprintf("%s(%s)", funcName, strings.Join(callArgs, ", ")))
	}

	return out.String(), nil
}

// Helper types and functions

type varAssignment struct {
	varName string
	column  string
}

type whereField struct {
	column   string
	variable string
	operator string
}

// whereFieldWithValue holds a WHERE condition that can be either a variable or literal
type whereFieldWithValue struct {
	column    string
	value     string // Go code for the value (variable name or literal)
	operator  string
	isComplex bool   // True if expression couldn't be converted to Go
	rawExpr   string // Original T-SQL for complex expressions
}

type insertField struct {
	column string
	value  string
}

type setField struct {
	column string
	value  string
}

// extractSelectAssignments extracts SELECT @var = col patterns.
func (dt *dmlTranspiler) extractSelectAssignments(s *ast.SelectStatement) []varAssignment {
	var assignments []varAssignment

	if s.Columns == nil {
		return assignments
	}

	for _, item := range s.Columns {
		// Check for @var = expr pattern
		if item.Variable != nil {
			varName := goIdentifier(strings.TrimPrefix(item.Variable.Name, "@"))
			colName := dt.exprToString(item.Expression)
			
			// For complex expressions (CASE, function calls, etc.), use the variable name
			// as a hint for the column name since mock results need simple field names
			if colName == "" || strings.Contains(colName, "(") || strings.Contains(colName, " ") {
				colName = varName
			}
			
			assignments = append(assignments, varAssignment{
				varName: varName,
				column:  colName,
			})
		}
	}

	return assignments
}

// extractWhereFields extracts fields from WHERE clause.
func (dt *dmlTranspiler) extractWhereFields(s *ast.SelectStatement) []whereField {
	var fields []whereField

	if s.Where == nil {
		return fields
	}

	dt.walkWhereExpr(s.Where, &fields)
	return fields
}

func (dt *dmlTranspiler) walkWhereExpr(expr ast.Expression, fields *[]whereField) {
	if expr == nil {
		return
	}

	switch e := expr.(type) {
	case *ast.InfixExpression:
		op := strings.ToUpper(e.Operator)
		if op == "AND" || op == "OR" {
			dt.walkWhereExpr(e.Left, fields)
			dt.walkWhereExpr(e.Right, fields)
			return
		}

		// Check for column = @variable pattern
		colName := ""
		varName := ""

		if id, ok := e.Left.(*ast.Identifier); ok {
			colName = id.Value
		} else if qid, ok := e.Left.(*ast.QualifiedIdentifier); ok {
			if len(qid.Parts) > 0 {
				colName = qid.Parts[len(qid.Parts)-1].Value
			}
		}

		if v, ok := e.Right.(*ast.Variable); ok {
			varName = goIdentifier(strings.TrimPrefix(v.Name, "@"))
		}

		if colName != "" && varName != "" {
			*fields = append(*fields, whereField{
				column:   colName,
				variable: varName,
				operator: op,
			})
		}
	}
}

// extractWhereFieldsWithLiterals extracts fields from WHERE clause including literals.
// This is used for gRPC request building where we want both variables and literal values.
func (dt *dmlTranspiler) extractWhereFieldsWithLiterals(s *ast.SelectStatement) []whereFieldWithValue {
	var fields []whereFieldWithValue

	if s.Where == nil {
		return fields
	}

	dt.walkWhereExprWithLiterals(s.Where, &fields)
	return fields
}

func (dt *dmlTranspiler) walkWhereExprWithLiterals(expr ast.Expression, fields *[]whereFieldWithValue) {
	if expr == nil {
		return
	}

	switch e := expr.(type) {
	case *ast.InfixExpression:
		op := strings.ToUpper(e.Operator)
		if op == "AND" || op == "OR" {
			dt.walkWhereExprWithLiterals(e.Left, fields)
			dt.walkWhereExprWithLiterals(e.Right, fields)
			return
		}

		// Extract column name from left side
		colName := ""
		if id, ok := e.Left.(*ast.Identifier); ok {
			colName = id.Value
		} else if qid, ok := e.Left.(*ast.QualifiedIdentifier); ok {
			if len(qid.Parts) > 0 {
				colName = qid.Parts[len(qid.Parts)-1].Value
			}
		}

		if colName == "" {
			return
		}

		// Extract value from right side - could be variable or literal
		var value string
		var isComplex bool
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
			// Could be TRUE/FALSE or a column reference
			upper := strings.ToUpper(v.Value)
			if upper == "TRUE" {
				value = "true"
			} else if upper == "FALSE" {
				value = "false"
			} else {
				value = goIdentifier(v.Value)
			}
		case *ast.FunctionCall:
			// Try to transpile simple functions, mark complex ones
			if goVal, ok := dt.tryTranspileSimpleFunc(v); ok {
				value = goVal
			} else {
				isComplex = true
			}
		default:
			// For other complex expressions, mark as needing manual handling
			isComplex = true
		}

		if value != "" && !isComplex {
			*fields = append(*fields, whereFieldWithValue{
				column:   colName,
				value:    value,
				operator: op,
			})
		} else if isComplex {
			// Add with complexity marker for proper handling
			*fields = append(*fields, whereFieldWithValue{
				column:    colName,
				value:     "",
				operator:  op,
				isComplex: true,
				rawExpr:   dt.exprToString(e.Right),
			})
		}
	}
}

// tryTranspileSimpleFunc attempts to convert simple T-SQL functions to Go.
// Returns the Go code and true if successful, empty and false otherwise.
func (dt *dmlTranspiler) tryTranspileSimpleFunc(f *ast.FunctionCall) (string, bool) {
	// Extract function name
	funcName := ""
	if id, ok := f.Function.(*ast.Identifier); ok {
		funcName = strings.ToUpper(id.Value)
	} else if qid, ok := f.Function.(*ast.QualifiedIdentifier); ok && len(qid.Parts) > 0 {
		funcName = strings.ToUpper(qid.Parts[len(qid.Parts)-1].Value)
	}
	if funcName == "" {
		return "", false
	}
	
	switch funcName {
	case "GETDATE", "GETUTCDATE", "SYSDATETIME", "SYSUTCDATETIME":
		dt.imports["time"] = true
		return "time.Now()", true
	case "NEWID":
		dt.imports["github.com/google/uuid"] = true
		return "uuid.New().String()", true
	default:
		// DATEADD, DATEDIFF, CAST, etc. are too complex for inline conversion
		return "", false
	}
}

func (dt *dmlTranspiler) extractWhereFieldsFromUpdate(s *ast.UpdateStatement) []whereField {
	var fields []whereField
	if s.Where != nil {
		dt.walkWhereExpr(s.Where, &fields)
	}
	return fields
}

func (dt *dmlTranspiler) extractWhereFieldsFromDelete(s *ast.DeleteStatement) []whereField {
	var fields []whereField
	if s.Where != nil {
		dt.walkWhereExpr(s.Where, &fields)
	}
	return fields
}

// extractInsertFields extracts column/value pairs from INSERT.
func (dt *dmlTranspiler) extractInsertFields(s *ast.InsertStatement) []insertField {
	var fields []insertField

	if s.Columns == nil || s.Values == nil {
		return fields
	}

	for i, col := range s.Columns {
		colName := col.Value

		value := ""
		if len(s.Values) > 0 && i < len(s.Values[0]) {
			value = dt.exprToGoValue(s.Values[0][i])
		}

		if colName != "" {
			fields = append(fields, insertField{column: colName, value: value})
		}
	}

	return fields
}

// extractUpdateSetFields extracts SET assignments from UPDATE.
func (dt *dmlTranspiler) extractUpdateSetFields(s *ast.UpdateStatement) []setField {
	var fields []setField

	for _, set := range s.SetClauses {
		colName := ""
		if set.Column != nil {
			// QualifiedIdentifier - get last part
			parts := set.Column.Parts
			if len(parts) > 0 {
				colName = parts[len(parts)-1].Value
			}
		}

		value := dt.exprToGoValue(set.Value)

		if colName != "" {
			fields = append(fields, setField{column: colName, value: value})
		}
	}

	return fields
}

// Table extraction helpers

func (dt *dmlTranspiler) extractMainTable(s *ast.SelectStatement) string {
	if s.From == nil || len(s.From.Tables) == 0 {
		return ""
	}
	if tn, ok := s.From.Tables[0].(*ast.TableName); ok {
		if tn.Name != nil && len(tn.Name.Parts) > 0 {
			return tn.Name.Parts[len(tn.Name.Parts)-1].Value
		}
	}
	return ""
}

func (dt *dmlTranspiler) extractInsertTable(s *ast.InsertStatement) string {
	if s.Table == nil {
		return ""
	}
	if len(s.Table.Parts) > 0 {
		return s.Table.Parts[len(s.Table.Parts)-1].Value
	}
	return ""
}

func (dt *dmlTranspiler) extractUpdateTable(s *ast.UpdateStatement) string {
	if s.Table == nil {
		return ""
	}
	if len(s.Table.Parts) > 0 {
		return s.Table.Parts[len(s.Table.Parts)-1].Value
	}
	return ""
}

func (dt *dmlTranspiler) extractDeleteTable(s *ast.DeleteStatement) string {
	if s.Table == nil {
		return ""
	}
	if len(s.Table.Parts) > 0 {
		return s.Table.Parts[len(s.Table.Parts)-1].Value
	}
	return ""
}

// isTempTable returns true if the table name indicates a temp table.
// T-SQL temp tables start with # (local) or ## (global).
func isTempTable(tableName string) bool {
	return strings.HasPrefix(tableName, "#")
}

// getEffectiveBackend returns the backend to use for a given table.
// For temp tables, it returns the fallback backend (typically SQL).
// For regular tables, it returns the primary backend.
// Also tracks temp tables encountered for warning purposes.
func (dt *dmlTranspiler) getEffectiveBackend(tableName string) BackendType {
	if isTempTable(tableName) {
		// Record this temp table for warning purposes
		dt.recordTempTable(tableName)
		if dt.config.FallbackBackend != "" {
			return dt.config.FallbackBackend
		}
	}
	return dt.config.Backend
}

// recordTempTable adds a temp table name to the tracking list (deduped).
func (dt *dmlTranspiler) recordTempTable(name string) {
	for _, existing := range dt.transpiler.tempTablesUsed {
		if existing == name {
			return
		}
	}
	dt.transpiler.tempTablesUsed = append(dt.transpiler.tempTablesUsed, name)
}

// Query building helpers

func (dt *dmlTranspiler) buildSelectQuery(s *ast.SelectStatement) (string, []string) {
	// Build dialect-appropriate SELECT query
	var query strings.Builder
	var args []string
	argNum := 1

	query.WriteString("SELECT ")

	// Columns - strip @Var = assignment syntax (handled by Scan)
	if s.Columns != nil {
		var cols []string
		for _, item := range s.Columns {
			// If this is a SELECT @var = expr, output only expr
			if item.Variable != nil && item.Expression != nil {
				cols = append(cols, item.Expression.String())
			} else {
				cols = append(cols, item.String())
			}
		}
		query.WriteString(strings.Join(cols, ", "))
	}

	// FROM
	if s.From != nil {
		query.WriteString(" FROM ")
		var tables []string
		for _, t := range s.From.Tables {
			tables = append(tables, t.String())
		}
		query.WriteString(strings.Join(tables, ", "))
	}

	// WHERE
	if s.Where != nil {
		query.WriteString(" WHERE ")
		whereSQL, whereArgs := dt.buildWhereClause(s.Where, &argNum)
		query.WriteString(whereSQL)
		args = append(args, whereArgs...)
	}

	return stripTableHints(query.String()), args
}

func (dt *dmlTranspiler) buildInsertQuery(s *ast.InsertStatement) (string, []string) {
	var query strings.Builder
	var args []string
	argNum := 1

	query.WriteString("INSERT INTO ")
	if s.Table != nil {
		query.WriteString(s.Table.String())
	}

	// Columns
	if s.Columns != nil {
		var cols []string
		for _, c := range s.Columns {
			cols = append(cols, c.Value)
		}
		query.WriteString(" (")
		query.WriteString(strings.Join(cols, ", "))
		query.WriteString(")")
	}

	// VALUES or SELECT
	if s.Values != nil && len(s.Values) > 0 && len(s.Values[0]) > 0 {
		query.WriteString(" VALUES (")
		var placeholders []string
		for _, val := range s.Values[0] {
			placeholder := dt.getPlaceholder(argNum)
			argNum++
			placeholders = append(placeholders, placeholder)
			args = append(args, dt.exprToGoValue(val))
		}
		query.WriteString(strings.Join(placeholders, ", "))
		query.WriteString(")")
	} else if s.Select != nil {
		// INSERT...SELECT
		query.WriteString(" ")
		selectQuery, selectArgs := dt.buildSelectQuery(s.Select)
		query.WriteString(selectQuery)
		args = append(args, selectArgs...)
	} else if s.DefaultValues {
		query.WriteString(" DEFAULT VALUES")
	}

	return stripTableHints(query.String()), args
}

func (dt *dmlTranspiler) buildUpdateQuery(s *ast.UpdateStatement) (string, []string) {
	var query strings.Builder
	var args []string
	argNum := 1

	query.WriteString("UPDATE ")
	if s.Table != nil {
		query.WriteString(s.Table.String())
	}
	
	// Handle alias if present
	if s.Alias != nil {
		query.WriteString(" ")
		query.WriteString(s.Alias.Value)
	}

	// SET
	query.WriteString(" SET ")
	var setClauses []string
	for _, set := range s.SetClauses {
		col := set.Column.String()
		
		// Check if the value expression contains column references
		// If so, we need to keep the SQL expression and only parameterize variables
		if dt.exprContainsColumnRef(set.Value) {
			// Build SQL expression with only variables as placeholders
			sqlExpr, exprArgs := dt.buildSQLExprWithPlaceholders(set.Value, &argNum)
			setClauses = append(setClauses, fmt.Sprintf("%s = %s", col, sqlExpr))
			args = append(args, exprArgs...)
		} else {
			// Simple value - use placeholder
			placeholder := dt.getPlaceholder(argNum)
			argNum++
			setClauses = append(setClauses, fmt.Sprintf("%s = %s", col, placeholder))
			args = append(args, dt.exprToGoValue(set.Value))
		}
	}
	query.WriteString(strings.Join(setClauses, ", "))

	// FROM clause (T-SQL specific, but supported by PostgreSQL too)
	if s.From != nil {
		query.WriteString(" ")
		fromSQL, fromArgs := dt.buildFromClause(s.From, &argNum)
		query.WriteString(fromSQL)
		args = append(args, fromArgs...)
	}

	// WHERE
	if s.Where != nil {
		query.WriteString(" WHERE ")
		whereSQL, whereArgs := dt.buildWhereClause(s.Where, &argNum)
		query.WriteString(whereSQL)
		args = append(args, whereArgs...)
	}

	return stripTableHints(query.String()), args
}

// buildFromClause builds the FROM clause for UPDATE/DELETE with JOINs
// For SQL backend, we preserve the FROM clause structure and only parameterize variables
func (dt *dmlTranspiler) buildFromClause(from *ast.FromClause, argNum *int) (string, []string) {
	if from == nil {
		return "", nil
	}
	
	// The FromClause.String() gives us the complete FROM clause with JOINs
	// We need to walk it to find and parameterize any variables in ON conditions
	var args []string
	
	// For now, use the native String() representation which handles all the join syntax
	// This works because FROM clauses in UPDATE typically don't have parameterized values
	// (the values are in SET and WHERE clauses)
	return from.String(), args
}

// buildTableReferenceSQL builds SQL for a table reference, parameterizing variables in ON conditions
func (dt *dmlTranspiler) buildTableReferenceSQL(tableRef ast.TableReference, argNum *int) (string, []string) {
	if tableRef == nil {
		return "", nil
	}
	
	switch t := tableRef.(type) {
	case *ast.TableName:
		var out strings.Builder
		out.WriteString(t.Name.String())
		if t.Alias != nil {
			out.WriteString(" AS ")
			out.WriteString(t.Alias.Value)
		}
		return out.String(), nil
		
	case *ast.JoinClause:
		var out strings.Builder
		var args []string
		
		// Left side
		leftSQL, leftArgs := dt.buildTableReferenceSQL(t.Left, argNum)
		out.WriteString(leftSQL)
		args = append(args, leftArgs...)
		
		// Join type
		out.WriteString(" ")
		if t.Type == "CROSS APPLY" || t.Type == "OUTER APPLY" {
			out.WriteString(t.Type)
		} else {
			out.WriteString(t.Type)
			if t.Hint != "" {
				out.WriteString(" ")
				out.WriteString(t.Hint)
			}
			out.WriteString(" JOIN")
		}
		out.WriteString(" ")
		
		// Right side
		rightSQL, rightArgs := dt.buildTableReferenceSQL(t.Right, argNum)
		out.WriteString(rightSQL)
		args = append(args, rightArgs...)
		
		// ON condition (may contain variables)
		if t.Condition != nil {
			out.WriteString(" ON ")
			condSQL, condArgs := dt.buildSQLExprWithPlaceholders(t.Condition, argNum)
			out.WriteString(condSQL)
			args = append(args, condArgs...)
		}
		
		return out.String(), args
	}
	
	// Fallback to String()
	return tableRef.String(), nil
}

// exprContainsColumnRef checks if an expression contains column references (not variables)
func (dt *dmlTranspiler) exprContainsColumnRef(expr ast.Expression) bool {
	if expr == nil {
		return false
	}
	
	switch e := expr.(type) {
	case *ast.Identifier:
		// Bare identifier is a column reference
		return true
	case *ast.QualifiedIdentifier:
		// table.column is a column reference
		return true
	case *ast.Variable:
		// @var is not a column reference
		return false
	case *ast.InfixExpression:
		// Check both sides
		return dt.exprContainsColumnRef(e.Left) || dt.exprContainsColumnRef(e.Right)
	case *ast.PrefixExpression:
		return dt.exprContainsColumnRef(e.Right)
	case *ast.FunctionCall:
		// Check function arguments
		for _, arg := range e.Arguments {
			if dt.exprContainsColumnRef(arg) {
				return true
			}
		}
		return false
	}
	
	return false
}

// buildSQLExprWithPlaceholders builds a SQL expression string, replacing only variables with placeholders
func (dt *dmlTranspiler) buildSQLExprWithPlaceholders(expr ast.Expression, argNum *int) (string, []string) {
	if expr == nil {
		return "", nil
	}
	
	var args []string
	
	switch e := expr.(type) {
	case *ast.Variable:
		// Replace variable with placeholder
		placeholder := dt.getPlaceholder(*argNum)
		*argNum++
		varName := goIdentifier(strings.TrimPrefix(e.Name, "@"))
		args = append(args, varName)
		return placeholder, args
		
	case *ast.Identifier:
		// Keep column name as-is
		return e.Value, nil
		
	case *ast.QualifiedIdentifier:
		return e.String(), nil
		
	case *ast.IntegerLiteral:
		return fmt.Sprintf("%d", e.Value), nil
		
	case *ast.FloatLiteral:
		return fmt.Sprintf("%v", e.Value), nil
		
	case *ast.StringLiteral:
		return fmt.Sprintf("'%s'", e.Value), nil
		
	case *ast.InfixExpression:
		leftSQL, leftArgs := dt.buildSQLExprWithPlaceholders(e.Left, argNum)
		rightSQL, rightArgs := dt.buildSQLExprWithPlaceholders(e.Right, argNum)
		args = append(args, leftArgs...)
		args = append(args, rightArgs...)
		return fmt.Sprintf("%s %s %s", leftSQL, e.Operator, rightSQL), args
		
	case *ast.PrefixExpression:
		rightSQL, rightArgs := dt.buildSQLExprWithPlaceholders(e.Right, argNum)
		args = append(args, rightArgs...)
		return fmt.Sprintf("%s%s", e.Operator, rightSQL), args
		
	case *ast.FunctionCall:
		// Function call - keep function name, process arguments
		var funcArgs []string
		for _, arg := range e.Arguments {
			argSQL, argArgs := dt.buildSQLExprWithPlaceholders(arg, argNum)
			funcArgs = append(funcArgs, argSQL)
			args = append(args, argArgs...)
		}
		funcName := e.Function.String()
		return fmt.Sprintf("%s(%s)", funcName, strings.Join(funcArgs, ", ")), args
	}
	
	// Fallback - use string representation
	return expr.String(), nil
}

func (dt *dmlTranspiler) buildDeleteQuery(s *ast.DeleteStatement) (string, []string) {
	var query strings.Builder
	var args []string
	argNum := 1

	query.WriteString("DELETE FROM ")
	if s.Table != nil {
		query.WriteString(s.Table.String())
	}

	// WHERE
	if s.Where != nil {
		query.WriteString(" WHERE ")
		whereSQL, whereArgs := dt.buildWhereClause(s.Where, &argNum)
		query.WriteString(whereSQL)
		args = append(args, whereArgs...)
	}

	return stripTableHints(query.String()), args
}

func (dt *dmlTranspiler) buildWhereClause(expr ast.Expression, argNum *int) (string, []string) {
	var args []string

	switch e := expr.(type) {
	case *ast.InfixExpression:
		op := strings.ToUpper(e.Operator)
		if op == "AND" || op == "OR" {
			leftSQL, leftArgs := dt.buildWhereClause(e.Left, argNum)
			rightSQL, rightArgs := dt.buildWhereClause(e.Right, argNum)
			args = append(args, leftArgs...)
			args = append(args, rightArgs...)
			return fmt.Sprintf("(%s %s %s)", leftSQL, op, rightSQL), args
		}

		// column op @variable
		left := dt.exprToString(e.Left)
		if v, ok := e.Right.(*ast.Variable); ok {
			placeholder := dt.getPlaceholder(*argNum)
			*argNum++
			varName := goIdentifier(strings.TrimPrefix(v.Name, "@"))
			args = append(args, varName)
			return fmt.Sprintf("%s %s %s", left, e.Operator, placeholder), args
		}

		// column op literal
		right := dt.exprToString(e.Right)
		return fmt.Sprintf("%s %s %s", left, e.Operator, right), args
	}

	return dt.exprToString(expr), args
}

func (dt *dmlTranspiler) getPlaceholder(n int) string {
	switch dt.config.SQLDialect {
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

func (dt *dmlTranspiler) isSingleRowSelect(s *ast.SelectStatement) bool {
	// Single row if TOP 1, or WHERE on unique key, or aggregate only
	if s.Top != nil {
		if intLit, ok := s.Top.Count.(*ast.IntegerLiteral); ok && intLit.Value == 1 {
			return true
		}
	}
	// Heuristic: WHERE on single column ending in ID/Id/id
	if s.Where != nil {
		whereFields := dt.extractWhereFields(s)
		if len(whereFields) == 1 {
			col := strings.ToLower(whereFields[0].column)
			if strings.HasSuffix(col, "id") || strings.HasSuffix(col, "_id") {
				return true
			}
		}
	}
	return false
}

// Method name inference

// inferGRPCMethod determines the gRPC method name for a SELECT statement.
// Priority: explicit GRPCMappings > table-to-service + verb detection > default inference
func (dt *dmlTranspiler) inferGRPCMethod(s *ast.SelectStatement, table string) string {
	whereFields := dt.extractWhereFields(s)
	entityName := toPascalCase(singularize(table))

	// Check for verb hints in WHERE clause variable names
	verb := dt.detectVerbFromWhereFields(whereFields)
	if verb != "" && !verbConflictsWithEntity(verb, entityName) {
		return verb + entityName
	}

	// Check for verb hints in column names being selected
	verb = dt.detectVerbFromSelectColumns(s)
	if verb != "" && !verbConflictsWithEntity(verb, entityName) {
		return verb + entityName
	}

	// Default inference based on query pattern
	if len(whereFields) == 0 {
		// Apply toPascalCase first, then pluralize to preserve word boundaries
		return "List" + pluralize(toPascalCase(table))
	}

	if len(whereFields) == 1 {
		col := whereFields[0].column
		if strings.ToLower(col) == "id" || strings.HasSuffix(strings.ToLower(col), "_id") {
			return "Get" + entityName
		}
		return "Get" + entityName + "By" + toPascalCase(col)
	}

	// Apply toPascalCase first, then pluralize to preserve word boundaries
	return "Find" + pluralize(toPascalCase(table))
}

// detectVerbFromWhereFields looks for action verbs in WHERE clause variable/column names.
func (dt *dmlTranspiler) detectVerbFromWhereFields(whereFields []whereField) string {
	for _, wf := range whereFields {
		// Check variable name for verb hints
		if wf.variable != "" {
			if verb := extractActionVerb(wf.variable); verb != "" {
				return verb
			}
		}
		// Check column name for verb hints
		if verb := extractActionVerb(wf.column); verb != "" {
			return verb
		}
	}
	return ""
}

// detectVerbFromSelectColumns looks for action verbs in selected column names.
func (dt *dmlTranspiler) detectVerbFromSelectColumns(s *ast.SelectStatement) string {
	if s.Columns == nil {
		return ""
	}
	for _, item := range s.Columns {
		if item.Alias != nil {
			if verb := extractActionVerb(item.Alias.Value); verb != "" {
				return verb
			}
		}
		// Check column expression for identifiers
		if ident, ok := item.Expression.(*ast.Identifier); ok {
			if verb := extractActionVerb(ident.Value); verb != "" {
				return verb
			}
		}
	}
	return ""
}

// extractActionVerb detects business process verbs in identifiers.
// Returns the verb in PascalCase if found, empty string otherwise.
func extractActionVerb(name string) string {
	nameLower := strings.ToLower(name)

	// Verb patterns in priority order (longer/more specific patterns first)
	// This ensures "deactivate" is matched before "activate", etc.
	verbPatterns := []struct {
		verb     string
		patterns []string
	}{
		// Compound verbs first (to avoid substring issues)
		{"Countersign", []string{"countersign", "countersigned", "countersigning"}},
		{"Deactivate", []string{"deactivate", "deactivated", "deactivating", "deactivation"}},
		{"Acknowledge", []string{"acknowledge", "acknowledged", "acknowledging", "acknowledgment", "acknowledgement"}},

		// Approval workflow verbs
		{"Approve", []string{"approve", "approved", "approving", "approval"}},
		{"Reject", []string{"reject", "rejected", "rejecting", "rejection"}},
		{"Certify", []string{"certify", "certified", "certifying", "certification"}},
		{"Attest", []string{"attest", "attested", "attesting", "attestation"}},
		{"Review", []string{"review", "reviewed", "reviewing"}},
		{"Assess", []string{"assess", "assessed", "assessing", "assessment"}},
		{"Audit", []string{"audit", "audited", "auditing"}},
		{"Authorize", []string{"authorize", "authorized", "authorizing", "authorization"}},
		{"Grant", []string{"grant", "granted", "granting"}},
		{"Deny", []string{"deny", "denied", "denying", "denial"}},
		{"Escalate", []string{"escalate", "escalated", "escalating", "escalation"}},
		{"Delegate", []string{"delegate", "delegated", "delegating", "delegation"}},

		// Lifecycle verbs
		{"Suspend", []string{"suspend", "suspended", "suspending", "suspension"}},
		{"Resume", []string{"resume", "resumed", "resuming"}},
		{"Cancel", []string{"cancel", "cancelled", "canceled", "cancelling", "canceling", "cancellation"}},
		{"Terminate", []string{"terminate", "terminated", "terminating", "termination"}},
		{"Complete", []string{"complete", "completed", "completing", "completion"}},
		{"Finalize", []string{"finalize", "finalized", "finalizing", "finalization"}},
		{"Activate", []string{"activate", "activated", "activating", "activation"}},

		// Communication verbs
		{"Notify", []string{"notify", "notified", "notifying", "notification"}},
		{"Alert", []string{"alert", "alerted", "alerting"}},

		// Signing verbs
		{"Sign", []string{"sign", "signed", "signing", "signature"}},

		// Calculation verbs
		{"Calculate", []string{"calculate", "calculated", "calculating", "calculation"}},
		{"Compute", []string{"compute", "computed", "computing", "computation"}},
		{"Estimate", []string{"estimate", "estimated", "estimating", "estimation"}},

		// Validation verbs
		{"Validate", []string{"validate", "validated", "validating", "validation"}},
		{"Verify", []string{"verify", "verified", "verifying", "verification"}},

		// Transfer verbs
		{"Transfer", []string{"transfer", "transferred", "transferring"}},
		{"Submit", []string{"submit", "submitted", "submitting", "submission"}},
	}

	for _, vp := range verbPatterns {
		for _, pattern := range vp.patterns {
			if strings.Contains(nameLower, pattern) {
				return vp.verb
			}
		}
	}

	return ""
}

// getGRPCClientForTable returns the gRPC client variable for a table based on configuration.
func (dt *dmlTranspiler) getGRPCClientForTable(table string) string {
	tableLower := strings.ToLower(table)

	// Check explicit table-to-client mapping
	if dt.config.TableToClient != nil {
		if client, ok := dt.config.TableToClient[table]; ok {
			return client
		}
		if client, ok := dt.config.TableToClient[tableLower]; ok {
			return client
		}
	}

	// Check table-to-service mapping and derive client name
	if dt.config.TableToService != nil {
		if service, ok := dt.config.TableToService[table]; ok {
			return toLowerCamel(service) + "Client"
		}
		if service, ok := dt.config.TableToService[tableLower]; ok {
			return toLowerCamel(service) + "Client"
		}
	}

	// Check explicit GRPCClientVar if set to non-default
	if dt.config.GRPCClientVar != "" && dt.config.GRPCClientVar != "client" {
		return dt.config.GRPCClientVar
	}

	// Fall back to StoreVar for backwards compatibility
	if dt.config.StoreVar != "" {
		return dt.config.StoreVar
	}

	// Final fallback
	return "client"
}

// getProtoPackageForTable returns the proto package for a table based on configuration.
func (dt *dmlTranspiler) getProtoPackageForTable(table string) string {
	tableLower := strings.ToLower(table)

	// Check table-to-service, then service-to-package
	if dt.config.TableToService != nil {
		var service string
		if svc, ok := dt.config.TableToService[table]; ok {
			service = svc
		} else if svc, ok := dt.config.TableToService[tableLower]; ok {
			service = svc
		}
		if service != "" {
			// First check explicit service-to-package mapping
			if dt.config.ServiceToPackage != nil {
				if pkg, ok := dt.config.ServiceToPackage[service]; ok {
					return pkg
				}
			}
			// Infer proto package from service name: CatalogService -> catalogpb
			return inferProtoPackage(service)
		}
	}

	// Fall back to config default
	return dt.config.ProtoPackage
}

// inferProtoPackage derives a proto package name from a service name.
// Examples:
//   - CatalogService -> catalogpb
//   - OrderService -> orderpb
//   - UserAccountService -> useraccountpb
func inferProtoPackage(serviceName string) string {
	// Remove common suffixes
	name := serviceName
	for _, suffix := range []string{"Service", "Svc", "API", "Api"} {
		if strings.HasSuffix(name, suffix) {
			name = strings.TrimSuffix(name, suffix)
			break
		}
	}

	// Convert to lowercase and add pb suffix
	return strings.ToLower(name) + "pb"
}

// toLowerCamel converts PascalCase to lowerCamelCase
func toLowerCamel(s string) string {
	if s == "" {
		return s
	}
	// Find first lowercase letter or end of string
	for i, r := range s {
		if i > 0 && (r >= 'a' && r <= 'z') {
			return strings.ToLower(s[:i-1]) + s[i-1:]
		}
	}
	return strings.ToLower(s)
}

func (dt *dmlTranspiler) inferMockMethod(s *ast.SelectStatement, table string) string {
	return dt.inferGRPCMethod(s, table) // Same logic
}

// Expression to string helpers

func (dt *dmlTranspiler) exprToString(expr ast.Expression) string {
	if expr == nil {
		return ""
	}
	switch e := expr.(type) {
	case *ast.Identifier:
		return e.Value
	case *ast.QualifiedIdentifier:
		var parts []string
		for _, p := range e.Parts {
			parts = append(parts, p.Value)
		}
		return strings.Join(parts, ".")
	case *ast.Variable:
		return e.Name
	case *ast.IntegerLiteral:
		return fmt.Sprintf("%d", e.Value)
	case *ast.FloatLiteral:
		return fmt.Sprintf("%v", e.Value)
	case *ast.StringLiteral:
		return fmt.Sprintf("'%s'", e.Value)
	default:
		return fmt.Sprintf("%v", expr)
	}
}

func (dt *dmlTranspiler) exprToGoValue(expr ast.Expression) string {
	if expr == nil {
		return "nil"
	}
	switch e := expr.(type) {
	case *ast.Variable:
		return goIdentifier(strings.TrimPrefix(e.Name, "@"))
	case *ast.IntegerLiteral:
		return fmt.Sprintf("%d", e.Value)
	case *ast.FloatLiteral:
		return fmt.Sprintf("%v", e.Value)
	case *ast.StringLiteral:
		return fmt.Sprintf("%q", e.Value)
	case *ast.NullLiteral:
		return "nil"
	default:
		// For complex expressions, transpile as Go expression
		result, err := dt.transpileExpression(expr)
		if err != nil {
			return fmt.Sprintf("%v", expr)
		}
		return result
	}
}

// Transaction and database variable helpers

// getDBVar returns the appropriate database variable: "tx" if in transaction, StoreVar otherwise
func (dt *dmlTranspiler) getDBVar() string {
	if dt.inTransaction {
		return "tx"
	}
	return dt.config.StoreVar
}

// Cursor transpilation
// Converts T-SQL cursor pattern to Go rows iteration:
//   DECLARE cursor CURSOR FOR SELECT ... -> (stored, emitted on OPEN)
//   OPEN cursor                           -> rows, err := db.QueryContext(...)
//   FETCH NEXT INTO @var1, @var2          -> (first one absorbed, rest ignored)
//   WHILE @@FETCH_STATUS = 0              -> for rows.Next()
//   CLOSE/DEALLOCATE                      -> (handled by defer rows.Close())

func (t *transpiler) transpileDeclareCursor(s *ast.DeclareCursorStatement) (string, error) {
	cursorName := s.Name.Value
	
	// Store cursor info for later use
	t.cursors[cursorName] = &cursorInfo{
		name:    cursorName,
		query:   s.ForSelect,
		rowsVar: goIdentifier(cursorName) + "Rows",
	}
	
	// Don't emit anything - query executed on OPEN
	return fmt.Sprintf("// DECLARE CURSOR %s (query stored for OPEN)", cursorName), nil
}

func (t *transpiler) transpileOpenCursor(s *ast.OpenCursorStatement) (string, error) {
	cursorName := s.CursorName.Value
	
	cursor, exists := t.cursors[cursorName]
	if !exists {
		return "", fmt.Errorf("cursor %s not declared", cursorName)
	}
	
	cursor.isOpen = true
	t.activeCursor = cursorName
	
	// Build the query
	dt := &dmlTranspiler{transpiler: t, config: t.dmlConfig}
	query, args := dt.buildSelectQuery(cursor.query)
	
	// Post-process to catch any remaining @variable references
	query, extraArgs := dt.substituteVariablesInQuery(query)
	args = append(args, extraArgs...)
	
	dbVar := dt.getDBVar()
	
	var out strings.Builder
	out.WriteString(fmt.Sprintf("// OPEN %s\n", cursorName))
	out.WriteString(t.indentStr())
	out.WriteString(fmt.Sprintf("%s, err := %s.QueryContext(ctx, %q", cursor.rowsVar, dbVar, query))
	for _, arg := range args {
		out.WriteString(", " + arg)
	}
	out.WriteString(")\n")
	out.WriteString(t.indentStr())
	out.WriteString("if err != nil {\n")
	out.WriteString(t.indentStr())
	out.WriteString("\t" + dt.buildErrorReturn() + "\n")
	out.WriteString(t.indentStr())
	out.WriteString("}\n")
	out.WriteString(t.indentStr())
	out.WriteString(fmt.Sprintf("defer %s.Close()", cursor.rowsVar))
	
	return out.String(), nil
}

func (t *transpiler) transpileFetch(s *ast.FetchStatement) (string, error) {
	cursorName := ""
	if s.CursorName != nil {
		cursorName = s.CursorName.Value
	}
	
	cursor, exists := t.cursors[cursorName]
	if !exists {
		return "", fmt.Errorf("cursor %s not declared", cursorName)
	}
	
	// Store fetch variables for use in WHILE loop detection
	cursor.fetchVars = s.IntoVars
	
	// The first FETCH before WHILE is absorbed into the for rows.Next() loop
	// Subsequent FETCHes inside the loop are also absorbed
	// Return a comment indicating the fetch is handled by the rows iteration
	return fmt.Sprintf("// FETCH from %s handled by rows.Next() loop", cursorName), nil
}

func (t *transpiler) transpileCloseCursor(s *ast.CloseCursorStatement) (string, error) {
	cursorName := s.CursorName.Value
	
	if cursor, exists := t.cursors[cursorName]; exists {
		cursor.isOpen = false
	}
	
	// Cleanup handled by defer rows.Close()
	return fmt.Sprintf("// CLOSE %s (handled by defer)", cursorName), nil
}

func (t *transpiler) transpileDeallocateCursor(s *ast.DeallocateCursorStatement) (string, error) {
	cursorName := s.CursorName.Value
	
	// Remove cursor from tracking
	delete(t.cursors, cursorName)
	if t.activeCursor == cursorName {
		t.activeCursor = ""
	}
	
	return fmt.Sprintf("// DEALLOCATE %s (no-op in Go)", cursorName), nil
}

// isFetchStatusCheck checks if an expression is or contains @@FETCH_STATUS = 0
// Returns true for both simple "@@FETCH_STATUS = 0" and compound "@@FETCH_STATUS = 0 AND other_condition"
func (t *transpiler) isFetchStatusCheck(expr ast.Expression) bool {
	return t.containsFetchStatusCheck(expr)
}

// containsFetchStatusCheck recursively checks if expression contains @@FETCH_STATUS = 0
func (t *transpiler) containsFetchStatusCheck(expr ast.Expression) bool {
	if expr == nil {
		return false
	}
	
	infix, ok := expr.(*ast.InfixExpression)
	if !ok {
		return false
	}
	
	// Check for compound AND - recurse into both sides
	if strings.ToUpper(infix.Operator) == "AND" {
		return t.containsFetchStatusCheck(infix.Left) || t.containsFetchStatusCheck(infix.Right)
	}
	
	// Check for direct @@FETCH_STATUS = 0
	if infix.Operator == "=" {
		// Check left side for @@FETCH_STATUS
		if v, ok := infix.Left.(*ast.Variable); ok {
			if strings.ToUpper(v.Name) == "@@FETCH_STATUS" {
				if intLit, ok := infix.Right.(*ast.IntegerLiteral); ok {
					return intLit.Value == 0
				}
			}
		}
		
		// Also check reversed: 0 = @@FETCH_STATUS
		if intLit, ok := infix.Left.(*ast.IntegerLiteral); ok && intLit.Value == 0 {
			if v, ok := infix.Right.(*ast.Variable); ok {
				return strings.ToUpper(v.Name) == "@@FETCH_STATUS"
			}
		}
	}
	
	return false
}

// extractNonFetchConditions extracts conditions from a compound expression that are NOT @@FETCH_STATUS = 0
// For "@@FETCH_STATUS = 0 AND @Count < @Max", returns "@Count < @Max"
func (t *transpiler) extractNonFetchConditions(expr ast.Expression) ast.Expression {
	if expr == nil {
		return nil
	}
	
	infix, ok := expr.(*ast.InfixExpression)
	if !ok {
		return nil
	}
	
	// If this is a compound AND expression
	if strings.ToUpper(infix.Operator) == "AND" {
		leftIsFetch := t.isDirectFetchStatusCheck(infix.Left)
		rightIsFetch := t.isDirectFetchStatusCheck(infix.Right)
		
		if leftIsFetch && rightIsFetch {
			return nil // Both are fetch status checks
		}
		if leftIsFetch {
			// Left is fetch status, return right (possibly recursing)
			return t.extractNonFetchConditions(infix.Right)
		}
		if rightIsFetch {
			// Right is fetch status, return left (possibly recursing)
			return t.extractNonFetchConditions(infix.Left)
		}
		
		// Neither side is directly a fetch check, but one might contain it in nested AND
		leftContains := t.containsFetchStatusCheck(infix.Left)
		rightContains := t.containsFetchStatusCheck(infix.Right)
		
		if leftContains && !rightContains {
			// Extract from left, combine with right
			leftExtracted := t.extractNonFetchConditions(infix.Left)
			if leftExtracted == nil {
				return infix.Right
			}
			return &ast.InfixExpression{
				Left:     leftExtracted,
				Operator: "AND",
				Right:    infix.Right,
			}
		}
		if rightContains && !leftContains {
			// Extract from right, combine with left
			rightExtracted := t.extractNonFetchConditions(infix.Right)
			if rightExtracted == nil {
				return infix.Left
			}
			return &ast.InfixExpression{
				Left:     infix.Left,
				Operator: "AND",
				Right:    rightExtracted,
			}
		}
		
		// Neither contains fetch status, return the whole expression
		if !leftContains && !rightContains {
			return expr
		}
	}
	
	// Not a compound, check if this is the fetch status check itself
	if t.isDirectFetchStatusCheck(expr) {
		return nil
	}
	
	return expr
}

// isDirectFetchStatusCheck checks if expression is exactly @@FETCH_STATUS = 0 (not compound)
func (t *transpiler) isDirectFetchStatusCheck(expr ast.Expression) bool {
	infix, ok := expr.(*ast.InfixExpression)
	if !ok {
		return false
	}
	
	if infix.Operator != "=" {
		return false
	}
	
	// Check @@FETCH_STATUS = 0
	if v, ok := infix.Left.(*ast.Variable); ok {
		if strings.ToUpper(v.Name) == "@@FETCH_STATUS" {
			if intLit, ok := infix.Right.(*ast.IntegerLiteral); ok {
				return intLit.Value == 0
			}
		}
	}
	
	// Check 0 = @@FETCH_STATUS
	if intLit, ok := infix.Left.(*ast.IntegerLiteral); ok && intLit.Value == 0 {
		if v, ok := infix.Right.(*ast.Variable); ok {
			return strings.ToUpper(v.Name) == "@@FETCH_STATUS"
		}
	}
	
	return false
}

// transpileCursorWhile handles WHILE @@FETCH_STATUS = 0 pattern
// Also handles compound conditions like "@@FETCH_STATUS = 0 AND @Count < @Max"
func (t *transpiler) transpileCursorWhile(whileStmt *ast.WhileStatement) (string, error) {
	if t.activeCursor == "" {
		return "", fmt.Errorf("no active cursor for WHILE @@FETCH_STATUS loop")
	}
	
	cursor := t.cursors[t.activeCursor]
	if cursor == nil {
		return "", fmt.Errorf("cursor %s not found", t.activeCursor)
	}
	
	var out strings.Builder
	
	// Generate scan targets from FETCH INTO variables
	var scanTargets []string
	for _, v := range cursor.fetchVars {
		varName := goIdentifier(strings.TrimPrefix(v.Name, "@"))
		scanTargets = append(scanTargets, "&"+varName)
	}
	scanList := strings.Join(scanTargets, ", ")
	if scanList == "" {
		scanList = "/* TODO: add scan targets */"
	}
	
	out.WriteString(fmt.Sprintf("for %s.Next() {\n", cursor.rowsVar))
	t.indent++
	out.WriteString(t.indentStr())
	out.WriteString(fmt.Sprintf("if err := %s.Scan(%s); err != nil {\n", cursor.rowsVar, scanList))
	out.WriteString(t.indentStr())
	out.WriteString("\t" + t.buildErrorReturn() + "\n")
	out.WriteString(t.indentStr())
	out.WriteString("}\n")
	
	// Check for additional conditions beyond @@FETCH_STATUS = 0
	// e.g., "@@FETCH_STATUS = 0 AND @ProcessedCount < @MaxOrders"
	additionalCond := t.extractNonFetchConditions(whileStmt.Condition)
	if additionalCond != nil {
		// Generate break condition: if !(<additional_cond>) { break }
		condCode, err := t.transpileExpression(additionalCond)
		if err != nil {
			return "", fmt.Errorf("failed to transpile additional condition: %w", err)
		}
		out.WriteString(t.indentStr())
		out.WriteString(fmt.Sprintf("if !(%s) {\n", condCode))
		out.WriteString(t.indentStr())
		out.WriteString("\tbreak\n")
		out.WriteString(t.indentStr())
		out.WriteString("}\n")
	}
	
	// Process body, filtering out FETCH statements
	if whileStmt.Body != nil {
		bodyCode, err := t.transpileCursorLoopBody(whileStmt.Body)
		if err != nil {
			return "", err
		}
		if bodyCode != "" {
			out.WriteString(bodyCode)
		}
	}
	
	t.indent--
	out.WriteString(t.indentStr())
	out.WriteString("}")
	
	return out.String(), nil
}

// transpileCursorLoopBody processes a WHILE body, filtering out FETCH statements
func (t *transpiler) transpileCursorLoopBody(stmt ast.Statement) (string, error) {
	switch s := stmt.(type) {
	case *ast.BeginEndBlock:
		var parts []string
		for _, innerStmt := range s.Statements {
			// Skip FETCH statements inside the loop
			if _, isFetch := innerStmt.(*ast.FetchStatement); isFetch {
				continue
			}
			code, err := t.transpileStatement(innerStmt)
			if err != nil {
				return "", err
			}
			if code != "" {
				parts = append(parts, t.indentStr()+code)
			}
		}
		return strings.Join(parts, "\n") + "\n", nil
	case *ast.FetchStatement:
		// Skip FETCH inside loop
		return "", nil
	default:
		code, err := t.transpileStatement(s)
		if err != nil {
			return "", err
		}
		if code != "" {
			return t.indentStr() + code + "\n", nil
		}
		return "", nil
	}
}

// Column extraction and scan target generation

// selectColumn represents a column from a SELECT clause
type selectColumn struct {
	name       string         // column name or alias
	expr       string         // original expression as string
	alias      string         // AS alias if present
	expression ast.Expression // the actual AST expression for type inference
}

// extractSelectColumns extracts column names from SELECT clause
func (dt *dmlTranspiler) extractSelectColumns(s *ast.SelectStatement) []selectColumn {
	var columns []selectColumn
	
	if s.Columns == nil {
		return columns
	}
	
	for _, item := range s.Columns {
		col := selectColumn{}
		
		// Store the actual expression for type inference
		col.expression = item.Expression
		
		// Get the expression string
		if item.Expression != nil {
			col.expr = item.Expression.String()
		}
		
		// Check for alias
		if item.Alias != nil {
			col.alias = item.Alias.Value
			col.name = item.Alias.Value
		} else if item.Expression != nil {
			// Try to extract column name from expression
			col.name = dt.extractColumnName(item.Expression)
		}
		
		// Check for SELECT *
		if col.expr == "*" {
			col.name = "*"
		}
		
		columns = append(columns, col)
	}
	
	return columns
}

// extractColumnName tries to get a usable name from a column expression
func (dt *dmlTranspiler) extractColumnName(expr ast.Expression) string {
	switch e := expr.(type) {
	case *ast.Identifier:
		return e.Value
	case *ast.QualifiedIdentifier:
		if len(e.Parts) > 0 {
			return e.Parts[len(e.Parts)-1].Value
		}
	case *ast.FunctionCall:
		// For functions like COUNT(*), SUM(x), use function name from the Function expression
		if id, ok := e.Function.(*ast.Identifier); ok {
			return strings.ToLower(id.Value)
		}
		// Fallback: use token literal
		return strings.ToLower(e.Token.Literal)
	}
	return "col"
}

// generateScanTargets generates variable declarations and scan arguments
func (dt *dmlTranspiler) generateScanTargets(columns []selectColumn) (string, string) {
	if len(columns) == 0 {
		return "", "/* no columns */"
	}
	
	// Check for SELECT *
	for _, col := range columns {
		if col.name == "*" {
			return "", "/* TODO: SELECT * requires explicit columns */"
		}
	}
	
	var decls []string
	var targets []string
	usedNames := make(map[string]int)
	
	for _, col := range columns {
		// Get a valid Go identifier
		name := goIdentifier(col.name)
		if name == "" {
			name = "col"
		}
		
		// Handle duplicate names
		if count, exists := usedNames[name]; exists {
			usedNames[name] = count + 1
			name = fmt.Sprintf("%s%d", name, count+1)
		} else {
			usedNames[name] = 1
		}
		
		// First, try to infer type from the actual expression
		goType := "interface{}"
		if col.expression != nil {
			if ti := dt.transpiler.inferType(col.expression); ti != nil && ti.goType != "" && ti.goType != "interface{}" {
				goType = ti.goType
				// Add imports if needed
				if ti.goType == "decimal.Decimal" {
					dt.imports["github.com/shopspring/decimal"] = true
				} else if ti.goType == "time.Time" {
					dt.imports["time"] = true
				}
			}
		}
		
		// If expression-based inference didn't work, fall back to name heuristics
		if goType == "interface{}" {
			lowerName := strings.ToLower(col.name)
			switch {
			case strings.HasSuffix(lowerName, "id"):
				goType = "int64"
			case strings.HasSuffix(lowerName, "at") || strings.HasSuffix(lowerName, "date") || strings.HasSuffix(lowerName, "time"):
				goType = "time.Time"
				dt.imports["time"] = true
			case lowerName == "count" || lowerName == "sum" || lowerName == "total":
				goType = "int64"
			case strings.HasPrefix(lowerName, "is") || strings.HasPrefix(lowerName, "has") || strings.HasSuffix(lowerName, "active"):
				goType = "bool"
			case strings.Contains(lowerName, "price") || strings.Contains(lowerName, "amount") || strings.Contains(lowerName, "total"):
				goType = "decimal.Decimal"
				dt.imports["github.com/shopspring/decimal"] = true
			case strings.Contains(lowerName, "name") || strings.Contains(lowerName, "email") || 
				strings.Contains(lowerName, "title") || strings.Contains(lowerName, "description"):
				goType = "string"
			}
		}
		
		decls = append(decls, fmt.Sprintf("var %s %s", name, goType))
		targets = append(targets, "&"+name)
	}
	
	declStr := strings.Join(decls, "\n"+dt.indentStr())
	targetStr := strings.Join(targets, ", ")
	
	return declStr, targetStr
}

// String helpers

func cleanProcedureName(name string) string {
	// Remove schema prefix
	if idx := strings.LastIndex(name, "."); idx >= 0 {
		name = name[idx+1:]
	}
	// Remove usp_/sp_/proc_ prefix
	name = strings.TrimPrefix(name, "usp_")
	name = strings.TrimPrefix(name, "sp_")
	name = strings.TrimPrefix(name, "proc_")
	return name
}

func toPascalCase(s string) string {
	// Strip T-SQL-specific prefixes
	s = strings.TrimPrefix(s, "#")  // temp table prefix
	s = strings.TrimPrefix(s, "##") // global temp table prefix
	s = strings.TrimPrefix(s, "@")  // variable prefix
	s = strings.TrimPrefix(s, "@@") // system variable prefix
	
	// Check if string is ALL_CAPS or ALL_CAPS_WITH_UNDERSCORES
	// If so, try smart word splitting first
	if isAllCapsOrUnderscored(s) {
		if strings.Contains(s, "_") {
			// Has underscores - just lowercase and let normal processing handle it
			s = strings.ToLower(s)
		} else {
			// No underscores - try to split using known words
			if split := splitAllCapsIdentifier(s); split != "" {
				return split
			}
			// Fallback to simple lowercase
			s = strings.ToLower(s)
		}
	}
	
	result := make([]byte, 0, len(s))
	capitalizeNext := true

	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '_' || c == '-' || c == ' ' || c == '#' || c == '@' {
			capitalizeNext = true
			continue
		}
		if capitalizeNext && c >= 'a' && c <= 'z' {
			c -= 'a' - 'A'
		}
		capitalizeNext = false
		result = append(result, c)
	}

	return string(result)
}

// knownWords contains words used for splitting ALL_CAPS identifiers.
// Ordered by length (longest first) to ensure greedy matching.
// IMPORTANT: Avoid plural forms that could cause false matches (e.g., "orders" matching "orderstatushistory")
// This list includes verbs from extractActionVerb plus common domain nouns.
var knownWords = []string{
	// Long compound words first (12+ chars)
	"reconciliation", "configuration", "authentication", "authorization",
	"infrastructure", "implementation", "administration", "communication",
	// 10-11 char words
	"notification", "transaction", "institution", "beneficiary",
	"calculation", "reservation", "information", "description", "destination",
	"integration", "progression", "termination", "confirmation", 
	"subscription", "registration", "processing", "settlement",
	"accounting", "compliance", "validation", "permission", "preference",
	"credential", "parameter", "statement", "operation", "attributes", "attribute",
	// 8-9 char words
	"inventory", "reference", "transfer", "customer", "location",
	"address", "payment", "product", "message", "request", "response",
	"balance", "account", "exchange", "generate", "calculate", "terminate",
	"complete", "finalize", "activate", "validate", "deactivate",
	"authorize", "transmitter", "category", "currency", "receiver",
	"currency", "history",
	// 7 char words  
	"network", "partner", "process", "service", "detail", "summary",
	"pending", "blocked", "invoice", "receipt", "refund", "session",
	"storage", "version", "channel", "country", "default", "enabled",
	"expired", "failure", "success", "warning", "primary", "foreign",
	"general", "approve", "certify", "suspend", "escalate", "delegate",
	"global", "number", "string", "closed",
	// 6 char words
	"status", "result", "source", "target", "amount", "active",
	"locked", "return", "sender", "record", "report", "reject",
	"attest", "review", "assess", "resume", "cancel", "notify",
	"verify", "submit", "credit", "charge", "scheme", "entity",
	"action", "config", "domain", "format", "method", "payer",
	// 5 char words
	"event", "order", "query", "level", "limit", "total", "price",
	"error", "alert", "audit", "grant", "claim", "batch", "queue",
	"agent", "store", "index", "count", "value", "state", "input",
	"valid", "payee", "item", "note",
	// 4 char words
	"user", "role", "type", "code", "date", "time", "cart", "name",
	"rate", "xref", "send", "move", "find", "list", "open", "data",
	"mode", "plan", "rule", "step", "task", "unit", "zone", "area",
	"bank", "card", "case", "cash", "file", "flag", "flow", "form",
	"hash", "hold", "host", "kind", "last", "link", "loan", "mail",
	"mark", "memo", "meta", "next", "node", "page", "path", "port",
	"post", "rank", "risk", "sign", "size", "slot", "sort", "spec",
	"sync", "term", "text", "tier", "week", "year", "test", "prod",
	// 3 char words
	"add", "get", "set", "new", "old", "all", "any", "key", "log",
	"net", "pix", "tax", "fee", "ref", "max", "min", "sum", "avg",
	"day", "end", "row", "col", "seq", "msg", "err", "req",
	// 2 char words (last)
	"id", "to", "by", "of", "in", "on", "at", "is", "as", "or", "st",
}

// splitAllCapsIdentifier attempts to split an ALL_CAPS identifier into PascalCase
// using known words. Returns empty string if splitting fails.
// Example: "TRANSFEREVENTNOTE" → "TransferEventNote"
func splitAllCapsIdentifier(s string) string {
	if len(s) == 0 {
		return ""
	}
	
	lower := strings.ToLower(s)
	var result strings.Builder
	
	for len(lower) > 0 {
		matched := false
		
		// Try to match known words (longest first due to ordering)
		for _, word := range knownWords {
			if strings.HasPrefix(lower, word) {
				// Found a match - capitalize first letter
				result.WriteString(strings.ToUpper(word[:1]) + word[1:])
				lower = lower[len(word):]
				matched = true
				break
			}
		}
		
		if !matched {
			// No known word matched - check if remaining is very short
			if len(lower) <= 2 {
				// Just capitalize what's left (likely an abbreviation like "Id")
				result.WriteString(strings.ToUpper(lower[:1]) + lower[1:])
				lower = ""
			} else {
				// Can't split this identifier reliably
				return ""
			}
		}
	}
	
	return result.String()
}

// isAllCapsOrUnderscored returns true if the string is ALL_CAPS or ALL_CAPS_WITH_UNDERSCORES.
// This helps detect T-SQL naming conventions like ST_ADD_NOTIFICATION that need
// special handling for PascalCase conversion.
func isAllCapsOrUnderscored(s string) bool {
	if len(s) == 0 {
		return false
	}
	hasLetter := false
	for _, c := range s {
		if c >= 'a' && c <= 'z' {
			return false // Has lowercase, not ALL_CAPS
		}
		if c >= 'A' && c <= 'Z' {
			hasLetter = true
		}
	}
	return hasLetter // Must have at least one letter
}

func singularize(s string) string {
	lower := strings.ToLower(s)
	
	// Check suffix patterns (case-insensitive) but preserve original casing
	if strings.HasSuffix(lower, "ies") {
		return s[:len(s)-3] + "y"
	}
	if strings.HasSuffix(lower, "es") {
		return s[:len(s)-2]
	}
	if strings.HasSuffix(lower, "s") && !strings.HasSuffix(lower, "ss") {
		return s[:len(s)-1]
	}
	return s
}

func pluralize(s string) string {
	lower := strings.ToLower(s)
	
	// If already looks plural (ends in 's' but not 'ss', 'us', 'is'), return as-is
	// This handles cases like "Attributes" → "Attributes" (not "Attributeses")
	if strings.HasSuffix(lower, "s") && !strings.HasSuffix(lower, "ss") &&
		!strings.HasSuffix(lower, "us") && !strings.HasSuffix(lower, "is") {
		return s
	}
	
	if strings.HasSuffix(lower, "y") && len(s) > 1 {
		// Check if preceded by consonant
		prev := lower[len(lower)-2]
		if prev != 'a' && prev != 'e' && prev != 'i' && prev != 'o' && prev != 'u' {
			return s[:len(s)-1] + "ies"
		}
	}
	if strings.HasSuffix(lower, "x") || strings.HasSuffix(lower, "z") ||
		strings.HasSuffix(lower, "ch") || strings.HasSuffix(lower, "sh") ||
		strings.HasSuffix(lower, "ss") {
		return s + "es"
	}
	return s + "s"
}

// ============================================================================
// Temp Table / DDL Transpilation
// ============================================================================

// transpileCreateTable converts CREATE TABLE to Go code.
// For temp tables (#table), generates tsqlruntime.TempTableManager calls.
// For regular tables, generates SQL DDL.
func (t *transpiler) transpileCreateTable(s *ast.CreateTableStatement) (string, error) {
	dt := &dmlTranspiler{transpiler: t, config: t.dmlConfig}
	return dt.transpileCreateTable(s)
}

func (dt *dmlTranspiler) transpileCreateTable(s *ast.CreateTableStatement) (string, error) {
	tableName := s.Name.String()
	
	// Check if temp table
	isTempTable := strings.HasPrefix(tableName, "#")
	
	if isTempTable {
		return dt.transpileCreateTempTable(s)
	}
	
	// For regular tables, generate SQL DDL
	switch dt.config.Backend {
	case BackendSQL:
		return dt.transpileCreateTableSQL(s)
	case BackendMock:
		return dt.transpileCreateTempTable(s) // Use in-memory for mock
	default:
		return dt.transpileCreateTableSQL(s)
	}
}

// transpileCreateTempTable generates code using tsqlruntime.TempTableManager
func (dt *dmlTranspiler) transpileCreateTempTable(s *ast.CreateTableStatement) (string, error) {
	dt.imports["github.com/ha1tch/tgpiler/tsqlruntime"] = true
	
	tableName := s.Name.String()
	var out strings.Builder
	
	// Add TODO marker if requested
	if dt.emitTODOs() {
		out.WriteString("// TODO(tgpiler): Temp table uses in-memory tsqlruntime.TempTables - verify initialisation\n")
		out.WriteString(dt.indentStr())
	}
	
	// Generate column definitions
	out.WriteString("// CREATE TABLE " + tableName + "\n")
	out.WriteString("{\n")
	out.WriteString("\tcolumns := []tsqlruntime.TempTableColumn{\n")
	
	for _, col := range s.Columns {
		out.WriteString("\t\t{\n")
		out.WriteString(fmt.Sprintf("\t\t\tName: %q,\n", col.Name.Value))
		
		// Parse data type
		if col.DataType != nil {
			goType := dt.dataTypeToRuntimeType(col.DataType)
			out.WriteString(fmt.Sprintf("\t\t\tType: %s,\n", goType))
			
			if col.DataType.Precision != nil {
				out.WriteString(fmt.Sprintf("\t\t\tPrecision: %d,\n", *col.DataType.Precision))
			}
			if col.DataType.Scale != nil {
				out.WriteString(fmt.Sprintf("\t\t\tScale: %d,\n", *col.DataType.Scale))
			}
			if col.DataType.Length != nil {
				out.WriteString(fmt.Sprintf("\t\t\tMaxLen: %d,\n", *col.DataType.Length))
			} else if col.DataType.Max {
				out.WriteString("\t\t\tMaxLen: -1,\n")
			}
		}
		
		// Nullable
		if col.Nullable != nil {
			out.WriteString(fmt.Sprintf("\t\t\tNullable: %v,\n", *col.Nullable))
		} else {
			out.WriteString("\t\t\tNullable: true,\n")
		}
		
		// Identity
		if col.Identity != nil {
			out.WriteString("\t\t\tIdentity: true,\n")
			out.WriteString(fmt.Sprintf("\t\t\tIdentitySeed: %d,\n", col.Identity.Seed))
			out.WriteString(fmt.Sprintf("\t\t\tIdentityIncr: %d,\n", col.Identity.Increment))
		}
		
		out.WriteString("\t\t},\n")
	}
	
	out.WriteString("\t}\n")
	out.WriteString(fmt.Sprintf("\tif _, err := tempTables.CreateTempTable(%q, columns); err != nil {\n", tableName))
	out.WriteString("\t\t")
	out.WriteString(dt.buildErrorReturn())
	out.WriteString("\n")
	out.WriteString("\t}\n")
	out.WriteString("}")
	
	return out.String(), nil
}

// transpileCreateTableSQL generates SQL DDL for CREATE TABLE
func (dt *dmlTranspiler) transpileCreateTableSQL(s *ast.CreateTableStatement) (string, error) {
	var out strings.Builder
	
	tableName := s.Name.String()
	
	// Build the SQL
	sqlBuilder := strings.Builder{}
	sqlBuilder.WriteString("CREATE TABLE ")
	sqlBuilder.WriteString(tableName)
	sqlBuilder.WriteString(" (")
	
	for i, col := range s.Columns {
		if i > 0 {
			sqlBuilder.WriteString(", ")
		}
		sqlBuilder.WriteString(col.Name.Value)
		sqlBuilder.WriteString(" ")
		if col.DataType != nil {
			sqlBuilder.WriteString(col.DataType.String())
		}
		if col.Nullable != nil && !*col.Nullable {
			sqlBuilder.WriteString(" NOT NULL")
		}
		if col.Identity != nil {
			// Convert IDENTITY to dialect-specific syntax
			switch dt.config.SQLDialect {
			case "postgres":
				sqlBuilder.WriteString(" GENERATED ALWAYS AS IDENTITY")
			case "mysql":
				sqlBuilder.WriteString(" AUTO_INCREMENT")
			default:
				sqlBuilder.WriteString(fmt.Sprintf(" IDENTITY(%d,%d)", col.Identity.Seed, col.Identity.Increment))
			}
		}
	}
	sqlBuilder.WriteString(")")
	
	sql := sqlBuilder.String()
	
	out.WriteString(fmt.Sprintf("// CREATE TABLE %s\n", tableName))
	out.WriteString(fmt.Sprintf("if _, err := %s.ExecContext(ctx, %q); err != nil {\n", dt.config.StoreVar, sql))
	out.WriteString("\t" + dt.buildErrorReturn() + "\n")
	out.WriteString("}")
	
	return out.String(), nil
}

// transpileDropTable converts DROP TABLE to Go code
func (t *transpiler) transpileDropTable(s *ast.DropTableStatement) (string, error) {
	dt := &dmlTranspiler{transpiler: t, config: t.dmlConfig}
	return dt.transpileDropTable(s)
}

func (dt *dmlTranspiler) transpileDropTable(s *ast.DropTableStatement) (string, error) {
	var out strings.Builder
	
	for i, table := range s.Tables {
		tableName := table.String()
		isTempTable := strings.HasPrefix(tableName, "#")
		
		if i > 0 {
			out.WriteString("\n")
		}
		
		if isTempTable {
			dt.imports["github.com/ha1tch/tgpiler/tsqlruntime"] = true
			out.WriteString(fmt.Sprintf("// DROP TABLE %s\n", tableName))
			if s.IfExists {
				out.WriteString(fmt.Sprintf("_ = tempTables.DropTempTable(%q) // IF EXISTS\n", tableName))
			} else {
				out.WriteString(fmt.Sprintf("if err := tempTables.DropTempTable(%q); err != nil {\n", tableName))
				out.WriteString("\t")
				out.WriteString(dt.buildErrorReturn())
				out.WriteString("\n")
				out.WriteString("}")
			}
		} else {
			// Regular table - generate SQL
			sql := "DROP TABLE "
			if s.IfExists {
				sql += "IF EXISTS "
			}
			sql += tableName
			
			out.WriteString(fmt.Sprintf("// DROP TABLE %s\n", tableName))
			out.WriteString(fmt.Sprintf("if _, err := %s.ExecContext(ctx, %q); err != nil {\n", dt.config.StoreVar, sql))
			out.WriteString("\t")
			out.WriteString(dt.buildErrorReturn())
			out.WriteString("\n")
			out.WriteString("}")
		}
	}
	
	return out.String(), nil
}

// transpileTruncateTable converts TRUNCATE TABLE to Go code
func (t *transpiler) transpileTruncateTable(s *ast.TruncateTableStatement) (string, error) {
	dt := &dmlTranspiler{transpiler: t, config: t.dmlConfig}
	return dt.transpileTruncateTable(s)
}

func (dt *dmlTranspiler) transpileTruncateTable(s *ast.TruncateTableStatement) (string, error) {
	tableName := s.Table.String()
	isTempTable := strings.HasPrefix(tableName, "#")
	
	var out strings.Builder
	
	if isTempTable {
		dt.imports["github.com/ha1tch/tgpiler/tsqlruntime"] = true
		out.WriteString(fmt.Sprintf("// TRUNCATE TABLE %s\n", tableName))
		out.WriteString(fmt.Sprintf("if table, ok := tempTables.GetTempTable(%q); ok {\n", tableName))
		out.WriteString("\ttable.Truncate()\n")
		out.WriteString("}")
	} else {
		sql := "TRUNCATE TABLE " + tableName
		out.WriteString(fmt.Sprintf("// TRUNCATE TABLE %s\n", tableName))
		out.WriteString(fmt.Sprintf("if _, err := %s.ExecContext(ctx, %q); err != nil {\n", dt.config.StoreVar, sql))
		out.WriteString("\t" + dt.buildErrorReturn() + "\n")
		out.WriteString("}")
	}
	
	return out.String(), nil
}

// dataTypeToRuntimeType converts AST DataType to tsqlruntime type constant
func (dt *dmlTranspiler) dataTypeToRuntimeType(dataType *ast.DataType) string {
	if dataType == nil {
		return "tsqlruntime.TypeVarChar"
	}
	
	switch strings.ToUpper(dataType.Name) {
	case "INT", "INTEGER":
		return "tsqlruntime.TypeInt"
	case "BIGINT":
		return "tsqlruntime.TypeBigInt"
	case "SMALLINT":
		return "tsqlruntime.TypeSmallInt"
	case "TINYINT":
		return "tsqlruntime.TypeTinyInt"
	case "BIT":
		return "tsqlruntime.TypeBit"
	case "DECIMAL", "NUMERIC":
		return "tsqlruntime.TypeDecimal"
	case "MONEY":
		return "tsqlruntime.TypeMoney"
	case "SMALLMONEY":
		return "tsqlruntime.TypeSmallMoney"
	case "FLOAT":
		return "tsqlruntime.TypeFloat"
	case "REAL":
		return "tsqlruntime.TypeReal"
	case "VARCHAR":
		return "tsqlruntime.TypeVarChar"
	case "NVARCHAR":
		return "tsqlruntime.TypeNVarChar"
	case "CHAR":
		return "tsqlruntime.TypeChar"
	case "NCHAR":
		return "tsqlruntime.TypeNChar"
	case "TEXT", "NTEXT":
		return "tsqlruntime.TypeVarChar"
	case "DATE":
		return "tsqlruntime.TypeDate"
	case "TIME":
		return "tsqlruntime.TypeTime"
	case "DATETIME", "DATETIME2":
		return "tsqlruntime.TypeDateTime"
	case "SMALLDATETIME":
		return "tsqlruntime.TypeSmallDateTime"
	case "BINARY", "VARBINARY", "IMAGE":
		return "tsqlruntime.TypeVarBinary"
	case "UNIQUEIDENTIFIER":
		return "tsqlruntime.TypeUniqueIdentifier"
	case "XML":
		return "tsqlruntime.TypeXML"
	default:
		return "tsqlruntime.TypeVarChar"
	}
}