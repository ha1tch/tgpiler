-- ============================================================================
-- MoneySend International Remittance Platform
-- Database Schema v1.0
-- ============================================================================
-- A comprehensive schema for an international money transfer company
-- supporting multiple corridors, compliance requirements, and payout methods.
-- ============================================================================

-- ============================================================================
-- SECTION 1: CUSTOMER MANAGEMENT
-- ============================================================================

-- Core customer (sender) table
CREATE TABLE Customers (
    CustomerId          BIGINT IDENTITY(1,1) PRIMARY KEY,
    ExternalId          UNIQUEIDENTIFIER NOT NULL DEFAULT NEWID(),
    Email               NVARCHAR(255) NOT NULL UNIQUE,
    PasswordHash        NVARCHAR(255) NOT NULL,
    PhoneNumber         NVARCHAR(50) NULL,
    PhoneVerified       BIT NOT NULL DEFAULT 0,
    FirstName           NVARCHAR(100) NOT NULL,
    MiddleName          NVARCHAR(100) NULL,
    LastName            NVARCHAR(100) NOT NULL,
    DateOfBirth         DATE NULL,
    Nationality         CHAR(2) NULL,              -- ISO 3166-1 alpha-2
    CountryOfResidence  CHAR(2) NOT NULL,          -- ISO 3166-1 alpha-2
    AddressLine1        NVARCHAR(255) NULL,
    AddressLine2        NVARCHAR(255) NULL,
    City                NVARCHAR(100) NULL,
    StateProvince       NVARCHAR(100) NULL,
    PostalCode          NVARCHAR(20) NULL,
    
    -- Verification & Compliance
    VerificationTier    TINYINT NOT NULL DEFAULT 1,  -- 1=Basic, 2=Enhanced, 3=Full
    KYCStatus           NVARCHAR(20) NOT NULL DEFAULT 'PENDING',  -- PENDING, VERIFIED, REJECTED, EXPIRED
    KYCVerifiedAt       DATETIME2 NULL,
    KYCExpiresAt        DATETIME2 NULL,
    RiskScore           DECIMAL(5,2) NULL,           -- 0.00 to 100.00
    RiskLevel           NVARCHAR(20) NULL,           -- LOW, MEDIUM, HIGH, PROHIBITED
    
    -- Limits (based on verification tier)
    DailyLimitUSD       DECIMAL(18,2) NOT NULL DEFAULT 1000.00,
    MonthlyLimitUSD     DECIMAL(18,2) NOT NULL DEFAULT 5000.00,
    YearlyLimitUSD      DECIMAL(18,2) NOT NULL DEFAULT 15000.00,
    SingleTxLimitUSD    DECIMAL(18,2) NOT NULL DEFAULT 500.00,
    
    -- Account status
    Status              NVARCHAR(20) NOT NULL DEFAULT 'ACTIVE',  -- ACTIVE, SUSPENDED, CLOSED, FROZEN
    SuspensionReason    NVARCHAR(500) NULL,
    ClosedAt            DATETIME2 NULL,
    
    -- Preferences
    PreferredLanguage   CHAR(2) NOT NULL DEFAULT 'en',
    PreferredCurrency   CHAR(3) NOT NULL DEFAULT 'USD',
    MarketingConsent    BIT NOT NULL DEFAULT 0,
    
    -- Audit
    CreatedAt           DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
    UpdatedAt           DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
    LastLoginAt         DATETIME2 NULL,
    FailedLoginAttempts INT NOT NULL DEFAULT 0,
    LockedUntil         DATETIME2 NULL,
    
    INDEX IX_Customers_Email (Email),
    INDEX IX_Customers_ExternalId (ExternalId),
    INDEX IX_Customers_PhoneNumber (PhoneNumber),
    INDEX IX_Customers_KYCStatus (KYCStatus),
    INDEX IX_Customers_RiskLevel (RiskLevel)
);

-- Customer identity documents for KYC
CREATE TABLE CustomerDocuments (
    DocumentId          BIGINT IDENTITY(1,1) PRIMARY KEY,
    CustomerId          BIGINT NOT NULL REFERENCES Customers(CustomerId),
    DocumentType        NVARCHAR(50) NOT NULL,      -- PASSPORT, DRIVERS_LICENSE, NATIONAL_ID, UTILITY_BILL, BANK_STATEMENT
    DocumentNumber      NVARCHAR(100) NULL,
    IssuingCountry      CHAR(2) NULL,
    IssueDate           DATE NULL,
    ExpiryDate          DATE NULL,
    
    -- Document storage
    FrontImagePath      NVARCHAR(500) NULL,
    BackImagePath       NVARCHAR(500) NULL,
    
    -- Verification
    VerificationStatus  NVARCHAR(20) NOT NULL DEFAULT 'PENDING',  -- PENDING, VERIFIED, REJECTED, EXPIRED
    VerifiedAt          DATETIME2 NULL,
    VerifiedBy          NVARCHAR(100) NULL,         -- Agent or system ID
    RejectionReason     NVARCHAR(500) NULL,
    
    -- Audit
    SubmittedAt         DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
    UpdatedAt           DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
    
    INDEX IX_CustomerDocuments_CustomerId (CustomerId),
    INDEX IX_CustomerDocuments_Status (VerificationStatus)
);

-- Customer verification history (audit trail)
CREATE TABLE CustomerVerificationHistory (
    HistoryId           BIGINT IDENTITY(1,1) PRIMARY KEY,
    CustomerId          BIGINT NOT NULL REFERENCES Customers(CustomerId),
    ActionType          NVARCHAR(50) NOT NULL,      -- TIER_UPGRADE, TIER_DOWNGRADE, KYC_APPROVED, KYC_REJECTED, LIMIT_CHANGE
    PreviousValue       NVARCHAR(500) NULL,
    NewValue            NVARCHAR(500) NULL,
    Reason              NVARCHAR(500) NULL,
    PerformedBy         NVARCHAR(100) NOT NULL,     -- Agent ID or 'SYSTEM'
    PerformedAt         DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
    
    INDEX IX_CustomerVerificationHistory_CustomerId (CustomerId)
);

