-- Loan Payment Calculation (PMT)
-- Calculates the periodic payment for a loan
-- PMT = P * [r(1+r)^n] / [(1+r)^n - 1]
-- Where: P = principal, r = periodic rate, n = number of payments

CREATE PROCEDURE dbo.LoanPayment
    @Principal DECIMAL(18,4),
    @AnnualRate DECIMAL(10,6),
    @PaymentsPerYear INT,
    @LoanTermYears INT,
    @MonthlyPayment DECIMAL(18,4) OUTPUT,
    @TotalPayments DECIMAL(18,4) OUTPUT,
    @TotalInterest DECIMAL(18,4) OUTPUT
AS
BEGIN
    SET NOCOUNT ON
    
    DECLARE @PeriodicRate DECIMAL(18,10)
    DECLARE @NumPayments INT
    DECLARE @Multiplier DECIMAL(18,10)
    DECLARE @i INT
    DECLARE @Numerator DECIMAL(18,10)
    DECLARE @Denominator DECIMAL(18,10)
    
    -- Validate inputs
    IF @Principal <= 0
    BEGIN
        SET @MonthlyPayment = 0
        SET @TotalPayments = 0
        SET @TotalInterest = 0
        RETURN
    END
    
    IF @PaymentsPerYear <= 0
        SET @PaymentsPerYear = 12
    
    IF @LoanTermYears <= 0
    BEGIN
        SET @MonthlyPayment = @Principal
        SET @TotalPayments = @Principal
        SET @TotalInterest = 0
        RETURN
    END
    
    -- Calculate periodic rate and number of payments
    SET @PeriodicRate = @AnnualRate / @PaymentsPerYear
    SET @NumPayments = @PaymentsPerYear * @LoanTermYears
    
    -- Handle zero interest rate
    IF @AnnualRate = 0 OR @PeriodicRate = 0
    BEGIN
        SET @MonthlyPayment = @Principal / @NumPayments
        SET @TotalPayments = @Principal
        SET @TotalInterest = 0
        RETURN
    END
    
    -- Calculate (1 + r)^n
    SET @Multiplier = 1.0
    SET @i = 0
    
    WHILE @i < @NumPayments
    BEGIN
        SET @Multiplier = @Multiplier * (1.0 + @PeriodicRate)
        SET @i = @i + 1
    END
    
    -- PMT = P * [r(1+r)^n] / [(1+r)^n - 1]
    SET @Numerator = @PeriodicRate * @Multiplier
    SET @Denominator = @Multiplier - 1.0
    
    IF @Denominator = 0
    BEGIN
        SET @MonthlyPayment = @Principal / @NumPayments
    END
    ELSE
    BEGIN
        SET @MonthlyPayment = @Principal * (@Numerator / @Denominator)
    END
    
    -- Calculate totals
    SET @TotalPayments = @MonthlyPayment * @NumPayments
    SET @TotalInterest = @TotalPayments - @Principal
    
    RETURN
END
