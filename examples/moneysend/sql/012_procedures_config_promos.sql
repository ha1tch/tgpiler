-- ============================================================================
-- MoneySend Stored Procedures
-- Part 12: System Configuration & Promotions
-- ============================================================================

-- ============================================================================
-- SYSTEM CONFIGURATION
-- ============================================================================

-- Get configuration value
CREATE PROCEDURE usp_GetConfigValue
    @ConfigKey NVARCHAR(100)
AS
BEGIN
    SET NOCOUNT ON;
    
    SELECT 
        ConfigKey,
        ConfigValue,
        ConfigType,
        Description
    FROM SystemConfiguration
    WHERE ConfigKey = @ConfigKey;
END;
GO

-- Get all configuration
CREATE PROCEDURE usp_GetAllConfiguration
    @Prefix NVARCHAR(50) = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    SELECT 
        ConfigKey,
        ConfigValue,
        ConfigType,
        Description,
        UpdatedAt,
        UpdatedBy
    FROM SystemConfiguration
    WHERE @Prefix IS NULL OR ConfigKey LIKE @Prefix + '%'
    ORDER BY ConfigKey;
END;
GO

-- Set configuration value
CREATE PROCEDURE usp_SetConfigValue
    @ConfigKey      NVARCHAR(100),
    @ConfigValue    NVARCHAR(MAX),
    @ConfigType     NVARCHAR(20) = 'STRING',
    @Description    NVARCHAR(500) = NULL,
    @UpdatedBy      NVARCHAR(100)
AS
BEGIN
    SET NOCOUNT ON;
    
    IF EXISTS (SELECT 1 FROM SystemConfiguration WHERE ConfigKey = @ConfigKey)
    BEGIN
        UPDATE SystemConfiguration
        SET ConfigValue = @ConfigValue,
            ConfigType = @ConfigType,
            Description = COALESCE(@Description, Description),
            UpdatedAt = SYSUTCDATETIME(),
            UpdatedBy = @UpdatedBy
        WHERE ConfigKey = @ConfigKey;
    END
    ELSE
    BEGIN
        INSERT INTO SystemConfiguration (ConfigKey, ConfigValue, ConfigType, Description, UpdatedBy)
        VALUES (@ConfigKey, @ConfigValue, @ConfigType, @Description, @UpdatedBy);
    END
    
    INSERT INTO AuditLog (ActorType, ActorId, ActionType, EntityType, EntityId, NewValues)
    VALUES ('AGENT', @UpdatedBy, 'CONFIG_CHANGE', 'CONFIG', 0, 
            '{"key":"' + @ConfigKey + '","value":"' + LEFT(@ConfigValue, 100) + '"}');
    
    SELECT 1 AS Success;
END;
GO

-- Delete configuration
CREATE PROCEDURE usp_DeleteConfigValue
    @ConfigKey  NVARCHAR(100),
    @DeletedBy  NVARCHAR(100)
AS
BEGIN
    SET NOCOUNT ON;
    
    DELETE FROM SystemConfiguration WHERE ConfigKey = @ConfigKey;
    
    INSERT INTO AuditLog (ActorType, ActorId, ActionType, EntityType, EntityId, OldValues)
    VALUES ('AGENT', @DeletedBy, 'CONFIG_DELETE', 'CONFIG', 0, '{"key":"' + @ConfigKey + '"}');
    
    SELECT @@ROWCOUNT AS Deleted;
END;
GO

-- ============================================================================
-- COUNTRY CONFIGURATION
-- ============================================================================

-- Get country configuration
CREATE PROCEDURE usp_GetCountryConfiguration
    @CountryCode CHAR(2)
AS
BEGIN
    SET NOCOUNT ON;
    
    SELECT 
        CountryCode,
        CountryName,
        RequiresSourceOfFunds,
        RequiresPurpose,
        MaxDailyLimit,
        RiskLevel,
        IsSendingEnabled,
        IsReceivingEnabled
    FROM CountryConfiguration
    WHERE CountryCode = @CountryCode;
