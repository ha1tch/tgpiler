-- ============================================================================
-- MoneySend Stored Procedures
-- Part 7: Ledger, Accounting & Reconciliation
-- ============================================================================

-- ============================================================================
-- LEDGER ACCOUNT MANAGEMENT
-- ============================================================================

-- Create ledger account
CREATE PROCEDURE usp_CreateLedgerAccount
    @AccountCode        NVARCHAR(20),
    @AccountName        NVARCHAR(200),
    @AccountType        NVARCHAR(30),
    @Currency           CHAR(3),
    @ParentAccountId    INT = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    IF EXISTS (SELECT 1 FROM LedgerAccounts WHERE AccountCode = @AccountCode)
    BEGIN
        SELECT 0 AS Success, 'DUPLICATE_CODE' AS ErrorCode;
        RETURN;
    END
    
    INSERT INTO LedgerAccounts (AccountCode, AccountName, AccountType, Currency, ParentAccountId, IsActive)
    VALUES (@AccountCode, @AccountName, @AccountType, @Currency, @ParentAccountId, 1);
    
    SELECT 1 AS Success, SCOPE_IDENTITY() AS AccountId;
END;
GO

-- Get ledger account by code
CREATE PROCEDURE usp_GetLedgerAccountByCode
    @AccountCode NVARCHAR(20)
AS
BEGIN
    SET NOCOUNT ON;
    
    SELECT 
        AccountId,
        AccountCode,
        AccountName,
        AccountType,
        Currency,
        ParentAccountId,
        IsActive
    FROM LedgerAccounts
    WHERE AccountCode = @AccountCode;
END;
GO

-- List ledger accounts
CREATE PROCEDURE usp_ListLedgerAccounts
    @AccountType NVARCHAR(30) = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    SELECT 
        la.AccountId,
        la.AccountCode,
        la.AccountName,
        la.AccountType,
        la.Currency,
        la.ParentAccountId,
        pa.AccountCode AS ParentAccountCode,
        la.IsActive
    FROM LedgerAccounts la
    LEFT JOIN LedgerAccounts pa ON la.ParentAccountId = pa.AccountId
    WHERE (@AccountType IS NULL OR la.AccountType = @AccountType)
    AND la.IsActive = 1
    ORDER BY la.AccountType, la.AccountCode;
END;
GO

-- Get account balance
CREATE PROCEDURE usp_GetLedgerAccountBalance
    @AccountId      INT,
    @AsOfDate       DATETIME2 = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    SET @AsOfDate = COALESCE(@AsOfDate, SYSUTCDATETIME());
    
    DECLARE @DebitTotal DECIMAL(18,2);
    DECLARE @CreditTotal DECIMAL(18,2);
    
    SELECT 
        @DebitTotal = COALESCE(SUM(CASE WHEN EntryType = 'DEBIT' THEN Amount ELSE 0 END), 0),
        @CreditTotal = COALESCE(SUM(CASE WHEN EntryType = 'CREDIT' THEN Amount ELSE 0 END), 0)
    FROM LedgerEntries
    WHERE AccountId = @AccountId
    AND CreatedAt <= @AsOfDate;
    
    DECLARE @AccountType NVARCHAR(30);
    SELECT @AccountType = AccountType FROM LedgerAccounts WHERE AccountId = @AccountId;
    
    -- Calculate balance based on account type
    DECLARE @Balance DECIMAL(18,2);
    IF @AccountType IN ('ASSET', 'EXPENSE')
        SET @Balance = @DebitTotal - @CreditTotal;
    ELSE -- LIABILITY, REVENUE, EQUITY
        SET @Balance = @CreditTotal - @DebitTotal;
    
    SELECT 
        @AccountId AS AccountId,
        @DebitTotal AS TotalDebits,
        @CreditTotal AS TotalCredits,
        @Balance AS Balance,
        @AsOfDate AS AsOfDate;
END;
GO

-- ============================================================================
-- LEDGER ENTRIES
-- ============================================================================

-- Record ledger entry (double-entry)
CREATE PROCEDURE usp_RecordLedgerEntry
    @DebitAccountId     INT,
    @CreditAccountId    INT,
    @Amount             DECIMAL(18,2),
    @Currency           CHAR(3),
    @ReferenceType      NVARCHAR(50),
    @ReferenceId        BIGINT,
    @Description        NVARCHAR(500) = NULL,
    @CreatedBy          NVARCHAR(100) = 'SYSTEM'
