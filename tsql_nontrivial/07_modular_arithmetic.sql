-- Modular Exponentiation (Binary Method)
-- Computes (base^exponent) mod modulus efficiently
-- Foundation for RSA and other cryptographic algorithms
CREATE PROCEDURE dbo.ModularExponentiation
    @Base BIGINT,
    @Exponent BIGINT,
    @Modulus BIGINT,
    @Result BIGINT OUTPUT
AS
BEGIN
    DECLARE @CurrentBase BIGINT
    DECLARE @CurrentExp BIGINT
    
    -- Handle edge cases
    IF @Modulus = 1
    BEGIN
        SET @Result = 0
        RETURN
    END
    
    IF @Exponent = 0
    BEGIN
        SET @Result = 1
        RETURN
    END
    
    IF @Exponent < 0
    BEGIN
        SET @Result = NULL  -- Negative exponents need modular inverse
        RETURN
    END
    
    -- Normalise base
    SET @CurrentBase = @Base % @Modulus
    IF @CurrentBase < 0
        SET @CurrentBase = @CurrentBase + @Modulus
    
    SET @Result = 1
    SET @CurrentExp = @Exponent
    
    -- Binary exponentiation (right-to-left)
    WHILE @CurrentExp > 0
    BEGIN
        -- If current bit is 1, multiply result by current base
        IF @CurrentExp % 2 = 1
        BEGIN
            SET @Result = (@Result * @CurrentBase) % @Modulus
        END
        
        -- Square the base for next bit
        SET @CurrentBase = (@CurrentBase * @CurrentBase) % @Modulus
        
        -- Move to next bit
        SET @CurrentExp = @CurrentExp / 2
    END
END
