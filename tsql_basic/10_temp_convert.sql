-- Temperature conversion between Celsius, Fahrenheit, and Kelvin
CREATE PROCEDURE dbo.ConvertTemperature
    @Value DECIMAL(10,2),
    @FromUnit CHAR(1),
    @ToUnit CHAR(1),
    @Result DECIMAL(10,2) OUTPUT,
    @Success BIT OUTPUT
AS
BEGIN
    DECLARE @Celsius DECIMAL(10,2)
    
    SET @Success = 1
    
    -- First convert to Celsius
    IF @FromUnit = 'C'
        SET @Celsius = @Value
    ELSE IF @FromUnit = 'F'
        SET @Celsius = (@Value - 32) * 5 / 9
    ELSE IF @FromUnit = 'K'
        SET @Celsius = @Value - 273.15
    ELSE
    BEGIN
        SET @Success = 0
        SET @Result = 0
        RETURN
    END
    
    -- Then convert from Celsius to target
    IF @ToUnit = 'C'
        SET @Result = @Celsius
    ELSE IF @ToUnit = 'F'
        SET @Result = (@Celsius * 9 / 5) + 32
    ELSE IF @ToUnit = 'K'
        SET @Result = @Celsius + 273.15
    ELSE
    BEGIN
        SET @Success = 0
        SET @Result = 0
    END
END
