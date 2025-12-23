# tgpiler User Manual

A T-SQL to Go transpiler and runtime interpreter for migrating SQL Server stored procedures to Go.

## Overview

tgpiler provides two complementary execution modes for T-SQL code:

**Transpiler Mode** generates static Go code from T-SQL stored procedures. The generated code is type-safe, idiomatic Go that can be compiled and executed without any runtime parsing overhead.

**Interpreter Mode** executes T-SQL dynamically at runtime, enabling scenarios like dynamic SQL (`EXEC(@sql)`), scrollable cursors, and nested transactions that cannot be statically transpiled.

## Installation

```bash
go install github.com/ha1tch/tgpiler/cmd/tgpiler@latest
```

Or build from source:

```bash
git clone https://github.com/ha1tch/tgpiler.git
cd tgpiler
make build
```

## Quick Start

### Basic Transpilation

```bash
# Transpile a single procedure (procedural logic only)
tgpiler input.sql -o output.go

# Transpile with DML support (SELECT, INSERT, UPDATE, DELETE)
tgpiler --dml input.sql -o output.go

# Transpile a directory of SQL files
tgpiler --dml -d ./sql -O ./go -p mypackage
```

### Example Transformation

**Input (T-SQL):**
```sql
CREATE PROCEDURE dbo.CalculateDiscount
    @OrderTotal DECIMAL(18,2),
    @CustomerTier INT,
    @DiscountPercent DECIMAL(5,2) OUTPUT,
    @FinalAmount DECIMAL(18,2) OUTPUT
AS
BEGIN
    SET NOCOUNT ON
    
    IF @CustomerTier = 1
        SET @DiscountPercent = 5.0
    ELSE IF @CustomerTier = 2
        SET @DiscountPercent = 10.0
    ELSE IF @CustomerTier = 3
        SET @DiscountPercent = 15.0
    ELSE
        SET @DiscountPercent = 0.0
    
    SET @FinalAmount = @OrderTotal * (1 - @DiscountPercent / 100)
    
    RETURN 0
END
```

**Output (Go):**
```go
func CalculateDiscount(ctx context.Context, orderTotal decimal.Decimal, 
    customerTier int32) (discountPercent decimal.Decimal, 
    finalAmount decimal.Decimal, returnCode int, err error) {
    
    if customerTier == 1 {
        discountPercent = decimal.NewFromFloat(5.0)
    } else if customerTier == 2 {
        discountPercent = decimal.NewFromFloat(10.0)
    } else if customerTier == 3 {
        discountPercent = decimal.NewFromFloat(15.0)
    } else {
        discountPercent = decimal.NewFromFloat(0.0)
    }
    
    finalAmount = orderTotal.Mul(decimal.NewFromInt(1).Sub(
        discountPercent.Div(decimal.NewFromInt(100))))
    
    return discountPercent, finalAmount, 0, nil
}
```

## Command-Line Interface

```
Usage:
  tgpiler [options] <input.sql>
  tgpiler [options] -s < input.sql
  tgpiler [options] -d <path>

Input (mutually exclusive):
  <file.sql>            Read single file
  -s, --stdin           Read from stdin
  -d, --dir <path>      Read all .sql files from directory

Output (mutually exclusive):
  (no flag)             Write to stdout
  -o, --output <file>   Write to single file
  -O, --outdir <path>   Write to directory (creates if needed)

Options:
  -p, --pkg <name>      Package name for generated code (default: main)
  --dml                 Enable DML mode (SELECT, INSERT, temp tables, JSON/XML)
  -f, --force           Allow overwriting existing files
  -h, --help            Show help
  -v, --version         Show version
```

## Supported T-SQL Features

### Procedural Constructs

| T-SQL | Go |
|-------|-----|
| `DECLARE @var TYPE` | `var varName type` |
| `SET @var = expr` | `varName = expr` |
| `SELECT @var = expr` | `varName = expr` |
| `IF ... ELSE` | `if ... else` |
| `WHILE` | `for` |
| `BREAK` | `break` |
| `CONTINUE` | `continue` |
| `RETURN` | `return` |
| `BEGIN ... END` | `{ ... }` |
| `GOTO label` | (refactored to structured control flow) |

