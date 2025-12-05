-- Validate and process JSON document
CREATE PROCEDURE dbo.ProcessApiPayload
    @JsonPayload NVARCHAR(MAX),
    @IsValid BIT OUTPUT,
    @ErrorMessage NVARCHAR(500) OUTPUT,
    @ProcessedCount INT OUTPUT
AS
BEGIN
    SET @IsValid = 0
    SET @ErrorMessage = NULL
    SET @ProcessedCount = 0
    
    -- Validate JSON structure
    IF ISJSON(@JsonPayload) = 0
    BEGIN
        SET @ErrorMessage = 'Invalid JSON format'
        RETURN
    END
    
    -- Check required fields
    IF JSON_VALUE(@JsonPayload, '$.apiVersion') IS NULL
    BEGIN
        SET @ErrorMessage = 'Missing required field: apiVersion'
        RETURN
    END
    
    IF JSON_QUERY(@JsonPayload, '$.data') IS NULL
    BEGIN
        SET @ErrorMessage = 'Missing required field: data'
        RETURN
    END
    
    -- Process data items
    DECLARE @DataArray NVARCHAR(MAX) = JSON_QUERY(@JsonPayload, '$.data')
    
    CREATE TABLE #ProcessedItems (
        ItemId INT,
        ItemType NVARCHAR(50),
        Value NVARCHAR(200)
    )
    
    INSERT INTO #ProcessedItems (ItemId, ItemType, Value)
    SELECT 
        CAST(JSON_VALUE([value], '$.id') AS INT),
        JSON_VALUE([value], '$.type'),
        JSON_VALUE([value], '$.value')
    FROM OPENJSON(@DataArray)
    WHERE JSON_VALUE([value], '$.id') IS NOT NULL
    
    SET @ProcessedCount = @@ROWCOUNT
    SET @IsValid = 1
    
    -- Return processed items
    SELECT * FROM #ProcessedItems ORDER BY ItemId
    
    DROP TABLE #ProcessedItems
END
