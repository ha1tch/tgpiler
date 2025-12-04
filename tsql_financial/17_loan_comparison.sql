-- Loan Comparison Calculator
-- Compares two loans and calculates total cost difference
-- Considers principal, rate, term, and fees
-- Computes true APR including fees

CREATE PROCEDURE dbo.LoanComparison
    -- Loan 1 parameters
    @Principal1 DECIMAL(18,4),
    @AnnualRate1 DECIMAL(10,6),
    @TermMonths1 INT,
    @OriginationFee1 DECIMAL(18,4),
    -- Loan 2 parameters
    @Principal2 DECIMAL(18,4),
    @AnnualRate2 DECIMAL(10,6),
    @TermMonths2 INT,
    @OriginationFee2 DECIMAL(18,4),
    -- Loan 1 outputs
    @Payment1 DECIMAL(18,4) OUTPUT,
    @TotalCost1 DECIMAL(18,4) OUTPUT,
    @TotalInterest1 DECIMAL(18,4) OUTPUT,
    @TrueAPR1 DECIMAL(10,6) OUTPUT,
    -- Loan 2 outputs
    @Payment2 DECIMAL(18,4) OUTPUT,
    @TotalCost2 DECIMAL(18,4) OUTPUT,
    @TotalInterest2 DECIMAL(18,4) OUTPUT,
    @TrueAPR2 DECIMAL(10,6) OUTPUT,
    -- Comparison outputs
    @Savings DECIMAL(18,4) OUTPUT,
    @BetterLoan INT OUTPUT  -- 1 or 2
AS
BEGIN
    SET NOCOUNT ON
    
    DECLARE @MonthlyRate DECIMAL(18,10)
    DECLARE @Multiplier DECIMAL(18,10)
    DECLARE @Numerator DECIMAL(18,10)
    DECLARE @Denominator DECIMAL(18,10)
    DECLARE @i INT
    
    -- Calculate Loan 1
    IF @Principal1 <= 0 OR @TermMonths1 <= 0
    BEGIN
        SET @Payment1 = 0
        SET @TotalCost1 = @OriginationFee1
        SET @TotalInterest1 = 0
        SET @TrueAPR1 = 0
    END
    ELSE
    BEGIN
        SET @MonthlyRate = @AnnualRate1 / 12.0
        
        IF @MonthlyRate = 0
        BEGIN
            SET @Payment1 = @Principal1 / @TermMonths1
        END
        ELSE
        BEGIN
            SET @Multiplier = 1.0
            SET @i = 0
            WHILE @i < @TermMonths1
            BEGIN
                SET @Multiplier = @Multiplier * (1.0 + @MonthlyRate)
                SET @i = @i + 1
            END
            
            SET @Numerator = @MonthlyRate * @Multiplier
            SET @Denominator = @Multiplier - 1.0
            
            IF @Denominator = 0
                SET @Payment1 = @Principal1 / @TermMonths1
            ELSE
                SET @Payment1 = @Principal1 * (@Numerator / @Denominator)
        END
        
        SET @TotalInterest1 = (@Payment1 * @TermMonths1) - @Principal1
        SET @TotalCost1 = (@Payment1 * @TermMonths1) + @OriginationFee1
        
        -- True APR considers fees: use net proceeds
        DECLARE @NetProceeds1 DECIMAL(18,4)
        SET @NetProceeds1 = @Principal1 - @OriginationFee1
        
        IF @NetProceeds1 > 0 AND @MonthlyRate > 0
        BEGIN
            -- Approximate true APR by solving for rate with net proceeds
            -- This is a simplification; exact solution requires iteration
            -- APR ≈ (Total Interest + Fees) / (Principal * Years) * adjustment
            DECLARE @TotalFinanceCharge1 DECIMAL(18,4)
            DECLARE @YearsTerm1 DECIMAL(10,4)
            SET @TotalFinanceCharge1 = @TotalInterest1 + @OriginationFee1
            SET @YearsTerm1 = CAST(@TermMonths1 AS DECIMAL(10,4)) / 12.0
            SET @TrueAPR1 = (2.0 * @TotalFinanceCharge1 * 12.0) / (@Principal1 * (@TermMonths1 + 1.0))
        END
        ELSE
            SET @TrueAPR1 = @AnnualRate1
    END
    
    -- Calculate Loan 2
    IF @Principal2 <= 0 OR @TermMonths2 <= 0
    BEGIN
        SET @Payment2 = 0
        SET @TotalCost2 = @OriginationFee2
        SET @TotalInterest2 = 0
        SET @TrueAPR2 = 0
    END
    ELSE
    BEGIN
        SET @MonthlyRate = @AnnualRate2 / 12.0
        
        IF @MonthlyRate = 0
        BEGIN
            SET @Payment2 = @Principal2 / @TermMonths2
        END
        ELSE
        BEGIN
            SET @Multiplier = 1.0
            SET @i = 0
            WHILE @i < @TermMonths2
            BEGIN
                SET @Multiplier = @Multiplier * (1.0 + @MonthlyRate)
                SET @i = @i + 1
            END
            
            SET @Numerator = @MonthlyRate * @Multiplier
            SET @Denominator = @Multiplier - 1.0
            
            IF @Denominator = 0
                SET @Payment2 = @Principal2 / @TermMonths2
            ELSE
                SET @Payment2 = @Principal2 * (@Numerator / @Denominator)
        END
        
        SET @TotalInterest2 = (@Payment2 * @TermMonths2) - @Principal2
        SET @TotalCost2 = (@Payment2 * @TermMonths2) + @OriginationFee2
        
        -- True APR for Loan 2
        DECLARE @NetProceeds2 DECIMAL(18,4)
        SET @NetProceeds2 = @Principal2 - @OriginationFee2
        
        IF @NetProceeds2 > 0 AND @MonthlyRate > 0
        BEGIN
            DECLARE @TotalFinanceCharge2 DECIMAL(18,4)
            SET @TotalFinanceCharge2 = @TotalInterest2 + @OriginationFee2
            SET @TrueAPR2 = (2.0 * @TotalFinanceCharge2 * 12.0) / (@Principal2 * (@TermMonths2 + 1.0))
        END
        ELSE
            SET @TrueAPR2 = @AnnualRate2
    END
    
    -- Compare loans
    IF @TotalCost1 <= @TotalCost2
    BEGIN
        SET @BetterLoan = 1
        SET @Savings = @TotalCost2 - @TotalCost1
    END
    ELSE
    BEGIN
        SET @BetterLoan = 2
        SET @Savings = @TotalCost1 - @TotalCost2
    END
    
    RETURN
END