### Data Types

| T-SQL Type | Go Type |
|------------|---------|
| `INT`, `SMALLINT`, `TINYINT` | `int32` |
| `BIGINT` | `int64` |
| `BIT` | `bool` |
| `DECIMAL(p,s)`, `NUMERIC(p,s)` | `decimal.Decimal` |
| `FLOAT`, `REAL` | `float64` |
| `VARCHAR`, `NVARCHAR`, `CHAR`, `NCHAR` | `string` |
| `DATE`, `DATETIME`, `DATETIME2` | `time.Time` |
| `UNIQUEIDENTIFIER` | `string` (UUID format) |
| `VARBINARY`, `BINARY` | `[]byte` |
| `XML` | `string` |
| `TABLE` (table variable) | `[]map[string]interface{}` |

### Expressions and Operators

**Arithmetic:** `+`, `-`, `*`, `/`, `%`

**Comparison:** `=`, `<>`, `!=`, `<`, `>`, `<=`, `>=`

**Logical:** `AND`, `OR`, `NOT`

**String:** `+` (concatenation), `LIKE`, `CHARINDEX`, `SUBSTRING`, `LEN`, `LEFT`, `RIGHT`, `LTRIM`, `RTRIM`, `REPLACE`, `UPPER`, `LOWER`

**NULL handling:** `IS NULL`, `IS NOT NULL`, `ISNULL()`, `COALESCE()`, `NULLIF()`

**CASE expressions:**
```sql
CASE @status
    WHEN 1 THEN 'Active'
    WHEN 2 THEN 'Pending'
    ELSE 'Unknown'
END

CASE 
    WHEN @amount > 1000 THEN 'Large'
    WHEN @amount > 100 THEN 'Medium'
    ELSE 'Small'
END
```

### Built-in Functions

**Mathematical:**
- `ABS`, `CEILING`, `FLOOR`, `ROUND`
- `POWER`, `SQRT`, `LOG`, `LOG10`, `EXP`
- `SIGN`, `RAND`

**String:**
- `LEN`, `DATALENGTH`
- `LEFT`, `RIGHT`, `SUBSTRING`
- `CHARINDEX`, `PATINDEX`
- `REPLACE`, `STUFF`
- `LTRIM`, `RTRIM`, `TRIM`
- `UPPER`, `LOWER`
- `REPLICATE`, `SPACE`
- `REVERSE`, `ASCII`, `CHAR`
- `CONCAT`, `CONCAT_WS`, `STRING_AGG`

**Date/Time:**
- `GETDATE`, `GETUTCDATE`, `SYSDATETIME`
- `DATEADD`, `DATEDIFF`, `DATEDIFF_BIG`
- `DATEPART`, `DATENAME`
- `YEAR`, `MONTH`, `DAY`
- `EOMONTH`, `DATEFROMPARTS`
- `FORMAT` (date formatting)

**Conversion:**
- `CAST(expr AS type)`
- `CONVERT(type, expr [, style])`
- `TRY_CAST`, `TRY_CONVERT`

**System:**
- `@@ROWCOUNT`, `@@ERROR`, `@@IDENTITY`
- `@@TRANCOUNT`, `@@FETCH_STATUS`
- `SCOPE_IDENTITY()`, `NEWID()`
- `ERROR_NUMBER()`, `ERROR_MESSAGE()`, `ERROR_SEVERITY()`, `ERROR_STATE()`, `ERROR_LINE()`

### Error Handling

**TRY/CATCH blocks:**
```sql
BEGIN TRY
    -- operations that might fail
    INSERT INTO Orders (CustomerID, Amount) VALUES (@CustomerID, @Amount)
END TRY
BEGIN CATCH
    -- error handling
    DECLARE @ErrorMessage NVARCHAR(4000) = ERROR_MESSAGE()
    DECLARE @ErrorNumber INT = ERROR_NUMBER()
    
    INSERT INTO ErrorLog (Message, Number, Timestamp)
    VALUES (@ErrorMessage, @ErrorNumber, GETDATE())
    
    -- Re-throw or return error
    THROW
END CATCH
```

