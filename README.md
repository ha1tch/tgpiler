# tgpiler

A proof-of-concept T-SQL to Go transpiler. Converts procedural T-SQL stored procedures to Go functions.

## Purpose

This tool is designed to help developers migrate business logic trapped in Microsoft SQL Server stored procedures to Go. It handles procedural constructs (variables, control flow, expressions) and generates idiomatic Go code.

**Note:** This is a proof-of-concept. It does not handle database operations (SELECT, INSERT, UPDATE, DELETE) or advanced T-SQL features like cursors, dynamic SQL, or transactions.

## Installation

```bash
go install github.com/ha1tch/tgpiler/cmd/tgpiler@latest
```

Or build from source:

```bash
git clone https://github.com/ha1tch/tgpiler.git
cd tgpiler
go build -o tgpiler ./cmd/tgpiler
```

## Dependencies

- [ha1tch/tsqlparser](https://github.com/ha1tch/tsqlparser) - T-SQL parser
- [shopspring/decimal](https://github.com/shopspring/decimal) - Arbitrary-precision decimals (for generated code using DECIMAL/MONEY types)

## Usage

```
tgpiler [options] [input.sql]

Input (mutually exclusive):
  (no argument)         Read from stdin
  <file.sql>            Read single file
  -d, --dir <path>      Read all .sql files from directory

Output (mutually exclusive):
  (no flag)             Write to stdout
  -o, --output <file>   Write to single file
  -O, --outdir <path>   Write to directory (creates if needed)

Options:
  -p, --pkg <name>      Package name for generated code (default: main)
  -f, --force           Allow overwriting existing files
  -h, --help            Show help
  -v, --version         Show version
```

### Examples

```bash
# Transpile single file to stdout
tgpiler input.sql

# Transpile with custom package name
tgpiler -p mypackage input.sql -o output.go

# Transpile directory of SQL files
tgpiler -d ./sql -O ./go -p procedures

# Pipe from stdin
cat input.sql | tgpiler -p handlers > output.go
```

## Supported Features

### Procedural Constructs

| T-SQL | Go |
|-------|-----|
| `CREATE PROCEDURE` | `func` |
| `DECLARE @var TYPE` | `var name type` |
| `SET @var = expr` | `name = expr` |
| `IF / ELSE IF / ELSE` | `if / else if / else` |
| `WHILE` | `for` |
| `BEGIN / END` | `{ }` |
| `TRY / CATCH` | `defer / recover` |
| `RETURN [value]` | `return [value]` |
| `BREAK / CONTINUE` | `break / continue` |
| `PRINT` | `fmt.Println` |
| `OUTPUT` parameters | Named return values |

### Type Mapping

| T-SQL | Go |
|-------|-----|
| `TINYINT` | `uint8` |
| `SMALLINT` | `int16` |
| `INT` | `int32` |
| `BIGINT` | `int64` |
| `REAL`, `FLOAT` | `float64` |
| `DECIMAL`, `NUMERIC`, `MONEY` | `decimal.Decimal` |
| `CHAR`, `VARCHAR`, `NVARCHAR`, `TEXT` | `string` |
| `DATE`, `DATETIME`, `DATETIME2` | `time.Time` |
| `BIT` | `bool` |
| `BINARY`, `VARBINARY` | `[]byte` |
| `UNIQUEIDENTIFIER` | `string` |

### Expressions & Functions

- Arithmetic operators with proper decimal handling
- Comparison operators
- Logical operators (`AND` → `&&`, `OR` → `||`, `NOT` → `!`)
- `CAST` / `CONVERT`
- `CASE` expressions
- `IIF`
- String functions: `LEN`, `UPPER`, `LOWER`, `TRIM`, `SUBSTRING`, `LEFT`, `RIGHT`, `CHARINDEX`, `REPLACE`, `CONCAT`
- Math functions: `ABS`, `CEILING`, `FLOOR`, `ROUND`, `POWER`, `SQRT`
- Date functions: `GETDATE`, `DATEADD`, `DATEDIFF`, `YEAR`, `MONTH`, `DAY`
- NULL functions: `ISNULL`, `COALESCE`, `NULLIF`
- Error functions: `ERROR_MESSAGE`

### Comments

Comments are preserved and associated with nearby code:

```sql
-- Calculate the total
SET @Total = @Price * @Quantity
```

Becomes:

```go
// Calculate the total
Total = Price.Mul(decimal.NewFromInt(int64(Quantity)))
```

## Example

Input (`discount.sql`):

```sql
-- Calculate discount for a purchase
CREATE PROCEDURE dbo.CalculateDiscount
    @Price DECIMAL(10,2),
    @Quantity INT,
    @Total DECIMAL(10,2) OUTPUT
AS
BEGIN
    DECLARE @Discount DECIMAL(10,2) = 0
    
    SET @Total = @Price * @Quantity
    
    -- Apply volume discount
    IF @Quantity >= 100
        SET @Discount = @Total * 0.15
    ELSE IF @Quantity >= 50
        SET @Discount = @Total * 0.10
    
    SET @Total = @Total - @Discount
END
```

Output:

```go
package main

import (
    "github.com/shopspring/decimal"
)

// Calculate discount for a purchase
func CalculateDiscount(Price decimal.Decimal, Quantity int32) (Total decimal.Decimal) {
    var Discount decimal.Decimal = decimal.NewFromInt(0)
    Total = Price.Mul(decimal.NewFromInt(int64(Quantity)))
    // Apply volume discount
    if (Quantity >= 100) {
        Discount = Total.Mul(decimal.NewFromFloat(0.15))
    } else if (Quantity >= 50) {
        Discount = Total.Mul(decimal.NewFromFloat(0.10))
    }
    Total = Total.Sub(Discount)
    return Total
}
```

## Not Supported (Yet)

- `SELECT`, `INSERT`, `UPDATE`, `DELETE`, `MERGE`
- Cursors
- Table variables and temp tables
- `EXEC` / `EXECUTE` (calling other procedures)
- Dynamic SQL (`EXEC(@sql)`)
- Transactions (`BEGIN TRAN`, `COMMIT`, `ROLLBACK`)
- `RAISERROR` / `THROW`
- User-defined functions
- Common Table Expressions (CTEs)
- Window functions

## Project Structure

```
tgpiler/
├── cmd/tgpiler/        # CLI entry point
├── transpiler/         # Core transpilation logic
│   ├── transpiler.go   # Main transpiler
│   ├── expressions.go  # Expression handling
│   ├── types.go        # Type mapping
│   ├── symbols.go      # Symbol table
│   └── comments.go     # Comment preservation
├── examples/           # Example T-SQL files
└── README.md
```

## License
GNU GENERAL PUBLIC LICENSE VERSION 3.0
https://github.com/ha1tch/LICENSE

## Contributing

This is a proof-of-concept. Issues and pull requests are welcome.

## Related Projects

- [tsqlparser](https://github.com/ha1tch/tsqlparser) - The T-SQL parser used by this project
