package tsqlruntime

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/ha1tch/tsqlparser/ast"
	"github.com/ha1tch/tsqlparser/lexer"
	"github.com/ha1tch/tsqlparser/parser"
)

// Dialect represents the target SQL dialect
type Dialect int

const (
	DialectGeneric Dialect = iota
	DialectPostgres
	DialectMySQL
	DialectSQLite
	DialectSQLServer
)

// ExecutionResult contains the results of executing dynamic SQL
type ExecutionResult struct {
	ResultSets   []ResultSet
	RowsAffected int64
	LastInsertID int64
	ReturnValue  *int64
	Error        *SQLError
}

// ResultSet represents a single result set from a query
type ResultSet struct {
	Columns []string
	Rows    [][]Value
}

// Interpreter executes T-SQL dynamically
type Interpreter struct {
	ctx       *ExecutionContext
	evaluator *ExpressionEvaluator
	ddl       *DDLHandler

	// Options
	Debug bool
}

// NewInterpreter creates a new T-SQL interpreter
func NewInterpreter(db *sql.DB, dialect Dialect) *Interpreter {
	ctx := NewExecutionContext(db, dialect)
	i := &Interpreter{
		ctx:       ctx,
		evaluator: NewExpressionEvaluator(),
	}
	i.ddl = NewDDLHandler(ctx)
	return i
}

// NewInterpreterWithContext creates an interpreter with an existing context
func NewInterpreterWithContext(ctx *ExecutionContext) *Interpreter {
	i := &Interpreter{
		ctx:       ctx,
		evaluator: NewExpressionEvaluator(),
	}
	i.ddl = NewDDLHandler(ctx)
	return i
}

// SetTransaction sets the transaction for execution
func (i *Interpreter) SetTransaction(tx *sql.Tx) {
	i.ctx.Tx = tx
}

// SetVariable sets a variable value
func (i *Interpreter) SetVariable(name string, value interface{}) {
	v := ToValue(value)
	i.evaluator.SetVariable(name, v)
	i.ctx.SetVariable(name, v)
}

// GetVariable gets a variable value
func (i *Interpreter) GetVariable(name string) (interface{}, bool) {
	v, ok := i.ctx.GetVariable(name)
	if !ok {
		v, ok = i.evaluator.GetVariable(name)
		if !ok {
			return nil, false
		}
	}
	return FromValue(v), true
}

// Execute parses and executes dynamic SQL
func (i *Interpreter) Execute(ctx context.Context, sqlStr string, params map[string]interface{}) (*ExecutionResult, error) {
	// Set parameters as variables
	for name, val := range params {
		v := ToValue(val)
		i.evaluator.SetVariable(name, v)
		i.ctx.SetVariable(name, v)
	}

	// Parse SQL
	l := lexer.New(sqlStr)
	p := parser.New(l)
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		return nil, fmt.Errorf("parse error: %s", p.Errors()[0])
	}

	result := &ExecutionResult{}

	// Execute each statement
	for _, stmt := range program.Statements {
		if err := i.executeStatement(ctx, stmt, result); err != nil {
			// Check if we're in a TRY block
			if i.ctx.ErrorHandler.HandleError(err) {
				// Error was caught, continue to CATCH block if available
				continue
			}
			return nil, err
		}
		
		// Check for RETURN
		if i.ctx.HasReturned {
			if i.ctx.ReturnValue != nil {
				retVal := i.ctx.ReturnValue.AsInt()
				result.ReturnValue = &retVal
			}
			break
		}
	}

	result.RowsAffected = i.ctx.RowCount
	result.LastInsertID = i.ctx.LastInsertID
	result.ResultSets = i.ctx.ResultSets

	return result, nil
}

// ExecuteQuery executes a single query and returns results
func (i *Interpreter) ExecuteQuery(ctx context.Context, sql string, params map[string]interface{}) (*ResultSet, error) {
	result, err := i.Execute(ctx, sql, params)
	if err != nil {
		return nil, err
	}
	if len(result.ResultSets) > 0 {
		return &result.ResultSets[0], nil
	}
	return &ResultSet{}, nil
}

// ExecuteScalar executes a query and returns the first column of the first row
func (i *Interpreter) ExecuteScalar(ctx context.Context, sql string, params map[string]interface{}) (interface{}, error) {
	rs, err := i.ExecuteQuery(ctx, sql, params)
	if err != nil {
		return nil, err
	}
	if len(rs.Rows) > 0 && len(rs.Rows[0]) > 0 {
		return FromValue(rs.Rows[0][0]), nil
	}
	return nil, nil
}

// ExecuteNonQuery executes a non-query statement and returns rows affected
func (i *Interpreter) ExecuteNonQuery(ctx context.Context, sql string, params map[string]interface{}) (int64, error) {
	result, err := i.Execute(ctx, sql, params)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected, nil
}