**RAISERROR and THROW:**
```sql
-- RAISERROR with severity and state
RAISERROR('Invalid order amount: %d', 16, 1, @Amount)

-- THROW (SQL Server 2012+)
THROW 50001, 'Custom error message', 1
```

### Cursors

Cursors are transpiled to idiomatic Go iteration patterns:

**T-SQL:**
```sql
DECLARE @ID INT, @Name VARCHAR(100)

DECLARE order_cursor CURSOR FOR
    SELECT OrderID, CustomerName FROM Orders WHERE Status = 'Pending'

OPEN order_cursor

FETCH NEXT FROM order_cursor INTO @ID, @Name

WHILE @@FETCH_STATUS = 0
BEGIN
    -- Process each row
    PRINT 'Processing order: ' + CAST(@ID AS VARCHAR)
    
    FETCH NEXT FROM order_cursor INTO @ID, @Name
END

CLOSE order_cursor
DEALLOCATE order_cursor
```

**Generated Go:**
```go
rows, err := r.db.QueryContext(ctx, 
    "SELECT OrderID, CustomerName FROM Orders WHERE Status = $1", "Pending")
if err != nil {
    return err
}
defer rows.Close()

for rows.Next() {
    var id int32
    var name string
    if err := rows.Scan(&id, &name); err != nil {
        return err
    }
    
    // Process each row
    fmt.Printf("Processing order: %d\n", id)
}
```

### Temporary Tables

Temporary tables are transpiled to in-memory data structures:

```sql
CREATE TABLE #TempOrders (
    OrderID INT,
    Amount DECIMAL(18,2),
    Status VARCHAR(20)
)

INSERT INTO #TempOrders
SELECT OrderID, Amount, Status FROM Orders WHERE CustomerID = @CustomerID

-- Query the temp table
SELECT * FROM #TempOrders WHERE Amount > 100
```

### JSON Support

tgpiler supports SQL Server's JSON functions (requires `--dml` mode):

**JSON_VALUE and JSON_QUERY:**
```sql
DECLARE @json NVARCHAR(MAX) = '{"name":"John","address":{"city":"NYC"}}'

SELECT JSON_VALUE(@json, '$.name')           -- Returns 'John'
SELECT JSON_QUERY(@json, '$.address')        -- Returns '{"city":"NYC"}'
```

**JSON_MODIFY:**
```sql
SET @json = JSON_MODIFY(@json, '$.age', 30)
SET @json = JSON_MODIFY(@json, '$.address.zip', '10001')
```

**OPENJSON:**
```sql
SELECT *
FROM OPENJSON(@json)
WITH (
    name VARCHAR(100) '$.name',
    city VARCHAR(100) '$.address.city'
)
```

**FOR JSON:**
```sql
SELECT OrderID, CustomerName, Amount
FROM Orders
WHERE Status = 'Active'
FOR JSON PATH, ROOT('orders')
```

### XML Support

Full XML function support (requires `--dml` mode):

**XQuery methods:**
```sql
DECLARE @xml XML = '<root><item id="1">Value</item></root>'

SELECT @xml.value('(/root/item/@id)[1]', 'INT')      -- Returns 1
SELECT @xml.query('/root/item')                       -- Returns XML fragment
SELECT @xml.exist('/root/item[@id=1]')               -- Returns 1 (true)
```

**nodes() for shredding:**
```sql
SELECT 
    T.c.value('@id', 'INT') AS ItemID,
    T.c.value('.', 'VARCHAR(100)') AS ItemValue
FROM @xml.nodes('/root/item') AS T(c)
```

**FOR XML:**
```sql
SELECT OrderID, Amount
FROM Orders
FOR XML PATH('order'), ROOT('orders')
```

## DML Mode

Enable with `--dml` flag for database operations. See [DML.md](DML.md) for complete documentation.

**Supported statements:**
- `SELECT` (single row, multi-row, with joins, subqueries, CTEs)
- `INSERT` (values, select, default values)
- `UPDATE` (simple, with joins, with OUTPUT)
- `DELETE` (simple, with joins, with OUTPUT)
- `MERGE` (partial support)

