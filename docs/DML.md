# DML Mode Documentation

DML mode enables tgpiler to transpile data manipulation operations (SELECT, INSERT, UPDATE, DELETE) from T-SQL stored procedures to Go code targeting various backends.

## Enabling DML Mode

```bash
# Command line
tgpiler --dml input.sql -o output.go

# API
import "github.com/ha1tch/tgpiler/transpiler"

config := transpiler.DefaultDMLConfig()
config.Backend = transpiler.BackendSQL
config.SQLDialect = "postgres"

result, err := transpiler.TranspileWithDML(source, "main", config)
```

## Backend Types

tgpiler supports four backend types for generated code:

| Backend | Description | Use Case |
|---------|-------------|----------|
| `BackendSQL` | Standard `database/sql` calls | Production database access |
| `BackendGRPC` | gRPC client calls | Microservices architecture |
| `BackendMock` | Mock store calls | Unit testing |
| `BackendInline` | Inline SQL strings | Migration scaffolding |

### SQL Backend (Default)

Generates idiomatic Go code using the `database/sql` package:

```go
config := transpiler.DMLConfig{
    Backend:    transpiler.BackendSQL,
    SQLDialect: "postgres",  // or "mysql", "sqlite", "sqlserver"
    StoreVar:   "r.db",      // database connection variable
}
```

### gRPC Backend

Generates gRPC client calls. See [GRPC.md](GRPC.md) for complete documentation.

```go
config := transpiler.DMLConfig{
    Backend:      transpiler.BackendGRPC,
    ProtoPackage: "catalog.v1",
    StoreVar:     "r.client",
}
```

### Mock Backend

Generates mock store calls for testing:

```go
config := transpiler.DMLConfig{
    Backend:  transpiler.BackendMock,
    StoreVar: "store",
}
```

### Inline Backend

Generates SQL strings without execution code, useful for migration:

```go
config := transpiler.DMLConfig{
    Backend: transpiler.BackendInline,
}
```

## SQL Dialects

tgpiler adapts generated SQL to the target dialect:

| Dialect | Placeholder | Features |
|---------|-------------|----------|
| `postgres` | `$1, $2, ...` | RETURNING, ON CONFLICT, USING |
| `mysql` | `?, ?, ...` | LAST_INSERT_ID(), ON DUPLICATE KEY |
| `sqlite` | `?, ?, ...` | Basic SQL, last_insert_rowid() |
| `sqlserver` | `@p1, @p2, ...` | OUTPUT, MERGE (native) |

### Dialect-Specific Transformations

**Parameter Placeholders:**
```sql
-- T-SQL input
WHERE CustomerID = @CustomerID AND Status = @Status

-- PostgreSQL output
WHERE CustomerID = $1 AND Status = $2

-- MySQL output
WHERE CustomerID = ? AND Status = ?
```

**INSERT with Identity:**
```sql
-- T-SQL input
INSERT INTO Orders (CustomerID) VALUES (@CustomerID)
SET @OrderID = SCOPE_IDENTITY()

-- PostgreSQL output (uses RETURNING)
INSERT INTO Orders (CustomerID) VALUES ($1) RETURNING OrderID

-- MySQL output (uses LAST_INSERT_ID)
INSERT INTO Orders (CustomerID) VALUES (?)
-- followed by: SELECT LAST_INSERT_ID()
```

**UPSERT/MERGE:**
```sql
-- T-SQL input
MERGE INTO Products AS target
USING (SELECT @SKU AS SKU, @Name AS Name) AS source
ON target.SKU = source.SKU
WHEN MATCHED THEN UPDATE SET Name = source.Name
WHEN NOT MATCHED THEN INSERT (SKU, Name) VALUES (source.SKU, source.Name);

-- PostgreSQL output
INSERT INTO Products (SKU, Name) VALUES ($1, $2)
ON CONFLICT (SKU) DO UPDATE SET Name = EXCLUDED.Name

-- MySQL output
INSERT INTO Products (SKU, Name) VALUES (?, ?)
ON DUPLICATE KEY UPDATE Name = VALUES(Name)
```

## SELECT Statements

### Basic SELECT

**T-SQL:**
```sql
SELECT OrderID, CustomerName, Amount
FROM Orders
WHERE Status = @Status AND Amount > @MinAmount
ORDER BY Amount DESC
```

