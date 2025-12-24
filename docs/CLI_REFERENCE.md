# CLI Reference

Complete command-line reference for tgpiler.

## Synopsis

```
tgpiler [options] <input.sql>
tgpiler [options] -s < input.sql
tgpiler [options] -d <path>
tgpiler --gen-server --proto <file> [options]
tgpiler --gen-impl --proto-dir <path> --sql-dir <path> [options]
tgpiler --show-mappings --proto-dir <path> --sql-dir <path> [options]
```

## Input Options

| Flag | Description |
|------|-------------|
| `<file.sql>` | Read single SQL file |
| `-s, --stdin` | Read from stdin |
| `-d, --dir <path>` | Read all .sql files from directory |

## Output Options

| Flag | Description |
|------|-------------|
| (none) | Write to stdout |
| `-o, --output <file>` | Write to single file |
| `-O, --outdir <path>` | Write to directory (creates if needed) |
| `-f, --force` | Allow overwriting existing files |

## General Options

| Flag | Default | Description |
|------|---------|-------------|
| `-p, --pkg <name>` | `main` | Package name for generated code |
| `--dml` | off | Enable DML mode (SELECT, INSERT, UPDATE, DELETE, JSON/XML) |
| `--dialect <name>` | `postgres` | SQL dialect: `postgres`, `mysql`, `sqlite`, `sqlserver` |
| `--store <var>` | `r.db` | Store/database variable name |
| `--receiver <var>` | `r` | Receiver variable name (empty for standalone functions) |
| `--receiver-type <type>` | `*Repository` | Receiver type |
| `--preserve-go` | off | Don't strip GO batch separators |
| `-h, --help` | | Show help |
| `-v, --version` | | Show version |

## Backend Options

Requires `--dml`.

| Flag | Default | Description |
|------|---------|-------------|
| `--backend <type>` | `sql` | Backend type: `sql`, `grpc`, `mock`, `inline` |
| `--fallback-backend <type>` | `sql` | Backend for temp table operations: `sql`, `mock` |
| `--grpc-client <var>` | `client` | gRPC client variable name |
| `--grpc-package <path>` | (none) | Import path for generated gRPC package |
| `--mock-store <var>` | `store` | Mock store variable name |

### Backend Types

| Backend | Description | Generated Code |
|---------|-------------|----------------|
| `sql` | Standard database/sql | `r.db.QueryContext(ctx, "SELECT ...")` |
| `grpc` | gRPC client calls | `client.GetOrder(ctx, &pb.GetOrderRequest{})` |
| `mock` | Mock store for testing | `store.Select("Orders", filter)` |
| `inline` | Embedded SQL strings | `query := "SELECT ..."` |

## gRPC Mapping Options

Requires `--dml --backend=grpc`.

| Flag | Format | Description |
|------|--------|-------------|
| `--table-service <map>` | `Table:Service,...` | Map tables to owning services |
| `--table-client <map>` | `Table:clientVar,...` | Map tables to client variable names |
| `--grpc-mappings <map>` | `proc:Service.Method,...` | Explicit procedure-to-method mappings |

**Example:**
```bash
tgpiler --dml --backend=grpc --grpc-package=catalogpb \
  --table-service="Products:CatalogService,Orders:OrderService" \
  --table-client="Products:catalogClient,Orders:orderClient" \
  input.sql
```

## Proto Generation Options

Mutually exclusive with transpilation.

| Flag | Description |
|------|-------------|
| `--proto <file>` | Single proto file |
| `--proto-dir <path>` | Directory of proto files |
| `--sql-dir <path>` | Directory of SQL procedure files (for mapping) |
| `--service <name>` | Target specific service (default: all) |
| `--gen-server` | Generate gRPC server stubs |
| `--gen-impl` | Generate repository implementations with procedure mappings |
| `--gen-mock` | Generate mock server scaffolding |
| `--show-mappings` | Display procedure-to-method mappings |
| `--output-format <fmt>` | Output format for `--show-mappings`: `text`, `json`, `markdown`, `html` |
| `--warn-threshold <n>` | Confidence threshold (0-100) for low-confidence warnings (default: 50) |

## NEWID() Handling

| Flag | Default | Description |
|------|---------|-------------|
| `--newid <mode>` | `app` | How to generate UUIDs |
| `--id-service <var>` | (none) | gRPC client variable for `--newid=grpc` |

### NEWID Modes

