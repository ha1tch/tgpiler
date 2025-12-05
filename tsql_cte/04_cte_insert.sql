-- CTE with INSERT: Archive old orders
CREATE PROCEDURE ArchiveOldOrders
    @CutoffDate DATE,
    @RowsArchived INT OUTPUT
AS
BEGIN
    WITH OldOrders AS (
        SELECT 
            OrderID, 
            CustomerID, 
            Amount, 
            OrderDate,
            Status
        FROM Orders
        WHERE OrderDate < @CutoffDate
          AND Status = 'Completed'
    )
    INSERT INTO ArchivedOrders (OrderID, CustomerID, Amount, OrderDate, Status, ArchivedAt)
    SELECT 
        OrderID, 
        CustomerID, 
        Amount, 
        OrderDate, 
        Status,
        GETDATE()
    FROM OldOrders
    
    SET @RowsArchived = @@ROWCOUNT
END
