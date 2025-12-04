-- Bond Pricing Calculator
-- Calculates the fair value price of a bond
-- Price = Σ [C / (1+r)^t] + [FV / (1+r)^n]
-- Where: C = coupon payment, r = yield/period, FV = face value, n = periods

CREATE PROCEDURE dbo.BondPrice
    @FaceValue DECIMAL(18,4),
    @CouponRate DECIMAL(10,6),      -- Annual coupon rate
    @MarketYield DECIMAL(10,6),     -- Required yield (discount rate)
    @YearsToMaturity INT,
    @PaymentsPerYear INT,           -- 1=Annual, 2=Semi-annual, 4=Quarterly
    @BondPrice DECIMAL(18,4) OUTPUT,
    @CurrentYield DECIMAL(10,6) OUTPUT,
    @AnnualCoupon DECIMAL(18,4) OUTPUT,
    @TotalCouponPayments DECIMAL(18,4) OUTPUT,
    @PriceType VARCHAR(20) OUTPUT   -- Premium, Par, Discount
AS
BEGIN
    SET NOCOUNT ON
    
    DECLARE @CouponPayment DECIMAL(18,10)
    DECLARE @PeriodicYield DECIMAL(18,10)
    DECLARE @TotalPeriods INT
    DECLARE @PVCoupons DECIMAL(18,10)
    DECLARE @PVFaceValue DECIMAL(18,10)
    DECLARE @DiscountFactor DECIMAL(18,10)
    DECLARE @i INT
    
    -- Validate inputs
    IF @FaceValue <= 0
    BEGIN
        SET @BondPrice = 0
        SET @CurrentYield = 0
        SET @AnnualCoupon = 0
        SET @TotalCouponPayments = 0
        SET @PriceType = 'Invalid'
        RETURN
    END
    
    IF @PaymentsPerYear <= 0
        SET @PaymentsPerYear = 2  -- Default to semi-annual
    
    IF @YearsToMaturity <= 0
    BEGIN
        -- Bond at maturity - return face value
        SET @BondPrice = @FaceValue
        SET @AnnualCoupon = @FaceValue * @CouponRate
        SET @CurrentYield = @CouponRate
        SET @TotalCouponPayments = 0
        SET @PriceType = 'Par'
        RETURN
    END
    
    -- Calculate periodic values
    SET @CouponPayment = (@FaceValue * @CouponRate) / @PaymentsPerYear
    SET @PeriodicYield = @MarketYield / @PaymentsPerYear
    SET @TotalPeriods = @YearsToMaturity * @PaymentsPerYear
    SET @AnnualCoupon = @FaceValue * @CouponRate
    
    -- Calculate present value of coupon payments
    SET @PVCoupons = 0
    SET @i = 1
    
    WHILE @i <= @TotalPeriods
    BEGIN
        -- Discount factor: 1 / (1 + r)^i
        SET @DiscountFactor = 1.0
        DECLARE @j INT
        SET @j = 0
        WHILE @j < @i
        BEGIN
            SET @DiscountFactor = @DiscountFactor / (1.0 + @PeriodicYield)
            SET @j = @j + 1
        END
        
        SET @PVCoupons = @PVCoupons + (@CouponPayment * @DiscountFactor)
        SET @i = @i + 1
    END
    
    -- Calculate present value of face value
    SET @DiscountFactor = 1.0
    SET @i = 0
    WHILE @i < @TotalPeriods
    BEGIN
        SET @DiscountFactor = @DiscountFactor / (1.0 + @PeriodicYield)
        SET @i = @i + 1
    END
    SET @PVFaceValue = @FaceValue * @DiscountFactor
    
    -- Total bond price
    SET @BondPrice = @PVCoupons + @PVFaceValue
    
    -- Calculate current yield: Annual Coupon / Price
    IF @BondPrice > 0
        SET @CurrentYield = @AnnualCoupon / @BondPrice
    ELSE
        SET @CurrentYield = 0
    
    -- Total coupon payments over life of bond
    SET @TotalCouponPayments = @CouponPayment * @TotalPeriods
    
    -- Determine if bond trades at premium, par, or discount
    IF @BondPrice > (@FaceValue * 1.001)  -- > 0.1% above par
        SET @PriceType = 'Premium'
    ELSE IF @BondPrice < (@FaceValue * 0.999)  -- > 0.1% below par
        SET @PriceType = 'Discount'
    ELSE
        SET @PriceType = 'Par'
    
    RETURN
END
