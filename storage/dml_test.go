package storage

import (
	"strings"
	"testing"

	"github.com/ha1tch/tsqlparser"
	"github.com/ha1tch/tsqlparser/ast"
)

// =============================================================================
// Helper Functions
// =============================================================================

func parseProcedure(t *testing.T, sql string) *ast.CreateProcedureStatement {
	t.Helper()
	program, errs := tsqlparser.Parse(sql)
	if len(errs) > 0 {
		t.Fatalf("Parse failed: %v", errs)
	}
	if len(program.Statements) == 0 {
		t.Fatal("No statements parsed")
	}
	cp, ok := program.Statements[0].(*ast.CreateProcedureStatement)
	if !ok {
		t.Fatalf("Expected CreateProcedureStatement, got %T", program.Statements[0])
	}
	return cp
}

func detectOps(t *testing.T, sql string) []Operation {
	t.Helper()
	cp := parseProcedure(t, sql)
	detector := NewSQLDetector(DetectorConfig{IncludeRawSQL: true})
	ops, err := detector.DetectOperations(cp)
	if err != nil {
		t.Fatalf("DetectOperations failed: %v", err)
	}
	return ops
}

func assertOpCount(t *testing.T, ops []Operation, expected int) {
	t.Helper()
	if len(ops) != expected {
		t.Fatalf("Expected %d operations, got %d", expected, len(ops))
	}
}

func assertOpType(t *testing.T, op Operation, expected OperationType) {
	t.Helper()
	if op.Type != expected {
		t.Errorf("Expected operation type %v, got %v", expected, op.Type)
	}
}

func assertTable(t *testing.T, op Operation, expected string) {
	t.Helper()
	if op.Table != expected {
		t.Errorf("Expected table '%s', got '%s'", expected, op.Table)
	}
}

func assertFieldCount(t *testing.T, op Operation, expected int) {
	t.Helper()
	if len(op.Fields) != expected {
		t.Errorf("Expected %d fields, got %d: %+v", expected, len(op.Fields), op.Fields)
	}
}

func assertKeyFieldCount(t *testing.T, op Operation, expected int) {
	t.Helper()
	if len(op.KeyFields) != expected {
		t.Errorf("Expected %d key fields, got %d: %+v", expected, len(op.KeyFields), op.KeyFields)
	}
}

func hasFieldNamed(op Operation, name string) bool {
	for _, f := range op.Fields {
		if f.Name == name {
			return true
		}
	}
	return false
}

func hasKeyFieldNamed(op Operation, name string) bool {
	for _, f := range op.KeyFields {
		if f.Name == name {
			return true
		}
	}
	return false
}

// =============================================================================
// POSITIVE TESTS - SELECT
// =============================================================================

func TestDML_Select_Simple(t *testing.T) {
	sql := `
CREATE PROCEDURE SelectSimple AS
BEGIN
    SELECT ID, Name, Email FROM Users
END`
	ops := detectOps(t, sql)
	assertOpCount(t, ops, 1)
	assertOpType(t, ops[0], OpSelect)
	assertTable(t, ops[0], "Users")
	assertFieldCount(t, ops[0], 3)
}

func TestDML_Select_WithWhere(t *testing.T) {
	sql := `
CREATE PROCEDURE SelectWithWhere @ID INT AS
BEGIN
    SELECT Name FROM Users WHERE ID = @ID
END`
	ops := detectOps(t, sql)
	assertOpCount(t, ops, 1)
	assertKeyFieldCount(t, ops[0], 1)
	if !hasKeyFieldNamed(ops[0], "ID") {
		t.Error("Expected key field 'ID'")
	}
}

func TestDML_Select_WithMultipleWhereConditions(t *testing.T) {
	sql := `
CREATE PROCEDURE SelectMultiWhere @Status INT, @Type INT AS
BEGIN
    SELECT * FROM Orders WHERE Status = @Status AND Type = @Type
END`
	ops := detectOps(t, sql)
	assertOpCount(t, ops, 1)
	assertKeyFieldCount(t, ops[0], 2)
}

func TestDML_Select_WithInnerJoin(t *testing.T) {
	sql := `
CREATE PROCEDURE SelectWithJoin AS
BEGIN
    SELECT u.Name, o.Amount
    FROM Users u
    INNER JOIN Orders o ON u.ID = o.UserID
END`
	ops := detectOps(t, sql)
	assertOpCount(t, ops, 1)
	assertFieldCount(t, ops[0], 2)
	// Note: JOIN conditions are tracked but may not always appear in KeyFields
	// depending on how the FROM clause is structured
}

func TestDML_Select_WithLeftJoin(t *testing.T) {
	sql := `
CREATE PROCEDURE SelectLeftJoin AS
BEGIN
    SELECT u.Name, ISNULL(o.Amount, 0) AS Amount
    FROM Users u
    LEFT JOIN Orders o ON u.ID = o.UserID
END`
	ops := detectOps(t, sql)
	assertOpCount(t, ops, 1)
	assertFieldCount(t, ops[0], 2)
}

func TestDML_Select_WithMultipleJoins(t *testing.T) {
	sql := `
CREATE PROCEDURE SelectMultiJoin AS
BEGIN
    SELECT u.Name, o.Amount, p.Name AS ProductName
    FROM Users u
    INNER JOIN Orders o ON u.ID = o.UserID
    INNER JOIN Products p ON o.ProductID = p.ID
END`
	ops := detectOps(t, sql)
	assertOpCount(t, ops, 1)
	assertFieldCount(t, ops[0], 3)
}

func TestDML_Select_WithSubqueryInWhere(t *testing.T) {
	sql := `
CREATE PROCEDURE SelectSubquery @MinAmount DECIMAL AS
BEGIN
    SELECT Name FROM Users
    WHERE ID IN (SELECT UserID FROM Orders WHERE Amount > @MinAmount)
END`
	ops := detectOps(t, sql)
	// Main SELECT + subquery SELECT
	if len(ops) < 1 {
		t.Fatal("Expected at least 1 operation")
	}
	assertOpType(t, ops[0], OpSelect)
}

func TestDML_Select_WithAlias(t *testing.T) {
	sql := `
CREATE PROCEDURE SelectAlias AS
BEGIN
    SELECT ID AS UserID, Name AS UserName FROM Users AS u
END`
	ops := detectOps(t, sql)
	assertOpCount(t, ops, 1)
	assertFieldCount(t, ops[0], 2)
}

func TestDML_Select_WithTop(t *testing.T) {
	sql := `
CREATE PROCEDURE SelectTop AS
BEGIN
    SELECT TOP 10 ID, Name FROM Users ORDER BY Name
END`
	ops := detectOps(t, sql)
	assertOpCount(t, ops, 1)
	assertFieldCount(t, ops[0], 2)
}

func TestDML_Select_WithDistinct(t *testing.T) {
	sql := `
CREATE PROCEDURE SelectDistinct AS
BEGIN
    SELECT DISTINCT Status FROM Orders
END`
	ops := detectOps(t, sql)
	assertOpCount(t, ops, 1)
	assertFieldCount(t, ops[0], 1)
}

func TestDML_Select_WithGroupBy(t *testing.T) {
	sql := `
CREATE PROCEDURE SelectGroupBy AS
BEGIN
    SELECT Status, COUNT(*) AS Total
    FROM Orders
    GROUP BY Status
END`
	ops := detectOps(t, sql)
	assertOpCount(t, ops, 1)
	assertFieldCount(t, ops[0], 2)
}

func TestDML_Select_WithHaving(t *testing.T) {
	sql := `
CREATE PROCEDURE SelectHaving AS
BEGIN
    SELECT UserID, SUM(Amount) AS Total
    FROM Orders
    GROUP BY UserID
    HAVING SUM(Amount) > 1000
END`
	ops := detectOps(t, sql)
	assertOpCount(t, ops, 1)
}

