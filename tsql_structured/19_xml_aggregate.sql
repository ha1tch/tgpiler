-- Aggregate data from XML using .nodes()
CREATE PROCEDURE dbo.CalculateXmlOrderTotals
    @XmlData XML,
    @TotalAmount DECIMAL(18,2) OUTPUT,
    @ItemCount INT OUTPUT,
    @AveragePrice DECIMAL(18,2) OUTPUT
AS
BEGIN
    -- Shred XML items into temp table for aggregation
    CREATE TABLE #Items (
        ProductName NVARCHAR(100),
        Quantity INT,
        UnitPrice DECIMAL(10,2),
        LineTotal DECIMAL(18,2)
    )
    
    INSERT INTO #Items (ProductName, Quantity, UnitPrice, LineTotal)
    SELECT
        Item.value('(name)[1]', 'NVARCHAR(100)'),
        Item.value('(qty)[1]', 'INT'),
        Item.value('(price)[1]', 'DECIMAL(10,2)'),
        Item.value('(qty)[1]', 'INT') * Item.value('(price)[1]', 'DECIMAL(10,2)')
    FROM @XmlData.nodes('/order/items/item') AS T(Item)
    
    -- Calculate aggregates
    SELECT 
        @TotalAmount = ISNULL(SUM(LineTotal), 0),
        @ItemCount = ISNULL(SUM(Quantity), 0),
        @AveragePrice = ISNULL(AVG(UnitPrice), 0)
    FROM #Items
    
    -- Return detailed breakdown
    SELECT 
        ProductName,
        Quantity,
        UnitPrice,
        LineTotal,
        SUM(LineTotal) OVER (ORDER BY ProductName) AS RunningTotal,
        CAST(LineTotal * 100.0 / NULLIF(@TotalAmount, 0) AS DECIMAL(5,2)) AS PercentOfTotal
    FROM #Items
    ORDER BY ProductName
    
    DROP TABLE #Items
END