END;
GO

-- List country configurations
CREATE PROCEDURE usp_ListCountryConfigurations
    @SendingEnabled     BIT = NULL,
    @ReceivingEnabled   BIT = NULL,
    @RiskLevel          NVARCHAR(20) = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    SELECT 
        CountryCode,
        CountryName,
        RequiresSourceOfFunds,
        RequiresPurpose,
        MaxDailyLimit,
        RiskLevel,
        IsSendingEnabled,
        IsReceivingEnabled
    FROM CountryConfiguration
    WHERE (@SendingEnabled IS NULL OR IsSendingEnabled = @SendingEnabled)
    AND (@ReceivingEnabled IS NULL OR IsReceivingEnabled = @ReceivingEnabled)
    AND (@RiskLevel IS NULL OR RiskLevel = @RiskLevel)
    ORDER BY CountryName;
END;
GO

-- Update country configuration
CREATE PROCEDURE usp_UpdateCountryConfiguration
    @CountryCode            CHAR(2),
    @CountryName            NVARCHAR(100) = NULL,
    @RequiresSourceOfFunds  BIT = NULL,
    @RequiresPurpose        BIT = NULL,
    @MaxDailyLimit          DECIMAL(18,2) = NULL,
    @RiskLevel              NVARCHAR(20) = NULL,
    @IsSendingEnabled       BIT = NULL,
    @IsReceivingEnabled     BIT = NULL,
    @UpdatedBy              NVARCHAR(100)
AS
BEGIN
    SET NOCOUNT ON;
    
    IF EXISTS (SELECT 1 FROM CountryConfiguration WHERE CountryCode = @CountryCode)
    BEGIN
        UPDATE CountryConfiguration
        SET CountryName = COALESCE(@CountryName, CountryName),
            RequiresSourceOfFunds = COALESCE(@RequiresSourceOfFunds, RequiresSourceOfFunds),
            RequiresPurpose = COALESCE(@RequiresPurpose, RequiresPurpose),
            MaxDailyLimit = COALESCE(@MaxDailyLimit, MaxDailyLimit),
            RiskLevel = COALESCE(@RiskLevel, RiskLevel),
            IsSendingEnabled = COALESCE(@IsSendingEnabled, IsSendingEnabled),
            IsReceivingEnabled = COALESCE(@IsReceivingEnabled, IsReceivingEnabled)
        WHERE CountryCode = @CountryCode;
    END
    ELSE
    BEGIN
        INSERT INTO CountryConfiguration (CountryCode, CountryName, RiskLevel, IsSendingEnabled, IsReceivingEnabled)
        VALUES (@CountryCode, COALESCE(@CountryName, @CountryCode), COALESCE(@RiskLevel, 'STANDARD'), 
                COALESCE(@IsSendingEnabled, 0), COALESCE(@IsReceivingEnabled, 0));
    END
    
    INSERT INTO AuditLog (ActorType, ActorId, ActionType, EntityType, EntityId, NewValues)
    VALUES ('AGENT', @UpdatedBy, 'COUNTRY_CONFIG_UPDATE', 'COUNTRY', 0, '{"country":"' + @CountryCode + '"}');
    
    SELECT 1 AS Success;
END;
GO

-- Enable country for sending
CREATE PROCEDURE usp_EnableCountryForSending
    @CountryCode    CHAR(2),
    @EnabledBy      NVARCHAR(100)
AS
BEGIN
    SET NOCOUNT ON;
    
    UPDATE CountryConfiguration SET IsSendingEnabled = 1 WHERE CountryCode = @CountryCode;
    
    INSERT INTO AuditLog (ActorType, ActorId, ActionType, EntityType, EntityId, NewValues)
    VALUES ('AGENT', @EnabledBy, 'ENABLE_COUNTRY_SENDING', 'COUNTRY', 0, '{"country":"' + @CountryCode + '"}');
    
    SELECT 1 AS Success;
END;
GO