func (i *Interpreter) executeStatement(ctx context.Context, stmt ast.Statement, result *ExecutionResult) error {
	if i.Debug {
		fmt.Printf("Executing: %T\n", stmt)
	}

	switch s := stmt.(type) {
	case *ast.SelectStatement:
		return i.executeSelect(ctx, s, result)

	case *ast.InsertStatement:
		return i.executeInsert(ctx, s)

	case *ast.UpdateStatement:
		return i.executeUpdate(ctx, s)

	case *ast.DeleteStatement:
		return i.executeDelete(ctx, s)

	case *ast.SetStatement:
		return i.executeSet(s)

	case *ast.DeclareStatement:
		return i.executeDeclare(s)

	case *ast.PrintStatement:
		return i.executePrint(s)

	case *ast.ExecStatement:
		// Recursive dynamic SQL execution
		return i.executeExec(ctx, s, result)

	case *ast.IfStatement:
		return i.executeIf(ctx, s, result)

	case *ast.WhileStatement:
		return i.executeWhile(ctx, s, result)

	case *ast.BeginEndBlock:
		for _, inner := range s.Statements {
			if err := i.executeStatement(ctx, inner, result); err != nil {
				return err
			}
		}
		return nil

	case *ast.ReturnStatement:
		// Handle RETURN statement
		if s.Value != nil {
			val, err := i.evaluator.Evaluate(s.Value)
			if err != nil {
				return err
			}
			i.ctx.ReturnValue = &val
		}
		i.ctx.HasReturned = true
		return nil

	case *ast.TryCatchStatement:
		return i.executeTryCatch(ctx, s, result)

	case *ast.CreateTableStatement:
		return i.ddl.ExecuteCreateTable(s)

	case *ast.DropTableStatement:
		return i.ddl.ExecuteDropTable(s)

	case *ast.TruncateTableStatement:
		return i.ddl.ExecuteTruncateTable(s)

	case *ast.BeginTransactionStatement:
		return i.ctx.BeginTransaction(ctx)

	case *ast.CommitTransactionStatement:
		return i.ctx.CommitTransaction()

	case *ast.RollbackTransactionStatement:
		return i.ctx.RollbackTransaction()

	case *ast.RaiserrorStatement:
		return i.executeRaiserror(s)

	case *ast.ThrowStatement:
		return i.executeThrow(s)

	// Stage 3: Cursor statements
	case *ast.DeclareCursorStatement:
		return i.executeDeclareCursor(ctx, s)

	case *ast.OpenCursorStatement:
		return i.executeOpenCursor(ctx, s, result)

	case *ast.FetchStatement:
		return i.executeFetch(ctx, s)

	case *ast.CloseCursorStatement:
		return i.executeCloseCursor(s)

	case *ast.DeallocateCursorStatement:
		return i.executeDeallocateCursor(s)

	default:
		return fmt.Errorf("unsupported statement type: %T", stmt)
	}
}

func (i *Interpreter) executeSelect(ctx context.Context, s *ast.SelectStatement, result *ExecutionResult) error {
	// Check for SELECT INTO #temp
	if s.Into != nil {
		return i.executeSelectInto(ctx, s, result)
	}

	// Check if selecting from a temp table
	if i.isSelectFromTempTable(s) {
		return i.executeSelectFromTempTable(ctx, s, result)
	}

	// Build the query
	query, args, err := i.buildSelectQuery(s)
	if err != nil {
		return err
	}

	if i.Debug {
		fmt.Printf("Query: %s\nArgs: %v\n", query, args)
	}

	// Execute query
	var rows *sql.Rows
	if i.ctx.Tx != nil {
		rows, err = i.ctx.Tx.QueryContext(ctx, query, args...)
	} else {
		rows, err = i.ctx.DB.QueryContext(ctx, query, args...)
	}
	if err != nil {
		return fmt.Errorf("query error: %w", err)
	}
	defer rows.Close()

	// Get column info
	columns, err := rows.Columns()
	if err != nil {
		return err
	}

	rs := ResultSet{Columns: columns}

	// Scan rows
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for j := range values {
			valuePtrs[j] = &values[j]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return err
		}

		row := make([]Value, len(columns))
		for j, v := range values {
			row[j] = ToValue(v)
		}
		rs.Rows = append(rs.Rows, row)
	}

	result.ResultSets = append(result.ResultSets, rs)
	i.ctx.UpdateRowCount(int64(len(rs.Rows)))
	i.ctx.AddResultSet(rs)

	return rows.Err()
}

