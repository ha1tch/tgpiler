-- Currency Conversion with Bid/Ask Spread
-- Converts currency amounts with proper spread handling
-- Handles buy (use ask) vs sell (use bid) scenarios
-- Calculates effective rate and spread cost

CREATE PROCEDURE dbo.CurrencyConvert
    @Amount DECIMAL(18,4),
    @BidRate DECIMAL(18,6),      -- Rate to sell base currency
    @AskRate DECIMAL(18,6),      -- Rate to buy base currency
    @IsBuying BIT,               -- 1 = buying foreign currency, 0 = selling
    @ConvertedAmount DECIMAL(18,4) OUTPUT,
    @EffectiveRate DECIMAL(18,6) OUTPUT,
    @SpreadCost DECIMAL(18,4) OUTPUT,
    @SpreadPercent DECIMAL(10,4) OUTPUT
AS
BEGIN
    SET NOCOUNT ON
    
    DECLARE @MidRate DECIMAL(18,6)
    DECLARE @MidAmount DECIMAL(18,4)
    
    -- Validate inputs
    IF @Amount <= 0
    BEGIN
        SET @ConvertedAmount = 0
        SET @EffectiveRate = 0
        SET @SpreadCost = 0
        SET @SpreadPercent = 0
        RETURN
    END
    
    -- Validate rates
    IF @BidRate <= 0 OR @AskRate <= 0
    BEGIN
        SET @ConvertedAmount = 0
        SET @EffectiveRate = 0
        SET @SpreadCost = 0
        SET @SpreadPercent = 0
        RETURN
    END
    
    -- Ensure ask >= bid (ask is always higher)
    IF @AskRate < @BidRate
    BEGIN
        -- Swap if reversed
        DECLARE @Temp DECIMAL(18,6)
        SET @Temp = @BidRate
        SET @BidRate = @AskRate
        SET @AskRate = @Temp
    END
    
    -- Calculate mid rate for spread calculation
    SET @MidRate = (@BidRate + @AskRate) / 2.0
    
    -- When buying foreign currency, you pay the ask rate (higher)
    -- When selling foreign currency, you receive the bid rate (lower)
    IF @IsBuying = 1
    BEGIN
        SET @EffectiveRate = @AskRate
        SET @ConvertedAmount = @Amount * @AskRate
        SET @MidAmount = @Amount * @MidRate
        SET @SpreadCost = @ConvertedAmount - @MidAmount
    END
    ELSE
    BEGIN
        SET @EffectiveRate = @BidRate
        SET @ConvertedAmount = @Amount * @BidRate
        SET @MidAmount = @Amount * @MidRate
        SET @SpreadCost = @MidAmount - @ConvertedAmount
    END
    
    -- Calculate spread percentage
    IF @MidRate > 0
        SET @SpreadPercent = ((@AskRate - @BidRate) / @MidRate) * 100.0
    ELSE
        SET @SpreadPercent = 0
    
    RETURN
END
