package transpiler

import (
	"strings"
	"testing"
)

func TestTranspileWithDML_Select(t *testing.T) {
	sql := `
CREATE PROCEDURE dbo.GetUserById
    @UserId INT
AS
BEGIN
    SELECT Id, Email, FirstName, LastName
    FROM Users
    WHERE Id = @UserId
END
`
	config := DefaultDMLConfig()
	config.Backend = BackendSQL
	config.SQLDialect = "postgres"
	config.StoreVar = "r.db"

	result, err := TranspileWithDML(sql, "main", config)
	if err != nil {
		t.Fatalf("TranspileWithDML failed: %v", err)
	}

	// Check that it generates QueryRowContext for single-row SELECT
	if !strings.Contains(result, "QueryRowContext") {
		t.Errorf("Expected QueryRowContext for single-row SELECT, got:\n%s", result)
	}

	// Check that it uses correct placeholder
	if !strings.Contains(result, "$1") {
		t.Errorf("Expected PostgreSQL placeholder $1, got:\n%s", result)
	}

	t.Logf("Generated code:\n%s", result)
}

func TestTranspileWithDML_Insert(t *testing.T) {
	sql := `
CREATE PROCEDURE dbo.CreateUser
    @Email VARCHAR(255),
    @FirstName VARCHAR(100),
    @LastName VARCHAR(100)
AS
BEGIN
    INSERT INTO Users (Email, FirstName, LastName)
    VALUES (@Email, @FirstName, @LastName)
END
`
	config := DefaultDMLConfig()
	config.Backend = BackendSQL
	config.SQLDialect = "postgres"

	result, err := TranspileWithDML(sql, "main", config)
	if err != nil {
		t.Fatalf("TranspileWithDML failed: %v", err)
	}

	// Check that it generates ExecContext for INSERT
	if !strings.Contains(result, "ExecContext") {
		t.Errorf("Expected ExecContext for INSERT, got:\n%s", result)
	}

	t.Logf("Generated code:\n%s", result)
}

func TestTranspileWithDML_Update(t *testing.T) {
	sql := `
CREATE PROCEDURE dbo.UpdateUser
    @UserId INT,
    @FirstName VARCHAR(100),
    @LastName VARCHAR(100)
AS
BEGIN
    UPDATE Users 
    SET FirstName = @FirstName, LastName = @LastName
    WHERE Id = @UserId
END
`
	config := DefaultDMLConfig()
	config.Backend = BackendSQL
	config.SQLDialect = "mysql"

	result, err := TranspileWithDML(sql, "main", config)
	if err != nil {
		t.Fatalf("TranspileWithDML failed: %v", err)
	}

	// Check that it generates ExecContext for UPDATE
	if !strings.Contains(result, "ExecContext") {
		t.Errorf("Expected ExecContext for UPDATE, got:\n%s", result)
	}

	// Check MySQL placeholder
	if !strings.Contains(result, "?") {
		t.Errorf("Expected MySQL placeholder ?, got:\n%s", result)
	}

	t.Logf("Generated code:\n%s", result)
}

func TestTranspileWithDML_Delete(t *testing.T) {
	sql := `
CREATE PROCEDURE dbo.DeleteUser
    @UserId INT
AS
BEGIN
    DELETE FROM Users
    WHERE Id = @UserId
END
`
	config := DefaultDMLConfig()
	config.Backend = BackendSQL
	config.SQLDialect = "postgres"

	result, err := TranspileWithDML(sql, "main", config)
	if err != nil {
		t.Fatalf("TranspileWithDML failed: %v", err)
	}

	// Check that it generates ExecContext for DELETE
	if !strings.Contains(result, "ExecContext") {
		t.Errorf("Expected ExecContext for DELETE, got:\n%s", result)
	}

	t.Logf("Generated code:\n%s", result)
}

