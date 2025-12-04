-- Break-Even Analysis
-- Calculates break-even point in units and revenue
-- Break-even units = Fixed Costs / (Price per Unit - Variable Cost per Unit)
-- Also calculates margin of safety and contribution margin

CREATE PROCEDURE dbo.BreakEvenAnalysis
    @FixedCosts DECIMAL(18,4),
    @VariableCostPerUnit DECIMAL(18,4),
    @SellingPricePerUnit DECIMAL(18,4),
    @ActualSalesUnits INT,
    @BreakEvenUnits INT OUTPUT,
    @BreakEvenRevenue DECIMAL(18,4) OUTPUT,
    @ContributionMarginPerUnit DECIMAL(18,4) OUTPUT,
    @ContributionMarginRatio DECIMAL(10,6) OUTPUT,
    @MarginOfSafetyUnits INT OUTPUT,
    @MarginOfSafetyPercent DECIMAL(10,4) OUTPUT,
    @ProfitAtActualSales DECIMAL(18,4) OUTPUT
AS
BEGIN
    SET NOCOUNT ON
    
    DECLARE @ContributionMargin DECIMAL(18,4)
    DECLARE @BreakEvenExact DECIMAL(18,4)
    
    -- Validate inputs
    IF @FixedCosts < 0
        SET @FixedCosts = 0
    
    IF @VariableCostPerUnit < 0
        SET @VariableCostPerUnit = 0
    
    IF @SellingPricePerUnit <= 0
    BEGIN
        SET @BreakEvenUnits = 0
        SET @BreakEvenRevenue = 0
        SET @ContributionMarginPerUnit = 0
        SET @ContributionMarginRatio = 0
        SET @MarginOfSafetyUnits = 0
        SET @MarginOfSafetyPercent = 0
        SET @ProfitAtActualSales = -@FixedCosts
        RETURN
    END
    
    IF @ActualSalesUnits < 0
        SET @ActualSalesUnits = 0
    
    -- Calculate contribution margin per unit
    SET @ContributionMargin = @SellingPricePerUnit - @VariableCostPerUnit
    SET @ContributionMarginPerUnit = @ContributionMargin
    
    -- Calculate contribution margin ratio
    SET @ContributionMarginRatio = @ContributionMargin / @SellingPricePerUnit
    
    -- Check if contribution margin is positive
    IF @ContributionMargin <= 0
    BEGIN
        -- Cannot break even if variable cost >= price
        SET @BreakEvenUnits = 0
        SET @BreakEvenRevenue = 0
        SET @MarginOfSafetyUnits = 0
        SET @MarginOfSafetyPercent = 0
        
        -- Calculate loss at actual sales
        SET @ProfitAtActualSales = (@ContributionMargin * @ActualSalesUnits) - @FixedCosts
        RETURN
    END
    
    -- Calculate break-even point in units (round up)
    SET @BreakEvenExact = @FixedCosts / @ContributionMargin
    SET @BreakEvenUnits = CAST(@BreakEvenExact AS INT)
    
    -- Round up if there's a fractional part
    IF @BreakEvenExact > @BreakEvenUnits
        SET @BreakEvenUnits = @BreakEvenUnits + 1
    
    -- Calculate break-even revenue
    SET @BreakEvenRevenue = CAST(@BreakEvenUnits AS DECIMAL(18,4)) * @SellingPricePerUnit
    
    -- Calculate margin of safety
    SET @MarginOfSafetyUnits = @ActualSalesUnits - @BreakEvenUnits
    
    IF @ActualSalesUnits > 0
        SET @MarginOfSafetyPercent = (CAST(@MarginOfSafetyUnits AS DECIMAL(18,4)) / @ActualSalesUnits) * 100.0
    ELSE
        SET @MarginOfSafetyPercent = 0
    
    -- Calculate profit at actual sales level
    SET @ProfitAtActualSales = (@ContributionMargin * @ActualSalesUnits) - @FixedCosts
    
    RETURN
END
