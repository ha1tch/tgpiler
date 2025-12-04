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
make build
```

## Dependencies

- [ha1tch/tsqlparser](https://github.com/ha1tch/tsqlparser) - T-SQL parser
- [shopspring/decimal](https://github.com/shopspring/decimal) - Arbitrary-precision decimals (for generated code using DECIMAL/MONEY types)


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
  -p, --pkg <n>      Package name for generated code (default: main)
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

# Read from stdin (explicit flag required)
tgpiler -s < input.sql
cat input.sql | tgpiler -s

# Transpile directory of SQL files
tgpiler -d ./sql -O ./go -p procedures
```

## Testing

The project includes a comprehensive test suite with multiple levels of verification.

### Quick Start

```bash
# Run all tests
make test

# Quick smoke test (~5 key tests)
make test-quick

# End-to-end tests (transpile → compile → execute)
make test-e2e
```

### Test Categories

| Command | Description | Tests |
|---------|-------------|-------|
| `make test` | Run all unit tests | ~285 |
| `make test-e2e` | Full end-to-end tests | 68 |
| `make test-e2e-compile` | Verify all 55 files compile | 56 |
| `make test-e2e-execute` | Execute 12 transpiled functions | 12 |
| `make test-compilation` | Syntax verification (gofmt) | 55 |
| `make test-basic` | Basic algorithm unit tests | ~89 |
| `make test-nontrivial` | Complex algorithm unit tests | ~85 |
| `make test-financial` | Financial calculation unit tests | ~44 |

### End-to-End Testing

The e2e test suite provides full verification that generated code actually works:

1. **Compile All** (`TestE2ECompileAll`): Transpiles all 55 SQL files, writes them to a temp workspace with `go.mod`, and runs `go build` to verify they compile as a package.

2. **Execute Tests** (`TestE2EExecuteBasic`, `TestE2EExecuteFinancial`, `TestE2EExecuteNontrivial`): Transpiles SQL files, wraps functions in a test harness, compiles to a binary, executes, and verifies output matches expected values.

Functions verified by execution:

| Category | Function | Test Input | Expected |
|----------|----------|------------|----------|
| Basic | `AddNumbers` | 10, 20 | 30 |
| Basic | `Factorial` | 5 | 120 |
| Basic | `Gcd` | 48, 18 | 6 |
| Basic | `IsPrime` | 17, 18 | true, false |
| Basic | `Fibonacci` | 10 | 55 |
| Financial | `FutureValue` | $10k, 5%, 10yr | $16,470.09 |
| Financial | `LoanPayment` | $250k, 6.5%, 30yr | $1,580.17/mo |
| Financial | `StraightLineDepreciation` | $10k, $2k salvage, 5yr | $1,600/yr |
| Nontrivial | `LevenshteinDistance` | "kitten", "sitting" | 3 |
| Nontrivial | `ExtendedEuclidean` | 35, 15 | gcd=5, x=1, y=-2 |
| Nontrivial | `CalculateEasterDate` | 2024 | March 31 |
| Nontrivial | `ModularExponentiation` | 2^10 mod 1000 | 24 |

### Scripts

Convenience scripts are available in `scripts/`:

```bash
./scripts/test-all.sh         # Run all tests with summary
./scripts/test-e2e.sh         # End-to-end tests (compile + execute)
./scripts/test-quick.sh       # Quick smoke test
./scripts/test-compilation.sh # Compilation verification only
./scripts/test-financial.sh   # Financial tests only
./scripts/transpile.sh FILE   # Transpile and display output
./scripts/build.sh            # Build the binary
```

### Makefile Targets

```bash
make help              # Show all available targets
make build             # Build the transpiler
make test              # Run all unit tests
make test-e2e          # Full end-to-end tests
make test-e2e-compile  # Verify all files compile
make test-e2e-execute  # Execute selected functions
make test-quick        # Quick smoke test
make test-compilation  # Verify transpilation syntax
make transpile-all     # Transpile all samples to /tmp
make fmt               # Format code
make lint              # Run go vet
make clean             # Remove build artifacts
```

## Sample Files

