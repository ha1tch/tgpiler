-- Window functions: Pagination with ROW_NUMBER
CREATE PROCEDURE GetOrdersPage
    @PageNumber INT,
    @PageSize INT,
    @CustomerID INT = NULL
AS
BEGIN
    WITH OrdersWithRowNum AS (
        SELECT 
            OrderID,
            CustomerID,
            OrderDate,
            Amount,
            Status,
            ROW_NUMBER() OVER (ORDER BY OrderDate DESC, OrderID DESC) AS RowNum,
            COUNT(*) OVER () AS TotalCount
        FROM Orders
        WHERE @CustomerID IS NULL OR CustomerID = @CustomerID
    )
    SELECT 
        OrderID,
        CustomerID,
        OrderDate,
        Amount,
        Status,
        RowNum,
        TotalCount,
        CEILING(CAST(TotalCount AS DECIMAL) / @PageSize) AS TotalPages
    FROM OrdersWithRowNum
    WHERE RowNum > (@PageNumber - 1) * @PageSize
      AND RowNum <= @PageNumber * @PageSize
    ORDER BY RowNum
END