func (i *Interpreter) executeInsert(ctx context.Context, s *ast.InsertStatement) error {
	// Check if inserting into a temp table
	tableName := ""
	if s.Table != nil {
		tableName = s.Table.String()
	}

	if IsTempTable(tableName) || IsTableVariable(tableName) {
		return i.executeInsertIntoTempTable(ctx, s)
	}

	query, args, err := i.buildInsertQuery(s)
	if err != nil {
		return err
	}

	if i.Debug {
		fmt.Printf("Insert: %s\nArgs: %v\n", query, args)
	}

	var res sql.Result
	if i.ctx.Tx != nil {
		res, err = i.ctx.Tx.ExecContext(ctx, query, args...)
	} else {
		res, err = i.ctx.DB.ExecContext(ctx, query, args...)
	}
	if err != nil {
		return fmt.Errorf("insert error: %w", err)
	}

	rowsAffected, _ := res.RowsAffected()
	lastInsertID, _ := res.LastInsertId()
	i.ctx.UpdateRowCount(rowsAffected)
	i.ctx.UpdateLastInsertID(lastInsertID)

	return nil
}

func (i *Interpreter) executeUpdate(ctx context.Context, s *ast.UpdateStatement) error {
	// Check if updating a temp table
	tableName := ""
	if s.Table != nil {
		tableName = s.Table.String()
	}

	if IsTempTable(tableName) || IsTableVariable(tableName) {
		return i.executeUpdateTempTable(ctx, s)
	}

	query, args, err := i.buildUpdateQuery(s)
	if err != nil {
		return err
	}

	if i.Debug {
		fmt.Printf("Update: %s\nArgs: %v\n", query, args)
	}

	var res sql.Result
	if i.ctx.Tx != nil {
		res, err = i.ctx.Tx.ExecContext(ctx, query, args...)
	} else {
		res, err = i.ctx.DB.ExecContext(ctx, query, args...)
	}
	if err != nil {
		return fmt.Errorf("update error: %w", err)
	}

	rowsAffected, _ := res.RowsAffected()
	i.ctx.UpdateRowCount(rowsAffected)

	return nil
}

func (i *Interpreter) executeDelete(ctx context.Context, s *ast.DeleteStatement) error {
	// Check if deleting from a temp table
	tableName := ""
	if s.Table != nil {
		tableName = s.Table.String()
	}

	if IsTempTable(tableName) || IsTableVariable(tableName) {
		return i.executeDeleteFromTempTable(ctx, s)
	}

	query, args, err := i.buildDeleteQuery(s)
	if err != nil {
		return err
	}

	if i.Debug {
		fmt.Printf("Delete: %s\nArgs: %v\n", query, args)
	}

	var res sql.Result
	if i.ctx.Tx != nil {
		res, err = i.ctx.Tx.ExecContext(ctx, query, args...)
	} else {
		res, err = i.ctx.DB.ExecContext(ctx, query, args...)
	}
	if err != nil {
		return fmt.Errorf("delete error: %w", err)
	}

	rowsAffected, _ := res.RowsAffected()
	i.ctx.UpdateRowCount(rowsAffected)

	return nil
}

func (i *Interpreter) executeSet(s *ast.SetStatement) error {
	if s.Variable == nil || s.Value == nil {
		// Handle SET options like SET NOCOUNT ON
		if s.Option != "" {
			// Could track options but for now just acknowledge
			return nil
		}
		return nil
	}

	value, err := i.evaluator.Evaluate(s.Value)
	if err != nil {
		return err
	}

	// Extract variable name
	var name string
	switch v := s.Variable.(type) {
	case *ast.Variable:
		name = v.Name
	case *ast.Identifier:
		name = v.Value
	default:
		return fmt.Errorf("unsupported variable type in SET: %T", s.Variable)
	}
	
	i.evaluator.SetVariable(name, value)
	return nil
}

func (i *Interpreter) executeDeclare(s *ast.DeclareStatement) error {
	for _, v := range s.Variables {
		// Initialize with NULL or default value
		var value Value
		if v.Value != nil {
			var err error
			value, err = i.evaluator.Evaluate(v.Value)
			if err != nil {
				return err
			}
		} else {
			// Determine type and set NULL
			dt := TypeVarChar
			if v.DataType != nil {
				dt, _, _, _ = ParseDataType(v.DataType.Name)
			}
			value = Null(dt)
		}
		i.evaluator.SetVariable(v.Name, value)
	}
	return nil
}

func (i *Interpreter) executePrint(s *ast.PrintStatement) error {
	if s.Expression == nil {
		return nil
	}

	value, err := i.evaluator.Evaluate(s.Expression)
	if err != nil {
		return err
	}

	fmt.Println(value.AsString())
	return nil
}

func (i *Interpreter) executeExec(ctx context.Context, s *ast.ExecStatement, result *ExecutionResult) error {
	// Handle EXEC(@sql)
	if s.DynamicSQL != nil {
		sqlVal, err := i.evaluator.Evaluate(s.DynamicSQL)
		if err != nil {
			return err
		}

		// Recursively execute the dynamic SQL
		return i.executeNestedSQL(ctx, sqlVal.AsString(), result)
	}

	// Handle EXEC sp_executesql
	if s.Procedure != nil {
		procName := strings.ToUpper(s.Procedure.String())
		if procName == "SP_EXECUTESQL" || strings.HasSuffix(procName, ".SP_EXECUTESQL") {
			return i.executeSpExecuteSQL(ctx, s.Parameters, result)
		}
	}

	return fmt.Errorf("procedure execution not supported: %v", s.Procedure)
}