-- ============================================================================
-- SECTION 2: BENEFICIARY MANAGEMENT
-- ============================================================================

-- Beneficiaries (recipients of transfers)
CREATE TABLE Beneficiaries (
    BeneficiaryId       BIGINT IDENTITY(1,1) PRIMARY KEY,
    CustomerId          BIGINT NOT NULL REFERENCES Customers(CustomerId),
    ExternalId          UNIQUEIDENTIFIER NOT NULL DEFAULT NEWID(),
    
    -- Beneficiary details
    FirstName           NVARCHAR(100) NOT NULL,
    MiddleName          NVARCHAR(100) NULL,
    LastName            NVARCHAR(100) NOT NULL,
    Relationship        NVARCHAR(50) NULL,          -- FAMILY, FRIEND, BUSINESS, SELF, OTHER
    
    -- Contact
    Email               NVARCHAR(255) NULL,
    PhoneNumber         NVARCHAR(50) NULL,
    
    -- Location
    Country             CHAR(2) NOT NULL,           -- ISO 3166-1 alpha-2
    City                NVARCHAR(100) NULL,
    AddressLine1        NVARCHAR(255) NULL,
    AddressLine2        NVARCHAR(255) NULL,
    StateProvince       NVARCHAR(100) NULL,
    PostalCode          NVARCHAR(20) NULL,
    
    -- Preferred payout
    PreferredPayoutMethod NVARCHAR(50) NULL,        -- BANK_DEPOSIT, CASH_PICKUP, MOBILE_WALLET, HOME_DELIVERY
    
    -- Compliance
    ScreeningStatus     NVARCHAR(20) NOT NULL DEFAULT 'PENDING',  -- PENDING, CLEARED, FLAGGED, BLOCKED
    LastScreenedAt      DATETIME2 NULL,
    
    -- Status
    IsActive            BIT NOT NULL DEFAULT 1,
    IsFavorite          BIT NOT NULL DEFAULT 0,
    
    -- Audit
    CreatedAt           DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
    UpdatedAt           DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
    
    INDEX IX_Beneficiaries_CustomerId (CustomerId),
    INDEX IX_Beneficiaries_Country (Country),
    INDEX IX_Beneficiaries_ExternalId (ExternalId)
);

-- Beneficiary bank accounts
CREATE TABLE BeneficiaryBankAccounts (
    BankAccountId       BIGINT IDENTITY(1,1) PRIMARY KEY,
    BeneficiaryId       BIGINT NOT NULL REFERENCES Beneficiaries(BeneficiaryId),
    
    -- Bank details
    BankName            NVARCHAR(200) NOT NULL,
    BankCode            NVARCHAR(50) NULL,          -- SWIFT/BIC or local code
    BranchCode          NVARCHAR(50) NULL,
    AccountNumber       NVARCHAR(50) NOT NULL,
    AccountType         NVARCHAR(20) NOT NULL DEFAULT 'SAVINGS',  -- SAVINGS, CHECKING, CURRENT
    Currency            CHAR(3) NOT NULL,           -- ISO 4217
    
    -- International routing
    IBAN                NVARCHAR(50) NULL,
    RoutingNumber       NVARCHAR(50) NULL,
    CLABE               NVARCHAR(50) NULL,          -- Mexico
    IFSC                NVARCHAR(50) NULL,          -- India
    
    -- Account holder (may differ from beneficiary)
    AccountHolderName   NVARCHAR(200) NOT NULL,
    
    -- Verification
    IsVerified          BIT NOT NULL DEFAULT 0,
    VerifiedAt          DATETIME2 NULL,
    
    -- Status
    IsActive            BIT NOT NULL DEFAULT 1,
    IsPrimary           BIT NOT NULL DEFAULT 0,
    
    -- Audit
    CreatedAt           DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
    UpdatedAt           DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
    
    INDEX IX_BeneficiaryBankAccounts_BeneficiaryId (BeneficiaryId)
);

-- Beneficiary mobile wallets
CREATE TABLE BeneficiaryMobileWallets (
    WalletId            BIGINT IDENTITY(1,1) PRIMARY KEY,
    BeneficiaryId       BIGINT NOT NULL REFERENCES Beneficiaries(BeneficiaryId),
    
    -- Wallet details
    ProviderCode        NVARCHAR(50) NOT NULL,      -- MPESA, GCASH, PAYTM, etc.
    WalletNumber        NVARCHAR(50) NOT NULL,      -- Usually phone number
    WalletHolderName    NVARCHAR(200) NOT NULL,
    Currency            CHAR(3) NOT NULL,
    
    -- Status
    IsActive            BIT NOT NULL DEFAULT 1,
    IsPrimary           BIT NOT NULL DEFAULT 0,
    
    -- Audit
    CreatedAt           DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
    UpdatedAt           DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
    
    INDEX IX_BeneficiaryMobileWallets_BeneficiaryId (BeneficiaryId)
);

-- ============================================================================
-- SECTION 3: CORRIDOR & RATE MANAGEMENT
-- ============================================================================

