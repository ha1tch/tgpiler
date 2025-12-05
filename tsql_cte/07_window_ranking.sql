-- Window functions: Ranking functions
CREATE PROCEDURE GetProductRankings
    @CategoryID INT
AS
BEGIN
    SELECT 
        ProductID,
        ProductName,
        Price,
        ROW_NUMBER() OVER (ORDER BY Price DESC) AS PriceRank,
        RANK() OVER (ORDER BY Price DESC) AS PriceRankWithTies,
        DENSE_RANK() OVER (ORDER BY Price DESC) AS DensePriceRank,
        NTILE(4) OVER (ORDER BY Price) AS PriceQuartile
    FROM Products
    WHERE CategoryID = @CategoryID
    ORDER BY PriceRank
END
