-- ============================================================================
-- MoneySend Stored Procedures
-- Part 3: Quotes, Rates, Corridors & Core Transfers
-- ============================================================================

-- ============================================================================
-- CORRIDOR & RATE MANAGEMENT
-- ============================================================================

-- Get corridor by code
CREATE PROCEDURE usp_GetCorridorByCode
    @CorridorCode NVARCHAR(20)
AS
BEGIN
    SET NOCOUNT ON;
    
    SELECT 
        CorridorId,
        CorridorCode,
        DisplayName,
        OriginCountry,
        DestinationCountry,
        OriginCurrency,
        DestinationCurrency,
        MinSendAmount,
        MaxSendAmount,
        SupportedPayoutMethods,
        EstimatedDeliveryMinutes,
        CutoffTimeUTC,
        IsActive
    FROM Corridors
    WHERE CorridorCode = @CorridorCode;
END;
GO

-- Get corridor by countries
CREATE PROCEDURE usp_GetCorridorByCountries
    @OriginCountry      CHAR(2),
    @DestinationCountry CHAR(2)
AS
BEGIN
    SET NOCOUNT ON;
    
    SELECT 
        CorridorId,
        CorridorCode,
        DisplayName,
        OriginCountry,
        DestinationCountry,
        OriginCurrency,
        DestinationCurrency,
        MinSendAmount,
        MaxSendAmount,
        SupportedPayoutMethods,
        EstimatedDeliveryMinutes,
        CutoffTimeUTC,
        IsActive
    FROM Corridors
    WHERE OriginCountry = @OriginCountry
    AND DestinationCountry = @DestinationCountry
    AND IsActive = 1;
END;
GO

-- List all active corridors
CREATE PROCEDURE usp_ListActiveCorridors
    @OriginCountry CHAR(2) = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    SELECT 
        c.CorridorId,
        c.CorridorCode,
        c.DisplayName,
        c.OriginCountry,
        c.DestinationCountry,
        c.OriginCurrency,
        c.DestinationCurrency,
        c.MinSendAmount,
        c.MaxSendAmount,
        c.SupportedPayoutMethods,
        c.EstimatedDeliveryMinutes,
        r.SellRate AS CurrentRate
    FROM Corridors c
    LEFT JOIN FXRates r ON c.CorridorId = r.CorridorId AND r.EffectiveTo IS NULL
    WHERE c.IsActive = 1
    AND (@OriginCountry IS NULL OR c.OriginCountry = @OriginCountry)
    ORDER BY c.DisplayName;
END;
GO

-- List corridors by destination
CREATE PROCEDURE usp_ListCorridorsByDestination
    @DestinationCountry CHAR(2)
AS
BEGIN
    SET NOCOUNT ON;
    
    SELECT 
        c.CorridorId,
        c.CorridorCode,
        c.DisplayName,
        c.OriginCountry,
        c.DestinationCountry,
        c.OriginCurrency,
        c.DestinationCurrency,
        c.MinSendAmount,
        c.MaxSendAmount,
        c.SupportedPayoutMethods,
        r.SellRate AS CurrentRate
    FROM Corridors c
    LEFT JOIN FXRates r ON c.CorridorId = r.CorridorId AND r.EffectiveTo IS NULL
    WHERE c.IsActive = 1
    AND c.DestinationCountry = @DestinationCountry
    ORDER BY c.OriginCountry;
END;
GO

-- Get current exchange rate
CREATE PROCEDURE usp_GetCurrentExchangeRate
    @CorridorId INT
AS
BEGIN
    SET NOCOUNT ON;
    
    SELECT 
        r.RateId,
        r.CorridorId,
        r.MidMarketRate,
        r.BuyRate,
        r.SellRate,
        r.Spread,
        r.EffectiveFrom,
        r.RateSource,
        c.OriginCurrency,
        c.DestinationCurrency
    FROM FXRates r
    JOIN Corridors c ON r.CorridorId = c.CorridorId
    WHERE r.CorridorId = @CorridorId
    AND r.EffectiveTo IS NULL;
END;
GO

-- Get exchange rate by corridor code
CREATE PROCEDURE usp_GetExchangeRateByCorridorCode
    @CorridorCode NVARCHAR(20)
AS
BEGIN
    SET NOCOUNT ON;
    
    SELECT 
        r.RateId,
        r.CorridorId,
        c.CorridorCode,
        r.MidMarketRate,
        r.BuyRate,
        r.SellRate,
        r.Spread,
        r.EffectiveFrom,
        r.RateSource,
        c.OriginCurrency,
        c.DestinationCurrency
    FROM FXRates r
    JOIN Corridors c ON r.CorridorId = c.CorridorId
    WHERE c.CorridorCode = @CorridorCode
    AND r.EffectiveTo IS NULL;
END;
GO

-- Update exchange rate
CREATE PROCEDURE usp_UpdateExchangeRate
    @CorridorId     INT,
    @MidMarketRate  DECIMAL(18,8),
    @Spread         DECIMAL(8,4),
    @RateSource     NVARCHAR(50)
