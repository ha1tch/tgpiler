-- ============================================================================
-- MoneySend Stored Procedures
-- Part 1: Customer Management & Authentication
-- ============================================================================

-- ============================================================================
-- CUSTOMER REGISTRATION & AUTHENTICATION
-- ============================================================================

-- Register a new customer
CREATE PROCEDURE usp_RegisterCustomer
    @Email              NVARCHAR(255),
    @PasswordHash       NVARCHAR(255),
    @FirstName          NVARCHAR(100),
    @LastName           NVARCHAR(100),
    @PhoneNumber        NVARCHAR(50) = NULL,
    @CountryOfResidence CHAR(2),
    @PreferredLanguage  CHAR(2) = 'en',
    @PreferredCurrency  CHAR(3) = 'USD',
    @MarketingConsent   BIT = 0,
    @SourceChannel      NVARCHAR(50) = 'WEB',
    @SourceIP           NVARCHAR(50) = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    -- Check if email already exists
    IF EXISTS (SELECT 1 FROM Customers WHERE Email = @Email)
    BEGIN
        SELECT 
            0 AS Success,
            'EMAIL_EXISTS' AS ErrorCode,
            'A customer with this email already exists' AS ErrorMessage;
        RETURN;
    END
    
    -- Check if country is enabled for sending
    IF NOT EXISTS (SELECT 1 FROM CountryConfiguration WHERE CountryCode = @CountryOfResidence AND IsSendingEnabled = 1)
    BEGIN
        SELECT 
            0 AS Success,
            'COUNTRY_NOT_SUPPORTED' AS ErrorCode,
            'This country is not currently supported for sending' AS ErrorMessage;
        RETURN;
    END
    
    -- Get default limits based on country risk
    DECLARE @DailyLimit DECIMAL(18,2) = 1000.00;
    DECLARE @MonthlyLimit DECIMAL(18,2) = 5000.00;
    DECLARE @SingleTxLimit DECIMAL(18,2) = 500.00;
    DECLARE @RiskLevel NVARCHAR(20);
    
    SELECT @RiskLevel = RiskLevel FROM CountryConfiguration WHERE CountryCode = @CountryOfResidence;
    
    IF @RiskLevel = 'HIGH'
    BEGIN
        SET @DailyLimit = 500.00;
        SET @MonthlyLimit = 2000.00;
        SET @SingleTxLimit = 250.00;
    END
    
    -- Insert customer
    INSERT INTO Customers (
        Email, PasswordHash, FirstName, LastName, PhoneNumber,
        CountryOfResidence, PreferredLanguage, PreferredCurrency,
        MarketingConsent, DailyLimitUSD, MonthlyLimitUSD, SingleTxLimitUSD,
        RiskLevel, Status, VerificationTier, KYCStatus
    )
    VALUES (
        @Email, @PasswordHash, @FirstName, @LastName, @PhoneNumber,
        @CountryOfResidence, @PreferredLanguage, @PreferredCurrency,
        @MarketingConsent, @DailyLimit, @MonthlyLimit, @SingleTxLimit,
        @RiskLevel, 'ACTIVE', 1, 'PENDING'
    );
    
    DECLARE @CustomerId BIGINT = SCOPE_IDENTITY();
    DECLARE @ExternalId UNIQUEIDENTIFIER;
    SELECT @ExternalId = ExternalId FROM Customers WHERE CustomerId = @CustomerId;
    
    -- Log the registration
    INSERT INTO AuditLog (ActorType, ActorId, ActionType, EntityType, EntityId, NewValues, IPAddress)
    VALUES ('CUSTOMER', CAST(@CustomerId AS NVARCHAR), 'REGISTER', 'CUSTOMER', @CustomerId, 
            '{"channel":"' + @SourceChannel + '"}', @SourceIP);
    
    -- Return success with customer details
    SELECT 
        1 AS Success,
        @CustomerId AS CustomerId,
        @ExternalId AS ExternalId,
        @Email AS Email,
        @FirstName AS FirstName,
        @LastName AS LastName,
        'PENDING' AS KYCStatus,
        1 AS VerificationTier;
END;
GO

