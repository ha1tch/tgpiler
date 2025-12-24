-- ============================================================================
-- MoneySend Stored Procedures
-- Part 6: Payout Partners & Settlements
-- ============================================================================

-- ============================================================================
-- PAYOUT PARTNER MANAGEMENT
-- ============================================================================

-- Create payout partner
CREATE PROCEDURE usp_CreatePayoutPartner
    @PartnerCode        NVARCHAR(50),
    @PartnerName        NVARCHAR(200),
    @PartnerType        NVARCHAR(50),
    @Countries          NVARCHAR(500),
    @Currencies         NVARCHAR(200),
    @PayoutMethods      NVARCHAR(200),
    @IntegrationType    NVARCHAR(30),
    @ApiEndpoint        NVARCHAR(500) = NULL,
    @ApiVersion         NVARCHAR(20) = NULL,
    @SettlementCurrency CHAR(3),
    @SettlementFrequency NVARCHAR(20),
    @CreditLimit        DECIMAL(18,2) = NULL,
    @PartnerFeeType     NVARCHAR(20) = NULL,
    @PartnerFee         DECIMAL(18,4) = NULL,
    @PrimaryContactName NVARCHAR(200) = NULL,
    @PrimaryContactEmail NVARCHAR(255) = NULL,
    @PrimaryContactPhone NVARCHAR(50) = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    IF EXISTS (SELECT 1 FROM PayoutPartners WHERE PartnerCode = @PartnerCode)
    BEGIN
        SELECT 0 AS Success, 'DUPLICATE_CODE' AS ErrorCode;
        RETURN;
    END
    
    INSERT INTO PayoutPartners (
        PartnerCode, PartnerName, PartnerType, Countries, Currencies, PayoutMethods,
        IntegrationType, ApiEndpoint, ApiVersion, SettlementCurrency, SettlementFrequency,
        CreditLimit, PartnerFeeType, PartnerFee,
        PrimaryContactName, PrimaryContactEmail, PrimaryContactPhone, Status
    )
    VALUES (
        @PartnerCode, @PartnerName, @PartnerType, @Countries, @Currencies, @PayoutMethods,
        @IntegrationType, @ApiEndpoint, @ApiVersion, @SettlementCurrency, @SettlementFrequency,
        @CreditLimit, @PartnerFeeType, @PartnerFee,
        @PrimaryContactName, @PrimaryContactEmail, @PrimaryContactPhone, 'ACTIVE'
    );
    
    SELECT 1 AS Success, SCOPE_IDENTITY() AS PartnerId;
END;
GO

-- Get partner by ID
CREATE PROCEDURE usp_GetPartnerById
    @PartnerId INT
AS
BEGIN
    SET NOCOUNT ON;
    
    SELECT 
        PartnerId,
        PartnerCode,
        PartnerName,
        PartnerType,
        Countries,
        Currencies,
        PayoutMethods,
        IntegrationType,
        ApiEndpoint,
        ApiVersion,
        SettlementCurrency,
        SettlementFrequency,
        CreditLimit,
        CurrentBalance,
        PartnerFeeType,
        PartnerFee,
        Status,
        PrimaryContactName,
        PrimaryContactEmail,
        PrimaryContactPhone,
        CreatedAt,
        UpdatedAt
    FROM PayoutPartners
    WHERE PartnerId = @PartnerId;
END;
GO

-- Get partner by code
CREATE PROCEDURE usp_GetPartnerByCode
    @PartnerCode NVARCHAR(50)
AS
BEGIN
    SET NOCOUNT ON;
    
    SELECT 
        PartnerId,
        PartnerCode,
        PartnerName,
        PartnerType,
        Countries,
        Currencies,
        PayoutMethods,
        IntegrationType,
        SettlementCurrency,
        SettlementFrequency,
        CreditLimit,
        CurrentBalance,
        Status
    FROM PayoutPartners
    WHERE PartnerCode = @PartnerCode;
END;
GO

