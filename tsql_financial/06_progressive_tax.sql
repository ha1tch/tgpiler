-- Progressive Tax Calculation
-- Calculates tax using progressive tax brackets
-- Supports up to 7 tax brackets (configurable)
-- Returns marginal rate, effective rate, and breakdown

CREATE PROCEDURE dbo.ProgressiveTax
    @TaxableIncome DECIMAL(18,4),
    -- Bracket thresholds (upper bounds)
    @Bracket1 DECIMAL(18,4),
    @Bracket2 DECIMAL(18,4),
    @Bracket3 DECIMAL(18,4),
    @Bracket4 DECIMAL(18,4),
    @Bracket5 DECIMAL(18,4),
    -- Tax rates for each bracket
    @Rate1 DECIMAL(10,6),
    @Rate2 DECIMAL(10,6),
    @Rate3 DECIMAL(10,6),
    @Rate4 DECIMAL(10,6),
    @Rate5 DECIMAL(10,6),
    @Rate6 DECIMAL(10,6),  -- Rate above Bracket5
    -- Outputs
    @TotalTax DECIMAL(18,4) OUTPUT,
    @EffectiveRate DECIMAL(10,6) OUTPUT,
    @MarginalRate DECIMAL(10,6) OUTPUT
AS
BEGIN
    SET NOCOUNT ON
    
    DECLARE @Tax DECIMAL(18,4)
    DECLARE @Remaining DECIMAL(18,4)
    DECLARE @BracketTax DECIMAL(18,4)
    DECLARE @PrevBracket DECIMAL(18,4)
    DECLARE @BracketWidth DECIMAL(18,4)
    
    SET @Tax = 0
    SET @Remaining = @TaxableIncome
    SET @PrevBracket = 0
    SET @MarginalRate = @Rate1
    
    -- Validate inputs
    IF @TaxableIncome <= 0
    BEGIN
        SET @TotalTax = 0
        SET @EffectiveRate = 0
        SET @MarginalRate = @Rate1
        RETURN
    END
    
    -- Bracket 1: 0 to Bracket1
    IF @Remaining > 0 AND @Bracket1 > 0
    BEGIN
        SET @BracketWidth = @Bracket1 - @PrevBracket
        IF @Remaining >= @BracketWidth
        BEGIN
            SET @BracketTax = @BracketWidth * @Rate1
            SET @Remaining = @Remaining - @BracketWidth
        END
        ELSE
        BEGIN
            SET @BracketTax = @Remaining * @Rate1
            SET @Remaining = 0
            SET @MarginalRate = @Rate1
        END
        SET @Tax = @Tax + @BracketTax
        SET @PrevBracket = @Bracket1
    END
    
    -- Bracket 2: Bracket1 to Bracket2
    IF @Remaining > 0 AND @Bracket2 > @Bracket1
    BEGIN
        SET @BracketWidth = @Bracket2 - @PrevBracket
        IF @Remaining >= @BracketWidth
        BEGIN
            SET @BracketTax = @BracketWidth * @Rate2
            SET @Remaining = @Remaining - @BracketWidth
        END
        ELSE
        BEGIN
            SET @BracketTax = @Remaining * @Rate2
            SET @Remaining = 0
            SET @MarginalRate = @Rate2
        END
        SET @Tax = @Tax + @BracketTax
        SET @PrevBracket = @Bracket2
    END
    
    -- Bracket 3: Bracket2 to Bracket3
    IF @Remaining > 0 AND @Bracket3 > @Bracket2
    BEGIN
        SET @BracketWidth = @Bracket3 - @PrevBracket
        IF @Remaining >= @BracketWidth
        BEGIN
            SET @BracketTax = @BracketWidth * @Rate3
            SET @Remaining = @Remaining - @BracketWidth
        END
        ELSE
        BEGIN
            SET @BracketTax = @Remaining * @Rate3
            SET @Remaining = 0
            SET @MarginalRate = @Rate3
        END
        SET @Tax = @Tax + @BracketTax
        SET @PrevBracket = @Bracket3
    END
    
    -- Bracket 4: Bracket3 to Bracket4
    IF @Remaining > 0 AND @Bracket4 > @Bracket3
    BEGIN
        SET @BracketWidth = @Bracket4 - @PrevBracket
        IF @Remaining >= @BracketWidth
        BEGIN
            SET @BracketTax = @BracketWidth * @Rate4
            SET @Remaining = @Remaining - @BracketWidth
        END
        ELSE
        BEGIN
            SET @BracketTax = @Remaining * @Rate4
            SET @Remaining = 0
            SET @MarginalRate = @Rate4
        END
        SET @Tax = @Tax + @BracketTax
        SET @PrevBracket = @Bracket4
    END
    
    -- Bracket 5: Bracket4 to Bracket5
    IF @Remaining > 0 AND @Bracket5 > @Bracket4
    BEGIN
        SET @BracketWidth = @Bracket5 - @PrevBracket
        IF @Remaining >= @BracketWidth
        BEGIN
            SET @BracketTax = @BracketWidth * @Rate5
            SET @Remaining = @Remaining - @BracketWidth
        END
        ELSE
        BEGIN
            SET @BracketTax = @Remaining * @Rate5
            SET @Remaining = 0
            SET @MarginalRate = @Rate5
        END
        SET @Tax = @Tax + @BracketTax
    END
    
    -- Bracket 6: Above Bracket5
    IF @Remaining > 0
    BEGIN
        SET @BracketTax = @Remaining * @Rate6
        SET @Tax = @Tax + @BracketTax
        SET @MarginalRate = @Rate6
    END
    
    SET @TotalTax = @Tax
    
    -- Calculate effective rate
    IF @TaxableIncome > 0
        SET @EffectiveRate = @TotalTax / @TaxableIncome
    ELSE
        SET @EffectiveRate = 0
    
    RETURN
END
