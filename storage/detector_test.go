package storage

import (
	"testing"

	"github.com/ha1tch/tsqlparser"
)

func parseSQL(t *testing.T, sql string) *tsqlparser.Program {
	program, errs := tsqlparser.Parse(sql)
	if len(errs) > 0 {
		t.Fatalf("Parse failed: %v", errs)
	}
	if len(program.Statements) == 0 {
		t.Fatal("No statements parsed")
	}
	return program
}

func TestSQLDetector_DetectOperations_Select(t *testing.T) {
	sql := `
CREATE PROCEDURE GetUserByID
    @UserID INT
AS
BEGIN
    SELECT ID, Name, Email
    FROM Users
    WHERE ID = @UserID
END
`
	program := parseSQL(t, sql)
	
	detector := NewSQLDetector(DetectorConfig{IncludeRawSQL: true})
	ops, err := detector.DetectOperations(program.Statements[0])
	if err != nil {
		t.Fatalf("DetectOperations failed: %v", err)
	}
	
	if len(ops) != 1 {
		t.Fatalf("Expected 1 operation, got %d", len(ops))
	}
	
	op := ops[0]
	if op.Type != OpSelect {
		t.Errorf("Expected OpSelect, got %v", op.Type)
	}
	
	if op.Table != "Users" {
		t.Errorf("Expected table 'Users', got '%s'", op.Table)
	}
	
	// Should have 3 fields: ID, Name, Email
	if len(op.Fields) != 3 {
		t.Errorf("Expected 3 fields, got %d", len(op.Fields))
	}
	
	// Should have 1 key field: ID
	if len(op.KeyFields) != 1 {
		t.Errorf("Expected 1 key field, got %d", len(op.KeyFields))
	}
	
	if op.KeyFields[0].Name != "ID" {
		t.Errorf("Expected key field 'ID', got '%s'", op.KeyFields[0].Name)
	}
}

func TestSQLDetector_DetectOperations_Insert(t *testing.T) {
	sql := `
CREATE PROCEDURE CreateUser
    @Name NVARCHAR(100),
    @Email NVARCHAR(255)
AS
BEGIN
    INSERT INTO Users (Name, Email)
    VALUES (@Name, @Email)
END
`
	program := parseSQL(t, sql)
	
	detector := NewSQLDetector(DetectorConfig{})
	ops, err := detector.DetectOperations(program.Statements[0])
	if err != nil {
		t.Fatalf("DetectOperations failed: %v", err)
	}
	
	if len(ops) != 1 {
		t.Fatalf("Expected 1 operation, got %d", len(ops))
	}
	
	op := ops[0]
	if op.Type != OpInsert {
		t.Errorf("Expected OpInsert, got %v", op.Type)
	}
	
	if op.Table != "Users" {
		t.Errorf("Expected table 'Users', got '%s'", op.Table)
	}
	
	// Should have 2 fields: Name, Email
	if len(op.Fields) != 2 {
		t.Errorf("Expected 2 fields, got %d", len(op.Fields))
	}
}

func TestSQLDetector_DetectOperations_Update(t *testing.T) {
	sql := `
CREATE PROCEDURE UpdateUser
    @UserID INT,
    @Name NVARCHAR(100)
AS
BEGIN
    UPDATE Users
    SET Name = @Name
    WHERE ID = @UserID
END
`
	program := parseSQL(t, sql)
	
	detector := NewSQLDetector(DetectorConfig{})
	ops, err := detector.DetectOperations(program.Statements[0])
	if err != nil {
		t.Fatalf("DetectOperations failed: %v", err)
	}
	
	if len(ops) != 1 {
		t.Fatalf("Expected 1 operation, got %d", len(ops))
	}
	
	op := ops[0]
	if op.Type != OpUpdate {
		t.Errorf("Expected OpUpdate, got %v", op.Type)
	}
	
	if op.Table != "Users" {
		t.Errorf("Expected table 'Users', got '%s'", op.Table)
	}
	
	// Should have 1 SET field: Name
	if len(op.Fields) != 1 {
		t.Errorf("Expected 1 field, got %d", len(op.Fields))
	}
	
	// Should have 1 key field: ID
	if len(op.KeyFields) != 1 {
		t.Errorf("Expected 1 key field, got %d", len(op.KeyFields))
	}
}