**Target backends:**
- PostgreSQL (default)
- MySQL
- SQLite
- SQL Server (for compatibility testing)
- gRPC (see [GRPC.md](GRPC.md))
- Mock (for testing)

## Runtime Interpreter

The `tsqlruntime` package provides dynamic SQL execution for scenarios that cannot be statically transpiled:

```go
import "github.com/ha1tch/tgpiler/tsqlruntime"

// Create interpreter with database connection
interp := tsqlruntime.NewInterpreter(db, tsqlruntime.DialectPostgres)

// Set parameters
interp.SetVariable("@CustomerID", 123)
interp.SetVariable("@MinAmount", 100.0)

// Execute dynamic SQL
result, err := interp.Execute(ctx, `
    SELECT OrderID, Amount 
    FROM Orders 
    WHERE CustomerID = @CustomerID AND Amount > @MinAmount
`, nil)

// Process results
for _, row := range result.ResultSets[0].Rows {
    orderID := row[0].AsInt64()
    amount := row[1].AsDecimal()
    // ...
}
```

### Interpreter-Only Features

These features require the runtime interpreter and cannot be statically transpiled:

**Dynamic SQL:**
```sql
DECLARE @sql NVARCHAR(MAX)
DECLARE @table VARCHAR(100) = 'Orders'

SET @sql = 'SELECT * FROM ' + @table + ' WHERE Status = @Status'

EXEC sp_executesql @sql, N'@Status VARCHAR(20)', @Status = 'Active'
```

**Scrollable Cursors:**
```sql
DECLARE scroll_cursor SCROLL CURSOR FOR
    SELECT OrderID FROM Orders ORDER BY OrderDate

OPEN scroll_cursor
FETCH LAST FROM scroll_cursor INTO @ID       -- Jump to last row
FETCH ABSOLUTE 5 FROM scroll_cursor INTO @ID -- Jump to row 5
FETCH RELATIVE -2 FROM scroll_cursor INTO @ID -- Move back 2 rows
```

**Nested Transactions:**
```sql
BEGIN TRANSACTION

    INSERT INTO Orders (...) VALUES (...)
    
    SAVE TRANSACTION SavePoint1
    
    BEGIN TRY
        UPDATE Inventory SET Quantity = Quantity - 1 WHERE ProductID = @ProductID
    END TRY
    BEGIN CATCH
        ROLLBACK TRANSACTION SavePoint1
    END CATCH

COMMIT TRANSACTION
```

## SPLogger: Error Logging

The SPLogger system provides structured error logging that mirrors the common T-SQL pattern of logging errors to a database table:

```go
import "github.com/ha1tch/tgpiler/tsqlruntime"

// Create a database logger
logger := tsqlruntime.NewDBLogger(db, "dbo.ErrorLog")

// Or use slog
logger := tsqlruntime.NewSlogLogger(slog.Default())

// Or multi-logger for multiple destinations
logger := tsqlruntime.NewMultiLogger(
    tsqlruntime.NewDBLogger(db, "dbo.ErrorLog"),
    tsqlruntime.NewSlogLogger(slog.Default()),
)

// Log errors from CATCH blocks
err := logger.LogError(ctx, tsqlruntime.SPError{
    ProcedureName: "ProcessOrder",
    Parameters:    map[string]interface{}{"OrderID": 123},
    ErrorMessage:  "Insufficient inventory",
    ErrorNumber:   50001,
    Severity:      16,
})
```

## Testing Generated Code

tgpiler includes a mock store for testing generated code without a database:

```go
import "github.com/ha1tch/tgpiler/mock"

store := mock.NewMockStore()

// Seed test data
store.Insert("Orders", map[string]interface{}{
    "OrderID":    1,
    "CustomerID": 100,
    "Amount":     250.00,
    "Status":     "Pending",
})

// Query with filtering
orders, _ := store.Select("Orders", map[string]interface{}{
    "CustomerID": 100,
})

// Update
store.Update("Orders", 
    map[string]interface{}{"Status": "Processed"},
    map[string]interface{}{"OrderID": 1},
)
```

