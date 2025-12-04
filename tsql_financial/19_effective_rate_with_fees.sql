-- Effective Interest Rate with Fees Calculator
-- Calculates true effective rate considering all costs
-- Includes origination fees, points, closing costs, and periodic fees
-- Uses iterative method to find rate

CREATE PROCEDURE dbo.EffectiveRateWithFees
    @Principal DECIMAL(18,4),
    @NominalAnnualRate DECIMAL(10,6),
    @TermMonths INT,
    @OriginationFee DECIMAL(18,4),      -- Upfront fee
    @DiscountPoints DECIMAL(10,4),       -- Points (1 point = 1% of principal)
    @ClosingCosts DECIMAL(18,4),         -- Other upfront costs
    @MonthlyFee DECIMAL(18,4),           -- Monthly service fee
    @CompoundingPeriodsPerYear INT,      -- For nominal to effective conversion
    @EffectiveAnnualRate DECIMAL(10,6) OUTPUT,
    @TrueAPR DECIMAL(10,6) OUTPUT,
    @TotalUpfrontCosts DECIMAL(18,4) OUTPUT,
    @TotalPeriodicCosts DECIMAL(18,4) OUTPUT,
    @TotalFinanceCharge DECIMAL(18,4) OUTPUT,
    @NetProceeds DECIMAL(18,4) OUTPUT