-- Authenticate customer login
CREATE PROCEDURE usp_AuthenticateCustomer
    @Email          NVARCHAR(255),
    @PasswordHash   NVARCHAR(255),
    @IPAddress      NVARCHAR(50) = NULL,
    @DeviceInfo     NVARCHAR(500) = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @CustomerId BIGINT;
    DECLARE @StoredHash NVARCHAR(255);
    DECLARE @Status NVARCHAR(20);
    DECLARE @FailedAttempts INT;
    DECLARE @LockedUntil DATETIME2;
    
    SELECT 
        @CustomerId = CustomerId,
        @StoredHash = PasswordHash,
        @Status = Status,
        @FailedAttempts = FailedLoginAttempts,
        @LockedUntil = LockedUntil
    FROM Customers 
    WHERE Email = @Email;
    
    -- Check if customer exists
    IF @CustomerId IS NULL
    BEGIN
        SELECT 0 AS Success, 'INVALID_CREDENTIALS' AS ErrorCode, 'Invalid email or password' AS ErrorMessage;
        RETURN;
    END
    
    -- Check if account is locked
    IF @LockedUntil IS NOT NULL AND @LockedUntil > SYSUTCDATETIME()
    BEGIN
        SELECT 0 AS Success, 'ACCOUNT_LOCKED' AS ErrorCode, 
               'Account is locked. Please try again later or reset your password' AS ErrorMessage,
               @LockedUntil AS LockedUntil;
        RETURN;
    END
    
    -- Check account status
    IF @Status NOT IN ('ACTIVE')
    BEGIN
        SELECT 0 AS Success, 'ACCOUNT_' + @Status AS ErrorCode, 
               'Your account is ' + LOWER(@Status) + '. Please contact support.' AS ErrorMessage;
        RETURN;
    END
    
    -- Verify password
    IF @StoredHash != @PasswordHash
    BEGIN
        -- Increment failed attempts
        UPDATE Customers 
        SET FailedLoginAttempts = FailedLoginAttempts + 1,
            LockedUntil = CASE WHEN FailedLoginAttempts >= 4 
                               THEN DATEADD(MINUTE, 30, SYSUTCDATETIME()) 
                               ELSE NULL END
        WHERE CustomerId = @CustomerId;
        
        SELECT 0 AS Success, 'INVALID_CREDENTIALS' AS ErrorCode, 'Invalid email or password' AS ErrorMessage;
        RETURN;
    END
    
    -- Successful login - reset failed attempts and update last login
    UPDATE Customers 
    SET FailedLoginAttempts = 0,
        LockedUntil = NULL,
        LastLoginAt = SYSUTCDATETIME()
    WHERE CustomerId = @CustomerId;
    
    -- Log the login
    INSERT INTO AuditLog (ActorType, ActorId, ActionType, EntityType, EntityId, IPAddress, UserAgent)
    VALUES ('CUSTOMER', CAST(@CustomerId AS NVARCHAR), 'LOGIN', 'CUSTOMER', @CustomerId, @IPAddress, @DeviceInfo);
    
    -- Return customer details
    SELECT 
        1 AS Success,
        c.CustomerId,
        c.ExternalId,
        c.Email,
        c.FirstName,
        c.LastName,
        c.PhoneNumber,
        c.PhoneVerified,
        c.VerificationTier,
        c.KYCStatus,
        c.DailyLimitUSD,
        c.MonthlyLimitUSD,
        c.SingleTxLimitUSD,
        c.PreferredLanguage,
        c.PreferredCurrency
    FROM Customers c
    WHERE c.CustomerId = @CustomerId;
END;
GO

-- Get customer by ID
CREATE PROCEDURE usp_GetCustomerById
    @CustomerId BIGINT
AS
BEGIN
    SET NOCOUNT ON;
    
    SELECT 
        c.CustomerId,
        c.ExternalId,
        c.Email,
        c.PhoneNumber,
        c.PhoneVerified,
        c.FirstName,
        c.MiddleName,
        c.LastName,
        c.DateOfBirth,
        c.Nationality,
        c.CountryOfResidence,
        c.AddressLine1,
        c.AddressLine2,
        c.City,
        c.StateProvince,
        c.PostalCode,
        c.VerificationTier,
        c.KYCStatus,
        c.KYCVerifiedAt,
        c.KYCExpiresAt,
        c.RiskScore,
        c.RiskLevel,
        c.DailyLimitUSD,
        c.MonthlyLimitUSD,
        c.YearlyLimitUSD,
        c.SingleTxLimitUSD,
        c.Status,
        c.PreferredLanguage,
        c.PreferredCurrency,
        c.CreatedAt,
        c.LastLoginAt
    FROM Customers c
    WHERE c.CustomerId = @CustomerId;
