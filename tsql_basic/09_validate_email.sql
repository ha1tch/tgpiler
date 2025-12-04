-- Simple email format validation
-- Checks for @ symbol and domain
CREATE PROCEDURE dbo.ValidateEmail
    @Email VARCHAR(255),
    @IsValid BIT OUTPUT,
    @ErrorMessage VARCHAR(100) OUTPUT
AS
BEGIN
    DECLARE @AtPos INT
    DECLARE @DotPos INT
    DECLARE @Length INT
    
    SET @IsValid = 0
    SET @ErrorMessage = ''
    SET @Length = LEN(@Email)
    
    -- Check minimum length
    IF @Length < 5
    BEGIN
        SET @ErrorMessage = 'Email too short'
        RETURN
    END
    
    -- Find @ symbol
    SET @AtPos = CHARINDEX('@', @Email)
    IF @AtPos = 0
    BEGIN
        SET @ErrorMessage = 'Missing @ symbol'
        RETURN
    END
    
    -- Check for content before @
    IF @AtPos = 1
    BEGIN
        SET @ErrorMessage = 'No username before @'
        RETURN
    END
    
    -- Check for dot after @
    SET @DotPos = CHARINDEX('.', @Email, @AtPos)
    IF @DotPos = 0
    BEGIN
        SET @ErrorMessage = 'No domain extension'
        RETURN
    END
    
    -- Check for content between @ and .
    IF @DotPos = @AtPos + 1
    BEGIN
        SET @ErrorMessage = 'No domain name'
        RETURN
    END
    
    -- Check for content after dot
    IF @DotPos = @Length
    BEGIN
        SET @ErrorMessage = 'No extension after dot'
        RETURN
    END
    
    SET @IsValid = 1
END