-- List all partners
CREATE PROCEDURE usp_ListPayoutPartners
    @PartnerType    NVARCHAR(50) = NULL,
    @Status         NVARCHAR(20) = NULL,
    @Country        CHAR(2) = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    SELECT 
        PartnerId,
        PartnerCode,
        PartnerName,
        PartnerType,
        Countries,
        Currencies,
        PayoutMethods,
        IntegrationType,
        SettlementCurrency,
        CreditLimit,
        CurrentBalance,
        Status,
        CreatedAt
    FROM PayoutPartners
    WHERE (@PartnerType IS NULL OR PartnerType = @PartnerType)
    AND (@Status IS NULL OR Status = @Status)
    AND (@Country IS NULL OR Countries LIKE '%' + @Country + '%')
    ORDER BY PartnerName;
END;
GO

-- Update partner
CREATE PROCEDURE usp_UpdatePayoutPartner
    @PartnerId          INT,
    @PartnerName        NVARCHAR(200) = NULL,
    @ApiEndpoint        NVARCHAR(500) = NULL,
    @ApiVersion         NVARCHAR(20) = NULL,
    @CreditLimit        DECIMAL(18,2) = NULL,
    @PartnerFeeType     NVARCHAR(20) = NULL,
    @PartnerFee         DECIMAL(18,4) = NULL,
    @PrimaryContactName NVARCHAR(200) = NULL,
    @PrimaryContactEmail NVARCHAR(255) = NULL,
    @PrimaryContactPhone NVARCHAR(50) = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    UPDATE PayoutPartners
    SET PartnerName = COALESCE(@PartnerName, PartnerName),
        ApiEndpoint = COALESCE(@ApiEndpoint, ApiEndpoint),
        ApiVersion = COALESCE(@ApiVersion, ApiVersion),
        CreditLimit = COALESCE(@CreditLimit, CreditLimit),
        PartnerFeeType = COALESCE(@PartnerFeeType, PartnerFeeType),
        PartnerFee = COALESCE(@PartnerFee, PartnerFee),
        PrimaryContactName = COALESCE(@PrimaryContactName, PrimaryContactName),
        PrimaryContactEmail = COALESCE(@PrimaryContactEmail, PrimaryContactEmail),
        PrimaryContactPhone = COALESCE(@PrimaryContactPhone, PrimaryContactPhone),
        UpdatedAt = SYSUTCDATETIME()
    WHERE PartnerId = @PartnerId;
    
    SELECT 1 AS Success;
END;
GO

-- Suspend partner
CREATE PROCEDURE usp_SuspendPayoutPartner
    @PartnerId      INT,
    @Reason         NVARCHAR(500),
    @SuspendedBy    NVARCHAR(100)
AS
BEGIN
    SET NOCOUNT ON;
    
    UPDATE PayoutPartners
    SET Status = 'SUSPENDED',
        UpdatedAt = SYSUTCDATETIME()
    WHERE PartnerId = @PartnerId;
    
    INSERT INTO AuditLog (ActorType, ActorId, ActionType, EntityType, EntityId, NewValues)
    VALUES ('AGENT', @SuspendedBy, 'SUSPEND_PARTNER', 'PARTNER', @PartnerId, 
            '{"reason":"' + @Reason + '"}');
    
    SELECT 1 AS Success;
END;
GO

-- Reactivate partner
CREATE PROCEDURE usp_ReactivatePayoutPartner
    @PartnerId      INT,
    @ReactivatedBy  NVARCHAR(100)
AS
BEGIN
    SET NOCOUNT ON;
    
    UPDATE PayoutPartners
    SET Status = 'ACTIVE',
        UpdatedAt = SYSUTCDATETIME()
    WHERE PartnerId = @PartnerId;
    
    INSERT INTO AuditLog (ActorType, ActorId, ActionType, EntityType, EntityId)
    VALUES ('AGENT', @ReactivatedBy, 'REACTIVATE_PARTNER', 'PARTNER', @PartnerId);
    
    SELECT 1 AS Success;
END;
GO