-- Supported corridors (origin → destination pairs)
CREATE TABLE Corridors (
    CorridorId          INT IDENTITY(1,1) PRIMARY KEY,
    OriginCountry       CHAR(2) NOT NULL,
    DestinationCountry  CHAR(2) NOT NULL,
    OriginCurrency      CHAR(3) NOT NULL,
    DestinationCurrency CHAR(3) NOT NULL,
    
    -- Corridor name (e.g., "USA to Mexico")
    DisplayName         NVARCHAR(100) NOT NULL,
    CorridorCode        NVARCHAR(20) NOT NULL UNIQUE,  -- e.g., "US_MX"
    
    -- Limits
    MinSendAmount       DECIMAL(18,2) NOT NULL DEFAULT 1.00,
    MaxSendAmount       DECIMAL(18,2) NOT NULL DEFAULT 10000.00,
    
    -- Supported payout methods (comma-separated for simplicity)
    SupportedPayoutMethods NVARCHAR(500) NOT NULL,
    
    -- Status
    IsActive            BIT NOT NULL DEFAULT 1,
    
    -- Processing
    EstimatedDeliveryMinutes INT NOT NULL DEFAULT 60,
    CutoffTimeUTC       TIME NULL,                  -- For same-day delivery
    
    -- Audit
    CreatedAt           DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
    UpdatedAt           DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
    
    UNIQUE (OriginCountry, DestinationCountry, OriginCurrency, DestinationCurrency),
    INDEX IX_Corridors_Code (CorridorCode)
);

-- FX rates (updated frequently)
CREATE TABLE FXRates (
    RateId              BIGINT IDENTITY(1,1) PRIMARY KEY,
    CorridorId          INT NOT NULL REFERENCES Corridors(CorridorId),
    
    -- Rate details
    MidMarketRate       DECIMAL(18,8) NOT NULL,     -- Interbank rate
    BuyRate             DECIMAL(18,8) NOT NULL,     -- Rate we buy at (customer sells)
    SellRate            DECIMAL(18,8) NOT NULL,     -- Rate we sell at (customer buys)
    Spread              DECIMAL(8,4) NOT NULL,      -- Our margin percentage
    
    -- Validity
    EffectiveFrom       DATETIME2 NOT NULL,
    EffectiveTo         DATETIME2 NULL,             -- NULL = current rate
    
    -- Source
    RateSource          NVARCHAR(50) NOT NULL,      -- REUTERS, BLOOMBERG, INTERNAL
    
    -- Audit
    CreatedAt           DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
    
    INDEX IX_FXRates_CorridorId (CorridorId),
    INDEX IX_FXRates_EffectiveFrom (EffectiveFrom)
);

-- Fee schedules
CREATE TABLE FeeSchedules (
    FeeScheduleId       INT IDENTITY(1,1) PRIMARY KEY,
    CorridorId          INT NOT NULL REFERENCES Corridors(CorridorId),
    PayoutMethod        NVARCHAR(50) NOT NULL,
    
    -- Fee structure
    FeeType             NVARCHAR(20) NOT NULL,      -- FLAT, PERCENTAGE, TIERED
    FlatFee             DECIMAL(18,2) NULL,
    PercentageFee       DECIMAL(8,4) NULL,          -- As decimal (0.0150 = 1.5%)
    MinFee              DECIMAL(18,2) NULL,
    MaxFee              DECIMAL(18,2) NULL,
    
    -- Tier boundaries (for TIERED type)
    MinAmount           DECIMAL(18,2) NULL,
    MaxAmount           DECIMAL(18,2) NULL,
    
    -- Promotional
    PromoCode           NVARCHAR(50) NULL,
    PromoValidFrom      DATETIME2 NULL,
    PromoValidTo        DATETIME2 NULL,
    
    -- Status
    IsActive            BIT NOT NULL DEFAULT 1,
    
    -- Audit
    CreatedAt           DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
    UpdatedAt           DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
    
    INDEX IX_FeeSchedules_CorridorId (CorridorId)
);

-- ============================================================================
-- SECTION 4: TRANSFER MANAGEMENT
-- ============================================================================

-- Core transfers table
CREATE TABLE Transfers (
    TransferId          BIGINT IDENTITY(1,1) PRIMARY KEY,
    TransferNumber      NVARCHAR(20) NOT NULL UNIQUE,  -- Human-readable (e.g., "MS-2024-001234")
    ExternalId          UNIQUEIDENTIFIER NOT NULL DEFAULT NEWID(),
    
    -- Participants
    CustomerId          BIGINT NOT NULL REFERENCES Customers(CustomerId),
    BeneficiaryId       BIGINT NOT NULL REFERENCES Beneficiaries(BeneficiaryId),
    
    -- Corridor
    CorridorId          INT NOT NULL REFERENCES Corridors(CorridorId),
    
    -- Amounts
    SendAmount          DECIMAL(18,2) NOT NULL,
    SendCurrency        CHAR(3) NOT NULL,
    ReceiveAmount       DECIMAL(18,2) NOT NULL,
    ReceiveCurrency     CHAR(3) NOT NULL,
    
    -- FX
    ExchangeRate        DECIMAL(18,8) NOT NULL,
    RateLockedAt        DATETIME2 NOT NULL,
    RateLockedUntil     DATETIME2 NOT NULL,
    
    -- Fees
    TotalFees           DECIMAL(18,2) NOT NULL,
    TransferFee         DECIMAL(18,2) NOT NULL,
    FXMargin            DECIMAL(18,2) NOT NULL,
    PromoDiscount       DECIMAL(18,2) NOT NULL DEFAULT 0,
    PromoCode           NVARCHAR(50) NULL,
    
    -- Total charged to customer
    TotalCharged        DECIMAL(18,2) NOT NULL,
    
    -- Payout
    PayoutMethod        NVARCHAR(50) NOT NULL,
    PayoutBankAccountId BIGINT NULL REFERENCES BeneficiaryBankAccounts(BankAccountId),
    PayoutWalletId      BIGINT NULL REFERENCES BeneficiaryMobileWallets(WalletId),
    PayoutPartnerId     INT NULL,                   -- References PayoutPartners
    PayoutReference     NVARCHAR(100) NULL,         -- Partner's reference number
    
    -- Cash pickup details (if applicable)
    CashPickupCode      NVARCHAR(20) NULL,          -- Code for recipient to collect
    CashPickupLocation  NVARCHAR(500) NULL,
    
    -- Status
    Status              NVARCHAR(30) NOT NULL DEFAULT 'CREATED',
    -- CREATED, PENDING_PAYMENT, PAYMENT_RECEIVED, COMPLIANCE_REVIEW, 
    -- PROCESSING, SENT_TO_PARTNER, PAYOUT_PENDING, COMPLETED,
    -- CANCELLED, REFUNDED, FAILED, ON_HOLD
    
    SubStatus           NVARCHAR(50) NULL,          -- Detailed status
    StatusReason        NVARCHAR(500) NULL,
    
    -- Compliance
    ComplianceStatus    NVARCHAR(20) NOT NULL DEFAULT 'PENDING',  -- PENDING, CLEARED, FLAGGED, BLOCKED
    ComplianceReviewedAt DATETIME2 NULL,
    ComplianceReviewedBy NVARCHAR(100) NULL,
    RiskScore           DECIMAL(5,2) NULL,
    
    -- Timestamps
    CreatedAt           DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
    UpdatedAt           DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
    PaymentReceivedAt   DATETIME2 NULL,
    ProcessingStartedAt DATETIME2 NULL,
    SentToPartnerAt     DATETIME2 NULL,
    CompletedAt         DATETIME2 NULL,
    CancelledAt         DATETIME2 NULL,
    ExpectedDeliveryAt  DATETIME2 NULL,
    
    -- Source
    SourceChannel       NVARCHAR(50) NOT NULL DEFAULT 'WEB',  -- WEB, MOBILE_IOS, MOBILE_ANDROID, API
    SourceIP            NVARCHAR(50) NULL,
    DeviceFingerprint   NVARCHAR(500) NULL,
    
    -- Purpose (required for compliance in some corridors)
    Purpose             NVARCHAR(100) NULL,         -- FAMILY_SUPPORT, EDUCATION, MEDICAL, BUSINESS, OTHER
    PurposeDescription  NVARCHAR(500) NULL,
    
    INDEX IX_Transfers_CustomerId (CustomerId),
    INDEX IX_Transfers_BeneficiaryId (BeneficiaryId),
    INDEX IX_Transfers_Status (Status),
    INDEX IX_Transfers_TransferNumber (TransferNumber),
    INDEX IX_Transfers_ExternalId (ExternalId),
    INDEX IX_Transfers_CreatedAt (CreatedAt),
    INDEX IX_Transfers_ComplianceStatus (ComplianceStatus)
);

