-- Check if a number is prime
CREATE PROCEDURE dbo.IsPrime
    @N INT,
    @IsPrime BIT OUTPUT
AS
BEGIN
    DECLARE @Divisor INT = 2
    DECLARE @Limit INT
    
    -- Handle edge cases
    IF @N <= 1
    BEGIN
        SET @IsPrime = 0
        RETURN
    END
    
    IF @N <= 3
    BEGIN
        SET @IsPrime = 1
        RETURN
    END
    
    IF @N % 2 = 0
    BEGIN
        SET @IsPrime = 0
        RETURN
    END
    
    -- Only check odd divisors up to sqrt(N)
    SET @Limit = CAST(SQRT(CAST(@N AS FLOAT)) AS INT)
    SET @Divisor = 3
    
    WHILE @Divisor <= @Limit
    BEGIN
        IF @N % @Divisor = 0
        BEGIN
            SET @IsPrime = 0
            RETURN
        END
        SET @Divisor = @Divisor + 2
    END
    
    SET @IsPrime = 1
END