func (i *Interpreter) executeNestedSQL(ctx context.Context, sql string, result *ExecutionResult) error {
	l := lexer.New(sql)
	p := parser.New(l)
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		return fmt.Errorf("nested SQL parse error: %s", p.Errors()[0])
	}

	for _, stmt := range program.Statements {
		if err := i.executeStatement(ctx, stmt, result); err != nil {
			return err
		}
	}
	return nil
}

func (i *Interpreter) executeSpExecuteSQL(ctx context.Context, params []*ast.ExecParameter, result *ExecutionResult) error {
	if len(params) < 1 {
		return fmt.Errorf("sp_executesql requires at least 1 parameter")
	}

	// First parameter is the SQL string
	sqlVal, err := i.evaluator.Evaluate(params[0].Value)
	if err != nil {
		return err
	}
	sql := sqlVal.AsString()

	// Second parameter is parameter definitions (optional)
	// Third+ parameters are the actual values
	// For now, assume parameters are already set as variables

	// Parse and map parameters if provided
	if len(params) >= 3 {
		// params[1] is the parameter definition string like N'@p1 int, @p2 varchar(50)'
		// params[2+] are the actual values
		paramDef, err := i.evaluator.Evaluate(params[1].Value)
		if err != nil {
			return err
		}

		// Parse parameter names from definition
		paramNames := parseParamDef(paramDef.AsString())

		// Set parameter values
		for j := 2; j < len(params) && j-2 < len(paramNames); j++ {
			val, err := i.evaluator.Evaluate(params[j].Value)
			if err != nil {
				return err
			}
			i.evaluator.SetVariable(paramNames[j-2], val)
		}
	}

	return i.executeNestedSQL(ctx, sql, result)
}

// parseParamDef parses a sp_executesql parameter definition string
func parseParamDef(def string) []string {
	var names []string
	parts := strings.Split(def, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		// Format: @name type
		tokens := strings.Fields(part)
		if len(tokens) >= 1 {
			name := strings.TrimPrefix(tokens[0], "@")
			names = append(names, name)
		}
	}
	return names
}

func (i *Interpreter) executeIf(ctx context.Context, s *ast.IfStatement, result *ExecutionResult) error {
	cond, err := i.evaluator.Evaluate(s.Condition)
	if err != nil {
		return err
	}

	if cond.IsTruthy() {
		return i.executeStatement(ctx, s.Consequence, result)
	} else if s.Alternative != nil {
		return i.executeStatement(ctx, s.Alternative, result)
	}
	return nil
}

func (i *Interpreter) executeWhile(ctx context.Context, s *ast.WhileStatement, result *ExecutionResult) error {
	maxIterations := 10000 // Safety limit
	for iter := 0; iter < maxIterations; iter++ {
		cond, err := i.evaluator.Evaluate(s.Condition)
		if err != nil {
			return err
		}

		if !cond.IsTruthy() {
			break
		}

		if err := i.executeStatement(ctx, s.Body, result); err != nil {
			return err
		}
	}
	return nil
}

// Query building methods

func (i *Interpreter) buildSelectQuery(s *ast.SelectStatement) (string, []interface{}, error) {
	var args []interface{}
	paramIndex := 0

	// Use the AST's String() method as a base and substitute variables
	query := s.String()
	query, args, paramIndex = i.substituteVariables(query, args, paramIndex)

	return query, args, nil
}

func (i *Interpreter) buildInsertQuery(s *ast.InsertStatement) (string, []interface{}, error) {
	var args []interface{}
	paramIndex := 0

	query := s.String()
	query, args, paramIndex = i.substituteVariables(query, args, paramIndex)

	return query, args, nil
}

func (i *Interpreter) buildUpdateQuery(s *ast.UpdateStatement) (string, []interface{}, error) {
	var args []interface{}
	paramIndex := 0

	query := s.String()
	query, args, paramIndex = i.substituteVariables(query, args, paramIndex)

	return query, args, nil
}

func (i *Interpreter) buildDeleteQuery(s *ast.DeleteStatement) (string, []interface{}, error) {
	var args []interface{}
	paramIndex := 0

	query := s.String()
	query, args, paramIndex = i.substituteVariables(query, args, paramIndex)

	return query, args, nil
}