func TestDML_Select_VariableAssignment(t *testing.T) {
	sql := `
CREATE PROCEDURE SelectIntoVar @ID INT, @Result NVARCHAR(100) OUTPUT AS
BEGIN
    SELECT @Result = Name FROM Users WHERE ID = @ID
END`
	ops := detectOps(t, sql)
	assertOpCount(t, ops, 1)
	
	found := false
	for _, f := range ops[0].Fields {
		if f.Variable == "@Result" && f.IsAssigned {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected variable assignment to @Result")
	}
}

func TestDML_Select_MultipleVariableAssignments(t *testing.T) {
	sql := `
CREATE PROCEDURE SelectMultiVar @ID INT AS
BEGIN
    DECLARE @Name NVARCHAR(100), @Email NVARCHAR(255)
    SELECT @Name = Name, @Email = Email FROM Users WHERE ID = @ID
END`
	ops := detectOps(t, sql)
	assertOpCount(t, ops, 1)
	
	assignedVars := 0
	for _, f := range ops[0].Fields {
		if f.IsAssigned {
			assignedVars++
		}
	}
	if assignedVars != 2 {
		t.Errorf("Expected 2 variable assignments, got %d", assignedVars)
	}
}

func TestDML_Select_WithCTE(t *testing.T) {
	sql := `
CREATE PROCEDURE SelectCTE AS
BEGIN
    ;WITH ActiveUsers AS (
        SELECT ID, Name FROM Users WHERE IsActive = 1
    )
    SELECT * FROM ActiveUsers
END`
	
	program, errs := tsqlparser.Parse(sql)
	if len(errs) > 0 {
		t.Skipf("Parser doesn't fully support CTE in this context: %v", errs)
	}
	
	detector := NewSQLDetector(DetectorConfig{})
	ops, err := detector.DetectOperations(program.Statements[0])
	if err != nil {
		t.Skipf("CTE detection not fully supported: %v", err)
	}
	
	// CTE handling varies by parser - just verify no crash
	t.Logf("CTE test: found %d operations", len(ops))
}

func TestDML_Select_WithUnion(t *testing.T) {
	sql := `
CREATE PROCEDURE SelectUnion AS
BEGIN
    SELECT ID, Name FROM Users WHERE Type = 1
    UNION ALL
    SELECT ID, Name FROM Users WHERE Type = 2
END`
	ops := detectOps(t, sql)
	if len(ops) < 1 {
		t.Fatal("Expected at least 1 operation")
	}
}

func TestDML_Select_Star(t *testing.T) {
	sql := `
CREATE PROCEDURE SelectStar AS
BEGIN
    SELECT * FROM Users
END`
	ops := detectOps(t, sql)
	assertOpCount(t, ops, 1)
	
	hasStar := false
	for _, f := range ops[0].Fields {
		if f.Name == "*" {
			hasStar = true
			break
		}
	}
	if !hasStar {
		t.Error("Expected * field")
	}
}

func TestDML_Select_TableStar(t *testing.T) {
	sql := `
CREATE PROCEDURE SelectTableStar AS
BEGIN
    SELECT u.*, o.Amount
    FROM Users u
    JOIN Orders o ON u.ID = o.UserID
END`
	ops := detectOps(t, sql)
	assertOpCount(t, ops, 1)
}

// =============================================================================
// POSITIVE TESTS - INSERT
// =============================================================================

func TestDML_Insert_Simple(t *testing.T) {
	sql := `
CREATE PROCEDURE InsertSimple @Name NVARCHAR(100) AS
BEGIN
    INSERT INTO Users (Name) VALUES (@Name)
END`
	ops := detectOps(t, sql)
	assertOpCount(t, ops, 1)
	assertOpType(t, ops[0], OpInsert)
	assertTable(t, ops[0], "Users")
	assertFieldCount(t, ops[0], 1)
}

func TestDML_Insert_MultipleColumns(t *testing.T) {
	sql := `
CREATE PROCEDURE InsertMulti @Name NVARCHAR(100), @Email NVARCHAR(255), @Age INT AS
BEGIN
    INSERT INTO Users (Name, Email, Age) VALUES (@Name, @Email, @Age)
END`
	ops := detectOps(t, sql)
	assertOpCount(t, ops, 1)
	assertFieldCount(t, ops[0], 3)
}

func TestDML_Insert_MultipleRows(t *testing.T) {
	sql := `
CREATE PROCEDURE InsertMultiRow AS
BEGIN
    INSERT INTO StatusCodes (Code, Description)
    VALUES (1, 'Active'), (2, 'Inactive'), (3, 'Pending')
END`
	ops := detectOps(t, sql)
	assertOpCount(t, ops, 1)
	assertFieldCount(t, ops[0], 2)
}

func TestDML_Insert_WithOutput(t *testing.T) {
	sql := `
CREATE PROCEDURE InsertWithOutput @Name NVARCHAR(100) AS
BEGIN
    INSERT INTO Users (Name)
    OUTPUT INSERTED.ID, INSERTED.CreatedAt
    VALUES (@Name)
END`
	ops := detectOps(t, sql)
	assertOpCount(t, ops, 1)
	
	if len(ops[0].OutputFields) != 2 {
		t.Errorf("Expected 2 output fields, got %d", len(ops[0].OutputFields))
	}
}

func TestDML_Insert_WithOutputInto(t *testing.T) {
	sql := `
CREATE PROCEDURE InsertWithOutputInto @Name NVARCHAR(100) AS
BEGIN
    DECLARE @InsertedIDs TABLE (ID INT)
    
    INSERT INTO Users (Name)
    OUTPUT INSERTED.ID INTO @InsertedIDs
    VALUES (@Name)
END`
	ops := detectOps(t, sql)
	assertOpCount(t, ops, 1)
	assertOpType(t, ops[0], OpInsert)
}

func TestDML_Insert_Select(t *testing.T) {
	sql := `
CREATE PROCEDURE InsertSelect AS
BEGIN
    INSERT INTO ArchivedUsers (ID, Name, Email)
    SELECT ID, Name, Email FROM Users WHERE IsArchived = 1
END`
	ops := detectOps(t, sql)
	// INSERT + embedded SELECT
	if len(ops) < 2 {
		t.Fatalf("Expected at least 2 operations, got %d", len(ops))
	}
	
	// Order depends on detection: INSERT is detected first, then its embedded SELECT
	hasInsert := false
	hasSelect := false
	for _, op := range ops {
		if op.Type == OpInsert {
			hasInsert = true
		}
		if op.Type == OpSelect {
			hasSelect = true
		}
	}
	
	if !hasInsert {
		t.Error("Expected INSERT operation")
	}
	if !hasSelect {
		t.Error("Expected SELECT operation")
	}
}

func TestDML_Insert_DefaultValues(t *testing.T) {
	sql := `
CREATE PROCEDURE InsertDefault AS
BEGIN
    INSERT INTO AuditLog DEFAULT VALUES
END`
	ops := detectOps(t, sql)
	assertOpCount(t, ops, 1)
	assertOpType(t, ops[0], OpInsert)
}

func TestDML_Insert_WithTableHint(t *testing.T) {
	sql := `
CREATE PROCEDURE InsertWithHint @Name NVARCHAR(100) AS
BEGIN
    INSERT INTO Users WITH (TABLOCK) (Name) VALUES (@Name)
END`
	ops := detectOps(t, sql)
	assertOpCount(t, ops, 1)
	assertOpType(t, ops[0], OpInsert)
}

// =============================================================================
// POSITIVE TESTS - UPDATE
// =============================================================================

func TestDML_Update_Simple(t *testing.T) {
	sql := `
CREATE PROCEDURE UpdateSimple @ID INT, @Name NVARCHAR(100) AS
BEGIN
    UPDATE Users SET Name = @Name WHERE ID = @ID
END`
	ops := detectOps(t, sql)
	assertOpCount(t, ops, 1)
	assertOpType(t, ops[0], OpUpdate)
	assertTable(t, ops[0], "Users")
	assertFieldCount(t, ops[0], 1)
	assertKeyFieldCount(t, ops[0], 1)
}

func TestDML_Update_MultipleColumns(t *testing.T) {
	sql := `
CREATE PROCEDURE UpdateMulti @ID INT, @Name NVARCHAR(100), @Email NVARCHAR(255) AS
BEGIN
    UPDATE Users SET Name = @Name, Email = @Email WHERE ID = @ID
END`
	ops := detectOps(t, sql)
	assertOpCount(t, ops, 1)
	assertFieldCount(t, ops[0], 2)
}

func TestDML_Update_WithFrom(t *testing.T) {
	sql := `
CREATE PROCEDURE UpdateWithFrom @CustomerID INT, @Discount DECIMAL AS
BEGIN
    UPDATE o
    SET o.Discount = @Discount
    FROM Orders o
    INNER JOIN Customers c ON o.CustomerID = c.ID
    WHERE c.ID = @CustomerID
END`
	ops := detectOps(t, sql)
	assertOpCount(t, ops, 1)
	assertOpType(t, ops[0], OpUpdate)
	
	if ops[0].Alias != "o" {
		t.Errorf("Expected alias 'o', got '%s'", ops[0].Alias)
	}
	if ops[0].Table != "Orders" {
		t.Errorf("Expected table 'Orders', got '%s'", ops[0].Table)
	}
}

func TestDML_Update_WithOutput(t *testing.T) {
	sql := `
CREATE PROCEDURE UpdateWithOutput @ID INT, @Name NVARCHAR(100) AS
BEGIN
    UPDATE Users
    SET Name = @Name
    OUTPUT DELETED.Name AS OldName, INSERTED.Name AS NewName
    WHERE ID = @ID
END`
	ops := detectOps(t, sql)
	assertOpCount(t, ops, 1)
	
	if len(ops[0].OutputFields) != 2 {
		t.Errorf("Expected 2 output fields, got %d", len(ops[0].OutputFields))
	}
}

func TestDML_Update_WithTop(t *testing.T) {
	sql := `
CREATE PROCEDURE UpdateTop @Status INT AS
BEGIN
    UPDATE TOP (100) Orders
    SET Status = @Status
    WHERE Status = 0
END`
	ops := detectOps(t, sql)
	assertOpCount(t, ops, 1)
	assertOpType(t, ops[0], OpUpdate)
}

func TestDML_Update_CompoundOperator(t *testing.T) {
	sql := `
CREATE PROCEDURE UpdateCompound @ID INT, @Amount DECIMAL AS
BEGIN
    UPDATE Accounts SET Balance += @Amount WHERE ID = @ID
END`
	ops := detectOps(t, sql)
	assertOpCount(t, ops, 1)
	assertFieldCount(t, ops[0], 1)
}

func TestDML_Update_WithSubquery(t *testing.T) {
	sql := `
CREATE PROCEDURE UpdateSubquery @MinOrders INT AS
BEGIN
    UPDATE Users
    SET Status = 'Premium'
    WHERE ID IN (
        SELECT UserID FROM Orders
        GROUP BY UserID
        HAVING COUNT(*) >= @MinOrders
    )
END`
	ops := detectOps(t, sql)
	if len(ops) < 1 {
		t.Fatal("Expected at least 1 operation")
	}
	assertOpType(t, ops[0], OpUpdate)
}

func TestDML_Update_NoWhere(t *testing.T) {
	sql := `
CREATE PROCEDURE UpdateAll @Status INT AS
BEGIN
    UPDATE Orders SET Status = @Status
END`
	ops := detectOps(t, sql)
	assertOpCount(t, ops, 1)
	assertKeyFieldCount(t, ops[0], 0) // No WHERE clause
}

// =============================================================================
// POSITIVE TESTS - DELETE
// =============================================================================

func TestDML_Delete_Simple(t *testing.T) {
	sql := `
CREATE PROCEDURE DeleteSimple @ID INT AS
BEGIN
    DELETE FROM Users WHERE ID = @ID
END`
	ops := detectOps(t, sql)
	assertOpCount(t, ops, 1)
	assertOpType(t, ops[0], OpDelete)
	assertTable(t, ops[0], "Users")
	assertKeyFieldCount(t, ops[0], 1)
}

func TestDML_Delete_WithFrom(t *testing.T) {
	sql := `
CREATE PROCEDURE DeleteWithFrom @CustomerID INT AS
BEGIN
    DELETE o
    FROM Orders o
    INNER JOIN Customers c ON o.CustomerID = c.ID
    WHERE c.ID = @CustomerID AND c.IsDeleted = 1
END`
	ops := detectOps(t, sql)
	assertOpCount(t, ops, 1)
	assertOpType(t, ops[0], OpDelete)
	
	if ops[0].Alias != "o" {
		t.Errorf("Expected alias 'o', got '%s'", ops[0].Alias)
	}
}

func TestDML_Delete_WithOutput(t *testing.T) {
	sql := `
CREATE PROCEDURE DeleteWithOutput @ID INT AS
BEGIN
    DELETE FROM Users
    OUTPUT DELETED.ID, DELETED.Name, DELETED.Email
    WHERE ID = @ID
END`
	ops := detectOps(t, sql)
	assertOpCount(t, ops, 1)
	
	if len(ops[0].OutputFields) != 3 {
		t.Errorf("Expected 3 output fields, got %d", len(ops[0].OutputFields))
	}
}

func TestDML_Delete_WithTop(t *testing.T) {
	sql := `
CREATE PROCEDURE DeleteTop AS
BEGIN
    DELETE TOP (1000) FROM AuditLog WHERE CreatedAt < DATEADD(year, -1, GETDATE())
END`
	ops := detectOps(t, sql)
	assertOpCount(t, ops, 1)
	assertOpType(t, ops[0], OpDelete)
}

func TestDML_Delete_NoWhere(t *testing.T) {
	sql := `
CREATE PROCEDURE DeleteAll AS
BEGIN
    DELETE FROM TempData
END`
	ops := detectOps(t, sql)
	assertOpCount(t, ops, 1)
	assertKeyFieldCount(t, ops[0], 0)
}

func TestDML_Delete_MultipleConditions(t *testing.T) {
	sql := `
CREATE PROCEDURE DeleteMultiCond @Status INT, @Type INT AS
BEGIN
    DELETE FROM Orders WHERE Status = @Status AND Type = @Type
END`
	ops := detectOps(t, sql)
	assertOpCount(t, ops, 1)
	assertKeyFieldCount(t, ops[0], 2)
}

// =============================================================================
// POSITIVE TESTS - MERGE
// =============================================================================

func TestDML_Merge_Simple(t *testing.T) {
	sql := `
CREATE PROCEDURE MergeSimple @ID INT, @Name NVARCHAR(100) AS
BEGIN
    MERGE Users AS target
    USING (SELECT @ID AS ID, @Name AS Name) AS source
    ON target.ID = source.ID
    WHEN MATCHED THEN
        UPDATE SET Name = source.Name
    WHEN NOT MATCHED THEN
        INSERT (ID, Name) VALUES (source.ID, source.Name);
END`
	ops := detectOps(t, sql)
	if len(ops) < 1 {
		t.Fatal("Expected at least 1 operation")
	}
	// MERGE detected as UPDATE (upsert pattern)
	assertOpType(t, ops[0], OpUpdate)
	assertTable(t, ops[0], "Users")
}

// =============================================================================
// POSITIVE TESTS - EXEC
// =============================================================================

func TestDML_Exec_Simple(t *testing.T) {
	sql := `
CREATE PROCEDURE ExecSimple @ID INT AS
BEGIN
    EXEC ProcessOrder @ID
END`
	ops := detectOps(t, sql)
	assertOpCount(t, ops, 1)
	assertOpType(t, ops[0], OpExec)
	
	if ops[0].CalledProcedure != "ProcessOrder" {
		t.Errorf("Expected 'ProcessOrder', got '%s'", ops[0].CalledProcedure)
	}
}

func TestDML_Exec_WithNamedParams(t *testing.T) {
	sql := `
CREATE PROCEDURE ExecNamed @UserID INT, @Amount DECIMAL AS
BEGIN
    EXEC CreateOrder @UserID = @UserID, @Amount = @Amount
END`
	ops := detectOps(t, sql)
	assertOpCount(t, ops, 1)
	
	if len(ops[0].Parameters) != 2 {
		t.Errorf("Expected 2 parameters, got %d", len(ops[0].Parameters))
	}
}

func TestDML_Exec_WithOutputParam(t *testing.T) {
	sql := `
CREATE PROCEDURE ExecOutput @ID INT AS
BEGIN
    DECLARE @Result INT
    EXEC GetResult @ID, @Result OUTPUT
END`
	ops := detectOps(t, sql)
	assertOpCount(t, ops, 1)
	
	hasOutput := false
	for _, p := range ops[0].Parameters {
		if p.IsAssigned {
			hasOutput = true
			break
		}
	}
	if !hasOutput {
		t.Error("Expected OUTPUT parameter")
	}
}

func TestDML_Exec_SchemaQualified(t *testing.T) {
	sql := `
CREATE PROCEDURE ExecSchema AS
BEGIN
    EXEC dbo.ProcessData
END`
	ops := detectOps(t, sql)
	assertOpCount(t, ops, 1)
	
	if ops[0].CalledProcedure != "dbo.ProcessData" {
		t.Errorf("Expected 'dbo.ProcessData', got '%s'", ops[0].CalledProcedure)
	}
}

// =============================================================================
// POSITIVE TESTS - Control Flow
// =============================================================================

func TestDML_IfElse(t *testing.T) {
	sql := `
CREATE PROCEDURE IfElse @ID INT, @Action INT AS
BEGIN
    IF @Action = 1
    BEGIN
        INSERT INTO AuditLog (Action) VALUES ('Insert')
    END
    ELSE
    BEGIN
        INSERT INTO AuditLog (Action) VALUES ('Other')
    END
END`
	ops := detectOps(t, sql)
	assertOpCount(t, ops, 2)
	assertOpType(t, ops[0], OpInsert)
	assertOpType(t, ops[1], OpInsert)
}

func TestDML_IfExists(t *testing.T) {
	sql := `
CREATE PROCEDURE IfExists @ID INT AS
BEGIN
    IF EXISTS (SELECT 1 FROM Users WHERE ID = @ID)
    BEGIN
        UPDATE Users SET LastAccess = GETDATE() WHERE ID = @ID
    END
END`
	ops := detectOps(t, sql)
	assertOpCount(t, ops, 2)
	assertOpType(t, ops[0], OpSelect)
	assertOpType(t, ops[1], OpUpdate)
}

func TestDML_IfNotExists(t *testing.T) {
	sql := `
CREATE PROCEDURE IfNotExists @ID INT, @Name NVARCHAR(100) AS
BEGIN
    IF NOT EXISTS (SELECT 1 FROM Users WHERE ID = @ID)
    BEGIN
        INSERT INTO Users (ID, Name) VALUES (@ID, @Name)
    END
END`
	ops := detectOps(t, sql)
	assertOpCount(t, ops, 2)
}

func TestDML_While(t *testing.T) {
	sql := `
CREATE PROCEDURE WhileLoop AS
BEGIN
    DECLARE @i INT = 0
    WHILE @i < 10
    BEGIN
        INSERT INTO Numbers (Value) VALUES (@i)
        SET @i = @i + 1
    END
END`
	ops := detectOps(t, sql)
	assertOpCount(t, ops, 1)
	assertOpType(t, ops[0], OpInsert)
}

func TestDML_TryCatch(t *testing.T) {
	sql := `
CREATE PROCEDURE TryCatch @Name NVARCHAR(100) AS
BEGIN
    BEGIN TRY
        INSERT INTO Users (Name) VALUES (@Name)
    END TRY
    BEGIN CATCH
        INSERT INTO ErrorLog (Message) VALUES (ERROR_MESSAGE())
    END CATCH
END`
	ops := detectOps(t, sql)
	assertOpCount(t, ops, 2)
	assertOpType(t, ops[0], OpInsert)
	assertOpType(t, ops[1], OpInsert)
}

func TestDML_NestedIfElse(t *testing.T) {
	sql := `
CREATE PROCEDURE NestedIf @Type INT AS
BEGIN
    IF @Type = 1
    BEGIN
        IF EXISTS (SELECT 1 FROM Config WHERE Key = 'Enabled')
        BEGIN
            INSERT INTO Log (Msg) VALUES ('Type1-Enabled')
        END
    END
    ELSE
    BEGIN
        INSERT INTO Log (Msg) VALUES ('Other')
    END
END`
	ops := detectOps(t, sql)
	assertOpCount(t, ops, 3)
}

// =============================================================================
// POSITIVE TESTS - Complex Scenarios
// =============================================================================

func TestDML_Transaction(t *testing.T) {
	sql := `
CREATE PROCEDURE TransactionProc @FromID INT, @ToID INT, @Amount DECIMAL AS
BEGIN
    BEGIN TRANSACTION
    
    UPDATE Accounts SET Balance = Balance - @Amount WHERE ID = @FromID
    UPDATE Accounts SET Balance = Balance + @Amount WHERE ID = @ToID
    INSERT INTO Transfers (FromID, ToID, Amount) VALUES (@FromID, @ToID, @Amount)
    
    COMMIT TRANSACTION
END`
	ops := detectOps(t, sql)
	assertOpCount(t, ops, 3)
	assertOpType(t, ops[0], OpUpdate)
	assertOpType(t, ops[1], OpUpdate)
	assertOpType(t, ops[2], OpInsert)
}

func TestDML_MultipleTables(t *testing.T) {
	sql := `
CREATE PROCEDURE MultiTable @UserID INT AS
BEGIN
    SELECT * FROM Users WHERE ID = @UserID
    SELECT * FROM Orders WHERE UserID = @UserID
    SELECT * FROM Payments WHERE UserID = @UserID
END`
	ops := detectOps(t, sql)
	assertOpCount(t, ops, 3)
	
	tables := make(map[string]bool)
	for _, op := range ops {
		tables[op.Table] = true
	}
	
	if !tables["Users"] || !tables["Orders"] || !tables["Payments"] {
		t.Error("Expected all three tables")
	}
}

func TestDML_CRUDComplete(t *testing.T) {
	sql := `
CREATE PROCEDURE CRUD @ID INT, @Name NVARCHAR(100), @Action CHAR(1) AS
BEGIN
    IF @Action = 'C'
        INSERT INTO Items (Name) VALUES (@Name)
    ELSE IF @Action = 'R'
        SELECT * FROM Items WHERE ID = @ID
    ELSE IF @Action = 'U'
        UPDATE Items SET Name = @Name WHERE ID = @ID
    ELSE IF @Action = 'D'
        DELETE FROM Items WHERE ID = @ID
END`
	ops := detectOps(t, sql)
	assertOpCount(t, ops, 4)
	
	types := make(map[OperationType]bool)
	for _, op := range ops {
		types[op.Type] = true
	}
	
	if !types[OpInsert] || !types[OpSelect] || !types[OpUpdate] || !types[OpDelete] {
		t.Error("Expected all CRUD operations")
	}
}

// =============================================================================
// NEGATIVE TESTS - Error Handling
// =============================================================================

func TestDML_Negative_NotProcedure(t *testing.T) {
	detector := NewSQLDetector(DetectorConfig{})
	
	// Pass wrong type
	_, err := detector.DetectOperations("not a procedure")
	if err == nil {
		t.Error("Expected error for non-procedure input")
	}
}

func TestDML_Negative_NilBody(t *testing.T) {
	sql := `CREATE PROCEDURE EmptyProc AS SELECT 1`
	program, errs := tsqlparser.Parse(sql)
	if len(errs) > 0 {
		t.Skipf("Parser doesn't support this syntax: %v", errs)
	}
	
	detector := NewSQLDetector(DetectorConfig{})
	ops, err := detector.DetectOperations(program.Statements[0])
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	
	// Should handle gracefully
	t.Logf("Operations found: %d", len(ops))
}

func TestDML_Negative_DynamicSQL(t *testing.T) {
	sql := `
CREATE PROCEDURE DynamicSQL @Table NVARCHAR(100) AS
BEGIN
    EXEC('SELECT * FROM ' + @Table)
END`
	
	cp := parseProcedure(t, sql)
	detector := NewSQLDetector(DetectorConfig{})
	ops, _ := detector.DetectOperations(cp)
	
	// Dynamic SQL should generate warning, not operation
	warnings := detector.GetWarnings()
	if len(warnings) == 0 {
		t.Error("Expected warning for dynamic SQL")
	}
	
	// Should not have detected an OpSelect (can't analyze dynamic SQL)
	for _, op := range ops {
		if op.Type == OpSelect {
			t.Error("Should not detect SELECT from dynamic SQL")
		}
	}
}

// =============================================================================
// NEGATIVE TESTS - Edge Cases
// =============================================================================

func TestDML_Edge_EmptyProcedure(t *testing.T) {
	sql := `
CREATE PROCEDURE EmptyProc AS
BEGIN
    -- Just a comment
    DECLARE @x INT = 1
END`
	ops := detectOps(t, sql)
	assertOpCount(t, ops, 0)
}

func TestDML_Edge_OnlyDeclare(t *testing.T) {
	sql := `
CREATE PROCEDURE DeclareOnly AS
BEGIN
    DECLARE @x INT
    DECLARE @y NVARCHAR(100)
    SET @x = 1
    SET @y = 'test'
END`
	ops := detectOps(t, sql)
	assertOpCount(t, ops, 0)
}

func TestDML_Edge_SelectNoFrom(t *testing.T) {
	sql := `
CREATE PROCEDURE SelectNoFrom AS
BEGIN
    SELECT 1 AS One, 'test' AS Text, GETDATE() AS Now
END`
	ops := detectOps(t, sql)
	assertOpCount(t, ops, 1)
	assertOpType(t, ops[0], OpSelect)
	// No table
	if ops[0].Table != "" {
		t.Logf("Table detected: %s (may be empty string)", ops[0].Table)
	}
}

func TestDML_Edge_QualifiedTableName(t *testing.T) {
	sql := `
CREATE PROCEDURE QualifiedTable AS
BEGIN
    SELECT * FROM dbo.Users
    SELECT * FROM OtherDB.dbo.Items
    SELECT * FROM LinkedServer.RemoteDB.dbo.Products
END`
	ops := detectOps(t, sql)
	assertOpCount(t, ops, 3)
	
	// Should capture full qualified names
	for i, op := range ops {
		if op.Table == "" {
			t.Errorf("Operation %d has empty table", i)
		}
	}
}

func TestDML_Edge_ReservedWordColumn(t *testing.T) {
	sql := `
CREATE PROCEDURE ReservedWords AS
BEGIN
    SELECT [Order], [Select], [From] FROM [Table]
END`
	ops := detectOps(t, sql)
	assertOpCount(t, ops, 1)
	assertFieldCount(t, ops[0], 3)
}

func TestDML_Edge_UnicodeNames(t *testing.T) {
	sql := `
CREATE PROCEDURE UnicodeProc AS
BEGIN
    SELECT ID, [Nombre], [Descripción] FROM [Usuarios]
END`
	ops := detectOps(t, sql)
	assertOpCount(t, ops, 1)
}

func TestDML_Edge_VeryLongProcedure(t *testing.T) {
	// Generate a procedure with many statements
	sql := `CREATE PROCEDURE LongProc AS BEGIN `
	for i := 0; i < 50; i++ {
		sql += `INSERT INTO Log (Msg) VALUES ('Entry'); `
	}
	sql += `END`
	
	ops := detectOps(t, sql)
	assertOpCount(t, ops, 50)
}

func TestDML_Edge_DeepNesting(t *testing.T) {
	sql := `
CREATE PROCEDURE DeepNest @Level INT AS
BEGIN
    IF @Level > 0
    BEGIN
        IF @Level > 1
        BEGIN
            IF @Level > 2
            BEGIN
                IF @Level > 3
                BEGIN
                    INSERT INTO DeepLog (Level) VALUES (@Level)
                END
            END
        END
    END
END`
	ops := detectOps(t, sql)
	assertOpCount(t, ops, 1)
	assertOpType(t, ops[0], OpInsert)
}

// =============================================================================
// TESTS - Model and Repository Detection
// =============================================================================

func TestDML_DetectModels(t *testing.T) {
	sql := `
CREATE PROCEDURE MultiTableProc AS
BEGIN
    SELECT ID, Name FROM Users
    SELECT ID, Amount FROM Orders
    INSERT INTO Payments (UserID, Amount) VALUES (1, 100)
END`
	
	cp := parseProcedure(t, sql)
	detector := NewSQLDetector(DetectorConfig{})
	ops, _ := detector.DetectOperations(cp)
	
	models, err := detector.DetectModels(ops)
	if err != nil {
		t.Fatalf("DetectModels failed: %v", err)
	}
	
	if len(models) != 3 {
		t.Errorf("Expected 3 models, got %d", len(models))
	}
	
	modelNames := make(map[string]bool)
	for _, m := range models {
		modelNames[m.Name] = true
	}
	
	expected := []string{"Users", "Orders", "Payments"}
	for _, name := range expected {
		if !modelNames[name] {
			t.Errorf("Missing model: %s", name)
		}
	}
}

func TestDML_DetectModels_MergesFields(t *testing.T) {
	sql := `
CREATE PROCEDURE UserOps AS
BEGIN
    SELECT ID, Name FROM Users
    SELECT ID, Email FROM Users
    UPDATE Users SET Status = 1 WHERE ID = 1
END`
	
	cp := parseProcedure(t, sql)
	detector := NewSQLDetector(DetectorConfig{})
	ops, _ := detector.DetectOperations(cp)
	
	models, _ := detector.DetectModels(ops)
	
	if len(models) != 1 {
		t.Fatalf("Expected 1 model, got %d", len(models))
	}
	
	// Should have merged fields from all operations
	fieldNames := make(map[string]bool)
	for _, f := range models[0].Fields {
		fieldNames[f.Name] = true
	}
	
	// Should have ID, Name, Email, Status
	if len(fieldNames) < 3 {
		t.Errorf("Expected at least 3 fields, got %d", len(fieldNames))
	}
}

func TestDML_DetectRepositories(t *testing.T) {
	sql := `
CREATE PROCEDURE UserCRUD @ID INT, @Name NVARCHAR(100) AS
BEGIN
    SELECT ID, Name FROM Users WHERE ID = @ID
    INSERT INTO Users (Name) VALUES (@Name)
    UPDATE Users SET Name = @Name WHERE ID = @ID
    DELETE FROM Users WHERE ID = @ID
END`
	
	cp := parseProcedure(t, sql)
	detector := NewSQLDetector(DetectorConfig{})
	ops, _ := detector.DetectOperations(cp)
	models, _ := detector.DetectModels(ops)
	repos, err := detector.DetectRepositories(ops, models)
	
	if err != nil {
		t.Fatalf("DetectRepositories failed: %v", err)
	}
	
	if len(repos) != 1 {
		t.Fatalf("Expected 1 repository, got %d", len(repos))
	}
	
	repo := repos[0]
	if repo.Name != "UsersRepository" {
		t.Errorf("Expected 'UsersRepository', got '%s'", repo.Name)
	}
	
	if len(repo.Methods) < 4 {
		t.Errorf("Expected at least 4 methods, got %d", len(repo.Methods))
	}
}

// =============================================================================
// TESTS - WHERE Clause Analysis
// =============================================================================

func TestDML_Where_Equality(t *testing.T) {
	sql := `
CREATE PROCEDURE WhereEq @ID INT AS
BEGIN
    SELECT * FROM Users WHERE ID = @ID
END`
	ops := detectOps(t, sql)
	assertKeyFieldCount(t, ops[0], 1)
	if ops[0].KeyFields[0].Variable != "@ID" {
		t.Errorf("Expected variable @ID, got %s", ops[0].KeyFields[0].Variable)
	}
}

func TestDML_Where_And(t *testing.T) {
	sql := `
CREATE PROCEDURE WhereAnd @Status INT, @Type INT AS
BEGIN
    SELECT * FROM Orders WHERE Status = @Status AND Type = @Type
END`
	ops := detectOps(t, sql)
	assertKeyFieldCount(t, ops[0], 2)
}

func TestDML_Where_Or(t *testing.T) {
	sql := `
CREATE PROCEDURE WhereOr @Status1 INT, @Status2 INT AS
BEGIN
    SELECT * FROM Orders WHERE Status = @Status1 OR Status = @Status2
END`
	ops := detectOps(t, sql)
	assertKeyFieldCount(t, ops[0], 2)
}

func TestDML_Where_In(t *testing.T) {
	sql := `
CREATE PROCEDURE WhereIn AS
BEGIN
    SELECT * FROM Users WHERE Status IN (1, 2, 3)
END`
	ops := detectOps(t, sql)
	assertKeyFieldCount(t, ops[0], 1)
	if !hasKeyFieldNamed(ops[0], "Status") {
		t.Error("Expected Status in key fields")
	}
}

func TestDML_Where_Between(t *testing.T) {
	sql := `
CREATE PROCEDURE WhereBetween @Start DATE, @End DATE AS
BEGIN
    SELECT * FROM Orders WHERE OrderDate BETWEEN @Start AND @End
END`
	ops := detectOps(t, sql)
	assertKeyFieldCount(t, ops[0], 1)
	if !hasKeyFieldNamed(ops[0], "OrderDate") {
		t.Error("Expected OrderDate in key fields")
	}
}

func TestDML_Where_Like(t *testing.T) {
	sql := `
CREATE PROCEDURE WhereLike @Pattern NVARCHAR(100) AS
BEGIN
    SELECT * FROM Users WHERE Name LIKE @Pattern
END`
	ops := detectOps(t, sql)
	assertKeyFieldCount(t, ops[0], 1)
	if !hasKeyFieldNamed(ops[0], "Name") {
		t.Error("Expected Name in key fields")
	}
}

func TestDML_Where_IsNull(t *testing.T) {
	sql := `
CREATE PROCEDURE WhereNull AS
BEGIN
    SELECT * FROM Users WHERE DeletedAt IS NULL
END`
	ops := detectOps(t, sql)
	assertKeyFieldCount(t, ops[0], 1)
	if !hasKeyFieldNamed(ops[0], "DeletedAt") {
		t.Error("Expected DeletedAt in key fields")
	}
}

func TestDML_Where_IsNotNull(t *testing.T) {
	sql := `
CREATE PROCEDURE WhereNotNull AS
BEGIN
    SELECT * FROM Users WHERE Email IS NOT NULL
END`
	ops := detectOps(t, sql)
	assertKeyFieldCount(t, ops[0], 1)
}

func TestDML_Where_Comparison(t *testing.T) {
	sql := `
CREATE PROCEDURE WhereComparison @MinAmount DECIMAL AS
BEGIN
    SELECT * FROM Orders WHERE Amount > @MinAmount
END`
	ops := detectOps(t, sql)
	assertKeyFieldCount(t, ops[0], 1)
	if !hasKeyFieldNamed(ops[0], "Amount") {
		t.Error("Expected Amount in key fields")
	}
}

func TestDML_Where_Complex(t *testing.T) {
	sql := `
CREATE PROCEDURE WhereComplex @Status INT, @MinAmount DECIMAL, @Type INT AS
BEGIN
    SELECT * FROM Orders 
    WHERE (Status = @Status OR Status = 0)
    AND Amount >= @MinAmount
    AND Type IN (1, 2, @Type)
END`
	ops := detectOps(t, sql)
	// Should extract multiple key fields
	if len(ops[0].KeyFields) < 2 {
		t.Errorf("Expected multiple key fields, got %d", len(ops[0].KeyFields))
	}
}

// =============================================================================
// HIGH PRIORITY TESTS - Temporary Tables (NEGATIVE)
// =============================================================================

func TestDML_TempTable_LocalTemp(t *testing.T) {
	sql := `
CREATE PROCEDURE UseTempTable AS
BEGIN
    CREATE TABLE #TempResults (ID INT, Value NVARCHAR(100))
    
    INSERT INTO #TempResults (ID, Value)
    SELECT ID, Name FROM Users
    
    SELECT * FROM #TempResults
    
    DROP TABLE #TempResults
END`
	
	cp := parseProcedure(t, sql)
	detector := NewSQLDetector(DetectorConfig{})
	ops, _ := detector.DetectOperations(cp)
	
	// Should detect operations on temp table
	hasTempOp := false
	for _, op := range ops {
		if strings.HasPrefix(op.Table, "#") {
			hasTempOp = true
			break
		}
	}
	if !hasTempOp {
		t.Log("No temp table operations detected (may be expected if CREATE TABLE not parsed)")
	}
	
	// But DetectModels should NOT create a model for #TempResults
	models, _ := detector.DetectModels(ops)
	for _, m := range models {
		if strings.HasPrefix(m.Table, "#") {
			t.Errorf("Temp table %s should NOT become a model", m.Table)
		}
	}
}

func TestDML_TempTable_GlobalTemp(t *testing.T) {
	sql := `
CREATE PROCEDURE UseGlobalTemp AS
BEGIN
    INSERT INTO ##GlobalTemp (ID, Value) VALUES (1, 'test')
    SELECT * FROM ##GlobalTemp
END`
	
	cp := parseProcedure(t, sql)
	detector := NewSQLDetector(DetectorConfig{})
	ops, _ := detector.DetectOperations(cp)
	
	models, _ := detector.DetectModels(ops)
	for _, m := range models {
		if strings.HasPrefix(m.Table, "#") {
			t.Errorf("Global temp table %s should NOT become a model", m.Table)
		}
	}
}

func TestDML_TempTable_TableVariable(t *testing.T) {
	sql := `
CREATE PROCEDURE UseTableVar AS
BEGIN
    DECLARE @Results TABLE (ID INT, Name NVARCHAR(100))
    
    INSERT INTO @Results (ID, Name)
    SELECT ID, Name FROM Users
    
    SELECT * FROM @Results
END`
	
	cp := parseProcedure(t, sql)
	detector := NewSQLDetector(DetectorConfig{})
	ops, _ := detector.DetectOperations(cp)
	
	models, _ := detector.DetectModels(ops)
	for _, m := range models {
		if strings.HasPrefix(m.Table, "@") {
			t.Errorf("Table variable %s should NOT become a model", m.Table)
		}
	}
}

func TestDML_TempTable_MixedWithRealTables(t *testing.T) {
	sql := `
CREATE PROCEDURE MixedTables AS
BEGIN
    -- Real table
    SELECT ID, Name FROM Users
    
    -- Temp table
    INSERT INTO #Temp (UserID) SELECT ID FROM Users
    SELECT * FROM #Temp
    
    -- Another real table
    INSERT INTO AuditLog (Action) VALUES ('Processed')
END`
	
	cp := parseProcedure(t, sql)
	detector := NewSQLDetector(DetectorConfig{})
	ops, _ := detector.DetectOperations(cp)
	
	models, _ := detector.DetectModels(ops)
	
	// Should have models for Users and AuditLog, but NOT #Temp
	modelNames := make(map[string]bool)
	for _, m := range models {
		modelNames[m.Table] = true
		if strings.HasPrefix(m.Table, "#") || strings.HasPrefix(m.Table, "@") {
			t.Errorf("Temp table %s should NOT become a model", m.Table)
		}
	}
	
	if !modelNames["Users"] {
		t.Error("Expected model for Users table")
	}
	if !modelNames["AuditLog"] {
		t.Error("Expected model for AuditLog table")
	}
}

// =============================================================================
// HIGH PRIORITY TESTS - Cursors (NEGATIVE)
// =============================================================================

func TestDML_Cursor_DeclareCursor(t *testing.T) {
	sql := `
CREATE PROCEDURE UseCursor AS
BEGIN
    DECLARE @ID INT, @Name NVARCHAR(100)
    
    DECLARE user_cursor CURSOR FOR
        SELECT ID, Name FROM Users WHERE IsActive = 1
    
    OPEN user_cursor
    FETCH NEXT FROM user_cursor INTO @ID, @Name
    
    WHILE @@FETCH_STATUS = 0
    BEGIN
        INSERT INTO ProcessedUsers (UserID, UserName) VALUES (@ID, @Name)
        FETCH NEXT FROM user_cursor INTO @ID, @Name
    END
    
    CLOSE user_cursor
    DEALLOCATE user_cursor
END`
	
	cp := parseProcedure(t, sql)
	detector := NewSQLDetector(DetectorConfig{})
	_, _ = detector.DetectOperations(cp)
	
	// Should have warnings about cursor usage
	warnings := detector.GetWarnings()
	
	cursorWarnings := 0
	for _, w := range warnings {
		if strings.Contains(w.Message, "Cursor") || strings.Contains(w.Message, "cursor") {
			cursorWarnings++
		}
	}
	
	if cursorWarnings == 0 {
		t.Error("Expected warnings about cursor usage")
	}
}

func TestDML_Cursor_MultipleCursors(t *testing.T) {
	sql := `
CREATE PROCEDURE MultiCursor AS
BEGIN
    DECLARE cursor1 CURSOR FOR SELECT ID FROM Table1
    DECLARE cursor2 CURSOR FOR SELECT ID FROM Table2
    
    OPEN cursor1
    OPEN cursor2
    
    CLOSE cursor1
    CLOSE cursor2
    DEALLOCATE cursor1
    DEALLOCATE cursor2
END`
	
	cp := parseProcedure(t, sql)
	detector := NewSQLDetector(DetectorConfig{})
	_, _ = detector.DetectOperations(cp)
	
	warnings := detector.GetWarnings()
	
	// Should have multiple cursor-related warnings
	cursorWarnings := 0
	for _, w := range warnings {
		if strings.Contains(w.Message, "ursor") {
			cursorWarnings++
		}
	}
	
	if cursorWarnings < 2 {
		t.Errorf("Expected multiple cursor warnings, got %d", cursorWarnings)
	}
}

// =============================================================================
// HIGH PRIORITY TESTS - TRUNCATE (POSITIVE)
// =============================================================================

func TestDML_Truncate_Simple(t *testing.T) {
	sql := `
CREATE PROCEDURE TruncateTable AS
BEGIN
    TRUNCATE TABLE TempData
END`
	
	ops := detectOps(t, sql)
	assertOpCount(t, ops, 1)
	assertOpType(t, ops[0], OpTruncate)
	assertTable(t, ops[0], "TempData")
}

func TestDML_Truncate_WithSchema(t *testing.T) {
	sql := `
CREATE PROCEDURE TruncateWithSchema AS
BEGIN
    TRUNCATE TABLE dbo.AuditLog
END`
	
	ops := detectOps(t, sql)
	assertOpCount(t, ops, 1)
	assertOpType(t, ops[0], OpTruncate)
}

func TestDML_Truncate_GeneratesWarning(t *testing.T) {
	sql := `
CREATE PROCEDURE TruncateWarning AS
BEGIN
    TRUNCATE TABLE ImportantData
END`
	
	cp := parseProcedure(t, sql)
	detector := NewSQLDetector(DetectorConfig{})
	_, _ = detector.DetectOperations(cp)
	
	warnings := detector.GetWarnings()
	
	found := false
	for _, w := range warnings {
		if strings.Contains(w.Message, "TRUNCATE") {
			found = true
			break
		}
	}
	
	if !found {
		t.Error("Expected warning for TRUNCATE TABLE")
	}
}

// =============================================================================
// HIGH PRIORITY TESTS - Self-JOIN / Auto-JOIN (POSITIVE)
// =============================================================================

func TestDML_SelfJoin_EmployeeManager(t *testing.T) {
	sql := `
CREATE PROCEDURE GetEmployeeWithManager @EmployeeID INT AS
BEGIN
    SELECT 
        e.ID AS EmployeeID,
        e.Name AS EmployeeName,
        m.ID AS ManagerID,
        m.Name AS ManagerName
    FROM Employees e
    LEFT JOIN Employees m ON e.ManagerID = m.ID
    WHERE e.ID = @EmployeeID
END`
	
	ops := detectOps(t, sql)
	assertOpCount(t, ops, 1)
	assertOpType(t, ops[0], OpSelect)
	
	// Should have 4 fields
	assertFieldCount(t, ops[0], 4)
	
	// Table should be Employees
	if ops[0].Table != "Employees" {
		t.Errorf("Expected table 'Employees', got '%s'", ops[0].Table)
	}
}

func TestDML_SelfJoin_Hierarchy(t *testing.T) {
	sql := `
CREATE PROCEDURE GetCategoryPath @CategoryID INT AS
BEGIN
    SELECT 
        c.ID,
        c.Name,
        p.Name AS ParentName,
        gp.Name AS GrandparentName
    FROM Categories c
    LEFT JOIN Categories p ON c.ParentID = p.ID
    LEFT JOIN Categories gp ON p.ParentID = gp.ID
    WHERE c.ID = @CategoryID
END`
	
	ops := detectOps(t, sql)
	assertOpCount(t, ops, 1)
	assertFieldCount(t, ops[0], 4)
}

func TestDML_SelfJoin_Update(t *testing.T) {
	sql := `
CREATE PROCEDURE PromoteEmployee @EmployeeID INT, @NewManagerID INT AS
BEGIN
    UPDATE e
    SET e.ManagerID = @NewManagerID
    FROM Employees e
    INNER JOIN Employees m ON m.ID = @NewManagerID
    WHERE e.ID = @EmployeeID
END`
	
	ops := detectOps(t, sql)
	assertOpCount(t, ops, 1)
	assertOpType(t, ops[0], OpUpdate)
}

// =============================================================================
// HIGH PRIORITY TESTS - Correlated Subqueries (POSITIVE)
// =============================================================================

func TestDML_CorrelatedSubquery_Exists(t *testing.T) {
	sql := `
CREATE PROCEDURE GetUsersWithOrders AS
BEGIN
    SELECT u.ID, u.Name
    FROM Users u
    WHERE EXISTS (
        SELECT 1 FROM Orders o WHERE o.UserID = u.ID
    )
END`
	
	ops := detectOps(t, sql)
	
	// Should have at least the main SELECT
	// EXISTS subquery may or may not be detected separately depending on implementation
	if len(ops) < 1 {
		t.Fatalf("Expected at least 1 operation, got %d", len(ops))
	}
	
	assertOpType(t, ops[0], OpSelect)
	assertTable(t, ops[0], "Users")
}

func TestDML_CorrelatedSubquery_InSelect(t *testing.T) {
	sql := `
CREATE PROCEDURE GetUsersOrderCount AS
BEGIN
    SELECT 
        u.ID,
        u.Name,
        (SELECT COUNT(*) FROM Orders o WHERE o.UserID = u.ID) AS OrderCount
    FROM Users u
END`
	
	ops := detectOps(t, sql)
	assertOpCount(t, ops, 1) // Correlated subquery in SELECT may not be detected separately
	assertOpType(t, ops[0], OpSelect)
}

func TestDML_CorrelatedSubquery_NotIn(t *testing.T) {
	sql := `
CREATE PROCEDURE GetUsersWithoutOrders AS
BEGIN
    SELECT u.ID, u.Name
    FROM Users u
    WHERE u.ID NOT IN (
        SELECT o.UserID FROM Orders o WHERE o.Status = 'Active'
    )
END`
	
	ops := detectOps(t, sql)
	if len(ops) < 1 {
		t.Fatal("Expected at least 1 operation")
	}
	assertOpType(t, ops[0], OpSelect)
}

func TestDML_CorrelatedSubquery_Update(t *testing.T) {
	sql := `
CREATE PROCEDURE UpdateUserOrderCount AS
BEGIN
    UPDATE u
    SET u.OrderCount = (
        SELECT COUNT(*) FROM Orders o WHERE o.UserID = u.ID
    )
    FROM Users u
END`
	
	ops := detectOps(t, sql)
	assertOpCount(t, ops, 1)
	assertOpType(t, ops[0], OpUpdate)
}

// =============================================================================
// HIGH PRIORITY TESTS - DELETE/UPDATE sin WHERE (NEGATIVE)
// =============================================================================

func TestDML_DeleteNoWhere_GeneratesWarning(t *testing.T) {
	sql := `
CREATE PROCEDURE DeleteAll AS
BEGIN
    DELETE FROM TempData
END`
	
	cp := parseProcedure(t, sql)
	detector := NewSQLDetector(DetectorConfig{})
	ops, _ := detector.DetectOperations(cp)
	
	assertOpCount(t, ops, 1)
	assertOpType(t, ops[0], OpDelete)
	
	warnings := detector.GetWarnings()
	
	found := false
	for _, w := range warnings {
		if strings.Contains(w.Message, "DELETE") && strings.Contains(w.Message, "WHERE") {
			found = true
			break
		}
	}
	
	if !found {
		t.Error("Expected warning for DELETE without WHERE")
	}
}

func TestDML_UpdateNoWhere_GeneratesWarning(t *testing.T) {
	sql := `
CREATE PROCEDURE UpdateAll @Status INT AS
BEGIN
    UPDATE Orders SET Status = @Status
END`
	
	cp := parseProcedure(t, sql)
	detector := NewSQLDetector(DetectorConfig{})
	ops, _ := detector.DetectOperations(cp)
	
	assertOpCount(t, ops, 1)
	assertOpType(t, ops[0], OpUpdate)
	
	warnings := detector.GetWarnings()
	
	found := false
	for _, w := range warnings {
		if strings.Contains(w.Message, "UPDATE") && strings.Contains(w.Message, "WHERE") {
			found = true
			break
		}
	}
	
	if !found {
		t.Error("Expected warning for UPDATE without WHERE")
	}
}

func TestDML_DeleteWithWhere_NoWarning(t *testing.T) {
	sql := `
CREATE PROCEDURE DeleteSafe @ID INT AS
BEGIN
    DELETE FROM Users WHERE ID = @ID
END`
	
	cp := parseProcedure(t, sql)
	detector := NewSQLDetector(DetectorConfig{})
	_, _ = detector.DetectOperations(cp)
	
	warnings := detector.GetWarnings()
	
	for _, w := range warnings {
		if strings.Contains(w.Message, "DELETE") && strings.Contains(w.Message, "WHERE") {
			t.Errorf("Should NOT warn when DELETE has WHERE clause: %s", w.Message)
		}
	}
}

func TestDML_UpdateWithWhere_NoWarning(t *testing.T) {
	sql := `
CREATE PROCEDURE UpdateSafe @ID INT, @Name NVARCHAR(100) AS
BEGIN
    UPDATE Users SET Name = @Name WHERE ID = @ID
END`
	
	cp := parseProcedure(t, sql)
	detector := NewSQLDetector(DetectorConfig{})
	_, _ = detector.DetectOperations(cp)
	
	warnings := detector.GetWarnings()
	
	for _, w := range warnings {
		if strings.Contains(w.Message, "UPDATE") && strings.Contains(w.Message, "WHERE") {
			t.Errorf("Should NOT warn when UPDATE has WHERE clause: %s", w.Message)
		}
	}
}

func TestDML_DeleteWithFromJoin_WarnsIfNoWhere(t *testing.T) {
	sql := `
CREATE PROCEDURE DeleteJoinNoWhere AS
BEGIN
    DELETE o
    FROM Orders o
    INNER JOIN Customers c ON o.CustomerID = c.ID
END`
	
	cp := parseProcedure(t, sql)
	detector := NewSQLDetector(DetectorConfig{})
	_, _ = detector.DetectOperations(cp)
	
	warnings := detector.GetWarnings()
	
	found := false
	for _, w := range warnings {
		if strings.Contains(w.Message, "DELETE") {
			found = true
			break
		}
	}
	
	if !found {
		t.Error("Expected warning for DELETE with JOIN but no WHERE")
	}
}

// =============================================================================
// COMBINED SCENARIOS
// =============================================================================

func TestDML_ComplexProcedure_MultipleWarnings(t *testing.T) {
	sql := `
CREATE PROCEDURE ComplexProc AS
BEGIN
    -- Cursor (should warn)
    DECLARE cur CURSOR FOR SELECT ID FROM Users
    OPEN cur
    CLOSE cur
    DEALLOCATE cur
    
    -- DELETE without WHERE (should warn)
    DELETE FROM TempTable
    
    -- TRUNCATE (should warn)
    TRUNCATE TABLE AnotherTemp
    
    -- Safe operations (no warning)
    SELECT * FROM Users WHERE ID = 1
    UPDATE Users SET Name = 'test' WHERE ID = 1
END`
	
	cp := parseProcedure(t, sql)
	detector := NewSQLDetector(DetectorConfig{})
	_, _ = detector.DetectOperations(cp)
	
	warnings := detector.GetWarnings()
	
	// Should have at least 3 warnings: cursor, delete without where, truncate
	if len(warnings) < 3 {
		t.Errorf("Expected at least 3 warnings, got %d", len(warnings))
		for _, w := range warnings {
			t.Logf("  Warning: %s", w.Message)
		}
	}
}

