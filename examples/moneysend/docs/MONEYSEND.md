# MoneySend: A Deliberately Imperfect Example

## Why Include Errors?

MoneySend contains intentional inconsistencies, naming drift, and structural issues. This is by design.

**The problem with pristine examples:** A perfectly consistent codebase only tests the happy path. It proves a tool works in ideal conditions that don't exist in production. Any parser or mapper can handle code that follows every convention perfectly.

**The reality of enterprise code:** Real stored procedure libraries are accumulated over years by developers who never met each other, following style guides that evolved (or were ignored), with copy-paste inheritance from Stack Overflow, and "temporary" workarounds that became permanent. Documentation describes intent from 2009. Comments lie. Parameters exist because removing them would break something nobody can identify.

**What imperfect examples test:**
- Parser resilience to syntactic variations
- Error message quality when things don't align
- Fuzzy matching capability for naming drift
- Graceful degradation vs catastrophic failure
- Whether warnings are actionable or cryptic

A tool that only works on perfect input is a demonstration. A tool that handles messy reality is production-ready.

MoneySend is the latter kind of test.

---

## Catalogued Issues & Discordances

This section documents known issues, inconsistencies, and mismatches. These are **intentionally preserved** as they represent realistic enterprise code patterns that tgpiler must handle gracefully.

## 1. Duplicate Procedure Definitions

Two procedures are defined in multiple files:

| Procedure | Files |
|-----------|-------|
| `usp_GetCorridorPerformanceReport` | 008_procedures_ledger_reconciliation.sql, 010_procedures_reporting.sql |
| `usp_GetDailyTransferSummary` | 005_procedures_transfer_status_payment.sql, 010_procedures_reporting.sql |

**Impact:** SQL Server would fail on second CREATE. tgpiler should detect and warn.

**Real-world cause:** Different developers added similar functionality to different modules without checking for existing implementations.

---

## 2. Dead Code: Unused Function

`fn_GenerateTransferNumber()` is defined but never called:

```sql
-- Defined at 004_procedures_quotes_transfers.sql:575
CREATE FUNCTION fn_GenerateTransferNumber()

-- But usp_InitiateTransfer duplicates the logic inline at line 795-796:
SET @Seq = NEXT VALUE FOR TransferNumberSeq;
SET @TransferNumber = 'MS-' + CAST(YEAR(SYSUTCDATETIME()) AS NVARCHAR) + '-' + ...
```

**Impact:** Code duplication, maintenance risk.

**Real-world cause:** Function was written first, then someone didn't know it existed and reimplemented inline.

---

## 3. Sequence Definition Order Errors

Sequences are used BEFORE they are defined in the same file:

| Sequence | Used at | Defined at | File |
|----------|---------|------------|------|
| `TransferNumberSeq` | Line 584 | Line 591 | 004_procedures_quotes_transfers.sql |
| `JournalIdSeq` | Line 137 | Line 154 | 008_procedures_ledger_reconciliation.sql |
| `SARNumberSeq` | (check) | Line 422 | 006_procedures_compliance_aml.sql |

**Impact:** Procedures would fail to create because sequence doesn't exist yet.

**Real-world cause:** Code was added incrementally, dependency order wasn't verified.

---

## 4. Sequences in Wrong Location

Three sequences are defined in procedure files instead of schema:

- `TransferNumberSeq` — 004_procedures_quotes_transfers.sql
- `SARNumberSeq` — 006_procedures_compliance_aml.sql  
- `JournalIdSeq` — 008_procedures_ledger_reconciliation.sql

**Impact:** Architectural inconsistency. Schema file should contain all database objects.

**Real-world cause:** Developer added sequence alongside the procedure that uses it for convenience.

---

## 5. Proto ↔ Procedure Naming Mismatches

| Proto RPC | Actual Procedure | Discrepancy |
|-----------|------------------|-------------|
| `ConfirmPayment` | `usp_ConfirmPaymentReceived` | Suffix mismatch |
| `ApproveTransfer` | `usp_ClearTransferForProcessing` | Completely different verb |
| `SendToPartner` | `usp_SendToPayoutPartner` | Extra qualifier |
| `HoldTransfer` | `usp_FlagTransferForInvestigation` | Different verb |
| `RefundTransfer` | `usp_ProcessRefund` | Verb swap |
| `ProcessTransfer` | `usp_ClearTransferForProcessing` | Overloaded procedure |

**Impact:** Mapper must use fuzzy matching or explicit configuration.

**Real-world cause:** Proto designed by API team, procedures by database team, naming conventions diverged.

---

## 6. Inconsistent Error Return Patterns

Two different patterns for error returns:

**Pattern A (75 occurrences):** ErrorCode only
```sql
SELECT 0 AS Success, 'CUSTOMER_NOT_FOUND' AS ErrorCode;
```

**Pattern B (23 occurrences):** ErrorCode + ErrorMessage
```sql
SELECT 0 AS Success, 'DUPLICATE_BENEFICIARY' AS ErrorCode,
       'A beneficiary with this name already exists' AS ErrorMessage;
```

**Impact:** Proto response messages expect `error_message` but not all procedures provide it.

**Real-world cause:** Style evolved over time, no refactoring to standardise.

---

## 7. Inconsistent Parameter Naming

Same semantic concept, different parameter names:

| Concept | Variations Used |
|---------|-----------------|
| Who performed action | `@ClearedBy`, `@ApprovedBy`, `@ReviewedBy`, `@UpdatedBy`, `@PerformedBy` |
| Notes/Comments | `@Notes`, `@Reason`, `@Description`, `@Comments` |
| Status change | `@NewStatus`, `@Status`, `@TargetStatus` |

**Impact:** Proto message field names can't directly map to procedure parameters.

**Real-world cause:** Multiple developers, no enforced naming convention.

---

## 8. Proto Field ↔ Procedure Return Mismatches

Example from `CreateBeneficiaryResponse`:

Proto expects:
```protobuf
message CreateBeneficiaryResponse {
  bool success = 1;
  string error_code = 2;
  string error_message = 3;  // ← Not always returned
  int64 beneficiary_id = 4;
  string external_id = 5;
}
```

Procedure returns (on error):
```sql
SELECT 0 AS Success, 'CUSTOMER_NOT_ACTIVE' AS ErrorCode;
-- No ErrorMessage column
```

**Impact:** Generated code must handle missing columns gracefully.

---

## 9. No Seed Data

Schema and procedures exist, but no reference data for:
- Corridors
- Exchange rates
- Fee schedules
- Countries
- Notification templates

**Impact:** Cannot execute end-to-end without setup scripts.

---

## 10. Untested SQL

All 222 procedures are syntactically plausible but have never been executed. Potential issues:
- Column names that don't match schema
- JOIN conditions on wrong columns
- Type mismatches in expressions
- Logic errors in business rules

**Impact:** Unknown runtime failures.

---

## Summary Statistics

| Issue Category | Count |
|----------------|-------|
| Duplicate procedures | 2 |
| Dead code (unused functions) | 1 |
| Sequence order errors | 3 |
| Misplaced sequences | 3 |
| Proto naming mismatches | 6+ |
| Error pattern inconsistencies | 98 total (75 + 23) |
| Parameter naming variations | 10+ |

---

## Value as Test Cases

These issues make MoneySend a more valuable test suite than a pristine example:

1. **Parser resilience** — Can tgpiler handle the variations?
2. **Error quality** — Does it report issues clearly?
3. **Fuzzy mapping** — Can it suggest matches despite naming drift?
4. **Real-world readiness** — Enterprise code is never consistent

A tool that only works on perfect input is a toy. A tool that handles messy reality is production-ready.