| Mode | Description | Generated Code |
|------|-------------|----------------|
| `app` | Application-side UUID | `uuid.New().String()` |
| `db` | Database-side UUID | `gen_random_uuid()` (Postgres) |
| `grpc` | Call gRPC ID service | `idClient.GenerateUUID(ctx)` |
| `mock` | Sequential predictable UUIDs | `tsqlruntime.NextMockUUID()` |
| `stub` | TODO placeholder | `"" /* TODO: implement NEWID() */` |

## DDL Handling

| Flag | Default | Description |
|------|---------|-------------|
| `--skip-ddl` | on | Skip DDL statements with warning |
| `--strict-ddl` | off | Fail on any DDL statement |
| `--extract-ddl <file>` | (none) | Collect skipped DDL into separate file |

## Annotation Options

| Flag | Default | Description |
|------|---------|-------------|
| `--annotate[=level]` | (none) | Add code annotations |

### Annotation Levels

| Level | Description |
|-------|-------------|
| `none` | No extra comments |
| `minimal` | TODO markers for patterns needing attention |
| `standard` | TODOs + original T-SQL as comments |
| `verbose` | All + type annotations + section markers |

**Examples:**
```bash
# Standard annotations (default when flag present)
tgpiler --dml --annotate input.sql

# Verbose annotations
tgpiler --dml --annotate=verbose input.sql
```

## SPLogger Options

Requires `--dml`.

| Flag | Default | Description |
|------|---------|-------------|
| `--splogger` | off | Enable SPLogger for CATCH block error logging |
| `--logger <var>` | `spLogger` | SPLogger variable name |
| `--logger-type <type>` | `slog` | Logger type: `slog`, `db`, `file`, `multi`, `nop` |
| `--logger-table <name>` | `Error.LogForStoreProcedure` | Table name for db logger |
| `--logger-file <path>` | (none) | File path for file logger |
| `--logger-format <fmt>` | `json` | Format for file logger: `json`, `text` |
| `--logger-init` | off | Generate SPLogger initialisation code |

## Sequence Handling

| Flag | Default | Description |
|------|---------|-------------|
| `--sequence-mode <mode>` | `db` | How to handle NEXT VALUE FOR |

### Sequence Modes

| Mode | Description |
|------|-------------|
| `db` | Use database RETURNING/LAST_INSERT_ID |
| `uuid` | Generate `uuid.New()` application-side |
| `stub` | Generate TODO placeholder |

## Examples

### Basic Transpilation

```bash
# Transpile to stdout
tgpiler input.sql

# Transpile with DML to file
tgpiler --dml input.sql -o output.go

# Transpile directory
tgpiler --dml -d ./sql -O ./go -p mypackage
```

### Backend Selection

```bash
# SQL backend (default)
tgpiler --dml --dialect=postgres input.sql

# gRPC backend
tgpiler --dml --backend=grpc --grpc-package=orderpb input.sql

# Mock backend for testing
tgpiler --dml --backend=mock input.sql

# gRPC with temp table fallback (automatic)
tgpiler --dml --backend=grpc --grpc-package=orderpb input.sql
# Output: info: Temp tables detected. Using --fallback-backend=sql (default).
```

### Proto Generation

```bash
# Preview mappings
tgpiler --show-mappings --proto-dir ./protos --sql-dir ./procedures

# Generate HTML mapping report
tgpiler --show-mappings --proto-dir ./protos --sql-dir ./procedures --output-format=html -o mappings.html

# Generate repository implementations
tgpiler --gen-impl --proto-dir ./protos --sql-dir ./procedures -o repo.go

# Generate server stubs
tgpiler --gen-server --proto-dir ./protos -o server.go

# Generate mocks
tgpiler --gen-mock --proto-dir ./protos -o mocks.go
```

### Advanced Options

```bash
# With annotations
tgpiler --dml --annotate=verbose input.sql

# Extract DDL to migration file
tgpiler --dml --extract-ddl=migrations.sql input.sql

# Mock UUIDs for testing
tgpiler --dml --newid=mock input.sql

# With SPLogger
tgpiler --dml --splogger --logger-type=multi input.sql
```

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Error (parse error, file not found, etc.) |

## Environment Variables

tgpiler does not currently use environment variables.

## See Also

- [QUICKSTART_EN.md](QUICKSTART_EN.md) — Get started in 5 minutes
- [QUICKSTART_ES.md](QUICKSTART_ES.md) — Guía de inicio rápido (Español)
- [QUICKSTART_PT.md](QUICKSTART_PT.md) — Guia de início rápido (Português)
- [DML.md](DML.md) — Database operations and dialects
- [GRPC.md](GRPC.md) — gRPC backend and proto generation
- [MANUAL.md](MANUAL.md) — Complete user manual