-- Update partner balance
CREATE PROCEDURE usp_UpdatePartnerBalance
    @PartnerId      INT,
    @Amount         DECIMAL(18,2),
    @Operation      NVARCHAR(10)  -- ADD, SUBTRACT
AS
BEGIN
    SET NOCOUNT ON;
    
    IF @Operation = 'ADD'
        UPDATE PayoutPartners SET CurrentBalance = CurrentBalance + @Amount, UpdatedAt = SYSUTCDATETIME() WHERE PartnerId = @PartnerId;
    ELSE IF @Operation = 'SUBTRACT'
        UPDATE PayoutPartners SET CurrentBalance = CurrentBalance - @Amount, UpdatedAt = SYSUTCDATETIME() WHERE PartnerId = @PartnerId;
    
    SELECT CurrentBalance FROM PayoutPartners WHERE PartnerId = @PartnerId;
END;
GO

-- ============================================================================
-- CORRIDOR-PARTNER MAPPING
-- ============================================================================

-- Add corridor-partner mapping
CREATE PROCEDURE usp_AddCorridorPartnerMapping
    @CorridorId     INT,
    @PartnerId      INT,
    @PayoutMethod   NVARCHAR(50),
    @Priority       INT = 1
AS
BEGIN
    SET NOCOUNT ON;
    
    IF EXISTS (SELECT 1 FROM CorridorPartners 
               WHERE CorridorId = @CorridorId AND PartnerId = @PartnerId AND PayoutMethod = @PayoutMethod)
    BEGIN
        SELECT 0 AS Success, 'MAPPING_EXISTS' AS ErrorCode;
        RETURN;
    END
    
    INSERT INTO CorridorPartners (CorridorId, PartnerId, PayoutMethod, Priority, IsActive)
    VALUES (@CorridorId, @PartnerId, @PayoutMethod, @Priority, 1);
    
    SELECT 1 AS Success, SCOPE_IDENTITY() AS MappingId;
END;
GO

-- Update corridor-partner priority
CREATE PROCEDURE usp_UpdateCorridorPartnerPriority
    @CorridorId     INT,
    @PartnerId      INT,
    @PayoutMethod   NVARCHAR(50),
    @Priority       INT
AS
BEGIN
    SET NOCOUNT ON;
    
    UPDATE CorridorPartners
    SET Priority = @Priority
    WHERE CorridorId = @CorridorId AND PartnerId = @PartnerId AND PayoutMethod = @PayoutMethod;
    
    SELECT 1 AS Success;
END;
GO

-- Deactivate corridor-partner mapping
CREATE PROCEDURE usp_DeactivateCorridorPartnerMapping
    @CorridorId     INT,
    @PartnerId      INT,
    @PayoutMethod   NVARCHAR(50)
AS
BEGIN
    SET NOCOUNT ON;
    
    UPDATE CorridorPartners
    SET IsActive = 0
    WHERE CorridorId = @CorridorId AND PartnerId = @PartnerId AND PayoutMethod = @PayoutMethod;
    
    SELECT 1 AS Success;
END;
GO

-- Get partners for corridor
CREATE PROCEDURE usp_GetPartnersForCorridor
    @CorridorId     INT,
    @PayoutMethod   NVARCHAR(50) = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    SELECT 
        cp.Id,
        cp.PartnerId,
        cp.PayoutMethod,
        cp.Priority,
        cp.IsActive,
        p.PartnerCode,
        p.PartnerName,
        p.PartnerType,
        p.IntegrationType,
        p.Status AS PartnerStatus
    FROM CorridorPartners cp
    JOIN PayoutPartners p ON cp.PartnerId = p.PartnerId
    WHERE cp.CorridorId = @CorridorId
    AND cp.IsActive = 1
    AND p.Status = 'ACTIVE'
    AND (@PayoutMethod IS NULL OR cp.PayoutMethod = @PayoutMethod)
    ORDER BY cp.Priority ASC;
END;
GO