AS
BEGIN
    SET NOCOUNT ON;
    BEGIN TRANSACTION;
    
    -- Generate journal ID (groups the related entries)
    DECLARE @JournalId BIGINT = NEXT VALUE FOR JournalIdSeq;
    
    -- Debit entry
    INSERT INTO LedgerEntries (JournalId, AccountId, EntryType, Amount, Currency, ReferenceType, ReferenceId, Description, CreatedBy)
    VALUES (@JournalId, @DebitAccountId, 'DEBIT', @Amount, @Currency, @ReferenceType, @ReferenceId, @Description, @CreatedBy);
    
    -- Credit entry
    INSERT INTO LedgerEntries (JournalId, AccountId, EntryType, Amount, Currency, ReferenceType, ReferenceId, Description, CreatedBy)
    VALUES (@JournalId, @CreditAccountId, 'CREDIT', @Amount, @Currency, @ReferenceType, @ReferenceId, @Description, @CreatedBy);
    
    COMMIT;
    
    SELECT 1 AS Success, @JournalId AS JournalId;
END;
GO

-- Create sequence for journal IDs
CREATE SEQUENCE JournalIdSeq
    START WITH 1
    INCREMENT BY 1;
GO

-- Record transfer accounting entries
CREATE PROCEDURE usp_RecordTransferAccounting
    @TransferId BIGINT
AS
BEGIN
    SET NOCOUNT ON;
    BEGIN TRANSACTION;
    
    DECLARE @SendAmount DECIMAL(18,2);
    DECLARE @SendCurrency CHAR(3);
    DECLARE @TotalFees DECIMAL(18,2);
    DECLARE @TotalCharged DECIMAL(18,2);
    
    SELECT 
        @SendAmount = SendAmount,
        @SendCurrency = SendCurrency,
        @TotalFees = TotalFees,
        @TotalCharged = TotalCharged
    FROM Transfers WHERE TransferId = @TransferId;
    
    -- Get account IDs (would be configured in production)
    DECLARE @CustomerReceivableAccount INT = 1001;  -- Customer funds received
    DECLARE @FeeRevenueAccount INT = 4001;          -- Fee revenue
    DECLARE @TransferPayableAccount INT = 2001;     -- Amount payable to beneficiary
    
    -- Record customer payment received
    -- DR: Cash/Bank (Asset)
    -- CR: Customer Receivable
    DECLARE @CashAccount INT = 1000;
    EXEC usp_RecordLedgerEntry @CashAccount, @CustomerReceivableAccount, @TotalCharged, @SendCurrency, 'TRANSFER', @TransferId, 'Customer payment received';
    
    -- Record fee revenue
    -- DR: Customer Receivable
    -- CR: Fee Revenue
    EXEC usp_RecordLedgerEntry @CustomerReceivableAccount, @FeeRevenueAccount, @TotalFees, @SendCurrency, 'TRANSFER', @TransferId, 'Transfer fee revenue';
    
    -- Record transfer payable
    -- DR: Customer Receivable
    -- CR: Transfer Payable
    EXEC usp_RecordLedgerEntry @CustomerReceivableAccount, @TransferPayableAccount, @SendAmount, @SendCurrency, 'TRANSFER', @TransferId, 'Amount payable to beneficiary';
    
    COMMIT;
    SELECT 1 AS Success;
END;
GO

-- Record payout accounting entries
CREATE PROCEDURE usp_RecordPayoutAccounting
    @TransferId BIGINT
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @ReceiveAmount DECIMAL(18,2);
    DECLARE @ReceiveCurrency CHAR(3);
    DECLARE @PartnerId INT;
    
    SELECT 
        @ReceiveAmount = ReceiveAmount,
        @ReceiveCurrency = ReceiveCurrency,
        @PartnerId = PayoutPartnerId
    FROM Transfers WHERE TransferId = @TransferId;
    
    -- Record payout to partner
    DECLARE @TransferPayableAccount INT = 2001;
    DECLARE @PartnerPayableAccount INT = 2002;
    
    EXEC usp_RecordLedgerEntry @TransferPayableAccount, @PartnerPayableAccount, @ReceiveAmount, @ReceiveCurrency, 'TRANSFER', @TransferId, 'Payout sent to partner';
    
    SELECT 1 AS Success;