-- Transfer status history (full audit trail)
CREATE TABLE TransferStatusHistory (
    HistoryId           BIGINT IDENTITY(1,1) PRIMARY KEY,
    TransferId          BIGINT NOT NULL REFERENCES Transfers(TransferId),
    
    PreviousStatus      NVARCHAR(30) NULL,
    NewStatus           NVARCHAR(30) NOT NULL,
    SubStatus           NVARCHAR(50) NULL,
    Reason              NVARCHAR(500) NULL,
    
    -- Actor
    ChangedBy           NVARCHAR(100) NOT NULL,     -- CustomerID, AgentID, 'SYSTEM', 'PARTNER'
    ChangedAt           DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
    
    -- Additional context
    Notes               NVARCHAR(1000) NULL,
    
    INDEX IX_TransferStatusHistory_TransferId (TransferId),
    INDEX IX_TransferStatusHistory_ChangedAt (ChangedAt)
);

-- ============================================================================
-- SECTION 5: PAYMENT MANAGEMENT
-- ============================================================================

-- Customer funding sources
CREATE TABLE CustomerFundingSources (
    FundingSourceId     BIGINT IDENTITY(1,1) PRIMARY KEY,
    CustomerId          BIGINT NOT NULL REFERENCES Customers(CustomerId),
    
    -- Source type
    SourceType          NVARCHAR(30) NOT NULL,      -- DEBIT_CARD, CREDIT_CARD, BANK_ACCOUNT, ACH
    
    -- Card details (masked)
    CardLastFour        CHAR(4) NULL,
    CardBrand           NVARCHAR(20) NULL,          -- VISA, MASTERCARD, AMEX
    CardExpiry          CHAR(7) NULL,               -- MM/YYYY
    CardHolderName      NVARCHAR(200) NULL,
    
    -- Bank details
    BankName            NVARCHAR(200) NULL,
    AccountLastFour     CHAR(4) NULL,
    RoutingNumber       NVARCHAR(20) NULL,
    
    -- Tokenization
    TokenProvider       NVARCHAR(50) NULL,          -- STRIPE, BRAINTREE, etc.
    PaymentToken        NVARCHAR(500) NULL,
    
    -- Verification
    IsVerified          BIT NOT NULL DEFAULT 0,
    VerifiedAt          DATETIME2 NULL,
    
    -- Status
    IsActive            BIT NOT NULL DEFAULT 1,
    IsPrimary           BIT NOT NULL DEFAULT 0,
    
    -- Audit
    CreatedAt           DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
    UpdatedAt           DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
    
    INDEX IX_CustomerFundingSources_CustomerId (CustomerId)
);

-- Payment transactions
CREATE TABLE PaymentTransactions (
    PaymentId           BIGINT IDENTITY(1,1) PRIMARY KEY,
    TransferId          BIGINT NOT NULL REFERENCES Transfers(TransferId),
    FundingSourceId     BIGINT NULL REFERENCES CustomerFundingSources(FundingSourceId),
    
    -- Payment details
    PaymentType         NVARCHAR(30) NOT NULL,      -- CHARGE, REFUND, CHARGEBACK
    Amount              DECIMAL(18,2) NOT NULL,
    Currency            CHAR(3) NOT NULL,
    
    -- Status
    Status              NVARCHAR(20) NOT NULL,      -- PENDING, COMPLETED, FAILED, REVERSED
    
    -- Gateway details
    GatewayProvider     NVARCHAR(50) NULL,
    GatewayTransactionId NVARCHAR(100) NULL,
    GatewayResponse     NVARCHAR(MAX) NULL,
    
    -- For failures
    FailureCode         NVARCHAR(50) NULL,
    FailureReason       NVARCHAR(500) NULL,
    
    -- Audit
    CreatedAt           DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
    CompletedAt         DATETIME2 NULL,
    
    INDEX IX_PaymentTransactions_TransferId (TransferId)
);