AS
BEGIN
    SET NOCOUNT ON;
    
    -- End current rate
    UPDATE FXRates
    SET EffectiveTo = SYSUTCDATETIME()
    WHERE CorridorId = @CorridorId AND EffectiveTo IS NULL;
    
    -- Calculate buy/sell rates
    DECLARE @BuyRate DECIMAL(18,8) = @MidMarketRate * (1 - @Spread / 2);
    DECLARE @SellRate DECIMAL(18,8) = @MidMarketRate * (1 + @Spread / 2);
    
    -- Insert new rate
    INSERT INTO FXRates (CorridorId, MidMarketRate, BuyRate, SellRate, Spread, EffectiveFrom, RateSource)
    VALUES (@CorridorId, @MidMarketRate, @BuyRate, @SellRate, @Spread, SYSUTCDATETIME(), @RateSource);
    
    SELECT 1 AS Success, SCOPE_IDENTITY() AS RateId, @SellRate AS SellRate;
END;
GO

-- Get rate history
CREATE PROCEDURE usp_GetRateHistory
    @CorridorId     INT,
    @StartDate      DATETIME2,
    @EndDate        DATETIME2 = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    SET @EndDate = COALESCE(@EndDate, SYSUTCDATETIME());
    
    SELECT 
        RateId,
        MidMarketRate,
        SellRate,
        Spread,
        EffectiveFrom,
        EffectiveTo,
        RateSource
    FROM FXRates
    WHERE CorridorId = @CorridorId
    AND EffectiveFrom >= @StartDate
    AND EffectiveFrom <= @EndDate
    ORDER BY EffectiveFrom DESC;
END;
GO

-- Get fee schedule
CREATE PROCEDURE usp_GetFeeSchedule
    @CorridorId     INT,
    @PayoutMethod   NVARCHAR(50),
    @SendAmount     DECIMAL(18,2) = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    SELECT 
        FeeScheduleId,
        CorridorId,
        PayoutMethod,
        FeeType,
        FlatFee,
        PercentageFee,
        MinFee,
        MaxFee,
        MinAmount,
        MaxAmount,
        PromoCode,
        PromoValidFrom,
        PromoValidTo
    FROM FeeSchedules
    WHERE CorridorId = @CorridorId
    AND PayoutMethod = @PayoutMethod
    AND IsActive = 1
    AND (@SendAmount IS NULL OR (
        (MinAmount IS NULL OR @SendAmount >= MinAmount)
        AND (MaxAmount IS NULL OR @SendAmount <= MaxAmount)
    ))
    ORDER BY MinAmount;
END;
GO

-- Calculate fees
CREATE PROCEDURE usp_CalculateFees
    @CorridorId     INT,
    @PayoutMethod   NVARCHAR(50),
    @SendAmount     DECIMAL(18,2),
    @PromoCode      NVARCHAR(50) = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @FeeType NVARCHAR(20);
    DECLARE @FlatFee DECIMAL(18,2);
    DECLARE @PercentageFee DECIMAL(8,4);
    DECLARE @MinFee DECIMAL(18,2);
    DECLARE @MaxFee DECIMAL(18,2);
    DECLARE @CalculatedFee DECIMAL(18,2);
    DECLARE @PromoDiscount DECIMAL(18,2) = 0;
    
    -- Get base fee
    SELECT TOP 1
        @FeeType = FeeType,
        @FlatFee = FlatFee,
        @PercentageFee = PercentageFee,
        @MinFee = MinFee,
        @MaxFee = MaxFee
    FROM FeeSchedules
    WHERE CorridorId = @CorridorId
    AND PayoutMethod = @PayoutMethod
    AND IsActive = 1
    AND (MinAmount IS NULL OR @SendAmount >= MinAmount)
    AND (MaxAmount IS NULL OR @SendAmount <= MaxAmount)
    ORDER BY MinAmount DESC;
    
    -- Calculate fee based on type
    IF @FeeType = 'FLAT'
        SET @CalculatedFee = @FlatFee;
    ELSE IF @FeeType = 'PERCENTAGE'
        SET @CalculatedFee = @SendAmount * @PercentageFee;
    ELSE IF @FeeType = 'TIERED'
        SET @CalculatedFee = COALESCE(@FlatFee, 0) + (@SendAmount * COALESCE(@PercentageFee, 0));
    ELSE
        SET @CalculatedFee = 4.99; -- Default
    
    -- Apply min/max
    IF @MinFee IS NOT NULL AND @CalculatedFee < @MinFee
        SET @CalculatedFee = @MinFee;
    IF @MaxFee IS NOT NULL AND @CalculatedFee > @MaxFee
        SET @CalculatedFee = @MaxFee;
    
    -- Apply promo discount
    IF @PromoCode IS NOT NULL
    BEGIN
        SELECT @PromoDiscount = 
            CASE DiscountType
                WHEN 'FLAT_FEE' THEN DiscountValue
                WHEN 'PERCENTAGE_FEE' THEN @CalculatedFee * DiscountValue
                WHEN 'FREE_TRANSFER' THEN @CalculatedFee
                ELSE 0
            END
        FROM PromoCodes
        WHERE Code = @PromoCode
        AND IsActive = 1
        AND ValidFrom <= SYSUTCDATETIME()
        AND ValidTo >= SYSUTCDATETIME()
        AND (MaxUsesTotal IS NULL OR CurrentUsageCount < MaxUsesTotal);
    END
    
    DECLARE @FinalFee DECIMAL(18,2) = @CalculatedFee - @PromoDiscount;
    IF @FinalFee < 0 SET @FinalFee = 0;
    
    SELECT 
        @CalculatedFee AS BaseFee,
        @PromoDiscount AS PromoDiscount,
        @FinalFee AS TotalFee,
        @FeeType AS FeeType;
