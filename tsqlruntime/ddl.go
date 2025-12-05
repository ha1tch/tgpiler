package tsqlruntime

import (
	"fmt"
	"strings"

	"github.com/ha1tch/tsqlparser/ast"
)

// DDLHandler handles DDL statements for temp tables
type DDLHandler struct {
	ctx *ExecutionContext
}

// NewDDLHandler creates a new DDL handler
func NewDDLHandler(ctx *ExecutionContext) *DDLHandler {
	return &DDLHandler{ctx: ctx}
}

// ExecuteCreateTable handles CREATE TABLE for temp tables
func (h *DDLHandler) ExecuteCreateTable(stmt *ast.CreateTableStatement) error {
	if stmt == nil || stmt.Name == nil {
		return fmt.Errorf("invalid CREATE TABLE statement")
	}

	tableName := stmt.Name.String()

	// Only handle temp tables
	if !strings.HasPrefix(tableName, "#") {
		return fmt.Errorf("CREATE TABLE only supported for temp tables (#table)")
	}

	// Parse column definitions
	columns := h.parseColumnDefinitions(stmt.Columns)

	// Create the temp table
	_, err := h.ctx.TempTables.CreateTempTable(tableName, columns)
	if err != nil {
		return err
	}

	return nil
}

// ExecuteDropTable handles DROP TABLE for temp tables
func (h *DDLHandler) ExecuteDropTable(stmt *ast.DropTableStatement) error {
	if stmt == nil {
		return fmt.Errorf("invalid DROP TABLE statement")
	}

	for _, table := range stmt.Tables {
		tableName := table.String()

		// Only handle temp tables
		if !strings.HasPrefix(tableName, "#") {
			return fmt.Errorf("DROP TABLE only supported for temp tables (#table)")
		}

		if err := h.ctx.TempTables.DropTempTable(tableName); err != nil {
			// If IF EXISTS is specified, ignore "not found" errors
			if stmt.IfExists {
				continue
			}
			return err
		}
	}

	return nil
}

// ExecuteTruncateTable handles TRUNCATE TABLE for temp tables
func (h *DDLHandler) ExecuteTruncateTable(stmt *ast.TruncateTableStatement) error {
	if stmt == nil || stmt.Table == nil {
		return fmt.Errorf("invalid TRUNCATE TABLE statement")
	}

	tableName := stmt.Table.String()

	// Only handle temp tables
	if !strings.HasPrefix(tableName, "#") {
		return fmt.Errorf("TRUNCATE TABLE only supported for temp tables (#table)")
	}

	table, ok := h.ctx.TempTables.GetTempTable(tableName)
	if !ok {
		return fmt.Errorf("temp table %s does not exist", tableName)
	}

	table.Truncate()
	h.ctx.UpdateRowCount(0)

	return nil
}

// ExecuteSelectInto handles SELECT INTO #temp
func (h *DDLHandler) ExecuteSelectInto(columns []string, rows [][]Value, intoTable string) error {
	if !strings.HasPrefix(intoTable, "#") && !strings.HasPrefix(intoTable, "@") {
		return fmt.Errorf("SELECT INTO only supported for temp tables (#table) or table variables (@table)")
	}

	// Create column definitions from the result set
	colDefs := make([]TempTableColumn, len(columns))
	for i, name := range columns {
		colDefs[i] = TempTableColumn{
			Name:     name,
			Type:     TypeVarChar, // Default type - could infer from data
			Nullable: true,
			MaxLen:   -1,
		}

		// Try to infer type from first row
		if len(rows) > 0 && i < len(rows[0]) {
			colDefs[i].Type = rows[0][i].Type
		}
	}

	// Create the table
	var table *TempTable
	if strings.HasPrefix(intoTable, "@") {
		tv, err := h.ctx.TempTables.CreateTableVariable(intoTable, colDefs)
		if err != nil {
			return err
		}
		table = tv.TempTable
	} else {
		var err error
		table, err = h.ctx.TempTables.CreateTempTable(intoTable, colDefs)
		if err != nil {
			return err
		}
	}

	// Insert all rows
	for _, row := range rows {
		if _, err := table.InsertRow(row); err != nil {
			return err
		}
	}

	h.ctx.UpdateRowCount(int64(len(rows)))

	return nil
}

// parseColumnDefinitions parses column definitions from AST
func (h *DDLHandler) parseColumnDefinitions(defs []*ast.ColumnDefinition) []TempTableColumn {
	columns := make([]TempTableColumn, len(defs))
	for i, def := range defs {
		columns[i] = h.parseColumnDef(def)
	}
	return columns
}

// parseColumnDef parses a single column definition
func (h *DDLHandler) parseColumnDef(def *ast.ColumnDefinition) TempTableColumn {
	col := TempTableColumn{
		Name:     def.Name.Value,
		Nullable: true, // Default to nullable
	}

	// Parse data type
	if def.DataType != nil {
		col.Type, col.Precision, col.Scale, col.MaxLen = ParseDataType(def.DataType.String())
	}

	// Handle Nullable field
	if def.Nullable != nil {
		col.Nullable = *def.Nullable
	}

	// Handle Identity
	if def.Identity != nil {
		col.Identity = true
		col.IdentitySeed = def.Identity.Seed
		col.IdentityIncr = def.Identity.Increment
		if col.IdentitySeed == 0 {
			col.IdentitySeed = 1
		}
		if col.IdentityIncr == 0 {
			col.IdentityIncr = 1
		}
	}

	// Handle Default
	if def.Default != nil {
		col.DefaultValue = NewVarChar(def.Default.String(), -1)
	}

	// Check inline constraints
	for _, constraint := range def.Constraints {
		if constraint.IsPrimaryKey {
			col.Nullable = false
		}
	}

	// Set default value to NULL if not specified
	if col.DefaultValue.Type == TypeUnknown {
		col.DefaultValue = Null(col.Type)
	}

	return col
}

// DeclareTableVariable handles DECLARE @t TABLE (...)
func (h *DDLHandler) DeclareTableVariable(name string, columns []TempTableColumn) error {
	_, err := h.ctx.TempTables.CreateTableVariable(name, columns)
	return err
}

// IsTempTable checks if a table name refers to a temp table
func IsTempTable(name string) bool {
	return strings.HasPrefix(name, "#")
}

// IsTableVariable checks if a name refers to a table variable
func IsTableVariable(name string) bool {
	return strings.HasPrefix(name, "@") && !strings.HasPrefix(name, "@@")
}