func TestTranspileWithDML_SelectIntoVar(t *testing.T) {
	sql := `
CREATE PROCEDURE dbo.GetUserEmail
    @UserId INT,
    @Email VARCHAR(255) OUTPUT
AS
BEGIN
    SELECT @Email = Email
    FROM Users
    WHERE Id = @UserId
END
`
	config := DefaultDMLConfig()
	config.Backend = BackendSQL
	config.SQLDialect = "postgres"

	result, err := TranspileWithDML(sql, "main", config)
	if err != nil {
		t.Fatalf("TranspileWithDML failed: %v", err)
	}

	// Check that it generates proper variable assignment
	if !strings.Contains(result, "Scan(&email") {
		t.Errorf("Expected Scan with &email, got:\n%s", result)
	}

	t.Logf("Generated code:\n%s", result)
}

func TestTranspileWithDML_Exec(t *testing.T) {
	sql := `
CREATE PROCEDURE dbo.ProcessOrder
    @OrderId INT
AS
BEGIN
    EXEC dbo.ValidateOrder @OrderId
    EXEC dbo.CalculateTax @OrderId
END
`
	config := DefaultDMLConfig()
	config.Backend = BackendSQL

	result, err := TranspileWithDML(sql, "main", config)
	if err != nil {
		t.Fatalf("TranspileWithDML failed: %v", err)
	}

	// Check that EXEC becomes Go function call
	if !strings.Contains(result, "ValidateOrder(") {
		t.Errorf("Expected ValidateOrder function call, got:\n%s", result)
	}
	if !strings.Contains(result, "CalculateTax(") {
		t.Errorf("Expected CalculateTax function call, got:\n%s", result)
	}

	t.Logf("Generated code:\n%s", result)
}

func TestTranspileWithDML_gRPCBackend(t *testing.T) {
	sql := `
CREATE PROCEDURE dbo.GetUserById
    @UserId INT
AS
BEGIN
    SELECT Id, Email, FirstName, LastName
    FROM Users
    WHERE Id = @UserId
END
`
	config := DefaultDMLConfig()
	config.Backend = BackendGRPC
	config.StoreVar = "r.client"
	config.ProtoPackage = "pb"

	result, err := TranspileWithDML(sql, "main", config)
	if err != nil {
		t.Fatalf("TranspileWithDML failed: %v", err)
	}

	// Check that it generates gRPC call
	if !strings.Contains(result, "gRPC call") {
		t.Errorf("Expected gRPC call comment, got:\n%s", result)
	}
	if !strings.Contains(result, "r.client.") {
		t.Errorf("Expected r.client call, got:\n%s", result)
	}

	t.Logf("Generated code:\n%s", result)
}

func TestTranspileWithDML_MixedProceduralAndDML(t *testing.T) {
	sql := `
CREATE PROCEDURE dbo.GetUserOrDefault
    @UserId INT,
    @DefaultEmail VARCHAR(255)
AS
BEGIN
    DECLARE @Email VARCHAR(255)
    
    SELECT @Email = Email
    FROM Users
    WHERE Id = @UserId
    
    IF @Email IS NULL
        SET @Email = @DefaultEmail
    
    RETURN
END
`
	config := DefaultDMLConfig()
	config.Backend = BackendSQL
	config.SQLDialect = "postgres"

	result, err := TranspileWithDML(sql, "main", config)
	if err != nil {
		t.Fatalf("TranspileWithDML failed: %v", err)
	}

	// Check for variable declaration
	if !strings.Contains(result, "var email string") {
		t.Errorf("Expected var email declaration, got:\n%s", result)
	}

	// Check for SELECT with Scan
	if !strings.Contains(result, "Scan(&email") {
		t.Errorf("Expected Scan with variable, got:\n%s", result)
	}

	// Check for IF statement (IS NULL becomes == "")
	if !strings.Contains(result, "if") {
		t.Errorf("Expected IF statement, got:\n%s", result)
	}
	
	// Check for assignment in IF
	if !strings.Contains(result, "defaultEmail") {
		t.Errorf("Expected assignment using defaultEmail, got:\n%s", result)
	}

	t.Logf("Generated code:\n%s", result)
}

