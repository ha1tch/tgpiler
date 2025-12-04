-- Modular Multiplicative Inverse using Extended Euclidean
-- Finds x such that (a * x) mod m = 1
CREATE PROCEDURE dbo.ModularInverse
    @A BIGINT,
    @Modulus BIGINT,
    @Inverse BIGINT OUTPUT,
    @Exists BIT OUTPUT
AS
BEGIN
    DECLARE @M0 BIGINT = @Modulus
    DECLARE @X0 BIGINT = 0
    DECLARE @X1 BIGINT = 1
    DECLARE @Quotient BIGINT
    DECLARE @Temp BIGINT
    DECLARE @TempA BIGINT = @A
    
    SET @Exists = 0
    SET @Inverse = 0
    
    IF @Modulus = 1
    BEGIN
        RETURN
    END
    
    -- Extended Euclidean Algorithm
    WHILE @TempA > 1
    BEGIN
        IF @Modulus = 0
        BEGIN
            RETURN  -- No inverse exists (gcd != 1)
        END
        
        SET @Quotient = @TempA / @Modulus
        SET @Temp = @Modulus
        
        SET @Modulus = @TempA % @Modulus
        SET @TempA = @Temp
        
        SET @Temp = @X0
        SET @X0 = @X1 - @Quotient * @X0
        SET @X1 = @Temp
    END
    
    -- Check if inverse exists (gcd must be 1)
    IF @TempA = 1
    BEGIN
        SET @Exists = 1
        -- Make result positive
        IF @X1 < 0
        BEGIN
            SET @X1 = @X1 + @M0
        END
        SET @Inverse = @X1
    END
END
