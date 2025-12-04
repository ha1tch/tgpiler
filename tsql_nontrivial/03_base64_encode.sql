-- Base64 Encoder
-- Encodes a string to Base64 representation
-- Implements RFC 4648 standard encoding
CREATE PROCEDURE dbo.Base64Encode
    @Input VARCHAR(1000),
    @Output VARCHAR(2000) OUTPUT
AS
BEGIN
    DECLARE @Alphabet VARCHAR(64) = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/'
    DECLARE @InputLen INT = LEN(@Input)
    DECLARE @Pos INT = 1
    DECLARE @Byte1 INT
    DECLARE @Byte2 INT
    DECLARE @Byte3 INT
    DECLARE @Sextet1 INT
    DECLARE @Sextet2 INT
    DECLARE @Sextet3 INT
    DECLARE @Sextet4 INT
    DECLARE @Padding INT
    
    SET @Output = ''
    
    -- Process input in groups of 3 bytes
    WHILE @Pos <= @InputLen
    BEGIN
        -- Get up to 3 bytes
        SET @Byte1 = ASCII(SUBSTRING(@Input, @Pos, 1))
        
        IF @Pos + 1 <= @InputLen
            SET @Byte2 = ASCII(SUBSTRING(@Input, @Pos + 1, 1))
        ELSE
            SET @Byte2 = 0
            
        IF @Pos + 2 <= @InputLen
            SET @Byte3 = ASCII(SUBSTRING(@Input, @Pos + 2, 1))
        ELSE
            SET @Byte3 = 0
        
        -- Calculate padding needed
        SET @Padding = 0
        IF @Pos + 1 > @InputLen
            SET @Padding = 2
        ELSE IF @Pos + 2 > @InputLen
            SET @Padding = 1
        
        -- Extract 6-bit sextets from 24-bit group
        -- Sextet 1: bits 7-2 of byte 1
        SET @Sextet1 = @Byte1 / 4
        
        -- Sextet 2: bits 1-0 of byte 1, bits 7-4 of byte 2
        SET @Sextet2 = ((@Byte1 % 4) * 16) + (@Byte2 / 16)
        
        -- Sextet 3: bits 3-0 of byte 2, bits 7-6 of byte 3
        SET @Sextet3 = ((@Byte2 % 16) * 4) + (@Byte3 / 64)
        
        -- Sextet 4: bits 5-0 of byte 3
        SET @Sextet4 = @Byte3 % 64
        
        -- Encode sextets to Base64 characters
        SET @Output = @Output + SUBSTRING(@Alphabet, @Sextet1 + 1, 1)
        SET @Output = @Output + SUBSTRING(@Alphabet, @Sextet2 + 1, 1)
        
        IF @Padding < 2
            SET @Output = @Output + SUBSTRING(@Alphabet, @Sextet3 + 1, 1)
        ELSE
            SET @Output = @Output + '='
            
        IF @Padding < 1
            SET @Output = @Output + SUBSTRING(@Alphabet, @Sextet4 + 1, 1)
        ELSE
            SET @Output = @Output + '='
        
        SET @Pos = @Pos + 3
    END
END