func TestTranspileWithDML_SQLDialects(t *testing.T) {
	sql := `
CREATE PROCEDURE dbo.GetUserById
    @UserId INT
AS
BEGIN
    SELECT Id, Email FROM Users WHERE Id = @UserId
END
`
	tests := []struct {
		dialect     string
		placeholder string
	}{
		{"postgres", "$1"},
		{"mysql", "?"},
		{"sqlite", "?"},
		{"sqlserver", "@p1"},
	}

	for _, tt := range tests {
		t.Run(tt.dialect, func(t *testing.T) {
			config := DefaultDMLConfig()
			config.SQLDialect = tt.dialect

			result, err := TranspileWithDML(sql, "main", config)
			if err != nil {
				t.Fatalf("TranspileWithDML failed: %v", err)
			}

			if !strings.Contains(result, tt.placeholder) {
				t.Errorf("Expected %s placeholder for %s dialect, got:\n%s", tt.placeholder, tt.dialect, result)
			}
		})
	}
}

func TestTranspileWithDML_Transaction(t *testing.T) {
	sql := `
CREATE PROCEDURE TransferFunds
    @FromAccountID INT,
    @ToAccountID INT,
    @Amount DECIMAL(18,2)
AS
BEGIN
    BEGIN TRANSACTION;
    
    UPDATE Accounts SET Balance = Balance - @Amount WHERE ID = @FromAccountID;
    UPDATE Accounts SET Balance = Balance + @Amount WHERE ID = @ToAccountID;
    
    COMMIT TRANSACTION;
END
`

	config := DefaultDMLConfig()
	config.SQLDialect = "postgres"

	result, err := TranspileWithDML(sql, "banking", config)
	if err != nil {
		t.Fatalf("TranspileWithDML failed: %v", err)
	}

	t.Logf("Generated code:\n%s", result)

	// Should have transaction handling
	if !strings.Contains(result, "BeginTx(ctx") {
		t.Error("Expected BeginTx for BEGIN TRANSACTION")
	}
	if !strings.Contains(result, "tx.Commit()") {
		t.Error("Expected tx.Commit() for COMMIT TRANSACTION")
	}
	// DML inside transaction should use tx
	if !strings.Contains(result, "tx.ExecContext(ctx") {
		t.Error("Expected tx.ExecContext inside transaction")
	}
}

func TestTranspileWithDML_Rollback(t *testing.T) {
	sql := `
CREATE PROCEDURE SafeTransfer
    @FromID INT,
    @Amount DECIMAL(18,2)
AS
BEGIN
    DECLARE @Balance DECIMAL(18,2);
    
    BEGIN TRANSACTION;
    
    SELECT @Balance = Balance FROM Accounts WHERE ID = @FromID;
    
    IF @Balance < @Amount
    BEGIN
        ROLLBACK TRANSACTION;
        RETURN;
    END
    
    UPDATE Accounts SET Balance = Balance - @Amount WHERE ID = @FromID;
    
    COMMIT TRANSACTION;
END
`

	config := DefaultDMLConfig()
	config.SQLDialect = "postgres"

	result, err := TranspileWithDML(sql, "banking", config)
	if err != nil {
		t.Fatalf("TranspileWithDML failed: %v", err)
	}

	t.Logf("Generated code:\n%s", result)

	// Should have rollback
	if !strings.Contains(result, "tx.Rollback()") {
		t.Error("Expected tx.Rollback() for ROLLBACK TRANSACTION")
	}
}

