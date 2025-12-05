-- Multiple CTEs: Customer metrics with categorization
CREATE PROCEDURE GetCustomerMetrics
    @MinOrders INT
AS
BEGIN
    WITH 
        ActiveCustomers AS (
            SELECT CustomerID, Name, Email
            FROM Customers 
            WHERE IsActive = 1
        ),
        CustomerOrders AS (
            SELECT 
                CustomerID, 
                COUNT(*) AS OrderCount, 
                SUM(Amount) AS TotalAmount,
                AVG(Amount) AS AvgOrderValue
            FROM Orders
            GROUP BY CustomerID
        ),
        CustomerCategories AS (
            SELECT 
                co.CustomerID,
                CASE 
                    WHEN co.TotalAmount >= 10000 THEN 'VIP'
                    WHEN co.TotalAmount >= 1000 THEN 'Regular'
                    ELSE 'Occasional'
                END AS Category
            FROM CustomerOrders co
        )
    SELECT 
        ac.Name,
        ac.Email,
        co.OrderCount,
        co.TotalAmount,
        co.AvgOrderValue,
        cc.Category
    FROM ActiveCustomers ac
    INNER JOIN CustomerOrders co ON ac.CustomerID = co.CustomerID
    INNER JOIN CustomerCategories cc ON ac.CustomerID = cc.CustomerID
    WHERE co.OrderCount >= @MinOrders
    ORDER BY co.TotalAmount DESC
END