-- Disable country for sending
CREATE PROCEDURE usp_DisableCountryForSending
    @CountryCode    CHAR(2),
    @DisabledBy     NVARCHAR(100),
    @Reason         NVARCHAR(500)
AS
BEGIN
    SET NOCOUNT ON;
    
    UPDATE CountryConfiguration SET IsSendingEnabled = 0 WHERE CountryCode = @CountryCode;
    
    -- Also deactivate corridors from this country
    UPDATE Corridors SET IsActive = 0 WHERE OriginCountry = @CountryCode;
    
    INSERT INTO AuditLog (ActorType, ActorId, ActionType, EntityType, EntityId, NewValues)
    VALUES ('AGENT', @DisabledBy, 'DISABLE_COUNTRY_SENDING', 'COUNTRY', 0, 
            '{"country":"' + @CountryCode + '","reason":"' + @Reason + '"}');
    
    SELECT 1 AS Success;
END;
GO

-- ============================================================================
-- PROMOTIONS MANAGEMENT
-- ============================================================================

-- Create promo code
CREATE PROCEDURE usp_CreatePromoCode
    @Code               NVARCHAR(50),
    @PromoName          NVARCHAR(200),
    @Description        NVARCHAR(500) = NULL,
    @DiscountType       NVARCHAR(20),
    @DiscountValue      DECIMAL(18,4),
    @MaxDiscountAmount  DECIMAL(18,2) = NULL,
    @MinSendAmount      DECIMAL(18,2) = NULL,
    @MaxSendAmount      DECIMAL(18,2) = NULL,
    @EligibleCorridors  NVARCHAR(500) = NULL,
    @NewCustomersOnly   BIT = 0,
    @MaxUsesTotal       INT = NULL,
    @MaxUsesPerCustomer INT = 1,
    @ValidFrom          DATETIME2,
    @ValidTo            DATETIME2,
    @CreatedBy          NVARCHAR(100)
AS
BEGIN
    SET NOCOUNT ON;
    
    IF EXISTS (SELECT 1 FROM PromoCodes WHERE Code = @Code)
    BEGIN
        SELECT 0 AS Success, 'CODE_EXISTS' AS ErrorCode;
        RETURN;
    END
    
    IF @ValidTo <= @ValidFrom
    BEGIN
        SELECT 0 AS Success, 'INVALID_DATES' AS ErrorCode;
        RETURN;
    END
    
    INSERT INTO PromoCodes (
        Code, PromoName, Description, DiscountType, DiscountValue,
        MaxDiscountAmount, MinSendAmount, MaxSendAmount, EligibleCorridors,
        NewCustomersOnly, MaxUsesTotal, MaxUsesPerCustomer, ValidFrom, ValidTo, CreatedBy
    )
    VALUES (
        @Code, @PromoName, @Description, @DiscountType, @DiscountValue,
        @MaxDiscountAmount, @MinSendAmount, @MaxSendAmount, @EligibleCorridors,
        @NewCustomersOnly, @MaxUsesTotal, @MaxUsesPerCustomer, @ValidFrom, @ValidTo, @CreatedBy
    );
    
    SELECT 1 AS Success, SCOPE_IDENTITY() AS PromoCodeId;
END;
GO

-- Get promo code by code
CREATE PROCEDURE usp_GetPromoCodeByCode
    @Code NVARCHAR(50)
AS
BEGIN
    SET NOCOUNT ON;
    
    SELECT 
        PromoCodeId,
        Code,
        PromoName,
        Description,
        DiscountType,
        DiscountValue,
        MaxDiscountAmount,
        MinSendAmount,
        MaxSendAmount,
        EligibleCorridors,
        NewCustomersOnly,
        MaxUsesTotal,
        MaxUsesPerCustomer,
        CurrentUsageCount,
        ValidFrom,
        ValidTo,
        IsActive,
        CreatedAt,
        CreatedBy
    FROM PromoCodes
    WHERE Code = @Code;
END;
GO