END;
GO

-- ============================================================================
-- QUOTE GENERATION
-- ============================================================================

-- Get transfer quote
CREATE PROCEDURE usp_GetTransferQuote
    @CustomerId         BIGINT,
    @CorridorId         INT,
    @SendAmount         DECIMAL(18,2),
    @PayoutMethod       NVARCHAR(50),
    @PromoCode          NVARCHAR(50) = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    -- Validate corridor is active
    DECLARE @CorridorCode NVARCHAR(20);
    DECLARE @OriginCurrency CHAR(3);
    DECLARE @DestinationCurrency CHAR(3);
    DECLARE @MinSend DECIMAL(18,2);
    DECLARE @MaxSend DECIMAL(18,2);
    DECLARE @EstDelivery INT;
    
    SELECT 
        @CorridorCode = CorridorCode,
        @OriginCurrency = OriginCurrency,
        @DestinationCurrency = DestinationCurrency,
        @MinSend = MinSendAmount,
        @MaxSend = MaxSendAmount,
        @EstDelivery = EstimatedDeliveryMinutes
    FROM Corridors WHERE CorridorId = @CorridorId AND IsActive = 1;
    
    IF @CorridorCode IS NULL
    BEGIN
        SELECT 0 AS Success, 'CORRIDOR_NOT_ACTIVE' AS ErrorCode;
        RETURN;
    END
    
    -- Validate amount
    IF @SendAmount < @MinSend OR @SendAmount > @MaxSend
    BEGIN
        SELECT 0 AS Success, 'INVALID_AMOUNT' AS ErrorCode,
               'Amount must be between ' + CAST(@MinSend AS NVARCHAR) + ' and ' + CAST(@MaxSend AS NVARCHAR) AS ErrorMessage;
        RETURN;
    END
    
    -- Check customer limits
    DECLARE @SingleLimit DECIMAL(18,2);
    SELECT @SingleLimit = SingleTxLimitUSD FROM Customers WHERE CustomerId = @CustomerId;
    
    IF @SendAmount > @SingleLimit
    BEGIN
        SELECT 0 AS Success, 'EXCEEDS_LIMIT' AS ErrorCode,
               'Amount exceeds your transaction limit of ' + CAST(@SingleLimit AS NVARCHAR) AS ErrorMessage;
        RETURN;
    END
    
    -- Get exchange rate
    DECLARE @ExchangeRate DECIMAL(18,8);
    DECLARE @MidMarketRate DECIMAL(18,8);
    
    SELECT @ExchangeRate = SellRate, @MidMarketRate = MidMarketRate
    FROM FXRates WHERE CorridorId = @CorridorId AND EffectiveTo IS NULL;
    
    IF @ExchangeRate IS NULL
    BEGIN
        SELECT 0 AS Success, 'NO_RATE' AS ErrorCode, 'Exchange rate not available' AS ErrorMessage;
        RETURN;
    END
    
    -- Calculate fees
    DECLARE @TransferFee DECIMAL(18,2);
    DECLARE @PromoDiscount DECIMAL(18,2) = 0;
    
    -- Get base fee
    SELECT TOP 1 @TransferFee = 
        CASE FeeType
            WHEN 'FLAT' THEN FlatFee
            WHEN 'PERCENTAGE' THEN @SendAmount * PercentageFee
            ELSE COALESCE(FlatFee, 0) + (@SendAmount * COALESCE(PercentageFee, 0))
        END
    FROM FeeSchedules
    WHERE CorridorId = @CorridorId AND PayoutMethod = @PayoutMethod AND IsActive = 1
    AND (MinAmount IS NULL OR @SendAmount >= MinAmount)
    AND (MaxAmount IS NULL OR @SendAmount <= MaxAmount);
    
    SET @TransferFee = COALESCE(@TransferFee, 4.99);
    
    -- Apply promo
    IF @PromoCode IS NOT NULL
    BEGIN
        SELECT @PromoDiscount = 
            CASE DiscountType
                WHEN 'FLAT_FEE' THEN DiscountValue
                WHEN 'PERCENTAGE_FEE' THEN @TransferFee * DiscountValue
                WHEN 'FREE_TRANSFER' THEN @TransferFee
                ELSE 0
            END
        FROM PromoCodes
        WHERE Code = @PromoCode AND IsActive = 1
        AND ValidFrom <= SYSUTCDATETIME() AND ValidTo >= SYSUTCDATETIME();
    END
    
    -- Calculate amounts
    DECLARE @ReceiveAmount DECIMAL(18,2) = @SendAmount * @ExchangeRate;
    DECLARE @FXMargin DECIMAL(18,2) = @SendAmount * (@ExchangeRate - @MidMarketRate) / @MidMarketRate;
    DECLARE @TotalFees DECIMAL(18,2) = @TransferFee - @PromoDiscount;
    IF @TotalFees < 0 SET @TotalFees = 0;
    DECLARE @TotalCharged DECIMAL(18,2) = @SendAmount + @TotalFees;
    
    -- Rate validity (15 minutes)
    DECLARE @RateValidUntil DATETIME2 = DATEADD(MINUTE, 15, SYSUTCDATETIME());
    DECLARE @ExpectedDelivery DATETIME2 = DATEADD(MINUTE, @EstDelivery, SYSUTCDATETIME());
    
    SELECT 
        1 AS Success,
        @CorridorCode AS CorridorCode,
        @SendAmount AS SendAmount,
        @OriginCurrency AS SendCurrency,
        @ReceiveAmount AS ReceiveAmount,
        @DestinationCurrency AS ReceiveCurrency,
        @ExchangeRate AS ExchangeRate,
        @MidMarketRate AS MidMarketRate,
        @TransferFee AS TransferFee,
        @FXMargin AS FXMargin,
        @PromoDiscount AS PromoDiscount,
        @TotalFees AS TotalFees,
        @TotalCharged AS TotalCharged,
        @PayoutMethod AS PayoutMethod,
        @RateValidUntil AS RateValidUntil,
        @ExpectedDelivery AS ExpectedDelivery;
