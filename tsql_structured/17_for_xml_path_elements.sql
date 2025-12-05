-- Build XML with nested elements using FOR XML PATH
CREATE PROCEDURE dbo.ExportOrdersXml
    @CustomerId INT
AS
BEGIN
    CREATE TABLE #Orders (
        OrderId INT,
        CustId INT,
        OrderDate DATE,
        ShipAddress NVARCHAR(200),
        ShipCity NVARCHAR(100),
        ShipState NVARCHAR(50),
        Total DECIMAL(10,2)
    )
    
    INSERT INTO #Orders VALUES (101, 1, '2024-01-15', '123 Main St', 'Boston', 'MA', 150.00)
    INSERT INTO #Orders VALUES (102, 1, '2024-01-20', '123 Main St', 'Boston', 'MA', 275.50)
    INSERT INTO #Orders VALUES (103, 2, '2024-01-18', '456 Oak Ave', 'Chicago', 'IL', 89.00)
    INSERT INTO #Orders VALUES (104, 1, '2024-02-01', '789 Pine Rd', 'Boston', 'MA', 420.00)
    
    -- Generate hierarchical XML with elements
    SELECT 
        OrderId AS 'order/@id',
        OrderDate AS 'order/orderDate',
        Total AS 'order/total',
        ShipAddress AS 'order/shipping/address',
        ShipCity AS 'order/shipping/city',
        ShipState AS 'order/shipping/state'
    FROM #Orders
    WHERE CustId = @CustomerId
    ORDER BY OrderDate
    FOR XML PATH(''), ROOT('orders'), ELEMENTS
    
    DROP TABLE #Orders
END
