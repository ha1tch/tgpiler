package storage

import (
	"fmt"
	"strings"

	"github.com/ha1tch/tsqlparser"
	"github.com/ha1tch/tsqlparser/ast"
)

// SQLDetector implements the Detector interface for T-SQL.
type SQLDetector struct {
	config DetectorConfig
	
	// Current context during detection
	currentProc string
	operations  []Operation
	warnings    []DetectionWarning
	errors      []DetectionError
}

// NewSQLDetector creates a new SQL operation detector.
func NewSQLDetector(config DetectorConfig) *SQLDetector {
	return &SQLDetector{
		config: config,
	}
}

// DetectOperations scans a procedure and returns all data operations found.
func (d *SQLDetector) DetectOperations(proc interface{}) ([]Operation, error) {
	cp, ok := proc.(*ast.CreateProcedureStatement)
	if !ok {
		return nil, fmt.Errorf("expected *ast.CreateProcedureStatement, got %T", proc)
	}
	
	d.currentProc = cp.Name.String()
	d.operations = nil
	d.warnings = nil
	d.errors = nil
	
	// Walk all statements in the procedure body
	if cp.Body != nil {
		d.walkStatements(cp.Body.Statements)
	}
	
	return d.operations, nil
}

// walkStatements recursively walks through statements.
func (d *SQLDetector) walkStatements(stmts []ast.Statement) {
	for _, stmt := range stmts {
		d.walkStatement(stmt)
	}
}

// walkStatement processes a single statement.
func (d *SQLDetector) walkStatement(stmt ast.Statement) {
	if stmt == nil {
		return
	}
	
	switch s := stmt.(type) {
	case *ast.SelectStatement:
		d.detectSelect(s)
	case *ast.InsertStatement:
		d.detectInsert(s)
	case *ast.UpdateStatement:
		d.detectUpdate(s)
	case *ast.DeleteStatement:
		d.detectDelete(s)
	case *ast.ExecStatement:
		d.detectExec(s)
	case *ast.MergeStatement:
		d.detectMerge(s)
	case *ast.TruncateTableStatement:
		d.detectTruncate(s)
	case *ast.DeclareCursorStatement:
		d.detectCursor(s)
	case *ast.OpenCursorStatement:
		d.warnCursorOperation("OPEN CURSOR", s.String())
	case *ast.FetchStatement:
		d.warnCursorOperation("FETCH", s.String())
	case *ast.CloseCursorStatement:
		d.warnCursorOperation("CLOSE CURSOR", s.String())
	case *ast.DeallocateCursorStatement:
		d.warnCursorOperation("DEALLOCATE CURSOR", s.String())
	case *ast.IfStatement:
		d.walkIfStatement(s)
	case *ast.WhileStatement:
		d.walkWhileStatement(s)
	case *ast.TryCatchStatement:
		d.walkTryCatch(s)
	case *ast.BeginEndBlock:
		d.walkStatements(s.Statements)
	}
}

// walkIfStatement walks IF/ELSE blocks.
func (d *SQLDetector) walkIfStatement(s *ast.IfStatement) {
	// Check for EXISTS/NOT EXISTS in condition
	d.checkExistsCondition(s.Condition)
	
	if s.Consequence != nil {
		d.walkStatement(s.Consequence)
	}
	if s.Alternative != nil {
		d.walkStatement(s.Alternative)
	}
}

// walkWhileStatement walks WHILE blocks.
func (d *SQLDetector) walkWhileStatement(s *ast.WhileStatement) {
	d.checkExistsCondition(s.Condition)
	if s.Body != nil {
		d.walkStatement(s.Body)
	}
}

// walkTryCatch walks TRY/CATCH blocks.
func (d *SQLDetector) walkTryCatch(s *ast.TryCatchStatement) {
	if s.TryBlock != nil {
		d.walkStatements(s.TryBlock.Statements)
	}
	if s.CatchBlock != nil {
		d.walkStatements(s.CatchBlock.Statements)
	}
}

