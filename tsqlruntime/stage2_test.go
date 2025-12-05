package tsqlruntime

import (
	"fmt"
	"testing"

	"github.com/shopspring/decimal"
)

func TestTempTable(t *testing.T) {
	manager := NewTempTableManager()

	// Create temp table
	columns := []TempTableColumn{
		{Name: "ID", Type: TypeInt, Nullable: false, Identity: true, IdentitySeed: 1, IdentityIncr: 1},
		{Name: "Name", Type: TypeVarChar, Nullable: true, MaxLen: 50},
		{Name: "Value", Type: TypeDecimal, Precision: 18, Scale: 2, Nullable: true},
	}

	table, err := manager.CreateTempTable("#test", columns)
	if err != nil {
		t.Fatalf("CreateTempTable failed: %v", err)
	}

	// Insert rows
	_, err = table.Insert(map[string]Value{
		"name":  NewVarChar("First", -1),
		"value": NewDecimal(decimal.NewFromFloat(10.5), 18, 2),
	})
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	_, err = table.Insert(map[string]Value{
		"name":  NewVarChar("Second", -1),
		"value": NewDecimal(decimal.NewFromFloat(20.75), 18, 2),
	})
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	// Check row count
	if table.RowCount() != 2 {
		t.Errorf("Expected 2 rows, got %d", table.RowCount())
	}

	// Select all
	rows := table.SelectAll()
	if len(rows) != 2 {
		t.Errorf("SelectAll returned %d rows, expected 2", len(rows))
	}

	// Check identity values
	if rows[0][0].AsInt() != 1 {
		t.Errorf("First row ID = %d, expected 1", rows[0][0].AsInt())
	}
	if rows[1][0].AsInt() != 2 {
		t.Errorf("Second row ID = %d, expected 2", rows[1][0].AsInt())
	}

	// Select with predicate
	filteredRows := table.Select(func(row []Value) bool {
		return row[0].AsInt() == 1
	})
	if len(filteredRows) != 1 {
		t.Errorf("Filtered select returned %d rows, expected 1", len(filteredRows))
	}

	// Update
	count := table.Update(
		map[string]Value{"name": NewVarChar("Updated", -1)},
		func(row []Value) bool { return row[0].AsInt() == 1 },
	)
	if count != 1 {
		t.Errorf("Update affected %d rows, expected 1", count)
	}

	// Verify update
	rows = table.SelectAll()
	if rows[0][1].AsString() != "Updated" {
		t.Errorf("Expected 'Updated', got '%s'", rows[0][1].AsString())
	}

	// Delete
	count = table.Delete(func(row []Value) bool { return row[0].AsInt() == 2 })
	if count != 1 {
		t.Errorf("Delete affected %d rows, expected 1", count)
	}
	if table.RowCount() != 1 {
		t.Errorf("After delete, expected 1 row, got %d", table.RowCount())
	}

	// Truncate
	table.Truncate()
	if table.RowCount() != 0 {
		t.Errorf("After truncate, expected 0 rows, got %d", table.RowCount())
	}

	// Drop table
	err = manager.DropTempTable("#test")
	if err != nil {
		t.Fatalf("DropTempTable failed: %v", err)
	}

	// Verify dropped
	if manager.TempTableExists("#test") {
		t.Error("Table should not exist after drop")
	}
}

func TestTableVariable(t *testing.T) {
	manager := NewTempTableManager()

	columns := []TempTableColumn{
		{Name: "ID", Type: TypeInt},
		{Name: "Data", Type: TypeVarChar, MaxLen: 100},
	}

	tv, err := manager.CreateTableVariable("@results", columns)
	if err != nil {
		t.Fatalf("CreateTableVariable failed: %v", err)
	}

	// Insert
	tv.InsertRow([]Value{NewInt(1), NewVarChar("Test", -1)})
	tv.InsertRow([]Value{NewInt(2), NewVarChar("Data", -1)})

	if tv.RowCount() != 2 {
		t.Errorf("Expected 2 rows, got %d", tv.RowCount())
	}

	// Get table variable
	retrieved, ok := manager.GetTableVariable("@results")
	if !ok {
		t.Fatal("Failed to retrieve table variable")
	}
	if retrieved.RowCount() != 2 {
		t.Errorf("Retrieved table has %d rows, expected 2", retrieved.RowCount())
	}
}

func TestGlobalTempTable(t *testing.T) {
	manager := NewTempTableManager()

	columns := []TempTableColumn{
		{Name: "ID", Type: TypeInt},
	}

	// Create global temp table
	_, err := manager.CreateTempTable("##global", columns)
	if err != nil {
		t.Fatalf("CreateTempTable for global table failed: %v", err)
	}

	// Verify it exists
	if !manager.TempTableExists("##global") {
		t.Error("Global temp table should exist")
	}

	// Local temp tables shouldn't see it by that name
	if manager.TempTableExists("#global") {
		t.Error("Should not find ##global as #global")
	}
}

func TestTempTableToResultSet(t *testing.T) {
	manager := NewTempTableManager()

	columns := []TempTableColumn{
		{Name: "Col1", Type: TypeInt},
		{Name: "Col2", Type: TypeVarChar, MaxLen: 50},
	}

	table, _ := manager.CreateTempTable("#rs", columns)
	table.InsertRow([]Value{NewInt(1), NewVarChar("A", -1)})
	table.InsertRow([]Value{NewInt(2), NewVarChar("B", -1)})

	rs := table.ToResultSet()

	if len(rs.Columns) != 2 {
		t.Errorf("Expected 2 columns, got %d", len(rs.Columns))
	}
	if rs.Columns[0] != "Col1" || rs.Columns[1] != "Col2" {
		t.Errorf("Unexpected column names: %v", rs.Columns)
	}
	if len(rs.Rows) != 2 {
		t.Errorf("Expected 2 rows, got %d", len(rs.Rows))
	}
}

