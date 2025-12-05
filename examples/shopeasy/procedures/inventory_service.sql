-- ============================================================================
-- ShopEasy Inventory Service Stored Procedures
-- ============================================================================

-- ============================================================================
-- usp_GetInventoryLevel
-- Retrieves inventory level for a product
-- ============================================================================
CREATE PROCEDURE usp_GetInventoryLevel
    @ProductId BIGINT
AS
BEGIN
    SET NOCOUNT ON;
    
    SELECT 
        i.ProductId, p.Sku AS ProductSku, p.Name AS ProductName,
        i.QuantityOnHand, i.QuantityReserved,
        i.QuantityOnHand - i.QuantityReserved AS QuantityAvailable,
        i.ReorderPoint, i.ReorderQuantity, i.UpdatedAt
    FROM Inventory i
    INNER JOIN Products p ON i.ProductId = p.Id
    WHERE i.ProductId = @ProductId;
END
GO

-- ============================================================================
-- usp_ListInventoryLevels
-- Lists inventory levels with filtering and pagination
-- ============================================================================
CREATE PROCEDURE usp_ListInventoryLevels
    @LowStockOnly BIT = 0,
    @OutOfStockOnly BIT = 0,
    @CategoryId BIGINT = NULL,
    @PageSize INT = 20,
    @PageToken NVARCHAR(100) = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @Offset INT = 0;
    IF @PageToken IS NOT NULL
        SET @Offset = TRY_CAST(@PageToken AS INT);
    
    -- Get total count
    SELECT COUNT(*) AS TotalCount
    FROM Inventory i
    INNER JOIN Products p ON i.ProductId = p.Id
    WHERE p.IsActive = 1
      AND p.TrackInventory = 1
      AND (@CategoryId IS NULL OR p.CategoryId = @CategoryId)
      AND (@LowStockOnly = 0 OR i.QuantityOnHand - i.QuantityReserved <= i.ReorderPoint)
      AND (@OutOfStockOnly = 0 OR i.QuantityOnHand - i.QuantityReserved <= 0);
    
    -- Get inventory levels
    SELECT 
        i.ProductId, p.Sku AS ProductSku, p.Name AS ProductName,
        i.QuantityOnHand, i.QuantityReserved,
        i.QuantityOnHand - i.QuantityReserved AS QuantityAvailable,
        i.ReorderPoint, i.ReorderQuantity, i.UpdatedAt
    FROM Inventory i
    INNER JOIN Products p ON i.ProductId = p.Id
    WHERE p.IsActive = 1
      AND p.TrackInventory = 1
      AND (@CategoryId IS NULL OR p.CategoryId = @CategoryId)
      AND (@LowStockOnly = 0 OR i.QuantityOnHand - i.QuantityReserved <= i.ReorderPoint)
      AND (@OutOfStockOnly = 0 OR i.QuantityOnHand - i.QuantityReserved <= 0)
    ORDER BY 
        CASE WHEN i.QuantityOnHand - i.QuantityReserved <= 0 THEN 0
             WHEN i.QuantityOnHand - i.QuantityReserved <= i.ReorderPoint THEN 1
             ELSE 2 END,
        p.Name
    OFFSET @Offset ROWS FETCH NEXT @PageSize ROWS ONLY;
END
GO

-- ============================================================================
-- usp_AdjustInventory
-- Adjusts inventory quantity with transaction logging
-- ============================================================================
CREATE PROCEDURE usp_AdjustInventory
    @ProductId BIGINT,
    @QuantityChange INT,
    @Reason NVARCHAR(50),  -- RECEIVED, ADJUSTMENT, DAMAGED, RETURNED
    @Reference NVARCHAR(100) = NULL,
    @Notes NVARCHAR(MAX) = NULL,
    @AdjustedBy BIGINT
AS
BEGIN
    SET NOCOUNT ON;
    BEGIN TRANSACTION;
    
    BEGIN TRY
        DECLARE @QuantityBefore INT;
        DECLARE @QuantityAfter INT;
        
        SELECT @QuantityBefore = QuantityOnHand - QuantityReserved
        FROM Inventory
        WHERE ProductId = @ProductId;
        
        IF @QuantityBefore IS NULL
        BEGIN
            RAISERROR('Product inventory not found', 16, 1);
            RETURN;
        END
        
        SET @QuantityAfter = @QuantityBefore + @QuantityChange;
        
        IF @QuantityAfter < 0
        BEGIN
            RAISERROR('Insufficient inventory', 16, 1);
            RETURN;
        END
        
        -- Update inventory
        UPDATE Inventory
        SET QuantityOnHand = QuantityOnHand + @QuantityChange,
            UpdatedAt = GETUTCDATE()
        WHERE ProductId = @ProductId;
        
        -- Log transaction
        INSERT INTO InventoryTransactions (
            ProductId, TransactionType, Quantity,
            QuantityBefore, QuantityAfter, Reference, Notes, CreatedBy
        )
        VALUES (
            @ProductId, @Reason, ABS(@QuantityChange),
            @QuantityBefore, @QuantityAfter, @Reference, @Notes, @AdjustedBy
        );
        
        COMMIT TRANSACTION;
        
        -- Return results
        EXEC usp_GetInventoryLevel @ProductId;
        
        SELECT TOP 1 * FROM InventoryTransactions
        WHERE ProductId = @ProductId
        ORDER BY Id DESC;
    END TRY
    BEGIN CATCH
        ROLLBACK TRANSACTION;
        THROW;
    END CATCH
