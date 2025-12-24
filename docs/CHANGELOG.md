# Changelog

All notable changes to tgpiler are documented here.

## [Unreleased] - December 2024

### Added

#### gRPC DML Transpilation
- **`--backend=grpc`**: Transpile DML statements to gRPC client calls
- **`--backend=mock`**: Transpile DML statements to mock store calls for testing
- **`--grpc-package`**: Specify the proto package for generated gRPC code
- **`--grpc-client`**: Specify the gRPC client variable name (default: `client`)
- **`--mock-store`**: Specify the mock store variable name (default: `store`)

#### Temp Table Fallback
- **`--fallback-backend`**: Specify fallback backend for temp table operations in gRPC/mock modes
- Temp tables (`#tableName`) automatically use SQL backend when `--backend=grpc` is specified
- Informational warnings when temp tables detected without explicit fallback backend

#### DML-to-gRPC Improvements
- **EXISTS → gRPC**: `EXISTS (SELECT ... FROM Table WHERE ...)` converts to gRPC existence checks
- **SELECT INTO response extraction**: `SELECT @var = col FROM ...` extracts values from gRPC responses
- **Verb detection**: Infers gRPC method verbs from DML patterns (e.g., UPDATE with ApprovalStatus → `ApproveOrder`)

#### Naming Convention Improvements
- **ALL_CAPS word splitting**: `TRANSFEREVENTNOTE` → `TransferEventNote` using domain word dictionary
- **Verb-entity collision prevention**: Prevents `TransferTransfer` → generates `UpdateTransfer` instead
- **CamelCase preservation**: `OrderStatusHistory` preserved through singularize/pluralize operations

#### NEWID() Handling
- **`--newid=app`** (default): Generate `uuid.New().String()` in Go
- **`--newid=db`**: Use database UUID function (`gen_random_uuid()` for Postgres)
- **`--newid=grpc`**: Call gRPC ID service via `--id-service` client
- **`--newid=mock`**: Sequential predictable UUIDs for testing
- **`--newid=stub`**: Generate TODO placeholder

#### DDL Handling
- **`--skip-ddl`** (default): Skip DDL statements with helpful warnings
- **`--extract-ddl=FILE`**: Collect skipped DDL into separate migration file
- **`--strict-ddl`**: Fail on any DDL statement
- Helpful hints for DDL-only files suggesting migration tools

#### Proto Generation
- **`--gen-server`**: Generate gRPC server stubs from proto files
- **`--gen-impl`**: Generate repository implementations with procedure mappings
- **`--gen-mock`**: Generate mock server scaffolding
- **`--show-mappings`**: Display procedure-to-method mappings with confidence scores
- **`--output-format`**: Output format for mappings (text, json, markdown, html)
- **`--warn-threshold`**: Confidence threshold for low-confidence warnings (default: 50%)

#### Annotation System
- **`--annotate`**: Add code annotations at various levels
  - `none`: No extra comments (default)
  - `minimal`: TODO markers for patterns needing attention
  - `standard`: TODOs + original T-SQL as comments
  - `verbose`: All + type annotations + section markers

#### SPLogger Integration
- **`--splogger`**: Enable SPLogger for CATCH block error logging
- **`--logger-type`**: Logger type (slog, db, file, multi, nop)
- **`--logger-table`**: Table name for database logger
- **`--logger-file`**: File path for file logger
- **`--logger-init`**: Generate logger initialisation code

### Fixed

- **@@ROWCOUNT**: Properly declares `rowsAffected int32` when used
- **@Var = stripping**: SELECT queries no longer contain T-SQL assignment syntax
- **OBJECT_ID handling**: `OBJECT_ID('tempdb..#tableName')` → `tempTables.Exists("#tableName")`
- **INSERT...SELECT**: Query now includes the SELECT clause
- **TRY/CATCH returns**: Error handling uses `_ = err` pattern
- **GO statement handling**: Stripped by default (use `--preserve-go` to keep)
- **Variable scoping**: Nested blocks use `=` not `:=` for existing variables
- **Temp table names in gRPC**: `#tmpTable` no longer generates invalid method names

### Improved

- **Result set mapping**: Populated Scan() calls with proto field matching
- **SQL dialect consistency**: Correct placeholders and functions per dialect
- **Code cleanliness**: Removed unnecessary `_ = varName` statements
- **Helpful error messages**: Hints and workarounds for unsupported constructs
- **Unmapped procedure reporting**: Shows procedures without matching RPC methods

---

## Test Results

### MoneySend Example (222 procedures, 191 RPC methods)

| Action | Result |
|--------|--------|
| `--show-mappings` | 191/191 methods mapped (100%) |
| `--gen-impl` | 4,811 lines, 157/191 methods with SQL (82%) |
| `--gen-server` | 1,365 lines |
| `--gen-mock` | 229 lines |
| All backends compile | ✓ |
| gofmt passes | ✓ |

### File Transpilation

| Backend | MoneySend | ShopEasy |
|---------|-----------|----------|
| SQL (all dialects) | 12/12 | 6/6 |
| gRPC | 12/12 | 6/6 |
| Mock | 12/12 | 6/6 |

### Real-World Test: SP_RSL_CreateCancel.sql

- 561-line production stored procedure
- Temp tables, nested procedures, complex conditionals
- SQL backend: 352 lines, gofmt PASS
- gRPC backend: 506 lines, gofmt PASS (with temp table fallback)
- Mock backend: 356 lines, gofmt PASS

---

## Migration Guide

### From SQL-only to gRPC Backend

```bash
# Before: SQL backend only
tgpiler --dml --dialect=postgres input.sql

# After: gRPC backend with fallback for temp tables
tgpiler --dml --backend=grpc --grpc-package=mypb --fallback-backend=sql input.sql
```

### Using the Four Actions

```bash
# 1. Preview mappings
tgpiler --show-mappings --proto-dir ./protos --sql-dir ./procedures

# 2. Generate implementations
tgpiler --gen-impl --proto-dir ./protos --sql-dir ./procedures -o repo.go

# 3. Generate server stubs
tgpiler --gen-server --proto-dir ./protos -o server.go

# 4. Generate mocks
tgpiler --gen-mock --proto-dir ./protos -o mocks.go
```
