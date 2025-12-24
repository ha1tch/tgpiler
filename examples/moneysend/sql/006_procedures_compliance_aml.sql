-- ============================================================================
-- MoneySend Stored Procedures
-- Part 5: Compliance, AML & Risk Management
-- ============================================================================

-- ============================================================================
-- COMPLIANCE SCREENING
-- ============================================================================

-- Screen transfer for compliance
CREATE PROCEDURE usp_ScreenTransferForCompliance
    @TransferId         BIGINT,
    @ScreeningProvider  NVARCHAR(50) = 'INTERNAL'
AS
BEGIN
    SET NOCOUNT ON;
    BEGIN TRANSACTION;
    
    DECLARE @CustomerId BIGINT;
    DECLARE @BeneficiaryId BIGINT;
    DECLARE @SendAmount DECIMAL(18,2);
    DECLARE @CorridorId INT;
    DECLARE @CustomerRiskLevel NVARCHAR(20);
    DECLARE @BeneficiaryScreeningStatus NVARCHAR(20);
    DECLARE @DestinationCountry CHAR(2);
    DECLARE @CountryRiskLevel NVARCHAR(20);
    
    SELECT 
        @CustomerId = t.CustomerId,
        @BeneficiaryId = t.BeneficiaryId,
        @SendAmount = t.SendAmount,
        @CorridorId = t.CorridorId
    FROM Transfers t WHERE t.TransferId = @TransferId;
    
    SELECT @CustomerRiskLevel = RiskLevel FROM Customers WHERE CustomerId = @CustomerId;
    SELECT @BeneficiaryScreeningStatus = ScreeningStatus, @DestinationCountry = Country 
    FROM Beneficiaries WHERE BeneficiaryId = @BeneficiaryId;
    SELECT @CountryRiskLevel = RiskLevel FROM CountryConfiguration WHERE CountryCode = @DestinationCountry;
    
    -- Calculate risk score based on multiple factors
    DECLARE @RiskScore DECIMAL(5,2) = 0;
    DECLARE @RiskFactors NVARCHAR(MAX) = '';
    
    -- Customer risk
    IF @CustomerRiskLevel = 'HIGH'
    BEGIN
        SET @RiskScore = @RiskScore + 30;
        SET @RiskFactors = @RiskFactors + 'HIGH_RISK_CUSTOMER;';
    END
    ELSE IF @CustomerRiskLevel = 'MEDIUM'
    BEGIN
        SET @RiskScore = @RiskScore + 15;
        SET @RiskFactors = @RiskFactors + 'MEDIUM_RISK_CUSTOMER;';
    END
    
    -- Beneficiary screening
    IF @BeneficiaryScreeningStatus = 'FLAGGED'
    BEGIN
        SET @RiskScore = @RiskScore + 25;
        SET @RiskFactors = @RiskFactors + 'FLAGGED_BENEFICIARY;';
    END
    ELSE IF @BeneficiaryScreeningStatus = 'PENDING'
    BEGIN
        SET @RiskScore = @RiskScore + 10;
        SET @RiskFactors = @RiskFactors + 'UNSCREENED_BENEFICIARY;';
    END
    
    -- Country risk
    IF @CountryRiskLevel = 'HIGH'
    BEGIN
        SET @RiskScore = @RiskScore + 20;
        SET @RiskFactors = @RiskFactors + 'HIGH_RISK_COUNTRY;';
    END
    
    -- Amount-based risk
    IF @SendAmount > 5000
    BEGIN
        SET @RiskScore = @RiskScore + 15;
        SET @RiskFactors = @RiskFactors + 'LARGE_AMOUNT;';
    END
    ELSE IF @SendAmount > 2000
    BEGIN
        SET @RiskScore = @RiskScore + 5;
    END
    
    -- Velocity check (multiple transfers in short period)
    DECLARE @RecentTransferCount INT;
    SELECT @RecentTransferCount = COUNT(*) FROM Transfers
    WHERE CustomerId = @CustomerId 
    AND CreatedAt >= DATEADD(HOUR, -24, SYSUTCDATETIME())
    AND TransferId != @TransferId;
    
    IF @RecentTransferCount >= 3
    BEGIN
        SET @RiskScore = @RiskScore + 20;
        SET @RiskFactors = @RiskFactors + 'HIGH_VELOCITY;';
    END
    
    -- First-time beneficiary
    IF NOT EXISTS (SELECT 1 FROM Transfers 
                   WHERE CustomerId = @CustomerId 
                   AND BeneficiaryId = @BeneficiaryId 
                   AND TransferId != @TransferId 
                   AND Status = 'COMPLETED')
    BEGIN
        SET @RiskScore = @RiskScore + 5;
        SET @RiskFactors = @RiskFactors + 'NEW_BENEFICIARY;';
    END
    
    -- Record screening result
    DECLARE @Result NVARCHAR(20);
    SET @Result = CASE 
        WHEN @RiskScore >= 50 THEN 'MATCH'
        WHEN @RiskScore >= 25 THEN 'POTENTIAL_MATCH'
        ELSE 'CLEAR'
    END;
    
    INSERT INTO ComplianceScreenings (
        EntityType, EntityId, TransferId, ScreeningType, ScreeningProvider,
        Result, MatchScore, MatchDetails, ResolutionStatus, ScreenedAt
    )
    VALUES (
        'TRANSFER', @TransferId, @TransferId, 'TRANSACTION_MONITORING', @ScreeningProvider,
        @Result, @RiskScore, @RiskFactors,
        CASE WHEN @Result != 'CLEAR' THEN 'PENDING_REVIEW' ELSE NULL END,
        SYSUTCDATETIME()
    );
    
    -- Update transfer with risk score
    UPDATE Transfers
    SET RiskScore = @RiskScore,
        ComplianceStatus = CASE 
            WHEN @Result = 'CLEAR' THEN 'CLEARED'
            ELSE 'FLAGGED'
        END,
        UpdatedAt = SYSUTCDATETIME()
    WHERE TransferId = @TransferId;
    
    -- If high risk, automatically put on hold
    IF @RiskScore >= 50
    BEGIN
        UPDATE Transfers
        SET Status = 'ON_HOLD',
            SubStatus = 'HIGH_RISK',
            StatusReason = 'Flagged for compliance review: ' + @RiskFactors
        WHERE TransferId = @TransferId;
        
        INSERT INTO TransferStatusHistory (TransferId, PreviousStatus, NewStatus, SubStatus, Reason, ChangedBy)
        SELECT TransferId, Status, 'ON_HOLD', 'HIGH_RISK', 'Risk score: ' + CAST(@RiskScore AS NVARCHAR), 'COMPLIANCE_SYSTEM'
        FROM Transfers WHERE TransferId = @TransferId;
    END
    
    COMMIT;
    
    SELECT 
        1 AS Success,
        @Result AS ScreeningResult,
        @RiskScore AS RiskScore,
        @RiskFactors AS RiskFactors;
