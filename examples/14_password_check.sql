-- Check password strength
-- Returns score from 0-5 based on criteria met
CREATE PROCEDURE dbo.CheckPasswordStrength
    @Password VARCHAR(100),
    @Score INT OUTPUT,
    @Feedback VARCHAR(500) OUTPUT
AS
BEGIN
    DECLARE @Length INT
    DECLARE @HasUpper BIT = 0
    DECLARE @HasLower BIT = 0
    DECLARE @HasDigit BIT = 0
    DECLARE @HasSpecial BIT = 0
    DECLARE @Index INT = 1
    DECLARE @Char CHAR(1)
    DECLARE @CharCode INT
    
    SET @Score = 0
    SET @Feedback = ''
    SET @Length = LEN(@Password)
    
    -- Check length
    IF @Length >= 8
    BEGIN
        SET @Score = @Score + 1
        IF @Length >= 12
            SET @Score = @Score + 1
    END
    ELSE
        SET @Feedback = @Feedback + 'Password should be at least 8 characters. '
    
    -- Scan each character
    WHILE @Index <= @Length
    BEGIN
        SET @Char = SUBSTRING(@Password, @Index, 1)
        SET @CharCode = ASCII(@Char)
        
        IF @CharCode >= 65 AND @CharCode <= 90
            SET @HasUpper = 1
        ELSE IF @CharCode >= 97 AND @CharCode <= 122
            SET @HasLower = 1
        ELSE IF @CharCode >= 48 AND @CharCode <= 57
            SET @HasDigit = 1
        ELSE
            SET @HasSpecial = 1
        
        SET @Index = @Index + 1
    END
    
    -- Add points for character variety
    IF @HasUpper = 1
        SET @Score = @Score + 1
    ELSE
        SET @Feedback = @Feedback + 'Add uppercase letters. '
    
    IF @HasLower = 1
        SET @Score = @Score + 1
    ELSE
        SET @Feedback = @Feedback + 'Add lowercase letters. '
    
    IF @HasDigit = 1
        SET @Score = @Score + 1
    ELSE
        SET @Feedback = @Feedback + 'Add numbers. '
    
    IF @HasSpecial = 1
        SET @Score = @Score + 1
    ELSE
        SET @Feedback = @Feedback + 'Add special characters. '
    
    IF @Score >= 6
        SET @Feedback = 'Strong password!'
    ELSE IF @Score >= 4
        SET @Feedback = 'Moderate password. ' + @Feedback
    ELSE
        SET @Feedback = 'Weak password. ' + @Feedback
END
