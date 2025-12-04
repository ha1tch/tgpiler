-- Binary search simulation
-- Demonstrates the algorithm with print output
CREATE PROCEDURE dbo.BinarySearchDemo
    @Target INT,
    @ArraySize INT
AS
BEGIN
    DECLARE @Low INT = 1
    DECLARE @High INT
    DECLARE @Mid INT
    DECLARE @Iterations INT = 0
    DECLARE @Found BIT = 0
    
    SET @High = @ArraySize
    
    PRINT 'Searching for ' + CAST(@Target AS VARCHAR(10)) + ' in array of size ' + CAST(@ArraySize AS VARCHAR(10))
    
    WHILE @Low <= @High
    BEGIN
        SET @Iterations = @Iterations + 1
        SET @Mid = (@Low + @High) / 2
        
        PRINT 'Iteration ' + CAST(@Iterations AS VARCHAR(10)) + ': checking position ' + CAST(@Mid AS VARCHAR(10))
        
        IF @Mid = @Target
        BEGIN
            SET @Found = 1
            PRINT 'Found at position ' + CAST(@Mid AS VARCHAR(10))
            BREAK
        END
        ELSE IF @Mid < @Target
        BEGIN
            SET @Low = @Mid + 1
            PRINT 'Target is higher, searching right half'
        END
        ELSE
        BEGIN
            SET @High = @Mid - 1
            PRINT 'Target is lower, searching left half'
        END
    END
    
    IF @Found = 0
        PRINT 'Target not found'
    
    PRINT 'Total iterations: ' + CAST(@Iterations AS VARCHAR(10))
END