END;
GO

-- Screen customer against sanctions lists
CREATE PROCEDURE usp_ScreenCustomerSanctions
    @CustomerId         BIGINT,
    @ScreeningProvider  NVARCHAR(50) = 'INTERNAL'
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @FirstName NVARCHAR(100), @LastName NVARCHAR(100);
    DECLARE @DateOfBirth DATE, @Nationality CHAR(2);
    
    SELECT 
        @FirstName = FirstName,
        @LastName = LastName,
        @DateOfBirth = DateOfBirth,
        @Nationality = Nationality
    FROM Customers WHERE CustomerId = @CustomerId;
    
    -- In production, this would call external screening service
    -- Simulating a clear result for demo
    DECLARE @Result NVARCHAR(20) = 'CLEAR';
    DECLARE @MatchScore DECIMAL(5,2) = 0;
    
    INSERT INTO ComplianceScreenings (
        EntityType, EntityId, ScreeningType, ScreeningProvider,
        Result, MatchScore, ScreenedAt
    )
    VALUES (
        'CUSTOMER', @CustomerId, 'SANCTIONS', @ScreeningProvider,
        @Result, @MatchScore, SYSUTCDATETIME()
    );
    
    SELECT 1 AS Success, @Result AS ScreeningResult, @MatchScore AS MatchScore;
END;
GO

-- Screen customer for PEP (Politically Exposed Person)
CREATE PROCEDURE usp_ScreenCustomerPEP
    @CustomerId         BIGINT,
    @ScreeningProvider  NVARCHAR(50) = 'INTERNAL'
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @FirstName NVARCHAR(100), @LastName NVARCHAR(100);
    DECLARE @CountryOfResidence CHAR(2);
    
    SELECT 
        @FirstName = FirstName,
        @LastName = LastName,
        @CountryOfResidence = CountryOfResidence
    FROM Customers WHERE CustomerId = @CustomerId;
    
    -- Simulating screening
    DECLARE @Result NVARCHAR(20) = 'CLEAR';
    DECLARE @MatchScore DECIMAL(5,2) = 0;
    
    INSERT INTO ComplianceScreenings (
        EntityType, EntityId, ScreeningType, ScreeningProvider,
        Result, MatchScore, ScreenedAt
    )
    VALUES (
        'CUSTOMER', @CustomerId, 'PEP', @ScreeningProvider,
        @Result, @MatchScore, SYSUTCDATETIME()
    );
    
    SELECT 1 AS Success, @Result AS ScreeningResult;
