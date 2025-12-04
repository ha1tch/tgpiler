-- Longest Common Subsequence (LCS)
-- Finds length of longest subsequence common to two strings
-- Classic dynamic programming problem
CREATE PROCEDURE dbo.LongestCommonSubsequenceLength
    @String1 NVARCHAR(255),
    @String2 NVARCHAR(255),
    @Length INT OUTPUT
AS
BEGIN
    DECLARE @Len1 INT = LEN(@String1)
    DECLARE @Len2 INT = LEN(@String2)
    DECLARE @I INT
    DECLARE @J INT
    DECLARE @Char1 NCHAR(1)
    DECLARE @Char2 NCHAR(1)
    
    -- Use two rows for space efficiency
    DECLARE @PrevRow NVARCHAR(256)
    DECLARE @CurrRow NVARCHAR(256)
    DECLARE @Diag INT
    DECLARE @Above INT
    DECLARE @Left INT
    DECLARE @MaxVal INT
    
    -- Handle empty strings
    IF @Len1 = 0 OR @Len2 = 0
    BEGIN
        SET @Length = 0
        RETURN
    END
    
    -- Initialise first row with zeros (using NCHAR codes)
    SET @PrevRow = ''
    SET @I = 0
    WHILE @I <= @Len2
    BEGIN
        SET @PrevRow = @PrevRow + NCHAR(0)
        SET @I = @I + 1
    END
    
    -- Fill the DP table row by row
    SET @I = 1
    WHILE @I <= @Len1
    BEGIN
        SET @CurrRow = NCHAR(0)  -- First column is always 0
        SET @Char1 = SUBSTRING(@String1, @I, 1)
        
        SET @J = 1
        WHILE @J <= @Len2
        BEGIN
            SET @Char2 = SUBSTRING(@String2, @J, 1)
            
            SET @Diag = UNICODE(SUBSTRING(@PrevRow, @J, 1))
            SET @Above = UNICODE(SUBSTRING(@PrevRow, @J + 1, 1))
            SET @Left = UNICODE(SUBSTRING(@CurrRow, @J, 1))
            
            IF @Char1 = @Char2
            BEGIN
                -- Characters match: LCS extends
                SET @MaxVal = @Diag + 1
            END
            ELSE
            BEGIN
                -- No match: take max of above or left
                IF @Above > @Left
                    SET @MaxVal = @Above
                ELSE
                    SET @MaxVal = @Left
            END
            
            SET @CurrRow = @CurrRow + NCHAR(@MaxVal)
            SET @J = @J + 1
        END
        
        SET @PrevRow = @CurrRow
        SET @I = @I + 1
    END
    
    SET @Length = UNICODE(SUBSTRING(@CurrRow, @Len2 + 1, 1))
END