END
GO

-- ============================================================================
-- usp_ReserveInventory
-- Reserves inventory for an order
-- ============================================================================
CREATE PROCEDURE usp_ReserveInventory
    @OrderId BIGINT,
    @Items NVARCHAR(MAX)  -- JSON array: [{"ProductId": 1, "Quantity": 2}, ...]
AS
BEGIN
    SET NOCOUNT ON;
    BEGIN TRANSACTION;
    
    BEGIN TRY
        -- Parse JSON items
        DECLARE @ItemTable TABLE (
            ProductId BIGINT,
            Quantity INT
        );
        
        INSERT INTO @ItemTable (ProductId, Quantity)
        SELECT 
            JSON_VALUE(value, '$.ProductId'),
            JSON_VALUE(value, '$.Quantity')
        FROM OPENJSON(@Items);
        
        -- Check availability
        SELECT 
            it.ProductId,
            CASE 
                WHEN i.ProductId IS NULL THEN 0
                WHEN i.QuantityOnHand - i.QuantityReserved >= it.Quantity THEN 1
                ELSE 0
            END AS Reserved,
            it.Quantity AS QuantityRequested,
            CASE 
                WHEN i.ProductId IS NULL THEN 0
                WHEN i.QuantityOnHand - i.QuantityReserved >= it.Quantity THEN it.Quantity
                ELSE i.QuantityOnHand - i.QuantityReserved
            END AS QuantityReserved,
            CASE 
                WHEN i.ProductId IS NULL THEN 'Product not found'
                WHEN i.QuantityOnHand - i.QuantityReserved < it.Quantity THEN 'Insufficient stock'
                ELSE NULL
            END AS Error
        INTO #Results
        FROM @ItemTable it
        LEFT JOIN Inventory i ON it.ProductId = i.ProductId;
        
        -- Check if all items can be reserved
        IF EXISTS (SELECT 1 FROM #Results WHERE Reserved = 0)
        BEGIN
            SELECT 0 AS Success;
            SELECT * FROM #Results;
            DROP TABLE #Results;
            ROLLBACK TRANSACTION;
            RETURN;
        END
        
        -- Reserve inventory
        UPDATE i
        SET QuantityReserved = i.QuantityReserved + it.Quantity,
            UpdatedAt = GETUTCDATE()
        FROM Inventory i
        INNER JOIN @ItemTable it ON i.ProductId = it.ProductId;
        
        -- Log transactions
        INSERT INTO InventoryTransactions (
            ProductId, TransactionType, Quantity,
            QuantityBefore, QuantityAfter, OrderId, CreatedBy
        )
        SELECT 
            it.ProductId, 'RESERVED', it.Quantity,
            i.QuantityOnHand - i.QuantityReserved + it.Quantity,
            i.QuantityOnHand - i.QuantityReserved,
            @OrderId, 0
        FROM @ItemTable it
        INNER JOIN Inventory i ON it.ProductId = i.ProductId;
        
        COMMIT TRANSACTION;
        
        SELECT 1 AS Success;
        SELECT * FROM #Results;
        DROP TABLE #Results;
    END TRY
    BEGIN CATCH
        ROLLBACK TRANSACTION;
        THROW;
    END CATCH
END
GO

-- ============================================================================
-- usp_ReleaseInventory
-- Releases reserved inventory for a cancelled order
-- ============================================================================
CREATE PROCEDURE usp_ReleaseInventory
    @OrderId BIGINT
AS
BEGIN
    SET NOCOUNT ON;
    BEGIN TRANSACTION;
    
    BEGIN TRY
        DECLARE @ItemsReleased INT;
        
        -- Get items to release
        SELECT oi.ProductId, oi.Quantity
        INTO #ItemsToRelease
        FROM OrderItems oi
        WHERE oi.OrderId = @OrderId;
        
        SET @ItemsReleased = @@ROWCOUNT;
        
        -- Release reserved quantities
        UPDATE i
        SET QuantityReserved = i.QuantityReserved - r.Quantity,
            UpdatedAt = GETUTCDATE()
        FROM Inventory i
        INNER JOIN #ItemsToRelease r ON i.ProductId = r.ProductId;
        
        -- Log transactions
        INSERT INTO InventoryTransactions (
            ProductId, TransactionType, Quantity,
            QuantityBefore, QuantityAfter, OrderId, CreatedBy
        )
        SELECT 
            r.ProductId, 'RELEASED', r.Quantity,
            i.QuantityOnHand - i.QuantityReserved - r.Quantity,
            i.QuantityOnHand - i.QuantityReserved,
            @OrderId, 0
        FROM #ItemsToRelease r
        INNER JOIN Inventory i ON r.ProductId = i.ProductId;
        
        DROP TABLE #ItemsToRelease;
        
        COMMIT TRANSACTION;
        
        SELECT 1 AS Success, @ItemsReleased AS ItemsReleased;
    END TRY
    BEGIN CATCH
        IF OBJECT_ID('tempdb..#ItemsToRelease') IS NOT NULL
            DROP TABLE #ItemsToRelease;
        ROLLBACK TRANSACTION;
        THROW;
    END CATCH
END
GO

-- ============================================================================
-- usp_SetReorderPoint
-- Sets reorder point and quantity for a product
-- ============================================================================
CREATE PROCEDURE usp_SetReorderPoint
    @ProductId BIGINT,
    @ReorderPoint INT,
    @ReorderQuantity INT
AS
BEGIN
    SET NOCOUNT ON;
    
    UPDATE Inventory
    SET ReorderPoint = @ReorderPoint,
        ReorderQuantity = @ReorderQuantity,
        UpdatedAt = GETUTCDATE()
    WHERE ProductId = @ProductId;
    
    EXEC usp_GetInventoryLevel @ProductId;
END
GO

-- ============================================================================
-- usp_ListInventoryTransactions
-- Lists inventory transactions with filtering
-- ============================================================================
CREATE PROCEDURE usp_ListInventoryTransactions
    @ProductId BIGINT = NULL,
    @OrderId BIGINT = NULL,
    @FromDate DATETIME2 = NULL,
    @ToDate DATETIME2 = NULL,
    @PageSize INT = 20,
    @PageToken NVARCHAR(100) = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @Offset INT = 0;
    IF @PageToken IS NOT NULL
        SET @Offset = TRY_CAST(@PageToken AS INT);
    
    -- Get total count
    SELECT COUNT(*) AS TotalCount
    FROM InventoryTransactions
    WHERE (@ProductId IS NULL OR ProductId = @ProductId)
      AND (@OrderId IS NULL OR OrderId = @OrderId)
      AND (@FromDate IS NULL OR CreatedAt >= @FromDate)
      AND (@ToDate IS NULL OR CreatedAt <= @ToDate);
    
    -- Get transactions
    SELECT 
        it.Id, it.ProductId, p.Sku AS ProductSku,
        it.TransactionType, it.Quantity,
        it.QuantityBefore, it.QuantityAfter,
        it.OrderId, it.Reference, it.Notes,
        it.CreatedBy, it.CreatedAt
    FROM InventoryTransactions it
    INNER JOIN Products p ON it.ProductId = p.Id
    WHERE (@ProductId IS NULL OR it.ProductId = @ProductId)
      AND (@OrderId IS NULL OR it.OrderId = @OrderId)
      AND (@FromDate IS NULL OR it.CreatedAt >= @FromDate)
      AND (@ToDate IS NULL OR it.CreatedAt <= @ToDate)
    ORDER BY it.CreatedAt DESC
    OFFSET @Offset ROWS FETCH NEXT @PageSize ROWS ONLY;
END
GO

-- ============================================================================
-- usp_GetLowStockProducts
-- Returns products at or below reorder threshold
-- ============================================================================
CREATE PROCEDURE usp_GetLowStockProducts
    @ThresholdPercentage INT = 100  -- 100 = at reorder point, 50 = at 50% of reorder point
AS
BEGIN
    SET NOCOUNT ON;
    
    SELECT 
        i.ProductId, p.Sku AS ProductSku, p.Name AS ProductName,
        i.QuantityOnHand, i.QuantityReserved,
        i.QuantityOnHand - i.QuantityReserved AS QuantityAvailable,
        i.ReorderPoint, i.ReorderQuantity, i.UpdatedAt
    FROM Inventory i
    INNER JOIN Products p ON i.ProductId = p.Id
    WHERE p.IsActive = 1
      AND p.TrackInventory = 1
      AND (i.QuantityOnHand - i.QuantityReserved) <= (i.ReorderPoint * @ThresholdPercentage / 100)
    ORDER BY 
        CAST(i.QuantityOnHand - i.QuantityReserved AS FLOAT) / NULLIF(i.ReorderPoint, 0) ASC,
        p.Name;
END
GO
