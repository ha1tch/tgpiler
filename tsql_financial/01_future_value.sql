-- Future Value Calculation
-- Calculates the future value of an investment with compound interest
-- FV = PV * (1 + r/n)^(n*t)
-- Where: PV = present value, r = annual rate, n = compounds per year, t = years

CREATE PROCEDURE dbo.FutureValue
    @PresentValue DECIMAL(18,4),
    @AnnualRate DECIMAL(10,6),
    @CompoundsPerYear INT,
    @Years INT,
    @FutureValue DECIMAL(18,4) OUTPUT,
    @TotalInterest DECIMAL(18,4) OUTPUT
AS
BEGIN
    SET NOCOUNT ON
    
    DECLARE @RatePerPeriod DECIMAL(18,10)
    DECLARE @TotalPeriods INT
    DECLARE @Multiplier DECIMAL(18,10)
    DECLARE @i INT
    
    -- Validate inputs
    IF @PresentValue <= 0
    BEGIN
        SET @FutureValue = 0
        SET @TotalInterest = 0
        RETURN
    END
    
    IF @CompoundsPerYear <= 0
        SET @CompoundsPerYear = 1
    
    IF @Years < 0
        SET @Years = 0
    
    -- Calculate rate per period
    SET @RatePerPeriod = @AnnualRate / @CompoundsPerYear
    SET @TotalPeriods = @CompoundsPerYear * @Years
    
    -- Calculate (1 + r/n)^(n*t) using iterative multiplication
    SET @Multiplier = 1.0
    SET @i = 0
    
    WHILE @i < @TotalPeriods
    BEGIN
        SET @Multiplier = @Multiplier * (1.0 + @RatePerPeriod)
        SET @i = @i + 1
    END
    
    -- Calculate future value
    SET @FutureValue = @PresentValue * @Multiplier
    SET @TotalInterest = @FutureValue - @PresentValue
    
    RETURN
END