func TestTranspileWithDML_ScanTargets(t *testing.T) {
	sql := `
CREATE PROCEDURE GetUserDetails
    @UserID INT
AS
BEGIN
    SELECT ID, Username, Email, CreatedAt, IsActive
    FROM Users
    WHERE ID = @UserID;
END
`

	config := DefaultDMLConfig()
	config.SQLDialect = "postgres"

	result, err := TranspileWithDML(sql, "users", config)
	if err != nil {
		t.Fatalf("TranspileWithDML failed: %v", err)
	}

	t.Logf("Generated code:\n%s", result)

	// Should have variable declarations for scan targets
	if !strings.Contains(result, "var id ") {
		t.Error("Expected 'var id' declaration for ID column")
	}
	if !strings.Contains(result, "var username ") {
		t.Error("Expected 'var username' declaration")
	}
	if !strings.Contains(result, "var email ") {
		t.Error("Expected 'var email' declaration")
	}
	// Should have Scan with addresses
	if !strings.Contains(result, "&id") {
		t.Error("Expected &id in Scan")
	}
	if !strings.Contains(result, "&email") {
		t.Error("Expected &email in Scan")
	}
}

func TestTranspileWithDML_MultiRowSelect(t *testing.T) {
	sql := `
CREATE PROCEDURE ListUsersByStatus
    @IsActive BIT
AS
BEGIN
    SELECT ID, Username, Email
    FROM Users
    WHERE IsActive = @IsActive;
END
`

	config := DefaultDMLConfig()
	config.SQLDialect = "postgres"

	result, err := TranspileWithDML(sql, "users", config)
	if err != nil {
		t.Fatalf("TranspileWithDML failed: %v", err)
	}

	t.Logf("Generated code:\n%s", result)

	// Multi-row SELECT should use QueryContext and rows iteration
	if !strings.Contains(result, "QueryContext") {
		t.Error("Expected QueryContext for multi-row SELECT")
	}
	if !strings.Contains(result, "rows.Next()") {
		t.Error("Expected rows.Next() for multi-row iteration")
	}
	if !strings.Contains(result, "rows.Close()") {
		t.Error("Expected rows.Close() for cleanup")
	}
}

func TestTranspileWithDML_Cursor(t *testing.T) {
	sql := `
CREATE PROCEDURE ProcessAllUsers
AS
BEGIN
    DECLARE @UserID INT
    DECLARE @Email VARCHAR(255)
    
    DECLARE user_cursor CURSOR FOR
        SELECT ID, Email FROM Users WHERE IsActive = 1
    
    OPEN user_cursor
    FETCH NEXT FROM user_cursor INTO @UserID, @Email
    
    WHILE @@FETCH_STATUS = 0
    BEGIN
        PRINT @Email
        FETCH NEXT FROM user_cursor INTO @UserID, @Email
    END
    
    CLOSE user_cursor
    DEALLOCATE user_cursor
END
`

	config := DefaultDMLConfig()
	config.SQLDialect = "postgres"

	result, err := TranspileWithDML(sql, "users", config)
	if err != nil {
		t.Fatalf("TranspileWithDML failed: %v", err)
	}

	t.Logf("Generated code:\n%s", result)

	// Should have QueryContext for cursor
	if !strings.Contains(result, "QueryContext") {
		t.Error("Expected QueryContext for cursor OPEN")
	}
	// Should have rows.Next() loop
	if !strings.Contains(result, "userCursorRows.Next()") {
		t.Error("Expected userCursorRows.Next() for WHILE @@FETCH_STATUS loop")
	}
	// Should have Scan with cursor variables
	if !strings.Contains(result, "Scan(&userId, &email)") {
		t.Error("Expected Scan(&userId, &email) for FETCH INTO")
	}
	// Should have defer rows.Close()
	if !strings.Contains(result, "defer userCursorRows.Close()") {
		t.Error("Expected defer userCursorRows.Close()")
	}
}

