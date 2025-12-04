-- Simple Interest Calculation
-- Calculates simple interest and total amount
-- I = P * r * t
-- A = P + I
-- Includes daily, monthly, and annual calculation modes

CREATE PROCEDURE dbo.SimpleInterest
    @Principal DECIMAL(18,4),
    @AnnualRate DECIMAL(10,6),
    @TimePeriod INT,
    @TimeUnit INT,  -- 1=Days, 2=Months, 3=Years
    @Interest DECIMAL(18,4) OUTPUT,
    @TotalAmount DECIMAL(18,4) OUTPUT,
    @EffectiveRate DECIMAL(10,6) OUTPUT
AS
BEGIN
    SET NOCOUNT ON
    
    DECLARE @TimeInYears DECIMAL(18,10)
    DECLARE @DaysInYear INT
    
    SET @DaysInYear = 365
    
    -- Validate inputs
    IF @Principal <= 0
    BEGIN
        SET @Interest = 0
        SET @TotalAmount = 0
        SET @EffectiveRate = 0
        RETURN
    END
    
    IF @TimePeriod < 0
        SET @TimePeriod = 0
    
    -- Convert time period to years
    IF @TimeUnit = 1  -- Days
    BEGIN
        SET @TimeInYears = CAST(@TimePeriod AS DECIMAL(18,10)) / @DaysInYear
    END
    ELSE IF @TimeUnit = 2  -- Months
    BEGIN
        SET @TimeInYears = CAST(@TimePeriod AS DECIMAL(18,10)) / 12.0
    END
    ELSE  -- Years (default)
    BEGIN
        SET @TimeInYears = CAST(@TimePeriod AS DECIMAL(18,10))
    END
    
    -- Calculate simple interest: I = P * r * t
    SET @Interest = @Principal * @AnnualRate * @TimeInYears
    
    -- Calculate total amount: A = P + I
    SET @TotalAmount = @Principal + @Interest
    
    -- Calculate effective rate for the period
    IF @Principal > 0
        SET @EffectiveRate = @Interest / @Principal
    ELSE
        SET @EffectiveRate = 0
    
    RETURN
END