-- List promo codes
CREATE PROCEDURE usp_ListPromoCodes
    @Status     NVARCHAR(20) = NULL,  -- ACTIVE, EXPIRED, UPCOMING, ALL
    @PageNumber INT = 1,
    @PageSize   INT = 50
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @Offset INT = (@PageNumber - 1) * @PageSize;
    DECLARE @Now DATETIME2 = SYSUTCDATETIME();
    
    SELECT 
        PromoCodeId,
        Code,
        PromoName,
        DiscountType,
        DiscountValue,
        MaxUsesTotal,
        CurrentUsageCount,
        ValidFrom,
        ValidTo,
        IsActive,
        CASE 
            WHEN IsActive = 0 THEN 'INACTIVE'
            WHEN ValidTo < @Now THEN 'EXPIRED'
            WHEN ValidFrom > @Now THEN 'UPCOMING'
            ELSE 'ACTIVE'
        END AS Status
    FROM PromoCodes
    WHERE @Status IS NULL OR @Status = 'ALL' OR
        (@Status = 'ACTIVE' AND IsActive = 1 AND ValidFrom <= @Now AND ValidTo >= @Now) OR
        (@Status = 'EXPIRED' AND ValidTo < @Now) OR
        (@Status = 'UPCOMING' AND ValidFrom > @Now)
    ORDER BY ValidFrom DESC
    OFFSET @Offset ROWS
    FETCH NEXT @PageSize ROWS ONLY;
END;
GO

-- Update promo code
CREATE PROCEDURE usp_UpdatePromoCode
    @PromoCodeId        INT,
    @PromoName          NVARCHAR(200) = NULL,
    @Description        NVARCHAR(500) = NULL,
    @MaxUsesTotal       INT = NULL,
    @MaxUsesPerCustomer INT = NULL,
    @ValidTo            DATETIME2 = NULL,
    @IsActive           BIT = NULL,
    @UpdatedBy          NVARCHAR(100)
AS
BEGIN
    SET NOCOUNT ON;
    
    IF NOT EXISTS (SELECT 1 FROM PromoCodes WHERE PromoCodeId = @PromoCodeId)
    BEGIN
        SELECT 0 AS Success, 'PROMO_NOT_FOUND' AS ErrorCode;
        RETURN;
    END
    
    UPDATE PromoCodes
    SET PromoName = COALESCE(@PromoName, PromoName),
        Description = COALESCE(@Description, Description),
        MaxUsesTotal = COALESCE(@MaxUsesTotal, MaxUsesTotal),
        MaxUsesPerCustomer = COALESCE(@MaxUsesPerCustomer, MaxUsesPerCustomer),
        ValidTo = COALESCE(@ValidTo, ValidTo),
        IsActive = COALESCE(@IsActive, IsActive)
    WHERE PromoCodeId = @PromoCodeId;
    
    SELECT 1 AS Success;
END;
GO

-- Deactivate promo code
CREATE PROCEDURE usp_DeactivatePromoCode
    @PromoCodeId    INT,
    @DeactivatedBy  NVARCHAR(100)
AS
BEGIN
    SET NOCOUNT ON;
    
    UPDATE PromoCodes SET IsActive = 0 WHERE PromoCodeId = @PromoCodeId;
    
    INSERT INTO AuditLog (ActorType, ActorId, ActionType, EntityType, EntityId)
    VALUES ('AGENT', @DeactivatedBy, 'DEACTIVATE_PROMO', 'PROMO', @PromoCodeId);
    
    SELECT 1 AS Success;
END;
GO

