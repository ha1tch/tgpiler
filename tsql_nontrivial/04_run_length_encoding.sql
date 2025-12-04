-- Run-Length Encoding (RLE) Compression
-- Compresses repeated characters: "AAABBC" -> "3A2B1C"
CREATE PROCEDURE dbo.RunLengthEncode
    @Input VARCHAR(1000),
    @Encoded VARCHAR(2000) OUTPUT
AS
BEGIN
    DECLARE @InputLen INT = LEN(@Input)
    DECLARE @Pos INT = 1
    DECLARE @CurrentChar CHAR(1)
    DECLARE @NextChar CHAR(1)
    DECLARE @RunCount INT
    DECLARE @Counting BIT
    
    SET @Encoded = ''
    
    IF @InputLen = 0
    BEGIN
        RETURN
    END
    
    WHILE @Pos <= @InputLen
    BEGIN
        SET @CurrentChar = SUBSTRING(@Input, @Pos, 1)
        SET @RunCount = 1
        SET @Counting = 1
        
        -- Count consecutive identical characters
        WHILE @Counting = 1
        BEGIN
            IF @Pos + @RunCount <= @InputLen
            BEGIN
                SET @NextChar = SUBSTRING(@Input, @Pos + @RunCount, 1)
                IF @NextChar = @CurrentChar
                BEGIN
                    SET @RunCount = @RunCount + 1
                END
                ELSE
                BEGIN
                    SET @Counting = 0
                END
            END
            ELSE
            BEGIN
                SET @Counting = 0
            END
        END
        
        -- Append count and character
        SET @Encoded = @Encoded + CAST(@RunCount AS VARCHAR(10)) + @CurrentChar
        
        SET @Pos = @Pos + @RunCount
    END
END