END;
GO

-- Resolve compliance screening
CREATE PROCEDURE usp_ResolveComplianceScreening
    @ScreeningId        BIGINT,
    @Resolution         NVARCHAR(30),  -- TRUE_POSITIVE, FALSE_POSITIVE, ESCALATED
    @ResolvedBy         NVARCHAR(100),
    @Notes              NVARCHAR(1000) = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @EntityType NVARCHAR(30);
    DECLARE @EntityId BIGINT;
    DECLARE @TransferId BIGINT;
    
    SELECT 
        @EntityType = EntityType,
        @EntityId = EntityId,
        @TransferId = TransferId
    FROM ComplianceScreenings WHERE ScreeningId = @ScreeningId;
    
    IF @EntityType IS NULL
    BEGIN
        SELECT 0 AS Success, 'SCREENING_NOT_FOUND' AS ErrorCode;
        RETURN;
    END
    
    UPDATE ComplianceScreenings
    SET ResolutionStatus = @Resolution,
        ResolvedBy = @ResolvedBy,
        ResolvedAt = SYSUTCDATETIME(),
        ResolutionNotes = @Notes
    WHERE ScreeningId = @ScreeningId;
    
    -- If this was a transfer screening, update transfer status
    IF @TransferId IS NOT NULL
    BEGIN
        IF @Resolution = 'FALSE_POSITIVE'
        BEGIN
            -- Clear for processing
            EXEC usp_ClearTransferForProcessing @TransferId, @ResolvedBy, @Notes;
        END
        ELSE IF @Resolution = 'TRUE_POSITIVE'
        BEGIN
            -- Block the transfer
            EXEC usp_BlockTransfer @TransferId, @ResolvedBy, 'Compliance screening confirmed: ' + COALESCE(@Notes, 'True positive match'), 1;
        END
        ELSE IF @Resolution = 'ESCALATED'
        BEGIN
            UPDATE Transfers SET SubStatus = 'ESCALATED', UpdatedAt = SYSUTCDATETIME() WHERE TransferId = @TransferId;
        END
    END
    
    INSERT INTO AgentActivityLog (AgentId, ActivityType, EntityType, EntityId, Description)
    SELECT AgentId, 'RESOLVE_SCREENING', 'SCREENING', @ScreeningId, 
           'Resolved as ' + @Resolution + ': ' + COALESCE(@Notes, '')
    FROM Agents WHERE EmployeeId = @ResolvedBy;
    
    SELECT 1 AS Success;
END;
GO

-- Get pending screenings
CREATE PROCEDURE usp_GetPendingScreenings
    @ScreeningType  NVARCHAR(50) = NULL,
    @EntityType     NVARCHAR(30) = NULL,
    @PageNumber     INT = 1,
    @PageSize       INT = 50
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @Offset INT = (@PageNumber - 1) * @PageSize;
    
    SELECT 
        cs.ScreeningId,
        cs.EntityType,
        cs.EntityId,
        cs.TransferId,
        cs.ScreeningType,
        cs.ScreeningProvider,
        cs.Result,
        cs.MatchScore,
        cs.MatchDetails,
        cs.ListsMatched,
        cs.ScreenedAt,
        DATEDIFF(MINUTE, cs.ScreenedAt, SYSUTCDATETIME()) AS MinutesPending,
        CASE cs.EntityType
            WHEN 'CUSTOMER' THEN c.Email
            WHEN 'BENEFICIARY' THEN b.Email
            ELSE NULL
        END AS EntityEmail,
        CASE cs.EntityType
            WHEN 'CUSTOMER' THEN c.FirstName + ' ' + c.LastName
            WHEN 'BENEFICIARY' THEN b.FirstName + ' ' + b.LastName
            ELSE 'Transfer #' + t.TransferNumber
        END AS EntityName,
        t.SendAmount,
        t.SendCurrency
    FROM ComplianceScreenings cs
    LEFT JOIN Customers c ON cs.EntityType = 'CUSTOMER' AND cs.EntityId = c.CustomerId
    LEFT JOIN Beneficiaries b ON cs.EntityType = 'BENEFICIARY' AND cs.EntityId = b.BeneficiaryId
    LEFT JOIN Transfers t ON cs.TransferId = t.TransferId
    WHERE cs.ResolutionStatus = 'PENDING_REVIEW'
    AND (@ScreeningType IS NULL OR cs.ScreeningType = @ScreeningType)
    AND (@EntityType IS NULL OR cs.EntityType = @EntityType)
    ORDER BY cs.MatchScore DESC, cs.ScreenedAt ASC
    OFFSET @Offset ROWS
    FETCH NEXT @PageSize ROWS ONLY;
    
    SELECT COUNT(*) AS TotalPending FROM ComplianceScreenings WHERE ResolutionStatus = 'PENDING_REVIEW';