**Generated Go:**
```go
rows, err := r.db.QueryContext(ctx, 
    "SELECT OrderID, CustomerName, Amount FROM Orders WHERE Status = $1 AND Amount > $2 ORDER BY Amount DESC",
    status, minAmount)
if err != nil {
    return err
}
defer rows.Close()

for rows.Next() {
    var orderID int64
    var customerName string
    var amount decimal.Decimal
    if err := rows.Scan(&orderID, &customerName, &amount); err != nil {
        return err
    }
    // Process row...
}
```

### SELECT INTO Variables

**T-SQL:**
```sql
SELECT @OrderCount = COUNT(*), @TotalAmount = SUM(Amount)
FROM Orders
WHERE CustomerID = @CustomerID
```

**Generated Go:**
```go
row := r.db.QueryRowContext(ctx,
    "SELECT COUNT(*), SUM(Amount) FROM Orders WHERE CustomerID = $1",
    customerID)
if err := row.Scan(&orderCount, &totalAmount); err != nil {
    return err
}
```

### SELECT with TOP

**T-SQL:**
```sql
SELECT TOP 10 ProductName, Price
FROM Products
ORDER BY SalesCount DESC
```

**Generated Go (PostgreSQL):**
```go
rows, err := r.db.QueryContext(ctx,
    "SELECT ProductName, Price FROM Products ORDER BY SalesCount DESC LIMIT 10")
```

### SELECT with JOINs

**T-SQL:**
```sql
SELECT o.OrderID, c.CustomerName, o.Amount
FROM Orders o
INNER JOIN Customers c ON o.CustomerID = c.CustomerID
LEFT JOIN OrderDetails d ON o.OrderID = d.OrderID
WHERE o.Status = @Status
```

**Generated Go:**
```go
rows, err := r.db.QueryContext(ctx,
    `SELECT o.OrderID, c.CustomerName, o.Amount 
     FROM Orders AS o 
     INNER JOIN Customers AS c ON o.CustomerID = c.CustomerID 
     LEFT JOIN OrderDetails AS d ON o.OrderID = d.OrderID 
     WHERE o.Status = $1`,
    status)
```

### SELECT with Subqueries

**T-SQL:**
```sql
SELECT CustomerName
FROM Customers
WHERE CustomerID IN (
    SELECT CustomerID FROM Orders WHERE Amount > 1000
)
```

**Generated Go:**
```go
rows, err := r.db.QueryContext(ctx,
    `SELECT CustomerName FROM Customers 
     WHERE CustomerID IN (SELECT CustomerID FROM Orders WHERE Amount > 1000)`)
```

### Common Table Expressions (CTEs)

**T-SQL:**
```sql
WITH TopCustomers AS (
    SELECT CustomerID, SUM(Amount) AS TotalSpent
    FROM Orders
    GROUP BY CustomerID
    HAVING SUM(Amount) > 10000
)
SELECT c.CustomerName, tc.TotalSpent
FROM Customers c
JOIN TopCustomers tc ON c.CustomerID = tc.CustomerID
ORDER BY tc.TotalSpent DESC
```

**Generated Go:**
```go
rows, err := r.db.QueryContext(ctx,
    `WITH TopCustomers AS (
        SELECT CustomerID, SUM(Amount) AS TotalSpent
        FROM Orders
        GROUP BY CustomerID
        HAVING SUM(Amount) > 10000
    )
    SELECT c.CustomerName, tc.TotalSpent
    FROM Customers AS c
    INNER JOIN TopCustomers AS tc ON c.CustomerID = tc.CustomerID
    ORDER BY tc.TotalSpent DESC`)
```

### Recursive CTEs

**T-SQL:**
```sql
WITH CategoryHierarchy AS (
    -- Anchor: root categories
    SELECT CategoryID, Name, ParentID, 0 AS Level
    FROM Categories
    WHERE ParentID IS NULL
    
    UNION ALL
    
    -- Recursive: child categories
    SELECT c.CategoryID, c.Name, c.ParentID, ch.Level + 1
    FROM Categories c
    JOIN CategoryHierarchy ch ON c.ParentID = ch.CategoryID
)
SELECT * FROM CategoryHierarchy
```

**Generated Go:**
```go
rows, err := r.db.QueryContext(ctx,
    `WITH RECURSIVE CategoryHierarchy AS (
        SELECT CategoryID, Name, ParentID, 0 AS Level
        FROM Categories WHERE ParentID IS NULL
        UNION ALL
        SELECT c.CategoryID, c.Name, c.ParentID, ch.Level + 1
        FROM Categories AS c
        INNER JOIN CategoryHierarchy AS ch ON c.ParentID = ch.CategoryID
    )
    SELECT * FROM CategoryHierarchy`)
```

