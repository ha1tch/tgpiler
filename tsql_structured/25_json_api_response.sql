-- Handle paginated API response in JSON format
CREATE PROCEDURE dbo.ProcessApiResponse
    @ResponseJson NVARCHAR(MAX),
    @Success BIT OUTPUT,
    @TotalRecords INT OUTPUT,
    @PageNumber INT OUTPUT,
    @PageSize INT OUTPUT,
    @HasMorePages BIT OUTPUT,
    @ErrorCode NVARCHAR(50) OUTPUT,
    @ErrorMessage NVARCHAR(500) OUTPUT
AS
BEGIN
    SET @Success = 0
    SET @ErrorCode = NULL
    SET @ErrorMessage = NULL
    
    -- Validate JSON
    IF ISJSON(@ResponseJson) = 0
    BEGIN
        SET @ErrorCode = 'INVALID_JSON'
        SET @ErrorMessage = 'Response is not valid JSON'
        RETURN
    END
    
    -- Check for error response
    IF JSON_VALUE(@ResponseJson, '$.error') IS NOT NULL
    BEGIN
        SET @ErrorCode = JSON_VALUE(@ResponseJson, '$.error.code')
        SET @ErrorMessage = JSON_VALUE(@ResponseJson, '$.error.message')
        RETURN
    END
    
    -- Extract pagination metadata
    SET @TotalRecords = CAST(JSON_VALUE(@ResponseJson, '$.meta.totalRecords') AS INT)
    SET @PageNumber = CAST(JSON_VALUE(@ResponseJson, '$.meta.page') AS INT)
    SET @PageSize = CAST(JSON_VALUE(@ResponseJson, '$.meta.pageSize') AS INT)
    SET @HasMorePages = CASE 
        WHEN JSON_VALUE(@ResponseJson, '$.meta.hasMore') = 'true' THEN 1 
        ELSE 0 
    END
    
    SET @Success = 1
    
    -- Return data records
    SELECT 
        RecordId,
        RecordType,
        RecordData,
        CreatedAt,
        UpdatedAt
    FROM OPENJSON(@ResponseJson, '$.data')
    WITH (
        RecordId INT '$.id',
        RecordType NVARCHAR(50) '$.type',
        RecordData NVARCHAR(MAX) '$.attributes' AS JSON,
        CreatedAt DATETIME2 '$.createdAt',
        UpdatedAt DATETIME2 '$.updatedAt'
    )
    ORDER BY RecordId
END
