-- Extended Euclidean Algorithm
-- Computes GCD and Bézout coefficients: ax + by = gcd(a,b)
-- Essential for modular inverse calculations in cryptography
CREATE PROCEDURE dbo.ExtendedEuclidean
    @A BIGINT,
    @B BIGINT,
    @GCD BIGINT OUTPUT,
    @X BIGINT OUTPUT,
    @Y BIGINT OUTPUT
AS
BEGIN
    DECLARE @OriginalA BIGINT = @A
    DECLARE @OriginalB BIGINT = @B
    
    -- Handle negative inputs
    IF @A < 0 SET @A = -@A
    IF @B < 0 SET @B = -@B
    
    -- Handle zero cases
    IF @A = 0
    BEGIN
        SET @GCD = @B
        SET @X = 0
        SET @Y = 1
        RETURN
    END
    
    IF @B = 0
    BEGIN
        SET @GCD = @A
        SET @X = 1
        SET @Y = 0
        RETURN
    END
    
    -- Extended Euclidean using iterative approach
    DECLARE @X0 BIGINT = 1
    DECLARE @X1 BIGINT = 0
    DECLARE @Y0 BIGINT = 0
    DECLARE @Y1 BIGINT = 1
    DECLARE @Quotient BIGINT
    DECLARE @Remainder BIGINT
    DECLARE @TempX BIGINT
    DECLARE @TempY BIGINT
    DECLARE @TempVal BIGINT
    
    WHILE @B <> 0
    BEGIN
        SET @Quotient = @A / @B
        SET @Remainder = @A % @B
        
        -- Update x coefficients
        SET @TempX = @X0 - @Quotient * @X1
        SET @X0 = @X1
        SET @X1 = @TempX
        
        -- Update y coefficients
        SET @TempY = @Y0 - @Quotient * @Y1
        SET @Y0 = @Y1
        SET @Y1 = @TempY
        
        -- Shift a, b
        SET @A = @B
        SET @B = @Remainder
    END
    
    SET @GCD = @A
    SET @X = @X0
    SET @Y = @Y0
    
    -- Adjust signs based on original inputs
    IF @OriginalA < 0
        SET @X = -@X
    IF @OriginalB < 0
        SET @Y = -@Y
END