END;
GO

-- Validate promo code
CREATE PROCEDURE usp_ValidatePromoCode
    @PromoCode      NVARCHAR(50),
    @CustomerId     BIGINT,
    @CorridorId     INT = NULL,
    @SendAmount     DECIMAL(18,2) = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @PromoCodeId INT;
    DECLARE @DiscountType NVARCHAR(20);
    DECLARE @DiscountValue DECIMAL(18,4);
    DECLARE @MaxDiscount DECIMAL(18,2);
    DECLARE @MinSend DECIMAL(18,2);
    DECLARE @MaxSend DECIMAL(18,2);
    DECLARE @EligibleCorridors NVARCHAR(500);
    DECLARE @NewCustomersOnly BIT;
    DECLARE @MaxUsesPerCustomer INT;
    DECLARE @CurrentUsageCount INT;
    DECLARE @MaxUsesTotal INT;
    
    SELECT 
        @PromoCodeId = PromoCodeId,
        @DiscountType = DiscountType,
        @DiscountValue = DiscountValue,
        @MaxDiscount = MaxDiscountAmount,
        @MinSend = MinSendAmount,
        @MaxSend = MaxSendAmount,
        @EligibleCorridors = EligibleCorridors,
        @NewCustomersOnly = NewCustomersOnly,
        @MaxUsesPerCustomer = MaxUsesPerCustomer,
        @MaxUsesTotal = MaxUsesTotal,
        @CurrentUsageCount = CurrentUsageCount
    FROM PromoCodes
    WHERE Code = @PromoCode
    AND IsActive = 1
    AND ValidFrom <= SYSUTCDATETIME()
    AND ValidTo >= SYSUTCDATETIME();
    
    IF @PromoCodeId IS NULL
    BEGIN
        SELECT 0 AS IsValid, 'INVALID_CODE' AS ErrorCode, 'Promo code is invalid or expired' AS ErrorMessage;
        RETURN;
    END
    
    -- Check total usage limit
    IF @MaxUsesTotal IS NOT NULL AND @CurrentUsageCount >= @MaxUsesTotal
    BEGIN
        SELECT 0 AS IsValid, 'CODE_EXHAUSTED' AS ErrorCode, 'This promo code has reached its usage limit' AS ErrorMessage;
        RETURN;
    END
    
    -- Check per-customer usage
    DECLARE @CustomerUsageCount INT;
    SELECT @CustomerUsageCount = COUNT(*) FROM PromoCodeUsage WHERE PromoCodeId = @PromoCodeId AND CustomerId = @CustomerId;
    
    IF @CustomerUsageCount >= @MaxUsesPerCustomer
    BEGIN
        SELECT 0 AS IsValid, 'ALREADY_USED' AS ErrorCode, 'You have already used this promo code' AS ErrorMessage;
        RETURN;
    END
    
    -- Check new customers only
    IF @NewCustomersOnly = 1
    BEGIN
        IF EXISTS (SELECT 1 FROM Transfers WHERE CustomerId = @CustomerId AND Status = 'COMPLETED')
        BEGIN
            SELECT 0 AS IsValid, 'NOT_NEW_CUSTOMER' AS ErrorCode, 'This promo is only for new customers' AS ErrorMessage;
            RETURN;
        END
    END
    
    -- Check amount limits
    IF @SendAmount IS NOT NULL
    BEGIN
        IF @MinSend IS NOT NULL AND @SendAmount < @MinSend
        BEGIN
            SELECT 0 AS IsValid, 'BELOW_MINIMUM' AS ErrorCode, 
                   'Minimum send amount for this promo is ' + CAST(@MinSend AS NVARCHAR) AS ErrorMessage;
            RETURN;
        END
        IF @MaxSend IS NOT NULL AND @SendAmount > @MaxSend
        BEGIN
            SELECT 0 AS IsValid, 'ABOVE_MAXIMUM' AS ErrorCode,
                   'Maximum send amount for this promo is ' + CAST(@MaxSend AS NVARCHAR) AS ErrorMessage;
            RETURN;
        END
    END
    
    SELECT 
        1 AS IsValid,
        @DiscountType AS DiscountType,
        @DiscountValue AS DiscountValue,
        @MaxDiscount AS MaxDiscountAmount;
