-- Modify JSON document using JSON_MODIFY
CREATE PROCEDURE dbo.UpdateCustomerJson
    @JsonData NVARCHAR(MAX),
    @NewEmail NVARCHAR(200),
    @NewPhone NVARCHAR(50),
    @NewStatus NVARCHAR(20),
    @UpdatedJson NVARCHAR(MAX) OUTPUT
AS
BEGIN
    DECLARE @Result NVARCHAR(MAX) = @JsonData
    
    -- Update existing values
    SET @Result = JSON_MODIFY(@Result, '$.customer.email', @NewEmail)
    SET @Result = JSON_MODIFY(@Result, '$.customer.phone', @NewPhone)
    
    -- Add new property
    SET @Result = JSON_MODIFY(@Result, '$.customer.status', @NewStatus)
    
    -- Add timestamp
    SET @Result = JSON_MODIFY(@Result, '$.lastUpdated', CONVERT(VARCHAR(30), GETDATE(), 126))
    
    SET @UpdatedJson = @Result
END
