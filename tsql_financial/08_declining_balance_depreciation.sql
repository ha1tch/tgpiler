-- Declining Balance Depreciation
-- Calculates depreciation using double-declining balance method
-- Rate = (1/Useful Life) * Multiplier (typically 2 for double-declining)
-- Annual Depreciation = Book Value at Beginning * Rate

CREATE PROCEDURE dbo.DecliningBalanceDepreciation
    @AssetCost DECIMAL(18,4),
    @SalvageValue DECIMAL(18,4),
    @UsefulLifeYears INT,
    @CurrentYear INT,
    @Multiplier DECIMAL(10,4),  -- 2.0 for double-declining, 1.5 for 150% declining
    @AnnualDepreciation DECIMAL(18,4) OUTPUT,
    @AccumulatedDepreciation DECIMAL(18,4) OUTPUT,
    @BookValue DECIMAL(18,4) OUTPUT,
    @DepreciationRate DECIMAL(10,6) OUTPUT
AS
BEGIN
    SET NOCOUNT ON
    
    DECLARE @Rate DECIMAL(18,10)
    DECLARE @BeginningBookValue DECIMAL(18,4)
    DECLARE @YearDepreciation DECIMAL(18,4)
    DECLARE @TotalDepreciation DECIMAL(18,4)
    DECLARE @i INT
    
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
    
    IF @Multiplier <= 0
        SET @Multiplier = 2.0  -- Default to double-declining
    
    -- Calculate depreciation rate
    SET @Rate = @Multiplier / @UsefulLifeYears
    SET @DepreciationRate = @Rate
    
    -- Cap rate at 1 (100%)
    IF @Rate > 1.0
        SET @Rate = 1.0
    
    -- Calculate depreciation year by year up to current year
    SET @BeginningBookValue = @AssetCost
    SET @TotalDepreciation = 0
    SET @AnnualDepreciation = 0
    SET @i = 1
    
    WHILE @i <= @CurrentYear AND @BeginningBookValue > @SalvageValue
    BEGIN
        -- Calculate depreciation for this year
        SET @YearDepreciation = @BeginningBookValue * @Rate
        
        -- Don't depreciate below salvage value
        IF (@BeginningBookValue - @YearDepreciation) < @SalvageValue
        BEGIN
            SET @YearDepreciation = @BeginningBookValue - @SalvageValue
        END
        
        -- If this is the current year, save the depreciation amount
        IF @i = @CurrentYear
            SET @AnnualDepreciation = @YearDepreciation
        
        SET @TotalDepreciation = @TotalDepreciation + @YearDepreciation
        SET @BeginningBookValue = @BeginningBookValue - @YearDepreciation
        SET @i = @i + 1
    END
    
    -- Set outputs
    SET @AccumulatedDepreciation = @TotalDepreciation
    SET @BookValue = @AssetCost - @TotalDepreciation
    
    -- If past useful life or at salvage value, no more depreciation
    IF @CurrentYear > @UsefulLifeYears OR @BookValue <= @SalvageValue
    BEGIN
        SET @AnnualDepreciation = 0
        SET @BookValue = @SalvageValue
        SET @AccumulatedDepreciation = @AssetCost - @SalvageValue
    END
    
    RETURN
END