END;
GO

-- ============================================================================
-- CORE TRANSFER OPERATIONS
-- ============================================================================

-- Generate transfer number
CREATE FUNCTION fn_GenerateTransferNumber()
RETURNS NVARCHAR(20)
AS
BEGIN
    DECLARE @Prefix NVARCHAR(3) = 'MS-';
    DECLARE @Year NVARCHAR(4) = CAST(YEAR(SYSUTCDATETIME()) AS NVARCHAR);
    DECLARE @Seq NVARCHAR(10);
    
    -- This would use a sequence in production
    SELECT @Seq = RIGHT('000000' + CAST(NEXT VALUE FOR TransferNumberSeq AS NVARCHAR), 6);
    
    RETURN @Prefix + @Year + '-' + @Seq;
END;
GO

-- Create sequence for transfer numbers
CREATE SEQUENCE TransferNumberSeq
    START WITH 1
    INCREMENT BY 1;
GO

-- Initiate a new transfer
CREATE PROCEDURE usp_InitiateTransfer
    @CustomerId         BIGINT,
    @BeneficiaryId      BIGINT,
    @CorridorId         INT,
    @SendAmount         DECIMAL(18,2),
    @PayoutMethod       NVARCHAR(50),
    @PayoutBankAccountId BIGINT = NULL,
    @PayoutWalletId     BIGINT = NULL,
    @PromoCode          NVARCHAR(50) = NULL,
    @Purpose            NVARCHAR(100) = NULL,
    @PurposeDescription NVARCHAR(500) = NULL,
    @SourceChannel      NVARCHAR(50) = 'WEB',
    @SourceIP           NVARCHAR(50) = NULL,
    @DeviceFingerprint  NVARCHAR(500) = NULL
