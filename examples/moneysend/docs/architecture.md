# MoneySend Architecture

## System Overview

MoneySend is designed as a microservices-oriented remittance platform where business logic resides in SQL Server stored procedures. This architecture document describes the system design, data flows, and integration patterns.

## High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              Client Applications                              │
├─────────────┬─────────────┬─────────────┬─────────────┬─────────────────────┤
│  Web App    │  iOS App    │ Android App │  Agent UI   │   Partner Portal    │
└──────┬──────┴──────┬──────┴──────┬──────┴──────┬──────┴──────────┬──────────┘
       │             │             │             │                 │
       └─────────────┴─────────────┴─────────────┴─────────────────┘
                                   │
                            ┌──────▼──────┐
                            │  API Gateway │
                            │   (gRPC)     │
                            └──────┬──────┘
                                   │
       ┌───────────────────────────┼───────────────────────────┐
       │                           │                           │
┌──────▼──────┐  ┌────────────────▼────────────────┐  ┌───────▼───────┐
│  Customer   │  │         Transfer                │  │  Compliance   │
│  Service    │  │         Service                 │  │   Service     │
└──────┬──────┘  └────────────────┬────────────────┘  └───────┬───────┘
       │                          │                           │
       │         ┌────────────────┼────────────────┐          │
       │         │                │                │          │
┌──────▼──────┐  │  ┌─────────────▼─────────────┐  │  ┌───────▼───────┐
│ Beneficiary │  │  │      Partner Service      │  │  │   Reporting   │
│   Service   │  │  └─────────────┬─────────────┘  │  │   Service     │
└──────┬──────┘  │                │                │  └───────┬───────┘
       │         │                │                │          │
       └─────────┴────────────────┴────────────────┴──────────┘
                                  │
                           ┌──────▼──────┐
                           │  SQL Server │
                           │  (Business  │
                           │   Logic)    │
                           └─────────────┘
```

## Service Boundaries

### CustomerService
**Responsibility:** Customer identity and account management

**Owns tables:**
- Customers
- CustomerDocuments
- CustomerVerificationHistory
- CustomerFundingSources

**Key integrations:**
- ComplianceService (risk assessment, KYC)
- NotificationService (account notifications)

### BeneficiaryService
**Responsibility:** Recipient management and payout destinations

**Owns tables:**
- Beneficiaries
- BeneficiaryBankAccounts
- BeneficiaryMobileWallets

**Key integrations:**
- ComplianceService (screening)
- TransferService (payout destination)

### TransferService
**Responsibility:** Transfer lifecycle management

**Owns tables:**
- Transfers
- TransferStatusHistory
- PaymentTransactions
- FXRates
- Corridors
- FeeSchedules

**Key integrations:**
- All services (central orchestrator)

### ComplianceService
**Responsibility:** AML/KYC compliance and regulatory reporting

**Owns tables:**
- ComplianceScreenings
- SuspiciousActivityReports
- CustomerRiskAssessments

**Key integrations:**
- External screening providers (Refinitiv, Dow Jones)
- Regulatory filing systems (FinCEN)

### PartnerService
**Responsibility:** Payout partner network management

**Owns tables:**
- PayoutPartners
- CorridorPartners
- PartnerSettlements

**Key integrations:**
- External partner APIs
- TransferService (payout routing)

### NotificationService
**Responsibility:** Multi-channel communications

**Owns tables:**
- Notifications
- NotificationTemplates

**Key integrations:**
- Email provider (SendGrid, SES)
- SMS provider (Twilio)
- Push notification service

### ReportingService
**Responsibility:** Analytics and business intelligence

**Owns tables:**
- DailyReconciliation

**Key integrations:**
- All services (read-only analytics)
- Data warehouse (batch exports)

### AgentService
**Responsibility:** Internal staff and operations

**Owns tables:**
- Agents
- AgentActivityLog

**Key integrations:**
- All services (audit logging)

### ConfigService
**Responsibility:** System configuration

**Owns tables:**
- SystemConfiguration
- CountryConfiguration
- PromoCodes
- PromoCodeUsage

## Data Flow: Transfer Lifecycle

### 1. Quote Generation
```
Client                TransferService              Database
  │                        │                          │
  │──GetTransferQuote────►│                          │
  │                        │──usp_GetTransferQuote──►│
  │                        │                          │
  │                        │◄──Quote + Rate + Fees───│
  │◄──QuoteResponse───────│                          │
```

### 2. Transfer Initiation
```
Client          TransferService       ComplianceService      Database
  │                  │                       │                  │
  │──InitiateTransfer►                       │                  │
  │                  │──usp_InitiateTransfer─────────────────►│
  │                  │  (validates limits, locks rate)         │
  │                  │◄─────────────TransferId────────────────│
  │                  │                       │                  │
  │                  │──ScreenTransfer──────►│                  │
  │                  │                       │──usp_ScreenTransfer►
  │                  │◄──ScreeningResult─────│◄─────────────────│
  │◄──TransferCreated│                       │                  │
```

### 3. Payment Confirmation
```
PaymentGateway    TransferService                    Database
     │                  │                               │
     │──PaymentWebhook─►│                               │
     │                  │──usp_ConfirmPayment─────────►│
     │                  │──usp_UpdateTransferStatus───►│
     │                  │                               │
     │                  │  (if compliance cleared)      │
     │                  │──usp_ProcessTransfer────────►│
     │◄──Acknowledged───│                               │
```

### 4. Partner Payout
```
TransferService      PartnerService       ExternalPartner     Database
     │                     │                    │                │
     │──SendToPartner─────►│                    │                │
     │                     │──API Call─────────►│                │
     │                     │◄──PartnerRef──────│                │
     │                     │──usp_SendToPartner────────────────►│
     │◄──SentConfirmation──│                    │                │
     │                     │                    │                │
     │    (async callback) │◄──PayoutComplete──│                │
     │                     │──usp_CompleteTransfer─────────────►│