-- Get best partner for transfer
CREATE PROCEDURE usp_GetBestPartnerForTransfer
    @CorridorId     INT,
    @PayoutMethod   NVARCHAR(50),
    @Amount         DECIMAL(18,2)
AS
BEGIN
    SET NOCOUNT ON;
    
    SELECT TOP 1
        p.PartnerId,
        p.PartnerCode,
        p.PartnerName,
        p.IntegrationType,
        p.CreditLimit,
        p.CurrentBalance,
        p.PartnerFeeType,
        p.PartnerFee
    FROM CorridorPartners cp
    JOIN PayoutPartners p ON cp.PartnerId = p.PartnerId
    WHERE cp.CorridorId = @CorridorId
    AND cp.PayoutMethod = @PayoutMethod
    AND cp.IsActive = 1
    AND p.Status = 'ACTIVE'
    AND (p.CreditLimit IS NULL OR p.CurrentBalance + @Amount <= p.CreditLimit)
    ORDER BY cp.Priority ASC;
END;
GO

-- ============================================================================
-- SETTLEMENTS
-- ============================================================================

-- Create settlement record
CREATE PROCEDURE usp_CreateSettlement
    @PartnerId          INT,
    @PeriodStart        DATETIME2,
    @PeriodEnd          DATETIME2
AS
BEGIN
    SET NOCOUNT ON;
    BEGIN TRANSACTION;
    
    DECLARE @SettlementCurrency CHAR(3);
    SELECT @SettlementCurrency = SettlementCurrency FROM PayoutPartners WHERE PartnerId = @PartnerId;
    
    -- Calculate settlement amounts
    DECLARE @TotalTransactions INT;
    DECLARE @TotalAmount DECIMAL(18,2);
    DECLARE @PartnerFees DECIMAL(18,2);
    
    SELECT 
        @TotalTransactions = COUNT(*),
        @TotalAmount = COALESCE(SUM(ReceiveAmount), 0)
    FROM Transfers
    WHERE PayoutPartnerId = @PartnerId
    AND Status = 'COMPLETED'
    AND CompletedAt >= @PeriodStart
    AND CompletedAt < @PeriodEnd;
    
    -- Calculate partner fees
    SELECT @PartnerFees = 
        CASE PartnerFeeType
            WHEN 'FLAT' THEN PartnerFee * @TotalTransactions
            WHEN 'PERCENTAGE' THEN @TotalAmount * PartnerFee
            ELSE 0
        END
    FROM PayoutPartners WHERE PartnerId = @PartnerId;
    
    DECLARE @NetSettlement DECIMAL(18,2) = @TotalAmount - @PartnerFees;
    DECLARE @Direction NVARCHAR(20) = CASE WHEN @NetSettlement > 0 THEN 'PAYABLE' ELSE 'RECEIVABLE' END;
    
    INSERT INTO PartnerSettlements (
        PartnerId, PeriodStart, PeriodEnd, TotalTransactions, TotalAmount,
        Currency, PartnerFees, NetSettlement, SettlementDirection, Status
    )
    VALUES (
        @PartnerId, @PeriodStart, @PeriodEnd, @TotalTransactions, @TotalAmount,
        @SettlementCurrency, @PartnerFees, ABS(@NetSettlement), @Direction, 'PENDING'
    );
    
    DECLARE @SettlementId BIGINT = SCOPE_IDENTITY();
    
    COMMIT;
    
    SELECT 
        1 AS Success, 
        @SettlementId AS SettlementId,
        @TotalTransactions AS TotalTransactions,
        @TotalAmount AS TotalAmount,
        @PartnerFees AS PartnerFees,
        @NetSettlement AS NetSettlement,
        @Direction AS Direction;
END;
GO

-- Get settlement by ID
CREATE PROCEDURE usp_GetSettlementById
    @SettlementId BIGINT
