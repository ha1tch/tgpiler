-- Newton-Raphson Square Root
-- Iteratively approximates square root using Newton's method
-- Converges quadratically for positive inputs
CREATE PROCEDURE dbo.NewtonSqrt
    @Value DECIMAL(38,18),
    @Precision DECIMAL(38,18),
    @Result DECIMAL(38,18) OUTPUT,
    @Iterations INT OUTPUT
AS
BEGIN
    DECLARE @Guess DECIMAL(38,18)
    DECLARE @PrevGuess DECIMAL(38,18)
    DECLARE @Diff DECIMAL(38,18)
    DECLARE @MaxIterations INT = 100
    
    SET @Iterations = 0
    
    -- Handle edge cases
    IF @Value < 0
    BEGIN
        SET @Result = NULL  -- sqrt of negative is undefined
        RETURN
    END
    
    IF @Value = 0
    BEGIN
        SET @Result = 0
        RETURN
    END
    
    IF @Value = 1
    BEGIN
        SET @Result = 1
        RETURN
    END
    
    -- Initial guess: value/2 for values > 1, value for values < 1
    IF @Value > 1
        SET @Guess = @Value / 2
    ELSE
        SET @Guess = @Value
    
    -- Newton-Raphson iteration: x_new = (x + value/x) / 2
    SET @Diff = @Precision + 1  -- Ensure we enter the loop
    
    WHILE @Diff > @Precision AND @Iterations < @MaxIterations
    BEGIN
        SET @PrevGuess = @Guess
        SET @Guess = (@Guess + @Value / @Guess) / 2
        
        -- Calculate absolute difference
        SET @Diff = @Guess - @PrevGuess
        IF @Diff < 0
            SET @Diff = -@Diff
        
        SET @Iterations = @Iterations + 1
    END
    
    SET @Result = @Guess
END
