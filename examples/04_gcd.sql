-- Greatest Common Divisor using Euclidean algorithm
CREATE PROCEDURE dbo.GCD
    @A INT,
    @B INT,
    @Result INT OUTPUT
AS
BEGIN
    DECLARE @Temp INT
    
    -- Ensure A >= B
    IF @A < @B
    BEGIN
        SET @Temp = @A
        SET @A = @B
        SET @B = @Temp
    END
    
    -- Euclidean algorithm
    WHILE @B <> 0
    BEGIN
        SET @Temp = @B
        SET @B = @A % @B
        SET @A = @Temp
    END
    
    SET @Result = @A
END
