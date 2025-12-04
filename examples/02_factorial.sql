-- Calculate factorial of a number
-- Uses iterative approach with WHILE loop
CREATE PROCEDURE dbo.Factorial
    @N INT,
    @Result BIGINT OUTPUT
AS
BEGIN
    DECLARE @Counter INT = 1
    SET @Result = 1
    
    WHILE @Counter <= @N
    BEGIN
        SET @Result = @Result * @Counter
        SET @Counter = @Counter + 1
    END
END