## INSERT Statements

### Basic INSERT

**T-SQL:**
```sql
INSERT INTO Orders (CustomerID, OrderDate, Amount, Status)
VALUES (@CustomerID, GETDATE(), @Amount, 'Pending')
```

**Generated Go:**
```go
result, err := r.db.ExecContext(ctx,
    "INSERT INTO Orders (CustomerID, OrderDate, Amount, Status) VALUES ($1, NOW(), $2, 'Pending')",
    customerID, amount)
if err != nil {
    return err
}
rowsAffected, _ := result.RowsAffected()
```

### INSERT with OUTPUT (RETURNING)

**T-SQL:**
```sql
INSERT INTO Orders (CustomerID, Amount)
OUTPUT INSERTED.OrderID, INSERTED.CreatedAt
VALUES (@CustomerID, @Amount)
```

**Generated Go (PostgreSQL):**
```go
var orderID int64
var createdAt time.Time
err := r.db.QueryRowContext(ctx,
    "INSERT INTO Orders (CustomerID, Amount) VALUES ($1, $2) RETURNING OrderID, CreatedAt",
    customerID, amount).Scan(&orderID, &createdAt)
```

### INSERT ... SELECT

**T-SQL:**
```sql
INSERT INTO OrderArchive (OrderID, CustomerID, Amount)
SELECT OrderID, CustomerID, Amount
FROM Orders
WHERE OrderDate < @CutoffDate
```

**Generated Go:**
```go
result, err := r.db.ExecContext(ctx,
    `INSERT INTO OrderArchive (OrderID, CustomerID, Amount)
     SELECT OrderID, CustomerID, Amount FROM Orders WHERE OrderDate < $1`,
    cutoffDate)
```

### INSERT with DEFAULT VALUES

**T-SQL:**
```sql
INSERT INTO AuditLog DEFAULT VALUES
```

**Generated Go:**
```go
result, err := r.db.ExecContext(ctx, "INSERT INTO AuditLog DEFAULT VALUES")
```

## UPDATE Statements

### Basic UPDATE

**T-SQL:**
```sql
UPDATE Orders
SET Status = @NewStatus, ModifiedAt = GETDATE()
WHERE OrderID = @OrderID
```

**Generated Go:**
```go
result, err := r.db.ExecContext(ctx,
    "UPDATE Orders SET Status = $1, ModifiedAt = NOW() WHERE OrderID = $2",
    newStatus, orderID)
```

### UPDATE with JOIN

**T-SQL:**
```sql
UPDATE o
SET o.Status = 'Shipped', o.ShippedAt = GETDATE()
FROM Orders o
JOIN Shipments s ON o.OrderID = s.OrderID
WHERE s.TrackingNumber IS NOT NULL
```

**Generated Go (PostgreSQL):**
```go
result, err := r.db.ExecContext(ctx,
    `UPDATE Orders AS o
     SET Status = 'Shipped', ShippedAt = NOW()
     FROM Shipments AS s
     WHERE o.OrderID = s.OrderID AND s.TrackingNumber IS NOT NULL`)
```

### UPDATE with OUTPUT

**T-SQL:**
```sql
UPDATE Products
SET Price = @NewPrice
OUTPUT DELETED.Price AS OldPrice, INSERTED.Price AS NewPrice
WHERE ProductID = @ProductID
```

**Generated Go (PostgreSQL):**
```go
var oldPrice, newPrice decimal.Decimal
err := r.db.QueryRowContext(ctx,
    `UPDATE Products SET Price = $1 WHERE ProductID = $2
     RETURNING (SELECT Price FROM Products WHERE ProductID = $2), Price`,
    newPrice, productID).Scan(&oldPrice, &newPrice)
```

### UPDATE with CASE

**T-SQL:**
```sql
UPDATE Products
SET Price = CASE
    WHEN Category = 'Electronics' THEN Price * 1.10
    WHEN Category = 'Clothing' THEN Price * 1.05
    ELSE Price
END
WHERE IsActive = 1
```

**Generated Go:**
```go
result, err := r.db.ExecContext(ctx,
    `UPDATE Products SET Price = CASE
        WHEN Category = 'Electronics' THEN Price * 1.10
        WHEN Category = 'Clothing' THEN Price * 1.05
        ELSE Price
     END WHERE IsActive = TRUE`)
```

