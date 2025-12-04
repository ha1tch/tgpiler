-- CRC-16 Checksum Calculator
-- Implements CRC-16-CCITT (polynomial 0x1021)
-- Used in XMODEM, HDLC, Bluetooth, and many protocols
CREATE PROCEDURE dbo.CRC16_CCITT
    @Data VARCHAR(8000),
    @InitialValue INT,
    @Checksum INT OUTPUT
AS
BEGIN
    DECLARE @Polynomial INT = 4129  -- 0x1021 CRC-16-CCITT polynomial
    DECLARE @DataLen INT = LEN(@Data)
    DECLARE @Pos INT = 1
    DECLARE @Byte INT
    DECLARE @Bit INT
    DECLARE @CRC INT = @InitialValue
    
    WHILE @Pos <= @DataLen
    BEGIN
        SET @Byte = ASCII(SUBSTRING(@Data, @Pos, 1))
        
        -- XOR byte into high byte of CRC
        SET @CRC = @CRC ^ (@Byte * 256)  -- Shift byte left 8 bits and XOR
        
        -- Process 8 bits
        SET @Bit = 0
        WHILE @Bit < 8
        BEGIN
            -- If MSB is set
            IF @CRC >= 32768  -- 0x8000
            BEGIN
                SET @CRC = ((@CRC * 2) % 65536) ^ @Polynomial
            END
            ELSE
            BEGIN
                SET @CRC = (@CRC * 2) % 65536
            END
            SET @Bit = @Bit + 1
        END
        
        SET @Pos = @Pos + 1
    END
    
    SET @Checksum = @CRC
END