AS
BEGIN
    SET NOCOUNT ON;
    
    SELECT 
        s.SettlementId,
        s.PartnerId,
        s.PeriodStart,
        s.PeriodEnd,
        s.TotalTransactions,
        s.TotalAmount,
        s.Currency,
        s.PartnerFees,
        s.NetSettlement,
        s.SettlementDirection,
        s.Status,
        s.PaymentReference,
        s.PaidAt,
        s.CreatedAt,
        s.UpdatedAt,
        p.PartnerCode,
        p.PartnerName
    FROM PartnerSettlements s
    JOIN PayoutPartners p ON s.PartnerId = p.PartnerId
    WHERE s.SettlementId = @SettlementId;
END;
GO

-- List settlements by partner
CREATE PROCEDURE usp_ListSettlementsByPartner
    @PartnerId      INT,
    @Status         NVARCHAR(30) = NULL,
    @PageNumber     INT = 1,
    @PageSize       INT = 50
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @Offset INT = (@PageNumber - 1) * @PageSize;
    
    SELECT 
        SettlementId,
        PeriodStart,
        PeriodEnd,
        TotalTransactions,
        TotalAmount,
        Currency,
        PartnerFees,
        NetSettlement,
        SettlementDirection,
        Status,
        PaymentReference,
        PaidAt,
        CreatedAt
    FROM PartnerSettlements
    WHERE PartnerId = @PartnerId
    AND (@Status IS NULL OR Status = @Status)
    ORDER BY PeriodEnd DESC
    OFFSET @Offset ROWS
    FETCH NEXT @PageSize ROWS ONLY;
END;
GO

-- Mark settlement as invoiced
CREATE PROCEDURE usp_MarkSettlementInvoiced
    @SettlementId       BIGINT,
    @InvoiceReference   NVARCHAR(100)
AS
BEGIN
    SET NOCOUNT ON;
    
    UPDATE PartnerSettlements
    SET Status = 'INVOICED',
        PaymentReference = @InvoiceReference,
        UpdatedAt = SYSUTCDATETIME()
    WHERE SettlementId = @SettlementId AND Status = 'PENDING';
    
    IF @@ROWCOUNT = 0
    BEGIN
        SELECT 0 AS Success, 'INVALID_STATUS' AS ErrorCode;
        RETURN;
    END
    
    SELECT 1 AS Success;
END;
GO

-- Mark settlement as paid
CREATE PROCEDURE usp_MarkSettlementPaid
    @SettlementId       BIGINT,
    @PaymentReference   NVARCHAR(100)
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @PartnerId INT;
    DECLARE @NetSettlement DECIMAL(18,2);
    DECLARE @Direction NVARCHAR(20);
    
    SELECT 
        @PartnerId = PartnerId,
        @NetSettlement = NetSettlement,
        @Direction = SettlementDirection
    FROM PartnerSettlements 
    WHERE SettlementId = @SettlementId AND Status = 'INVOICED';
    
    IF @PartnerId IS NULL
    BEGIN
        SELECT 0 AS Success, 'INVALID_STATUS' AS ErrorCode;
        RETURN;
    END
    
    UPDATE PartnerSettlements
    SET Status = 'PAID',
        PaymentReference = @PaymentReference,
        PaidAt = SYSUTCDATETIME(),
        UpdatedAt = SYSUTCDATETIME()
    WHERE SettlementId = @SettlementId;
    
    -- Update partner balance
    IF @Direction = 'PAYABLE'
        UPDATE PayoutPartners SET CurrentBalance = CurrentBalance - @NetSettlement WHERE PartnerId = @PartnerId;
    ELSE
        UPDATE PayoutPartners SET CurrentBalance = CurrentBalance + @NetSettlement WHERE PartnerId = @PartnerId;
    
    SELECT 1 AS Success;
END;
GO

