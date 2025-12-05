-- ============================================================================
-- ShopEasy Cart Service Stored Procedures
-- ============================================================================

-- ============================================================================
-- usp_GetCart
-- Retrieves a user's shopping cart
-- ============================================================================
CREATE PROCEDURE usp_GetCart
    @UserId BIGINT
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @CartId BIGINT;
    
    -- Get or create cart
    SELECT @CartId = Id FROM Carts WHERE UserId = @UserId;
    
    IF @CartId IS NULL
    BEGIN
        INSERT INTO Carts (UserId) VALUES (@UserId);
        SET @CartId = SCOPE_IDENTITY();
    END
    
    -- Return cart header
    SELECT 
        c.Id, c.UserId, c.CreatedAt, c.UpdatedAt,
        COUNT(ci.Id) AS ItemCount,
        COALESCE(SUM(ci.Quantity * p.PriceUnits), 0) AS SubtotalUnits,
        COALESCE(SUM(ci.Quantity * p.PriceNanos), 0) AS SubtotalNanos
    FROM Carts c
    LEFT JOIN CartItems ci ON c.Id = ci.CartId
    LEFT JOIN Products p ON ci.ProductId = p.Id
    WHERE c.Id = @CartId
    GROUP BY c.Id, c.UserId, c.CreatedAt, c.UpdatedAt;
    
    -- Return cart items
    SELECT 
        ci.Id, ci.ProductId, p.Name AS ProductName, p.Sku AS ProductSku,
        (SELECT TOP 1 ImageUrl FROM ProductImages WHERE ProductId = p.Id ORDER BY DisplayOrder) AS ProductImageUrl,
        ci.Quantity,
        p.PriceUnits AS UnitPriceUnits, p.PriceNanos AS UnitPriceNanos,
        ci.Quantity * p.PriceUnits AS SubtotalUnits,
        ci.Quantity * p.PriceNanos AS SubtotalNanos
    FROM CartItems ci
    INNER JOIN Products p ON ci.ProductId = p.Id
    WHERE ci.CartId = @CartId
    ORDER BY ci.AddedAt;
END
GO

-- ============================================================================
-- usp_AddToCart
-- Adds an item to the cart or increases quantity if already present
-- ============================================================================
CREATE PROCEDURE usp_AddToCart
    @UserId BIGINT,
    @ProductId BIGINT,
    @Quantity INT = 1
AS
BEGIN
    SET NOCOUNT ON;
    BEGIN TRANSACTION;
    
    BEGIN TRY
        -- Validate product exists and is active
        IF NOT EXISTS (SELECT 1 FROM Products WHERE Id = @ProductId AND IsActive = 1)
        BEGIN
            RAISERROR('Product not found or inactive', 16, 1);
            RETURN;
        END
        
        DECLARE @CartId BIGINT;
        
        -- Get or create cart
        SELECT @CartId = Id FROM Carts WHERE UserId = @UserId;
        
        IF @CartId IS NULL
        BEGIN
            INSERT INTO Carts (UserId) VALUES (@UserId);
            SET @CartId = SCOPE_IDENTITY();
        END
        
        -- Add or update item
        MERGE CartItems AS target
        USING (SELECT @CartId AS CartId, @ProductId AS ProductId, @Quantity AS Quantity) AS source
        ON target.CartId = source.CartId AND target.ProductId = source.ProductId
        WHEN MATCHED THEN
            UPDATE SET Quantity = target.Quantity + source.Quantity
        WHEN NOT MATCHED THEN
            INSERT (CartId, ProductId, Quantity)
            VALUES (source.CartId, source.ProductId, source.Quantity);
        
        -- Update cart timestamp
        UPDATE Carts SET UpdatedAt = GETUTCDATE() WHERE Id = @CartId;
        
        COMMIT TRANSACTION;
        
        -- Return updated cart
        EXEC usp_GetCart @UserId;
    END TRY
    BEGIN CATCH
        ROLLBACK TRANSACTION;
        THROW;
    END CATCH
END
GO