// checkExistsCondition looks for EXISTS/NOT EXISTS subqueries in conditions.
func (d *SQLDetector) checkExistsCondition(expr ast.Expression) {
	if expr == nil {
		return
	}
	
	switch e := expr.(type) {
	case *ast.ExistsExpression:
		if e.Subquery != nil {
			d.detectSelectForExists(e.Subquery)
		}
	case *ast.InfixExpression:
		d.checkExistsCondition(e.Left)
		d.checkExistsCondition(e.Right)
	case *ast.PrefixExpression:
		d.checkExistsCondition(e.Right)
	}
}

// detectSelect extracts information from a SELECT statement.
func (d *SQLDetector) detectSelect(s *ast.SelectStatement) {
	op := Operation{
		Type:      OpSelect,
		Procedure: d.currentProc,
	}
	
	if d.config.IncludeRawSQL {
		op.RawSQL = s.String()
	}
	
	// Extract table info from FROM clause
	if s.From != nil {
		d.extractFromClause(s.From, &op)
	}
	
	// Extract columns and variable assignments
	for _, col := range s.Columns {
		field := d.extractSelectColumn(col)
		op.Fields = append(op.Fields, field)
	}
	
	// Extract WHERE clause fields
	if s.Where != nil {
		op.KeyFields = d.extractWhereFields(s.Where)
	}
	
	// Check for SELECT INTO
	if s.Into != nil {
		op.OutputFields = append(op.OutputFields, Field{
			Name:   s.Into.String(),
			GoName: toPascalCase(s.Into.String()),
		})
	}
	
	d.operations = append(d.operations, op)
}

// detectSelectForExists handles SELECT inside EXISTS - typically SELECT 1
func (d *SQLDetector) detectSelectForExists(s *ast.SelectStatement) {
	op := Operation{
		Type:      OpSelect,
		Procedure: d.currentProc,
	}
	
	if d.config.IncludeRawSQL {
		op.RawSQL = s.String()
	}
	
	// Mark as existence check
	if len(s.Columns) == 1 {
		col := s.Columns[0]
		if lit, ok := col.Expression.(*ast.IntegerLiteral); ok {
			if lit.Value == 1 {
				// This is SELECT 1 - existence check pattern
				op.Fields = append(op.Fields, Field{
					Name:   "exists",
					GoType: "bool",
				})
			}
		}
	}
	
	if s.From != nil {
		d.extractFromClause(s.From, &op)
	}
	
	if s.Where != nil {
		op.KeyFields = d.extractWhereFields(s.Where)
	}
	
	d.operations = append(d.operations, op)
}

// detectInsert extracts information from an INSERT statement.
func (d *SQLDetector) detectInsert(s *ast.InsertStatement) {
	op := Operation{
		Type:      OpInsert,
		Procedure: d.currentProc,
		Table:     s.Table.String(),
	}
	
	if d.config.IncludeRawSQL {
		op.RawSQL = s.String()
	}
	
	// Extract columns
	for _, col := range s.Columns {
		op.Fields = append(op.Fields, Field{
			Name:   col.Value,
			GoName: toPascalCase(col.Value),
		})
	}
	
	// Extract values (for single row inserts) or detect SELECT
	if s.Select != nil {
		// INSERT ... SELECT - the select is also an operation
		d.detectSelect(s.Select)
	}
	
	// Extract OUTPUT clause
	if s.Output != nil {
		for _, col := range s.Output.Columns {
			f := d.extractSelectColumn(col)
			f.IsAssigned = true
			op.OutputFields = append(op.OutputFields, f)
		}
	}
	
	d.operations = append(d.operations, op)
}