AS
BEGIN
    SET NOCOUNT ON
    
    DECLARE @MonthlyRate DECIMAL(18,10)
    DECLARE @Payment DECIMAL(18,10)
    DECLARE @Multiplier DECIMAL(18,10)
    DECLARE @Numerator DECIMAL(18,10)
    DECLARE @Denominator DECIMAL(18,10)
    DECLARE @PointsCost DECIMAL(18,4)
    DECLARE @TotalInterest DECIMAL(18,4)
    DECLARE @EffectiveRate DECIMAL(18,10)
    DECLARE @i INT
    DECLARE @TotalPayments DECIMAL(18,4)
    
    -- Validate inputs
    IF @Principal <= 0 OR @TermMonths <= 0
    BEGIN
        SET @EffectiveAnnualRate = 0
        SET @TrueAPR = 0
        SET @TotalUpfrontCosts = 0
        SET @TotalPeriodicCosts = 0
        SET @TotalFinanceCharge = 0
        SET @NetProceeds = 0
        RETURN
    END
    
    IF @CompoundingPeriodsPerYear <= 0
        SET @CompoundingPeriodsPerYear = 12
    
    -- Calculate points cost
    SET @PointsCost = @Principal * (@DiscountPoints / 100.0)
    
    -- Total upfront costs
    SET @TotalUpfrontCosts = @OriginationFee + @PointsCost + @ClosingCosts
    
    -- Net proceeds (what borrower actually receives)
    SET @NetProceeds = @Principal - @TotalUpfrontCosts
    
    IF @NetProceeds <= 0
    BEGIN
        -- Fees exceed principal
        SET @EffectiveAnnualRate = 9.9999
        SET @TrueAPR = 9.9999
        SET @TotalPeriodicCosts = @MonthlyFee * @TermMonths
        SET @TotalFinanceCharge = @TotalUpfrontCosts + @TotalPeriodicCosts
        RETURN
    END
    
    -- Calculate monthly payment based on nominal rate
    SET @MonthlyRate = @NominalAnnualRate / 12.0
    
    IF @MonthlyRate = 0
    BEGIN
        SET @Payment = @Principal / @TermMonths
    END
    ELSE
    BEGIN
        SET @Multiplier = 1.0
        SET @i = 0
        WHILE @i < @TermMonths
        BEGIN
            SET @Multiplier = @Multiplier * (1.0 + @MonthlyRate)
            SET @i = @i + 1
        END
        
        SET @Numerator = @MonthlyRate * @Multiplier
        SET @Denominator = @Multiplier - 1.0
        
        IF @Denominator = 0
            SET @Payment = @Principal / @TermMonths
        ELSE
            SET @Payment = @Principal * (@Numerator / @Denominator)
    END
    
    -- Add monthly fee to payment
    DECLARE @TotalMonthlyPayment DECIMAL(18,4)
    SET @TotalMonthlyPayment = @Payment + @MonthlyFee
    
    -- Calculate total periodic costs (monthly fees)
    SET @TotalPeriodicCosts = @MonthlyFee * @TermMonths
    
    -- Total payments and interest
    SET @TotalPayments = @TotalMonthlyPayment * @TermMonths
    SET @TotalInterest = (@Payment * @TermMonths) - @Principal
    
    -- Total finance charge (all costs)
    SET @TotalFinanceCharge = @TotalInterest + @TotalUpfrontCosts + @TotalPeriodicCosts
    
    -- Calculate effective annual rate from nominal rate
    -- EAR = (1 + r/n)^n - 1
    SET @Multiplier = 1.0
    SET @i = 0
    WHILE @i < @CompoundingPeriodsPerYear
    BEGIN
        SET @Multiplier = @Multiplier * (1.0 + (@NominalAnnualRate / @CompoundingPeriodsPerYear))
        SET @i = @i + 1
    END
    SET @EffectiveAnnualRate = @Multiplier - 1.0
    
    -- Calculate True APR using Actuarial Method approximation
    -- APR = 2 * n * F / [P * (N + 1)]
    -- Where: n = payments per year, F = finance charge, P = amount financed, N = total payments
    DECLARE @AmountFinanced DECIMAL(18,4)
    SET @AmountFinanced = @NetProceeds
    
    IF @AmountFinanced > 0 AND @TermMonths > 0
    BEGIN
        SET @TrueAPR = (2.0 * 12.0 * @TotalFinanceCharge) / 
                       (@AmountFinanced * (CAST(@TermMonths AS DECIMAL(18,4)) + 1.0))
    END
    ELSE
    BEGIN
        SET @TrueAPR = @EffectiveAnnualRate
    END
    
    -- Iterative refinement of True APR using Newton-Raphson
    -- Find rate r where: NetProceeds = Σ[Payment / (1+r/12)^t]
    DECLARE @TestRate DECIMAL(18,10)
    DECLARE @PV DECIMAL(18,10)
    DECLARE @dPV DECIMAL(18,10)
    DECLARE @j INT
    DECLARE @MaxIterations INT
    
    SET @TestRate = @TrueAPR  -- Start with approximation
    SET @MaxIterations = 20
    SET @j = 0
    
    WHILE @j < @MaxIterations
    BEGIN
        SET @PV = 0
        SET @dPV = 0
        SET @i = 1
        
        WHILE @i <= @TermMonths
        BEGIN
            -- Discount factor
            DECLARE @Factor DECIMAL(18,10)
            DECLARE @k INT
            SET @Factor = 1.0
            SET @k = 0
            WHILE @k < @i
            BEGIN
                SET @Factor = @Factor / (1.0 + (@TestRate / 12.0))
                SET @k = @k + 1
            END
            
            SET @PV = @PV + (@TotalMonthlyPayment * @Factor)
            SET @dPV = @dPV - (@i * @TotalMonthlyPayment * @Factor / (12.0 * (1.0 + @TestRate / 12.0)))
            
            SET @i = @i + 1
        END
        
        -- Newton step
        DECLARE @Error DECIMAL(18,10)
        SET @Error = @PV - @NetProceeds
        
        IF ABS(@Error) < 0.01  -- Close enough
            SET @j = @MaxIterations
        ELSE IF ABS(@dPV) > 0.0001
            SET @TestRate = @TestRate - (@Error / @dPV)
        
        IF @TestRate < 0
            SET @TestRate = 0.001
        IF @TestRate > 1.0
            SET @TestRate = 1.0
            
        SET @j = @j + 1
    END
    
    SET @TrueAPR = @TestRate
    
    RETURN
END