END;
GO

-- Get screening history for entity
CREATE PROCEDURE usp_GetScreeningHistoryByEntity
    @EntityType NVARCHAR(30),
    @EntityId   BIGINT
AS
BEGIN
    SET NOCOUNT ON;
    
    SELECT 
        ScreeningId,
        TransferId,
        ScreeningType,
        ScreeningProvider,
        ProviderReference,
        Result,
        MatchScore,
        MatchDetails,
        ListsMatched,
        ResolutionStatus,
        ResolvedBy,
        ResolvedAt,
        ResolutionNotes,
        ScreenedAt
    FROM ComplianceScreenings
    WHERE EntityType = @EntityType AND EntityId = @EntityId
    ORDER BY ScreenedAt DESC;
END;
GO

-- ============================================================================
-- SUSPICIOUS ACTIVITY REPORTS (SARs)
-- ============================================================================

-- Create SAR
CREATE PROCEDURE usp_CreateSAR
    @CustomerId         BIGINT = NULL,
    @BeneficiaryId      BIGINT = NULL,
    @TransferIds        NVARCHAR(MAX) = NULL,
    @ActivityType       NVARCHAR(100),
    @ActivityDescription NVARCHAR(MAX),
    @SuspicionLevel     NVARCHAR(20),
    @TotalAmountInvolved DECIMAL(18,2) = NULL,
    @Currency           CHAR(3) = NULL,
    @CreatedBy          NVARCHAR(100)
AS
BEGIN
    SET NOCOUNT ON;
    
    -- Generate SAR number
    DECLARE @SARNumber NVARCHAR(50);
    SET @SARNumber = 'SAR-' + CAST(YEAR(SYSUTCDATETIME()) AS NVARCHAR) + '-' + 
                     RIGHT('00000' + CAST(NEXT VALUE FOR SARNumberSeq AS NVARCHAR), 5);
    
    INSERT INTO SuspiciousActivityReports (
        SARNumber, CustomerId, BeneficiaryId, TransferIds,
        ActivityType, ActivityDescription, SuspicionLevel,
        TotalAmountInvolved, Currency, Status, CreatedBy
    )
    VALUES (
        @SARNumber, @CustomerId, @BeneficiaryId, @TransferIds,
        @ActivityType, @ActivityDescription, @SuspicionLevel,
        @TotalAmountInvolved, @Currency, 'DRAFT', @CreatedBy
    );
    
    DECLARE @SARId BIGINT = SCOPE_IDENTITY();
    
    -- Log activity
    INSERT INTO AuditLog (ActorType, ActorId, ActionType, EntityType, EntityId, NewValues)
    VALUES ('AGENT', @CreatedBy, 'CREATE_SAR', 'SAR', @SARId, 
            '{"activity_type":"' + @ActivityType + '","suspicion_level":"' + @SuspicionLevel + '"}');
    
    SELECT 1 AS Success, @SARId AS SARId, @SARNumber AS SARNumber;
END;
GO

-- Create sequence for SAR numbers
CREATE SEQUENCE SARNumberSeq
    START WITH 1
    INCREMENT BY 1;
GO

-- Get SAR by ID
CREATE PROCEDURE usp_GetSARById
    @SARId BIGINT
AS
BEGIN
    SET NOCOUNT ON;
    
    SELECT 
        s.SARId,
        s.SARNumber,
        s.CustomerId,
        s.BeneficiaryId,
        s.TransferIds,
        s.ActivityType,
        s.ActivityDescription,
        s.SuspicionLevel,
        s.TotalAmountInvolved,
        s.Currency,
        s.Status,
        s.FiledWith,
        s.FilingReference,
        s.FiledAt,
        s.FiledBy,
        s.CreatedBy,
        s.CreatedAt,
        s.UpdatedAt,
        c.Email AS CustomerEmail,
        c.FirstName AS CustomerFirstName,
        c.LastName AS CustomerLastName,
        b.FirstName AS BeneficiaryFirstName,
        b.LastName AS BeneficiaryLastName
    FROM SuspiciousActivityReports s
    LEFT JOIN Customers c ON s.CustomerId = c.CustomerId
    LEFT JOIN Beneficiaries b ON s.BeneficiaryId = b.BeneficiaryId
    WHERE s.SARId = @SARId;
