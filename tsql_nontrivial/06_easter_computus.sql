-- Computus: Easter Date Calculation
-- Implements the Anonymous Gregorian algorithm
-- Valid for years 1583 onwards (Gregorian calendar)
CREATE PROCEDURE dbo.CalculateEasterDate
    @Year INT,
    @EasterMonth INT OUTPUT,
    @EasterDay INT OUTPUT
AS
BEGIN
    DECLARE @A INT
    DECLARE @B INT
    DECLARE @C INT
    DECLARE @D INT
    DECLARE @E INT
    DECLARE @F INT
    DECLARE @G INT
    DECLARE @H INT
    DECLARE @I INT
    DECLARE @K INT
    DECLARE @L INT
    DECLARE @M INT
    
    -- Anonymous Gregorian algorithm
    SET @A = @Year % 19
    SET @B = @Year / 100
    SET @C = @Year % 100
    SET @D = @B / 4
    SET @E = @B % 4
    SET @F = (@B + 8) / 25
    SET @G = (@B - @F + 1) / 3
    SET @H = (19 * @A + @B - @D - @G + 15) % 30
    SET @I = @C / 4
    SET @K = @C % 4
    SET @L = (32 + 2 * @E + 2 * @I - @H - @K) % 7
    SET @M = (@A + 11 * @H + 22 * @L) / 451
    
    SET @EasterMonth = (@H + @L - 7 * @M + 114) / 31
    SET @EasterDay = ((@H + @L - 7 * @M + 114) % 31) + 1
END
