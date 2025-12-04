-- Validate credit card number using Luhn algorithm
CREATE PROCEDURE dbo.ValidateCreditCard
    @CardNumber VARCHAR(20),
    @IsValid BIT OUTPUT,
    @CardType VARCHAR(20) OUTPUT
AS
BEGIN
    DECLARE @CleanNumber VARCHAR(20) = ''
    DECLARE @Index INT = 1
    DECLARE @Length INT
    DECLARE @Char CHAR(1)
    DECLARE @Sum INT = 0
    DECLARE @Digit INT
    DECLARE @IsDouble BIT
    DECLARE @Prefix2 VARCHAR(2)
    DECLARE @Prefix4 VARCHAR(4)
    
    SET @IsValid = 0
    SET @CardType = 'Unknown'
    
    -- Remove spaces and dashes
    SET @Length = LEN(@CardNumber)
    WHILE @Index <= @Length
    BEGIN
        SET @Char = SUBSTRING(@CardNumber, @Index, 1)
        IF @Char >= '0' AND @Char <= '9'
            SET @CleanNumber = @CleanNumber + @Char
        SET @Index = @Index + 1
    END
    
    SET @Length = LEN(@CleanNumber)
    
    -- Check minimum length
    IF @Length < 13 OR @Length > 19
    BEGIN
        RETURN
    END
    
    -- Determine card type by prefix
    SET @Prefix2 = LEFT(@CleanNumber, 2)
    SET @Prefix4 = LEFT(@CleanNumber, 4)
    
    IF LEFT(@CleanNumber, 1) = '4'
        SET @CardType = 'Visa'
    
    IF @Prefix2 >= '51' AND @Prefix2 <= '55'
        SET @CardType = 'MasterCard'
    
    IF @Prefix2 = '34' OR @Prefix2 = '37'
        SET @CardType = 'American Express'
    
    IF @Prefix4 = '6011'
        SET @CardType = 'Discover'
    
    -- Luhn algorithm
    SET @Index = @Length
    SET @IsDouble = 0
    
    WHILE @Index >= 1
    BEGIN
        SET @Digit = CAST(SUBSTRING(@CleanNumber, @Index, 1) AS INT)
        
        IF @IsDouble = 1
        BEGIN
            SET @Digit = @Digit * 2
            IF @Digit > 9
                SET @Digit = @Digit - 9
        END
        
        SET @Sum = @Sum + @Digit
        SET @IsDouble = 1 - @IsDouble
        SET @Index = @Index - 1
    END
    
    IF @Sum % 10 = 0
        SET @IsValid = 1
END