END;
GO

-- Get ledger entries by reference
CREATE PROCEDURE usp_GetLedgerEntriesByReference
    @ReferenceType  NVARCHAR(50),
    @ReferenceId    BIGINT
AS
BEGIN
    SET NOCOUNT ON;
    
    SELECT 
        le.EntryId,
        le.JournalId,
        le.AccountId,
        la.AccountCode,
        la.AccountName,
        le.EntryType,
        le.Amount,
        le.Currency,
        le.Description,
        le.CreatedAt,
        le.CreatedBy
    FROM LedgerEntries le
    JOIN LedgerAccounts la ON le.AccountId = la.AccountId
    WHERE le.ReferenceType = @ReferenceType
    AND le.ReferenceId = @ReferenceId
    ORDER BY le.JournalId, le.EntryType DESC;  -- DEBIT first
END;
GO

-- Get ledger entries by account
CREATE PROCEDURE usp_GetLedgerEntriesByAccount
    @AccountId      INT,
    @StartDate      DATETIME2,
    @EndDate        DATETIME2 = NULL,
    @PageNumber     INT = 1,
    @PageSize       INT = 100
AS
BEGIN
    SET NOCOUNT ON;
    
    SET @EndDate = COALESCE(@EndDate, SYSUTCDATETIME());
    DECLARE @Offset INT = (@PageNumber - 1) * @PageSize;
    
    SELECT 
        le.EntryId,
        le.JournalId,
        le.EntryType,
        le.Amount,
        le.Currency,
        le.ReferenceType,
        le.ReferenceId,
        le.Description,
        le.CreatedAt,
        le.CreatedBy
    FROM LedgerEntries le
    WHERE le.AccountId = @AccountId
    AND le.CreatedAt >= @StartDate
    AND le.CreatedAt <= @EndDate
    ORDER BY le.CreatedAt DESC
    OFFSET @Offset ROWS
    FETCH NEXT @PageSize ROWS ONLY;
END;
GO

-- Get trial balance
CREATE PROCEDURE usp_GetTrialBalance
    @AsOfDate   DATETIME2 = NULL,
    @Currency   CHAR(3) = 'USD'
AS
BEGIN
    SET NOCOUNT ON;
    
    SET @AsOfDate = COALESCE(@AsOfDate, SYSUTCDATETIME());
    
    SELECT 
        la.AccountId,
        la.AccountCode,
        la.AccountName,
        la.AccountType,
        COALESCE(SUM(CASE WHEN le.EntryType = 'DEBIT' THEN le.Amount ELSE 0 END), 0) AS Debits,
        COALESCE(SUM(CASE WHEN le.EntryType = 'CREDIT' THEN le.Amount ELSE 0 END), 0) AS Credits,
        CASE la.AccountType
            WHEN 'ASSET' THEN COALESCE(SUM(CASE WHEN le.EntryType = 'DEBIT' THEN le.Amount ELSE -le.Amount END), 0)
            WHEN 'EXPENSE' THEN COALESCE(SUM(CASE WHEN le.EntryType = 'DEBIT' THEN le.Amount ELSE -le.Amount END), 0)
            ELSE COALESCE(SUM(CASE WHEN le.EntryType = 'CREDIT' THEN le.Amount ELSE -le.Amount END), 0)
        END AS Balance
    FROM LedgerAccounts la
    LEFT JOIN LedgerEntries le ON la.AccountId = le.AccountId 
        AND le.Currency = @Currency
        AND le.CreatedAt <= @AsOfDate
    WHERE la.IsActive = 1
    GROUP BY la.AccountId, la.AccountCode, la.AccountName, la.AccountType
    ORDER BY la.AccountType, la.AccountCode;
END;
GO

-- ============================================================================
-- DAILY RECONCILIATION
-- ============================================================================

-- Run daily reconciliation
CREATE PROCEDURE usp_RunDailyReconciliation
    @ReconciliationDate DATE = NULL,
    @Currency           CHAR(3) = 'USD'
