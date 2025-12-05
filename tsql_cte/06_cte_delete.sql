-- CTE with DELETE: Remove duplicate records
CREATE PROCEDURE RemoveDuplicateOrders
AS
BEGIN
    WITH DuplicateOrders AS (
        SELECT 
            OrderID,
            ROW_NUMBER() OVER (
                PARTITION BY CustomerID, OrderDate, Amount 
                ORDER BY OrderID
            ) AS RowNum
        FROM Orders
    )
    DELETE FROM DuplicateOrders
    WHERE RowNum > 1
END
