-- Loan Amortisation Calculator
-- Calculates monthly payment and generates amortisation schedule data
-- Uses standard amortisation formula: M = P[r(1+r)^n]/[(1+r)^n-1]
CREATE PROCEDURE dbo.CalculateMonthlyPayment
    @Principal DECIMAL(18,2),
    @AnnualInterestRate DECIMAL(10,6),
    @TermMonths INT,
    @MonthlyPayment DECIMAL(18,2) OUTPUT,
    @TotalPayment DECIMAL(18,2) OUTPUT,
    @TotalInterest DECIMAL(18,2) OUTPUT
AS
BEGIN
    DECLARE @MonthlyRate DECIMAL(18,10)
    DECLARE @CompoundFactor DECIMAL(18,10)
    DECLARE @I INT
    
    -- Handle zero interest case
    IF @AnnualInterestRate = 0
    BEGIN
        SET @MonthlyPayment = @Principal / @TermMonths
        SET @TotalPayment = @Principal
        SET @TotalInterest = 0
        RETURN
    END
    
    -- Calculate monthly rate
    SET @MonthlyRate = @AnnualInterestRate / 100.0 / 12.0
    
    -- Calculate (1 + r)^n using iteration
    SET @CompoundFactor = 1.0
    SET @I = 0
    WHILE @I < @TermMonths
    BEGIN
        SET @CompoundFactor = @CompoundFactor * (1.0 + @MonthlyRate)
        SET @I = @I + 1
    END
    
    -- M = P * [r * (1+r)^n] / [(1+r)^n - 1]
    SET @MonthlyPayment = @Principal * (@MonthlyRate * @CompoundFactor) / (@CompoundFactor - 1.0)
    
    -- Round to 2 decimal places
    SET @MonthlyPayment = ROUND(@MonthlyPayment, 2)
    
    SET @TotalPayment = @MonthlyPayment * @TermMonths
    SET @TotalInterest = @TotalPayment - @Principal
END