```

## Compliance Flow

### Real-time Screening
```
                    ┌─────────────────┐
                    │ Customer/Benef/ │
                    │    Transfer     │
                    └────────┬────────┘
                             │
                    ┌────────▼────────┐
                    │   Pre-Screen    │
                    │   Validation    │
                    └────────┬────────┘
                             │
              ┌──────────────┴──────────────┐
              │                             │
     ┌────────▼────────┐          ┌────────▼────────┐
     │  Sanctions List │          │   PEP Database  │
     │    (OFAC, UN)   │          │                 │
     └────────┬────────┘          └────────┬────────┘
              │                             │
              └──────────────┬──────────────┘
                             │
                    ┌────────▼────────┐
                    │  Match Scoring  │
                    │   (0-100)       │
                    └────────┬────────┘
                             │
           ┌─────────────────┼─────────────────┐
           │                 │                 │
    ┌──────▼──────┐   ┌──────▼──────┐   ┌──────▼──────┐
    │   CLEAR     │   │  POTENTIAL  │   │   MATCH     │
    │  (< 30)     │   │   (30-70)   │   │   (> 70)    │
    └──────┬──────┘   └──────┬──────┘   └──────┬──────┘
           │                 │                 │
           │          ┌──────▼──────┐          │
           │          │   Manual    │          │
           │          │   Review    │          │
           │          └──────┬──────┘          │
           │                 │                 │
           │     ┌───────────┴───────────┐     │
           │     │                       │     │
    ┌──────▼─────▼──────┐       ┌────────▼─────▼────┐
    │    APPROVED       │       │     BLOCKED       │
    │  (Continue flow)  │       │  (Reject + SAR?)  │
    └───────────────────┘       └───────────────────┘
```

## Risk Assessment Model

### Customer Risk Factors

| Factor | Weight | Data Source |
|--------|--------|-------------|
| Country Risk | 25% | CountryConfiguration.RiskLevel |
| Transaction Pattern | 25% | Transfer history analysis |
| Behaviour Score | 25% | Login patterns, device changes |
| Document Status | 25% | KYC verification status |

### Transaction Risk Factors

| Factor | Trigger |
|--------|---------|
| High Value | > $3,000 single transaction |
| Velocity | > 3 transfers in 24 hours |
| New Beneficiary | First transfer to recipient |
| High-Risk Corridor | Destination in HIGH/PROHIBITED list |
| Unusual Pattern | Deviation from customer norm |

## Double-Entry Ledger

All financial movements are recorded with balanced entries:

```
Transfer Initiated:
  DR  Customer Receivable (Asset)     $1,000
  CR  Transfer Payable (Liability)    $1,000

Payment Received:
  DR  Cash/Bank (Asset)               $1,004.99
  CR  Customer Receivable (Asset)     $1,000.00
  CR  Fee Revenue (Revenue)           $4.99

Payout Executed:
  DR  Transfer Payable (Liability)    $1,000
  CR  Partner Payable (Liability)     $1,000

Settlement:
  DR  Partner Payable (Liability)     $1,000
  CR  Cash/Bank (Asset)               $1,000
```

## Partner Integration Patterns

### Synchronous API (Real-time payout)
```
MoneySend ──POST /transfers──► Partner
          ◄──TransferResponse──
```

### Asynchronous with Callback
```
MoneySend ──POST /transfers──────────────────► Partner
          ◄──Accepted (pending)──
          
Partner   ──POST /webhooks/payout-complete──► MoneySend
          ◄──Acknowledged──
```

### Batch File (SFTP)
```
MoneySend ──Upload payouts.csv──► Partner SFTP
          
[Next day]

Partner   ──Upload results.csv──► MoneySend SFTP
```

## Settlement Process

Daily settlement between MoneySend and payout partners:

```
┌─────────────────────────────────────────────────────────────┐
│                    Daily Settlement Job                      │
└─────────────────────────────────────────────────────────────┘
                              │
         ┌────────────────────┴────────────────────┐
         │                                         │
┌────────▼────────┐                     ┌─────────▼─────────┐
│ Aggregate       │                     │ Calculate Partner │
│ Completed       │                     │ Fees              │
│ Transfers       │                     │                   │
└────────┬────────┘                     └─────────┬─────────┘
         │                                         │
         └────────────────────┬────────────────────┘
                              │
                    ┌─────────▼─────────┐
                    │ Create Settlement │
                    │ Record            │
                    └─────────┬─────────┘
                              │
         ┌────────────────────┴────────────────────┐
         │                                         │
┌────────▼────────┐                     ┌─────────▼─────────┐
│ Generate        │                     │ Create Ledger     │
│ Settlement      │                     │ Entries           │
│ Invoice         │                     │                   │
└─────────────────┘                     └───────────────────┘
```

## Security Architecture

### Authentication
- Customer: Email + Password (bcrypt hash)
- Agent: SSO integration
- API: JWT tokens with short expiry

### Authorisation
- Role-based for agents (AGENT, SUPERVISOR, COMPLIANCE_OFFICER, ADMIN)
- Resource-based for customers (own data only)

### Data Protection
- PII encrypted at rest
- Payment tokens stored (not card numbers)
- Audit logging on all sensitive operations

## Scalability Considerations

### Database
- Read replicas for reporting queries
- Partitioning on Transfers by date
- Index optimisation for common queries

### Services
- Stateless services (horizontal scaling)
- Queue-based async processing
- Circuit breakers for partner integrations

### Caching
- FX rates (60-second TTL)
- Corridor configuration (5-minute TTL)
- Country configuration (15-minute TTL)
