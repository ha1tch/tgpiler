-- Merge two JSON documents
CREATE PROCEDURE dbo.MergeCustomerData
    @BaseJson NVARCHAR(MAX),
    @UpdateJson NVARCHAR(MAX),
    @MergedJson NVARCHAR(MAX) OUTPUT
AS
BEGIN
    DECLARE @Result NVARCHAR(MAX) = @BaseJson
    
    -- Get values from update JSON and apply to base
    DECLARE @NewName NVARCHAR(100) = JSON_VALUE(@UpdateJson, '$.name')
    DECLARE @NewEmail NVARCHAR(200) = JSON_VALUE(@UpdateJson, '$.email')
    DECLARE @NewPhone NVARCHAR(50) = JSON_VALUE(@UpdateJson, '$.phone')
    DECLARE @NewAddress NVARCHAR(MAX) = JSON_QUERY(@UpdateJson, '$.address')
    
    -- Apply non-null updates
    IF @NewName IS NOT NULL
        SET @Result = JSON_MODIFY(@Result, '$.name', @NewName)
    
    IF @NewEmail IS NOT NULL
        SET @Result = JSON_MODIFY(@Result, '$.email', @NewEmail)
    
    IF @NewPhone IS NOT NULL
        SET @Result = JSON_MODIFY(@Result, '$.phone', @NewPhone)
    
    -- Merge nested object (address)
    IF @NewAddress IS NOT NULL
    BEGIN
        -- Update individual address fields
        DECLARE @Street NVARCHAR(200) = JSON_VALUE(@UpdateJson, '$.address.street')
        DECLARE @City NVARCHAR(100) = JSON_VALUE(@UpdateJson, '$.address.city')
        DECLARE @State NVARCHAR(50) = JSON_VALUE(@UpdateJson, '$.address.state')
        DECLARE @Zip NVARCHAR(20) = JSON_VALUE(@UpdateJson, '$.address.zip')
        
        IF @Street IS NOT NULL
            SET @Result = JSON_MODIFY(@Result, '$.address.street', @Street)
        IF @City IS NOT NULL
            SET @Result = JSON_MODIFY(@Result, '$.address.city', @City)
        IF @State IS NOT NULL
            SET @Result = JSON_MODIFY(@Result, '$.address.state', @State)
        IF @Zip IS NOT NULL
            SET @Result = JSON_MODIFY(@Result, '$.address.zip', @Zip)
    END
    
    SET @MergedJson = @Result
END