func TestSQLDetector_DetectOperations_Delete(t *testing.T) {
	sql := `
CREATE PROCEDURE DeleteUser
    @UserID INT
AS
BEGIN
    DELETE FROM Users
    WHERE ID = @UserID
END
`
	program := parseSQL(t, sql)
	
	detector := NewSQLDetector(DetectorConfig{})
	ops, err := detector.DetectOperations(program.Statements[0])
	if err != nil {
		t.Fatalf("DetectOperations failed: %v", err)
	}
	
	if len(ops) != 1 {
		t.Fatalf("Expected 1 operation, got %d", len(ops))
	}
	
	op := ops[0]
	if op.Type != OpDelete {
		t.Errorf("Expected OpDelete, got %v", op.Type)
	}
	
	if op.Table != "Users" {
		t.Errorf("Expected table 'Users', got '%s'", op.Table)
	}
	
	// Should have 1 key field: ID
	if len(op.KeyFields) != 1 {
		t.Errorf("Expected 1 key field, got %d", len(op.KeyFields))
	}
}

func TestSQLDetector_DetectOperations_MultipleStatements(t *testing.T) {
	sql := `
CREATE PROCEDURE TransferMoney
    @FromAccount INT,
    @ToAccount INT,
    @Amount DECIMAL(18,2)
AS
BEGIN
    UPDATE Accounts
    SET Balance = Balance - @Amount
    WHERE AccountID = @FromAccount
    
    UPDATE Accounts
    SET Balance = Balance + @Amount
    WHERE AccountID = @ToAccount
END
`
	program := parseSQL(t, sql)
	
	detector := NewSQLDetector(DetectorConfig{})
	ops, err := detector.DetectOperations(program.Statements[0])
	if err != nil {
		t.Fatalf("DetectOperations failed: %v", err)
	}
	
	if len(ops) != 2 {
		t.Fatalf("Expected 2 operations, got %d", len(ops))
	}
	
	for i, op := range ops {
		if op.Type != OpUpdate {
			t.Errorf("Operation %d: expected OpUpdate, got %v", i, op.Type)
		}
		if op.Table != "Accounts" {
			t.Errorf("Operation %d: expected table 'Accounts', got '%s'", i, op.Table)
		}
	}
}

func TestSQLDetector_DetectOperations_WithJoin(t *testing.T) {
	sql := `
CREATE PROCEDURE GetOrdersWithCustomer
    @CustomerID INT
AS
BEGIN
    SELECT o.OrderID, o.OrderDate, c.Name
    FROM Orders o
    INNER JOIN Customers c ON o.CustomerID = c.CustomerID
    WHERE c.CustomerID = @CustomerID
END
`
	program := parseSQL(t, sql)
	
	detector := NewSQLDetector(DetectorConfig{})
	ops, err := detector.DetectOperations(program.Statements[0])
	if err != nil {
		t.Fatalf("DetectOperations failed: %v", err)
	}
	
	if len(ops) != 1 {
		t.Fatalf("Expected 1 operation, got %d", len(ops))
	}
	
	op := ops[0]
	if op.Type != OpSelect {
		t.Errorf("Expected OpSelect, got %v", op.Type)
	}
	
	// Should have 3 fields from SELECT
	if len(op.Fields) != 3 {
		t.Errorf("Expected 3 fields, got %d", len(op.Fields))
	}
	
	// Key fields should include JOIN condition and WHERE
	if len(op.KeyFields) < 1 {
		t.Errorf("Expected at least 1 key field, got %d", len(op.KeyFields))
	}
}

