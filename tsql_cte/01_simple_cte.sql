-- Simple CTE: Calculate customer totals
CREATE PROCEDURE GetCustomerSummary
    @MinSales DECIMAL(18,2)
AS
BEGIN
    WITH CustomerSales AS (
        SELECT 
            CustomerID,
            SUM(Amount) AS TotalSales,
            COUNT(*) AS OrderCount
        FROM Orders
        GROUP BY CustomerID
    )
    SELECT 
        c.Name,
        cs.TotalSales,
        cs.OrderCount
    FROM Customers c
    INNER JOIN CustomerSales cs ON c.ID = cs.CustomerID
    WHERE cs.TotalSales >= @MinSales
    ORDER BY cs.TotalSales DESC
END