-- Reconcile settlement
CREATE PROCEDURE usp_ReconcileSettlement
    @SettlementId   BIGINT,
    @ReconciledBy   NVARCHAR(100),
    @Notes          NVARCHAR(500) = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    UPDATE PartnerSettlements
    SET Status = 'RECONCILED',
        UpdatedAt = SYSUTCDATETIME()
    WHERE SettlementId = @SettlementId AND Status = 'PAID';
    
    IF @@ROWCOUNT = 0
    BEGIN
        SELECT 0 AS Success, 'INVALID_STATUS' AS ErrorCode;
        RETURN;
    END
    
    INSERT INTO AuditLog (ActorType, ActorId, ActionType, EntityType, EntityId, NewValues)
    VALUES ('AGENT', @ReconciledBy, 'RECONCILE_SETTLEMENT', 'SETTLEMENT', @SettlementId, 
            '{"notes":"' + COALESCE(@Notes, '') + '"}');
    
    SELECT 1 AS Success;
END;
GO

-- Dispute settlement
CREATE PROCEDURE usp_DisputeSettlement
    @SettlementId   BIGINT,
    @DisputeReason  NVARCHAR(500),
    @DisputedBy     NVARCHAR(100)
AS
BEGIN
    SET NOCOUNT ON;
    
    UPDATE PartnerSettlements
    SET Status = 'DISPUTED',
        UpdatedAt = SYSUTCDATETIME()
    WHERE SettlementId = @SettlementId AND Status IN ('INVOICED', 'PAID');
    
    IF @@ROWCOUNT = 0
    BEGIN
        SELECT 0 AS Success, 'INVALID_STATUS' AS ErrorCode;
        RETURN;
    END
    
    INSERT INTO AuditLog (ActorType, ActorId, ActionType, EntityType, EntityId, NewValues)
    VALUES ('AGENT', @DisputedBy, 'DISPUTE_SETTLEMENT', 'SETTLEMENT', @SettlementId, 
            '{"reason":"' + @DisputeReason + '"}');
    
    SELECT 1 AS Success;
END;
GO

-- Get pending settlements
CREATE PROCEDURE usp_GetPendingSettlements
    @Status NVARCHAR(30) = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    SELECT 
        s.SettlementId,
        s.PartnerId,
        s.PeriodStart,
        s.PeriodEnd,
        s.TotalTransactions,
        s.TotalAmount,
        s.Currency,
        s.NetSettlement,
        s.SettlementDirection,
        s.Status,
        s.CreatedAt,
        p.PartnerCode,
        p.PartnerName
    FROM PartnerSettlements s
    JOIN PayoutPartners p ON s.PartnerId = p.PartnerId
    WHERE s.Status IN ('PENDING', 'INVOICED')
    AND (@Status IS NULL OR s.Status = @Status)
    ORDER BY s.PeriodEnd ASC;
END;
GO

-- Generate daily settlements for all partners
CREATE PROCEDURE usp_GenerateDailySettlements
    @SettlementDate DATE = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    SET @SettlementDate = COALESCE(@SettlementDate, DATEADD(DAY, -1, CAST(SYSUTCDATETIME() AS DATE)));
    
    DECLARE @PeriodStart DATETIME2 = CAST(@SettlementDate AS DATETIME2);
    DECLARE @PeriodEnd DATETIME2 = DATEADD(DAY, 1, @PeriodStart);
    
    DECLARE @PartnerId INT;
    DECLARE @GeneratedCount INT = 0;
    
    DECLARE partner_cursor CURSOR FOR
    SELECT PartnerId FROM PayoutPartners WHERE Status = 'ACTIVE' AND SettlementFrequency = 'DAILY';
    
    OPEN partner_cursor;
    FETCH NEXT FROM partner_cursor INTO @PartnerId;
    
    WHILE @@FETCH_STATUS = 0
    BEGIN
        -- Check if settlement already exists
        IF NOT EXISTS (SELECT 1 FROM PartnerSettlements 
                       WHERE PartnerId = @PartnerId 
                       AND PeriodStart = @PeriodStart)
        BEGIN
            -- Check if there are any completed transfers
            IF EXISTS (SELECT 1 FROM Transfers 
                       WHERE PayoutPartnerId = @PartnerId 
                       AND Status = 'COMPLETED'
                       AND CompletedAt >= @PeriodStart 
                       AND CompletedAt < @PeriodEnd)
            BEGIN
                EXEC usp_CreateSettlement @PartnerId, @PeriodStart, @PeriodEnd;
                SET @GeneratedCount = @GeneratedCount + 1;
            END
        END
        
        FETCH NEXT FROM partner_cursor INTO @PartnerId;
    END
    
    CLOSE partner_cursor;
    DEALLOCATE partner_cursor;
    
    SELECT @GeneratedCount AS SettlementsGenerated, @SettlementDate AS SettlementDate;