-- ============================================================================
-- SECTION 6: COMPLIANCE & AML
-- ============================================================================

-- Compliance screening results
CREATE TABLE ComplianceScreenings (
    ScreeningId         BIGINT IDENTITY(1,1) PRIMARY KEY,
    
    -- What was screened
    EntityType          NVARCHAR(30) NOT NULL,      -- CUSTOMER, BENEFICIARY, TRANSFER
    EntityId            BIGINT NOT NULL,
    
    -- Related transfer (if applicable)
    TransferId          BIGINT NULL REFERENCES Transfers(TransferId),
    
    -- Screening type
    ScreeningType       NVARCHAR(50) NOT NULL,      -- SANCTIONS, PEP, ADVERSE_MEDIA, TRANSACTION_MONITORING
    
    -- Provider
    ScreeningProvider   NVARCHAR(50) NOT NULL,      -- REFINITIV, DOW_JONES, INTERNAL
    ProviderReference   NVARCHAR(100) NULL,
    
    -- Result
    Result              NVARCHAR(20) NOT NULL,      -- CLEAR, POTENTIAL_MATCH, MATCH, ERROR
    MatchScore          DECIMAL(5,2) NULL,          -- 0-100
    MatchDetails        NVARCHAR(MAX) NULL,         -- JSON with match details
    
    -- Resolution
    ResolutionStatus    NVARCHAR(30) NULL,          -- PENDING_REVIEW, TRUE_POSITIVE, FALSE_POSITIVE, ESCALATED
    ResolvedBy          NVARCHAR(100) NULL,
    ResolvedAt          DATETIME2 NULL,
    ResolutionNotes     NVARCHAR(1000) NULL,
    
    -- Lists matched
    ListsMatched        NVARCHAR(500) NULL,         -- OFAC, UN, EU, etc.
    
    -- Audit
    ScreenedAt          DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
    
    INDEX IX_ComplianceScreenings_EntityType (EntityType, EntityId),
    INDEX IX_ComplianceScreenings_TransferId (TransferId),
    INDEX IX_ComplianceScreenings_Result (Result)
);

-- Suspicious Activity Reports (SARs)
CREATE TABLE SuspiciousActivityReports (
    SARId               BIGINT IDENTITY(1,1) PRIMARY KEY,
    SARNumber           NVARCHAR(50) NOT NULL UNIQUE,
    
    -- Subject
    CustomerId          BIGINT NULL REFERENCES Customers(CustomerId),
    BeneficiaryId       BIGINT NULL REFERENCES Beneficiaries(BeneficiaryId),
    
    -- Related transfers
    TransferIds         NVARCHAR(MAX) NULL,         -- JSON array of related transfer IDs
    
    -- Report details
    ActivityType        NVARCHAR(100) NOT NULL,     -- STRUCTURING, UNUSUAL_PATTERN, SANCTIONS_EVASION, etc.
    ActivityDescription NVARCHAR(MAX) NOT NULL,
    SuspicionLevel      NVARCHAR(20) NOT NULL,      -- LOW, MEDIUM, HIGH, CRITICAL
    
    -- Financial impact
    TotalAmountInvolved DECIMAL(18,2) NULL,
    Currency            CHAR(3) NULL,
    
    -- Status
    Status              NVARCHAR(30) NOT NULL DEFAULT 'DRAFT',  -- DRAFT, PENDING_REVIEW, FILED, ARCHIVED
    
    -- Filing
    FiledWith           NVARCHAR(100) NULL,         -- FINCEN, FCA, etc.
    FilingReference     NVARCHAR(100) NULL,
    FiledAt             DATETIME2 NULL,
    FiledBy             NVARCHAR(100) NULL,
    
    -- Audit
    CreatedBy           NVARCHAR(100) NOT NULL,
    CreatedAt           DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
    UpdatedAt           DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
    
    INDEX IX_SuspiciousActivityReports_CustomerId (CustomerId),
    INDEX IX_SuspiciousActivityReports_Status (Status)
);

-- Customer risk assessments
CREATE TABLE CustomerRiskAssessments (
    AssessmentId        BIGINT IDENTITY(1,1) PRIMARY KEY,
    CustomerId          BIGINT NOT NULL REFERENCES Customers(CustomerId),
    
    -- Assessment details
    AssessmentType      NVARCHAR(50) NOT NULL,      -- INITIAL, PERIODIC, TRIGGER_BASED
    TriggerReason       NVARCHAR(200) NULL,
    
    -- Risk factors
    CountryRiskScore    DECIMAL(5,2) NULL,
    TransactionRiskScore DECIMAL(5,2) NULL,
    BehaviorRiskScore   DECIMAL(5,2) NULL,
    DocumentRiskScore   DECIMAL(5,2) NULL,
    
    -- Overall
    OverallRiskScore    DECIMAL(5,2) NOT NULL,
    RiskLevel           NVARCHAR(20) NOT NULL,      -- LOW, MEDIUM, HIGH, PROHIBITED
    
    -- Previous assessment
    PreviousRiskLevel   NVARCHAR(20) NULL,
    RiskLevelChanged    BIT NOT NULL DEFAULT 0,
    
    -- Recommendations
    RecommendedActions  NVARCHAR(500) NULL,
    
    -- Approval (if risk level high)
    RequiresApproval    BIT NOT NULL DEFAULT 0,
    ApprovedBy          NVARCHAR(100) NULL,
    ApprovedAt          DATETIME2 NULL,
    
    -- Audit
    AssessedBy          NVARCHAR(100) NOT NULL,
    AssessedAt          DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
    
    INDEX IX_CustomerRiskAssessments_CustomerId (CustomerId)
);

