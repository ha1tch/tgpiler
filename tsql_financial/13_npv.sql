-- Net Present Value (NPV) Calculator
-- Calculates NPV of a series of cash flows at a given discount rate
-- NPV = Σ [CFt / (1 + r)^t] for t = 0 to n
-- Supports up to 12 cash flows with profitability index

CREATE PROCEDURE dbo.NetPresentValue
    @DiscountRate DECIMAL(10,6),
    @CashFlow0 DECIMAL(18,4),  -- Initial investment (typically negative)
    @CashFlow1 DECIMAL(18,4),
    @CashFlow2 DECIMAL(18,4),
    @CashFlow3 DECIMAL(18,4),
    @CashFlow4 DECIMAL(18,4),
    @CashFlow5 DECIMAL(18,4),
    @CashFlow6 DECIMAL(18,4),
    @CashFlow7 DECIMAL(18,4),
    @CashFlow8 DECIMAL(18,4),
    @CashFlow9 DECIMAL(18,4),
    @CashFlow10 DECIMAL(18,4),
    @CashFlow11 DECIMAL(18,4),
    @NumPeriods INT,
    @NPV DECIMAL(18,4) OUTPUT,
    @ProfitabilityIndex DECIMAL(10,4) OUTPUT,
    @PVofInflows DECIMAL(18,4) OUTPUT,
    @PVofOutflows DECIMAL(18,4) OUTPUT,
    @IsAcceptable BIT OUTPUT  -- NPV > 0
AS
BEGIN
    SET NOCOUNT ON
    
    DECLARE @TotalNPV DECIMAL(18,10)
    DECLARE @Inflows DECIMAL(18,10)
    DECLARE @Outflows DECIMAL(18,10)
    DECLARE @DiscountFactor DECIMAL(18,10)
    DECLARE @CashFlow DECIMAL(18,4)
    DECLARE @PVCashFlow DECIMAL(18,10)
    DECLARE @Period INT
    DECLARE @i INT
    
    SET @TotalNPV = 0
    SET @Inflows = 0
    SET @Outflows = 0
    
    -- Validate number of periods
    IF @NumPeriods < 0
        SET @NumPeriods = 0
    IF @NumPeriods > 12
        SET @NumPeriods = 12
    
    -- Calculate NPV for each period
    SET @Period = 0
    
    WHILE @Period <= @NumPeriods
    BEGIN
        -- Get cash flow for this period
        IF @Period = 0
            SET @CashFlow = @CashFlow0
        ELSE IF @Period = 1
            SET @CashFlow = @CashFlow1
        ELSE IF @Period = 2
            SET @CashFlow = @CashFlow2
        ELSE IF @Period = 3
            SET @CashFlow = @CashFlow3
        ELSE IF @Period = 4
            SET @CashFlow = @CashFlow4
        ELSE IF @Period = 5
            SET @CashFlow = @CashFlow5
        ELSE IF @Period = 6
            SET @CashFlow = @CashFlow6
        ELSE IF @Period = 7
            SET @CashFlow = @CashFlow7
        ELSE IF @Period = 8
            SET @CashFlow = @CashFlow8
        ELSE IF @Period = 9
            SET @CashFlow = @CashFlow9
        ELSE IF @Period = 10
            SET @CashFlow = @CashFlow10
        ELSE
            SET @CashFlow = @CashFlow11
        
        -- Calculate discount factor: 1 / (1 + r)^period
        SET @DiscountFactor = 1.0
        SET @i = 0
        
        WHILE @i < @Period
        BEGIN
            SET @DiscountFactor = @DiscountFactor / (1.0 + @DiscountRate)
            SET @i = @i + 1
        END
        
        -- Calculate present value of this cash flow
        SET @PVCashFlow = @CashFlow * @DiscountFactor
        
        -- Add to NPV total
        SET @TotalNPV = @TotalNPV + @PVCashFlow
        
        -- Track inflows and outflows separately
        IF @CashFlow > 0
            SET @Inflows = @Inflows + @PVCashFlow
        ELSE
            SET @Outflows = @Outflows + ABS(@PVCashFlow)
        
        SET @Period = @Period + 1
    END
    
    -- Set outputs
    SET @NPV = @TotalNPV
    SET @PVofInflows = @Inflows
    SET @PVofOutflows = @Outflows
    
    -- Calculate profitability index: PV of inflows / PV of outflows
    IF @Outflows > 0
        SET @ProfitabilityIndex = @Inflows / @Outflows
    ELSE IF @Inflows > 0
        SET @ProfitabilityIndex = 999.99  -- Infinite (all positive cash flows)
    ELSE
        SET @ProfitabilityIndex = 0
    
    -- Determine if investment is acceptable (NPV > 0)
    IF @TotalNPV > 0
        SET @IsAcceptable = 1
    ELSE
        SET @IsAcceptable = 0
    
    RETURN
END