AS
BEGIN
    SET NOCOUNT ON;
    BEGIN TRANSACTION;
    
    SET @ReconciliationDate = COALESCE(@ReconciliationDate, DATEADD(DAY, -1, CAST(SYSUTCDATETIME() AS DATE)));
    
    DECLARE @StartDate DATETIME2 = CAST(@ReconciliationDate AS DATETIME2);
    DECLARE @EndDate DATETIME2 = DATEADD(DAY, 1, @StartDate);
    
    -- Calculate expected values from transfers
    DECLARE @ExpectedTransferCount INT;
    DECLARE @ExpectedTransferVolume DECIMAL(18,2);
    DECLARE @ExpectedFeeRevenue DECIMAL(18,2);
    
    SELECT 
        @ExpectedTransferCount = COUNT(*),
        @ExpectedTransferVolume = COALESCE(SUM(SendAmount), 0),
        @ExpectedFeeRevenue = COALESCE(SUM(TotalFees), 0)
    FROM Transfers
    WHERE SendCurrency = @Currency
    AND Status = 'COMPLETED'
    AND CompletedAt >= @StartDate
    AND CompletedAt < @EndDate;
    
    -- Calculate actual values from ledger
    DECLARE @ActualTransferCount INT;
    DECLARE @ActualTransferVolume DECIMAL(18,2);
    DECLARE @ActualFeeRevenue DECIMAL(18,2);
    
    SELECT @ActualTransferCount = COUNT(DISTINCT ReferenceId)
    FROM LedgerEntries
    WHERE ReferenceType = 'TRANSFER'
    AND Currency = @Currency
    AND CreatedAt >= @StartDate
    AND CreatedAt < @EndDate;
    
    -- Get from specific accounts
    SELECT @ActualFeeRevenue = COALESCE(SUM(Amount), 0)
    FROM LedgerEntries le
    JOIN LedgerAccounts la ON le.AccountId = la.AccountId
    WHERE la.AccountType = 'REVENUE'
    AND le.EntryType = 'CREDIT'
    AND le.Currency = @Currency
    AND le.CreatedAt >= @StartDate
    AND le.CreatedAt < @EndDate;
    
    -- Calculate from transfer payable credits
    SELECT @ActualTransferVolume = COALESCE(SUM(Amount), 0)
    FROM LedgerEntries le
    WHERE le.AccountId = 2001  -- Transfer Payable
    AND le.EntryType = 'CREDIT'
    AND le.Currency = @Currency
    AND le.CreatedAt >= @StartDate
    AND le.CreatedAt < @EndDate;
    
    -- Calculate variances
    DECLARE @TransferCountVariance INT = @ExpectedTransferCount - @ActualTransferCount;
    DECLARE @TransferVolumeVariance DECIMAL(18,2) = @ExpectedTransferVolume - @ActualTransferVolume;
    DECLARE @FeeRevenueVariance DECIMAL(18,2) = @ExpectedFeeRevenue - @ActualFeeRevenue;
    
    -- Determine status
    DECLARE @Status NVARCHAR(20);
    IF @TransferCountVariance = 0 AND ABS(@TransferVolumeVariance) < 0.01 AND ABS(@FeeRevenueVariance) < 0.01
        SET @Status = 'MATCHED';
    ELSE
        SET @Status = 'VARIANCE';
    
    -- Insert or update reconciliation record
    IF EXISTS (SELECT 1 FROM DailyReconciliation WHERE ReconciliationDate = @ReconciliationDate AND Currency = @Currency)
    BEGIN
        UPDATE DailyReconciliation
        SET ExpectedTransferCount = @ExpectedTransferCount,
            ExpectedTransferVolume = @ExpectedTransferVolume,
            ExpectedFeeRevenue = @ExpectedFeeRevenue,
            ActualTransferCount = @ActualTransferCount,
            ActualTransferVolume = @ActualTransferVolume,
            ActualFeeRevenue = @ActualFeeRevenue,
            TransferCountVariance = @TransferCountVariance,
            TransferVolumeVariance = @TransferVolumeVariance,
            FeeRevenueVariance = @FeeRevenueVariance,
            Status = @Status,
            RunAt = SYSUTCDATETIME()
        WHERE ReconciliationDate = @ReconciliationDate AND Currency = @Currency;
    END
    ELSE
    BEGIN
        INSERT INTO DailyReconciliation (
            ReconciliationDate, Currency,
            ExpectedTransferCount, ExpectedTransferVolume, ExpectedFeeRevenue,
            ActualTransferCount, ActualTransferVolume, ActualFeeRevenue,
            TransferCountVariance, TransferVolumeVariance, FeeRevenueVariance,
            Status
        )
        VALUES (
            @ReconciliationDate, @Currency,
            @ExpectedTransferCount, @ExpectedTransferVolume, @ExpectedFeeRevenue,
            @ActualTransferCount, @ActualTransferVolume, @ActualFeeRevenue,
            @TransferCountVariance, @TransferVolumeVariance, @FeeRevenueVariance,
            @Status
        );
    END
    
    COMMIT;
    
    SELECT 
        1 AS Success,
        @ReconciliationDate AS ReconciliationDate,
        @Status AS Status,
        @ExpectedTransferCount AS ExpectedCount,
        @ActualTransferCount AS ActualCount,
        @TransferCountVariance AS CountVariance,
        @ExpectedTransferVolume AS ExpectedVolume,
        @ActualTransferVolume AS ActualVolume,
        @TransferVolumeVariance AS VolumeVariance;
