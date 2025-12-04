-- Convert integer to Roman numerals
-- Supports numbers 1-3999
CREATE PROCEDURE dbo.ToRomanNumeral
    @Number INT,
    @Roman VARCHAR(50) OUTPUT
AS
BEGIN
    SET @Roman = ''
    
    -- Validate range
    IF @Number <= 0 OR @Number >= 4000
    BEGIN
        SET @Roman = 'OUT OF RANGE'
        RETURN
    END
    
    -- Thousands
    WHILE @Number >= 1000
    BEGIN
        SET @Roman = @Roman + 'M'
        SET @Number = @Number - 1000
    END
    
    -- 900
    IF @Number >= 900
    BEGIN
        SET @Roman = @Roman + 'CM'
        SET @Number = @Number - 900
    END
    
    -- 500
    IF @Number >= 500
    BEGIN
        SET @Roman = @Roman + 'D'
        SET @Number = @Number - 500
    END
    
    -- 400
    IF @Number >= 400
    BEGIN
        SET @Roman = @Roman + 'CD'
        SET @Number = @Number - 400
    END
    
    -- Hundreds
    WHILE @Number >= 100
    BEGIN
        SET @Roman = @Roman + 'C'
        SET @Number = @Number - 100
    END
    
    -- 90
    IF @Number >= 90
    BEGIN
        SET @Roman = @Roman + 'XC'
        SET @Number = @Number - 90
    END
    
    -- 50
    IF @Number >= 50
    BEGIN
        SET @Roman = @Roman + 'L'
        SET @Number = @Number - 50
    END
    
    -- 40
    IF @Number >= 40
    BEGIN
        SET @Roman = @Roman + 'XL'
        SET @Number = @Number - 40
    END
    
    -- Tens
    WHILE @Number >= 10
    BEGIN
        SET @Roman = @Roman + 'X'
        SET @Number = @Number - 10
    END
    
    -- 9
    IF @Number >= 9
    BEGIN
        SET @Roman = @Roman + 'IX'
        SET @Number = @Number - 9
    END
    
    -- 5
    IF @Number >= 5
    BEGIN
        SET @Roman = @Roman + 'V'
        SET @Number = @Number - 5
    END
    
    -- 4
    IF @Number >= 4
    BEGIN
        SET @Roman = @Roman + 'IV'
        SET @Number = @Number - 4
    END
    
    -- Ones
    WHILE @Number >= 1
    BEGIN
        SET @Roman = @Roman + 'I'
        SET @Number = @Number - 1
    END
END