END;
GO

-- Get partner transfer summary
CREATE PROCEDURE usp_GetPartnerTransferSummary
    @PartnerId      INT,
    @StartDate      DATETIME2,
    @EndDate        DATETIME2 = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    SET @EndDate = COALESCE(@EndDate, SYSUTCDATETIME());
    
    SELECT 
        COUNT(*) AS TotalTransfers,
        SUM(CASE WHEN Status = 'COMPLETED' THEN 1 ELSE 0 END) AS CompletedTransfers,
        SUM(CASE WHEN Status = 'FAILED' THEN 1 ELSE 0 END) AS FailedTransfers,
        SUM(CASE WHEN Status IN ('SENT_TO_PARTNER', 'PAYOUT_PENDING') THEN 1 ELSE 0 END) AS PendingTransfers,
        SUM(ReceiveAmount) AS TotalVolume,
        AVG(ReceiveAmount) AS AvgTransferAmount,
        MIN(CreatedAt) AS FirstTransferDate,
        MAX(CreatedAt) AS LastTransferDate
    FROM Transfers
    WHERE PayoutPartnerId = @PartnerId
    AND CreatedAt >= @StartDate
    AND CreatedAt <= @EndDate;
    
    -- By corridor
    SELECT 
        c.CorridorCode,
        c.DisplayName,
        COUNT(*) AS TransferCount,
        SUM(t.ReceiveAmount) AS Volume
    FROM Transfers t
    JOIN Corridors c ON t.CorridorId = c.CorridorId
    WHERE t.PayoutPartnerId = @PartnerId
    AND t.CreatedAt >= @StartDate
    AND t.CreatedAt <= @EndDate
    GROUP BY c.CorridorId, c.CorridorCode, c.DisplayName
    ORDER BY Volume DESC;
    
    -- By payout method
    SELECT 
        PayoutMethod,
        COUNT(*) AS TransferCount,
        SUM(ReceiveAmount) AS Volume
    FROM Transfers
    WHERE PayoutPartnerId = @PartnerId
    AND CreatedAt >= @StartDate
    AND CreatedAt <= @EndDate
    GROUP BY PayoutMethod;
END;
GO

-- Check partner credit availability
CREATE PROCEDURE usp_CheckPartnerCreditAvailability
    @PartnerId      INT,
    @Amount         DECIMAL(18,2)
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @CreditLimit DECIMAL(18,2);
    DECLARE @CurrentBalance DECIMAL(18,2);
    DECLARE @Status NVARCHAR(20);
    
    SELECT 
        @CreditLimit = CreditLimit,
        @CurrentBalance = CurrentBalance,
        @Status = Status
    FROM PayoutPartners WHERE PartnerId = @PartnerId;
    
    IF @Status != 'ACTIVE'
    BEGIN
        SELECT 0 AS IsAvailable, 'PARTNER_NOT_ACTIVE' AS Reason;
        RETURN;
    END
    
    IF @CreditLimit IS NULL
    BEGIN
        SELECT 1 AS IsAvailable, 'NO_LIMIT' AS Reason, NULL AS AvailableCredit;
        RETURN;
    END
    
    DECLARE @AvailableCredit DECIMAL(18,2) = @CreditLimit - @CurrentBalance;
    
    IF @AvailableCredit >= @Amount
    BEGIN
        SELECT 1 AS IsAvailable, @AvailableCredit AS AvailableCredit;
    END
    ELSE
    BEGIN
        SELECT 0 AS IsAvailable, 'INSUFFICIENT_CREDIT' AS Reason, @AvailableCredit AS AvailableCredit;
    END
END;
GO