END;
GO

-- Get customer by external ID
CREATE PROCEDURE usp_GetCustomerByExternalId
    @ExternalId UNIQUEIDENTIFIER
AS
BEGIN
    SET NOCOUNT ON;
    
    SELECT 
        c.CustomerId,
        c.ExternalId,
        c.Email,
        c.PhoneNumber,
        c.PhoneVerified,
        c.FirstName,
        c.MiddleName,
        c.LastName,
        c.DateOfBirth,
        c.Nationality,
        c.CountryOfResidence,
        c.AddressLine1,
        c.AddressLine2,
        c.City,
        c.StateProvince,
        c.PostalCode,
        c.VerificationTier,
        c.KYCStatus,
        c.RiskScore,
        c.RiskLevel,
        c.DailyLimitUSD,
        c.MonthlyLimitUSD,
        c.SingleTxLimitUSD,
        c.Status,
        c.PreferredLanguage,
        c.PreferredCurrency,
        c.CreatedAt
    FROM Customers c
    WHERE c.ExternalId = @ExternalId;
END;
GO

-- Get customer by email
CREATE PROCEDURE usp_GetCustomerByEmail
    @Email NVARCHAR(255)
AS
BEGIN
    SET NOCOUNT ON;
    
    SELECT 
        c.CustomerId,
        c.ExternalId,
        c.Email,
        c.PhoneNumber,
        c.FirstName,
        c.LastName,
        c.CountryOfResidence,
        c.VerificationTier,
        c.KYCStatus,
        c.RiskLevel,
        c.Status,
        c.CreatedAt
    FROM Customers c
    WHERE c.Email = @Email;
END;
GO

-- Update customer profile
CREATE PROCEDURE usp_UpdateCustomerProfile
    @CustomerId     BIGINT,
    @FirstName      NVARCHAR(100) = NULL,
    @MiddleName     NVARCHAR(100) = NULL,
    @LastName       NVARCHAR(100) = NULL,
    @DateOfBirth    DATE = NULL,
    @Nationality    CHAR(2) = NULL,
    @PhoneNumber    NVARCHAR(50) = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    -- Check customer exists and is active
    IF NOT EXISTS (SELECT 1 FROM Customers WHERE CustomerId = @CustomerId AND Status = 'ACTIVE')
    BEGIN
        SELECT 0 AS Success, 'CUSTOMER_NOT_FOUND' AS ErrorCode;
        RETURN;
    END
    
    UPDATE Customers
    SET FirstName = COALESCE(@FirstName, FirstName),
        MiddleName = COALESCE(@MiddleName, MiddleName),
        LastName = COALESCE(@LastName, LastName),
        DateOfBirth = COALESCE(@DateOfBirth, DateOfBirth),
        Nationality = COALESCE(@Nationality, Nationality),
        PhoneNumber = COALESCE(@PhoneNumber, PhoneNumber),
        PhoneVerified = CASE WHEN @PhoneNumber IS NOT NULL AND @PhoneNumber != PhoneNumber THEN 0 ELSE PhoneVerified END,
        UpdatedAt = SYSUTCDATETIME()
    WHERE CustomerId = @CustomerId;
    
    SELECT 1 AS Success;
    
    -- Return updated customer
    EXEC usp_GetCustomerById @CustomerId;
END;
GO

-- Update customer address
CREATE PROCEDURE usp_UpdateCustomerAddress
    @CustomerId     BIGINT,
    @AddressLine1   NVARCHAR(255),
    @AddressLine2   NVARCHAR(255) = NULL,
    @City           NVARCHAR(100),
    @StateProvince  NVARCHAR(100) = NULL,
    @PostalCode     NVARCHAR(20),
    @Country        CHAR(2)