func TestSQLDetector_DetectOperations_IfExists(t *testing.T) {
	sql := `
CREATE PROCEDURE CreateOrUpdateUser
    @UserID INT,
    @Name NVARCHAR(100)
AS
BEGIN
    IF EXISTS (SELECT 1 FROM Users WHERE ID = @UserID)
    BEGIN
        UPDATE Users SET Name = @Name WHERE ID = @UserID
    END
    ELSE
    BEGIN
        INSERT INTO Users (ID, Name) VALUES (@UserID, @Name)
    END
END
`
	program := parseSQL(t, sql)
	
	detector := NewSQLDetector(DetectorConfig{})
	ops, err := detector.DetectOperations(program.Statements[0])
	if err != nil {
		t.Fatalf("DetectOperations failed: %v", err)
	}
	
	// Should have 3 operations: SELECT (exists check), UPDATE, INSERT
	if len(ops) != 3 {
		t.Fatalf("Expected 3 operations, got %d", len(ops))
	}
	
	// First should be the EXISTS SELECT
	if ops[0].Type != OpSelect {
		t.Errorf("Expected first operation to be OpSelect, got %v", ops[0].Type)
	}
	
	// Second should be UPDATE
	if ops[1].Type != OpUpdate {
		t.Errorf("Expected second operation to be OpUpdate, got %v", ops[1].Type)
	}
	
	// Third should be INSERT
	if ops[2].Type != OpInsert {
		t.Errorf("Expected third operation to be OpInsert, got %v", ops[2].Type)
	}
}

func TestSQLDetector_DetectOperations_Output(t *testing.T) {
	sql := `
CREATE PROCEDURE CreateUser
    @Name NVARCHAR(100)
AS
BEGIN
    INSERT INTO Users (Name)
    OUTPUT INSERTED.ID, INSERTED.Name
    VALUES (@Name)
END
`
	program := parseSQL(t, sql)
	
	detector := NewSQLDetector(DetectorConfig{})
	ops, err := detector.DetectOperations(program.Statements[0])
	if err != nil {
		t.Fatalf("DetectOperations failed: %v", err)
	}
	
	if len(ops) != 1 {
		t.Fatalf("Expected 1 operation, got %d", len(ops))
	}
	
	op := ops[0]
	if op.Type != OpInsert {
		t.Errorf("Expected OpInsert, got %v", op.Type)
	}
	
	// Should have 2 output fields from OUTPUT clause
	if len(op.OutputFields) != 2 {
		t.Errorf("Expected 2 output fields, got %d", len(op.OutputFields))
	}
}

func TestSQLDetector_DetectOperations_VariableAssignment(t *testing.T) {
	sql := `
CREATE PROCEDURE GetUserName
    @UserID INT,
    @Name NVARCHAR(100) OUTPUT
AS
BEGIN
    SELECT @Name = Name
    FROM Users
    WHERE ID = @UserID
END
`
	program := parseSQL(t, sql)
	
	detector := NewSQLDetector(DetectorConfig{})
	ops, err := detector.DetectOperations(program.Statements[0])
	if err != nil {
		t.Fatalf("DetectOperations failed: %v", err)
	}
	
	if len(ops) != 1 {
		t.Fatalf("Expected 1 operation, got %d", len(ops))
	}
	
	op := ops[0]
	if len(op.Fields) < 1 {
		t.Fatal("Expected at least 1 field")
	}
	
	// The field should have a variable assignment
	found := false
	for _, f := range op.Fields {
		if f.Variable == "@Name" && f.IsAssigned {
			found = true
			break
		}
	}
	
	if !found {
		t.Error("Expected to find variable assignment to @Name")
	}
}

func TestSQLDetector_DetectModels(t *testing.T) {
	sql := `
CREATE PROCEDURE UserOperations
    @UserID INT
AS
BEGIN
    SELECT ID, Name, Email FROM Users WHERE ID = @UserID
    INSERT INTO Orders (UserID, Amount) VALUES (@UserID, 100)
END
`
	program := parseSQL(t, sql)
	
	detector := NewSQLDetector(DetectorConfig{})
	ops, err := detector.DetectOperations(program.Statements[0])
	if err != nil {
		t.Fatalf("DetectOperations failed: %v", err)
	}
	
	models, err := detector.DetectModels(ops)
	if err != nil {
		t.Fatalf("DetectModels failed: %v", err)
	}
	
	// Should have 2 models: Users and Orders
	if len(models) != 2 {
		t.Fatalf("Expected 2 models, got %d", len(models))
	}
	
	modelNames := make(map[string]bool)
	for _, m := range models {
		modelNames[m.Name] = true
	}
	
	if !modelNames["Users"] {
		t.Error("Expected 'Users' model")
	}
	if !modelNames["Orders"] {
		t.Error("Expected 'Orders' model")
	}
}

