# UUID Generation and DDL Handling

This document describes tgpiler's handling of UUID generation (NEWID()) and DDL statements (CREATE SEQUENCE, CREATE VIEW, etc.).

## Current State

### NEWID()
- **Status**: ✓ Fully implemented with 5 modes
- **Default**: `--newid=app` (uuid.New().String())

### CREATE SEQUENCE / DDL
- **Status**: ✓ Skip with warning by default
- **Extract**: `--extract-ddl=FILE` collects DDL for migration scripts

## CLI Options

```
NEWID handling:
  --newid=MODE          How to generate UUIDs (default: app)
                        app   - uuid.New() in Go (recommended)
                        db    - Database-specific UUID function
                        grpc  - Call gRPC ID service (requires --id-service)
                        stub  - Generate TODO placeholder
                        mock  - Predictable IDs for testing

  --id-service=CLIENT   gRPC client variable for --newid=grpc
                        Example: --id-service=idClient

DDL Handling:
  --skip-ddl            Skip DDL statements with warning (default: true)
  --strict-ddl          Fail on any DDL statement
  --extract-ddl=FILE    Collect skipped DDL into separate file
```

## Test Results

### Mock UUID Mode (--newid=mock)
```
=== Mock UUID Test ===
uuid1: 00000000-0000-0000-0000-000000000001
uuid2: 00000000-0000-0000-0000-000000000002
code:  00000000

✓ All UUIDs are sequential and predictable
✓ Reset works correctly
```

### App UUID Mode (--newid=app)
```
=== App-side UUID Test ===
uuid1: e8cda5da-0802-4f53-aada-2c57b9d1fa40
uuid2: 78da20ce-d39e-4404-8b00-e3ea233fcb8c
code:  46A29B3B

✓ App-side UUID mode generates valid, unique UUIDs
```

### Extract DDL (--extract-ddl)
```sql
-- DDL statements extracted by tgpiler
-- These should be kept in your database schema/migrations

CREATE SEQUENCE TransferNumberSeq START WITH 1 INCREMENT BY 1;
GO
```

## MoneySend Results

| File | Before | After |
|------|--------|-------|
| 001_schema.sql | ✗ DDL | ✗ (pure schema) |
| 002_procedures_customer.sql | ✓ | ✓ 1,263 lines |
| 003_procedures_kyc_beneficiary.sql | ✓ | ✓ 1,641 lines |
| 004_procedures_quotes_transfers.sql | ✗ SEQUENCE | ✓ 1,287 lines |
| 005_procedures_transfer_status_payment.sql | ✗ NEWID | ✓ 1,502 lines |
| 006_procedures_compliance_aml.sql | ✗ SEQUENCE | ✓ 1,091 lines |
| 007_procedures_partners_settlements.sql | ✓ | ✓ 1,048 lines |
| 008_procedures_ledger_reconciliation.sql | ✗ SEQUENCE | ✓ 849 lines |
| 009_procedures_notifications.sql | ✓ | ✓ 501 lines |
| 010_procedures_reporting.sql | ✓ | ✓ 674 lines |
| 011_procedures_agents.sql | ✓ | ✓ 645 lines |
| 012_procedures_config_promos.sql | ✓ | ✓ 883 lines |
| **Total** | **7/12** | **11/12** |

## Generated Code Examples

### --newid=app (default)
```go
var id string = uuid.New().String()
var code string = strings.ToUpper((uuid.New().String())[:(8)])
```

### --newid=db (postgres)
```go
var id string = func() string { 
    var id string
    r.db.QueryRowContext(ctx, "SELECT gen_random_uuid()::text").Scan(&id)
    return id 
}()
```

### --newid=mock
```go
var id string = tsqlruntime.NextMockUUID()
// Generates: 00000000-0000-0000-0000-000000000001, 000...002, etc.
```

### --newid=grpc
```go
var id string = idClient.GenerateUUID(ctx)
```

### --newid=stub
```go
var id string = "" /* TODO: implement NEWID() */
```

## Mock UUID API (tsqlruntime package)

```go
// NextMockUUID generates sequential UUIDs for testing
func NextMockUUID() string

// ResetMockUUID resets counter to 0
func ResetMockUUID()

// SetMockUUID sets counter to specific value
func SetMockUUID(value uint64)

// GetMockUUIDCounter returns current counter
func GetMockUUIDCounter() uint64
```
