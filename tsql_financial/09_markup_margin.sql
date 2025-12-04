-- Markup and Margin Calculation
-- Converts between markup and margin percentages
-- Markup = (Selling Price - Cost) / Cost
-- Margin = (Selling Price - Cost) / Selling Price
-- Also calculates selling price from cost and desired margin/markup

CREATE PROCEDURE dbo.MarkupMargin
    @Cost DECIMAL(18,4),
    @SellingPrice DECIMAL(18,4),
    @DesiredMarkupPercent DECIMAL(10,4),
    @DesiredMarginPercent DECIMAL(10,4),
    @CalculationMode INT,  -- 1=Calculate from Cost/Price, 2=From Markup, 3=From Margin
    @ActualMarkupPercent DECIMAL(10,4) OUTPUT,
    @ActualMarginPercent DECIMAL(10,4) OUTPUT,
    @CalculatedSellingPrice DECIMAL(18,4) OUTPUT,
    @GrossProfit DECIMAL(18,4) OUTPUT
AS
BEGIN
    SET NOCOUNT ON
    
    DECLARE @Profit DECIMAL(18,4)
    
    -- Validate inputs
    IF @Cost < 0
        SET @Cost = 0
    
    IF @SellingPrice < 0
        SET @SellingPrice = 0
    
    -- Mode 1: Calculate markup and margin from cost and selling price
    IF @CalculationMode = 1
    BEGIN
        IF @Cost <= 0
        BEGIN
            SET @ActualMarkupPercent = 0
            SET @ActualMarginPercent = 0
            SET @CalculatedSellingPrice = @SellingPrice
            SET @GrossProfit = 0
            RETURN
        END
        
        SET @Profit = @SellingPrice - @Cost
        SET @GrossProfit = @Profit
        
        -- Markup = Profit / Cost * 100
        SET @ActualMarkupPercent = (@Profit / @Cost) * 100.0
        
        -- Margin = Profit / Selling Price * 100
        IF @SellingPrice > 0
            SET @ActualMarginPercent = (@Profit / @SellingPrice) * 100.0
        ELSE
            SET @ActualMarginPercent = 0
        
        SET @CalculatedSellingPrice = @SellingPrice
    END
    
    -- Mode 2: Calculate selling price from cost and desired markup
    ELSE IF @CalculationMode = 2
    BEGIN
        IF @Cost <= 0
        BEGIN
            SET @ActualMarkupPercent = 0
            SET @ActualMarginPercent = 0
            SET @CalculatedSellingPrice = 0
            SET @GrossProfit = 0
            RETURN
        END
        
        -- Selling Price = Cost * (1 + Markup/100)
        SET @CalculatedSellingPrice = @Cost * (1.0 + (@DesiredMarkupPercent / 100.0))
        SET @Profit = @CalculatedSellingPrice - @Cost
        SET @GrossProfit = @Profit
        
        SET @ActualMarkupPercent = @DesiredMarkupPercent
        
        -- Calculate resulting margin
        IF @CalculatedSellingPrice > 0
            SET @ActualMarginPercent = (@Profit / @CalculatedSellingPrice) * 100.0
        ELSE
            SET @ActualMarginPercent = 0
    END
    
    -- Mode 3: Calculate selling price from cost and desired margin
    ELSE IF @CalculationMode = 3
    BEGIN
        IF @Cost <= 0
        BEGIN
            SET @ActualMarkupPercent = 0
            SET @ActualMarginPercent = 0
            SET @CalculatedSellingPrice = 0
            SET @GrossProfit = 0
            RETURN
        END
        
        -- Margin cannot be 100% or more
        IF @DesiredMarginPercent >= 100.0
        BEGIN
            SET @DesiredMarginPercent = 99.99
        END
        
        -- Selling Price = Cost / (1 - Margin/100)
        SET @CalculatedSellingPrice = @Cost / (1.0 - (@DesiredMarginPercent / 100.0))
        SET @Profit = @CalculatedSellingPrice - @Cost
        SET @GrossProfit = @Profit
        
        SET @ActualMarginPercent = @DesiredMarginPercent
        
        -- Calculate resulting markup
        IF @Cost > 0
            SET @ActualMarkupPercent = (@Profit / @Cost) * 100.0
        ELSE
            SET @ActualMarkupPercent = 0
    END
    ELSE
    BEGIN
        -- Invalid mode
        SET @ActualMarkupPercent = 0
        SET @ActualMarginPercent = 0
        SET @CalculatedSellingPrice = 0
        SET @GrossProfit = 0
    END
    
    RETURN
END
