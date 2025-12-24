# tgpiler Improvement Roadmap

## Overview

tgpiler is a T-SQL to Go transpiler designed to help Go developers understand and migrate legacy SQL Server stored procedures. The generated code is a **comprehension accelerator** — it produces readable Go that helps developers understand business logic without learning T-SQL, even if manual cleanup is required.

## Recently Completed (December 2024)

### Core Transpilation
- **NEWID() handling**: 5 modes (app/db/grpc/mock/stub) via `--newid` flag
- **DDL skip logic**: `--skip-ddl` (default), `--extract-ddl`, `--strict-ddl`
- **@@ROWCOUNT support**: Proper declaration and capture from `result.RowsAffected()`
- **CREATE FUNCTION**: Scalar functions transpiled to Go functions
- **GO statement stripping**: Removed by default (use `--preserve-go` to keep)

### gRPC Backend
- **`--backend=grpc`**: Transpile DML statements to gRPC client calls
- **`--backend=mock`**: Transpile DML to mock store calls for testing
- **Temp table fallback**: `--fallback-backend` for hybrid SQL/gRPC generation
- **EXISTS → gRPC**: Existence checks converted to gRPC calls
- **SELECT INTO extraction**: Variable assignment extracts from gRPC responses

### Proto Generation (Four Actions)
- **`--show-mappings`**: Preview procedure-to-method mappings with confidence scores
- **`--gen-impl`**: Generate repository implementations with SQL/procedure mappings
- **`--gen-server`**: Generate gRPC server stubs from proto files
- **`--gen-mock`**: Generate mock server scaffolding
- **`--output-format`**: text, json, markdown, html for mapping reports

### Naming Improvements
- **ALL_CAPS word splitting**: `TRANSFEREVENTNOTE` → `TransferEventNote`
- **Verb-entity collision prevention**: Prevents `TransferTransfer` → `UpdateTransfer`
- **CamelCase preservation**: Through singularize/pluralize operations

### Annotation System (`--annotate` flag)
Four annotation levels for progressive code documentation:

| Level | Flag | What it adds |
|-------|------|--------------|
| none | `--annotate=none` | No extra comments (default) |
| minimal | `--annotate=minimal` | TODO markers for patterns needing attention |
| standard | `--annotate` or `--annotate=standard` | TODOs + `// Original:` showing source T-SQL |
| verbose | `--annotate=verbose` | All + type annotations + section markers |

### Bug Fixes
- **@Var = stripping**: SELECT queries no longer contain T-SQL assignment syntax
- **OBJECT_ID handling**: Converts to `tempTables.Exists("#tableName")`
- **INSERT...SELECT**: Query now includes the SELECT clause
- **TRY/CATCH returns**: Error handling uses `_ = err` pattern
- **Variable scoping**: Nested blocks use `=` not `:=` for existing vars

---

## Short-Term Improvements (1-2 weeks)

## Medium-Term Improvements (1-2 months)

### 1. `--explain` flag: AI-generated business logic summary
Use AI to generate a plain-English explanation of what the procedure does:

```bash
tgpiler --explain procedure.sql
```

Output:
```markdown
## SP_RSL_CreateCancel

**Purpose**: Cancels a money transfer, handling multiple payment networks (PIX, BTS, standard).

**Key Logic**:
1. Validates transfer exists and is in cancellable state
2. Checks network-specific rules
3. Either completes cancellation immediately or puts in "Cancel Waiting" state
4. Sends notifications and processes balance adjustments
```

### 2. MERGE statement support
Full support for T-SQL MERGE with dialect-specific output:
- PostgreSQL: `INSERT ... ON CONFLICT DO UPDATE`
- MySQL: `INSERT ... ON DUPLICATE KEY UPDATE`
- SQL Server: Native MERGE passthrough

### 3. Improved temp table handling
Generate consistent in-memory table operations:
```go
tmpResults := tempTables.Create("#TempResults", columns)
tmpResults.Insert(/* values from SELECT */)
for _, row := range tmpResults.Select(filter) { ... }
```

### 4. Generate helper functions for complex conditionals
Extract massive OR chains into readable helpers:
```go
// Before: 40+ line conditional
// After:
func isImmediateCancelAllowed(status int32, payerId int32, behavior string) bool {
    // Extracted logic with comments
}
```

### 5. Proto generation from SQL
Generate `.proto` files from stored procedure signatures:
```bash
tgpiler --gen-proto --sql-dir ./procedures -o services.proto
```

---

## Long-Term Improvements (3+ months)

### 1. `--review` flag: AI identifies potential issues
Static analysis + AI to flag problems:

```
REVIEW NOTES for SP_RSL_CreateCancel.go:

⚠ Line 126: transferCount = rowsAffected
  - @@ROWCOUNT captured but this SELECT won't affect rows. Did you mean to count results?

⚠ Line 268-276: Complex conditional with 40+ OR clauses
  - Consider extracting to a helper function

⚠ Lines 88-116: Temp table pattern
  - This creates an in-memory table. Ensure tempTables runtime is initialised.
```

### 2. Interactive chat mode
```bash
tgpiler --chat procedure.sql
```

```
> What does the @PaymentTypeBehavior = 'C' check mean?

The PaymentTypeBehavior column indicates how the payment type handles cancellation:
- 'C' = Cancellation allowed with specific rules
- Other values have different cancellation paths
```

### 3. Batch processing with summary report
```bash
tgpiler --batch ./sql/*.sql --report migration-report.html
```

Generate an HTML report showing:
- Procedures processed
- Success/failure counts
- Common patterns detected
- Estimated manual effort per procedure

### 4. Database schema integration
Read database schema to:
- Infer correct Go types from actual column types
- Validate table/column references
- Generate struct types matching tables

---

## AI Integration Architecture

For features using AI (explain, annotate, review, chat):

```
┌─────────────────┐     ┌──────────────┐     ┌─────────────┐
│   T-SQL Input   │────▶│   tgpiler    │────▶│  Go Output  │
└─────────────────┘     └──────────────┘     └─────────────┘
                               │
                               ▼
                        ┌──────────────┐
                        │  AI Context  │
                        │  - T-SQL src │
                        │  - Go output │
                        │  - Schema    │
                        └──────────────┘
                               │
                               ▼
                        ┌──────────────┐
                        │  OpenAI API  │
                        └──────────────┘
                               │
                               ▼
                        ┌──────────────┐
                        │ Explanation  │
                        │ Annotations  │
                        │ Review Notes │
                        └──────────────┘
```

**Configuration**:
```bash
export OPENAI_API_KEY=sk-...
tgpiler --explain --model gpt-4o procedure.sql
```

**Rate limiting**: Cache explanations, batch requests, use cheaper models for non-critical tasks.

---

## Success Metrics

The goal is to reduce time-to-understanding for Go developers:

| Metric | Before tgpiler | After (current) | Target |
|--------|----------------|-----------------|--------|
| Time to understand a complex procedure | 4-6 hours | 1-2 hours | 30 mins |
| Time to produce working Go | 3-5 hours | 2-3 hours | 1 hour |
| Procedures migrated per day | 1-2 | 3-5 | 10+ |

---

## Contributing

Contributions welcome! Priority areas:
1. Additional T-SQL function mappings
2. Test cases for edge cases
3. Documentation improvements
4. AI prompt engineering for better explanations
