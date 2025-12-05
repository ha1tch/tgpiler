-- Aggregate data from JSON array
CREATE PROCEDURE dbo.CalculateOrderTotals
    @OrdersJson NVARCHAR(MAX),
    @TotalAmount DECIMAL(18,2) OUTPUT,
    @ItemCount INT OUTPUT,
    @AveragePrice DECIMAL(18,2) OUTPUT
AS
BEGIN
    SET @TotalAmount = 0
    SET @ItemCount = 0
    SET @AveragePrice = 0
    
    -- Extract and aggregate from JSON array
    CREATE TABLE #LineItems (
        ProductName NVARCHAR(100),
        Quantity INT,
        UnitPrice DECIMAL(10,2),
        LineTotal DECIMAL(18,2)
    )
    
    INSERT INTO #LineItems (ProductName, Quantity, UnitPrice, LineTotal)
    SELECT 
        JSON_VALUE([value], '$.product') AS ProductName,
        CAST(JSON_VALUE([value], '$.qty') AS INT) AS Quantity,
        CAST(JSON_VALUE([value], '$.price') AS DECIMAL(10,2)) AS UnitPrice,
        CAST(JSON_VALUE([value], '$.qty') AS INT) * 
            CAST(JSON_VALUE([value], '$.price') AS DECIMAL(10,2)) AS LineTotal
    FROM OPENJSON(@OrdersJson, '$.items')
    
    -- Calculate aggregates
    SELECT 
        @TotalAmount = ISNULL(SUM(LineTotal), 0),
        @ItemCount = ISNULL(SUM(Quantity), 0),
        @AveragePrice = ISNULL(AVG(UnitPrice), 0)
    FROM #LineItems
    
    -- Return line items with running total
    SELECT 
        ProductName,
        Quantity,
        UnitPrice,
        LineTotal,
        SUM(LineTotal) OVER (ORDER BY ProductName ROWS UNBOUNDED PRECEDING) AS RunningTotal
    FROM #LineItems
    ORDER BY ProductName
    
    DROP TABLE #LineItems
END
