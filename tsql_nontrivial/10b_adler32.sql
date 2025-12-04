-- Adler-32 Checksum
-- Faster than CRC but slightly weaker, used in zlib
CREATE PROCEDURE dbo.Adler32
    @Data VARCHAR(8000),
    @Checksum BIGINT OUTPUT
AS
BEGIN
    DECLARE @MOD_ADLER INT = 65521  -- Largest prime < 2^16
    DECLARE @A BIGINT = 1
    DECLARE @B BIGINT = 0
    DECLARE @DataLen INT = LEN(@Data)
    DECLARE @Pos INT = 1
    DECLARE @Byte INT
    
    WHILE @Pos <= @DataLen
    BEGIN
        SET @Byte = ASCII(SUBSTRING(@Data, @Pos, 1))
        SET @A = (@A + @Byte) % @MOD_ADLER
        SET @B = (@B + @A) % @MOD_ADLER
        SET @Pos = @Pos + 1
    END
    
    -- Combine: (B << 16) | A
    SET @Checksum = @B * 65536 + @A
END
