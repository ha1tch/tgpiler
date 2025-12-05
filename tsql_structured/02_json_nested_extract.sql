-- Extract nested objects and arrays from JSON
CREATE PROCEDURE dbo.ParseOrderJson
    @JsonData NVARCHAR(MAX),
    @OrderId INT OUTPUT,
    @CustomerName NVARCHAR(100) OUTPUT,
    @ShippingCity NVARCHAR(100) OUTPUT,
    @FirstItemName NVARCHAR(100) OUTPUT,
    @TotalItems INT OUTPUT
AS
BEGIN
    -- Extract order-level data
    SET @OrderId = CAST(JSON_VALUE(@JsonData, '$.order.id') AS INT)
    
    -- Extract nested customer name
    SET @CustomerName = JSON_VALUE(@JsonData, '$.order.customer.name')
    
    -- Extract deeply nested shipping address
    SET @ShippingCity = JSON_VALUE(@JsonData, '$.order.shipping.address.city')
    
    -- Extract first item from items array
    SET @FirstItemName = JSON_VALUE(@JsonData, '$.order.items[0].name')
    
    -- Count items using JSON_QUERY and parsing
    DECLARE @ItemsArray NVARCHAR(MAX)
    SET @ItemsArray = JSON_QUERY(@JsonData, '$.order.items')
    
    -- Simple count based on commas (approximate for demo)
    SET @TotalItems = 0
    IF @ItemsArray IS NOT NULL AND ISJSON(@ItemsArray) = 1
    BEGIN
        -- Count by looking for "name" occurrences
        DECLARE @pos INT = 1
        WHILE CHARINDEX('"name"', @ItemsArray, @pos) > 0
        BEGIN
            SET @TotalItems = @TotalItems + 1
            SET @pos = CHARINDEX('"name"', @ItemsArray, @pos) + 1
        END
    END
END
