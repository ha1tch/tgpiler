-- FizzBuzz implementation
-- Classic programming interview question
CREATE PROCEDURE dbo.FizzBuzz
    @MaxNum INT
AS
BEGIN
    DECLARE @Counter INT = 1
    DECLARE @Output VARCHAR(100)
    
    WHILE @Counter <= @MaxNum
    BEGIN
        IF @Counter % 15 = 0
            SET @Output = 'FizzBuzz'
        ELSE IF @Counter % 3 = 0
            SET @Output = 'Fizz'
        ELSE IF @Counter % 5 = 0
            SET @Output = 'Buzz'
        ELSE
            SET @Output = CAST(@Counter AS VARCHAR(10))
        
        PRINT @Output
        SET @Counter = @Counter + 1
    END
END
