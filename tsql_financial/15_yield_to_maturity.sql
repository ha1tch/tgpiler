-- Yield to Maturity (YTM) Calculator
-- Uses bisection method to find the yield that makes price = present value
-- YTM is the internal rate of return on a bond held to maturity

CREATE PROCEDURE dbo.YieldToMaturity
    @FaceValue DECIMAL(18,4),
    @CurrentPrice DECIMAL(18,4),
    @CouponRate DECIMAL(10,6),
    @YearsToMaturity INT,
    @PaymentsPerYear INT,
    @YTM DECIMAL(10,6) OUTPUT,
    @ApproxYTM DECIMAL(10,6) OUTPUT,  -- Quick approximation formula
    @Iterations INT OUTPUT,
    @Converged BIT OUTPUT
AS
BEGIN
    SET NOCOUNT ON
    
    DECLARE @Low DECIMAL(18,10)
    DECLARE @High DECIMAL(18,10)
    DECLARE @Mid DECIMAL(18,10)
    DECLARE @PriceLow DECIMAL(18,10)
    DECLARE @PriceHigh DECIMAL(18,10)
    DECLARE @PriceMid DECIMAL(18,10)
    DECLARE @CouponPayment DECIMAL(18,10)
    DECLARE @TotalPeriods INT
    DECLARE @Tolerance DECIMAL(18,10)
    DECLARE @MaxIterations INT
    DECLARE @AnnualCoupon DECIMAL(18,10)
    DECLARE @i INT
    
    SET @Tolerance = 0.0001  -- Price tolerance
    SET @MaxIterations = 100
    SET @Converged = 0
    SET @Iterations = 0
    
    -- Validate inputs
    IF @FaceValue <= 0 OR @CurrentPrice <= 0
    BEGIN
        SET @YTM = 0
        SET @ApproxYTM = 0
        RETURN
    END
    
    IF @PaymentsPerYear <= 0
        SET @PaymentsPerYear = 2
    
    IF @YearsToMaturity <= 0
    BEGIN
        SET @YTM = 0
        SET @ApproxYTM = 0
        RETURN
    END
    
    -- Calculate values
    SET @AnnualCoupon = @FaceValue * @CouponRate
    SET @CouponPayment = @AnnualCoupon / @PaymentsPerYear
    SET @TotalPeriods = @YearsToMaturity * @PaymentsPerYear
    
    -- Quick approximation formula (for comparison):
    -- YTM ≈ [C + (FV - P) / n] / [(FV + P) / 2]
    SET @ApproxYTM = (@AnnualCoupon + ((@FaceValue - @CurrentPrice) / @YearsToMaturity)) / 
                     ((@FaceValue + @CurrentPrice) / 2.0)
    
    -- Bisection method to find exact YTM
    SET @Low = 0.0001   -- 0.01%
    SET @High = 1.0     -- 100%
    
    -- Helper to calculate bond price at a given yield
    -- This is done inline since we can't call stored procedures recursively
    
    SET @i = 0
    WHILE @i < @MaxIterations
    BEGIN
        SET @Mid = (@Low + @High) / 2.0
        
        -- Calculate price at mid yield
        DECLARE @PeriodicRate DECIMAL(18,10)
        DECLARE @DiscountFactor DECIMAL(18,10)
        DECLARE @PVCoupons DECIMAL(18,10)
        DECLARE @PVFace DECIMAL(18,10)
        DECLARE @j INT
        DECLARE @k INT
        
        SET @PeriodicRate = @Mid / @PaymentsPerYear
        SET @PVCoupons = 0
        
        SET @j = 1
        WHILE @j <= @TotalPeriods
        BEGIN
            SET @DiscountFactor = 1.0
            SET @k = 0
            WHILE @k < @j
            BEGIN
                SET @DiscountFactor = @DiscountFactor / (1.0 + @PeriodicRate)
                SET @k = @k + 1
            END
            SET @PVCoupons = @PVCoupons + (@CouponPayment * @DiscountFactor)
            SET @j = @j + 1
        END
        
        -- PV of face value
        SET @DiscountFactor = 1.0
        SET @k = 0
        WHILE @k < @TotalPeriods
        BEGIN
            SET @DiscountFactor = @DiscountFactor / (1.0 + @PeriodicRate)
            SET @k = @k + 1
        END
        SET @PVFace = @FaceValue * @DiscountFactor
        
        SET @PriceMid = @PVCoupons + @PVFace
        
        -- Check convergence
        IF ABS(@PriceMid - @CurrentPrice) < @Tolerance
        BEGIN
            SET @Converged = 1
            SET @YTM = @Mid
            SET @Iterations = @i + 1
            RETURN
        END
        
        -- Bisect
        -- Higher yield = Lower price
        IF @PriceMid > @CurrentPrice
            SET @Low = @Mid
        ELSE
            SET @High = @Mid
        
        SET @i = @i + 1
    END
    
    -- Return best estimate
    SET @YTM = @Mid
    SET @Iterations = @MaxIterations
    SET @Converged = 0
    
    RETURN
END