-- ============================================================================
-- SECTION 7: PAYOUT PARTNER MANAGEMENT
-- ============================================================================

-- Payout partners (banks, cash networks, wallet providers)
CREATE TABLE PayoutPartners (
    PartnerId           INT IDENTITY(1,1) PRIMARY KEY,
    PartnerCode         NVARCHAR(50) NOT NULL UNIQUE,
    PartnerName         NVARCHAR(200) NOT NULL,
    
    -- Partner type
    PartnerType         NVARCHAR(50) NOT NULL,      -- BANK, CASH_NETWORK, MOBILE_WALLET, CORRESPONDENT
    
    -- Coverage
    Countries           NVARCHAR(500) NOT NULL,     -- JSON array of country codes
    Currencies          NVARCHAR(200) NOT NULL,     -- JSON array of currency codes
    PayoutMethods       NVARCHAR(200) NOT NULL,     -- JSON array of supported methods
    
    -- Integration
    IntegrationType     NVARCHAR(30) NOT NULL,      -- API, SFTP, MANUAL
    ApiEndpoint         NVARCHAR(500) NULL,
    ApiVersion          NVARCHAR(20) NULL,
    
    -- Settlement
    SettlementCurrency  CHAR(3) NOT NULL,
    SettlementFrequency NVARCHAR(20) NOT NULL,      -- REALTIME, DAILY, WEEKLY
    CreditLimit         DECIMAL(18,2) NULL,
    CurrentBalance      DECIMAL(18,2) NOT NULL DEFAULT 0,
    
    -- Fees
    PartnerFeeType      NVARCHAR(20) NULL,          -- FLAT, PERCENTAGE
    PartnerFee          DECIMAL(18,4) NULL,
    
    -- Status
    Status              NVARCHAR(20) NOT NULL DEFAULT 'ACTIVE',  -- ACTIVE, SUSPENDED, TERMINATED
    
    -- Contact
    PrimaryContactName  NVARCHAR(200) NULL,
    PrimaryContactEmail NVARCHAR(255) NULL,
    PrimaryContactPhone NVARCHAR(50) NULL,
    
    -- Audit
    CreatedAt           DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
    UpdatedAt           DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
    
    INDEX IX_PayoutPartners_PartnerCode (PartnerCode),
    INDEX IX_PayoutPartners_PartnerType (PartnerType)
);

-- Corridor-partner mappings
CREATE TABLE CorridorPartners (
    Id                  INT IDENTITY(1,1) PRIMARY KEY,
    CorridorId          INT NOT NULL REFERENCES Corridors(CorridorId),
    PartnerId           INT NOT NULL REFERENCES PayoutPartners(PartnerId),
    PayoutMethod        NVARCHAR(50) NOT NULL,
    
    -- Priority (lower = preferred)
    Priority            INT NOT NULL DEFAULT 1,
    
    -- Status
    IsActive            BIT NOT NULL DEFAULT 1,
    
    UNIQUE (CorridorId, PartnerId, PayoutMethod)
);

-- Partner settlements
CREATE TABLE PartnerSettlements (
    SettlementId        BIGINT IDENTITY(1,1) PRIMARY KEY,
    PartnerId           INT NOT NULL REFERENCES PayoutPartners(PartnerId),
    
    -- Settlement period
    PeriodStart         DATETIME2 NOT NULL,
    PeriodEnd           DATETIME2 NOT NULL,
    
    -- Amounts
    TotalTransactions   INT NOT NULL,
    TotalAmount         DECIMAL(18,2) NOT NULL,
    Currency            CHAR(3) NOT NULL,
    PartnerFees         DECIMAL(18,2) NOT NULL,
    NetSettlement       DECIMAL(18,2) NOT NULL,
    
    -- Direction
    SettlementDirection NVARCHAR(20) NOT NULL,      -- PAYABLE (we owe), RECEIVABLE (they owe)
    
    -- Status
    Status              NVARCHAR(30) NOT NULL DEFAULT 'PENDING',  -- PENDING, INVOICED, PAID, RECONCILED, DISPUTED
    
    -- Payment
    PaymentReference    NVARCHAR(100) NULL,
    PaidAt              DATETIME2 NULL,
    
    -- Audit
    CreatedAt           DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
    UpdatedAt           DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
    
    INDEX IX_PartnerSettlements_PartnerId (PartnerId),
    INDEX IX_PartnerSettlements_Status (Status)
);

-- ============================================================================
-- SECTION 8: LEDGER & ACCOUNTING
-- ============================================================================

-- General ledger accounts
CREATE TABLE LedgerAccounts (
    AccountId           INT IDENTITY(1,1) PRIMARY KEY,
    AccountCode         NVARCHAR(20) NOT NULL UNIQUE,
    AccountName         NVARCHAR(200) NOT NULL,
    AccountType         NVARCHAR(30) NOT NULL,      -- ASSET, LIABILITY, REVENUE, EXPENSE, EQUITY
    Currency            CHAR(3) NOT NULL,
    ParentAccountId     INT NULL REFERENCES LedgerAccounts(AccountId),
    
    IsActive            BIT NOT NULL DEFAULT 1,
    
    INDEX IX_LedgerAccounts_AccountCode (AccountCode)
);

