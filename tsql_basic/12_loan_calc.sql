-- Calculate monthly loan payment
-- Uses standard amortization formula
CREATE PROCEDURE dbo.CalculateLoanPayment
    @Principal DECIMAL(18,2),
    @AnnualRate DECIMAL(5,4),
    @TermMonths INT,
    @MonthlyPayment DECIMAL(18,2) OUTPUT,
    @TotalInterest DECIMAL(18,2) OUTPUT
AS
BEGIN
    DECLARE @MonthlyRate DECIMAL(10,8)
    DECLARE @TotalPayment DECIMAL(18,2)
    
    -- Handle zero interest case
    IF @AnnualRate = 0
    BEGIN
        SET @MonthlyPayment = @Principal / @TermMonths
        SET @TotalInterest = 0
        RETURN
    END
    
    -- Calculate monthly interest rate
    SET @MonthlyRate = @AnnualRate / 12
    
    -- Monthly payment formula: P * (r(1+r)^n) / ((1+r)^n - 1)
    SET @MonthlyPayment = @Principal * 
        (@MonthlyRate * POWER(1 + @MonthlyRate, @TermMonths)) /
        (POWER(1 + @MonthlyRate, @TermMonths) - 1)
    
    -- Calculate total interest
    SET @TotalPayment = @MonthlyPayment * @TermMonths
    SET @TotalInterest = @TotalPayment - @Principal
END