AS
BEGIN
    SET NOCOUNT ON;
    BEGIN TRANSACTION;
    
    -- Validate customer
    DECLARE @CustomerStatus NVARCHAR(20);
    DECLARE @KYCStatus NVARCHAR(20);
    DECLARE @SingleLimit DECIMAL(18,2);
    DECLARE @DailyLimit DECIMAL(18,2);
    DECLARE @MonthlyLimit DECIMAL(18,2);
    
    SELECT 
        @CustomerStatus = Status,
        @KYCStatus = KYCStatus,
        @SingleLimit = SingleTxLimitUSD,
        @DailyLimit = DailyLimitUSD,
        @MonthlyLimit = MonthlyLimitUSD
    FROM Customers WHERE CustomerId = @CustomerId;
    
    IF @CustomerStatus != 'ACTIVE'
    BEGIN
        ROLLBACK;
        SELECT 0 AS Success, 'CUSTOMER_NOT_ACTIVE' AS ErrorCode;
        RETURN;
    END
    
    IF @KYCStatus != 'VERIFIED'
    BEGIN
        ROLLBACK;
        SELECT 0 AS Success, 'KYC_NOT_VERIFIED' AS ErrorCode, 'Please complete identity verification' AS ErrorMessage;
        RETURN;
    END
    
    -- Validate beneficiary
    DECLARE @BeneficiaryCustomerId BIGINT;
    DECLARE @BeneficiaryCountry CHAR(2);
    DECLARE @ScreeningStatus NVARCHAR(20);
    
    SELECT 
        @BeneficiaryCustomerId = CustomerId,
        @BeneficiaryCountry = Country,
        @ScreeningStatus = ScreeningStatus
    FROM Beneficiaries WHERE BeneficiaryId = @BeneficiaryId AND IsActive = 1;
    
    IF @BeneficiaryCustomerId IS NULL OR @BeneficiaryCustomerId != @CustomerId
    BEGIN
        ROLLBACK;
        SELECT 0 AS Success, 'INVALID_BENEFICIARY' AS ErrorCode;
        RETURN;
    END
    
    IF @ScreeningStatus = 'BLOCKED'
    BEGIN
        ROLLBACK;
        SELECT 0 AS Success, 'BENEFICIARY_BLOCKED' AS ErrorCode;
        RETURN;
    END
    
    -- Validate corridor
    DECLARE @OriginCurrency CHAR(3);
    DECLARE @DestinationCurrency CHAR(3);
    DECLARE @MinSend DECIMAL(18,2);
    DECLARE @MaxSend DECIMAL(18,2);
    DECLARE @EstDelivery INT;
    
    SELECT 
        @OriginCurrency = OriginCurrency,
        @DestinationCurrency = DestinationCurrency,
        @MinSend = MinSendAmount,
        @MaxSend = MaxSendAmount,
        @EstDelivery = EstimatedDeliveryMinutes
    FROM Corridors WHERE CorridorId = @CorridorId AND IsActive = 1;
    
    IF @OriginCurrency IS NULL
    BEGIN
        ROLLBACK;
        SELECT 0 AS Success, 'CORRIDOR_NOT_ACTIVE' AS ErrorCode;
        RETURN;
    END
    
    -- Validate amount against limits
    IF @SendAmount < @MinSend OR @SendAmount > @MaxSend
    BEGIN
        ROLLBACK;
        SELECT 0 AS Success, 'INVALID_AMOUNT' AS ErrorCode;
        RETURN;
    END
    
    IF @SendAmount > @SingleLimit
    BEGIN
        ROLLBACK;
        SELECT 0 AS Success, 'EXCEEDS_TX_LIMIT' AS ErrorCode;
        RETURN;
    END
    
    -- Check daily limit
    DECLARE @TodayTotal DECIMAL(18,2);
    SELECT @TodayTotal = COALESCE(SUM(SendAmount), 0) FROM Transfers
    WHERE CustomerId = @CustomerId 
    AND CAST(CreatedAt AS DATE) = CAST(SYSUTCDATETIME() AS DATE)
    AND Status NOT IN ('CANCELLED', 'FAILED', 'REFUNDED');
    
    IF @TodayTotal + @SendAmount > @DailyLimit
    BEGIN
        ROLLBACK;
        SELECT 0 AS Success, 'EXCEEDS_DAILY_LIMIT' AS ErrorCode,
               'Daily limit: ' + CAST(@DailyLimit AS NVARCHAR) + ', Used: ' + CAST(@TodayTotal AS NVARCHAR) AS ErrorMessage;
        RETURN;
    END
    
    -- Check monthly limit
    DECLARE @MonthTotal DECIMAL(18,2);
    SELECT @MonthTotal = COALESCE(SUM(SendAmount), 0) FROM Transfers
    WHERE CustomerId = @CustomerId 
    AND CreatedAt >= DATEADD(DAY, 1-DAY(SYSUTCDATETIME()), CAST(SYSUTCDATETIME() AS DATE))
    AND Status NOT IN ('CANCELLED', 'FAILED', 'REFUNDED');
    
    IF @MonthTotal + @SendAmount > @MonthlyLimit
    BEGIN
        ROLLBACK;
        SELECT 0 AS Success, 'EXCEEDS_MONTHLY_LIMIT' AS ErrorCode;
        RETURN;
    END
    
    -- Get exchange rate
    DECLARE @ExchangeRate DECIMAL(18,8);
    DECLARE @MidMarketRate DECIMAL(18,8);
    
    SELECT @ExchangeRate = SellRate, @MidMarketRate = MidMarketRate
    FROM FXRates WHERE CorridorId = @CorridorId AND EffectiveTo IS NULL;
    
    IF @ExchangeRate IS NULL
    BEGIN
        ROLLBACK;
        SELECT 0 AS Success, 'NO_RATE' AS ErrorCode;
        RETURN;
    END
    
    -- Calculate amounts
    DECLARE @ReceiveAmount DECIMAL(18,2) = ROUND(@SendAmount * @ExchangeRate, 2);
    DECLARE @FXMargin DECIMAL(18,2) = ROUND(@SendAmount * (@ExchangeRate - @MidMarketRate) / @MidMarketRate, 2);
    
    -- Calculate fees
    DECLARE @TransferFee DECIMAL(18,2) = 4.99;
    SELECT TOP 1 @TransferFee = 
        CASE FeeType
            WHEN 'FLAT' THEN FlatFee
            WHEN 'PERCENTAGE' THEN @SendAmount * PercentageFee
            ELSE COALESCE(FlatFee, 0) + (@SendAmount * COALESCE(PercentageFee, 0))
        END
    FROM FeeSchedules
    WHERE CorridorId = @CorridorId AND PayoutMethod = @PayoutMethod AND IsActive = 1
    AND (MinAmount IS NULL OR @SendAmount >= MinAmount);
    
    -- Apply promo
    DECLARE @PromoDiscount DECIMAL(18,2) = 0;
    DECLARE @PromoCodeId INT = NULL;
    
    IF @PromoCode IS NOT NULL
    BEGIN
        SELECT @PromoCodeId = PromoCodeId, @PromoDiscount = 
            CASE DiscountType
                WHEN 'FLAT_FEE' THEN DiscountValue
                WHEN 'PERCENTAGE_FEE' THEN @TransferFee * DiscountValue
                WHEN 'FREE_TRANSFER' THEN @TransferFee
                ELSE 0
            END
        FROM PromoCodes
        WHERE Code = @PromoCode AND IsActive = 1
        AND ValidFrom <= SYSUTCDATETIME() AND ValidTo >= SYSUTCDATETIME();
    END
    
    DECLARE @TotalFees DECIMAL(18,2) = @TransferFee - @PromoDiscount;
    IF @TotalFees < 0 SET @TotalFees = 0;
    DECLARE @TotalCharged DECIMAL(18,2) = @SendAmount + @TotalFees;
    
    -- Rate lock validity
    DECLARE @RateLockUntil DATETIME2 = DATEADD(MINUTE, 15, SYSUTCDATETIME());
    DECLARE @ExpectedDelivery DATETIME2 = DATEADD(MINUTE, @EstDelivery, SYSUTCDATETIME());
    
    -- Generate transfer number
    DECLARE @TransferNumber NVARCHAR(20);
    DECLARE @Seq INT;
    SET @Seq = NEXT VALUE FOR TransferNumberSeq;
    SET @TransferNumber = 'MS-' + CAST(YEAR(SYSUTCDATETIME()) AS NVARCHAR) + '-' + RIGHT('000000' + CAST(@Seq AS NVARCHAR), 6);
    
    -- Find payout partner
    DECLARE @PayoutPartnerId INT;
    SELECT TOP 1 @PayoutPartnerId = PartnerId 
    FROM CorridorPartners 
    WHERE CorridorId = @CorridorId AND PayoutMethod = @PayoutMethod AND IsActive = 1
    ORDER BY Priority;
    
    -- Create transfer
    INSERT INTO Transfers (
        TransferNumber, CustomerId, BeneficiaryId, CorridorId,
        SendAmount, SendCurrency, ReceiveAmount, ReceiveCurrency,
        ExchangeRate, RateLockedAt, RateLockedUntil,
        TotalFees, TransferFee, FXMargin, PromoDiscount, PromoCode,
        TotalCharged, PayoutMethod, PayoutBankAccountId, PayoutWalletId,
        PayoutPartnerId, Status, ComplianceStatus,
        Purpose, PurposeDescription, SourceChannel, SourceIP, DeviceFingerprint,
        ExpectedDeliveryAt
    )
    VALUES (
        @TransferNumber, @CustomerId, @BeneficiaryId, @CorridorId,
        @SendAmount, @OriginCurrency, @ReceiveAmount, @DestinationCurrency,
        @ExchangeRate, SYSUTCDATETIME(), @RateLockUntil,
        @TotalFees, @TransferFee, @FXMargin, @PromoDiscount, @PromoCode,
        @TotalCharged, @PayoutMethod, @PayoutBankAccountId, @PayoutWalletId,
        @PayoutPartnerId, 'CREATED', 'PENDING',
        @Purpose, @PurposeDescription, @SourceChannel, @SourceIP, @DeviceFingerprint,
        @ExpectedDelivery
    );
    
    DECLARE @TransferId BIGINT = SCOPE_IDENTITY();
    DECLARE @ExternalId UNIQUEIDENTIFIER;
    SELECT @ExternalId = ExternalId FROM Transfers WHERE TransferId = @TransferId;
    
    -- Log status history
    INSERT INTO TransferStatusHistory (TransferId, NewStatus, ChangedBy, Notes)
    VALUES (@TransferId, 'CREATED', CAST(@CustomerId AS NVARCHAR), 'Transfer initiated');
    
    -- Record promo usage if applicable
    IF @PromoCodeId IS NOT NULL
    BEGIN
        INSERT INTO PromoCodeUsage (PromoCodeId, CustomerId, TransferId, DiscountApplied)
        VALUES (@PromoCodeId, @CustomerId, @TransferId, @PromoDiscount);
        
        UPDATE PromoCodes SET CurrentUsageCount = CurrentUsageCount + 1 WHERE PromoCodeId = @PromoCodeId;
    END
    
    COMMIT;
    
    SELECT 
        1 AS Success,
        @TransferId AS TransferId,
        @ExternalId AS ExternalId,
        @TransferNumber AS TransferNumber,
        @SendAmount AS SendAmount,
        @OriginCurrency AS SendCurrency,
        @ReceiveAmount AS ReceiveAmount,
        @DestinationCurrency AS ReceiveCurrency,
        @ExchangeRate AS ExchangeRate,
        @TotalFees AS TotalFees,
        @TotalCharged AS TotalCharged,
        @RateLockUntil AS RateLockUntil,
        'CREATED' AS Status;