END;
GO

-- Get reconciliation by date
CREATE PROCEDURE usp_GetReconciliationByDate
    @ReconciliationDate DATE,
    @Currency           CHAR(3) = 'USD'
AS
BEGIN
    SET NOCOUNT ON;
    
    SELECT 
        ReconciliationId,
        ReconciliationDate,
        Currency,
        ExpectedTransferCount,
        ExpectedTransferVolume,
        ExpectedFeeRevenue,
        ActualTransferCount,
        ActualTransferVolume,
        ActualFeeRevenue,
        TransferCountVariance,
        TransferVolumeVariance,
        FeeRevenueVariance,
        Status,
        ResolutionNotes,
        RunAt,
        ReviewedBy,
        ReviewedAt
    FROM DailyReconciliation
    WHERE ReconciliationDate = @ReconciliationDate
    AND Currency = @Currency;
END;
GO

-- List reconciliations with variance
CREATE PROCEDURE usp_ListReconciliationsWithVariance
    @StartDate  DATE = NULL,
    @EndDate    DATE = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    SET @StartDate = COALESCE(@StartDate, DATEADD(DAY, -30, CAST(SYSUTCDATETIME() AS DATE)));
    SET @EndDate = COALESCE(@EndDate, CAST(SYSUTCDATETIME() AS DATE));
    
    SELECT 
        ReconciliationId,
        ReconciliationDate,
        Currency,
        TransferCountVariance,
        TransferVolumeVariance,
        FeeRevenueVariance,
        Status,
        RunAt,
        ReviewedBy,
        ReviewedAt
    FROM DailyReconciliation
    WHERE ReconciliationDate >= @StartDate
    AND ReconciliationDate <= @EndDate
    AND Status IN ('VARIANCE', 'INVESTIGATING')
    ORDER BY ReconciliationDate DESC;
END;
GO

-- Resolve reconciliation variance
CREATE PROCEDURE usp_ResolveReconciliationVariance
    @ReconciliationId   BIGINT,
    @ResolutionNotes    NVARCHAR(1000),
    @ResolvedBy         NVARCHAR(100)
AS
BEGIN
    SET NOCOUNT ON;
    
    UPDATE DailyReconciliation
    SET Status = 'RESOLVED',
        ResolutionNotes = @ResolutionNotes,
        ReviewedBy = @ResolvedBy,
        ReviewedAt = SYSUTCDATETIME()
    WHERE ReconciliationId = @ReconciliationId;
    
    SELECT 1 AS Success;
END;
GO

-- Mark reconciliation as investigating
CREATE PROCEDURE usp_MarkReconciliationInvestigating
    @ReconciliationId   BIGINT,
    @InvestigatedBy     NVARCHAR(100),
    @Notes              NVARCHAR(500) = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    UPDATE DailyReconciliation
    SET Status = 'INVESTIGATING',
        ResolutionNotes = @Notes,
        ReviewedBy = @InvestigatedBy,
        ReviewedAt = SYSUTCDATETIME()
    WHERE ReconciliationId = @ReconciliationId;
    
    SELECT 1 AS Success;
