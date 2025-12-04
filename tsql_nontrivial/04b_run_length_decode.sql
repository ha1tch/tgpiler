-- Run-Length Decoding
-- Decompresses RLE: "3A2B1C" -> "AAABBC"
CREATE PROCEDURE dbo.RunLengthDecode
    @Encoded VARCHAR(2000),
    @Output VARCHAR(4000) OUTPUT
AS
BEGIN
    DECLARE @EncodedLen INT = LEN(@Encoded)
    DECLARE @Pos INT = 1
    DECLARE @NumStr VARCHAR(10)
    DECLARE @RunCount INT
    DECLARE @Char CHAR(1)
    DECLARE @I INT
    DECLARE @CurrentChar CHAR(1)
    DECLARE @IsDigit BIT
    
    SET @Output = ''
    
    IF @EncodedLen = 0
    BEGIN
        RETURN
    END
    
    WHILE @Pos <= @EncodedLen
    BEGIN
        -- Parse the number
        SET @NumStr = ''
        SET @IsDigit = 1
        
        WHILE @IsDigit = 1 AND @Pos <= @EncodedLen
        BEGIN
            SET @CurrentChar = SUBSTRING(@Encoded, @Pos, 1)
            IF @CurrentChar >= '0' AND @CurrentChar <= '9'
            BEGIN
                SET @NumStr = @NumStr + @CurrentChar
                SET @Pos = @Pos + 1
            END
            ELSE
            BEGIN
                SET @IsDigit = 0
            END
        END
        
        -- Get the character
        IF @Pos <= @EncodedLen
        BEGIN
            SET @Char = SUBSTRING(@Encoded, @Pos, 1)
            SET @Pos = @Pos + 1
            
            -- Convert and validate count
            IF LEN(@NumStr) > 0
            BEGIN
                SET @RunCount = CAST(@NumStr AS INT)
                
                -- Append character RunCount times
                SET @I = 0
                WHILE @I < @RunCount
                BEGIN
                    SET @Output = @Output + @Char
                    SET @I = @I + 1
                END
            END
        END
    END
END
