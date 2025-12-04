-- Present Value Calculation
-- Calculates the present value of a future sum with discount rate
-- PV = FV / (1 + r/n)^(n*t)
-- Used for discounted cash flow analysis

CREATE PROCEDURE dbo.PresentValue
    @FutureValue DECIMAL(18,4),
    @DiscountRate DECIMAL(10,6),
    @CompoundsPerYear INT,
    @Years INT,
    @PresentValue DECIMAL(18,4) OUTPUT,
    @DiscountAmount DECIMAL(18,4) OUTPUT
AS
BEGIN
    SET NOCOUNT ON
    
    DECLARE @RatePerPeriod DECIMAL(18,10)
    DECLARE @TotalPeriods INT
    DECLARE @Divisor DECIMAL(18,10)
    DECLARE @i INT
    
    -- Validate inputs
    IF @FutureValue <= 0
    BEGIN
        SET @PresentValue = 0
        SET @DiscountAmount = 0
        RETURN
    END
    
    IF @CompoundsPerYear <= 0
        SET @CompoundsPerYear = 1
    
    IF @Years < 0
        SET @Years = 0
    
    -- Calculate rate per period
    SET @RatePerPeriod = @DiscountRate / @CompoundsPerYear
    SET @TotalPeriods = @CompoundsPerYear * @Years
    
    -- Calculate (1 + r/n)^(n*t) using iterative multiplication
    SET @Divisor = 1.0
    SET @i = 0
    
    WHILE @i < @TotalPeriods
    BEGIN
        SET @Divisor = @Divisor * (1.0 + @RatePerPeriod)
        SET @i = @i + 1
    END
    
    -- Prevent division by zero
    IF @Divisor = 0
        SET @Divisor = 1
    
    -- Calculate present value
    SET @PresentValue = @FutureValue / @Divisor
    SET @DiscountAmount = @FutureValue - @PresentValue
    
    RETURN
END
