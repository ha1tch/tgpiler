-- CTE with UPDATE: Update customer tier based on spending
CREATE PROCEDURE UpdateCustomerTiers
AS
BEGIN
    WITH CustomerSpending AS (
        SELECT 
            CustomerID,
            SUM(Amount) AS TotalSpent
        FROM Orders
        WHERE OrderDate >= DATEADD(year, -1, GETDATE())
        GROUP BY CustomerID
    ),
    TierAssignments AS (
        SELECT 
            CustomerID,
            CASE 
                WHEN TotalSpent >= 10000 THEN 'Platinum'
                WHEN TotalSpent >= 5000 THEN 'Gold'
                WHEN TotalSpent >= 1000 THEN 'Silver'
                ELSE 'Bronze'
            END AS NewTier
        FROM CustomerSpending
    )
    UPDATE c
    SET c.Tier = t.NewTier,
        c.LastTierUpdate = GETDATE()
    FROM Customers c
    INNER JOIN TierAssignments t ON c.ID = t.CustomerID
    WHERE c.Tier <> t.NewTier
END
