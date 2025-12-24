# tgpiler Improvement Tracker

Tracks implementation status of items from [IMPROVEMENT_PLAN.md](./IMPROVEMENT_PLAN.md).

Last updated: 2024-12-24

---

## Status Legend

| Status | Meaning |
|--------|---------|
| Pending | Not started |
| Ongoing | In progress |
| Done | Completed and verified |

---

## Tracker

| # | Priority | Description | Status |
|---|----------|-------------|--------|
| 1 | P1 | Structural completeness (receiver, context, scoping) | Done |
| 2 | P2 | GO statement stripping (default on, --preserve-go flag) | Done |
| 3 | P2b | Helpful error messages with hints and workarounds | Done |
| 4 | P3 | Sequence support (NEXT VALUE FOR, SCOPE_IDENTITY, etc.) | Done |
| 5 | P4 | Mapping output formats (json/markdown/html) | Done |
| 6 | P5 | --gen-impl file structure (single package, split by service) | Done |
| 7 | P6 | Result set mapping (populate Scan() calls) | Done |
| 8 | P7 | SQL dialect consistency (placeholders, functions) | Done |
| 9 | P8 | Code cleanliness (remove noise, simplify expressions) | Done |
| 10 | P9 | Actionable warnings (alternatives, CLI override flags) | Done |
| 11 | P10 | Unmapped procedure reporting | Done |

---

## Phase Summary

| Phase | Items | Status |
|-------|-------|--------|
| 1 | #1, #2, #3 | Done |
| 2 | #4, #7 | Done |
| 3 | #5 | Done |
| 4 | #6, #8 | Done |
| 5 | #9, #10, #11 | Done |

---

## Change Log

| Date | Item | Change |
|------|------|--------|
| 2024-12-23 | — | Tracker created |
| 2024-12-23 | #1 (P1) | Done: Added --receiver, --receiver-type flags; ctx context.Context injected as first param; context import auto-added; verified compilation with MoneySend |
| 2024-12-23 | #2 (P2) | Done: GO statements stripped by default; added --preserve-go flag; regex pattern handles all whitespace variants |
| 2024-12-23 | #3 (P2b) | Done: Helpful error messages with hints for CREATE FUNCTION, USE, CREATE VIEW, ALTER, DROP, etc. |
| 2024-12-23 | #4 (P3) | Done: NEXT VALUE FOR, SCOPE_IDENTITY(), @@IDENTITY, @@ROWCOUNT; --sequence-mode flag (db/uuid/stub); dialect-aware code generation |
| 2024-12-24 | #7 (P6) | Done: Result set extraction fixed for SELECT without FROM; uses last result set for success case; handles nested message types in proto responses |
| 2024-12-24 | #6 (P5) | Done: --gen-impl now produces single package header with merged imports for all services; added GenerateAllServicesImpl method |
| 2024-12-24 | #5 (P4) | Done: --output-format flag (text/json/markdown/html) for --show-mappings; JSON with services/mappings/stats; Markdown tables with icons; HTML interactive report with filtering and pie chart |
| 2024-12-24 | #8 (P7) | Done: Dialect-aware query generation in --gen-impl; CALL + $n for postgres, EXEC + @pn for sqlserver, CALL + ? for mysql/sqlite |
| 2024-12-24 | #11 (P10) | Done: Unmapped procedure reporting added to text output; shows procedures with no matching RPC method |
| 2024-12-24 | #10 (P9) | Done: Actionable warnings for low-confidence mappings; shows alternatives from unmapped procs; --warn-threshold flag (default 50%); override syntax provided |
| 2024-12-24 | #9 (P8) | Done: Proper variable usage tracking; only emit _ = varName for genuinely unused variables; removed automatic blank assignments from DECLARE |
| 2024-12-24 | CREATE FUNCTION | Done: Scalar functions with BEGIN/END body now transpiled to Go functions; function calls via dbo.fn_Name() resolved to Go function calls |
| 2024-12-24 | @@ROWCOUNT support | Done: var rowsAffected declared when @@ROWCOUNT used; proper int32 type; captured from result.RowsAffected() |
| 2024-12-24 | Bug fixes from SP_RSL_CreateCancel | Done: ctx in scope; rowsAffected declared; SELECT INTO variable works; temp table # handled |
| 2024-12-24 | NEWID() handling | Done: 5 modes (app/db/grpc/mock/stub); --newid flag; --id-service for gRPC mode |
| 2024-12-24 | DDL skip logic | Done: --skip-ddl (default), --extract-ddl, --strict-ddl; helpful hints for DDL-only files |
| 2024-12-24 | SELECT INTO response extraction | Done: SELECT @var = col generates response value extraction for gRPC backend |
| 2024-12-24 | EXISTS → gRPC | Done: EXISTS subqueries convert to gRPC existence checks; falls back to SQL for temp tables |
| 2024-12-24 | Temp table fallback | Done: --fallback-backend flag; temp tables automatically use SQL in gRPC/mock modes; informational warnings |
| 2024-12-24 | Complex WHERE handling | Done: Complex expressions (DATEADD, function calls) emit warning comments instead of invalid gRPC |
| 2024-12-24 | Top-level DDL skip | Done: IF NOT EXISTS around CREATE SEQUENCE generates helpful migration hint |
| 2024-12-24 | Verb-entity collision | Done: Prevents TransferTransfer → UpdateTransfer; checks all inference functions |
| 2024-12-24 | ALL_CAPS word splitting | Done: TRANSFEREVENTNOTE → TransferEventNote; domain word dictionary; greedy matching |
| 2024-12-24 | CamelCase preservation | Done: singularize/pluralize preserve original casing |
| 2024-12-24 | Pluralize fix | Done: Already-plural words (ending in 's') not double-pluralized |

---

## Additional Features (Beyond Original Plan)

| Feature | Status | Description |
|---------|--------|-------------|
| gRPC DML backend | Done | `--backend=grpc` transpiles DML to gRPC calls |
| Mock DML backend | Done | `--backend=mock` transpiles DML to mock store calls |
| Four actions workflow | Done | show-mappings, gen-impl, gen-server, gen-mock |
| Ensemble mapper | Done | Multi-strategy procedure-to-method matching |
| Verb detection | Done | Infers verbs from column names and values |
| SPLogger integration | Done | Structured error logging for CATCH blocks |
| Annotation system | Done | Four levels of code documentation |