The project includes 55 T-SQL sample files demonstrating supported features:

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
| 20_order_processing.sql | ProcessOrder | Business logic with validation |

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
| 10_break_even.sql | BreakEvenAnalysis | Break-even point analysis |
| 11_amortization_period.sql | AmortizationPeriod | Per-period loan breakdown |
| 12_irr.sql | InternalRateOfReturn | Newton-Raphson IRR |
| 13_npv.sql | NetPresentValue | NPV with profitability index |
| 14_bond_price.sql | BondPrice | Bond fair value pricing |
| 15_yield_to_maturity.sql | YieldToMaturity | Bisection method YTM |
| 16_cagr.sql | CompoundAnnualGrowthRate | CAGR via Newton-Raphson |
| 17_loan_comparison.sql | LoanComparison | Compare loans with fees |
| 18_sinking_fund.sql | SinkingFund | Required periodic deposits |
| 19_effective_rate_with_fees.sql | EffectiveRateWithFees | True APR with all fees |
| 20_portfolio_return.sql | PortfolioWeightedReturn | Weighted return & Sharpe ratio |

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

### Decimal Handling

The transpiler includes comprehensive support for `decimal.Decimal` operations:

- Arithmetic: `Add`, `Sub`, `Mul`, `Div`, `Mod`
- Comparisons: `LessThan`, `GreaterThan`, `Equal`, `LessThanOrEqual`, `GreaterThanOrEqual`
- Unary minus: `.Neg()` method
- Power: `.Pow()` for decimal exponents
- Math functions: `Abs`, `Ceil`, `Floor`, `Round`
- Type conversions: `IntPart()`, `InexactFloat64()`, `String()`
- Null handling: `decimal.Zero` for NULL assignments

### Expressions & Functions

- Arithmetic operators with proper decimal handling
- Comparison operators (including datetime comparisons via `.Before()`, `.After()`, `.Equal()`)
- Logical operators (`AND` → `&&`, `OR` → `||`, `NOT` → `!`)
- `CAST` / `CONVERT` with proper type coercion
- `CASE` expressions (simple and searched)
- `IIF`
- String functions: `LEN`, `UPPER`, `LOWER`, `TRIM`, `LTRIM`, `RTRIM`, `SUBSTRING`, `LEFT`, `RIGHT`, `CHARINDEX`, `REPLACE`, `REPLICATE`, `CONCAT`, `CONCAT_WS`
- Math functions: `ABS`, `CEILING`, `FLOOR`, `ROUND`, `POWER`, `SQRT`, `SIGN`
- Date functions: `GETDATE`, `DATEADD`, `DATEDIFF`, `YEAR`, `MONTH`, `DAY`, `DATEPART`
- NULL functions: `ISNULL`, `COALESCE`, `NULLIF`
- Error functions: `ERROR_MESSAGE` (in CATCH blocks)

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

## Project Structure

```
tgpiler/
├── cmd/tgpiler/        # CLI entry point
├── transpiler/         # Core transpilation logic
│   ├── transpiler.go   # Main transpiler, control flow
│   ├── expressions.go  # Expression handling, operators
│   ├── types.go        # Type mapping
│   ├── symbols.go      # Symbol table
│   └── comments.go     # Comment preservation
├── tests/              # Test suite
│   ├── e2e_test.go           # End-to-end tests
│   ├── compilation_test.go   # Transpilation verification
│   ├── basic_test.go         # Basic algorithm tests
│   ├── nontrivial_test.go    # Complex algorithm tests
│   └── financial_test.go     # Financial calculation tests
├── tsql_basic/         # 20 basic T-SQL samples
├── tsql_nontrivial/    # 15 non-trivial T-SQL samples
├── tsql_financial/     # 20 financial T-SQL samples
├── scripts/            # Convenience scripts
├── Makefile            # Build and test automation
└── README.md
```

## Recent Changes

### Bug Fixes (Latest)

- **POWER/SQRT return type**: Functions now correctly return `decimal.Decimal` when argument is decimal
- **TRY/CATCH RETURN**: RETURN statements inside TRY blocks now generate correct code for anonymous functions
- **Integer literal recognition**: Negative literals like `-1` are now properly recognised, avoiding unnecessary type promotions
- **String NULL comparisons**: Comparisons between strings and NULL now use `""` instead of `nil`
- **Decimal NULL assignment**: `SET @DecimalVar = NULL` now generates `decimal.Zero` without double-wrapping

### CLI Improvements

- Running `tgpiler` with no arguments now shows help (previously read from stdin)
- Explicit `-s`/`--stdin` flag required for stdin input
- Mutually exclusive input modes are validated

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

## Author

Copyright (C) 2025 haitch

h@ual.fi

## License

GNU GENERAL PUBLIC LICENSE VERSION 3.0

https://github.com/ha1tch/tgpiler?tab=GPL-3.0-1-ov-file#readme

## Disclaimer

This is a proof-of-concept. It is released for educational purposes only, don't use in production.

## Related Projects

- [tsqlparser](https://github.com/ha1tch/tsqlparser) - The T-SQL parser used by this project
