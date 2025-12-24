# MoneySend - International Remittance Platform

MoneySend is a comprehensive example for testing tgpiler, representing a production-realistic international money transfer company. It is significantly more complex than the ShopEasy e-commerce example, featuring multi-party transactions, regulatory compliance as a core concern, complex state machines, and partner network management.

## Domain Complexity

Unlike e-commerce (which has relatively simple order states), remittance involves:

- **Multi-party transactions**: Sender → MoneySend → Payout Partner → Recipient
- **Regulatory compliance**: KYC/AML is mandatory, not optional
- **Complex state machines**: Transfers have 12+ states vs ~5 for orders
- **Multi-currency with real-time FX**: Rate locking, spreads, margin calculation
- **Risk assessment**: Integrated into every flow, not peripheral
- **Partner network management**: Settlement, reconciliation, credit limits

## Statistics

| Metric | Count |
|--------|-------|
| Database Tables | 28 |
| Stored Procedures | 222 |
| Proto Services | 10 |
| Proto RPC Methods | ~150 |

## Directory Structure

```
moneysend/
├── sql/
│   ├── 001_schema.sql                    # Database schema (28 tables)
│   ├── 002_procedures_customer.sql       # Customer management (22 procs)
│   ├── 003_procedures_kyc_beneficiary.sql # KYC & beneficiaries (30 procs)
│   ├── 004_procedures_quotes_transfers.sql # Quotes & transfers (19 procs)
│   ├── 005_procedures_transfer_status_payment.sql # Status & payment (24 procs)
│   ├── 006_procedures_compliance_aml.sql  # Compliance & AML (18 procs)
│   ├── 007_procedures_partners_settlements.sql # Partners (24 procs)
│   ├── 008_procedures_ledger_reconciliation.sql # Ledger (19 procs)
│   ├── 009_procedures_notifications.sql   # Notifications (17 procs)
│   ├── 010_procedures_reporting.sql       # Reporting (16 procs)
│   ├── 011_procedures_agents.sql          # Staff management (15 procs)
│   └── 012_procedures_config_promos.sql   # Config & promos (18 procs)
├── proto/
│   ├── customer.proto      # Customer service
│   ├── beneficiary.proto   # Beneficiary service
│   ├── transfer.proto      # Transfer lifecycle
│   ├── compliance.proto    # AML/KYC compliance
│   ├── kyc.proto           # Document verification
│   ├── partner.proto       # Payout partners
│   ├── notification.proto  # Communications
│   ├── reporting.proto     # Analytics & reports
│   ├── agent.proto         # Staff management
│   └── config.proto        # System configuration
└── docs/
    └── architecture.md     # System architecture
```

## Services Overview

### CustomerService
Customer registration, authentication, profile management, verification tiers, and limit management.

**Key procedures:**
- `usp_RegisterCustomer` - New customer registration with country validation
- `usp_AuthenticateCustomer` - Login with lockout protection
- `usp_UpgradeCustomerTier` - Tier upgrades with automatic limit increases
- `usp_GetCustomerRiskProfile` - Comprehensive risk view

### BeneficiaryService
Recipient management including bank accounts, mobile wallets, and compliance screening.

**Key procedures:**
- `usp_CreateBeneficiary` - Add recipient with country validation
- `usp_AddBeneficiaryBankAccount` - Bank account with routing details
- `usp_ScreenBeneficiary` - Sanctions/PEP screening
- `usp_BlockBeneficiary` / `usp_UnblockBeneficiary` - Compliance actions

### TransferService
Core transfer lifecycle from quote through completion.

**Key procedures:**
- `usp_GetTransferQuote` - Real-time quote with FX and fees
- `usp_InitiateTransfer` - Create transfer with limit validation
- `usp_ConfirmPayment` - Payment confirmation
- `usp_CompleteTransfer` - Final completion
- `usp_CancelTransfer` / `usp_RefundTransfer` - Reversal flows

### ComplianceService
AML screening, risk assessment, and regulatory reporting.

**Key procedures:**
- `usp_ScreenTransfer` - Transaction screening
- `usp_AssessCustomerRisk` - Risk scoring
- `usp_CreateSAR` / `usp_FileSAR` - Suspicious Activity Reports
- `usp_ApproveTransfer` / `usp_BlockTransfer` - Compliance decisions

### PartnerService
Payout partner management, settlements, and reconciliation.

**Key procedures:**
- `usp_CreatePayoutPartner` - Partner onboarding
- `usp_CreateSettlement` - Settlement creation
- `usp_ReconcileSettlement` - Settlement reconciliation
- `usp_GetPartnerBalance` - Credit tracking

### NotificationService
Multi-channel communications (email, SMS, push).

**Key procedures:**
- `usp_CreateNotificationFromTemplate` - Templated notifications
- `usp_SendTransferCompletedNotification` - Transfer events
- `usp_MarkNotificationDelivered` - Delivery tracking

### ReportingService
Analytics, dashboards, and business intelligence.

**Key procedures:**
- `usp_GetDailyTransferSummary` - Daily metrics
- `usp_GetCorridorPerformanceReport` - Corridor analysis
- `usp_GetComplianceDashboard` - Compliance overview
- `usp_GetCustomerLifetimeValueReport` - CLV analysis

