-- Demonstrate TRY/CATCH error handling
-- Division with error recovery
CREATE PROCEDURE dbo.SafeDivide
    @Numerator DECIMAL(18,4),
    @Denominator DECIMAL(18,4),
    @Result DECIMAL(18,4) OUTPUT,
    @ErrorOccurred BIT OUTPUT,
    @ErrorMsg VARCHAR(200) OUTPUT
AS
BEGIN
    SET @ErrorOccurred = 0
    SET @ErrorMsg = ''
    SET @Result = 0
    
    BEGIN TRY
        IF @Denominator = 0
        BEGIN
            -- Manually raise error for division by zero
            SET @ErrorOccurred = 1
            SET @ErrorMsg = 'Division by zero attempted'
            RETURN
        END
        
        SET @Result = @Numerator / @Denominator
    END TRY
    BEGIN CATCH
        SET @ErrorOccurred = 1
        SET @ErrorMsg = ERROR_MESSAGE()
    END CATCH
END
