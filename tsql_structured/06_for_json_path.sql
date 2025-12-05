-- Build JSON from temp table data using FOR JSON PATH
CREATE PROCEDURE dbo.BuildOrdersJson
    @MinTotal DECIMAL(10,2)
AS
BEGIN
    -- Create sample data in temp table
    CREATE TABLE #Orders (
        OrderId INT,
        CustomerName NVARCHAR(100),
        OrderDate DATE,
        Total DECIMAL(10,2),
        Status NVARCHAR(20)
    )
    
    INSERT INTO #Orders VALUES (1, 'Alice Smith', '2024-01-15', 150.00, 'Shipped')
    INSERT INTO #Orders VALUES (2, 'Bob Jones', '2024-01-16', 89.50, 'Pending')
    INSERT INTO #Orders VALUES (3, 'Carol White', '2024-01-17', 275.00, 'Delivered')
    INSERT INTO #Orders VALUES (4, 'Dave Brown', '2024-01-18', 45.00, 'Cancelled')
    
    -- Convert to JSON
    SELECT 
        OrderId AS 'order.id',
        CustomerName AS 'order.customer',
        OrderDate AS 'order.date',
        Total AS 'order.total',
        Status AS 'order.status'
    FROM #Orders
    WHERE Total >= @MinTotal
    ORDER BY OrderDate
    FOR JSON PATH
    
    DROP TABLE #Orders
END