func TestTranspileWithDML_CursorWithProcessing(t *testing.T) {
	sql := `
CREATE PROCEDURE UpdateAllPrices
    @Multiplier DECIMAL(5,2)
AS
BEGIN
    DECLARE @ProductID INT
    DECLARE @CurrentPrice DECIMAL(18,2)
    
    DECLARE price_cursor CURSOR FOR
        SELECT ID, Price FROM Products
    
    OPEN price_cursor
    FETCH NEXT FROM price_cursor INTO @ProductID, @CurrentPrice
    
    WHILE @@FETCH_STATUS = 0
    BEGIN
        UPDATE Products 
        SET Price = @CurrentPrice * @Multiplier
        WHERE ID = @ProductID
        
        FETCH NEXT FROM price_cursor INTO @ProductID, @CurrentPrice
    END
    
    CLOSE price_cursor
    DEALLOCATE price_cursor
END
`

	config := DefaultDMLConfig()
	config.SQLDialect = "postgres"

	result, err := TranspileWithDML(sql, "products", config)
	if err != nil {
		t.Fatalf("TranspileWithDML failed: %v", err)
	}

	t.Logf("Generated code:\n%s", result)

	// Should have UPDATE inside the loop
	if !strings.Contains(result, "ExecContext") {
		t.Error("Expected ExecContext for UPDATE inside cursor loop")
	}
	// Should have rows iteration
	if !strings.Contains(result, ".Next()") {
		t.Error("Expected .Next() for cursor iteration")
	}
}

// === Verb Detection Tests ===

func TestTranspileWithDML_VerbDetection_ApprovalStatus(t *testing.T) {
	sql := `
CREATE PROCEDURE dbo.ApproveOrder
    @OrderId INT
AS
BEGIN
    UPDATE Orders SET ApprovalStatus = 'Approved', ApprovedAt = GETDATE()
    WHERE OrderId = @OrderId
END
`
	config := DefaultDMLConfig()
	config.Backend = BackendGRPC
	config.StoreVar = "r.client"
	config.ProtoPackage = "orderpb"

	result, err := TranspileWithDML(sql, "main", config)
	if err != nil {
		t.Fatalf("TranspileWithDML failed: %v", err)
	}

	// Should detect "Approve" verb from SET ApprovalStatus = 'Approved'
	if !strings.Contains(result, "ApproveOrder") {
		t.Errorf("Expected ApproveOrder method (verb detection), got:\n%s", result)
	}

	t.Logf("Generated code:\n%s", result)
}

func TestTranspileWithDML_VerbDetection_Certification(t *testing.T) {
	sql := `
CREATE PROCEDURE dbo.CertifyDocument
    @DocumentId INT,
    @CertifierId INT
AS
BEGIN
    UPDATE Documents SET CertificationStatus = 'Certified', CertifiedBy = @CertifierId
    WHERE DocumentId = @DocumentId
END
`
	config := DefaultDMLConfig()
	config.Backend = BackendGRPC
	config.StoreVar = "r.client"
	config.ProtoPackage = "docpb"

	result, err := TranspileWithDML(sql, "main", config)
	if err != nil {
		t.Fatalf("TranspileWithDML failed: %v", err)
	}

	// Should detect "Certify" verb
	if !strings.Contains(result, "Certif") {
		t.Errorf("Expected Certify-related method, got:\n%s", result)
	}

	t.Logf("Generated code:\n%s", result)
}

func TestTranspileWithDML_GRPCMappings_ExplicitMapping(t *testing.T) {
	sql := `
CREATE PROCEDURE dbo.ProcessOrder
    @OrderId INT
AS
BEGIN
    EXEC usp_ValidateOrder @OrderId
END
`
	config := DefaultDMLConfig()
	config.Backend = BackendGRPC
	config.StoreVar = "orderClient"
	config.ProtoPackage = "orderpb"
	config.GRPCMappings = map[string]string{
		"usp_ValidateOrder": "OrderService.ValidateOrder",
	}

	result, err := TranspileWithDML(sql, "main", config)
	if err != nil {
		t.Fatalf("TranspileWithDML failed: %v", err)
	}

	// Should use explicit mapping
	if !strings.Contains(result, "ValidateOrder") {
		t.Errorf("Expected ValidateOrder from explicit mapping, got:\n%s", result)
	}
	if !strings.Contains(result, "gRPC") {
		t.Errorf("Expected gRPC comment, got:\n%s", result)
	}

	t.Logf("Generated code:\n%s", result)
}