// substituteVariables replaces @variable references with parameter placeholders
func (i *Interpreter) substituteVariables(query string, args []interface{}, startIndex int) (string, []interface{}, int) {
	// Find all @variable references and replace with placeholders
	var result strings.Builder
	idx := startIndex

	pos := 0
	for pos < len(query) {
		if query[pos] == '@' && pos+1 < len(query) && (isAlpha(query[pos+1]) || query[pos+1] == '@') {
			// Skip @@global variables - they should be evaluated
			if pos+1 < len(query) && query[pos+1] == '@' {
				// Global variable - evaluate and substitute
				end := pos + 2
				for end < len(query) && (isAlphaNum(query[end]) || query[end] == '_') {
					end++
				}
				varName := query[pos:end]
				val, _ := i.evaluator.GetVariable(varName)
				result.WriteString(val.AsString())
				pos = end
				continue
			}

			// Find variable name
			end := pos + 1
			for end < len(query) && (isAlphaNum(query[end]) || query[end] == '_') {
				end++
			}

			varName := query[pos+1 : end]
			if val, ok := i.evaluator.GetVariable(varName); ok {
				// Replace with placeholder
				placeholder := i.getPlaceholder(idx)
				result.WriteString(placeholder)
				args = append(args, FromValue(val))
				idx++
			} else {
				// Unknown variable - keep as is (might be a column alias)
				result.WriteString(query[pos:end])
			}
			pos = end
		} else {
			result.WriteByte(query[pos])
			pos++
		}
	}

	return result.String(), args, idx
}

func (i *Interpreter) getPlaceholder(index int) string {
	switch i.ctx.Dialect {
	case DialectPostgres:
		return fmt.Sprintf("$%d", index+1)
	case DialectMySQL, DialectSQLite:
		return "?"
	case DialectSQLServer:
		return fmt.Sprintf("@p%d", index)
	default:
		return fmt.Sprintf("$%d", index+1)
	}
}

func isAlpha(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_'
}

func isAlphaNum(c byte) bool {
	return isAlpha(c) || (c >= '0' && c <= '9')
}

// ============ Stage 2: TRY/CATCH ============

func (i *Interpreter) executeTryCatch(ctx context.Context, s *ast.TryCatchStatement, result *ExecutionResult) error {
	// Enter TRY block
	i.ctx.ErrorHandler.EnterTry()

	// Execute TRY block
	var tryErr error
	if s.TryBlock != nil {
		for _, stmt := range s.TryBlock.Statements {
			if err := i.executeStatement(ctx, stmt, result); err != nil {
				tryErr = err
				break
			}
		}
	}

	// Exit TRY block
	i.ctx.ErrorHandler.ExitTry()

	// If there was an error, execute CATCH block
	if tryErr != nil {
		// Record the error
		sqlErr := WrapError(tryErr)
		i.ctx.ErrorHandler.HandleError(sqlErr)
		i.ctx.UpdateError(sqlErr.Number)

		// Execute CATCH block
		i.ctx.ErrorHandler.EnterCatch()
		if s.CatchBlock != nil {
			for _, stmt := range s.CatchBlock.Statements {
				if err := i.executeStatement(ctx, stmt, result); err != nil {
					// Error in CATCH block - propagate it
					i.ctx.ErrorHandler.ExitCatch()
					return err
				}
			}
		}
		i.ctx.ErrorHandler.ExitCatch()
	}

	return nil
}

func (i *Interpreter) executeRaiserror(s *ast.RaiserrorStatement) error {
	// Evaluate message
	var msg string
	if s.Message != nil {
		msgVal, err := i.evaluator.Evaluate(s.Message)
		if err != nil {
			return err
		}
		msg = msgVal.AsString()
	}

	// Evaluate severity
	severity := 16
	if s.Severity != nil {
		sevVal, err := i.evaluator.Evaluate(s.Severity)
		if err != nil {
			return err
		}
		severity = int(sevVal.AsInt())
	}

	// Evaluate state
	state := 1
	if s.State != nil {
		stateVal, err := i.evaluator.Evaluate(s.State)
		if err != nil {
			return err
		}
		state = int(stateVal.AsInt())
	}

	// Format arguments
	var args []interface{}
	for _, arg := range s.Args {
		val, err := i.evaluator.Evaluate(arg)
		if err != nil {
			return err
		}
		args = append(args, FromValue(val))
	}

	err := RaiseError(msg, severity, state, args...)
	i.ctx.UpdateError(err.Number)

	// If severity >= 16, it's an error
	if severity >= 16 {
		return err
	}

	// Otherwise just print the message
	if i.Debug {
		fmt.Printf("RAISERROR: %s\n", err.Message)
	}
	return nil
}