AS
BEGIN
    SET NOCOUNT ON;
    
    IF NOT EXISTS (SELECT 1 FROM Customers WHERE CustomerId = @CustomerId AND Status = 'ACTIVE')
    BEGIN
        SELECT 0 AS Success, 'CUSTOMER_NOT_FOUND' AS ErrorCode;
        RETURN;
    END
    
    UPDATE Customers
    SET AddressLine1 = @AddressLine1,
        AddressLine2 = @AddressLine2,
        City = @City,
        StateProvince = @StateProvince,
        PostalCode = @PostalCode,
        CountryOfResidence = @Country,
        UpdatedAt = SYSUTCDATETIME()
    WHERE CustomerId = @CustomerId;
    
    SELECT 1 AS Success;
END;
GO

-- List customers by KYC status
CREATE PROCEDURE usp_ListCustomersByKYCStatus
    @KYCStatus      NVARCHAR(20),
    @PageNumber     INT = 1,
    @PageSize       INT = 50
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @Offset INT = (@PageNumber - 1) * @PageSize;
    
    SELECT 
        c.CustomerId,
        c.ExternalId,
        c.Email,
        c.FirstName,
        c.LastName,
        c.CountryOfResidence,
        c.VerificationTier,
        c.KYCStatus,
        c.KYCVerifiedAt,
        c.KYCExpiresAt,
        c.RiskLevel,
        c.CreatedAt
    FROM Customers c
    WHERE c.KYCStatus = @KYCStatus
    ORDER BY c.CreatedAt DESC
    OFFSET @Offset ROWS
    FETCH NEXT @PageSize ROWS ONLY;
    
    -- Return total count
    SELECT COUNT(*) AS TotalCount 
    FROM Customers 
    WHERE KYCStatus = @KYCStatus;
END;
GO

-- Search customers
CREATE PROCEDURE usp_SearchCustomers
    @SearchTerm     NVARCHAR(100),
    @Status         NVARCHAR(20) = NULL,
    @Country        CHAR(2) = NULL,
    @RiskLevel      NVARCHAR(20) = NULL,
    @PageNumber     INT = 1,
    @PageSize       INT = 50
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @Offset INT = (@PageNumber - 1) * @PageSize;
    
    SELECT 
        c.CustomerId,
        c.ExternalId,
        c.Email,
        c.PhoneNumber,
        c.FirstName,
        c.LastName,
        c.CountryOfResidence,
        c.VerificationTier,
        c.KYCStatus,
        c.RiskLevel,
        c.Status,
        c.CreatedAt
    FROM Customers c
    WHERE (
        c.Email LIKE '%' + @SearchTerm + '%'
        OR c.FirstName LIKE '%' + @SearchTerm + '%'
        OR c.LastName LIKE '%' + @SearchTerm + '%'
        OR c.PhoneNumber LIKE '%' + @SearchTerm + '%'
    )
    AND (@Status IS NULL OR c.Status = @Status)
    AND (@Country IS NULL OR c.CountryOfResidence = @Country)
    AND (@RiskLevel IS NULL OR c.RiskLevel = @RiskLevel)
    ORDER BY c.CreatedAt DESC
    OFFSET @Offset ROWS
    FETCH NEXT @PageSize ROWS ONLY;
END;
GO

-- Suspend customer account
CREATE PROCEDURE usp_SuspendCustomer
    @CustomerId         BIGINT,
    @SuspensionReason   NVARCHAR(500),
    @SuspendedBy        NVARCHAR(100)
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @PreviousStatus NVARCHAR(20);
    SELECT @PreviousStatus = Status FROM Customers WHERE CustomerId = @CustomerId;
    
    IF @PreviousStatus IS NULL
    BEGIN
        SELECT 0 AS Success, 'CUSTOMER_NOT_FOUND' AS ErrorCode;
        RETURN;
    END
    
    IF @PreviousStatus = 'SUSPENDED'
    BEGIN
        SELECT 0 AS Success, 'ALREADY_SUSPENDED' AS ErrorCode;
        RETURN;
    END
    
    UPDATE Customers
    SET Status = 'SUSPENDED',
        SuspensionReason = @SuspensionReason,
        UpdatedAt = SYSUTCDATETIME()
    WHERE CustomerId = @CustomerId;
    
    -- Log the action
    INSERT INTO CustomerVerificationHistory (CustomerId, ActionType, PreviousValue, NewValue, Reason, PerformedBy)
    VALUES (@CustomerId, 'STATUS_CHANGE', @PreviousStatus, 'SUSPENDED', @SuspensionReason, @SuspendedBy);
    
    INSERT INTO AuditLog (ActorType, ActorId, ActionType, EntityType, EntityId, OldValues, NewValues)
    VALUES ('AGENT', @SuspendedBy, 'SUSPEND_CUSTOMER', 'CUSTOMER', @CustomerId, 
            '{"status":"' + @PreviousStatus + '"}', '{"status":"SUSPENDED","reason":"' + @SuspensionReason + '"}');
    
    SELECT 1 AS Success;
