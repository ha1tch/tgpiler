# tgpiler

A T-SQL to Go transpiler and runtime interpreter. Converts T-SQL stored procedures to Go functions, and provides a runtime interpreter for dynamic SQL execution including transactions.

## Purpose

This tool helps developers migrate business logic from Microsoft SQL Server stored procedures to Go. It provides two execution modes:

**Transpiler Mode** — Static code generation:
- Procedural constructs (variables, control flow, expressions)
- DML operations (SELECT, INSERT, UPDATE, DELETE)
- Cursors → idiomatic Go `rows.Next()` iteration
- JSON functions (JSON_VALUE, JSON_QUERY, JSON_MODIFY, OPENJSON, FOR JSON)
- XML functions (.value(), .query(), .exist(), .nodes(), .modify(), OPENXML, FOR XML)
- Temp tables (#temp), RAISERROR/THROW

**Interpreter Mode** — Dynamic SQL execution at runtime:
- Everything above, plus:
- **Transactions** (BEGIN TRAN, COMMIT, ROLLBACK, nested transactions)
- **Dynamic SQL** (EXEC(@sql), sp_executesql with parameters)
- **Scrollable cursors** (FETCH ABSOLUTE, FETCH RELATIVE, FETCH FIRST/LAST)
- **Full error handling** (TRY/CATCH, ERROR_NUMBER(), ERROR_MESSAGE(), XACT_STATE())

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

## Dependencies

- [ha1tch/tsqlparser](https://github.com/ha1tch/tsqlparser) - T-SQL parser
- [shopspring/decimal](https://github.com/shopspring/decimal) - Arbitrary-precision decimals

## Usage

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

### Examples

```bash
# Transpile procedural logic (default mode)
tgpiler input.sql

# Transpile with DML support (database operations, JSON/XML)
tgpiler --dml input.sql

# Transpile with custom package name
tgpiler -p mypackage input.sql -o output.go

# Read from stdin
tgpiler -s < input.sql
cat input.sql | tgpiler -s

# Transpile directory of SQL files with DML mode
tgpiler --dml -d ./sql -O ./go -p procedures
```

## Testing

The project includes a comprehensive test suite with 80 T-SQL sample files.

### Quick Start

```bash
# Run all tests
make test

# Quick smoke test
make test-quick

# End-to-end tests (transpile → compile → execute)
make test-e2e
```

### Test Categories

| Command | Description |
|---------|-------------|
| `make test` | Run all unit tests |
| `make test-e2e` | Full end-to-end tests |
| `make test-e2e-compile` | Verify all files compile |
| `make test-e2e-execute` | Execute transpiled functions |
| `make test-compilation` | Syntax verification (gofmt) |
| `make test-structured` | JSON/XML DML mode tests |

### Structured Data Tests (JSON/XML)

```bash
# Run all structured data tests
go test -v ./tests/... -run "TestCompilationStructured|TestStructuredFullBuild|TestE2EExecute"

# Compilation only (fast)
go test -v ./tests/... -run TestCompilationStructuredDML

# Full build test (all 25 files as a package)
go test -v ./tests/... -run TestStructuredFullBuild

# E2E execution tests
go test -v ./tests/... -run "TestE2EExecuteJSON|TestE2EExecuteXML"
```

## Sample Files

The project includes 90 T-SQL sample files across 5 categories:

### Basic Algorithms (`tsql_basic/`) — 20 files

| File | Function | Description |
|------|----------|-------------|
| 01_simple_add.sql | AddNumbers | Basic arithmetic |
| 02_factorial.sql | Factorial | Iterative factorial |
| 03_fizzbuzz.sql | FizzBuzz | Classic interview problem |
| 04_gcd.sql | Gcd | Euclidean algorithm |
| 05_is_prime.sql | IsPrime | Primality testing |
| 06_fibonacci.sql | Fibonacci | Iterative Fibonacci |
| 07_discount_calc.sql | CalculateDiscount | Tiered pricing |
| 08_count_words.sql | CountWords | String parsing |
| 09_validate_email.sql | ValidateEmail | Basic validation |
| 10_temp_convert.sql | ConvertTemperature | Unit conversion |
| 11_business_days.sql | AddBusinessDays | Date arithmetic |
| 12_loan_calc.sql | CalculateLoan | Simple interest |
| 13_binary_search.sql | BinarySearch | Search algorithm |
| 14_password_check.sql | CheckPasswordStrength | String analysis |
| 15_safe_divide.sql | SafeDivide | TRY/CATCH error handling |
| 16_grade_calc.sql | CalculateGrade | Score classification |
| 17_roman_numerals.sql | ToRomanNumeral | Number conversion |
| 18_luhn_validation.sql | ValidateCreditCard | Luhn algorithm |
| 19_math_utils.sql | MathUtils | Multiple functions |
| 20_order_processing.sql | ProcessOrder | Business logic |

### Non-Trivial Algorithms (`tsql_nontrivial/`) — 15 files

| File | Function | Description |
|------|----------|-------------|
| 01_levenshtein.sql | LevenshteinDistance | Edit distance (O(n×m) DP) |
| 02_extended_euclidean.sql | ExtendedEuclidean | Bézout coefficients |
| 03_base64_encode.sql | Base64Encode | RFC 4648 encoding |
| 04_run_length_encoding.sql | RunLengthEncode | RLE compression |
| 04b_run_length_decode.sql | RunLengthDecode | RLE decompression |
| 05_newton_raphson.sql | NewtonSqrt | Square root approximation |
| 05b_newton_nth_root.sql | NewtonNthRoot | Nth root approximation |
| 06_easter_computus.sql | CalculateEasterDate | Anonymous Gregorian algorithm |
| 07_modular_arithmetic.sql | ModularExponentiation | Fast exponentiation |
| 07b_modular_inverse.sql | ModularInverse | Extended Euclidean method |
| 08_lcs.sql | LongestCommonSubsequence | LCS length (O(n×m) DP) |
| 09_amortisation.sql | AmortisationSchedule | Loan amortisation |
| 09b_effective_rate.sql | EffectiveAnnualRate | Interest rate conversion |
| 10_checksums.sql | CRC16_CCITT | CRC-16 CCITT polynomial |
| 10b_adler32.sql | Adler32 | Adler-32 checksum |

### Financial Calculations (`tsql_financial/`) — 20 files

| File | Function | Description |
|------|----------|-------------|
| 01_future_value.sql | FutureValue | Compound interest FV |
| 02_present_value.sql | PresentValue | Discounted cash flow PV |
| 03_simple_interest.sql | SimpleInterest | I = Prt calculation |
| 04_loan_payment.sql | LoanPayment | PMT formula for loans |
| 05_currency_convert.sql | CurrencyConvert | Bid/ask spread |
| 06_progressive_tax.sql | ProgressiveTax | 6-bracket marginal tax |
| 07_straight_line_depreciation.sql | StraightLineDepreciation | Asset depreciation |
| 08_declining_balance_depreciation.sql | DecliningBalanceDepreciation | Accelerated depreciation |
| 09_markup_margin.sql | MarkupMargin | Markup ↔ margin conversion |
| 10_break_even.sql | BreakEvenAnalysis | Break-even point |
| 11_amortization_period.sql | AmortizationPeriod | Per-period loan breakdown |
| 12_irr.sql | InternalRateOfReturn | Newton-Raphson IRR |
| 13_npv.sql | NetPresentValue | NPV with profitability index |
| 14_bond_price.sql | BondPrice | Bond fair value pricing |
| 15_yield_to_maturity.sql | YieldToMaturity | Bisection method YTM |
| 16_cagr.sql | CompoundAnnualGrowthRate | CAGR via Newton-Raphson |
| 17_loan_comparison.sql | LoanComparison | Compare loans with fees |
| 18_sinking_fund.sql | SinkingFund | Required periodic deposits |
| 19_effective_rate_with_fees.sql | EffectiveRateWithFees | True APR with all fees |
| 20_portfolio_return.sql | PortfolioWeightedReturn | Weighted return & Sharpe |

### Structured Data — JSON/XML (`tsql_structured/`) — 25 files

| File | Function | Description |
|------|----------|-------------|
| 01_json_value_extract.sql | ParseCustomerJson | JSON_VALUE scalar extraction |
| 02_json_nested_extract.sql | ParseOrderJson | Nested JSON with ISJSON |
| 03_openjson_basic.sql | ParseJsonArray | OPENJSON without schema |
| 04_openjson_schema.sql | ParseProductsJson | OPENJSON WITH schema |
| 05_json_modify.sql | UpdateCustomerJson | JSON_MODIFY updates |
| 06_for_json_path.sql | BuildOrdersJson | FOR JSON PATH output |
| 07_for_json_root.sql | BuildCustomerJson | FOR JSON with ROOT |
| 08_json_validate_process.sql | ValidateAndProcessJson | JSON validation workflow |
| 09_json_aggregate.sql | CalculateOrderTotals | JSON array aggregation |
| 10_json_merge.sql | MergeJsonDocuments | JSON document merging |
| 11_xml_value_extract.sql | ParseCustomerXml | XML .value() extraction |
| 12_xml_attributes.sql | ParseProductXmlAttributes | XML attribute extraction |
| 13_xml_exist.sql | ValidateOrderXml | XML .exist() validation |
| 14_xml_nodes.sql | ParseInvoiceItems | XML .nodes() shredding |
| 15_openxml.sql | ImportEmployeesXml | OPENXML legacy pattern |
| 16_for_xml_raw.sql | BuildEmployeesXml | FOR XML RAW output |
| 17_for_xml_path_elements.sql | BuildOrderXml | FOR XML PATH ELEMENTS |
| 18_xml_query.sql | ExtractXmlFragment | XML .query() fragments |
| 19_xml_aggregate.sql | SummarizeXmlData | XML data aggregation |
| 20_xml_modify.sql | UpdateConfigXml | XML .modify() DML |
| 21_xml_to_json.sql | ConvertXmlToJson | XML to JSON conversion |
| 22_json_to_xml.sql | ConvertJsonToXml | JSON to XML conversion |
| 23_json_config.sql | ParseAppConfig | JSON config parsing |
| 24_xml_invoice.sql | ProcessInvoiceXml | Complex XML invoice |
| 25_json_api_response.sql | BuildApiResponse | JSON API response builder |

### CTEs and Window Functions (`tsql_cte/`) — 10 files

| File | Function | Description |
|------|----------|-------------|
| 01_simple_cte.sql | GetCustomerSummary | Basic CTE usage |
| 02_multiple_cte.sql | GetCustomerMetrics | Multiple CTEs in one query |
| 03_recursive_cte.sql | GetEmployeeHierarchy | Recursive CTE (org chart) |
| 04_cte_insert.sql | ArchiveOldOrders | CTE with INSERT |
| 05_cte_update.sql | UpdateCustomerTiers | CTE with UPDATE |
| 06_cte_delete.sql | RemoveDuplicateOrders | CTE with DELETE |
| 07_window_ranking.sql | GetProductRankings | ROW_NUMBER, RANK, DENSE_RANK, NTILE |
| 08_window_navigation.sql | GetOrderHistory | LEAD, LAG, FIRST_VALUE, LAST_VALUE |
| 09_window_aggregates.sql | GetSalesAnalysis | Running totals, moving averages |
| 10_pagination.sql | GetOrdersPage | ROW_NUMBER for pagination |

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
| `TRY / CATCH` | `defer / recover` (IIFE pattern) |
| `RETURN [value]` | `return [value]` |
| `BREAK / CONTINUE` | `break / continue` |
| `PRINT` | `fmt.Println` |
| `OUTPUT` parameters | Named return values |
| `RAISERROR` | `fmt.Errorf` / `return err` |
| `@@FETCH_STATUS` | `rows.Next()` loop condition |

### DML Operations (--dml mode)

| T-SQL | Go |
|-------|-----|
| `SELECT ... INTO @var` | `db.QueryRowContext().Scan()` |
| `SELECT ... FROM` | `db.QueryContext()` with row iteration |
| `INSERT INTO` | `db.ExecContext()` |
| `UPDATE` | `db.ExecContext()` |
| `DELETE` | `db.ExecContext()` |
| `CREATE TABLE #temp` | `tsqlruntime.TempTableManager` |
| `DROP TABLE #temp` | `tempTables.DropTempTable()` |
| `DECLARE CURSOR ... OPEN ... FETCH ... CLOSE` | `db.QueryContext()` with `rows.Next()` loop |

### JSON Functions (--dml mode)

| T-SQL | Go |
|-------|-----|
| `JSON_VALUE(json, path)` | `JsonValue(json, path)` |
| `JSON_QUERY(json, path)` | `JsonQuery(json, path)` |
| `JSON_MODIFY(json, path, val)` | `JsonModify(json, path, val)` |
| `ISJSON(string)` | `Isjson(string)` |
| `OPENJSON(json)` | Table-valued function |
| `OPENJSON(json) WITH (...)` | Typed table-valued function |
| `FOR JSON PATH` | JSON array output |
| `FOR JSON AUTO` | Automatic JSON structure |

### XML Functions (--dml mode)

| T-SQL | Go |
|-------|-----|
| `@xml.value(xpath, type)` | `XmlValueString()` with type conversion |
| `@xml.query(xpath)` | `XmlQuery()` |
| `@xml.exist(xpath)` | `XmlExist()` returns `bool` |
| `@xml.nodes(xpath)` | `XmlNodes()` |
| `@xml.modify(dml)` | `XmlModify()` |
| `OPENXML(@hdoc, path) WITH` | Legacy XML shredding |
| `FOR XML RAW` | XML element output |
| `FOR XML PATH` | Customised XML structure |
| `FOR XML PATH, ELEMENTS` | Element-centric XML |

### Window Functions (--dml mode)

Window functions are fully supported with automatic type inference. The transpiler analyses each window function and assigns the correct Go type:

| Window Function | Go Type | Notes |
|-----------------|---------|-------|
| `ROW_NUMBER()` | `int64` | Always returns row position |
| `RANK()` | `int64` | Rank with gaps |
| `DENSE_RANK()` | `int64` | Rank without gaps |
| `NTILE(n)` | `int64` | Bucket number |
| `PERCENT_RANK()` | `float64` | Relative rank (0.0 to 1.0) |
| `CUME_DIST()` | `float64` | Cumulative distribution |
| `LEAD(expr)` | *Same as expr* | Next row value |
| `LAG(expr)` | *Same as expr* | Previous row value |
| `FIRST_VALUE(expr)` | *Same as expr* | First value in window |
| `LAST_VALUE(expr)` | *Same as expr* | Last value in window |
| `COUNT(*) OVER` | `int64` | Running/windowed count |
| `SUM(expr) OVER` | `decimal.Decimal` | Running/windowed sum |
| `AVG(expr) OVER` | `decimal.Decimal` | Running/windowed average |
| `MIN(expr) OVER` | *Same as expr* | Running/windowed minimum |
| `MAX(expr) OVER` | *Same as expr* | Running/windowed maximum |

### CTEs (Common Table Expressions)

CTEs are fully supported in both transpiler and interpreter modes:

| CTE Pattern | Support |
|-------------|---------|
| Simple CTE | `WITH Sales AS (...) SELECT ...` |
| Multiple CTEs | `WITH A AS (...), B AS (...) SELECT ...` |
| Recursive CTE | `WITH Hierarchy AS (... UNION ALL ...) SELECT ...` |
| CTE + INSERT | `WITH Source AS (...) INSERT INTO ... SELECT ...` |
| CTE + UPDATE | `WITH Calc AS (...) UPDATE ... FROM ... JOIN Calc` |
| CTE + DELETE | `WITH Dups AS (...) DELETE FROM Dups WHERE ...` |

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
| `XML` | `string` |

### Expressions & Functions

**String functions:** `LEN`, `UPPER`, `LOWER`, `TRIM`, `LTRIM`, `RTRIM`, `SUBSTRING`, `LEFT`, `RIGHT`, `CHARINDEX`, `REPLACE`, `REPLICATE`, `CONCAT`, `CONCAT_WS`, `STRING_AGG`

**Math functions:** `ABS`, `CEILING`, `FLOOR`, `ROUND`, `POWER`, `SQRT`, `SIGN`, `LOG`, `LOG10`, `EXP`

**Date functions:** `GETDATE`, `SYSDATETIME`, `DATEADD`, `DATEDIFF`, `YEAR`, `MONTH`, `DAY`, `DATEPART`, `DATENAME`, `EOMONTH`

**NULL functions:** `ISNULL`, `COALESCE`, `NULLIF`

**Conversion:** `CAST`, `CONVERT`, `TRY_CAST`, `TRY_CONVERT`

**Other:** `CASE` expressions, `IIF`, `CHOOSE`, `ERROR_MESSAGE`

## Examples

### Basic Procedural Logic

Input (`discount.sql`):

```sql
CREATE PROCEDURE dbo.CalculateDiscount
    @Price DECIMAL(10,2),
    @Quantity INT,
    @Total DECIMAL(10,2) OUTPUT
AS
BEGIN
    DECLARE @Discount DECIMAL(10,2) = 0
    SET @Total = @Price * @Quantity
    
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

import "github.com/shopspring/decimal"

func CalculateDiscount(Price decimal.Decimal, Quantity int32) (Total decimal.Decimal) {
    var Discount decimal.Decimal = decimal.NewFromInt(0)
    Total = Price.Mul(decimal.NewFromInt(int64(Quantity)))
    if Quantity >= 100 {
        Discount = Total.Mul(decimal.NewFromFloat(0.15))
    } else if Quantity >= 50 {
        Discount = Total.Mul(decimal.NewFromFloat(0.10))
    }
    Total = Total.Sub(Discount)
    return Total
}
```

### JSON Processing (--dml mode)

Input:

```sql
CREATE PROCEDURE dbo.ParseCustomerJson
    @JsonData NVARCHAR(MAX),
    @CustomerName NVARCHAR(100) OUTPUT,
    @CustomerId INT OUTPUT,
    @Email NVARCHAR(200) OUTPUT
AS
BEGIN
    SET @CustomerName = JSON_VALUE(@JsonData, '$.customer.name')
    SET @CustomerId = CAST(JSON_VALUE(@JsonData, '$.customer.id') AS INT)
    SET @Email = JSON_VALUE(@JsonData, '$.customer.email')
END
```

Output (`tgpiler --dml`):

```go
package main

import "strconv"

func ParseCustomerJson(jsonData string) (customerName string, customerId int32, email string) {
    customerName = JsonValue(jsonData, "$.customer.name")
    customerId = func() int32 { 
        v, _ := strconv.ParseInt(JsonValue(jsonData, "$.customer.id"), 10, 32)
        return int32(v) 
    }()
    email = JsonValue(jsonData, "$.customer.email")
    return customerName, customerId, email
}
```

### XML Validation (--dml mode)

Input:

```sql
CREATE PROCEDURE dbo.ValidateOrderXml
    @XmlData XML,
    @IsValid BIT OUTPUT,
    @HasCustomer BIT OUTPUT
AS
BEGIN
    SET @HasCustomer = @XmlData.exist('/order/customer')
    IF @HasCustomer = 0
        SET @IsValid = 0
    ELSE
        SET @IsValid = 1
END
```

Output (`tgpiler --dml`):

```go
package main

func ValidateOrderXml(xmlData string) (isValid bool, hasCustomer bool) {
    hasCustomer = XmlExist(xmlData, "/order/customer")
    if !hasCustomer {
        isValid = false
    } else {
        isValid = true
    }
    return isValid, hasCustomer
}
```

### Cursor Processing (--dml mode)

Input:

```sql
CREATE PROCEDURE dbo.ProcessUsers
AS
BEGIN
    DECLARE @UserID INT, @Email NVARCHAR(100)
    
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
```

Output (`tgpiler --dml`):

```go
package main

import "fmt"

func ProcessUsers() {
    var userId int32
    var email string
    
    // Cursor becomes idiomatic Go row iteration
    userCursorRows, err := r.db.QueryContext(ctx, 
        "SELECT ID, Email FROM Users WHERE IsActive = 1")
    if err != nil {
        return err
    }
    defer userCursorRows.Close()
    
    for userCursorRows.Next() {
        if err := userCursorRows.Scan(&userId, &email); err != nil {
            return err
        }
        fmt.Println(email)
    }
}
```

### Window Functions with Type Inference (--dml mode)

Input:

```sql
CREATE PROCEDURE dbo.GetSalesRankings
    @CategoryID INT
AS
BEGIN
    SELECT 
        ProductID,
        ProductName,
        Price,
        ROW_NUMBER() OVER (ORDER BY Price DESC) AS PriceRank,
        PERCENT_RANK() OVER (ORDER BY Price) AS PercentileRank,
        SUM(Price) OVER (ORDER BY Price ROWS UNBOUNDED PRECEDING) AS RunningTotal,
        LAG(Price, 1, 0) OVER (ORDER BY Price) AS PreviousPrice
    FROM Products
    WHERE CategoryID = @CategoryID
END
```

Output (`tgpiler --dml`):

```go
package main

import "github.com/shopspring/decimal"

func GetSalesRankings(categoryId int32) error {
    var productId int64
    var productName string
    var price decimal.Decimal
    var priceRank int64           // ROW_NUMBER → int64
    var percentileRank float64    // PERCENT_RANK → float64
    var runningTotal decimal.Decimal  // SUM(Price) → decimal.Decimal
    var previousPrice decimal.Decimal // LAG(Price) → same as Price
    
    rows, err := r.db.QueryContext(ctx, 
        `SELECT ProductID, ProductName, Price,
         ROW_NUMBER() OVER (ORDER BY Price DESC) AS PriceRank,
         PERCENT_RANK() OVER (ORDER BY Price ASC) AS PercentileRank,
         SUM(Price) OVER (ORDER BY Price ASC ROWS UNBOUNDED PRECEDING) AS RunningTotal,
         LAG(Price, 1, 0) OVER (ORDER BY Price ASC) AS PreviousPrice
         FROM Products WHERE CategoryID = $1`, categoryId)
    if err != nil {
        return err
    }
    defer rows.Close()
    
    for rows.Next() {
        if err := rows.Scan(&productId, &productName, &price, 
            &priceRank, &percentileRank, &runningTotal, &previousPrice); err != nil {
            return err
        }
    }
    return nil
}
```

### CTE with Recursive Query (--dml mode)

Input:

```sql
CREATE PROCEDURE dbo.GetEmployeeHierarchy
    @RootEmployeeID INT
AS
BEGIN
    WITH EmployeeCTE AS (
        -- Anchor: root employee
        SELECT EmployeeID, Name, ManagerID, 0 AS Level
        FROM Employees
        WHERE EmployeeID = @RootEmployeeID
        
        UNION ALL
        
        -- Recursive: subordinates
        SELECT e.EmployeeID, e.Name, e.ManagerID, cte.Level + 1
        FROM Employees e
        INNER JOIN EmployeeCTE cte ON e.ManagerID = cte.EmployeeID
    )
    SELECT EmployeeID, Name, Level
    FROM EmployeeCTE
    ORDER BY Level, Name
END
```

Output (`tgpiler --dml`):

```go
package main

func GetEmployeeHierarchy(rootEmployeeId int32) error {
    var employeeId int64
    var name string
    var level int64
    
    rows, err := r.db.QueryContext(ctx, 
        `WITH EmployeeCTE AS (
            SELECT EmployeeID, Name, ManagerID, 0 AS Level
            FROM Employees WHERE EmployeeID = $1
            UNION ALL
            SELECT e.EmployeeID, e.Name, e.ManagerID, (cte.Level + 1)
            FROM Employees AS e
            INNER JOIN EmployeeCTE AS cte ON (e.ManagerID = cte.EmployeeID))
         SELECT EmployeeID, Name, Level FROM EmployeeCTE
         ORDER BY Level ASC, Name ASC`, rootEmployeeId)
    if err != nil {
        return err
    }
    defer rows.Close()
    
    for rows.Next() {
        if err := rows.Scan(&employeeId, &name, &level); err != nil {
            return err
        }
    }
    return nil
}
```

## Project Structure

```
tgpiler/
├── cmd/tgpiler/           # CLI entry point
├── transpiler/            # Core transpilation logic
│   ├── transpiler.go      # Main transpiler, control flow
│   ├── expressions.go     # Expression handling
│   ├── dml.go             # DML statement transpilation
│   ├── types.go           # Type mapping
│   ├── symbols.go         # Symbol table
│   └── comments.go        # Comment preservation
├── tsqlruntime/           # Runtime library
│   ├── json.go            # JSON function implementations
│   ├── xml.go             # XML function implementations
│   ├── ddl.go             # Temp table support
│   └── functions.go       # Built-in function implementations
├── adapter/               # Database adapter patterns
├── storage/               # DML analysis utilities
├── protogen/              # Protocol buffer generation
├── mock/                  # Mock implementations for testing
├── tests/                 # Test suite
│   ├── e2e_test.go        # End-to-end tests
│   ├── structured_test.go # JSON/XML DML tests
│   ├── compilation_test.go
│   ├── basic_test.go
│   ├── nontrivial_test.go
│   └── financial_test.go
├── tsql_basic/            # 20 basic T-SQL samples
├── tsql_nontrivial/       # 15 non-trivial T-SQL samples
├── tsql_financial/        # 20 financial T-SQL samples
├── tsql_structured/       # 25 JSON/XML T-SQL samples
├── tsql_cte/              # 10 CTE and window function samples
├── scripts/               # Convenience scripts
├── Makefile               # Build and test automation
└── README.md
```

## Runtime Library

The `tsqlruntime` package provides both function implementations and a full T-SQL interpreter.

### Functions

```go
import "github.com/ha1tch/tgpiler/tsqlruntime"

// JSON functions
value := tsqlruntime.JSONValue(jsonStr, "$.customer.name")
modified := tsqlruntime.JSONModify(jsonStr, "$.status", "active")

// XML functions  
value := tsqlruntime.XMLValue(xmlStr, "/order/id", tsqlruntime.TypeInt)
exists := tsqlruntime.XMLExist(xmlStr, "/order/customer")

// Temp tables
tempTables := tsqlruntime.NewTempTableManager()
tempTables.CreateTempTable("#Orders", columns)
```

### Interpreter (Dynamic SQL Execution)

The interpreter executes T-SQL at runtime, supporting dynamic SQL and transactions:

```go
import "github.com/ha1tch/tgpiler/tsqlruntime"

// Create interpreter
interp := tsqlruntime.NewInterpreter(db, tsqlruntime.DialectPostgres)

// Set parameters
interp.SetVariable("@userID", 42)
interp.SetVariable("@amount", decimal.NewFromFloat(100.00))

// Execute dynamic SQL with transactions
result, err := interp.Execute(ctx, `
    BEGIN TRANSACTION
    
    DECLARE @balance DECIMAL(18,2)
    SELECT @balance = Balance FROM Accounts WHERE ID = @userID
    
    IF @balance >= @amount
    BEGIN
        UPDATE Accounts SET Balance = Balance - @amount WHERE ID = @userID
        INSERT INTO Transactions (AccountID, Amount, Type) VALUES (@userID, @amount, 'DEBIT')
        COMMIT
    END
    ELSE
    BEGIN
        ROLLBACK
        RAISERROR('Insufficient funds', 16, 1)
    END
`, nil)

// Access results
for _, rs := range result.ResultSets {
    for _, row := range rs.Rows {
        // Process rows
    }
}
```

### Cursors

```go
// Scrollable cursor support
cursor, _ := cursorMgr.DeclareCursor("myCursor", query, false,
    tsqlruntime.CursorStatic, tsqlruntime.CursorScrollForward, tsqlruntime.CursorReadOnly)
cursor.Open(columns, rows)

row, status := cursor.FetchNext()
row, status = cursor.FetchAbsolute(5)   // Jump to row 5
row, status = cursor.FetchRelative(-2)  // Go back 2 rows
row, status = cursor.FetchLast()        // Jump to last row
```

## Makefile Targets

```bash
make help              # Show all available targets
make build             # Build the transpiler
make test              # Run all unit tests
make test-e2e          # Full end-to-end tests
make test-structured   # JSON/XML DML tests
make test-quick        # Quick smoke test
make transpile-all     # Transpile all samples to /tmp
make fmt               # Format code
make lint              # Run go vet
make clean             # Remove build artifacts
```

## Execution Modes

tgpiler supports two execution modes with different capabilities:

### Transpiler Mode (Static Code Generation)

Converts T-SQL to standalone Go code. Use `tgpiler` or `tgpiler --dml`:

| Supported | Not Supported |
|-----------|---------------|
| Procedural logic (IF, WHILE, CASE) | Dynamic SQL (`EXEC(@sql)`) |
| DML (SELECT, INSERT, UPDATE, DELETE) | Transactions (in generated code) |
| Cursors → `rows.Next()` loops | `MERGE` statements |
| CTEs (WITH ... AS) | Linked servers |
| JSON/XML functions | |
| Temp tables (#temp) | |
| Window functions (with type inference) | |
| `EXEC ProcName` (static calls) | |
| `RAISERROR` / `THROW` → errors | |

### Interpreter Mode (Dynamic Execution)

Executes T-SQL at runtime via `tsqlruntime.Interpreter`. Supports everything above plus:

| Feature | Example |
|---------|---------|
| **Dynamic SQL** | `EXEC(@sql)`, `sp_executesql` |
| **Transactions** | `BEGIN TRAN`, `COMMIT`, `ROLLBACK` |
| **Nested transactions** | `@@TRANCOUNT`, `XACT_STATE()` |
| **Full cursor support** | `FETCH ABSOLUTE`, `FETCH RELATIVE`, scrollable cursors |
| **Error handling** | `TRY/CATCH`, `ERROR_NUMBER()`, `ERROR_MESSAGE()` |

```go
// Interpreter example
interp := tsqlruntime.NewInterpreter(db, tsqlruntime.DialectPostgres)
interp.SetVariable("@amount", 100.00)

result, err := interp.Execute(ctx, `
    BEGIN TRAN
    UPDATE Accounts SET Balance = Balance - @amount WHERE ID = 1
    UPDATE Accounts SET Balance = Balance + @amount WHERE ID = 2
    COMMIT
`, nil)
```

## Limitations

The following T-SQL features are not yet supported:

- User-defined functions (UDFs)
- `MERGE` statements
- Linked servers / distributed queries
- `WAITFOR` / Service Broker
- Full-text search

**Note:** CTEs (Common Table Expressions) and window functions (`ROW_NUMBER`, `RANK`, `DENSE_RANK`, `LEAD`, `LAG`, etc.) are fully supported in DML mode — queries are passed through to the database which handles them natively. Window functions have proper type inference: ranking functions return `int64`, percentage functions return `float64`, and navigation functions inherit their argument types.

## Author

Copyright (C) 2025 haitch — h@ual.fi

## Licence

GNU General Public License v3.0

https://github.com/ha1tch/tgpiler?tab=GPL-3.0-1-ov-file#readme

## Related Projects

- [tsqlparser](https://github.com/ha1tch/tsqlparser) - The T-SQL parser used by this project

## SPLogger - Structured Error Logging

The transpiler supports a professional logging system for stored procedure errors. Instead of generating inline error handling code with XML parameter building, you can use SPLogger for clean, structured logging.

### Enabling SPLogger

```bash
tgpiler --dml --splogger input.sql
```

### Before (without SPLogger)

```go
defer func() {
    if _recovered := recover(); _recovered != nil {
        hasError = true
        var parameters string = fmt.Sprintf(`<RootXml>...`, ...)
        result, err := r.db.ExecContext(ctx, "INSERT INTO Error.Log...", ...)
        // ...
    }
}()
```

### After (with SPLogger)

```go
defer func() {
    if _recovered := recover(); _recovered != nil {
        _spErr := tsqlruntime.CaptureError("ProcName", _recovered, params)
        hasError = true
        _ = spLogger.LogError(ctx, _spErr)
    }
}()
```

### SPLogger Implementations

The `tsqlruntime` package provides several logger implementations:

| Logger | Description |
|--------|-------------|
| `DatabaseSPLogger` | Logs to a database table (like original T-SQL pattern) |
| `SlogSPLogger` | Uses Go's `log/slog` structured logging |
| `FileSPLogger` | Logs to a file in JSON or text format |
| `MultiSPLogger` | Logs to multiple destinations |
| `BufferedSPLogger` | Buffers errors for batch insert |
| `NopSPLogger` | No-op logger for testing |

### Example Setup

```go
package main

import (
    "context"
    "database/sql"
    "log/slog"
    "os"
    
    "github.com/ha1tch/tgpiler/tsqlruntime"
)

func main() {
    // Option 1: Log to database (like original T-SQL)
    db, _ := sql.Open("postgres", "...")
    dbLogger := tsqlruntime.NewDatabaseSPLogger(db, "Error.LogForStoreProcedure", "postgres")
    
    // Option 2: Use Go's structured logging
    slogLogger := tsqlruntime.NewSlogSPLogger(slog.Default())
    
    // Option 3: Log to file
    fileLogger, _ := tsqlruntime.NewFileSPLogger("/var/log/sp_errors.json", "json")
    defer fileLogger.Close()
    
    // Option 4: Multiple destinations
    multiLogger := tsqlruntime.NewMultiSPLogger(dbLogger, slogLogger)
    
    // Set as default (used by generated code)
    tsqlruntime.SetDefaultSPLogger(multiLogger)
}
```

### SPError Structure

```go
type SPError struct {
    ProcedureName  string                 // Name of the stored procedure
    Parameters     map[string]interface{} // Input parameters at time of error
    ErrorMessage   string                 // Error message (ERROR_MESSAGE())
    ErrorNumber    int                    // Error number (ERROR_NUMBER())
    Severity       int                    // Error severity (ERROR_SEVERITY())
    State          int                    // Error state (ERROR_STATE())
    Line           int                    // Line number (runtime.Caller)
    Timestamp      time.Time              // When error occurred
    StackTrace     string                 // Go stack trace
}
```

The `SPError` type provides `ToXML()` and `ToJSON()` methods for serialisation.
