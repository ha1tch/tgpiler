-- Internal Rate of Return (IRR) Calculator
-- Uses Newton-Raphson iteration to find the rate where NPV = 0
-- Supports up to 10 cash flows (initial investment + 9 periods)
-- IRR is the discount rate that makes NPV of all cash flows equal zero

CREATE PROCEDURE dbo.InternalRateOfReturn
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
    @NumPeriods INT,
    @IRR DECIMAL(18,10) OUTPUT,
    @NPVatIRR DECIMAL(18,4) OUTPUT,
    @Iterations INT OUTPUT,
    @Converged BIT OUTPUT
AS
BEGIN
    SET NOCOUNT ON
    
    DECLARE @Rate DECIMAL(18,10)
    DECLARE @NewRate DECIMAL(18,10)
    DECLARE @NPV DECIMAL(18,10)
    DECLARE @dNPV DECIMAL(18,10)  -- Derivative of NPV
    DECLARE @Tolerance DECIMAL(18,10)
    DECLARE @MaxIterations INT
    DECLARE @DiscountFactor DECIMAL(18,10)
    DECLARE @i INT
    DECLARE @Period INT
    DECLARE @CashFlow DECIMAL(18,4)
    
    SET @Tolerance = 0.0000001
    SET @MaxIterations = 100
    SET @Converged = 0
    SET @Iterations = 0
    
    -- Validate number of periods
    IF @NumPeriods < 1
        SET @NumPeriods = 1
    IF @NumPeriods > 10
        SET @NumPeriods = 10
    
    -- Start with initial guess (10%)
    SET @Rate = 0.10
    
    -- Newton-Raphson iteration
    SET @i = 0
    
    WHILE @i < @MaxIterations
    BEGIN
        -- Calculate NPV and its derivative at current rate
        SET @NPV = 0
        SET @dNPV = 0
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
            ELSE
                SET @CashFlow = @CashFlow9
            
            -- Calculate discount factor: 1 / (1 + r)^period
            SET @DiscountFactor = 1.0
            DECLARE @j INT
            SET @j = 0
            WHILE @j < @Period
            BEGIN
                SET @DiscountFactor = @DiscountFactor / (1.0 + @Rate)
                SET @j = @j + 1
            END
            
            -- NPV += CashFlow / (1 + r)^period
            SET @NPV = @NPV + (@CashFlow * @DiscountFactor)
            
            -- Derivative: d/dr[CF/(1+r)^t] = -t * CF / (1+r)^(t+1)
            IF @Period > 0
            BEGIN
                SET @dNPV = @dNPV - (@Period * @CashFlow * @DiscountFactor / (1.0 + @Rate))
            END
            
            SET @Period = @Period + 1
        END
        
        -- Check for convergence
        IF ABS(@NPV) < @Tolerance
        BEGIN
            SET @Converged = 1
            SET @Iterations = @i + 1
            SET @IRR = @Rate
            SET @NPVatIRR = @NPV
            RETURN
        END
        
        -- Avoid division by zero in Newton step
        IF ABS(@dNPV) < 0.0000001
        BEGIN
            SET @dNPV = 0.0000001
        END
        
        -- Newton-Raphson update: r_new = r - f(r) / f'(r)
        SET @NewRate = @Rate - (@NPV / @dNPV)
        
        -- Prevent rate from going too negative or too large
        IF @NewRate < -0.99
            SET @NewRate = -0.99
        IF @NewRate > 10.0
            SET @NewRate = 10.0
        
        SET @Rate = @NewRate
        SET @i = @i + 1
    END
    
    -- Did not converge within max iterations
    SET @Converged = 0
    SET @Iterations = @MaxIterations
    SET @IRR = @Rate
    SET @NPVatIRR = @NPV
    
    RETURN
END