END;
GO

-- Update SAR
CREATE PROCEDURE usp_UpdateSAR
    @SARId              BIGINT,
    @ActivityDescription NVARCHAR(MAX) = NULL,
    @SuspicionLevel     NVARCHAR(20) = NULL,
    @TotalAmountInvolved DECIMAL(18,2) = NULL,
    @UpdatedBy          NVARCHAR(100)
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @CurrentStatus NVARCHAR(30);
    SELECT @CurrentStatus = Status FROM SuspiciousActivityReports WHERE SARId = @SARId;
    
    IF @CurrentStatus NOT IN ('DRAFT', 'PENDING_REVIEW')
    BEGIN
        SELECT 0 AS Success, 'CANNOT_UPDATE' AS ErrorCode, 
               'SAR cannot be updated in current status' AS ErrorMessage;
        RETURN;
    END
    
    UPDATE SuspiciousActivityReports
    SET ActivityDescription = COALESCE(@ActivityDescription, ActivityDescription),
        SuspicionLevel = COALESCE(@SuspicionLevel, SuspicionLevel),
        TotalAmountInvolved = COALESCE(@TotalAmountInvolved, TotalAmountInvolved),
        UpdatedAt = SYSUTCDATETIME()
    WHERE SARId = @SARId;
    
    SELECT 1 AS Success;
END;
GO

-- Submit SAR for review
CREATE PROCEDURE usp_SubmitSARForReview
    @SARId      BIGINT,
    @SubmittedBy NVARCHAR(100)
AS
BEGIN
    SET NOCOUNT ON;
    
    UPDATE SuspiciousActivityReports
    SET Status = 'PENDING_REVIEW',
        UpdatedAt = SYSUTCDATETIME()
    WHERE SARId = @SARId AND Status = 'DRAFT';
    
    IF @@ROWCOUNT = 0
    BEGIN
        SELECT 0 AS Success, 'INVALID_STATUS' AS ErrorCode;
        RETURN;
    END
    
    SELECT 1 AS Success;
END;
GO

-- File SAR with regulatory body
CREATE PROCEDURE usp_FileSAR
    @SARId              BIGINT,
    @FiledWith          NVARCHAR(100),
    @FilingReference    NVARCHAR(100),
    @FiledBy            NVARCHAR(100)
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @CurrentStatus NVARCHAR(30);
    SELECT @CurrentStatus = Status FROM SuspiciousActivityReports WHERE SARId = @SARId;
    
    IF @CurrentStatus NOT IN ('PENDING_REVIEW')
    BEGIN
        SELECT 0 AS Success, 'INVALID_STATUS' AS ErrorCode;
        RETURN;
    END
    
    UPDATE SuspiciousActivityReports
    SET Status = 'FILED',
        FiledWith = @FiledWith,
        FilingReference = @FilingReference,
        FiledAt = SYSUTCDATETIME(),
        FiledBy = @FiledBy,
        UpdatedAt = SYSUTCDATETIME()
    WHERE SARId = @SARId;
    
    INSERT INTO AuditLog (ActorType, ActorId, ActionType, EntityType, EntityId, NewValues)
    VALUES ('AGENT', @FiledBy, 'FILE_SAR', 'SAR', @SARId, 
            '{"filed_with":"' + @FiledWith + '","reference":"' + @FilingReference + '"}');
    
    SELECT 1 AS Success;
END;
GO

-- List SARs
CREATE PROCEDURE usp_ListSARs
    @Status         NVARCHAR(30) = NULL,
    @SuspicionLevel NVARCHAR(20) = NULL,
    @StartDate      DATETIME2 = NULL,
    @EndDate        DATETIME2 = NULL,
    @PageNumber     INT = 1,
    @PageSize       INT = 50
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @Offset INT = (@PageNumber - 1) * @PageSize;
    
    SELECT 
        s.SARId,
        s.SARNumber,
        s.ActivityType,
        s.SuspicionLevel,
        s.TotalAmountInvolved,
        s.Currency,
        s.Status,
        s.CreatedAt,
        s.FiledAt,
        c.Email AS CustomerEmail,
        c.FirstName + ' ' + c.LastName AS CustomerName
    FROM SuspiciousActivityReports s
    LEFT JOIN Customers c ON s.CustomerId = c.CustomerId
    WHERE (@Status IS NULL OR s.Status = @Status)
    AND (@SuspicionLevel IS NULL OR s.SuspicionLevel = @SuspicionLevel)
    AND (@StartDate IS NULL OR s.CreatedAt >= @StartDate)
    AND (@EndDate IS NULL OR s.CreatedAt <= @EndDate)
    ORDER BY s.CreatedAt DESC
    OFFSET @Offset ROWS
    FETCH NEXT @PageSize ROWS ONLY;