-- Ledger entries (double-entry)
CREATE TABLE LedgerEntries (
    EntryId             BIGINT IDENTITY(1,1) PRIMARY KEY,
    JournalId           BIGINT NOT NULL,            -- Groups related entries
    
    AccountId           INT NOT NULL REFERENCES LedgerAccounts(AccountId),
    
    -- Entry details
    EntryType           NVARCHAR(10) NOT NULL,      -- DEBIT, CREDIT
    Amount              DECIMAL(18,2) NOT NULL,
    Currency            CHAR(3) NOT NULL,
    
    -- Reference
    ReferenceType       NVARCHAR(50) NOT NULL,      -- TRANSFER, PAYMENT, SETTLEMENT, ADJUSTMENT
    ReferenceId         BIGINT NOT NULL,
    
    -- Description
    Description         NVARCHAR(500) NULL,
    
    -- Audit
    CreatedAt           DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
    CreatedBy           NVARCHAR(100) NOT NULL,
    
    INDEX IX_LedgerEntries_JournalId (JournalId),
    INDEX IX_LedgerEntries_AccountId (AccountId),
    INDEX IX_LedgerEntries_Reference (ReferenceType, ReferenceId)
);

-- Daily reconciliation
CREATE TABLE DailyReconciliation (
    ReconciliationId    BIGINT IDENTITY(1,1) PRIMARY KEY,
    ReconciliationDate  DATE NOT NULL,
    Currency            CHAR(3) NOT NULL,
    
    -- Expected
    ExpectedTransferCount INT NOT NULL,
    ExpectedTransferVolume DECIMAL(18,2) NOT NULL,
    ExpectedFeeRevenue  DECIMAL(18,2) NOT NULL,
    
    -- Actual
    ActualTransferCount INT NOT NULL,
    ActualTransferVolume DECIMAL(18,2) NOT NULL,
    ActualFeeRevenue    DECIMAL(18,2) NOT NULL,
    
    -- Variance
    TransferCountVariance INT NOT NULL,
    TransferVolumeVariance DECIMAL(18,2) NOT NULL,
    FeeRevenueVariance  DECIMAL(18,2) NOT NULL,
    
    -- Status
    Status              NVARCHAR(20) NOT NULL,      -- MATCHED, VARIANCE, INVESTIGATING, RESOLVED
    ResolutionNotes     NVARCHAR(1000) NULL,
    
    -- Audit
    RunAt               DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
    ReviewedBy          NVARCHAR(100) NULL,
    ReviewedAt          DATETIME2 NULL,
    
    UNIQUE (ReconciliationDate, Currency)
);

-- ============================================================================
-- SECTION 9: NOTIFICATIONS & COMMUNICATIONS
-- ============================================================================

CREATE TABLE NotificationTemplates (
    TemplateId          INT IDENTITY(1,1) PRIMARY KEY,
    TemplateCode        NVARCHAR(50) NOT NULL UNIQUE,
    TemplateName        NVARCHAR(200) NOT NULL,
    
    -- Channel
    Channel             NVARCHAR(20) NOT NULL,      -- EMAIL, SMS, PUSH
    
    -- Content
    Subject             NVARCHAR(500) NULL,         -- For email
    BodyTemplate        NVARCHAR(MAX) NOT NULL,
    
    -- Localisation
    Language            CHAR(2) NOT NULL DEFAULT 'en',
    
    IsActive            BIT NOT NULL DEFAULT 1,
    
    INDEX IX_NotificationTemplates_Code (TemplateCode)
);

CREATE TABLE Notifications (
    NotificationId      BIGINT IDENTITY(1,1) PRIMARY KEY,
    
    -- Recipient
    CustomerId          BIGINT NULL REFERENCES Customers(CustomerId),
    RecipientEmail      NVARCHAR(255) NULL,
    RecipientPhone      NVARCHAR(50) NULL,
    
    -- Related entity
    RelatedEntityType   NVARCHAR(50) NULL,          -- TRANSFER, CUSTOMER, BENEFICIARY
    RelatedEntityId     BIGINT NULL,
    
    -- Template
    TemplateId          INT NULL REFERENCES NotificationTemplates(TemplateId),
    
    -- Channel
    Channel             NVARCHAR(20) NOT NULL,
    
    -- Content (rendered)
    Subject             NVARCHAR(500) NULL,
    Body                NVARCHAR(MAX) NOT NULL,
    
    -- Status
    Status              NVARCHAR(20) NOT NULL DEFAULT 'PENDING',  -- PENDING, SENT, DELIVERED, FAILED, BOUNCED
    
    -- Delivery
    SentAt              DATETIME2 NULL,
    DeliveredAt         DATETIME2 NULL,
    FailureReason       NVARCHAR(500) NULL,
    
    -- Provider
    ProviderName        NVARCHAR(50) NULL,
    ProviderMessageId   NVARCHAR(100) NULL,
    
    -- Audit
    CreatedAt           DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
    
    INDEX IX_Notifications_CustomerId (CustomerId),
    INDEX IX_Notifications_Status (Status),
    INDEX IX_Notifications_RelatedEntity (RelatedEntityType, RelatedEntityId)
);

-- ============================================================================
-- SECTION 10: PROMOTIONS & CAMPAIGNS
-- ============================================================================

CREATE TABLE PromoCodes (
    PromoCodeId         INT IDENTITY(1,1) PRIMARY KEY,
    Code                NVARCHAR(50) NOT NULL UNIQUE,
    PromoName           NVARCHAR(200) NOT NULL,
    Description         NVARCHAR(500) NULL,
    
    -- Discount type
    DiscountType        NVARCHAR(20) NOT NULL,      -- FLAT_FEE, PERCENTAGE_FEE, FREE_TRANSFER, BETTER_RATE
    DiscountValue       DECIMAL(18,4) NOT NULL,
    MaxDiscountAmount   DECIMAL(18,2) NULL,
    
    -- Eligibility
    MinSendAmount       DECIMAL(18,2) NULL,
    MaxSendAmount       DECIMAL(18,2) NULL,
    EligibleCorridors   NVARCHAR(500) NULL,         -- NULL = all corridors
    NewCustomersOnly    BIT NOT NULL DEFAULT 0,
    
    -- Limits
    MaxUsesTotal        INT NULL,
    MaxUsesPerCustomer  INT NOT NULL DEFAULT 1,
    CurrentUsageCount   INT NOT NULL DEFAULT 0,
    
    -- Validity
    ValidFrom           DATETIME2 NOT NULL,
    ValidTo             DATETIME2 NOT NULL,
    
    -- Status
    IsActive            BIT NOT NULL DEFAULT 1,
    
    -- Audit
    CreatedAt           DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
    CreatedBy           NVARCHAR(100) NOT NULL,
    
    INDEX IX_PromoCodes_Code (Code),
    INDEX IX_PromoCodes_ValidTo (ValidTo)
);