## DELETE Statements

### Basic DELETE

**T-SQL:**
```sql
DELETE FROM Orders
WHERE Status = 'Cancelled' AND OrderDate < @CutoffDate
```

**Generated Go:**
```go
result, err := r.db.ExecContext(ctx,
    "DELETE FROM Orders WHERE Status = 'Cancelled' AND OrderDate < $1",
    cutoffDate)
```

### DELETE with JOIN

**T-SQL:**
```sql
DELETE o
FROM Orders o
JOIN Customers c ON o.CustomerID = c.CustomerID
WHERE c.IsDeleted = 1
```

**Generated Go (PostgreSQL):**
```go
result, err := r.db.ExecContext(ctx,
    `DELETE FROM Orders AS o
     USING Customers AS c
     WHERE o.CustomerID = c.CustomerID AND c.IsDeleted = TRUE`)
```

### DELETE with OUTPUT

**T-SQL:**
```sql
DELETE FROM Orders
OUTPUT DELETED.OrderID, DELETED.CustomerID
WHERE Status = 'Cancelled'
```

**Generated Go (PostgreSQL):**
```go
rows, err := r.db.QueryContext(ctx,
    "DELETE FROM Orders WHERE Status = 'Cancelled' RETURNING OrderID, CustomerID")
```

### DELETE with TOP

**T-SQL:**
```sql
DELETE TOP (1000) FROM LogEntries
WHERE LogDate < @CutoffDate
```

**Generated Go (PostgreSQL):**
```go
result, err := r.db.ExecContext(ctx,
    `DELETE FROM LogEntries WHERE ctid IN (
        SELECT ctid FROM LogEntries WHERE LogDate < $1 LIMIT 1000
    )`, cutoffDate)
```

## Table Hints

SQL Server table hints are automatically stripped when targeting non-SQL Server backends:

**T-SQL Input:**
```sql
SELECT o.OrderID, c.CustomerName
FROM Orders o (NOLOCK)
JOIN Customers c WITH (NOLOCK) ON o.CustomerID = c.CustomerID
WHERE o.Status = 'Pending'
```

**Generated Go (PostgreSQL):**
```go
rows, err := r.db.QueryContext(ctx,
    `SELECT o.OrderID, c.CustomerName 
     FROM Orders AS o 
     INNER JOIN Customers AS c ON o.CustomerID = c.CustomerID 
     WHERE o.Status = 'Pending'`)
```

Supported hint patterns that are stripped:
- `(NOLOCK)`, `(ROWLOCK)`, `(TABLOCK)`, etc.
- `WITH (NOLOCK)`, `WITH (ROWLOCK, UPDLOCK)`, etc.
- `(HOLDLOCK)`, `(READPAST)`, `(NOWAIT)`, etc.

## Transactions

### Explicit Transactions

**T-SQL:**
```sql
BEGIN TRANSACTION

INSERT INTO Orders (CustomerID, Amount) VALUES (@CustomerID, @Amount)
SET @OrderID = SCOPE_IDENTITY()

UPDATE Inventory SET Quantity = Quantity - @Quantity
WHERE ProductID = @ProductID

IF @@ERROR <> 0
BEGIN
    ROLLBACK TRANSACTION
    RETURN -1
END

COMMIT TRANSACTION
```

**Generated Go:**
```go
tx, err := r.db.BeginTx(ctx, nil)
if err != nil {
    return err
}
defer tx.Rollback()

var orderID int64
err = tx.QueryRowContext(ctx,
    "INSERT INTO Orders (CustomerID, Amount) VALUES ($1, $2) RETURNING OrderID",
    customerID, amount).Scan(&orderID)
if err != nil {
    return err
}

_, err = tx.ExecContext(ctx,
    "UPDATE Inventory SET Quantity = Quantity - $1 WHERE ProductID = $2",
    quantity, productID)
if err != nil {
    return err
}

if err := tx.Commit(); err != nil {
    return err
}
```

### Transaction Configuration

```go
config := transpiler.DMLConfig{
    Backend:         transpiler.BackendSQL,
    SQLDialect:      "postgres",
    StoreVar:        "r.db",
    UseTransactions: true, // Wrap DML in transactions
}
```

## Temporary Tables

Temporary tables are transpiled to in-memory structures:

**T-SQL:**
```sql
CREATE TABLE #TempResults (
    OrderID INT,
    CustomerName VARCHAR(100),
    Total DECIMAL(18,2)
)

INSERT INTO #TempResults
SELECT o.OrderID, c.CustomerName, SUM(d.Amount)
FROM Orders o
JOIN Customers c ON o.CustomerID = c.CustomerID
JOIN OrderDetails d ON o.OrderID = d.OrderID
GROUP BY o.OrderID, c.CustomerName

SELECT * FROM #TempResults WHERE Total > 1000
```

**Generated Go:**
```go
// Temp table represented as slice of structs
type tempResults struct {
    OrderID      int64
    CustomerName string
    Total        decimal.Decimal
}
var results []tempResults

// Populate from query
rows, err := r.db.QueryContext(ctx,
    `SELECT o.OrderID, c.CustomerName, SUM(d.Amount)
     FROM Orders AS o
     INNER JOIN Customers AS c ON o.CustomerID = c.CustomerID
     INNER JOIN OrderDetails AS d ON o.OrderID = d.OrderID
     GROUP BY o.OrderID, c.CustomerName`)
// ... scan into results ...

// Filter
var filtered []tempResults
for _, r := range results {
    if r.Total.GreaterThan(decimal.NewFromInt(1000)) {
        filtered = append(filtered, r)
    }
}
```

## JSON Functions

### JSON_VALUE

**T-SQL:**
```sql
SELECT JSON_VALUE(@json, '$.customer.name') AS CustomerName
```

**Generated Go:**
```go
import "github.com/ha1tch/tgpiler/tsqlruntime"

customerName := tsqlruntime.JSONValue(jsonStr, "$.customer.name")
```

### JSON_QUERY

**T-SQL:**
```sql
SELECT JSON_QUERY(@json, '$.items') AS Items
```

**Generated Go:**
```go
items := tsqlruntime.JSONQuery(jsonStr, "$.items")
```

### JSON_MODIFY

**T-SQL:**
```sql
SET @json = JSON_MODIFY(@json, '$.status', 'completed')
SET @json = JSON_MODIFY(@json, 'append $.tags', 'urgent')
```

**Generated Go:**
```go
jsonStr = tsqlruntime.JSONModify(jsonStr, "$.status", "completed")
jsonStr = tsqlruntime.JSONModifyAppend(jsonStr, "$.tags", "urgent")
```

### OPENJSON

**T-SQL:**
```sql
SELECT *
FROM OPENJSON(@json, '$.items')
WITH (
    ProductID INT '$.id',
    Name VARCHAR(100) '$.name',
    Price DECIMAL(10,2) '$.price'
)
```

**Generated Go:**
```go
items, err := tsqlruntime.OpenJSON(jsonStr, "$.items", map[string]string{
    "ProductID": "$.id",
    "Name":      "$.name",
    "Price":     "$.price",
})
```

### FOR JSON

**T-SQL:**
```sql
SELECT OrderID, CustomerName, Amount
FROM Orders
WHERE Status = 'Active'
FOR JSON PATH, ROOT('orders')
```

**Generated Go:**
```go
rows, _ := r.db.QueryContext(ctx, "SELECT OrderID, CustomerName, Amount FROM Orders WHERE Status = 'Active'")
jsonResult := tsqlruntime.ForJSONPath(rows, tsqlruntime.ForJSONOptions{
    Root: "orders",
})
```

## XML Functions

### .value() Method

**T-SQL:**
```sql
SELECT @xml.value('(/order/@id)[1]', 'INT') AS OrderID
```

**Generated Go:**
```go
orderID := tsqlruntime.XMLValue(xmlStr, "/order/@id", "INT")
```

### .query() Method

**T-SQL:**
```sql
SELECT @xml.query('/order/items') AS ItemsXml
```

**Generated Go:**
```go
itemsXml := tsqlruntime.XMLQuery(xmlStr, "/order/items")
```

### .exist() Method

**T-SQL:**
```sql
IF @xml.exist('/order/items/item[@qty > 10]') = 1
    PRINT 'Large order detected'
```

**Generated Go:**
```go
if tsqlruntime.XMLExist(xmlStr, "/order/items/item[@qty > 10]") {
    fmt.Println("Large order detected")
}
```

### .nodes() Method

**T-SQL:**
```sql
SELECT 
    T.c.value('@id', 'INT') AS ItemID,
    T.c.value('@name', 'VARCHAR(100)') AS ItemName
FROM @xml.nodes('/order/items/item') AS T(c)
```