END;
GO

-- Reactivate suspended customer
CREATE PROCEDURE usp_ReactivateCustomer
    @CustomerId     BIGINT,
    @ReactivatedBy  NVARCHAR(100),
    @Notes          NVARCHAR(500) = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @PreviousStatus NVARCHAR(20);
    SELECT @PreviousStatus = Status FROM Customers WHERE CustomerId = @CustomerId;
    
    IF @PreviousStatus IS NULL
    BEGIN
        SELECT 0 AS Success, 'CUSTOMER_NOT_FOUND' AS ErrorCode;
        RETURN;
    END
    
    IF @PreviousStatus != 'SUSPENDED'
    BEGIN
        SELECT 0 AS Success, 'NOT_SUSPENDED' AS ErrorCode, 'Customer is not suspended' AS ErrorMessage;
        RETURN;
    END
    
    UPDATE Customers
    SET Status = 'ACTIVE',
        SuspensionReason = NULL,
        UpdatedAt = SYSUTCDATETIME()
    WHERE CustomerId = @CustomerId;
    
    INSERT INTO CustomerVerificationHistory (CustomerId, ActionType, PreviousValue, NewValue, Reason, PerformedBy)
    VALUES (@CustomerId, 'STATUS_CHANGE', 'SUSPENDED', 'ACTIVE', @Notes, @ReactivatedBy);
    
    SELECT 1 AS Success;
END;
GO

-- Close customer account
CREATE PROCEDURE usp_CloseCustomerAccount
    @CustomerId     BIGINT,
    @ClosureReason  NVARCHAR(500),
    @ClosedBy       NVARCHAR(100)
AS
BEGIN
    SET NOCOUNT ON;
    
    -- Check for pending transfers
    IF EXISTS (SELECT 1 FROM Transfers WHERE CustomerId = @CustomerId AND Status NOT IN ('COMPLETED', 'CANCELLED', 'REFUNDED', 'FAILED'))
    BEGIN
        SELECT 0 AS Success, 'PENDING_TRANSFERS' AS ErrorCode, 
               'Cannot close account with pending transfers' AS ErrorMessage;
        RETURN;
    END
    
    UPDATE Customers
    SET Status = 'CLOSED',
        SuspensionReason = @ClosureReason,
        ClosedAt = SYSUTCDATETIME(),
        UpdatedAt = SYSUTCDATETIME()
    WHERE CustomerId = @CustomerId;
    
    INSERT INTO CustomerVerificationHistory (CustomerId, ActionType, NewValue, Reason, PerformedBy)
    VALUES (@CustomerId, 'ACCOUNT_CLOSED', 'CLOSED', @ClosureReason, @ClosedBy);
    
    SELECT 1 AS Success;
END;
GO

