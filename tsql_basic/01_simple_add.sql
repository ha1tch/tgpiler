-- Simple addition of two numbers
CREATE PROCEDURE dbo.AddNumbers
    @A INT,
    @B INT,
    @Result INT OUTPUT
AS
BEGIN
    SET @Result = @A + @B
END