END;
GO

-- Get transfer by ID
CREATE PROCEDURE usp_GetTransferById
    @TransferId BIGINT
AS
BEGIN
    SET NOCOUNT ON;
    
    SELECT 
        t.TransferId,
        t.TransferNumber,
        t.ExternalId,
        t.CustomerId,
        t.BeneficiaryId,
        t.CorridorId,
        t.SendAmount,
        t.SendCurrency,
        t.ReceiveAmount,
        t.ReceiveCurrency,
        t.ExchangeRate,
        t.RateLockedAt,
        t.RateLockedUntil,
        t.TotalFees,
        t.TransferFee,
        t.FXMargin,
        t.PromoDiscount,
        t.PromoCode,
        t.TotalCharged,
        t.PayoutMethod,
        t.PayoutBankAccountId,
        t.PayoutWalletId,
        t.PayoutPartnerId,
        t.PayoutReference,
        t.CashPickupCode,
        t.CashPickupLocation,
        t.Status,
        t.SubStatus,
        t.StatusReason,
        t.ComplianceStatus,
        t.ComplianceReviewedAt,
        t.RiskScore,
        t.Purpose,
        t.PurposeDescription,
        t.CreatedAt,
        t.UpdatedAt,
        t.PaymentReceivedAt,
        t.ProcessingStartedAt,
        t.SentToPartnerAt,
        t.CompletedAt,
        t.CancelledAt,
        t.ExpectedDeliveryAt,
        t.SourceChannel,
        b.FirstName AS BeneficiaryFirstName,
        b.LastName AS BeneficiaryLastName,
        b.Country AS BeneficiaryCountry,
        c.CorridorCode,
        c.DisplayName AS CorridorName
    FROM Transfers t
    JOIN Beneficiaries b ON t.BeneficiaryId = b.BeneficiaryId
    JOIN Corridors c ON t.CorridorId = c.CorridorId
    WHERE t.TransferId = @TransferId;
