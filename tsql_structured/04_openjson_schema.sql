-- Parse JSON array of objects with explicit schema using OPENJSON WITH
CREATE PROCEDURE dbo.ParseProductsJson
    @JsonData NVARCHAR(MAX)
AS
BEGIN
    SELECT 
        ProductId,
        ProductName,
        Price,
        Quantity,
        Category
    FROM OPENJSON(@JsonData, '$.products')
    WITH (
        ProductId INT '$.id',
        ProductName NVARCHAR(100) '$.name',
        Price DECIMAL(10,2) '$.price',
        Quantity INT '$.qty',
        Category NVARCHAR(50) '$.category'
    )
    WHERE Price > 0
    ORDER BY ProductName
END