// detectUpdate extracts information from an UPDATE statement.
func (d *SQLDetector) detectUpdate(s *ast.UpdateStatement) {
	op := Operation{
		Type:      OpUpdate,
		Procedure: d.currentProc,
		Table:     s.Table.String(),
	}
	
	// In T-SQL, UPDATE alias ... FROM Table alias ... pattern
	// The Table field might be the alias, not the actual table
	// We need to look in the FROM clause for the actual table
	if s.From != nil {
		// First extract FROM clause to get table info
		d.extractFromClause(s.From, &op)
		
		// If the UPDATE target matches an alias in FROM, it's the aliased table
		// The extractFromClause would have set op.Table and op.Alias
		// But we need to check if the original UPDATE table matches an alias
		targetAlias := s.Table.String()
		if actualTable, alias := d.findTableByAlias(s.From, targetAlias); actualTable != "" {
			op.Table = actualTable
			op.Alias = alias
		}
	}
	
	if s.Alias != nil {
		op.Alias = s.Alias.Value
	}
	
	if d.config.IncludeRawSQL {
		op.RawSQL = s.String()
	}
	
	// Extract SET clauses
	for _, set := range s.SetClauses {
		field := Field{
			Name:   set.Column.String(),
			GoName: toPascalCase(extractColumnName(set.Column.String())),
		}
		
		// Check if value is a variable
		if v, ok := set.Value.(*ast.Variable); ok {
			field.Variable = v.Name
		}
		
		op.Fields = append(op.Fields, field)
	}
	
	// Extract WHERE clause
	if s.Where != nil {
		op.KeyFields = d.extractWhereFields(s.Where)
	} else {
		// UPDATE without WHERE affects all rows - dangerous!
		d.warnDangerousOperation("UPDATE", op.Table, "no WHERE clause - will update ALL rows")
	}
	
	// Extract OUTPUT clause
	if s.Output != nil {
		for _, col := range s.Output.Columns {
			f := d.extractSelectColumn(col)
			op.OutputFields = append(op.OutputFields, f)
		}
	}
	
	d.operations = append(d.operations, op)
}

// detectDelete extracts information from a DELETE statement.
func (d *SQLDetector) detectDelete(s *ast.DeleteStatement) {
	op := Operation{
		Type:      OpDelete,
		Procedure: d.currentProc,
	}
	
	if s.Table != nil {
		op.Table = s.Table.String()
	}
	
	if s.Alias != nil {
		op.Alias = s.Alias.Value
	}
	
	if d.config.IncludeRawSQL {
		op.RawSQL = s.String()
	}
	
	// Extract WHERE clause
	if s.Where != nil {
		op.KeyFields = d.extractWhereFields(s.Where)
	} else if s.From == nil {
		// DELETE without WHERE and no FROM clause affects all rows - dangerous!
		d.warnDangerousOperation("DELETE", op.Table, "no WHERE clause - will delete ALL rows")
	}
	
	// Extract FROM clause (for DELETE with JOIN)
	if s.From != nil {
		d.extractFromClause(s.From, &op)
		// If there's a FROM but no WHERE, it's still potentially dangerous
		if s.Where == nil {
			d.warnDangerousOperation("DELETE", op.Table, "no WHERE clause with FROM/JOIN - verify join conditions")
		}
	}
	
	// Extract OUTPUT clause
	if s.Output != nil {
		for _, col := range s.Output.Columns {
			f := d.extractSelectColumn(col)
			op.OutputFields = append(op.OutputFields, f)
		}
	}
	
	d.operations = append(d.operations, op)
}

// detectExec extracts information from an EXEC statement.
func (d *SQLDetector) detectExec(s *ast.ExecStatement) {
	// Skip dynamic SQL
	if s.DynamicSQL != nil {
		d.warnings = append(d.warnings, DetectionWarning{
			Procedure: d.currentProc,
			Message:   "Dynamic SQL detected, cannot analyze",
			SQL:       s.String(),
		})
		return
	}
	
	op := Operation{
		Type:            OpExec,
		Procedure:       d.currentProc,
		CalledProcedure: s.Procedure.String(),
	}
	
	if d.config.IncludeRawSQL {
		op.RawSQL = s.String()
	}
	
	// Extract parameters
	for _, param := range s.Parameters {
		field := Field{
			Name: param.Name,
		}
		
		if v, ok := param.Value.(*ast.Variable); ok {
			field.Variable = v.Name
		}
		
		if param.Output {
			field.IsAssigned = true
		}
		
		op.Parameters = append(op.Parameters, field)
	}
	
	d.operations = append(d.operations, op)
}