-- Get promo code usage
CREATE PROCEDURE usp_GetPromoCodeUsage
    @PromoCodeId    INT,
    @PageNumber     INT = 1,
    @PageSize       INT = 50
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @Offset INT = (@PageNumber - 1) * @PageSize;
    
    SELECT 
        u.UsageId,
        u.CustomerId,
        c.Email AS CustomerEmail,
        c.FirstName,
        c.LastName,
        u.TransferId,
        t.TransferNumber,
        t.SendAmount,
        t.SendCurrency,
        u.DiscountApplied,
        u.UsedAt
    FROM PromoCodeUsage u
    JOIN Customers c ON u.CustomerId = c.CustomerId
    JOIN Transfers t ON u.TransferId = t.TransferId
    WHERE u.PromoCodeId = @PromoCodeId
    ORDER BY u.UsedAt DESC
    OFFSET @Offset ROWS
    FETCH NEXT @PageSize ROWS ONLY;
    
    -- Summary
    SELECT 
        COUNT(*) AS TotalUsages,
        SUM(DiscountApplied) AS TotalDiscountGiven,
        COUNT(DISTINCT CustomerId) AS UniqueCustomers
    FROM PromoCodeUsage
    WHERE PromoCodeId = @PromoCodeId;
END;
GO

-- Check promo eligibility for customer
CREATE PROCEDURE usp_CheckPromoEligibility
    @Code       NVARCHAR(50),
    @CustomerId BIGINT,
    @CorridorId INT = NULL,
    @SendAmount DECIMAL(18,2) = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @PromoCodeId INT;
    DECLARE @NewCustomersOnly BIT;
    DECLARE @MaxUsesPerCustomer INT;
    DECLARE @MaxUsesTotal INT;
    DECLARE @CurrentUsageCount INT;
    DECLARE @MinSendAmount DECIMAL(18,2);
    DECLARE @MaxSendAmount DECIMAL(18,2);
    DECLARE @EligibleCorridors NVARCHAR(500);
    DECLARE @ValidFrom DATETIME2;
    DECLARE @ValidTo DATETIME2;
    DECLARE @IsActive BIT;
    
    SELECT 
        @PromoCodeId = PromoCodeId,
        @NewCustomersOnly = NewCustomersOnly,
        @MaxUsesPerCustomer = MaxUsesPerCustomer,
        @MaxUsesTotal = MaxUsesTotal,
        @CurrentUsageCount = CurrentUsageCount,
        @MinSendAmount = MinSendAmount,
        @MaxSendAmount = MaxSendAmount,
        @EligibleCorridors = EligibleCorridors,
        @ValidFrom = ValidFrom,
        @ValidTo = ValidTo,
        @IsActive = IsActive
    FROM PromoCodes WHERE Code = @Code;
    
    IF @PromoCodeId IS NULL
    BEGIN
        SELECT 0 AS IsEligible, 'INVALID_CODE' AS Reason;
        RETURN;
    END
    
    IF @IsActive = 0
    BEGIN
        SELECT 0 AS IsEligible, 'CODE_INACTIVE' AS Reason;
        RETURN;
    END
    
    IF SYSUTCDATETIME() < @ValidFrom OR SYSUTCDATETIME() > @ValidTo
    BEGIN
        SELECT 0 AS IsEligible, 'CODE_EXPIRED' AS Reason;
        RETURN;
    END
    
    IF @MaxUsesTotal IS NOT NULL AND @CurrentUsageCount >= @MaxUsesTotal
    BEGIN
        SELECT 0 AS IsEligible, 'CODE_EXHAUSTED' AS Reason;
        RETURN;
    END
    
    DECLARE @CustomerUsageCount INT;
    SELECT @CustomerUsageCount = COUNT(*) FROM PromoCodeUsage 
    WHERE PromoCodeId = @PromoCodeId AND CustomerId = @CustomerId;
    
    IF @CustomerUsageCount >= @MaxUsesPerCustomer
    BEGIN
        SELECT 0 AS IsEligible, 'ALREADY_USED' AS Reason;
        RETURN;
    END
    
    IF @NewCustomersOnly = 1
    BEGIN
        IF EXISTS (SELECT 1 FROM Transfers WHERE CustomerId = @CustomerId AND Status = 'COMPLETED')
        BEGIN
            SELECT 0 AS IsEligible, 'NOT_NEW_CUSTOMER' AS Reason;
            RETURN;
        END
    END
    
    IF @SendAmount IS NOT NULL
    BEGIN
        IF @MinSendAmount IS NOT NULL AND @SendAmount < @MinSendAmount
        BEGIN
            SELECT 0 AS IsEligible, 'BELOW_MINIMUM' AS Reason;
            RETURN;
        END
        IF @MaxSendAmount IS NOT NULL AND @SendAmount > @MaxSendAmount
        BEGIN
            SELECT 0 AS IsEligible, 'ABOVE_MAXIMUM' AS Reason;
            RETURN;
        END
    END
    
    -- Corridor check would require JSON parsing in production
    
    SELECT 1 AS IsEligible, NULL AS Reason;
