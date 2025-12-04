-- Amortization Schedule - Single Period Calculator
-- Calculates payment breakdown for any period of a loan
-- Shows principal, interest, and remaining balance
-- Uses iterative calculation to track running totals

CREATE PROCEDURE dbo.AmortizationPeriod
    @Principal DECIMAL(18,4),
    @AnnualRate DECIMAL(10,6),
    @TotalMonths INT,
    @PeriodNumber INT,  -- Which period (1 to TotalMonths)
    @MonthlyPayment DECIMAL(18,4) OUTPUT,
    @PrincipalPayment DECIMAL(18,4) OUTPUT,
    @InterestPayment DECIMAL(18,4) OUTPUT,
    @RemainingBalance DECIMAL(18,4) OUTPUT,
    @TotalPrincipalPaid DECIMAL(18,4) OUTPUT,
    @TotalInterestPaid DECIMAL(18,4) OUTPUT
AS
BEGIN
    SET NOCOUNT ON
    
    DECLARE @MonthlyRate DECIMAL(18,10)
    DECLARE @Multiplier DECIMAL(18,10)
    DECLARE @Numerator DECIMAL(18,10)
    DECLARE @Denominator DECIMAL(18,10)
    DECLARE @Balance DECIMAL(18,4)
    DECLARE @InterestPortion DECIMAL(18,4)
    DECLARE @PrincipalPortion DECIMAL(18,4)
    DECLARE @CumulativePrincipal DECIMAL(18,4)
    DECLARE @CumulativeInterest DECIMAL(18,4)
    DECLARE @i INT
    
    -- Validate inputs
    IF @Principal <= 0 OR @TotalMonths <= 0
    BEGIN
        SET @MonthlyPayment = 0
        SET @PrincipalPayment = 0
        SET @InterestPayment = 0
        SET @RemainingBalance = 0
        SET @TotalPrincipalPaid = 0
        SET @TotalInterestPaid = 0
        RETURN
    END
    
    IF @PeriodNumber < 1
        SET @PeriodNumber = 1
    
    IF @PeriodNumber > @TotalMonths
        SET @PeriodNumber = @TotalMonths
    
    -- Calculate monthly rate
    SET @MonthlyRate = @AnnualRate / 12.0
    
    -- Handle zero interest rate
    IF @AnnualRate = 0 OR @MonthlyRate = 0
    BEGIN
        SET @MonthlyPayment = @Principal / @TotalMonths
        SET @InterestPayment = 0
        SET @PrincipalPayment = @MonthlyPayment
        SET @RemainingBalance = @Principal - (@MonthlyPayment * @PeriodNumber)
        SET @TotalPrincipalPaid = @MonthlyPayment * @PeriodNumber
        SET @TotalInterestPaid = 0
        
        IF @RemainingBalance < 0
            SET @RemainingBalance = 0
        
        RETURN
    END
    
    -- Calculate (1 + r)^n for PMT formula
    SET @Multiplier = 1.0
    SET @i = 0
    
    WHILE @i < @TotalMonths
    BEGIN
        SET @Multiplier = @Multiplier * (1.0 + @MonthlyRate)
        SET @i = @i + 1
    END
    
    -- PMT = P * [r(1+r)^n] / [(1+r)^n - 1]
    SET @Numerator = @MonthlyRate * @Multiplier
    SET @Denominator = @Multiplier - 1.0
    
    IF @Denominator = 0
        SET @MonthlyPayment = @Principal / @TotalMonths
    ELSE
        SET @MonthlyPayment = @Principal * (@Numerator / @Denominator)
    
    -- Calculate amortization up to the requested period
    SET @Balance = @Principal
    SET @CumulativePrincipal = 0
    SET @CumulativeInterest = 0
    SET @i = 1
    
    WHILE @i <= @PeriodNumber
    BEGIN
        -- Interest for this period
        SET @InterestPortion = @Balance * @MonthlyRate
        
        -- Principal for this period
        SET @PrincipalPortion = @MonthlyPayment - @InterestPortion
        
        -- Handle final payment rounding
        IF @i = @TotalMonths OR @PrincipalPortion > @Balance
        BEGIN
            SET @PrincipalPortion = @Balance
            SET @InterestPortion = @MonthlyPayment - @PrincipalPortion
            IF @InterestPortion < 0
                SET @InterestPortion = 0
        END
        
        -- Update running totals
        SET @CumulativePrincipal = @CumulativePrincipal + @PrincipalPortion
        SET @CumulativeInterest = @CumulativeInterest + @InterestPortion
        SET @Balance = @Balance - @PrincipalPortion
        
        IF @Balance < 0
            SET @Balance = 0
        
        -- Save this period's values if it's the one we want
        IF @i = @PeriodNumber
        BEGIN
            SET @PrincipalPayment = @PrincipalPortion
            SET @InterestPayment = @InterestPortion
        END
        
        SET @i = @i + 1
    END
    
    -- Set outputs
    SET @RemainingBalance = @Balance
    SET @TotalPrincipalPaid = @CumulativePrincipal
    SET @TotalInterestPaid = @CumulativeInterest
    
    RETURN
END