// detectMerge extracts information from a MERGE statement.
func (d *SQLDetector) detectMerge(s *ast.MergeStatement) {
	// MERGE is complex - create multiple operations
	op := Operation{
		Type:      OpUpdate, // MERGE is primarily an upsert
		Procedure: d.currentProc,
		Table:     s.Target.String(),
	}
	
	if s.TargetAlias != nil {
		op.Alias = s.TargetAlias.Value
	}
	
	if d.config.IncludeRawSQL {
		op.RawSQL = s.String()
	}
	
	// Extract ON condition as key fields
	op.KeyFields = d.extractWhereFields(s.OnCondition)
	
	d.operations = append(d.operations, op)
	
	// Note: Could further analyze WHEN clauses for more detailed operations
}

// detectTruncate extracts information from a TRUNCATE TABLE statement.
func (d *SQLDetector) detectTruncate(s *ast.TruncateTableStatement) {
	op := Operation{
		Type:      OpTruncate,
		Procedure: d.currentProc,
		Table:     s.Table.String(),
	}
	
	if d.config.IncludeRawSQL {
		op.RawSQL = s.String()
	}
	
	// TRUNCATE is always dangerous - add warning
	d.warnings = append(d.warnings, DetectionWarning{
		Procedure: d.currentProc,
		Message:   "TRUNCATE TABLE detected - this removes all rows without logging individual deletions",
		SQL:       s.String(),
	})
	
	d.operations = append(d.operations, op)
}

// detectCursor handles DECLARE CURSOR statements.
func (d *SQLDetector) detectCursor(s *ast.DeclareCursorStatement) {
	d.warnings = append(d.warnings, DetectionWarning{
		Procedure: d.currentProc,
		Message:   "Cursor detected - consider refactoring to set-based operations for better performance",
		SQL:       s.String(),
	})
	
	// Also analyze the SELECT statement inside the cursor
	if s.ForSelect != nil {
		d.detectSelect(s.ForSelect)
	}
}

// warnCursorOperation adds a warning for cursor operations.
func (d *SQLDetector) warnCursorOperation(opName, sql string) {
	d.warnings = append(d.warnings, DetectionWarning{
		Procedure: d.currentProc,
		Message:   opName + " detected - cursor operations indicate row-by-row processing",
		SQL:       sql,
	})
}

// warnDangerousOperation adds a warning for potentially dangerous operations.
func (d *SQLDetector) warnDangerousOperation(opType, table, reason string) {
	d.warnings = append(d.warnings, DetectionWarning{
		Procedure: d.currentProc,
		Message:   opType + " on " + table + ": " + reason,
	})
}

// isTemporaryTable checks if a table name indicates a temporary table.
func isTemporaryTable(tableName string) bool {
	if tableName == "" {
		return false
	}
	// #temp tables, ##global temp tables, @table variables
	return strings.HasPrefix(tableName, "#") || strings.HasPrefix(tableName, "@")
}

// extractFromClause extracts table references from FROM clause.
func (d *SQLDetector) extractFromClause(from *ast.FromClause, op *Operation) {
	for _, tableRef := range from.Tables {
		d.extractTableReference(tableRef, op)
	}
}

// extractTableReference recursively extracts table info from a table reference.
func (d *SQLDetector) extractTableReference(ref ast.TableReference, op *Operation) {
	switch t := ref.(type) {
	case *ast.TableName:
		if op.Table == "" {
			op.Table = t.Name.String()
			if t.Alias != nil {
				op.Alias = t.Alias.Value
			}
		}
	case *ast.JoinClause:
		// Process left side
		d.extractTableReference(t.Left, op)
		// Process right side (for JOIN info)
		d.extractTableReference(t.Right, op)
		// The join condition contains key fields
		if t.Condition != nil {
			joinFields := d.extractWhereFields(t.Condition)
			op.KeyFields = append(op.KeyFields, joinFields...)
		}
	case *ast.DerivedTable:
		// Subquery as table source - analyze the subquery
		if t.Subquery != nil {
			d.detectSelect(t.Subquery)
		}
	}
}

// findTableByAlias searches a FROM clause for a table with the given alias.
// Returns the actual table name and alias if found, empty strings otherwise.
func (d *SQLDetector) findTableByAlias(from *ast.FromClause, alias string) (tableName, tableAlias string) {
	for _, tableRef := range from.Tables {
		if name, a := d.findTableByAliasInRef(tableRef, alias); name != "" {
			return name, a
		}
	}
	return "", ""
}