END;
GO

-- Get SAR statistics
CREATE PROCEDURE usp_GetSARStatistics
    @Year INT = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    SET @Year = COALESCE(@Year, YEAR(SYSUTCDATETIME()));
    
    -- By status
    SELECT 
        Status,
        COUNT(*) AS Count
    FROM SuspiciousActivityReports
    WHERE YEAR(CreatedAt) = @Year
    GROUP BY Status;
    
    -- By activity type
    SELECT 
        ActivityType,
        COUNT(*) AS Count,
        SUM(TotalAmountInvolved) AS TotalAmount
    FROM SuspiciousActivityReports
    WHERE YEAR(CreatedAt) = @Year
    GROUP BY ActivityType
    ORDER BY Count DESC;
    
    -- By month
    SELECT 
        MONTH(CreatedAt) AS Month,
        COUNT(*) AS SARsCreated,
        SUM(CASE WHEN Status = 'FILED' THEN 1 ELSE 0 END) AS SARsFiled
    FROM SuspiciousActivityReports
    WHERE YEAR(CreatedAt) = @Year
    GROUP BY MONTH(CreatedAt)
    ORDER BY Month;
END;
GO

-- ============================================================================
-- RISK ASSESSMENT
-- ============================================================================

-- Assess customer risk
CREATE PROCEDURE usp_AssessCustomerRisk
    @CustomerId     BIGINT,
    @AssessmentType NVARCHAR(50) = 'PERIODIC',
    @TriggerReason  NVARCHAR(200) = NULL,
    @AssessedBy     NVARCHAR(100) = 'SYSTEM'
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @CountryOfResidence CHAR(2);
    DECLARE @VerificationTier TINYINT;
    DECLARE @CurrentRiskLevel NVARCHAR(20);
    
    SELECT 
        @CountryOfResidence = CountryOfResidence,
        @VerificationTier = VerificationTier,
        @CurrentRiskLevel = RiskLevel
    FROM Customers WHERE CustomerId = @CustomerId;
    
    IF @CountryOfResidence IS NULL
    BEGIN
        SELECT 0 AS Success, 'CUSTOMER_NOT_FOUND' AS ErrorCode;
        RETURN;
    END
    
    -- Calculate risk scores
    DECLARE @CountryRiskScore DECIMAL(5,2) = 0;
    DECLARE @TransactionRiskScore DECIMAL(5,2) = 0;
    DECLARE @BehaviorRiskScore DECIMAL(5,2) = 0;
    DECLARE @DocumentRiskScore DECIMAL(5,2) = 0;
    
    -- Country risk
    SELECT @CountryRiskScore = CASE RiskLevel
        WHEN 'LOW' THEN 10
        WHEN 'STANDARD' THEN 25
        WHEN 'HIGH' THEN 50
        ELSE 75
    END
    FROM CountryConfiguration WHERE CountryCode = @CountryOfResidence;
    
    -- Transaction risk (based on patterns)
    SELECT @TransactionRiskScore = 
        CASE 
            WHEN COUNT(*) = 0 THEN 10
            WHEN AVG(SendAmount) > 5000 THEN 40
            WHEN AVG(SendAmount) > 2000 THEN 25
            ELSE 15
        END +
        CASE 
            WHEN SUM(CASE WHEN ComplianceStatus = 'FLAGGED' THEN 1 ELSE 0 END) > 2 THEN 30
            WHEN SUM(CASE WHEN ComplianceStatus = 'FLAGGED' THEN 1 ELSE 0 END) > 0 THEN 15
            ELSE 0
        END
    FROM Transfers
    WHERE CustomerId = @CustomerId
    AND CreatedAt >= DATEADD(MONTH, -12, SYSUTCDATETIME());
    
    -- Behavior risk (velocity, patterns)
    DECLARE @DistinctBeneficiaries INT;
    DECLARE @DistinctCorridors INT;
    
    SELECT 
        @DistinctBeneficiaries = COUNT(DISTINCT BeneficiaryId),
        @DistinctCorridors = COUNT(DISTINCT CorridorId)
    FROM Transfers
    WHERE CustomerId = @CustomerId
    AND CreatedAt >= DATEADD(MONTH, -6, SYSUTCDATETIME());
    
    SET @BehaviorRiskScore = 
        CASE WHEN @DistinctBeneficiaries > 10 THEN 25 ELSE @DistinctBeneficiaries * 2 END +
        CASE WHEN @DistinctCorridors > 5 THEN 15 ELSE @DistinctCorridors * 2 END;
    
    -- Document risk
    SET @DocumentRiskScore = CASE @VerificationTier
        WHEN 3 THEN 10
        WHEN 2 THEN 25
        ELSE 40
    END;
    
    -- Overall risk score (weighted average)
    DECLARE @OverallRiskScore DECIMAL(5,2);
    SET @OverallRiskScore = 
        (@CountryRiskScore * 0.25) +
        (@TransactionRiskScore * 0.35) +
        (@BehaviorRiskScore * 0.25) +
        (@DocumentRiskScore * 0.15);
    
    -- Determine risk level
    DECLARE @NewRiskLevel NVARCHAR(20);
    SET @NewRiskLevel = CASE
        WHEN @OverallRiskScore >= 60 THEN 'HIGH'
        WHEN @OverallRiskScore >= 35 THEN 'MEDIUM'
        ELSE 'LOW'
    END;
    
    DECLARE @RiskChanged BIT = CASE WHEN @NewRiskLevel != @CurrentRiskLevel THEN 1 ELSE 0 END;
    DECLARE @RequiresApproval BIT = CASE WHEN @NewRiskLevel = 'HIGH' AND @RiskChanged = 1 THEN 1 ELSE 0 END;
    
    -- Record assessment
    INSERT INTO CustomerRiskAssessments (
        CustomerId, AssessmentType, TriggerReason,
        CountryRiskScore, TransactionRiskScore, BehaviorRiskScore, DocumentRiskScore,
        OverallRiskScore, RiskLevel, PreviousRiskLevel, RiskLevelChanged,
        RequiresApproval, AssessedBy
    )
    VALUES (
        @CustomerId, @AssessmentType, @TriggerReason,
        @CountryRiskScore, @TransactionRiskScore, @BehaviorRiskScore, @DocumentRiskScore,
        @OverallRiskScore, @NewRiskLevel, @CurrentRiskLevel, @RiskChanged,
        @RequiresApproval, @AssessedBy
    );
    
    -- Update customer if risk level changed and doesn't require approval (or if downgrading)
    IF @RiskChanged = 1 AND (@RequiresApproval = 0 OR @NewRiskLevel != 'HIGH')
    BEGIN
        UPDATE Customers
        SET RiskLevel = @NewRiskLevel,
            RiskScore = @OverallRiskScore,
            UpdatedAt = SYSUTCDATETIME()
        WHERE CustomerId = @CustomerId;
        
        INSERT INTO CustomerVerificationHistory (CustomerId, ActionType, PreviousValue, NewValue, Reason, PerformedBy)
        VALUES (@CustomerId, 'RISK_CHANGE', @CurrentRiskLevel, @NewRiskLevel, 
                'Automated assessment. Score: ' + CAST(@OverallRiskScore AS NVARCHAR), @AssessedBy);
    END
    
    SELECT 
        1 AS Success,
        @OverallRiskScore AS OverallRiskScore,
        @NewRiskLevel AS NewRiskLevel,
        @CurrentRiskLevel AS PreviousRiskLevel,
        @RiskChanged AS RiskLevelChanged,
        @RequiresApproval AS RequiresApproval,
        @CountryRiskScore AS CountryRiskScore,
        @TransactionRiskScore AS TransactionRiskScore,
        @BehaviorRiskScore AS BehaviorRiskScore,
        @DocumentRiskScore AS DocumentRiskScore;