func TestTranspileWithDML_TableToService_Mapping(t *testing.T) {
	sql := `
CREATE PROCEDURE dbo.GetProductInfo
    @ProductId INT
AS
BEGIN
    SELECT ProductId, Name, Price FROM Products WHERE ProductId = @ProductId
END
`
	config := DefaultDMLConfig()
	config.Backend = BackendGRPC
	config.ProtoPackage = "catalogpb"
	config.TableToService = map[string]string{
		"Products": "CatalogService",
	}
	config.TableToClient = map[string]string{
		"Products": "catalogClient",
	}

	result, err := TranspileWithDML(sql, "main", config)
	if err != nil {
		t.Fatalf("TranspileWithDML failed: %v", err)
	}

	// Should use catalogClient for Products table
	if !strings.Contains(result, "catalogClient") {
		t.Errorf("Expected catalogClient from TableToClient mapping, got:\n%s", result)
	}

	t.Logf("Generated code:\n%s", result)
}

func TestExtractActionVerb(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		// Approval verbs
		{"ApprovalStatus", "Approve"},
		{"ApprovedAt", "Approve"},
		{"ApprovedBy", "Approve"},
		{"IsApproved", "Approve"},
		{"RejectionReason", "Reject"},
		{"RejectedDate", "Reject"},
		{"DenialCode", "Deny"},

		// Certification verbs
		{"CertificationDate", "Certify"},
		{"CertifiedBy", "Certify"},
		{"AttestationId", "Attest"},

		// Lifecycle verbs
		{"SuspendedUntil", "Suspend"},
		{"SuspensionReason", "Suspend"},
		{"CancelledBy", "Cancel"},
		{"CancellationDate", "Cancel"},
		{"TerminatedAt", "Terminate"},
		{"TerminationReason", "Terminate"},
		{"CompletedDate", "Complete"},
		{"CompletionStatus", "Complete"},
		{"ActivatedAt", "Activate"},
		{"DeactivatedBy", "Deactivate"},

		// Communication verbs
		{"NotificationSent", "Notify"},
		{"NotifiedAt", "Notify"},
		{"AlertTriggered", "Alert"},

		// Validation verbs
		{"VerificationCode", "Verify"},
		{"ValidatedBy", "Validate"},
		{"ValidationStatus", "Validate"},

		// Calculation verbs
		{"CalculatedAmount", "Calculate"},
		{"ComputedTotal", "Compute"},
		{"EstimatedValue", "Estimate"},

		// Signing verbs
		{"SignedAt", "Sign"},
		{"SignatureId", "Sign"},
		{"CountersignedBy", "Countersign"},

		// Review verbs
		{"ReviewedBy", "Review"},
		{"AssessmentScore", "Assess"},
		{"AuditTrailId", "Audit"},

		// Escalation verbs
		{"EscalatedTo", "Escalate"},
		{"EscalationLevel", "Escalate"},
		{"DelegatedBy", "Delegate"},

		// Transfer verbs
		{"TransferredAt", "Transfer"},
		{"SubmittedDate", "Submit"},
		{"SubmissionId", "Submit"},

		// Negative cases - no verb detected
		{"ProductName", ""},
		{"UserId", ""},
		{"CreatedAt", ""},          // Create is CRUD, not action verb
		{"UpdatedBy", ""},          // Update is CRUD, not action verb
		{"OrderTotal", ""},
		{"CustomerEmail", ""},
		{"Price", ""},
		{"Quantity", ""},
		{"Description", ""},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := extractActionVerb(tc.input)
			if result != tc.expected {
				t.Errorf("extractActionVerb(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestInferProtoPackage(t *testing.T) {
	tests := []struct {
		serviceName string
		expected    string
	}{
		{"CatalogService", "catalogpb"},
		{"OrderService", "orderpb"},
		{"UserAccountService", "useraccountpb"},
		{"InventorySvc", "inventorypb"},
		{"PaymentAPI", "paymentpb"},
		{"ShippingApi", "shippingpb"},
		{"Catalog", "catalogpb"},
		{"Orders", "orderspb"},
	}

	for _, tc := range tests {
		t.Run(tc.serviceName, func(t *testing.T) {
			result := inferProtoPackage(tc.serviceName)
			if result != tc.expected {
				t.Errorf("inferProtoPackage(%q) = %q, want %q", tc.serviceName, result, tc.expected)
			}
		})
	}
}

func TestTranspileWithDML_ServiceToPackageInference(t *testing.T) {
	sql := `
CREATE PROCEDURE dbo.GetProductDetails
    @ProductId INT
AS
BEGIN
    SELECT ProductId, Name, Price FROM Products WHERE ProductId = @ProductId
END
`
	config := DefaultDMLConfig()
	config.Backend = BackendGRPC
	// No explicit ProtoPackage - should infer from service name
	config.TableToService = map[string]string{
		"Products": "CatalogService",
	}

	result, err := TranspileWithDML(sql, "main", config)
	if err != nil {
		t.Fatalf("TranspileWithDML failed: %v", err)
	}

	// Should infer catalogpb from CatalogService
	if !strings.Contains(result, "catalogpb") {
		t.Errorf("Expected inferred catalogpb package, got:\n%s", result)
	}

	t.Logf("Generated code:\n%s", result)
}

func TestTranspileWithDML_VerbDetection_Suspend(t *testing.T) {
	sql := `
CREATE PROCEDURE dbo.SuspendAccount
    @AccountId INT,
    @Reason VARCHAR(500)
AS
BEGIN
    UPDATE Accounts SET SuspendedAt = GETDATE(), SuspensionReason = @Reason
    WHERE AccountId = @AccountId
END
`
	config := DefaultDMLConfig()
	config.Backend = BackendGRPC
	config.StoreVar = "r.client"
	config.ProtoPackage = "accountpb"

	result, err := TranspileWithDML(sql, "main", config)
	if err != nil {
		t.Fatalf("TranspileWithDML failed: %v", err)
	}

	// Should detect "Suspend" verb from SuspendedAt column
	if !strings.Contains(result, "SuspendAccount") {
		t.Errorf("Expected SuspendAccount method (verb detection), got:\n%s", result)
	}

	t.Logf("Generated code:\n%s", result)
}

func TestTranspileWithDML_VerbDetection_Escalate(t *testing.T) {
	sql := `
CREATE PROCEDURE dbo.EscalateTicket
    @TicketId INT,
    @EscalatedTo INT
AS
BEGIN
    UPDATE Tickets SET EscalationLevel = EscalationLevel + 1, EscalatedTo = @EscalatedTo
    WHERE TicketId = @TicketId
END
`
	config := DefaultDMLConfig()
	config.Backend = BackendGRPC
	config.StoreVar = "r.client"
	config.ProtoPackage = "ticketpb"

	result, err := TranspileWithDML(sql, "main", config)
	if err != nil {
		t.Fatalf("TranspileWithDML failed: %v", err)
	}

	// Should detect "Escalate" verb from EscalationLevel or EscalatedTo
	if !strings.Contains(result, "Escalate") {
		t.Errorf("Expected Escalate-related method, got:\n%s", result)
	}

	t.Logf("Generated code:\n%s", result)
}

func TestTranspileWithDML_VerbDetection_Notify(t *testing.T) {
	sql := `
CREATE PROCEDURE dbo.SendNotification
    @UserId INT,
    @Message VARCHAR(1000)
AS
BEGIN
    INSERT INTO Notifications (UserId, Message, NotificationSent, NotifiedAt)
    VALUES (@UserId, @Message, 1, GETDATE())
END
`
	config := DefaultDMLConfig()
	config.Backend = BackendGRPC
	config.StoreVar = "r.client"
	config.ProtoPackage = "notifypb"

	result, err := TranspileWithDML(sql, "main", config)
	if err != nil {
		t.Fatalf("TranspileWithDML failed: %v", err)
	}

	// Should detect "Notify" verb from NotificationSent or NotifiedAt columns
	if !strings.Contains(result, "Notify") {
		t.Errorf("Expected Notify-related method, got:\n%s", result)
	}

	t.Logf("Generated code:\n%s", result)
}