END;
GO

-- Generate bulk promo codes
CREATE PROCEDURE usp_GenerateBulkPromoCodes
    @Prefix             NVARCHAR(20),
    @Count              INT,
    @PromoName          NVARCHAR(200),
    @DiscountType       NVARCHAR(20),
    @DiscountValue      DECIMAL(18,4),
    @MaxUsesPerCustomer INT = 1,
    @ValidFrom          DATETIME2,
    @ValidTo            DATETIME2,
    @CreatedBy          NVARCHAR(100)
AS
BEGIN
    SET NOCOUNT ON;
    
    IF @Count > 1000
    BEGIN
        SELECT 0 AS Success, 'MAX_1000_CODES' AS ErrorCode;
        RETURN;
    END
    
    DECLARE @i INT = 1;
    DECLARE @Generated INT = 0;
    DECLARE @Code NVARCHAR(50);
    
    WHILE @i <= @Count
    BEGIN
        SET @Code = @Prefix + '-' + RIGHT('0000' + CAST(@i AS NVARCHAR), 4);
        
        IF NOT EXISTS (SELECT 1 FROM PromoCodes WHERE Code = @Code)
        BEGIN
            INSERT INTO PromoCodes (
                Code, PromoName, DiscountType, DiscountValue,
                MaxUsesPerCustomer, ValidFrom, ValidTo, CreatedBy
            )
            VALUES (
                @Code, @PromoName, @DiscountType, @DiscountValue,
                @MaxUsesPerCustomer, @ValidFrom, @ValidTo, @CreatedBy
            );
            SET @Generated = @Generated + 1;
        END
        
        SET @i = @i + 1;
    END
    
    SELECT 1 AS Success, @Generated AS GeneratedCount;
END;
GO

-- Get active promotions for customer
CREATE PROCEDURE usp_GetActivePromotionsForCustomer
    @CustomerId BIGINT
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @Now DATETIME2 = SYSUTCDATETIME();
    DECLARE @IsNewCustomer BIT = 0;
    
    IF NOT EXISTS (SELECT 1 FROM Transfers WHERE CustomerId = @CustomerId AND Status = 'COMPLETED')
        SET @IsNewCustomer = 1;
    
    SELECT 
        p.PromoCodeId,
        p.Code,
        p.PromoName,
        p.Description,
        p.DiscountType,
        p.DiscountValue,
        p.MaxDiscountAmount,
        p.MinSendAmount,
        p.MaxSendAmount,
        p.ValidTo,
        p.NewCustomersOnly
    FROM PromoCodes p
    LEFT JOIN PromoCodeUsage u ON p.PromoCodeId = u.PromoCodeId AND u.CustomerId = @CustomerId
    WHERE p.IsActive = 1
    AND p.ValidFrom <= @Now AND p.ValidTo >= @Now
    AND (p.MaxUsesTotal IS NULL OR p.CurrentUsageCount < p.MaxUsesTotal)
    AND (p.NewCustomersOnly = 0 OR @IsNewCustomer = 1)
    GROUP BY p.PromoCodeId, p.Code, p.PromoName, p.Description, p.DiscountType, 
             p.DiscountValue, p.MaxDiscountAmount, p.MinSendAmount, p.MaxSendAmount, 
             p.ValidTo, p.NewCustomersOnly, p.MaxUsesPerCustomer
    HAVING COUNT(u.UsageId) < p.MaxUsesPerCustomer
    ORDER BY p.ValidTo;
END;
GO