END;
GO

-- Approve risk level change
CREATE PROCEDURE usp_ApproveRiskLevelChange
    @AssessmentId   BIGINT,
    @ApprovedBy     NVARCHAR(100),
    @Approved       BIT,
    @Notes          NVARCHAR(500) = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @CustomerId BIGINT;
    DECLARE @NewRiskLevel NVARCHAR(20);
    
    SELECT @CustomerId = CustomerId, @NewRiskLevel = RiskLevel
    FROM CustomerRiskAssessments 
    WHERE AssessmentId = @AssessmentId AND RequiresApproval = 1 AND ApprovedAt IS NULL;
    
    IF @CustomerId IS NULL
    BEGIN
        SELECT 0 AS Success, 'ASSESSMENT_NOT_FOUND' AS ErrorCode;
        RETURN;
    END
    
    UPDATE CustomerRiskAssessments
    SET ApprovedBy = @ApprovedBy,
        ApprovedAt = SYSUTCDATETIME(),
        RecommendedActions = @Notes
    WHERE AssessmentId = @AssessmentId;
    
    IF @Approved = 1
    BEGIN
        UPDATE Customers
        SET RiskLevel = @NewRiskLevel,
            UpdatedAt = SYSUTCDATETIME()
        WHERE CustomerId = @CustomerId;
        
        INSERT INTO CustomerVerificationHistory (CustomerId, ActionType, NewValue, Reason, PerformedBy)
        VALUES (@CustomerId, 'RISK_CHANGE_APPROVED', @NewRiskLevel, @Notes, @ApprovedBy);
    END
    ELSE
    BEGIN
        INSERT INTO CustomerVerificationHistory (CustomerId, ActionType, NewValue, Reason, PerformedBy)
        VALUES (@CustomerId, 'RISK_CHANGE_REJECTED', 'REJECTED', @Notes, @ApprovedBy);
    END
    
    SELECT 1 AS Success;
