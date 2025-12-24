# tgpiler Improvement Plan

> **Status: COMPLETED** — All items in this plan have been implemented. See [IMPROVEMENT_TRACKER.md](./IMPROVEMENT_TRACKER.md) for implementation details and [CHANGELOG.md](./CHANGELOG.md) for the complete list of changes.

Based on testing against the MoneySend example (222 procedures, 10 services, 191 RPC methods), this document outlined required improvements organised by priority. All items have been completed as of December 2024.

---

## Executive Summary

**Guiding principle:** Do as much as we can to help the developer. Every error should suggest a fix. Every output should be usable without manual repair. The tool should handle real-world messiness, not demand pristine input.

The parser and core translator are mature. The gaps are in:
1. Output formatting for developer consumption
2. Code completeness (generated code doesn't compile)
3. Missing T-SQL constructs (sequences, GO statements)
4. SQL dialect consistency

---

## Priority 1: Structural Completeness

**Problem:** Generated functions are orphaned bodies — no receiver, no context, variable scoping errors. Code doesn't compile.

**Observed in:** Actions 2, 3, 4

**Current output:**
```go
func UspInitiateTransfer(customerId int64, ...) (err error) {
    tx, err := r.db.BeginTx(ctx, nil)  // r and ctx undefined
    ...
}
```

**Required output:**
```go
func (r *Repository) UspInitiateTransfer(ctx context.Context, customerId int64, ...) (*InitiateTransferResult, error) {
    tx, err := r.db.BeginTx(ctx, nil)
    ...
}
```

**Changes:**
- Add `--receiver <name>` flag (default: `r`)
- Add `--receiver-type <type>` flag (default: `*Repository`)
- Always inject `ctx context.Context` as first parameter
- Generate result struct when procedure returns data via SELECT
- Fix variable shadowing in nested blocks (use `=` not `:=` for existing vars)

---

## Priority 2: GO Statement Handling

**Problem:** Parser rejects `GO` batch separators, which have no semantic meaning for transpilation.

**Observed in:** Action 2

**Current behaviour:**
```
error: unsupported statement type: *ast.GoStatement
```

**Required behaviour:** Strip `GO` silently by default.

**Changes:**
- Default: Remove `GO` statements before parsing
- Add `--preserve-go` flag for edge cases where user wants parse errors at batch boundaries
- No warning needed — `GO` is a client tool artifact, not T-SQL

---

## Priority 2b: Helpful Error Messages

**Problem:** When parser fails, error messages state what failed but don't suggest fixes.

**Observed in:** Action 2, Action 4

**Current behaviour:**
```
error: unsupported statement type: *ast.GoStatement
error: unsupported expression type: *ast.NextValueForExpression
```

**Required behaviour:** Suggest workarounds or relevant flags.

**Changes:**
- When encountering unsupported constructs, append suggestions:
  ```
  error: unsupported statement type: *ast.GoStatement
        Hint: GO is a batch separator with no semantic meaning.
        This will be stripped automatically in a future version.
        Workaround: Remove GO statements from input.
  ```
- For sequences (until supported):
  ```
  error: unsupported expression type: *ast.NextValueForExpression
        Hint: NEXT VALUE FOR sequences are not yet supported.
        Workaround: Replace with placeholder value and implement sequence logic in Go.
  ```
- Pattern: Every "unsupported X" error should have a "Hint:" with either a workaround or "This will be supported in version N"

---

## Priority 3: Sequence Support

**Problem:** Parser rejects `NEXT VALUE FOR` expressions.

**Observed in:** Action 4

**Current behaviour:**
```
error: unsupported expression type: *ast.NextValueForExpression
```

**Constructs to support:**
- `NEXT VALUE FOR <sequence>`
- `SCOPE_IDENTITY()`
- `@@IDENTITY`
- `IDENTITY(type, seed, increment)` in table definitions

**Changes:**
- Add AST node for sequence expressions
- Code generator options:
  - `--sequence-mode=db` — Use `RETURNING id` (Postgres) or `LAST_INSERT_ID()` (MySQL)
  - `--sequence-mode=uuid` — Generate `uuid.New()` application-side
  - `--sequence-mode=stub` — Generate `// TODO: implement sequence` placeholder
- Map `SCOPE_IDENTITY()` to `result.LastInsertId()` when following INSERT

---

## Priority 4: Mapping Output Formats

**Problem:** `--show-mappings` dumps 191 lines of text with no filtering or structure.

**Observed in:** Action 1

**Current behaviour:** Wall of text, developer must grep to find problems.

**Required behaviour:** Structured output for tooling and human review.

**Changes:**
- Add `--output-format` flag with values:
  - `text` (default) — Current behaviour
  - `json` — Machine-readable for tooling
  - `markdown` — Table format for documentation
  - `html` — Interactive report with charts and filtering

**JSON schema:**
```json
{
  "services": [...],
  "mappings": [
    {
      "rpc": "ConfirmPayment",
      "procedure": "usp_ConfirmPaymentReceived", 
      "confidence": 0.36,
      "signals": ["naming:substring", "params:2/5", "verb:Confirm~Confirm"],
      "warnings": ["Low confidence - manual review recommended"]
    }
  ],
  "statistics": {
    "total": 191,
    "high_confidence": 106,
    "medium_confidence": 65,
    "low_confidence": 20
  },
  "unmapped_procedures": ["usp_InternalHelper", ...]
}
```

**HTML report features:**
- Confidence distribution chart (pie or bar)
- Sortable/filterable table
- Colour coding: green (>80%), yellow (50-80%), red (<50%)
- Expandable rows showing matching signals
- List of unmapped procedures
- Export to CSV

---

## Priority 5: --gen-impl File Structure

**Problem:** Single-file output contains 10 `package main` declarations.

**Observed in:** Action 3

**Current behaviour:** Syntax errors from concatenated package headers.

**Changes:**
- When `-o <file>` specified: Consolidate to single package declaration, merge imports
- When `-O <dir>` specified: One file per service (`customer_repo.go`, `transfer_repo.go`, etc.)
- Add `--split-by-service` flag to force per-service files even with `-o`

---

## Priority 6: Result Set Mapping

**Problem:** Generated Scan() calls are empty; result structs are unpopulated.

**Observed in:** Actions 3, 4

**Current output:**
```go
err := row.Scan()  // Empty!
return &RegisterCustomerResponse{}, nil  // Empty struct
```

**Required output:**
```go
var result RegisterCustomerResponse
err := row.Scan(&result.Success, &result.CustomerId, &result.ExternalId)
if err != nil {
    return nil, fmt.Errorf("RegisterCustomer: %w", err)
}
return &result, nil
```

**Changes:**
- Parse procedure's final SELECT to extract column names
- Match columns to proto response fields (case-insensitive, underscore-tolerant)
- Generate Scan() with correct field references
- Warn when columns don't map to response fields

---

## Priority 7: SQL Dialect Consistency

**Problem:** Generated queries mix Postgres placeholders (`$1`) with SQL Server functions (`SYSUTCDATETIME()`).

**Observed in:** Action 4

**Changes:**
- `--dialect` flag already exists; enforce it consistently:
  - `postgres`: `$1`, `NOW()`, `COALESCE`
  - `mysql`: `?`, `NOW()`, `IFNULL`  
  - `sqlserver`: `@p1`, `SYSUTCDATETIME()`, `ISNULL`
- Add `--dialect=passthrough` to preserve original SQL (for SQL Server → SQL Server migrations)

---

## Priority 8: Code Cleanliness

**Problem:** Generated code has unnecessary noise.

**Observed in:** Actions 2, 4

**Issues:**
- `_ = varName` after every declaration
- Excessive parentheses in expressions
- Verbose string concatenation

**Changes:**
- Remove `_ = varName` — only add if variable genuinely unused at end of function
- Simplify expression parenthesisation
- Use `fmt.Sprintf` patterns for complex string building:
  ```go
  // Instead of:
  transferNumber = ((("MS-" + fmt.Sprintf("%v", year)) + "-") + ...)
  // Generate:
  transferNumber = fmt.Sprintf("MS-%d-%06d", time.Now().UTC().Year(), seq)
  ```

---

## Priority 9: Actionable Warnings

**Problem:** Low-confidence mappings don't suggest fixes.

**Observed in:** Action 1

**Current:**
```
HoldTransfer -> usp_BlockTransfer (30% confidence)
```

**Required:**
```
WARNING: HoldTransfer -> usp_BlockTransfer (30% confidence)
         Verb mismatch: Hold ≠ Block
         Candidate alternative: usp_FlagTransferForInvestigation (pattern match)
         To override: --grpc-mappings="HoldTransfer:usp_FlagTransferForInvestigation"
```

**Changes:**
- Include top 2-3 alternative candidates when confidence < 50%
- Show the exact CLI flag to override
- Group warnings at end of output for visibility

---

## Priority 10: Unmapped Procedure Reporting

**Problem:** No visibility into procedures that exist but aren't mapped to any RPC.

**Observed in:** Action 1 (implicitly — 222 procedures, 191 RPCs, difference unexplained)

**Changes:**
- Add "Unmapped Procedures" section to `--show-mappings` output
- Categorise: likely internal helpers vs potential missing RPCs
- Suggest: "Consider adding RPC for usp_RecordLedgerEntry (called by 3 other procedures)"

---

## Implementation Order

| Phase | Items | Effort | Impact |
|-------|-------|--------|--------|
| 1 | GO stripping, Structural completeness, Helpful errors | Medium | Code compiles, developers unblocked |
| 2 | Sequence support, Result set mapping | Medium | Code works |
| 3 | Output formats (JSON/MD/HTML) | Medium | Developer productivity |
| 4 | File structure, Dialect consistency | Low | Polish |
| 5 | Code cleanliness, Actionable warnings, Unmapped reporting | Low | Quality of life |

---

## Success Criteria

After implementing this plan, MoneySend should:
1. Parse without errors (no manual GO stripping)
2. Generate compilable Go code from any procedure
3. Produce `--show-mappings` HTML report usable for migration planning
4. Generate `--gen-impl` output that compiles and has populated Scan() calls
5. Clearly report the 31 procedures not mapped to RPCs

The goal: A developer runs tgpiler against their legacy codebase and gets *working scaffolding*, not a sketch that requires significant manual repair.