// findTableByAliasInRef recursively searches for a table by alias.
func (d *SQLDetector) findTableByAliasInRef(ref ast.TableReference, alias string) (tableName, tableAlias string) {
	switch t := ref.(type) {
	case *ast.TableName:
		if t.Alias != nil && t.Alias.Value == alias {
			return t.Name.String(), t.Alias.Value
		}
	case *ast.JoinClause:
		// Search left side
		if name, a := d.findTableByAliasInRef(t.Left, alias); name != "" {
			return name, a
		}
		// Search right side
		if name, a := d.findTableByAliasInRef(t.Right, alias); name != "" {
			return name, a
		}
	}
	return "", ""
}

// extractSelectColumn extracts field info from a SELECT column.
func (d *SQLDetector) extractSelectColumn(col ast.SelectColumn) Field {
	field := Field{}
	
	// Check for variable assignment: SELECT @var = expr
	if col.Variable != nil {
		field.Variable = col.Variable.Name
		field.IsAssigned = true
	}
	
	// Check for alias
	if col.Alias != nil {
		field.Name = col.Alias.Value
		field.GoName = toPascalCase(col.Alias.Value)
	}
	
	// Extract column name from expression
	if col.Expression != nil {
		colName := d.extractColumnFromExpression(col.Expression)
		if field.Name == "" {
			field.Name = colName
			field.GoName = toPascalCase(colName)
		}
	}
	
	// Handle SELECT *
	if col.AllColumns {
		field.Name = "*"
	}
	
	return field
}

// extractColumnFromExpression extracts the column name from an expression.
func (d *SQLDetector) extractColumnFromExpression(expr ast.Expression) string {
	switch e := expr.(type) {
	case *ast.Identifier:
		return e.Value
	case *ast.QualifiedIdentifier:
		// Return just the column name, not the full path
		parts := strings.Split(e.String(), ".")
		return parts[len(parts)-1]
	case *ast.Variable:
		return e.Name
	case *ast.FunctionCall:
		// For functions like ISNULL(col, 0), try to get the column name
		if len(e.Arguments) > 0 {
			return d.extractColumnFromExpression(e.Arguments[0])
		}
		return e.Function.String()
	case *ast.CastExpression:
		return d.extractColumnFromExpression(e.Expression)
	default:
		return expr.String()
	}
}

// extractWhereFields extracts fields from a WHERE clause expression.
func (d *SQLDetector) extractWhereFields(expr ast.Expression) []Field {
	var fields []Field
	d.extractWhereFieldsRecursive(expr, &fields)
	return fields
}

// extractWhereFieldsRecursive recursively extracts WHERE clause fields.
func (d *SQLDetector) extractWhereFieldsRecursive(expr ast.Expression, fields *[]Field) {
	if expr == nil {
		return
	}
	
	switch e := expr.(type) {
	case *ast.InfixExpression:
		op := strings.ToUpper(e.Operator)
		
		// Check if this is a comparison operation
		if isComparisonOp(op) {
			field := d.extractComparisonField(e)
			if field.Name != "" {
				*fields = append(*fields, field)
			}
		} else {
			// AND/OR - recurse both sides
			d.extractWhereFieldsRecursive(e.Left, fields)
			d.extractWhereFieldsRecursive(e.Right, fields)
		}
		
	case *ast.IsNullExpression:
		colName := d.extractColumnFromExpression(e.Expr)
		*fields = append(*fields, Field{
			Name:        colName,
			GoName:      toPascalCase(colName),
			IsKey:       true,
			Optionality: Optional,
		})
		
	case *ast.InExpression:
		colName := d.extractColumnFromExpression(e.Expr)
		*fields = append(*fields, Field{
			Name:   colName,
			GoName: toPascalCase(colName),
			IsKey:  true,
		})
		
	case *ast.BetweenExpression:
		colName := d.extractColumnFromExpression(e.Expr)
		*fields = append(*fields, Field{
			Name:   colName,
			GoName: toPascalCase(colName),
			IsKey:  true,
		})
		
	case *ast.LikeExpression:
		colName := d.extractColumnFromExpression(e.Expr)
		*fields = append(*fields, Field{
			Name:   colName,
			GoName: toPascalCase(colName),
			IsKey:  true,
		})
		
	case *ast.PrefixExpression:
		d.extractWhereFieldsRecursive(e.Right, fields)
	}
}