END;
GO

-- Get customer risk assessments
CREATE PROCEDURE usp_GetCustomerRiskAssessments
    @CustomerId BIGINT
AS
BEGIN
    SET NOCOUNT ON;
    
    SELECT 
        AssessmentId,
        AssessmentType,
        TriggerReason,
        CountryRiskScore,
        TransactionRiskScore,
        BehaviorRiskScore,
        DocumentRiskScore,
        OverallRiskScore,
        RiskLevel,
        PreviousRiskLevel,
        RiskLevelChanged,
        RequiresApproval,
        ApprovedBy,
        ApprovedAt,
        RecommendedActions,
        AssessedBy,
        AssessedAt
    FROM CustomerRiskAssessments
    WHERE CustomerId = @CustomerId
    ORDER BY AssessedAt DESC;
END;
GO

-- Get pending risk approvals
CREATE PROCEDURE usp_GetPendingRiskApprovals
    @PageNumber INT = 1,
    @PageSize   INT = 50
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @Offset INT = (@PageNumber - 1) * @PageSize;
    
    SELECT 
        ra.AssessmentId,
        ra.CustomerId,
        ra.OverallRiskScore,
        ra.RiskLevel AS NewRiskLevel,
        ra.PreviousRiskLevel,
        ra.AssessedAt,
        c.Email AS CustomerEmail,
        c.FirstName,
        c.LastName,
        c.CountryOfResidence
    FROM CustomerRiskAssessments ra
    JOIN Customers c ON ra.CustomerId = c.CustomerId
    WHERE ra.RequiresApproval = 1 AND ra.ApprovedAt IS NULL
    ORDER BY ra.OverallRiskScore DESC
    OFFSET @Offset ROWS
    FETCH NEXT @PageSize ROWS ONLY;
END;
GO

-- Batch risk assessment (periodic job)
CREATE PROCEDURE usp_BatchRiskAssessment
    @AssessmentType NVARCHAR(50) = 'PERIODIC'
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @CustomerId BIGINT;
    DECLARE @AssessedCount INT = 0;
    
    -- Cursor for customers due for assessment
    DECLARE customer_cursor CURSOR FOR
    SELECT c.CustomerId
    FROM Customers c
    WHERE c.Status = 'ACTIVE'
    AND NOT EXISTS (
        SELECT 1 FROM CustomerRiskAssessments ra
        WHERE ra.CustomerId = c.CustomerId
        AND ra.AssessedAt >= DATEADD(MONTH, -3, SYSUTCDATETIME())
    )
    ORDER BY c.LastLoginAt DESC;
    
    OPEN customer_cursor;
    FETCH NEXT FROM customer_cursor INTO @CustomerId;
    
    WHILE @@FETCH_STATUS = 0 AND @AssessedCount < 1000  -- Limit per batch
    BEGIN
        EXEC usp_AssessCustomerRisk @CustomerId, @AssessmentType;
        SET @AssessedCount = @AssessedCount + 1;
        FETCH NEXT FROM customer_cursor INTO @CustomerId;
    END
    
    CLOSE customer_cursor;
    DEALLOCATE customer_cursor;
    
    SELECT @AssessedCount AS CustomersAssessed;
END;
GO
