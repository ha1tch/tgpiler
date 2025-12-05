-- Window functions: Running aggregates
CREATE PROCEDURE GetSalesAnalysis
    @StartDate DATE,
    @EndDate DATE
AS
BEGIN
    SELECT 
        OrderDate,
        DailySales,
        SUM(DailySales) OVER (ORDER BY OrderDate ROWS UNBOUNDED PRECEDING) AS RunningTotal,
        AVG(DailySales) OVER (ORDER BY OrderDate ROWS BETWEEN 6 PRECEDING AND CURRENT ROW) AS SevenDayAvg,
        MIN(DailySales) OVER (ORDER BY OrderDate ROWS BETWEEN 29 PRECEDING AND CURRENT ROW) AS ThirtyDayMin,
        MAX(DailySales) OVER (ORDER BY OrderDate ROWS BETWEEN 29 PRECEDING AND CURRENT ROW) AS ThirtyDayMax,
        COUNT(*) OVER () AS TotalDays,
        SUM(DailySales) OVER () AS GrandTotal
    FROM (
        SELECT 
            CAST(OrderDate AS DATE) AS OrderDate,
            SUM(Amount) AS DailySales
        FROM Orders
        WHERE OrderDate BETWEEN @StartDate AND @EndDate
        GROUP BY CAST(OrderDate AS DATE)
    ) AS DailySummary
    ORDER BY OrderDate
END
