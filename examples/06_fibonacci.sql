-- Generate Nth Fibonacci number
CREATE PROCEDURE dbo.Fibonacci
    @N INT,
    @Result BIGINT OUTPUT
AS
BEGIN
    DECLARE @Prev BIGINT = 0
    DECLARE @Curr BIGINT = 1
    DECLARE @Temp BIGINT
    DECLARE @Counter INT = 2
    
    IF @N <= 0
    BEGIN
        SET @Result = 0
        RETURN
    END
    
    IF @N = 1
    BEGIN
        SET @Result = 1
        RETURN
    END
    
    WHILE @Counter <= @N
    BEGIN
        SET @Temp = @Curr
        SET @Curr = @Prev + @Curr
        SET @Prev = @Temp
        SET @Counter = @Counter + 1
    END
    
    SET @Result = @Curr
END
