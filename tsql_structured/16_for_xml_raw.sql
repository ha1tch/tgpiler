-- Build XML from data using FOR XML RAW
CREATE PROCEDURE dbo.ExportCustomersXml
    @MinBalance DECIMAL(10,2)
AS
BEGIN
    CREATE TABLE #Customers (
        CustomerId INT,
        CustomerName NVARCHAR(100),
        Email NVARCHAR(200),
        Balance DECIMAL(10,2),
        Status NVARCHAR(20)
    )
    
    INSERT INTO #Customers VALUES (1, 'Alice Smith', 'alice@example.com', 1500.00, 'Active')
    INSERT INTO #Customers VALUES (2, 'Bob Jones', 'bob@example.com', 250.00, 'Active')
    INSERT INTO #Customers VALUES (3, 'Carol White', 'carol@example.com', 5000.00, 'Premium')
    INSERT INTO #Customers VALUES (4, 'Dave Brown', 'dave@example.com', 100.00, 'Inactive')
    
    -- Generate XML with attributes (RAW mode default)
    SELECT 
        CustomerId,
        CustomerName,
        Email,
        Balance,
        Status
    FROM #Customers
    WHERE Balance >= @MinBalance
    ORDER BY CustomerName
    FOR XML RAW('customer'), ROOT('customers')
    
    DROP TABLE #Customers
END
