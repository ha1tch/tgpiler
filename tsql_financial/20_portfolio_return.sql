-- Portfolio Weighted Return Calculator
-- Calculates weighted average return of a multi-asset portfolio
-- Supports up to 8 assets with weights and returns
-- Includes variance and standard deviation estimation

CREATE PROCEDURE dbo.PortfolioWeightedReturn
    -- Asset weights (should sum to 1.0 or 100)
    @Weight1 DECIMAL(10,6),
    @Weight2 DECIMAL(10,6),
    @Weight3 DECIMAL(10,6),
    @Weight4 DECIMAL(10,6),
    @Weight5 DECIMAL(10,6),
    @Weight6 DECIMAL(10,6),
    @Weight7 DECIMAL(10,6),
    @Weight8 DECIMAL(10,6),
    -- Asset returns (as decimal, e.g., 0.10 for 10%)
    @AssetRet1 DECIMAL(10,6),
    @AssetRet2 DECIMAL(10,6),
    @AssetRet3 DECIMAL(10,6),
    @AssetRet4 DECIMAL(10,6),
    @AssetRet5 DECIMAL(10,6),
    @AssetRet6 DECIMAL(10,6),
    @AssetRet7 DECIMAL(10,6),
    @AssetRet8 DECIMAL(10,6),
    -- Asset risk (standard deviation)
    @StdDev1 DECIMAL(10,6),
    @StdDev2 DECIMAL(10,6),
    @StdDev3 DECIMAL(10,6),
    @StdDev4 DECIMAL(10,6),
    @StdDev5 DECIMAL(10,6),
    @StdDev6 DECIMAL(10,6),
    @StdDev7 DECIMAL(10,6),
    @StdDev8 DECIMAL(10,6),
    @NumAssets INT,
    -- Outputs
    @PortfolioReturn DECIMAL(10,6) OUTPUT,
    @PortfolioVariance DECIMAL(18,10) OUTPUT,
    @PortfolioStdDev DECIMAL(10,6) OUTPUT,
    @SharpeRatio DECIMAL(10,4) OUTPUT,
    @RiskFreeRate DECIMAL(10,6),
    @WeightsValid BIT OUTPUT
AS
BEGIN
    SET NOCOUNT ON
    
    DECLARE @TotalWeight DECIMAL(18,10)
    DECLARE @WeightedReturn DECIMAL(18,10)
    DECLARE @WeightedVariance DECIMAL(18,10)
    DECLARE @i INT
    DECLARE @Weight DECIMAL(10,6)
    DECLARE @AssetRet DECIMAL(10,6)
    DECLARE @StdDev DECIMAL(10,6)
    
    SET @TotalWeight = 0
    SET @WeightedReturn = 0
    SET @WeightedVariance = 0
    SET @WeightsValid = 1
    
    -- Validate number of assets
    IF @NumAssets < 1
        SET @NumAssets = 1
    IF @NumAssets > 8
        SET @NumAssets = 8
    
    -- Calculate total weight and weighted return
    SET @i = 1
    WHILE @i <= @NumAssets
    BEGIN
        -- Get weight and return for this asset
        IF @i = 1
        BEGIN
            SET @Weight = @Weight1
            SET @AssetRet = @AssetRet1
            SET @StdDev = @StdDev1
        END
        ELSE IF @i = 2
        BEGIN
            SET @Weight = @Weight2
            SET @AssetRet = @AssetRet2
            SET @StdDev = @StdDev2
        END
        ELSE IF @i = 3
        BEGIN
            SET @Weight = @Weight3
            SET @AssetRet = @AssetRet3
            SET @StdDev = @StdDev3
        END
        ELSE IF @i = 4
        BEGIN
            SET @Weight = @Weight4
            SET @AssetRet = @AssetRet4
            SET @StdDev = @StdDev4
        END
        ELSE IF @i = 5
        BEGIN
            SET @Weight = @Weight5
            SET @AssetRet = @AssetRet5
            SET @StdDev = @StdDev5
        END
        ELSE IF @i = 6
        BEGIN
            SET @Weight = @Weight6
            SET @AssetRet = @AssetRet6
            SET @StdDev = @StdDev6
        END
        ELSE IF @i = 7
        BEGIN
            SET @Weight = @Weight7
            SET @AssetRet = @AssetRet7
            SET @StdDev = @StdDev7
        END
        ELSE
        BEGIN
            SET @Weight = @Weight8
            SET @AssetRet = @AssetRet8
            SET @StdDev = @StdDev8
        END
        
        -- Accumulate
        SET @TotalWeight = @TotalWeight + @Weight
        SET @WeightedReturn = @WeightedReturn + (@Weight * @AssetRet)
        
        -- For variance: assuming uncorrelated assets (simplified)
        -- Portfolio Variance = Σ (wi² * σi²)
        SET @WeightedVariance = @WeightedVariance + (@Weight * @Weight * @StdDev * @StdDev)
        
        SET @i = @i + 1
    END
    
    -- Validate weights sum to approximately 1 (or 100%)
    IF @TotalWeight > 1.5  -- Weights given as percentages
    BEGIN
        -- Normalize
        IF @TotalWeight > 0
        BEGIN
            SET @WeightedReturn = @WeightedReturn / @TotalWeight
            SET @WeightedVariance = @WeightedVariance / (@TotalWeight * @TotalWeight)
        END
        
        IF ABS(@TotalWeight - 100.0) > 1.0
            SET @WeightsValid = 0
    END
    ELSE
    BEGIN
        IF ABS(@TotalWeight - 1.0) > 0.01
            SET @WeightsValid = 0
    END
    
    -- Set portfolio return
    SET @PortfolioReturn = @WeightedReturn
    
    -- Set portfolio variance
    SET @PortfolioVariance = @WeightedVariance
    
    -- Calculate portfolio standard deviation (sqrt of variance)
    -- Using Newton-Raphson for square root
    IF @WeightedVariance > 0
    BEGIN
        DECLARE @Guess DECIMAL(18,10)
        DECLARE @NewGuess DECIMAL(18,10)
        DECLARE @j INT
        
        SET @Guess = @WeightedVariance  -- Initial guess
        IF @Guess > 1
            SET @Guess = 1
        
        SET @j = 0
        WHILE @j < 20
        BEGIN
            IF @Guess > 0
                SET @NewGuess = (@Guess + @WeightedVariance / @Guess) / 2.0
            ELSE
                SET @NewGuess = 0.0001
            
            IF ABS(@NewGuess - @Guess) < 0.0000001
                SET @j = 20
            
            SET @Guess = @NewGuess
            SET @j = @j + 1
        END
        
        SET @PortfolioStdDev = @Guess
    END
    ELSE
    BEGIN
        SET @PortfolioStdDev = 0
    END
    
    -- Calculate Sharpe Ratio: (Return - RiskFreeRate) / StdDev
    IF @PortfolioStdDev > 0
        SET @SharpeRatio = (@PortfolioReturn - @RiskFreeRate) / @PortfolioStdDev
    ELSE IF @PortfolioReturn > @RiskFreeRate
        SET @SharpeRatio = 99.99  -- Infinite (return with no risk)
    ELSE
        SET @SharpeRatio = 0
    
    RETURN
END