END;
GO

-- Get reconciliation summary
CREATE PROCEDURE usp_GetReconciliationSummary
    @Year   INT = NULL,
    @Month  INT = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    SET @Year = COALESCE(@Year, YEAR(SYSUTCDATETIME()));
    SET @Month = COALESCE(@Month, MONTH(SYSUTCDATETIME()));
    
    SELECT 
        Currency,
        COUNT(*) AS TotalDays,
        SUM(CASE WHEN Status = 'MATCHED' THEN 1 ELSE 0 END) AS MatchedDays,
        SUM(CASE WHEN Status = 'VARIANCE' THEN 1 ELSE 0 END) AS VarianceDays,
        SUM(CASE WHEN Status = 'RESOLVED' THEN 1 ELSE 0 END) AS ResolvedDays,
        SUM(CASE WHEN Status = 'INVESTIGATING' THEN 1 ELSE 0 END) AS InvestigatingDays,
        SUM(ExpectedTransferCount) AS TotalExpectedTransfers,
        SUM(ActualTransferCount) AS TotalActualTransfers,
        SUM(ExpectedTransferVolume) AS TotalExpectedVolume,
        SUM(ActualTransferVolume) AS TotalActualVolume,
        SUM(ABS(TransferVolumeVariance)) AS TotalVolumeVariance
    FROM DailyReconciliation
    WHERE YEAR(ReconciliationDate) = @Year
    AND MONTH(ReconciliationDate) = @Month
    GROUP BY Currency;
END;
GO

-- ============================================================================
-- FINANCIAL REPORTS
-- ============================================================================

-- Get revenue summary
CREATE PROCEDURE usp_GetRevenueSummary
    @StartDate  DATETIME2,
    @EndDate    DATETIME2 = NULL,
    @Currency   CHAR(3) = 'USD'
AS
BEGIN
    SET NOCOUNT ON;
    
    SET @EndDate = COALESCE(@EndDate, SYSUTCDATETIME());
    
    -- Transfer fee revenue
    SELECT 
        'TRANSFER_FEES' AS RevenueType,
        COUNT(*) AS TransactionCount,
        SUM(TransferFee) AS GrossRevenue,
        SUM(PromoDiscount) AS Discounts,
        SUM(TransferFee - PromoDiscount) AS NetRevenue
    FROM Transfers
    WHERE SendCurrency = @Currency
    AND Status = 'COMPLETED'
    AND CompletedAt >= @StartDate
    AND CompletedAt <= @EndDate
    
    UNION ALL
    
    -- FX margin revenue
    SELECT 
        'FX_MARGIN' AS RevenueType,
        COUNT(*) AS TransactionCount,
        SUM(FXMargin) AS GrossRevenue,
        0 AS Discounts,
        SUM(FXMargin) AS NetRevenue
    FROM Transfers
    WHERE SendCurrency = @Currency
    AND Status = 'COMPLETED'
    AND CompletedAt >= @StartDate
    AND CompletedAt <= @EndDate;
    
    -- By corridor
    SELECT 
        c.CorridorCode,
        c.DisplayName,
        COUNT(*) AS TransferCount,
        SUM(t.TransferFee - t.PromoDiscount) AS FeeRevenue,
        SUM(t.FXMargin) AS FXRevenue,
        SUM(t.TransferFee - t.PromoDiscount + t.FXMargin) AS TotalRevenue
    FROM Transfers t
    JOIN Corridors c ON t.CorridorId = c.CorridorId
    WHERE t.SendCurrency = @Currency
    AND t.Status = 'COMPLETED'
    AND t.CompletedAt >= @StartDate
    AND t.CompletedAt <= @EndDate
    GROUP BY c.CorridorId, c.CorridorCode, c.DisplayName
    ORDER BY TotalRevenue DESC;
END;
GO

-- Get volume report
CREATE PROCEDURE usp_GetVolumeReport
    @StartDate  DATETIME2,
    @EndDate    DATETIME2 = NULL,
    @GroupBy    NVARCHAR(20) = 'DAY'  -- DAY, WEEK, MONTH
