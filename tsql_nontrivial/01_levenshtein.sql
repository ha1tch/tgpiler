-- Levenshtein Distance (Edit Distance)
-- Calculates minimum edits needed to transform one string into another
-- Uses dynamic programming approach with O(m*n) character comparisons
CREATE PROCEDURE dbo.LevenshteinDistance
    @Source NVARCHAR(255),
    @Target NVARCHAR(255),
    @Distance INT OUTPUT
AS
BEGIN
    DECLARE @SourceLen INT = LEN(@Source)
    DECLARE @TargetLen INT = LEN(@Target)
    
    -- Handle empty strings
    IF @SourceLen = 0
    BEGIN
        SET @Distance = @TargetLen
        RETURN
    END
    
    IF @TargetLen = 0
    BEGIN
        SET @Distance = @SourceLen
        RETURN
    END
    
    -- We use two rows instead of full matrix to save space
    -- Previous row and current row
    DECLARE @PrevRow NVARCHAR(256)
    DECLARE @CurrRow NVARCHAR(256)
    DECLARE @I INT
    DECLARE @J INT
    DECLARE @Cost INT
    DECLARE @InsertCost INT
    DECLARE @DeleteCost INT
    DECLARE @ReplaceCost INT
    DECLARE @MinCost INT
    DECLARE @PrevDiag INT
    DECLARE @PrevRowJ INT
    
    -- Initialise using character codes as compact storage
    -- Each position holds cost 0-255
    SET @PrevRow = ''
    SET @I = 0
    WHILE @I <= @TargetLen
    BEGIN
        SET @PrevRow = @PrevRow + NCHAR(@I)
        SET @I = @I + 1
    END
    
    -- Process each character of source
    SET @I = 1
    WHILE @I <= @SourceLen
    BEGIN
        SET @CurrRow = NCHAR(@I)  -- First element is distance from empty target
        SET @PrevDiag = @I - 1
        
        SET @J = 1
        WHILE @J <= @TargetLen
        BEGIN
            -- Cost is 0 if characters match, 1 otherwise
            IF SUBSTRING(@Source, @I, 1) = SUBSTRING(@Target, @J, 1)
                SET @Cost = 0
            ELSE
                SET @Cost = 1
            
            SET @PrevRowJ = UNICODE(SUBSTRING(@PrevRow, @J + 1, 1))
            
            -- Calculate costs for each operation
            SET @InsertCost = UNICODE(SUBSTRING(@CurrRow, @J, 1)) + 1
            SET @DeleteCost = @PrevRowJ + 1
            SET @ReplaceCost = @PrevDiag + @Cost
            
            -- Find minimum
            SET @MinCost = @InsertCost
            IF @DeleteCost < @MinCost
                SET @MinCost = @DeleteCost
            IF @ReplaceCost < @MinCost
                SET @MinCost = @ReplaceCost
            
            SET @CurrRow = @CurrRow + NCHAR(@MinCost)
            SET @PrevDiag = @PrevRowJ
            
            SET @J = @J + 1
        END
        
        SET @PrevRow = @CurrRow
        SET @I = @I + 1
    END
    
    SET @Distance = UNICODE(SUBSTRING(@CurrRow, @TargetLen + 1, 1))
END