CREATE TABLE PromoCodeUsage (
    UsageId             BIGINT IDENTITY(1,1) PRIMARY KEY,
    PromoCodeId         INT NOT NULL REFERENCES PromoCodes(PromoCodeId),
    CustomerId          BIGINT NOT NULL REFERENCES Customers(CustomerId),
    TransferId          BIGINT NOT NULL REFERENCES Transfers(TransferId),
    
    DiscountApplied     DECIMAL(18,2) NOT NULL,
    
    UsedAt              DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
    
    INDEX IX_PromoCodeUsage_PromoCodeId (PromoCodeId),
    INDEX IX_PromoCodeUsage_CustomerId (CustomerId)
);

-- ============================================================================
-- SECTION 11: AGENT/STAFF MANAGEMENT
-- ============================================================================

CREATE TABLE Agents (
    AgentId             INT IDENTITY(1,1) PRIMARY KEY,
    EmployeeId          NVARCHAR(50) NOT NULL UNIQUE,
    Email               NVARCHAR(255) NOT NULL UNIQUE,
    FirstName           NVARCHAR(100) NOT NULL,
    LastName            NVARCHAR(100) NOT NULL,
    
    -- Role
    Role                NVARCHAR(50) NOT NULL,      -- AGENT, SUPERVISOR, COMPLIANCE_OFFICER, ADMIN
    Department          NVARCHAR(50) NULL,
    
    -- Permissions (simplified - would be more complex in production)
    CanReviewKYC        BIT NOT NULL DEFAULT 0,
    CanApproveTransfers BIT NOT NULL DEFAULT 0,
    CanManageCustomers  BIT NOT NULL DEFAULT 0,
    CanFileSARs         BIT NOT NULL DEFAULT 0,
    CanManagePartners   BIT NOT NULL DEFAULT 0,
    CanViewReports      BIT NOT NULL DEFAULT 0,
    
    -- Status
    Status              NVARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    
    -- Audit
    CreatedAt           DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
    LastLoginAt         DATETIME2 NULL,
    
    INDEX IX_Agents_EmployeeId (EmployeeId)
);

-- Agent activity log
CREATE TABLE AgentActivityLog (
    LogId               BIGINT IDENTITY(1,1) PRIMARY KEY,
    AgentId             INT NOT NULL REFERENCES Agents(AgentId),
    
    ActivityType        NVARCHAR(100) NOT NULL,
    EntityType          NVARCHAR(50) NULL,
    EntityId            BIGINT NULL,
    
    Description         NVARCHAR(500) NOT NULL,
    IPAddress           NVARCHAR(50) NULL,
    
    LoggedAt            DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
    
    INDEX IX_AgentActivityLog_AgentId (AgentId),
    INDEX IX_AgentActivityLog_LoggedAt (LoggedAt)
);

-- ============================================================================
-- SECTION 12: SYSTEM CONFIGURATION
-- ============================================================================

CREATE TABLE SystemConfiguration (
    ConfigKey           NVARCHAR(100) PRIMARY KEY,
    ConfigValue         NVARCHAR(MAX) NOT NULL,
    ConfigType          NVARCHAR(20) NOT NULL,      -- STRING, INT, DECIMAL, BOOL, JSON
    Description         NVARCHAR(500) NULL,
    
    IsEncrypted         BIT NOT NULL DEFAULT 0,
    
    UpdatedAt           DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
    UpdatedBy           NVARCHAR(100) NOT NULL
);

-- Country configuration
CREATE TABLE CountryConfiguration (
    CountryCode         CHAR(2) PRIMARY KEY,
    CountryName         NVARCHAR(100) NOT NULL,
    
    -- Regulatory
    RequiresSourceOfFunds BIT NOT NULL DEFAULT 0,
    RequiresPurpose     BIT NOT NULL DEFAULT 0,
    MaxDailyLimit       DECIMAL(18,2) NULL,
    
    -- Risk
    RiskLevel           NVARCHAR(20) NOT NULL DEFAULT 'STANDARD',  -- LOW, STANDARD, HIGH, PROHIBITED
    
    -- Status
    IsSendingEnabled    BIT NOT NULL DEFAULT 0,
    IsReceivingEnabled  BIT NOT NULL DEFAULT 0,
    
    INDEX IX_CountryConfiguration_RiskLevel (RiskLevel)
);

-- Audit log (system-wide)
CREATE TABLE AuditLog (
    AuditId             BIGINT IDENTITY(1,1) PRIMARY KEY,
    
    -- Actor
    ActorType           NVARCHAR(30) NOT NULL,      -- CUSTOMER, AGENT, SYSTEM, PARTNER
    ActorId             NVARCHAR(100) NOT NULL,
    
    -- Action
    ActionType          NVARCHAR(100) NOT NULL,
    EntityType          NVARCHAR(50) NOT NULL,
    EntityId            BIGINT NULL,
    
    -- Changes
    OldValues           NVARCHAR(MAX) NULL,
    NewValues           NVARCHAR(MAX) NULL,
    
    -- Context
    IPAddress           NVARCHAR(50) NULL,
    UserAgent           NVARCHAR(500) NULL,
    
    -- Timestamp
    CreatedAt           DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
    
    INDEX IX_AuditLog_ActorType (ActorType, ActorId),
    INDEX IX_AuditLog_EntityType (EntityType, EntityId),
    INDEX IX_AuditLog_CreatedAt (CreatedAt)
);