-- ============================================================================
-- usp_UpdateCartItem
-- Updates quantity of an item in the cart
-- ============================================================================
CREATE PROCEDURE usp_UpdateCartItem
    @UserId BIGINT,
    @ProductId BIGINT,
    @Quantity INT
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @CartId BIGINT;
    SELECT @CartId = Id FROM Carts WHERE UserId = @UserId;
    
    IF @CartId IS NULL
    BEGIN
        RAISERROR('Cart not found', 16, 1);
        RETURN;
    END
    
    IF @Quantity <= 0
    BEGIN
        -- Remove item
        DELETE FROM CartItems WHERE CartId = @CartId AND ProductId = @ProductId;
    END
    ELSE
    BEGIN
        -- Update quantity
        UPDATE CartItems
        SET Quantity = @Quantity
        WHERE CartId = @CartId AND ProductId = @ProductId;
    END
    
    -- Update cart timestamp
    UPDATE Carts SET UpdatedAt = GETUTCDATE() WHERE Id = @CartId;
    
    -- Return updated cart
    EXEC usp_GetCart @UserId;
END
GO

-- ============================================================================
-- usp_RemoveFromCart
-- Removes an item from the cart
-- ============================================================================
CREATE PROCEDURE usp_RemoveFromCart
    @UserId BIGINT,
    @ProductId BIGINT
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @CartId BIGINT;
    SELECT @CartId = Id FROM Carts WHERE UserId = @UserId;
    
    IF @CartId IS NOT NULL
    BEGIN
        DELETE FROM CartItems WHERE CartId = @CartId AND ProductId = @ProductId;
        UPDATE Carts SET UpdatedAt = GETUTCDATE() WHERE Id = @CartId;
    END
    
    -- Return updated cart
    EXEC usp_GetCart @UserId;
END
GO

-- ============================================================================
-- usp_ClearCart
-- Removes all items from the cart
-- ============================================================================
CREATE PROCEDURE usp_ClearCart
    @UserId BIGINT
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @CartId BIGINT;
    SELECT @CartId = Id FROM Carts WHERE UserId = @UserId;
    
    IF @CartId IS NOT NULL
    BEGIN
        DELETE FROM CartItems WHERE CartId = @CartId;
        UPDATE Carts SET UpdatedAt = GETUTCDATE() WHERE Id = @CartId;
    END
    
    SELECT 1 AS Success;
END
GO

-- ============================================================================
-- usp_ValidateCart
-- Validates cart items for checkout
-- ============================================================================
CREATE PROCEDURE usp_ValidateCart
    @UserId BIGINT
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @CartId BIGINT;
    SELECT @CartId = Id FROM Carts WHERE UserId = @UserId;
    
    IF @CartId IS NULL
    BEGIN
        SELECT 0 AS IsValid;
        RETURN;
    END
    
    -- Check for issues
    SELECT 
        ci.ProductId,
        p.Name AS ProductName,
        CASE 
            WHEN p.IsActive = 0 THEN 'unavailable'
            WHEN i.QuantityOnHand - i.QuantityReserved <= 0 THEN 'out_of_stock'
            WHEN ci.Quantity > (i.QuantityOnHand - i.QuantityReserved) THEN 'insufficient_stock'
            ELSE NULL
        END AS IssueType,
        CASE 
            WHEN p.IsActive = 0 THEN 'This product is no longer available'
            WHEN i.QuantityOnHand - i.QuantityReserved <= 0 THEN 'This product is out of stock'
            WHEN ci.Quantity > (i.QuantityOnHand - i.QuantityReserved) THEN 'Only ' + CAST(i.QuantityOnHand - i.QuantityReserved AS NVARCHAR) + ' available'
            ELSE NULL
        END AS Message,
        i.QuantityOnHand - i.QuantityReserved AS AvailableQuantity,
        p.PriceUnits AS CurrentPriceUnits,
        p.PriceNanos AS CurrentPriceNanos
    INTO #Issues
    FROM CartItems ci
    INNER JOIN Products p ON ci.ProductId = p.Id
    LEFT JOIN Inventory i ON p.Id = i.ProductId
    WHERE ci.CartId = @CartId
      AND (p.IsActive = 0 
           OR i.QuantityOnHand - i.QuantityReserved <= 0
           OR ci.Quantity > (i.QuantityOnHand - i.QuantityReserved));
    
    -- Return validation result
    IF EXISTS (SELECT 1 FROM #Issues WHERE IssueType IS NOT NULL)
    BEGIN
        SELECT 0 AS IsValid;
        SELECT * FROM #Issues WHERE IssueType IS NOT NULL;
    END
    ELSE
    BEGIN
        SELECT 1 AS IsValid;
    END
    
    DROP TABLE #Issues;
END
GO
