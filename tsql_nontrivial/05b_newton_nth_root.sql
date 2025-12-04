-- Nth Root using Newton-Raphson
-- Generalised root: computes value^(1/n)
CREATE PROCEDURE dbo.NewtonNthRoot
    @Value DECIMAL(38,18),
    @N INT,
    @Precision DECIMAL(38,18),
    @Result DECIMAL(38,18) OUTPUT,
    @Iterations INT OUTPUT
AS
BEGIN
    DECLARE @Guess DECIMAL(38,18)
    DECLARE @PrevGuess DECIMAL(38,18)
    DECLARE @Diff DECIMAL(38,18)
    DECLARE @MaxIterations INT = 100
    DECLARE @NMinus1 INT = @N - 1
    DECLARE @PowerTerm DECIMAL(38,18)
    DECLARE @I INT
    
    SET @Iterations = 0
    
    -- Validate inputs
    IF @N <= 0
    BEGIN
        SET @Result = NULL
        RETURN
    END
    
    IF @Value < 0 AND @N % 2 = 0
    BEGIN
        SET @Result = NULL  -- Even root of negative undefined
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
    
    -- Initial guess
    IF @Value > 1
        SET @Guess = @Value / @N
    ELSE
        SET @Guess = @Value
    
    IF @Guess <= 0
        SET @Guess = 1
    
    -- Newton-Raphson: x_new = ((n-1)*x + value/x^(n-1)) / n
    SET @Diff = @Precision + 1
    
    WHILE @Diff > @Precision AND @Iterations < @MaxIterations
    BEGIN
        SET @PrevGuess = @Guess
        
        -- Calculate x^(n-1)
        SET @PowerTerm = 1
        SET @I = 0
        WHILE @I < @NMinus1
        BEGIN
            SET @PowerTerm = @PowerTerm * @Guess
            SET @I = @I + 1
        END
        
        -- Newton-Raphson update
        SET @Guess = (CAST(@NMinus1 AS DECIMAL(38,18)) * @Guess + @Value / @PowerTerm) / CAST(@N AS DECIMAL(38,18))
        
        SET @Diff = @Guess - @PrevGuess
        IF @Diff < 0
            SET @Diff = -@Diff
        
        SET @Iterations = @Iterations + 1
    END
    
    SET @Result = @Guess
END