func (i *Interpreter) executeThrow(s *ast.ThrowStatement) error {
	// If no parameters, re-throw the current error
	if s.ErrorNum == nil {
		if i.ctx.ErrorHandler.HasCaughtError() {
			return i.ctx.ErrorHandler.errorCtx.LastError
		}
		return NewSQLError(50000, "THROW without parameters is not valid outside a CATCH block")
	}

	// Evaluate error number
	numVal, err := i.evaluator.Evaluate(s.ErrorNum)
	if err != nil {
		return err
	}
	errNum := int(numVal.AsInt())

	// Evaluate message
	var msg string
	if s.Message != nil {
		msgVal, err := i.evaluator.Evaluate(s.Message)
		if err != nil {
			return err
		}
		msg = msgVal.AsString()
	}

	// Evaluate state
	state := 1
	if s.State != nil {
		stateVal, err := i.evaluator.Evaluate(s.State)
		if err != nil {
			return err
		}
		state = int(stateVal.AsInt())
	}

	sqlErr := ThrowError(errNum, msg, state)
	i.ctx.UpdateError(sqlErr.Number)
	return sqlErr
}

// ============ Stage 2: Temp Table Operations ============

func (i *Interpreter) isSelectFromTempTable(s *ast.SelectStatement) bool {
	if s.From == nil || len(s.From.Tables) == 0 {
		return false
	}
	for _, tableRef := range s.From.Tables {
		if tableName, ok := tableRef.(*ast.TableName); ok {
			if tableName.Name != nil {
				name := tableName.Name.String()
				if IsTempTable(name) || IsTableVariable(name) {
					return true
				}
			}
		}
	}
	return false
}

func (i *Interpreter) executeSelectFromTempTable(ctx context.Context, s *ast.SelectStatement, result *ExecutionResult) error {
	// For now, handle simple SELECT * FROM #temp
	if s.From == nil || len(s.From.Tables) != 1 {
		return fmt.Errorf("complex temp table queries not yet supported")
	}

	tableName, ok := s.From.Tables[0].(*ast.TableName)
	if !ok || tableName.Name == nil {
		return fmt.Errorf("complex temp table queries not yet supported")
	}

	name := tableName.Name.String()

	var table *TempTable
	if IsTempTable(name) {
		t, ok := i.ctx.TempTables.GetTempTable(name)
		if !ok {
			return fmt.Errorf("temp table %s does not exist", name)
		}
		table = t
	} else {
		tv, ok := i.ctx.TempTables.GetTableVariable(name)
		if !ok {
			return fmt.Errorf("table variable %s does not exist", name)
		}
		table = tv.TempTable
	}

	// Build predicate from WHERE clause
	var predicate func([]Value) bool
	if s.Where != nil {
		predicate = func(row []Value) bool {
			// Set up row values as variables for evaluation
			for j, col := range table.Columns {
				i.evaluator.SetVariable(col.Name, row[j])
			}
			result, err := i.evaluator.Evaluate(s.Where)
			if err != nil {
				return false
			}
			return result.IsTruthy()
		}
	}

	// Get column names
	columns := make([]string, len(table.Columns))
	for j, col := range table.Columns {
		columns[j] = col.Name
	}

	// Select rows
	rows := table.Select(predicate)

	rs := ResultSet{
		Columns: columns,
		Rows:    rows,
	}

	result.ResultSets = append(result.ResultSets, rs)
	i.ctx.UpdateRowCount(int64(len(rows)))
	i.ctx.AddResultSet(rs)

	return nil
}

func (i *Interpreter) executeSelectInto(ctx context.Context, s *ast.SelectStatement, result *ExecutionResult) error {
	intoTable := s.Into.String()

	// First execute the SELECT part (without INTO)
	selectCopy := *s
	selectCopy.Into = nil

	// Build and execute query
	query, args, err := i.buildSelectQuery(&selectCopy)
	if err != nil {
		return err
	}

	var rows *sql.Rows
	if i.ctx.Tx != nil {
		rows, err = i.ctx.Tx.QueryContext(ctx, query, args...)
	} else {
		rows, err = i.ctx.DB.QueryContext(ctx, query, args...)
	}
	if err != nil {
		return err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return err
	}

	var resultRows [][]Value
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for j := range values {
			valuePtrs[j] = &values[j]
		}
		if err := rows.Scan(valuePtrs...); err != nil {
			return err
		}
		row := make([]Value, len(columns))
		for j, v := range values {
			row[j] = ToValue(v)
		}
		resultRows = append(resultRows, row)
	}

	if err := rows.Err(); err != nil {
		return err
	}

	// Create the temp table and insert rows
	return i.ddl.ExecuteSelectInto(columns, resultRows, intoTable)
}

