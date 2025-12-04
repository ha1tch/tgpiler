-- Compound Annual Growth Rate (CAGR) Calculator
-- CAGR = (EndValue / BeginValue)^(1/Years) - 1
-- Uses Newton-Raphson to compute the nth root
-- Also calculates absolute and relative growth

CREATE PROCEDURE dbo.CompoundAnnualGrowthRate
    @BeginningValue DECIMAL(18,4),
    @EndingValue DECIMAL(18,4),
    @NumYears INT,
    @CAGR DECIMAL(10,6) OUTPUT,
    @TotalGrowth DECIMAL(18,4) OUTPUT,
    @TotalGrowthPercent DECIMAL(10,4) OUTPUT,
    @FutureValue5Years DECIMAL(18,4) OUTPUT,
    @FutureValue10Years DECIMAL(18,4) OUTPUT
AS
BEGIN
    SET NOCOUNT ON
    
    DECLARE @Ratio DECIMAL(18,10)
    DECLARE @Root DECIMAL(18,10)
    DECLARE @Guess DECIMAL(18,10)
    DECLARE @NewGuess DECIMAL(18,10)
    DECLARE @Power DECIMAL(18,10)
    DECLARE @Derivative DECIMAL(18,10)
    DECLARE @i INT
    DECLARE @j INT
    DECLARE @MaxIterations INT
    DECLARE @Tolerance DECIMAL(18,10)
    DECLARE @Multiplier DECIMAL(18,10)
    
    SET @MaxIterations = 50
    SET @Tolerance = 0.0000001
    
    -- Validate inputs
    IF @BeginningValue <= 0 OR @EndingValue <= 0
    BEGIN
        SET @CAGR = 0
        SET @TotalGrowth = 0
        SET @TotalGrowthPercent = 0
        SET @FutureValue5Years = @EndingValue
        SET @FutureValue10Years = @EndingValue
        RETURN
    END
    
    IF @NumYears <= 0
    BEGIN
        SET @CAGR = 0
        SET @TotalGrowth = @EndingValue - @BeginningValue
        SET @TotalGrowthPercent = ((@EndingValue - @BeginningValue) / @BeginningValue) * 100.0
        SET @FutureValue5Years = @EndingValue
        SET @FutureValue10Years = @EndingValue
        RETURN
    END
    
    -- Calculate ratio
    SET @Ratio = @EndingValue / @BeginningValue
    
    -- Calculate total growth
    SET @TotalGrowth = @EndingValue - @BeginningValue
    SET @TotalGrowthPercent = ((@EndingValue - @BeginningValue) / @BeginningValue) * 100.0
    
    -- Special case: 1 year
    IF @NumYears = 1
    BEGIN
        SET @CAGR = @Ratio - 1.0
        -- Calculate future values
        SET @Multiplier = 1.0 + @CAGR
        SET @FutureValue5Years = @EndingValue
        SET @i = 1
        WHILE @i < 5
        BEGIN
            SET @FutureValue5Years = @FutureValue5Years * @Multiplier
            SET @i = @i + 1
        END
        SET @FutureValue10Years = @FutureValue5Years
        SET @i = 5
        WHILE @i < 10
        BEGIN
            SET @FutureValue10Years = @FutureValue10Years * @Multiplier
            SET @i = @i + 1
        END
        RETURN
    END
    
    -- Newton-Raphson to find nth root: x^n = ratio
    -- We want to find x where x^n - ratio = 0
    -- f(x) = x^n - ratio
    -- f'(x) = n * x^(n-1)
    -- x_new = x - f(x)/f'(x) = x - (x^n - ratio) / (n * x^(n-1))
    --       = x - x/n + ratio/(n * x^(n-1))
    --       = x * (1 - 1/n) + ratio / (n * x^(n-1))
    
    -- Initial guess
    SET @Guess = 1.0 + (@TotalGrowthPercent / 100.0 / @NumYears)
    
    SET @i = 0
    WHILE @i < @MaxIterations
    BEGIN
        -- Calculate guess^(n-1) iteratively
        SET @Power = 1.0
        SET @j = 0
        WHILE @j < (@NumYears - 1)
        BEGIN
            SET @Power = @Power * @Guess
            SET @j = @j + 1
        END
        
        -- guess^n = Power * guess
        DECLARE @GuessToN DECIMAL(18,10)
        SET @GuessToN = @Power * @Guess
        
        -- f(x) = x^n - ratio
        DECLARE @FofX DECIMAL(18,10)
        SET @FofX = @GuessToN - @Ratio
        
        -- f'(x) = n * x^(n-1) = n * Power
        SET @Derivative = CAST(@NumYears AS DECIMAL(18,10)) * @Power
        
        IF ABS(@Derivative) < 0.0000001
            SET @Derivative = 0.0000001
        
        -- Newton step
        SET @NewGuess = @Guess - (@FofX / @Derivative)
        
        -- Ensure positive
        IF @NewGuess <= 0
            SET @NewGuess = 0.5 * @Guess
        
        -- Check convergence
        IF ABS(@NewGuess - @Guess) < @Tolerance
        BEGIN
            SET @Guess = @NewGuess
            SET @i = @MaxIterations  -- Exit loop
        END
        
        SET @Guess = @NewGuess
        SET @i = @i + 1
    END
    
    -- CAGR = root - 1
    SET @CAGR = @Guess - 1.0
    
    -- Calculate future values from ending value
    SET @Multiplier = 1.0 + @CAGR
    
    -- 5 years from ending value
    SET @FutureValue5Years = @EndingValue
    SET @i = 0
    WHILE @i < 5
    BEGIN
        SET @FutureValue5Years = @FutureValue5Years * @Multiplier
        SET @i = @i + 1
    END
    
    -- 10 years from ending value
    SET @FutureValue10Years = @EndingValue
    SET @i = 0
    WHILE @i < 10
    BEGIN
        SET @FutureValue10Years = @FutureValue10Years * @Multiplier
        SET @i = @i + 1
    END
    
    RETURN
END