### AgentService
Internal staff management and activity logging.

**Key procedures:**
- `usp_CreateAgent` - Staff onboarding with role-based permissions
- `usp_CheckAgentPermission` - Permission verification
- `usp_GetAgentPerformanceStats` - Staff metrics

### ConfigService
System configuration and country settings.

**Key procedures:**
- `usp_SetConfigValue` - Configuration management
- `usp_UpdateCountryConfiguration` - Country rules
- `usp_EnableCountryForSending` / `usp_DisableCountryForSending`

## Transfer State Machine

```
CREATED
    │
    ▼
PENDING_PAYMENT ──────────────────┐
    │                             │
    ▼                             │
PAYMENT_RECEIVED                  │
    │                             │
    ▼                             │
COMPLIANCE_REVIEW ───► ON_HOLD    │
    │         │                   │
    │         ▼                   │
    │      BLOCKED ──► CANCELLED ◄┤
    │                             │
    ▼                             │
PROCESSING                        │
    │                             │
    ▼                             │
SENT_TO_PARTNER                   │
    │                             │
    ▼                             │
PAYOUT_PENDING                    │
    │         │                   │
    ▼         ▼                   │
COMPLETED   FAILED ──► REFUNDED ◄─┘
```

## Key Business Logic Patterns

### 1. Limit Validation (usp_InitiateTransfer)
```sql
-- Check daily limit
SELECT @TodayTotal = COALESCE(SUM(SendAmount), 0) 
FROM Transfers
WHERE CustomerId = @CustomerId 
AND CAST(CreatedAt AS DATE) = CAST(SYSUTCDATETIME() AS DATE)
AND Status NOT IN ('CANCELLED', 'FAILED', 'REFUNDED');

IF @TodayTotal + @SendAmount > @DailyLimit
BEGIN
    SELECT 0 AS Success, 'EXCEEDS_DAILY_LIMIT' AS ErrorCode;
    RETURN;
END
```

### 2. Rate Locking (usp_InitiateTransfer)
```sql
-- Rate validity (15 minutes)
DECLARE @RateLockUntil DATETIME2 = DATEADD(MINUTE, 15, SYSUTCDATETIME());

-- Get current rate
SELECT @ExchangeRate = SellRate, @MidMarketRate = MidMarketRate
FROM FXRates WHERE CorridorId = @CorridorId AND EffectiveTo IS NULL;
```

### 3. Tiered Fee Calculation (usp_CalculateFees)
```sql
SELECT TOP 1 @TransferFee = 
    CASE FeeType
        WHEN 'FLAT' THEN FlatFee
        WHEN 'PERCENTAGE' THEN @SendAmount * PercentageFee
        ELSE COALESCE(FlatFee, 0) + (@SendAmount * COALESCE(PercentageFee, 0))
    END
FROM FeeSchedules
WHERE CorridorId = @CorridorId AND PayoutMethod = @PayoutMethod
AND (MinAmount IS NULL OR @SendAmount >= MinAmount)
AND (MaxAmount IS NULL OR @SendAmount <= MaxAmount);
```

### 4. Risk Assessment (usp_AssessCustomerRisk)
Multi-factor risk scoring combining country risk, transaction patterns, behaviour analysis, and document verification status.

### 5. Double-Entry Ledger (usp_CreateLedgerEntry)
All financial movements recorded with balanced debit/credit entries for audit and reconciliation.

## Verb Coverage for tgpiler

This example exercises a rich set of verbs beyond basic CRUD:

| Category | Verbs |
|----------|-------|
| **Lifecycle** | Initiate, Process, Complete, Cancel, Fail, Refund |
| **Compliance** | Screen, Review, Approve, Block, Flag, Freeze, Unfreeze |
| **Verification** | Verify, Validate, Authenticate, Confirm |
| **State** | Hold, Release, Suspend, Reactivate, Lock, Unlock |
| **Calculation** | Calculate, Assess, Score |
| **Batch** | Generate, Reconcile, Settle |

## Testing with tgpiler

This example is designed to stress test:

1. **Complex state transitions** - 12+ transfer states vs simple CRUD
2. **Multi-table transactions** - Transfers touch 5+ tables atomically
3. **Rich validation logic** - Limits, eligibility, compliance checks
4. **Calculation procedures** - Fees, FX, risk scores
5. **Audit patterns** - History tables, ledger entries
6. **Batch operations** - Settlements, reconciliation, bulk generation

## Sample Corridors

| Code | Route | Payout Methods |
|------|-------|----------------|
| US_MX | USA → Mexico | Bank, Cash, Wallet |
| US_PH | USA → Philippines | Bank, Wallet (GCash) |
| US_IN | USA → India | Bank (IFSC), Wallet |
| UK_PK | UK → Pakistan | Bank, Cash |
| CA_VN | Canada → Vietnam | Bank |

## Compliance Features

- **KYC Tiers**: Basic (Tier 1), Enhanced (Tier 2), Full (Tier 3)
- **Sanctions Screening**: OFAC, UN, EU lists
- **PEP Screening**: Politically exposed persons
- **Transaction Monitoring**: Velocity checks, pattern analysis
- **SAR Filing**: Suspicious Activity Report workflow
- **Document Verification**: ID, proof of address, source of funds