AS
BEGIN
    SET NOCOUNT ON;
    
    SET @EndDate = COALESCE(@EndDate, SYSUTCDATETIME());
    
    IF @GroupBy = 'DAY'
    BEGIN
        SELECT 
            CAST(CreatedAt AS DATE) AS Period,
            COUNT(*) AS TransferCount,
            SUM(SendAmount) AS SendVolume,
            SUM(ReceiveAmount) AS ReceiveVolume,
            SUM(TotalFees) AS FeeRevenue,
            COUNT(DISTINCT CustomerId) AS UniqueCustomers,
            COUNT(DISTINCT BeneficiaryId) AS UniqueBeneficiaries
        FROM Transfers
        WHERE CreatedAt >= @StartDate
        AND CreatedAt <= @EndDate
        GROUP BY CAST(CreatedAt AS DATE)
        ORDER BY Period;
    END
    ELSE IF @GroupBy = 'WEEK'
    BEGIN
        SELECT 
            DATEPART(YEAR, CreatedAt) AS Year,
            DATEPART(WEEK, CreatedAt) AS Week,
            COUNT(*) AS TransferCount,
            SUM(SendAmount) AS SendVolume,
            SUM(ReceiveAmount) AS ReceiveVolume,
            SUM(TotalFees) AS FeeRevenue
        FROM Transfers
        WHERE CreatedAt >= @StartDate
        AND CreatedAt <= @EndDate
        GROUP BY DATEPART(YEAR, CreatedAt), DATEPART(WEEK, CreatedAt)
        ORDER BY Year, Week;
    END
    ELSE IF @GroupBy = 'MONTH'
    BEGIN
        SELECT 
            DATEPART(YEAR, CreatedAt) AS Year,
            DATEPART(MONTH, CreatedAt) AS Month,
            COUNT(*) AS TransferCount,
            SUM(SendAmount) AS SendVolume,
            SUM(ReceiveAmount) AS ReceiveVolume,
            SUM(TotalFees) AS FeeRevenue
        FROM Transfers
        WHERE CreatedAt >= @StartDate
        AND CreatedAt <= @EndDate
        GROUP BY DATEPART(YEAR, CreatedAt), DATEPART(MONTH, CreatedAt)
        ORDER BY Year, Month;
    END
END;
GO

-- Get corridor performance report
CREATE PROCEDURE usp_GetCorridorPerformanceReport
    @StartDate  DATETIME2,
    @EndDate    DATETIME2 = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    SET @EndDate = COALESCE(@EndDate, SYSUTCDATETIME());
    
    SELECT 
        c.CorridorCode,
        c.DisplayName,
        c.OriginCurrency,
        c.DestinationCurrency,
        COUNT(*) AS TotalTransfers,
        SUM(CASE WHEN t.Status = 'COMPLETED' THEN 1 ELSE 0 END) AS CompletedTransfers,
        SUM(CASE WHEN t.Status = 'FAILED' THEN 1 ELSE 0 END) AS FailedTransfers,
        SUM(CASE WHEN t.Status = 'CANCELLED' THEN 1 ELSE 0 END) AS CancelledTransfers,
        CAST(SUM(CASE WHEN t.Status = 'COMPLETED' THEN 1 ELSE 0 END) * 100.0 / NULLIF(COUNT(*), 0) AS DECIMAL(5,2)) AS SuccessRate,
        SUM(t.SendAmount) AS TotalSendVolume,
        SUM(t.ReceiveAmount) AS TotalReceiveVolume,
        AVG(t.SendAmount) AS AvgTransferAmount,
        SUM(t.TotalFees) AS TotalFeeRevenue,
        SUM(t.FXMargin) AS TotalFXRevenue,
        AVG(DATEDIFF(MINUTE, t.CreatedAt, t.CompletedAt)) AS AvgCompletionMinutes
    FROM Corridors c
    LEFT JOIN Transfers t ON c.CorridorId = t.CorridorId
        AND t.CreatedAt >= @StartDate
        AND t.CreatedAt <= @EndDate
    WHERE c.IsActive = 1
    GROUP BY c.CorridorId, c.CorridorCode, c.DisplayName, c.OriginCurrency, c.DestinationCurrency
    ORDER BY TotalSendVolume DESC;
END;
GO