func TestSQLDetector_DetectRepositories(t *testing.T) {
	sql := `
CREATE PROCEDURE GetUser
    @UserID INT
AS
BEGIN
    SELECT ID, Name FROM Users WHERE ID = @UserID
END
`
	program := parseSQL(t, sql)
	
	detector := NewSQLDetector(DetectorConfig{})
	ops, err := detector.DetectOperations(program.Statements[0])
	if err != nil {
		t.Fatalf("DetectOperations failed: %v", err)
	}
	
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
	
	if repo.Entity != "Users" {
		t.Errorf("Expected entity 'Users', got '%s'", repo.Entity)
	}
	
	if len(repo.Methods) != 1 {
		t.Fatalf("Expected 1 method, got %d", len(repo.Methods))
	}
}

func TestSQLDetector_DetectOperations_Exec(t *testing.T) {
	sql := `
CREATE PROCEDURE ProcessOrder
    @OrderID INT
AS
BEGIN
    EXEC ValidateOrder @OrderID
    EXEC ShipOrder @OrderID = @OrderID
END
`
	program := parseSQL(t, sql)
	
	detector := NewSQLDetector(DetectorConfig{})
	ops, err := detector.DetectOperations(program.Statements[0])
	if err != nil {
		t.Fatalf("DetectOperations failed: %v", err)
	}
	
	// Should have 2 EXEC operations
	if len(ops) != 2 {
		t.Fatalf("Expected 2 operations, got %d", len(ops))
	}
	
	for i, op := range ops {
		if op.Type != OpExec {
			t.Errorf("Operation %d: expected OpExec, got %v", i, op.Type)
		}
	}
	
	if ops[0].CalledProcedure != "ValidateOrder" {
		t.Errorf("Expected 'ValidateOrder', got '%s'", ops[0].CalledProcedure)
	}
	
	if ops[1].CalledProcedure != "ShipOrder" {
		t.Errorf("Expected 'ShipOrder', got '%s'", ops[1].CalledProcedure)
	}
}

func TestSQLDetector_DetectOperations_TryCatch(t *testing.T) {
	sql := `
CREATE PROCEDURE SafeInsert
    @Name NVARCHAR(100)
AS
BEGIN
    BEGIN TRY
        INSERT INTO Users (Name) VALUES (@Name)
    END TRY
    BEGIN CATCH
        INSERT INTO ErrorLog (Message) VALUES (ERROR_MESSAGE())
    END CATCH
END
`
	program := parseSQL(t, sql)
	
	detector := NewSQLDetector(DetectorConfig{})
	ops, err := detector.DetectOperations(program.Statements[0])
	if err != nil {
		t.Fatalf("DetectOperations failed: %v", err)
	}
	
	// Should have 2 INSERT operations (one in TRY, one in CATCH)
	if len(ops) != 2 {
		t.Fatalf("Expected 2 operations, got %d", len(ops))
	}
	
	for i, op := range ops {
		if op.Type != OpInsert {
			t.Errorf("Operation %d: expected OpInsert, got %v", i, op.Type)
		}
	}
}

func TestSQLDetector_DetectOperations_UpdateWithFrom(t *testing.T) {
	sql := `
CREATE PROCEDURE UpdateOrderStatus
    @CustomerID INT,
    @NewStatus INT
AS
BEGIN
    UPDATE o
    SET o.Status = @NewStatus
    FROM Orders o
    INNER JOIN Customers c ON o.CustomerID = c.CustomerID
    WHERE c.CustomerID = @CustomerID
END
`
	program := parseSQL(t, sql)
	
	detector := NewSQLDetector(DetectorConfig{})
	ops, err := detector.DetectOperations(program.Statements[0])
	if err != nil {
		t.Fatalf("DetectOperations failed: %v", err)
	}
	
	if len(ops) != 1 {
		t.Fatalf("Expected 1 operation, got %d", len(ops))
	}
	
	op := ops[0]
	if op.Type != OpUpdate {
		t.Errorf("Expected OpUpdate, got %v", op.Type)
	}
	
	if op.Alias != "o" {
		t.Errorf("Expected alias 'o', got '%s'", op.Alias)
	}
}