**Generated Go:**
```go
nodes := tsqlruntime.XMLNodes(xmlStr, "/order/items/item")
for _, node := range nodes {
    itemID := node.Value("@id", "INT")
    itemName := node.Value("@name", "VARCHAR(100)")
    // Process...
}
```

### FOR XML

**T-SQL:**
```sql
SELECT OrderID, CustomerName
FROM Orders
FOR XML PATH('order'), ROOT('orders')
```

**Generated Go:**
```go
rows, _ := r.db.QueryContext(ctx, "SELECT OrderID, CustomerName FROM Orders")
xmlResult := tsqlruntime.ForXMLPath(rows, tsqlruntime.ForXMLOptions{
    ElementName: "order",
    Root:        "orders",
})
```

## Error Handling with SPLogger

The SPLogger system provides structured error logging for CATCH blocks:

**T-SQL:**
```sql
BEGIN TRY
    INSERT INTO Orders (CustomerID, Amount) VALUES (@CustomerID, @Amount)
END TRY
BEGIN CATCH
    INSERT INTO Error.LogForStoreProcedure (
        ProcedureName, ErrorNumber, ErrorMessage, ErrorTimestamp
    )
    SELECT 'ProcessOrder', ERROR_NUMBER(), ERROR_MESSAGE(), GETDATE()
    FOR XML PATH('Error'), ROOT('Errors')
END CATCH
```

**Generated Go with SPLogger:**
```go
config := transpiler.DMLConfig{
    UseSPLogger:   true,
    SPLoggerVar:   "spLogger",
    SPLoggerType:  "db",
    SPLoggerTable: "Error.LogForStoreProcedure",
}
```

```go
// Generated code
defer func() {
    if r := recover(); r != nil {
        spLogger.LogError(ctx, tsqlruntime.SPError{
            ProcedureName: "ProcessOrder",
            ErrorMessage:  fmt.Sprintf("%v", r),
            Parameters:    map[string]interface{}{"CustomerID": customerID, "Amount": amount},
            Timestamp:     time.Now(),
        })
    }
}()

_, err := r.db.ExecContext(ctx, 
    "INSERT INTO Orders (CustomerID, Amount) VALUES ($1, $2)",
    customerID, amount)
if err != nil {
    spLogger.LogError(ctx, tsqlruntime.SPError{
        ProcedureName: "ProcessOrder",
        ErrorMessage:  err.Error(),
        Parameters:    map[string]interface{}{"CustomerID": customerID, "Amount": amount},
        Timestamp:     time.Now(),
    })
    return err
}
```

## DMLConfig Reference

```go
type DMLConfig struct {
    // Target backend: BackendSQL, BackendGRPC, BackendMock, BackendInline
    Backend BackendType

    // SQL dialect: "postgres", "mysql", "sqlite", "sqlserver"
    SQLDialect string

    // Repository/store variable name (e.g., "r.db", "r.store", "r.client")
    StoreVar string

    // Whether to wrap operations in transactions
    UseTransactions bool

    // gRPC service mappings (procedure -> service.method)
    GRPCMappings map[string]string

    // Proto package for gRPC
    ProtoPackage string

    // SPLogger configuration
    UseSPLogger    bool   // Enable SPLogger for CATCH blocks
    SPLoggerVar    string // Variable name for logger
    SPLoggerType   string // Logger type: slog, db, file, multi, nop
    SPLoggerTable  string // Table name for db logger
    SPLoggerFile   string // File path for file logger
    SPLoggerFormat string // Format for file logger: json, text
    GenLoggerInit  bool   // Generate logger initialization code
}
```

## Best Practices

### 1. Choose the Right Dialect Early

Set the SQL dialect based on your target database:

```go
config.SQLDialect = "postgres" // Default, best for new projects
```

### 2. Use Transactions for Multi-Statement Operations

Enable transactions for procedures with multiple DML statements:

```go
config.UseTransactions = true
```

### 3. Handle NULL Values Explicitly

Use COALESCE or ISNULL in T-SQL for cleaner Go code:

```sql
SELECT COALESCE(MiddleName, '') AS MiddleName FROM Users
```

### 4. Use SPLogger for Production Error Handling

Enable SPLogger for consistent error logging:

```go
config.UseSPLogger = true
config.SPLoggerType = "multi" // Log to both DB and slog
```

### 5. Test with Mock Backend First

Use the mock backend for unit tests before integrating with real databases:

```go
testConfig := transpiler.DefaultDMLConfig()
testConfig.Backend = transpiler.BackendMock
testConfig.StoreVar = "mockStore"
```