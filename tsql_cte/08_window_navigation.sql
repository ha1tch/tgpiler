-- Window functions: Navigation (LEAD, LAG, FIRST_VALUE, LAST_VALUE)
CREATE PROCEDURE GetOrderHistory
    @CustomerID INT
AS
BEGIN
    SELECT 
        OrderID,
        OrderDate,
        Amount,
        LAG(Amount, 1, 0) OVER (ORDER BY OrderDate) AS PreviousAmount,
        LEAD(Amount, 1, 0) OVER (ORDER BY OrderDate) AS NextAmount,
        Amount - LAG(Amount, 1, 0) OVER (ORDER BY OrderDate) AS AmountChange,
        FIRST_VALUE(Amount) OVER (ORDER BY OrderDate) AS FirstOrderAmount,
        LAST_VALUE(Amount) OVER (
            ORDER BY OrderDate 
            ROWS BETWEEN CURRENT ROW AND UNBOUNDED FOLLOWING
        ) AS LastOrderAmount
    FROM Orders
    WHERE CustomerID = @CustomerID
    ORDER BY OrderDate
END