func (i *Interpreter) executeInsertIntoTempTable(ctx context.Context, s *ast.InsertStatement) error {
	tableName := s.Table.String()

	var table *TempTable
	if IsTempTable(tableName) {
		t, ok := i.ctx.TempTables.GetTempTable(tableName)
		if !ok {
			return fmt.Errorf("temp table %s does not exist", tableName)
		}
		table = t
	} else {
		tv, ok := i.ctx.TempTables.GetTableVariable(tableName)
		if !ok {
			return fmt.Errorf("table variable %s does not exist", tableName)
		}
		table = tv.TempTable
	}

	// Handle INSERT ... VALUES
	if s.Values != nil {
		count := 0
		for _, valueRow := range s.Values {
			row := make([]Value, len(valueRow))
			for j, expr := range valueRow {
				val, err := i.evaluator.Evaluate(expr)
				if err != nil {
					return err
				}
				row[j] = val
			}
			if _, err := table.InsertRow(row); err != nil {
				return err
			}
			count++
		}
		i.ctx.UpdateRowCount(int64(count))
		return nil
	}

	// Handle INSERT ... SELECT
	if s.Select != nil {
		// Execute the SELECT
		selectResult := &ExecutionResult{}
		if err := i.executeSelect(ctx, s.Select, selectResult); err != nil {
			return err
		}

		if len(selectResult.ResultSets) > 0 {
			rs := selectResult.ResultSets[0]
			for _, row := range rs.Rows {
				if _, err := table.InsertRow(row); err != nil {
					return err
				}
			}
			i.ctx.UpdateRowCount(int64(len(rs.Rows)))
		}
		return nil
	}

	return fmt.Errorf("unsupported INSERT format for temp table")
}

func (i *Interpreter) executeUpdateTempTable(ctx context.Context, s *ast.UpdateStatement) error {
	tableName := s.Table.String()

	var table *TempTable
	if IsTempTable(tableName) {
		t, ok := i.ctx.TempTables.GetTempTable(tableName)
		if !ok {
			return fmt.Errorf("temp table %s does not exist", tableName)
		}
		table = t
	} else {
		tv, ok := i.ctx.TempTables.GetTableVariable(tableName)
		if !ok {
			return fmt.Errorf("table variable %s does not exist", tableName)
		}
		table = tv.TempTable
	}

	// Build updates map
	updates := make(map[string]Value)
	for _, set := range s.SetClauses {
		if set.Column != nil && set.Value != nil {
			val, err := i.evaluator.Evaluate(set.Value)
			if err != nil {
				return err
			}
			updates[strings.ToLower(set.Column.String())] = val
		}
	}

	// Build predicate
	var predicate func([]Value) bool
	if s.Where != nil {
		predicate = func(row []Value) bool {
			for j, col := range table.Columns {
				i.evaluator.SetVariable(col.Name, row[j])
			}
			result, err := i.evaluator.Evaluate(s.Where)
			if err != nil {
				return false
			}
			return result.IsTruthy()
		}
	}

	count := table.Update(updates, predicate)
	i.ctx.UpdateRowCount(int64(count))

	return nil
}

func (i *Interpreter) executeDeleteFromTempTable(ctx context.Context, s *ast.DeleteStatement) error {
	tableName := s.Table.String()

	var table *TempTable
	if IsTempTable(tableName) {
		t, ok := i.ctx.TempTables.GetTempTable(tableName)
		if !ok {
			return fmt.Errorf("temp table %s does not exist", tableName)
		}
		table = t
	} else {
		tv, ok := i.ctx.TempTables.GetTableVariable(tableName)
		if !ok {
			return fmt.Errorf("table variable %s does not exist", tableName)
		}
		table = tv.TempTable
	}

	// Build predicate
	var predicate func([]Value) bool
	if s.Where != nil {
		predicate = func(row []Value) bool {
			for j, col := range table.Columns {
				i.evaluator.SetVariable(col.Name, row[j])
			}
			result, err := i.evaluator.Evaluate(s.Where)
			if err != nil {
				return false
			}
			return result.IsTruthy()
		}
	}

	count := table.Delete(predicate)
	i.ctx.UpdateRowCount(int64(count))

	return nil
}

// GetTempTable returns a temp table by name (for testing)
func (i *Interpreter) GetTempTable(name string) (*TempTable, bool) {
	return i.ctx.TempTables.GetTempTable(name)
}

// GetTableVariable returns a table variable by name (for testing)
func (i *Interpreter) GetTableVariable(name string) (*TableVariable, bool) {
	return i.ctx.TempTables.GetTableVariable(name)
}

// ============ Stage 3: Cursor Operations ============

func (i *Interpreter) executeDeclareCursor(ctx context.Context, s *ast.DeclareCursorStatement) error {
	if s.Name == nil {
		return fmt.Errorf("cursor name is required")
	}

	name := s.Name.Value

	// Parse cursor options
	isGlobal := false
	cursorType := CursorForwardOnly
	scrollType := CursorScrollNone
	lockType := CursorReadOnly

	if s.Options != nil {
		isGlobal = s.Options.Global

		if s.Options.Static {
			cursorType = CursorStatic
		} else if s.Options.Keyset {
			cursorType = CursorKeyset
		} else if s.Options.Dynamic {
			cursorType = CursorDynamic
		} else if s.Options.FastForward {
			cursorType = CursorFastForward
		}

		if s.Options.Scroll {
			scrollType = CursorScrollForward
		}

		if s.Options.ScrollLocks {
			lockType = CursorScrollLocks
		} else if s.Options.Optimistic {
			lockType = CursorOptimistic
		}
	}

	// Get the SELECT query string
	query := ""
	if s.ForSelect != nil {
		query = s.ForSelect.String()
	}

	_, err := i.ctx.Cursors.DeclareCursor(name, query, isGlobal, cursorType, scrollType, lockType)
	return err
}