END;
GO

-- Get transfer by number
CREATE PROCEDURE usp_GetTransferByNumber
    @TransferNumber NVARCHAR(20)
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @TransferId BIGINT;
    SELECT @TransferId = TransferId FROM Transfers WHERE TransferNumber = @TransferNumber;
    
    IF @TransferId IS NOT NULL
        EXEC usp_GetTransferById @TransferId;
END;
GO

-- Get transfer by external ID
CREATE PROCEDURE usp_GetTransferByExternalId
    @ExternalId UNIQUEIDENTIFIER
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @TransferId BIGINT;
    SELECT @TransferId = TransferId FROM Transfers WHERE ExternalId = @ExternalId;
    
    IF @TransferId IS NOT NULL
        EXEC usp_GetTransferById @TransferId;
END;
GO

-- List transfers by customer
CREATE PROCEDURE usp_ListTransfersByCustomer
    @CustomerId     BIGINT,
    @Status         NVARCHAR(30) = NULL,
    @StartDate      DATETIME2 = NULL,
    @EndDate        DATETIME2 = NULL,
    @PageNumber     INT = 1,
    @PageSize       INT = 20
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @Offset INT = (@PageNumber - 1) * @PageSize;
    
    SELECT 
        t.TransferId,
        t.TransferNumber,
        t.ExternalId,
        t.SendAmount,
        t.SendCurrency,
        t.ReceiveAmount,
        t.ReceiveCurrency,
        t.ExchangeRate,
        t.TotalFees,
        t.TotalCharged,
        t.PayoutMethod,
        t.Status,
        t.ComplianceStatus,
        t.CreatedAt,
        t.CompletedAt,
        t.ExpectedDeliveryAt,
        b.FirstName AS BeneficiaryFirstName,
        b.LastName AS BeneficiaryLastName,
        b.Country AS BeneficiaryCountry,
        c.DisplayName AS CorridorName
    FROM Transfers t
    JOIN Beneficiaries b ON t.BeneficiaryId = b.BeneficiaryId
    JOIN Corridors c ON t.CorridorId = c.CorridorId
    WHERE t.CustomerId = @CustomerId
    AND (@Status IS NULL OR t.Status = @Status)
    AND (@StartDate IS NULL OR t.CreatedAt >= @StartDate)
    AND (@EndDate IS NULL OR t.CreatedAt <= @EndDate)
    ORDER BY t.CreatedAt DESC
    OFFSET @Offset ROWS
    FETCH NEXT @PageSize ROWS ONLY;
    
    -- Total count
    SELECT COUNT(*) AS TotalCount FROM Transfers t
    WHERE t.CustomerId = @CustomerId
    AND (@Status IS NULL OR t.Status = @Status)
    AND (@StartDate IS NULL OR t.CreatedAt >= @StartDate)
    AND (@EndDate IS NULL OR t.CreatedAt <= @EndDate);
END;
GO

-- List transfers by status
CREATE PROCEDURE usp_ListTransfersByStatus
    @Status         NVARCHAR(30),
    @PageNumber     INT = 1,
    @PageSize       INT = 50
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @Offset INT = (@PageNumber - 1) * @PageSize;
    
    SELECT 
        t.TransferId,
        t.TransferNumber,
        t.CustomerId,
        t.SendAmount,
        t.SendCurrency,
        t.ReceiveAmount,
        t.ReceiveCurrency,
        t.PayoutMethod,
        t.Status,
        t.ComplianceStatus,
        t.CreatedAt,
        t.UpdatedAt,
        cust.Email AS CustomerEmail,
        cust.FirstName AS CustomerFirstName,
        cust.LastName AS CustomerLastName,
        b.FirstName AS BeneficiaryFirstName,
        b.LastName AS BeneficiaryLastName,
        b.Country AS BeneficiaryCountry
    FROM Transfers t
    JOIN Customers cust ON t.CustomerId = cust.CustomerId
    JOIN Beneficiaries b ON t.BeneficiaryId = b.BeneficiaryId
    WHERE t.Status = @Status
    ORDER BY t.CreatedAt ASC
    OFFSET @Offset ROWS
    FETCH NEXT @PageSize ROWS ONLY;
END;
GO

-- Get transfer status history
CREATE PROCEDURE usp_GetTransferStatusHistory
    @TransferId BIGINT
AS
BEGIN
    SET NOCOUNT ON;
    
    SELECT 
        HistoryId,
        PreviousStatus,
        NewStatus,
        SubStatus,
        Reason,
        ChangedBy,
        ChangedAt,
        Notes
    FROM TransferStatusHistory
    WHERE TransferId = @TransferId
    ORDER BY ChangedAt ASC;
END;
GO