-- Upgrade customer verification tier
CREATE PROCEDURE usp_UpgradeCustomerTier
    @CustomerId     BIGINT,
    @NewTier        TINYINT,
    @UpgradedBy     NVARCHAR(100),
    @Reason         NVARCHAR(500) = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @CurrentTier TINYINT;
    SELECT @CurrentTier = VerificationTier FROM Customers WHERE CustomerId = @CustomerId;
    
    IF @CurrentTier IS NULL
    BEGIN
        SELECT 0 AS Success, 'CUSTOMER_NOT_FOUND' AS ErrorCode;
        RETURN;
    END
    
    IF @NewTier <= @CurrentTier
    BEGIN
        SELECT 0 AS Success, 'INVALID_TIER' AS ErrorCode, 
               'New tier must be higher than current tier' AS ErrorMessage;
        RETURN;
    END
    
    -- Set new limits based on tier
    DECLARE @DailyLimit DECIMAL(18,2);
    DECLARE @MonthlyLimit DECIMAL(18,2);
    DECLARE @YearlyLimit DECIMAL(18,2);
    DECLARE @SingleTxLimit DECIMAL(18,2);
    
    IF @NewTier = 2
    BEGIN
        SET @DailyLimit = 5000.00;
        SET @MonthlyLimit = 20000.00;
        SET @YearlyLimit = 100000.00;
        SET @SingleTxLimit = 3000.00;
    END
    ELSE IF @NewTier = 3
    BEGIN
        SET @DailyLimit = 25000.00;
        SET @MonthlyLimit = 100000.00;
        SET @YearlyLimit = 500000.00;
        SET @SingleTxLimit = 15000.00;
    END
    
    UPDATE Customers
    SET VerificationTier = @NewTier,
        DailyLimitUSD = @DailyLimit,
        MonthlyLimitUSD = @MonthlyLimit,
        YearlyLimitUSD = @YearlyLimit,
        SingleTxLimitUSD = @SingleTxLimit,
        UpdatedAt = SYSUTCDATETIME()
    WHERE CustomerId = @CustomerId;
    
    INSERT INTO CustomerVerificationHistory (CustomerId, ActionType, PreviousValue, NewValue, Reason, PerformedBy)
    VALUES (@CustomerId, 'TIER_UPGRADE', CAST(@CurrentTier AS NVARCHAR), CAST(@NewTier AS NVARCHAR), @Reason, @UpgradedBy);
    
    SELECT 1 AS Success, @NewTier AS NewTier, @DailyLimit AS DailyLimit, @MonthlyLimit AS MonthlyLimit;
END;
GO

-- Lock customer account (security)
CREATE PROCEDURE usp_LockCustomerAccount
    @CustomerId     BIGINT,
    @LockDurationMinutes INT = 30,
    @Reason         NVARCHAR(500)
AS
BEGIN
    SET NOCOUNT ON;
    
    UPDATE Customers
    SET LockedUntil = DATEADD(MINUTE, @LockDurationMinutes, SYSUTCDATETIME()),
        UpdatedAt = SYSUTCDATETIME()
    WHERE CustomerId = @CustomerId;
    
    INSERT INTO AuditLog (ActorType, ActorId, ActionType, EntityType, EntityId, NewValues)
    VALUES ('SYSTEM', 'SECURITY', 'LOCK_ACCOUNT', 'CUSTOMER', @CustomerId, 
            '{"duration_minutes":' + CAST(@LockDurationMinutes AS NVARCHAR) + ',"reason":"' + @Reason + '"}');
    
    SELECT 1 AS Success;
END;
GO

-- Unlock customer account
CREATE PROCEDURE usp_UnlockCustomerAccount
    @CustomerId     BIGINT,
    @UnlockedBy     NVARCHAR(100)
AS
BEGIN
    SET NOCOUNT ON;
    
    UPDATE Customers
    SET LockedUntil = NULL,
        FailedLoginAttempts = 0,
        UpdatedAt = SYSUTCDATETIME()
    WHERE CustomerId = @CustomerId;
    
    INSERT INTO AuditLog (ActorType, ActorId, ActionType, EntityType, EntityId)
    VALUES ('AGENT', @UnlockedBy, 'UNLOCK_ACCOUNT', 'CUSTOMER', @CustomerId);
    
    SELECT 1 AS Success;
END;
GO

-- Get customer risk profile
CREATE PROCEDURE usp_GetCustomerRiskProfile
    @CustomerId BIGINT
