-- Count words in a string
-- Words are separated by spaces
CREATE PROCEDURE dbo.CountWords
    @Text VARCHAR(MAX),
    @WordCount INT OUTPUT
AS
BEGIN
    DECLARE @Length INT
    DECLARE @Index INT = 1
    DECLARE @InWord BIT = 0
    DECLARE @Char CHAR(1)
    
    SET @WordCount = 0
    SET @Length = LEN(@Text)
    
    IF @Length = 0
    BEGIN
        RETURN
    END
    
    WHILE @Index <= @Length
    BEGIN
        SET @Char = SUBSTRING(@Text, @Index, 1)
        
        IF @Char = ' ' AND @InWord = 1
            SET @InWord = 0
        
        IF @Char <> ' ' AND @InWord = 0
        BEGIN
            SET @InWord = 1
            SET @WordCount = @WordCount + 1
        END
        
        SET @Index = @Index + 1
    END
END
