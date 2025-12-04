-- Math utility procedures
-- Demonstrates multiple procedures in a single file

-- Calculate absolute value
CREATE PROCEDURE dbo.AbsoluteValue
    @Value DECIMAL(18,4),
    @Result DECIMAL(18,4) OUTPUT
AS
BEGIN
    IF @Value < 0
        SET @Result = @Value * -1
    ELSE
        SET @Result = @Value
END

-- Calculate power (base^exponent) for integer exponents
CREATE PROCEDURE dbo.IntPower
    @Base DECIMAL(18,4),
    @Exponent INT,
    @Result DECIMAL(18,4) OUTPUT
AS
BEGIN
    DECLARE @Counter INT = 0
    DECLARE @IsNegative BIT = 0
    
    SET @Result = 1
    
    IF @Exponent < 0
    BEGIN
        SET @IsNegative = 1
        SET @Exponent = @Exponent * -1
    END
    
    WHILE @Counter < @Exponent
    BEGIN
        SET @Result = @Result * @Base
        SET @Counter = @Counter + 1
    END
    
    IF @IsNegative = 1
        SET @Result = 1 / @Result
END

-- Find minimum of two values
CREATE PROCEDURE dbo.MinValue
    @A DECIMAL(18,4),
    @B DECIMAL(18,4),
    @Result DECIMAL(18,4) OUTPUT
AS
BEGIN
    IF @A <= @B
        SET @Result = @A
    ELSE
        SET @Result = @B
END

-- Find maximum of two values
CREATE PROCEDURE dbo.MaxValue
    @A DECIMAL(18,4),
    @B DECIMAL(18,4),
    @Result DECIMAL(18,4) OUTPUT
AS
BEGIN
    IF @A >= @B
        SET @Result = @A
    ELSE
        SET @Result = @B
END

-- Clamp value between min and max
CREATE PROCEDURE dbo.ClampValue
    @Value DECIMAL(18,4),
    @MinVal DECIMAL(18,4),
    @MaxVal DECIMAL(18,4),
    @Result DECIMAL(18,4) OUTPUT
AS
BEGIN
    IF @Value < @MinVal
        SET @Result = @MinVal
    ELSE IF @Value > @MaxVal
        SET @Result = @MaxVal
    ELSE
        SET @Result = @Value
END