AS
BEGIN
    SET NOCOUNT ON;
    
    -- Customer basic risk info
    SELECT 
        c.CustomerId,
        c.RiskScore,
        c.RiskLevel,
        c.VerificationTier,
        c.KYCStatus,
        c.CountryOfResidence,
        cc.RiskLevel AS CountryRiskLevel
    FROM Customers c
    LEFT JOIN CountryConfiguration cc ON c.CountryOfResidence = cc.CountryCode
    WHERE c.CustomerId = @CustomerId;
    
    -- Recent risk assessments
    SELECT TOP 5
        AssessmentId,
        AssessmentType,
        OverallRiskScore,
        RiskLevel,
        PreviousRiskLevel,
        RiskLevelChanged,
        AssessedAt
    FROM CustomerRiskAssessments
    WHERE CustomerId = @CustomerId
    ORDER BY AssessedAt DESC;
    
    -- Recent compliance screenings
    SELECT TOP 10
        ScreeningId,
        ScreeningType,
        Result,
        MatchScore,
        ResolutionStatus,
        ScreenedAt
    FROM ComplianceScreenings
    WHERE EntityType = 'CUSTOMER' AND EntityId = @CustomerId
    ORDER BY ScreenedAt DESC;
    
    -- Transfer statistics
    SELECT 
        COUNT(*) AS TotalTransfers,
        SUM(CASE WHEN Status = 'COMPLETED' THEN 1 ELSE 0 END) AS CompletedTransfers,
        SUM(CASE WHEN ComplianceStatus = 'FLAGGED' THEN 1 ELSE 0 END) AS FlaggedTransfers,
        SUM(SendAmount) AS TotalSendVolume,
        AVG(SendAmount) AS AvgTransferAmount
    FROM Transfers
    WHERE CustomerId = @CustomerId
    AND CreatedAt >= DATEADD(MONTH, -12, SYSUTCDATETIME());
END;
GO

-- List high-risk customers
CREATE PROCEDURE usp_ListHighRiskCustomers
    @RiskLevel      NVARCHAR(20) = 'HIGH',
    @PageNumber     INT = 1,
    @PageSize       INT = 50
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @Offset INT = (@PageNumber - 1) * @PageSize;
    
    SELECT 
        c.CustomerId,
        c.ExternalId,
        c.Email,
        c.FirstName,
        c.LastName,
        c.CountryOfResidence,
        c.RiskScore,
        c.RiskLevel,
        c.VerificationTier,
        c.KYCStatus,
        c.Status,
        c.CreatedAt,
        (SELECT COUNT(*) FROM Transfers t WHERE t.CustomerId = c.CustomerId AND t.ComplianceStatus = 'FLAGGED') AS FlaggedTransferCount
    FROM Customers c
    WHERE c.RiskLevel = @RiskLevel
    AND c.Status = 'ACTIVE'
    ORDER BY c.RiskScore DESC
    OFFSET @Offset ROWS
    FETCH NEXT @PageSize ROWS ONLY;
END;
GO

-- Update customer limits
CREATE PROCEDURE usp_UpdateCustomerLimits
    @CustomerId         BIGINT,
    @DailyLimitUSD      DECIMAL(18,2) = NULL,
    @MonthlyLimitUSD    DECIMAL(18,2) = NULL,
    @YearlyLimitUSD     DECIMAL(18,2) = NULL,
    @SingleTxLimitUSD   DECIMAL(18,2) = NULL,
    @UpdatedBy          NVARCHAR(100),
    @Reason             NVARCHAR(500)
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @OldDaily DECIMAL(18,2), @OldMonthly DECIMAL(18,2), @OldYearly DECIMAL(18,2), @OldSingle DECIMAL(18,2);
    
    SELECT 
        @OldDaily = DailyLimitUSD,
        @OldMonthly = MonthlyLimitUSD,
        @OldYearly = YearlyLimitUSD,
        @OldSingle = SingleTxLimitUSD
    FROM Customers WHERE CustomerId = @CustomerId;
    
    IF @OldDaily IS NULL
    BEGIN
        SELECT 0 AS Success, 'CUSTOMER_NOT_FOUND' AS ErrorCode;
        RETURN;
    END
    
    UPDATE Customers
    SET DailyLimitUSD = COALESCE(@DailyLimitUSD, DailyLimitUSD),
        MonthlyLimitUSD = COALESCE(@MonthlyLimitUSD, MonthlyLimitUSD),
        YearlyLimitUSD = COALESCE(@YearlyLimitUSD, YearlyLimitUSD),
        SingleTxLimitUSD = COALESCE(@SingleTxLimitUSD, SingleTxLimitUSD),
        UpdatedAt = SYSUTCDATETIME()
    WHERE CustomerId = @CustomerId;
    
    INSERT INTO CustomerVerificationHistory (CustomerId, ActionType, PreviousValue, NewValue, Reason, PerformedBy)
    VALUES (@CustomerId, 'LIMIT_CHANGE', 
            'D:' + CAST(@OldDaily AS NVARCHAR) + ',M:' + CAST(@OldMonthly AS NVARCHAR),
            'D:' + CAST(COALESCE(@DailyLimitUSD, @OldDaily) AS NVARCHAR) + ',M:' + CAST(COALESCE(@MonthlyLimitUSD, @OldMonthly) AS NVARCHAR),
            @Reason, @UpdatedBy);
    
    SELECT 1 AS Success;