// extractComparisonField extracts field info from a comparison expression.
func (d *SQLDetector) extractComparisonField(e *ast.InfixExpression) Field {
	field := Field{
		IsKey: true,
	}
	
	// Determine which side is the column and which is the value
	leftIsCol := isColumnExpression(e.Left)
	rightIsCol := isColumnExpression(e.Right)
	leftIsVar := isVariableExpression(e.Left)
	rightIsVar := isVariableExpression(e.Right)
	
	if leftIsCol && (rightIsVar || !rightIsCol) {
		// Pattern: column = @var or column = literal
		field.Name = d.extractColumnFromExpression(e.Left)
		field.GoName = toPascalCase(field.Name)
		if rightIsVar {
			field.Variable = e.Right.(*ast.Variable).Name
		}
	} else if rightIsCol && (leftIsVar || !leftIsCol) {
		// Pattern: @var = column or literal = column
		field.Name = d.extractColumnFromExpression(e.Right)
		field.GoName = toPascalCase(field.Name)
		if leftIsVar {
			field.Variable = e.Left.(*ast.Variable).Name
		}
	}
	
	return field
}

// isComparisonOp checks if an operator is a comparison operator.
func isComparisonOp(op string) bool {
	switch op {
	case "=", "<>", "!=", "<", ">", "<=", ">=":
		return true
	default:
		return false
	}
}

// isColumnExpression checks if an expression represents a column reference.
func isColumnExpression(expr ast.Expression) bool {
	switch expr.(type) {
	case *ast.Identifier, *ast.QualifiedIdentifier:
		return true
	default:
		return false
	}
}

// isVariableExpression checks if an expression is a variable.
func isVariableExpression(expr ast.Expression) bool {
	_, ok := expr.(*ast.Variable)
	return ok
}

// extractColumnName extracts just the column name from a qualified name.
func extractColumnName(name string) string {
	parts := strings.Split(name, ".")
	return parts[len(parts)-1]
}

// DetectModels infers models from detected operations.
func (d *SQLDetector) DetectModels(ops []Operation) ([]Model, error) {
	modelMap := make(map[string]*Model)
	
	for _, op := range ops {
		if op.Table == "" {
			continue
		}
		
		tableName := extractColumnName(op.Table)
		
		// Skip temporary tables - they should not become models
		if isTemporaryTable(tableName) {
			continue
		}
		
		model, exists := modelMap[tableName]
		if !exists {
			model = &Model{
				Name:  toPascalCase(tableName),
				Table: tableName,
			}
			modelMap[tableName] = model
		}
		
		// Merge fields from this operation
		for _, field := range op.Fields {
			if !modelHasField(model, field.Name) && field.Name != "*" {
				model.Fields = append(model.Fields, field)
			}
		}
		
		// Key fields might be primary keys
		for _, field := range op.KeyFields {
			if !modelHasField(model, field.Name) {
				field.IsKey = true
				model.Fields = append(model.Fields, field)
			}
		}
	}
	
	var models []Model
	for _, m := range modelMap {
		models = append(models, *m)
	}
	
	return models, nil
}

// modelHasField checks if a model already has a field by name.
func modelHasField(m *Model, name string) bool {
	for _, f := range m.Fields {
		if f.Name == name {
			return true
		}
	}
	return false
}

// DetectRepositories generates repository specs from operations and models.
func (d *SQLDetector) DetectRepositories(ops []Operation, models []Model) ([]Repository, error) {
	repoMap := make(map[string]*Repository)
	
	for _, op := range ops {
		if op.Table == "" {
			continue
		}
		
		tableName := extractColumnName(op.Table)
		
		// Skip temporary tables - they should not get repositories
		if isTemporaryTable(tableName) {
			continue
		}
		
		entityName := toPascalCase(tableName)
		repoName := entityName + "Repository"
		
		repo, exists := repoMap[repoName]
		if !exists {
			repo = &Repository{
				Name:   repoName,
				Entity: entityName,
				Table:  tableName,
			}
			repoMap[repoName] = repo
		}
		
		// Generate method from operation
		method := d.generateMethodFromOperation(op, entityName)
		if method.Name != "" && !repoHasMethod(repo, method.Name) {
			repo.Methods = append(repo.Methods, method)
		}
	}
	
	var repos []Repository
	for _, r := range repoMap {
		repos = append(repos, *r)
	}
	
	return repos, nil
}

