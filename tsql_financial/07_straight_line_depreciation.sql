-- Straight-Line Depreciation
-- Calculates depreciation using the straight-line method
-- Depreciation = (Cost - Salvage) / Useful Life
-- Returns annual depreciation and accumulated depreciation for any year

CREATE PROCEDURE dbo.StraightLineDepreciation
    @AssetCost DECIMAL(18,4),
    @SalvageValue DECIMAL(18,4),
    @UsefulLifeYears INT,
    @CurrentYear INT,
    @AnnualDepreciation DECIMAL(18,4) OUTPUT,
    @AccumulatedDepreciation DECIMAL(18,4) OUTPUT,
    @BookValue DECIMAL(18,4) OUTPUT,
    @DepreciationRate DECIMAL(10,6) OUTPUT
AS
BEGIN
    SET NOCOUNT ON
    
    DECLARE @DepreciableBase DECIMAL(18,4)
    
    -- Validate inputs
    IF @AssetCost <= 0
    BEGIN
        SET @AnnualDepreciation = 0
        SET @AccumulatedDepreciation = 0
        SET @BookValue = 0
        SET @DepreciationRate = 0
        RETURN
    END
    
    IF @UsefulLifeYears <= 0
        SET @UsefulLifeYears = 1
    
    IF @SalvageValue < 0
        SET @SalvageValue = 0
    
    IF @SalvageValue > @AssetCost
        SET @SalvageValue = @AssetCost
    
    IF @CurrentYear < 0
        SET @CurrentYear = 0
    
    -- Calculate depreciable base
    SET @DepreciableBase = @AssetCost - @SalvageValue
    
    -- Calculate annual depreciation
    SET @AnnualDepreciation = @DepreciableBase / @UsefulLifeYears
    
    -- Calculate depreciation rate
    IF @AssetCost > 0
        SET @DepreciationRate = @AnnualDepreciation / @AssetCost
    ELSE
        SET @DepreciationRate = 0
    
    -- Calculate accumulated depreciation for current year
    IF @CurrentYear >= @UsefulLifeYears
    BEGIN
        -- Fully depreciated
        SET @AccumulatedDepreciation = @DepreciableBase
        SET @BookValue = @SalvageValue
        SET @AnnualDepreciation = 0  -- No more depreciation after useful life
    END
    ELSE IF @CurrentYear > 0
    BEGIN
        SET @AccumulatedDepreciation = @AnnualDepreciation * @CurrentYear
        SET @BookValue = @AssetCost - @AccumulatedDepreciation
    END
    ELSE
    BEGIN
        -- Year 0: no depreciation yet
        SET @AccumulatedDepreciation = 0
        SET @BookValue = @AssetCost
    END
    
    RETURN
END