END;
GO

-- Verify phone number
CREATE PROCEDURE usp_VerifyCustomerPhone
    @CustomerId     BIGINT,
    @PhoneNumber    NVARCHAR(50)
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @StoredPhone NVARCHAR(50);
    SELECT @StoredPhone = PhoneNumber FROM Customers WHERE CustomerId = @CustomerId;
    
    IF @StoredPhone != @PhoneNumber
    BEGIN
        SELECT 0 AS Success, 'PHONE_MISMATCH' AS ErrorCode;
        RETURN;
    END
    
    UPDATE Customers
    SET PhoneVerified = 1,
        UpdatedAt = SYSUTCDATETIME()
    WHERE CustomerId = @CustomerId;
    
    SELECT 1 AS Success;
END;
GO

-- Change password
CREATE PROCEDURE usp_ChangeCustomerPassword
    @CustomerId         BIGINT,
    @OldPasswordHash    NVARCHAR(255),
    @NewPasswordHash    NVARCHAR(255)
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @StoredHash NVARCHAR(255);
    SELECT @StoredHash = PasswordHash FROM Customers WHERE CustomerId = @CustomerId;
    
    IF @StoredHash IS NULL
    BEGIN
        SELECT 0 AS Success, 'CUSTOMER_NOT_FOUND' AS ErrorCode;
        RETURN;
    END
    
    IF @StoredHash != @OldPasswordHash
    BEGIN
        SELECT 0 AS Success, 'INVALID_PASSWORD' AS ErrorCode, 'Current password is incorrect' AS ErrorMessage;
        RETURN;
    END
    
    UPDATE Customers
    SET PasswordHash = @NewPasswordHash,
        UpdatedAt = SYSUTCDATETIME()
    WHERE CustomerId = @CustomerId;
    
    INSERT INTO AuditLog (ActorType, ActorId, ActionType, EntityType, EntityId)
    VALUES ('CUSTOMER', CAST(@CustomerId AS NVARCHAR), 'PASSWORD_CHANGE', 'CUSTOMER', @CustomerId);
    
    SELECT 1 AS Success;
END;
GO

-- Request password reset
CREATE PROCEDURE usp_RequestPasswordReset
    @Email NVARCHAR(255)
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @CustomerId BIGINT;
    SELECT @CustomerId = CustomerId FROM Customers WHERE Email = @Email AND Status = 'ACTIVE';
    
    -- Always return success for security (don't reveal if email exists)
    SELECT 1 AS Success, 'If an account exists with this email, a reset link will be sent' AS Message;
    
    IF @CustomerId IS NOT NULL
    BEGIN
        INSERT INTO AuditLog (ActorType, ActorId, ActionType, EntityType, EntityId)
        VALUES ('CUSTOMER', CAST(@CustomerId AS NVARCHAR), 'PASSWORD_RESET_REQUEST', 'CUSTOMER', @CustomerId);
    END
END;
GO

-- Get customer verification history
CREATE PROCEDURE usp_GetCustomerVerificationHistory
    @CustomerId BIGINT
AS
BEGIN
    SET NOCOUNT ON;
    
    SELECT 
        HistoryId,
        ActionType,
        PreviousValue,
        NewValue,
        Reason,
        PerformedBy,
        PerformedAt
    FROM CustomerVerificationHistory
    WHERE CustomerId = @CustomerId
    ORDER BY PerformedAt DESC;
END;
GO