func TestErrorHandling(t *testing.T) {
	handler := NewTryCatchHandler()

	// Initially no error
	if handler.HasCaughtError() {
		t.Error("Should not have caught error initially")
	}

	// Enter TRY block
	handler.EnterTry()

	// Simulate error
	err := NewSQLError(8134, "Divide by zero error encountered")
	caught := handler.HandleError(err)

	if !caught {
		t.Error("Error should be caught in TRY block")
	}

	handler.ExitTry()

	// Enter CATCH block
	handler.EnterCatch()

	if !handler.HasCaughtError() {
		t.Error("Should have caught error")
	}

	if handler.GetErrorNumber() != 8134 {
		t.Errorf("ERROR_NUMBER() = %d, expected 8134", handler.GetErrorNumber())
	}

	if handler.GetErrorMessage() != "Divide by zero error encountered" {
		t.Errorf("ERROR_MESSAGE() = %s", handler.GetErrorMessage())
	}

	handler.ExitCatch()
}

func TestRaiseError(t *testing.T) {
	err := RaiseError("Error: %1 at position %2", 16, 1, "overflow", 42)

	if err.Number != ErrRaiseError {
		t.Errorf("Error number = %d, expected %d", err.Number, ErrRaiseError)
	}

	if err.Severity != 16 {
		t.Errorf("Severity = %d, expected 16", err.Severity)
	}

	// Check message formatting
	if err.Message != "Error: overflow at position 42" {
		t.Errorf("Message = %s", err.Message)
	}
}

func TestThrowError(t *testing.T) {
	err := ThrowError(51000, "Custom error message", 1)

	if err.Number != 51000 {
		t.Errorf("Error number = %d, expected 51000", err.Number)
	}

	if err.Severity != 16 {
		t.Errorf("Severity = %d, expected 16", err.Severity)
	}

	if err.Message != "Custom error message" {
		t.Errorf("Message = %s", err.Message)
	}
}

func TestWrapError(t *testing.T) {
	tests := []struct {
		msg      string
		expected int
	}{
		{"divide by zero", ErrDivideByZero},
		{"arithmetic overflow", ErrArithmeticOverflow},
		{"null not allowed", ErrNullNotAllowed},
		{"duplicate key violation", ErrDuplicateKey},
		{"deadlock detected", ErrDeadlock},
		{"timeout expired", ErrTimeout},
		{"generic error", 50000},
	}

	for _, tt := range tests {
		t.Run(tt.msg, func(t *testing.T) {
			err := WrapError(fmt.Errorf(tt.msg))
			if err.Number != tt.expected {
				t.Errorf("WrapError(%q) = %d, expected %d", tt.msg, err.Number, tt.expected)
			}
		})
	}
}

func TestExecutionContext(t *testing.T) {
	ctx := NewExecutionContext(nil, DialectPostgres)

	// Test variable setting
	ctx.SetVariable("@x", NewInt(42))
	v, ok := ctx.GetVariable("@x")
	if !ok || v.AsInt() != 42 {
		t.Error("Failed to get variable @x")
	}

	// Test system variables
	ctx.UpdateRowCount(10)
	v, ok = ctx.GetVariable("@@ROWCOUNT")
	if !ok || v.AsInt() != 10 {
		t.Error("@@ROWCOUNT not updated")
	}

	ctx.UpdateLastInsertID(100)
	v, ok = ctx.GetVariable("@@IDENTITY")
	if !ok || v.AsInt() != 100 {
		t.Error("@@IDENTITY not updated")
	}

	ctx.UpdateFetchStatus(-1)
	v, ok = ctx.GetVariable("@@FETCH_STATUS")
	if !ok || v.AsInt() != -1 {
		t.Error("@@FETCH_STATUS not updated")
	}

	// Test child context
	child := ctx.NewChildContext()
	child.SetVariable("@y", NewInt(99))

	// Child should have parent's variable
	v, ok = child.GetVariable("@x")
	if !ok || v.AsInt() != 42 {
		t.Error("Child context should inherit parent variable")
	}

	// Parent should not have child's variable
	_, ok = ctx.GetVariable("@y")
	if ok {
		t.Error("Parent should not see child variable")
	}
}

func TestTempTableManager_ClearSession(t *testing.T) {
	manager := NewTempTableManager()

	// Create some tables
	manager.CreateTempTable("#temp1", []TempTableColumn{{Name: "ID", Type: TypeInt}})
	manager.CreateTempTable("##global1", []TempTableColumn{{Name: "ID", Type: TypeInt}})
	manager.CreateTableVariable("@var1", []TempTableColumn{{Name: "ID", Type: TypeInt}})

	// Clear session
	manager.ClearSession()

	// Local tables and table variables should be gone
	if manager.TempTableExists("#temp1") {
		t.Error("#temp1 should be cleared")
	}
	if _, ok := manager.GetTableVariable("@var1"); ok {
		t.Error("@var1 should be cleared")
	}

	// Global tables remain
	if !manager.TempTableExists("##global1") {
		t.Error("##global1 should remain after ClearSession")
	}
}