## Project Structure

```
tgpiler/
├── cmd/tgpiler/         # CLI application
├── transpiler/          # Core transpilation logic
│   ├── transpiler.go    # Main transpiler
│   ├── dml.go           # DML statement handling
│   ├── expressions.go   # Expression transpilation
│   └── types.go         # Type mappings
├── tsqlruntime/         # Runtime interpreter
│   ├── interpreter.go   # Dynamic SQL execution
│   ├── context.go       # Execution context
│   ├── cursor.go        # Cursor support
│   ├── temptable.go     # Temp table support
│   ├── functions.go     # Built-in functions
│   ├── json.go          # JSON functions
│   ├── xml.go           # XML functions
│   └── splogger.go      # Error logging
├── storage/             # Storage layer generation
│   ├── detector.go      # Operation detection
│   ├── dialects.go      # SQL dialect support
│   └── mapper.go        # Type mapping
├── protogen/            # gRPC code generation
├── mock/                # Mock store for testing
├── adapter/             # Database adapters
└── examples/            # Example applications
```

## Best Practices

### 1. Start with Procedural Logic

Begin by transpiling procedures that contain only procedural logic (no DML). This validates the control flow and expressions before adding database operations.

### 2. Use Type-Safe Parameters

The generated Go code uses strongly-typed parameters. Ensure your T-SQL parameter types map correctly:

```sql
-- Good: explicit types
CREATE PROCEDURE GetOrder
    @OrderID BIGINT,           -- maps to int64
    @CustomerName VARCHAR(100) -- maps to string
AS ...

-- Avoid: ambiguous types
CREATE PROCEDURE GetOrder
    @ID INT,        -- Could be order ID, customer ID, etc.
    @Name VARCHAR   -- Missing length
AS ...
```

### 3. Handle NULLs Explicitly

T-SQL NULL semantics differ from Go. Use `ISNULL()` or `COALESCE()` to provide defaults:

```sql
-- Explicit NULL handling
SET @Result = ISNULL(@Input, 0)

-- Multiple fallbacks
SET @Name = COALESCE(@FirstName, @LastName, 'Unknown')
```

### 4. Prefer SET NOCOUNT ON

Always include `SET NOCOUNT ON` to prevent row count messages from interfering with result sets:

```sql
CREATE PROCEDURE MyProc
AS
BEGIN
    SET NOCOUNT ON
    -- ... procedure body
END
```

### 5. Test with the Mock Store

Use the mock store for unit testing before connecting to a real database:

```go
func TestProcessOrder(t *testing.T) {
    store := mock.NewMockStore()
    
    // Seed data
    store.Insert("Products", map[string]interface{}{
        "ProductID": 1,
        "Stock":     100,
    })
    
    // Test the transpiled function
    err := ProcessOrder(ctx, store, orderID, quantity)
    
    // Verify results
    product, _ := store.SelectOne("Products", map[string]interface{}{"ProductID": 1})
    assert.Equal(t, 95, product["Stock"])
}
```

## Troubleshooting

### Parse Errors

If transpilation fails with parse errors, check:

1. **Unsupported syntax**: Some T-SQL features may not be supported yet
2. **Missing semicolons**: While optional in T-SQL, adding them can help parsing
3. **Reserved words**: Ensure identifiers don't conflict with Go reserved words

### Type Mismatches

If the generated code has type errors:

1. Check the T-SQL parameter types match the expected Go types
2. Use explicit casts in T-SQL: `CAST(@value AS DECIMAL(18,2))`
3. Review the type mapping table above

### Runtime Errors

For errors during execution:

1. Enable debug mode: `interp.Debug = true`
2. Check variable initialization
3. Verify database connection and permissions
4. Review transaction state with `@@TRANCOUNT`

## Further Reading

- [DML.md](DML.md) - Complete DML mode documentation
- [GRPC.md](GRPC.md) - gRPC backend and protobuf generation
- [storage/DESIGN.md](../storage/DESIGN.md) - Storage layer architecture
- [tsqlruntime/README.md](../tsqlruntime/README.md) - Runtime interpreter details