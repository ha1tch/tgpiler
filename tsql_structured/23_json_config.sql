-- Parse application configuration from JSON
CREATE PROCEDURE dbo.ParseAppConfig
    @ConfigJson NVARCHAR(MAX),
    @AppName NVARCHAR(100) OUTPUT,
    @Environment NVARCHAR(50) OUTPUT,
    @DbConnectionString NVARCHAR(500) OUTPUT,
    @CacheEnabled BIT OUTPUT,
    @CacheTtlSeconds INT OUTPUT,
    @LogLevel NVARCHAR(20) OUTPUT,
    @MaxRetries INT OUTPUT
AS
BEGIN
    -- Validate JSON
    IF ISJSON(@ConfigJson) = 0
    BEGIN
        RAISERROR('Invalid JSON configuration', 16, 1)
        RETURN
    END
    
    -- Extract basic settings
    SET @AppName = JSON_VALUE(@ConfigJson, '$.application.name')
    SET @Environment = JSON_VALUE(@ConfigJson, '$.application.environment')
    
    -- Extract database config
    SET @DbConnectionString = JSON_VALUE(@ConfigJson, '$.database.connectionString')
    
    -- Extract cache settings with defaults
    SET @CacheEnabled = ISNULL(
        CAST(JSON_VALUE(@ConfigJson, '$.cache.enabled') AS BIT), 
        0
    )
    SET @CacheTtlSeconds = ISNULL(
        CAST(JSON_VALUE(@ConfigJson, '$.cache.ttlSeconds') AS INT), 
        300
    )
    
    -- Extract logging config
    SET @LogLevel = ISNULL(JSON_VALUE(@ConfigJson, '$.logging.level'), 'INFO')
    
    -- Extract retry policy
    SET @MaxRetries = ISNULL(
        CAST(JSON_VALUE(@ConfigJson, '$.retryPolicy.maxRetries') AS INT), 
        3
    )
    
    -- Return feature flags as result set
    SELECT 
        JSON_VALUE([value], '$.name') AS FeatureName,
        CAST(JSON_VALUE([value], '$.enabled') AS BIT) AS IsEnabled,
        JSON_VALUE([value], '$.description') AS Description
    FROM OPENJSON(@ConfigJson, '$.featureFlags')
    ORDER BY FeatureName
END
