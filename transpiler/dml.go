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

	// SQL dialect (postgres, mysql, sqlite, sqlserver)
	SQLDialect string

	// Repository/store variable name (e.g., "r.db", "r.store", "r.client")
	StoreVar string

	// Whether to use transactions
	UseTransactions bool

	// gRPC service mappings (procedure -> service.method)
	GRPCMappings map[string]string

	// Proto package for gRPC
	ProtoPackage string
}

// DefaultDMLConfig returns sensible defaults.
func DefaultDMLConfig() DMLConfig {
	return DMLConfig{
		Backend:         BackendSQL,
		SQLDialect:      "postgres",
		StoreVar:        "r.db",
		UseTransactions: false,
		GRPCMappings:    make(map[string]string),
	}
}

// dmlTranspiler handles DML statement conversion.
type dmlTranspiler struct {
	*transpiler
	config DMLConfig
}

// buildErrorReturn generates a return statement with error for DML operations
func (dt *dmlTranspiler) buildErrorReturn() string {
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
	switch dt.config.Backend {
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

	// Build query
	query, args := dt.buildSelectQuery(s)
	
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
	// For gRPC, we need to map the SELECT to a service method
	// This requires knowing which service/method corresponds to this query

	// Extract table name to determine service
	tableName := dt.extractMainTable(s)
	methodName := dt.inferGRPCMethod(s, tableName)

	var out strings.Builder
	out.WriteString(fmt.Sprintf("// gRPC call: %s\n", methodName))
	out.WriteString(dt.indentStr())
	out.WriteString(fmt.Sprintf("resp, err := %s.%s(ctx, &%s.%sRequest{\n",
		dt.config.StoreVar, methodName, dt.config.ProtoPackage, methodName))

	// Add request fields from WHERE clause
	whereFields := dt.extractWhereFields(s)
	for _, wf := range whereFields {
		out.WriteString(dt.indentStr())
		out.WriteString(fmt.Sprintf("\t%s: %s,\n", goExportedIdentifier(wf.column), wf.variable))
	}

	out.WriteString(dt.indentStr())
	out.WriteString("})\n")
	out.WriteString(dt.indentStr())
	out.WriteString("if err != nil {\n")
	out.WriteString(dt.indentStr())
	out.WriteString("\treturn err\n")
	out.WriteString(dt.indentStr())
	out.WriteString("}\n")
	out.WriteString(dt.indentStr())
	out.WriteString("_ = resp // TODO: use response")

	return out.String(), nil
}

// transpileSelectMock generates mock store code for SELECT.
func (dt *dmlTranspiler) transpileSelectMock(s *ast.SelectStatement) (string, error) {
	tableName := dt.extractMainTable(s)
	methodName := dt.inferMockMethod(s, tableName)

	var out strings.Builder
	out.WriteString(fmt.Sprintf("result, err := %s.%s(", dt.config.StoreVar, methodName))

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
	out.WriteString("\treturn err\n")
	out.WriteString(dt.indentStr())
	out.WriteString("}\n")
	out.WriteString(dt.indentStr())
	out.WriteString("_ = result // TODO: use result")

	return out.String(), nil
}

// transpileSelectInline generates inline SQL string.
func (dt *dmlTranspiler) transpileSelectInline(s *ast.SelectStatement) (string, error) {
	query, args := dt.buildSelectQuery(s)

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
	switch dt.config.Backend {
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

	query, args := dt.buildInsertQuery(s)
	
	// Get the database variable (tx if in transaction, StoreVar otherwise)
	dbVar := dt.getDBVar()

	// Check for OUTPUT clause (SQL Server) or RETURNING (PostgreSQL)
	hasOutput := s.Output != nil

	out.WriteString("// INSERT query\n")
	out.WriteString(dt.indentStr())

	if hasOutput && dt.config.SQLDialect == "postgres" {
		// PostgreSQL: use RETURNING
		out.WriteString(fmt.Sprintf("row := %s.QueryRowContext(ctx, %q", dbVar, query))
		for _, arg := range args {
			out.WriteString(", " + arg)
		}
		out.WriteString(")\n")
		out.WriteString(dt.indentStr())
		out.WriteString("if err := row.Scan(/* TODO: RETURNING columns */); err != nil {\n")
		out.WriteString(dt.indentStr())
		out.WriteString("\treturn err\n")
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
		out.WriteString("\t")
		out.WriteString(dt.buildErrorReturn())
		out.WriteString("\n")
		out.WriteString(dt.indentStr())
		out.WriteString("}\n")
		out.WriteString(dt.indentStr())
		out.WriteString("_ = result // Use result.LastInsertId() if needed")
	}

	return out.String(), nil
}

func (dt *dmlTranspiler) transpileInsertGRPC(s *ast.InsertStatement) (string, error) {
	tableName := dt.extractInsertTable(s)
	methodName := "Create" + toPascalCase(singularize(tableName))

	var out strings.Builder
	out.WriteString(fmt.Sprintf("// gRPC call: %s\n", methodName))
	out.WriteString(dt.indentStr())
	out.WriteString(fmt.Sprintf("resp, err := %s.%s(ctx, &%s.%sRequest{\n",
		dt.config.StoreVar, methodName, dt.config.ProtoPackage, methodName))

	// Add request fields from INSERT columns/values
	insertFields := dt.extractInsertFields(s)
	for _, f := range insertFields {
		out.WriteString(dt.indentStr())
		out.WriteString(fmt.Sprintf("\t%s: %s,\n", goExportedIdentifier(f.column), f.value))
	}

	out.WriteString(dt.indentStr())
	out.WriteString("})\n")
	out.WriteString(dt.indentStr())
	out.WriteString("if err != nil {\n")
	out.WriteString(dt.indentStr())
	out.WriteString("\treturn err\n")
	out.WriteString(dt.indentStr())
	out.WriteString("}\n")
	out.WriteString(dt.indentStr())
	out.WriteString("_ = resp")

	return out.String(), nil
}

func (dt *dmlTranspiler) transpileInsertMock(s *ast.InsertStatement) (string, error) {
	tableName := dt.extractInsertTable(s)
	methodName := "Create" + toPascalCase(singularize(tableName))

	var out strings.Builder
	out.WriteString(fmt.Sprintf("result, err := %s.%s(", dt.config.StoreVar, methodName))

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
	out.WriteString("\treturn err\n")
	out.WriteString(dt.indentStr())
	out.WriteString("}\n")
	out.WriteString(dt.indentStr())
	out.WriteString("_ = result")

	return out.String(), nil
}

// transpileUpdate converts an UPDATE statement to Go code.
func (t *transpiler) transpileUpdate(s *ast.UpdateStatement) (string, error) {
	dt := &dmlTranspiler{transpiler: t, config: t.dmlConfig}
	return dt.transpileUpdate(s)
}

func (dt *dmlTranspiler) transpileUpdate(s *ast.UpdateStatement) (string, error) {
	switch dt.config.Backend {
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

	query, args := dt.buildUpdateQuery(s)
	
	// Get the database variable (tx if in transaction, StoreVar otherwise)
	dbVar := dt.getDBVar()

	out.WriteString("// UPDATE query\n")
	out.WriteString(dt.indentStr())
	out.WriteString(fmt.Sprintf("result, err := %s.ExecContext(ctx, %q", dbVar, query))
	for _, arg := range args {
		out.WriteString(", " + arg)
	}
	out.WriteString(")\n")
	out.WriteString(dt.indentStr())
	out.WriteString("if err != nil {\n")
	out.WriteString(dt.indentStr())
	out.WriteString("\treturn err\n")
	out.WriteString(dt.indentStr())
	out.WriteString("}\n")
	out.WriteString(dt.indentStr())
	out.WriteString("_ = result // Use result.RowsAffected() if needed")

	return out.String(), nil
}

func (dt *dmlTranspiler) transpileUpdateGRPC(s *ast.UpdateStatement) (string, error) {
	tableName := dt.extractUpdateTable(s)
	methodName := "Update" + toPascalCase(singularize(tableName))

	var out strings.Builder
	out.WriteString(fmt.Sprintf("// gRPC call: %s\n", methodName))
	out.WriteString(dt.indentStr())
	out.WriteString(fmt.Sprintf("resp, err := %s.%s(ctx, &%s.%sRequest{\n",
		dt.config.StoreVar, methodName, dt.config.ProtoPackage, methodName))

	// Add SET fields
	setFields := dt.extractUpdateSetFields(s)
	for _, f := range setFields {
		out.WriteString(dt.indentStr())
		out.WriteString(fmt.Sprintf("\t%s: %s,\n", goExportedIdentifier(f.column), f.value))
	}

	// Add WHERE fields (for identifying the record)
	whereFields := dt.extractWhereFieldsFromUpdate(s)
	for _, wf := range whereFields {
		out.WriteString(dt.indentStr())
		out.WriteString(fmt.Sprintf("\t%s: %s,\n", goExportedIdentifier(wf.column), wf.variable))
	}

	out.WriteString(dt.indentStr())
	out.WriteString("})\n")
	out.WriteString(dt.indentStr())
	out.WriteString("if err != nil {\n")
	out.WriteString(dt.indentStr())
	out.WriteString("\treturn err\n")
	out.WriteString(dt.indentStr())
	out.WriteString("}\n")
	out.WriteString(dt.indentStr())
	out.WriteString("_ = resp")

	return out.String(), nil
}

func (dt *dmlTranspiler) transpileUpdateMock(s *ast.UpdateStatement) (string, error) {
	tableName := dt.extractUpdateTable(s)
	methodName := "Update" + toPascalCase(singularize(tableName))

	var out strings.Builder
	out.WriteString(fmt.Sprintf("err := %s.%s(", dt.config.StoreVar, methodName))

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
	out.WriteString("\treturn err\n")
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
	switch dt.config.Backend {
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

	query, args := dt.buildDeleteQuery(s)
	
	// Get the database variable (tx if in transaction, StoreVar otherwise)
	dbVar := dt.getDBVar()

	out.WriteString("// DELETE query\n")
	out.WriteString(dt.indentStr())
	out.WriteString(fmt.Sprintf("result, err := %s.ExecContext(ctx, %q", dbVar, query))
	for _, arg := range args {
		out.WriteString(", " + arg)
	}
	out.WriteString(")\n")
	out.WriteString(dt.indentStr())
	out.WriteString("if err != nil {\n")
	out.WriteString(dt.indentStr())
	out.WriteString("\treturn err\n")
	out.WriteString(dt.indentStr())
	out.WriteString("}\n")
	out.WriteString(dt.indentStr())
	out.WriteString("_ = result // Use result.RowsAffected() if needed")

	return out.String(), nil
}

func (dt *dmlTranspiler) transpileDeleteGRPC(s *ast.DeleteStatement) (string, error) {
	tableName := dt.extractDeleteTable(s)
	methodName := "Delete" + toPascalCase(singularize(tableName))

	var out strings.Builder
	out.WriteString(fmt.Sprintf("// gRPC call: %s\n", methodName))
	out.WriteString(dt.indentStr())
	out.WriteString(fmt.Sprintf("resp, err := %s.%s(ctx, &%s.%sRequest{\n",
		dt.config.StoreVar, methodName, dt.config.ProtoPackage, methodName))

	// Add WHERE fields (for identifying the record)
	whereFields := dt.extractWhereFieldsFromDelete(s)
	for _, wf := range whereFields {
		out.WriteString(dt.indentStr())
		out.WriteString(fmt.Sprintf("\t%s: %s,\n", goExportedIdentifier(wf.column), wf.variable))
	}

	out.WriteString(dt.indentStr())
	out.WriteString("})\n")
	out.WriteString(dt.indentStr())
	out.WriteString("if err != nil {\n")
	out.WriteString(dt.indentStr())
	out.WriteString("\treturn err\n")
	out.WriteString(dt.indentStr())
	out.WriteString("}\n")
	out.WriteString(dt.indentStr())
	out.WriteString("_ = resp")

	return out.String(), nil
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
	out.WriteString("\treturn err\n")
	out.WriteString(dt.indentStr())
	out.WriteString("}")

	return out.String(), nil
}

// transpileExec converts an EXEC/EXECUTE statement to a Go function call.
func (t *transpiler) transpileExec(s *ast.ExecStatement) (string, error) {
	dt := &dmlTranspiler{transpiler: t, config: t.dmlConfig}
	return dt.transpileExec(s)
}

func (dt *dmlTranspiler) transpileExec(s *ast.ExecStatement) (string, error) {
	// EXEC calls another stored procedure
	// In transpiled code, this becomes a Go function call
	procName := ""
	if s.Procedure != nil {
		procName = s.Procedure.String()
	}

	// Clean up procedure name (remove dbo. prefix, etc.)
	procName = cleanProcedureName(procName)
	funcName := goExportedIdentifier(procName)

	var out strings.Builder
	out.WriteString(fmt.Sprintf("// EXEC %s\n", procName))
	out.WriteString(dt.indentStr())

	// Check if result variable is assigned
	hasResultVar := s.ReturnVariable != nil

	// Build argument list
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

	// Generate the call
	if hasResultVar {
		resultVar := goIdentifier(s.ReturnVariable.Value)
		out.WriteString(fmt.Sprintf("%s = %s(%s)", resultVar, funcName, strings.Join(args, ", ")))
	} else {
		// Check for OUTPUT params that need to capture return values
		var outputVars []string
		for _, p := range s.Parameters {
			if p.Output {
				varName := ""
				if v, ok := p.Value.(*ast.Variable); ok {
					varName = goIdentifier(strings.TrimPrefix(v.Name, "@"))
				}
				if varName != "" {
					outputVars = append(outputVars, varName)
				}
			}
		}

		// Build non-output args
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
			assignments = append(assignments, varAssignment{
				varName: varName,
				column:  dt.exprToString(item.Expression),
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

// Query building helpers

func (dt *dmlTranspiler) buildSelectQuery(s *ast.SelectStatement) (string, []string) {
	// Build dialect-appropriate SELECT query
	var query strings.Builder
	var args []string
	argNum := 1

	query.WriteString("SELECT ")

	// Columns
	if s.Columns != nil {
		var cols []string
		for _, item := range s.Columns {
			cols = append(cols, item.String())
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

	return query.String(), args
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

	// VALUES
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
	}

	return query.String(), args
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

	return query.String(), args
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

	return query.String(), args
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

func (dt *dmlTranspiler) inferGRPCMethod(s *ast.SelectStatement, table string) string {
	whereFields := dt.extractWhereFields(s)

	if len(whereFields) == 0 {
		return "List" + toPascalCase(pluralize(table))
	}

	if len(whereFields) == 1 {
		col := whereFields[0].column
		if strings.ToLower(col) == "id" || strings.HasSuffix(strings.ToLower(col), "_id") {
			return "Get" + toPascalCase(singularize(table))
		}
		return "Get" + toPascalCase(singularize(table)) + "By" + toPascalCase(col)
	}

	return "Find" + toPascalCase(pluralize(table))
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
	out.WriteString("\treturn err\n")
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
	out.WriteString("\treturn err\n")
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
	name  string // column name or alias
	expr  string // original expression
	alias string // AS alias if present
}

// extractSelectColumns extracts column names from SELECT clause
func (dt *dmlTranspiler) extractSelectColumns(s *ast.SelectStatement) []selectColumn {
	var columns []selectColumn
	
	if s.Columns == nil {
		return columns
	}
	
	for _, item := range s.Columns {
		col := selectColumn{}
		
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
		
		// Infer type (default to interface{} since we don't have schema)
		goType := "interface{}"
		
		// Try to infer type from name patterns
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
	result := make([]byte, 0, len(s))
	capitalizeNext := true

	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '_' || c == '-' || c == ' ' {
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

func singularize(s string) string {
	s = strings.ToLower(s)
	if strings.HasSuffix(s, "ies") {
		return s[:len(s)-3] + "y"
	}
	if strings.HasSuffix(s, "es") {
		return s[:len(s)-2]
	}
	if strings.HasSuffix(s, "s") && !strings.HasSuffix(s, "ss") {
		return s[:len(s)-1]
	}
	return s
}

func pluralize(s string) string {
	s = strings.ToLower(s)
	if strings.HasSuffix(s, "y") && len(s) > 1 {
		// Check if preceded by consonant
		prev := s[len(s)-2]
		if prev != 'a' && prev != 'e' && prev != 'i' && prev != 'o' && prev != 'u' {
			return s[:len(s)-1] + "ies"
		}
	}
	if strings.HasSuffix(s, "s") || strings.HasSuffix(s, "x") ||
		strings.HasSuffix(s, "z") || strings.HasSuffix(s, "ch") ||
		strings.HasSuffix(s, "sh") {
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
	out.WriteString("\treturn err\n")
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
		out.WriteString("\treturn err\n")
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
