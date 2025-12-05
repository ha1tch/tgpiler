-- Extract scalar values from JSON using JSON_VALUE
CREATE PROCEDURE dbo.ParseCustomerJson
    @JsonData NVARCHAR(MAX),
    @CustomerName NVARCHAR(100) OUTPUT,
    @CustomerId INT OUTPUT,
    @Email NVARCHAR(200) OUTPUT
AS
BEGIN
    SET @CustomerName = JSON_VALUE(@JsonData, '$.customer.name')
    SET @CustomerId = CAST(JSON_VALUE(@JsonData, '$.customer.id') AS INT)
    SET @Email = JSON_VALUE(@JsonData, '$.customer.email')
END