// generateMethodFromOperation creates a repository method from an operation.
func (d *SQLDetector) generateMethodFromOperation(op Operation, entityName string) RepositoryMethod {
	method := RepositoryMethod{
		Operation: op,
	}
	
	switch op.Type {
	case OpSelect:
		// Analyze pattern
		info := &SelectInfo{
			WhereFields: make([]WhereField, len(op.KeyFields)),
		}
		for i, kf := range op.KeyFields {
			info.WhereFields[i] = WhereField{
				Column:   kf.Name,
				Variable: kf.Variable,
				Operator: "=",
			}
		}
		
		pattern := IdentifySelectPattern(info)
		method.Name = GenerateMethodName(pattern, info)
		method.OutputType = "*" + entityName
		
		// Generate input fields from key fields
		for _, kf := range op.KeyFields {
			method.InputFields = append(method.InputFields, kf)
		}
		
	case OpInsert:
		method.Name = "Create"
		method.OutputType = "*" + entityName
		method.InputFields = op.Fields
		
	case OpUpdate:
		if len(op.KeyFields) > 0 {
			method.Name = "Update"
		} else {
			method.Name = "UpdateAll"
		}
		method.OutputType = "int64" // rows affected
		method.InputFields = append(op.Fields, op.KeyFields...)
		
	case OpDelete:
		if len(op.KeyFields) > 0 {
			method.Name = "Delete"
		} else {
			method.Name = "DeleteAll"
		}
		method.OutputType = "int64"
		method.InputFields = op.KeyFields
	}
	
	return method
}

// repoHasMethod checks if a repository already has a method by name.
func repoHasMethod(r *Repository, name string) bool {
	for _, m := range r.Methods {
		if m.Name == name {
			return true
		}
	}
	return false
}

// GetWarnings returns detection warnings.
func (d *SQLDetector) GetWarnings() []DetectionWarning {
	return d.warnings
}

// GetErrors returns detection errors.
func (d *SQLDetector) GetErrors() []DetectionError {
	return d.errors
}

// DetectFromSQL parses raw SQL and detects operations.
// It handles both CREATE PROCEDURE statements and raw DML statements.
func (d *SQLDetector) DetectFromSQL(sql string) ([]Operation, error) {
	program, errs := tsqlparser.Parse(sql)
	if len(errs) > 0 {
		// Log parse errors but continue
		for _, e := range errs {
			d.errors = append(d.errors, DetectionError{
				Message: fmt.Sprintf("parse error: %v", e),
			})
		}
	}

	if program == nil || len(program.Statements) == 0 {
		return nil, fmt.Errorf("no statements parsed from SQL")
	}

	var allOps []Operation

	for _, stmt := range program.Statements {
		switch s := stmt.(type) {
		case *ast.CreateProcedureStatement:
			ops, err := d.DetectOperations(s)
			if err != nil {
				d.errors = append(d.errors, DetectionError{
					Message: fmt.Sprintf("detection error: %v", err),
				})
				continue
			}
			allOps = append(allOps, ops...)

		case *ast.SelectStatement:
			d.detectSelect(s)
			allOps = append(allOps, d.operations...)
			d.operations = nil

		case *ast.InsertStatement:
			d.detectInsert(s)
			allOps = append(allOps, d.operations...)
			d.operations = nil

		case *ast.UpdateStatement:
			d.detectUpdate(s)
			allOps = append(allOps, d.operations...)
			d.operations = nil

		case *ast.DeleteStatement:
			d.detectDelete(s)
			allOps = append(allOps, d.operations...)
			d.operations = nil

		case *ast.MergeStatement:
			d.detectMerge(s)
			allOps = append(allOps, d.operations...)
			d.operations = nil
		}
	}

	return allOps, nil
}
