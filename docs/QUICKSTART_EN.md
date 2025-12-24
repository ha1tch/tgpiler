# Quick Start Guide

Get tgpiler running in 5 minutes.

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

## Basic Usage

### Transpile a Stored Procedure

```bash
# Simple procedural logic
tgpiler input.sql -o output.go

# With database operations (SELECT, INSERT, UPDATE, DELETE)
tgpiler --dml input.sql -o output.go

# Specify target database dialect
tgpiler --dml --dialect=postgres input.sql -o output.go
```

### Example

**Input (T-SQL):**
```sql
CREATE PROCEDURE dbo.GetCustomerOrders
    @CustomerID INT,
    @Status VARCHAR(20) = 'Active'
AS
BEGIN
    SET NOCOUNT ON
    
    SELECT OrderID, Amount, OrderDate
    FROM Orders
    WHERE CustomerID = @CustomerID AND Status = @Status
    ORDER BY OrderDate DESC
END
```

**Output (Go):**
```go
func (r *Repository) GetCustomerOrders(ctx context.Context, customerID int32, status string) error {
    rows, err := r.db.QueryContext(ctx,
        "SELECT OrderID, Amount, OrderDate FROM Orders WHERE CustomerID = $1 AND Status = $2 ORDER BY OrderDate DESC",
        customerID, status)
    if err != nil {
        return err
    }
    defer rows.Close()
    
    for rows.Next() {
        var orderID int64
        var amount decimal.Decimal
        var orderDate time.Time
        if err := rows.Scan(&orderID, &amount, &orderDate); err != nil {
            return err
        }
        // Process row...
    }
    return rows.Err()
}
```

## Backend Selection

```bash
# SQL backend (default) - generates database/sql calls
tgpiler --dml --backend=sql input.sql

# gRPC backend - generates gRPC client calls
tgpiler --dml --backend=grpc --grpc-package=orderpb input.sql

# Mock backend - generates mock store calls (for testing)
tgpiler --dml --backend=mock input.sql
```

## The Four Actions (Proto-based Development)

If you have `.proto` files defining your gRPC services:

```bash
# 1. Preview mappings between procedures and RPC methods
tgpiler --show-mappings --proto-dir ./protos --sql-dir ./procedures

# 2. Generate repository implementations
tgpiler --gen-impl --proto-dir ./protos --sql-dir ./procedures -o repo.go

# 3. Generate gRPC server stubs
tgpiler --gen-server --proto-dir ./protos -o server.go

# 4. Generate mock server scaffolding
tgpiler --gen-mock --proto-dir ./protos -o mocks.go
```

## Common Options

| Flag | Description |
|------|-------------|
| `--dml` | Enable DML mode (SELECT, INSERT, UPDATE, DELETE) |
| `--dialect` | Target dialect: `postgres`, `mysql`, `sqlite`, `sqlserver` |
| `--backend` | Output backend: `sql`, `grpc`, `mock`, `inline` |
| `-p, --pkg` | Package name for generated code |
| `-o, --output` | Output file |
| `-f, --force` | Overwrite existing files |
| `--annotate` | Add code annotations (none, minimal, standard, verbose) |

## Handling Special Cases

### NEWID() (UUID Generation)

```bash
# Application-side UUID (default)
tgpiler --dml --newid=app input.sql

# Database-side UUID
tgpiler --dml --newid=db input.sql

# Mock UUIDs for testing
tgpiler --dml --newid=mock input.sql
```

### DDL Statements

```bash
# Skip DDL with warnings (default)
tgpiler --dml input.sql

# Extract DDL to separate file
tgpiler --dml --extract-ddl=migrations.sql input.sql

# Fail on DDL
tgpiler --dml --strict-ddl input.sql
```

### Temp Tables with gRPC

```bash
# Temp tables automatically fall back to SQL
tgpiler --dml --backend=grpc --grpc-package=orderpb input.sql
# Shows: info: Temp tables detected. Using --fallback-backend=sql (default).
```

## Next Steps

- [MANUAL.md](MANUAL.md) — Complete reference documentation
- [DML.md](DML.md) — Database operations and dialects
- [GRPC.md](GRPC.md) — gRPC backend and proto generation
- [CHANGELOG.md](CHANGELOG.md) — Recent changes and features

## Examples

The `examples/` directory contains complete working examples:

```
examples/
├── moneysend/      # Financial services (222 procedures, 10 services)
│   ├── proto/      # gRPC service definitions
│   ├── sql/        # T-SQL stored procedures
│   └── generated/  # Generated Go code
└── shopeasy/       # E-commerce (6 services)
    ├── protos/
    ├── procedures/
    └── generated/
```

Try them:

```bash
cd examples/moneysend

# View all procedure-to-method mappings
tgpiler --show-mappings --proto-dir proto --sql-dir sql

# Generate implementations
tgpiler --gen-impl --proto-dir proto --sql-dir sql -o repo.go
```