func (i *Interpreter) executeOpenCursor(ctx context.Context, s *ast.OpenCursorStatement, result *ExecutionResult) error {
	if s.CursorName == nil {
		return fmt.Errorf("cursor name is required")
	}

	name := s.CursorName.Value

	cursor, ok := i.ctx.Cursors.GetCursor(name)
	if !ok {
		return fmt.Errorf("cursor %s does not exist", name)
	}

	// Execute the cursor's query to get data
	query, args, _ := i.substituteVariables(cursor.Query, nil, 0)

	var rows *sql.Rows
	var err error
	if i.ctx.Tx != nil {
		rows, err = i.ctx.Tx.QueryContext(ctx, query, args...)
	} else {
		rows, err = i.ctx.DB.QueryContext(ctx, query, args...)
	}
	if err != nil {
		return fmt.Errorf("cursor query error: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return err
	}

	var resultRows [][]Value
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for j := range values {
			valuePtrs[j] = &values[j]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return err
		}

		row := make([]Value, len(columns))
		for j, v := range values {
			row[j] = ToValue(v)
		}
		resultRows = append(resultRows, row)
	}

	if err := rows.Err(); err != nil {
		return err
	}

	return cursor.Open(columns, resultRows)
}

func (i *Interpreter) executeFetch(ctx context.Context, s *ast.FetchStatement) error {
	if s.CursorName == nil {
		return fmt.Errorf("cursor name is required")
	}

	name := s.CursorName.Value

	cursor, ok := i.ctx.Cursors.GetCursor(name)
	if !ok {
		return fmt.Errorf("cursor %s does not exist", name)
	}

	// Fetch the row based on direction
	var row []Value
	var fetchStatus int

	direction := strings.ToUpper(s.Direction)
	if direction == "" {
		direction = "NEXT"
	}

	switch direction {
	case "NEXT":
		row, fetchStatus = cursor.FetchNext()
	case "PRIOR":
		row, fetchStatus = cursor.FetchPrior()
	case "FIRST":
		row, fetchStatus = cursor.FetchFirst()
	case "LAST":
		row, fetchStatus = cursor.FetchLast()
	case "ABSOLUTE":
		if s.Offset != nil {
			offsetVal, err := i.evaluator.Evaluate(s.Offset)
			if err != nil {
				return err
			}
			row, fetchStatus = cursor.FetchAbsolute(int(offsetVal.AsInt()))
		} else {
			return fmt.Errorf("FETCH ABSOLUTE requires an offset")
		}
	case "RELATIVE":
		if s.Offset != nil {
			offsetVal, err := i.evaluator.Evaluate(s.Offset)
			if err != nil {
				return err
			}
			row, fetchStatus = cursor.FetchRelative(int(offsetVal.AsInt()))
		} else {
			return fmt.Errorf("FETCH RELATIVE requires an offset")
		}
	default:
		return fmt.Errorf("unknown fetch direction: %s", direction)
	}

	// Update @@FETCH_STATUS
	i.ctx.UpdateFetchStatus(fetchStatus)

	// Assign values to INTO variables
	if row != nil && len(s.IntoVars) > 0 {
		for j, v := range s.IntoVars {
			if j < len(row) {
				varName := v.Name
				i.evaluator.SetVariable(varName, row[j])
				i.ctx.SetVariable(varName, row[j])
			}
		}
	}

	return nil
}

func (i *Interpreter) executeCloseCursor(s *ast.CloseCursorStatement) error {
	if s.CursorName == nil {
		return fmt.Errorf("cursor name is required")
	}

	name := s.CursorName.Value

	cursor, ok := i.ctx.Cursors.GetCursor(name)
	if !ok {
		return fmt.Errorf("cursor %s does not exist", name)
	}

	return cursor.Close()
}

func (i *Interpreter) executeDeallocateCursor(s *ast.DeallocateCursorStatement) error {
	if s.CursorName == nil {
		return fmt.Errorf("cursor name is required")
	}

	return i.ctx.Cursors.DeallocateCursor(s.CursorName.Value)
}

// GetCursor returns a cursor by name (for testing)
func (i *Interpreter) GetCursor(name string) (*Cursor, bool) {
	return i.ctx.Cursors.GetCursor(name)
}
