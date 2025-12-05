-- ============================================================================
-- ShopEasy Order Service Stored Procedures
-- ============================================================================

-- ============================================================================
-- usp_CreateOrder
-- Creates a new order from the user's cart
-- ============================================================================
CREATE PROCEDURE usp_CreateOrder
    @UserId BIGINT,
    @ShippingAddressLine1 NVARCHAR(255),
    @ShippingAddressLine2 NVARCHAR(255) = NULL,
    @ShippingCity NVARCHAR(100),
    @ShippingState NVARCHAR(100),
    @ShippingPostalCode NVARCHAR(20),
    @ShippingCountry NVARCHAR(100),
    @BillingAddressLine1 NVARCHAR(255),
    @BillingAddressLine2 NVARCHAR(255) = NULL,
    @BillingCity NVARCHAR(100),
    @BillingState NVARCHAR(100),
    @BillingPostalCode NVARCHAR(20),
    @BillingCountry NVARCHAR(100),
    @DiscountCode NVARCHAR(50) = NULL,
    @Notes NVARCHAR(MAX) = NULL,
    @PaymentMethodId NVARCHAR(255)
AS
BEGIN
    SET NOCOUNT ON;
    BEGIN TRANSACTION;
    
    BEGIN TRY
        DECLARE @CartId BIGINT;
        DECLARE @OrderId BIGINT;
        DECLARE @OrderNumber NVARCHAR(50);
        DECLARE @SubtotalUnits BIGINT = 0;
        DECLARE @TaxAmountUnits BIGINT = 0;
        DECLARE @ShippingAmountUnits BIGINT = 1000; -- $10 flat rate
        DECLARE @DiscountAmountUnits BIGINT = 0;
        
        -- Get cart
        SELECT @CartId = Id FROM Carts WHERE UserId = @UserId;
        
        IF @CartId IS NULL OR NOT EXISTS (SELECT 1 FROM CartItems WHERE CartId = @CartId)
        BEGIN
            RAISERROR('Cart is empty', 16, 1);
            RETURN;
        END
        
        -- Calculate subtotal
        SELECT @SubtotalUnits = SUM(ci.Quantity * p.PriceUnits)
        FROM CartItems ci
        INNER JOIN Products p ON ci.ProductId = p.Id
        WHERE ci.CartId = @CartId;
        
        -- Apply discount code if provided
        IF @DiscountCode IS NOT NULL
        BEGIN
            DECLARE @DiscountType NVARCHAR(20);
            DECLARE @DiscountValue DECIMAL(10,2);
            
            SELECT @DiscountType = DiscountType, @DiscountValue = DiscountValue
            FROM DiscountCodes
            WHERE Code = @DiscountCode
              AND IsActive = 1
              AND (StartsAt IS NULL OR StartsAt <= GETUTCDATE())
              AND (ExpiresAt IS NULL OR ExpiresAt > GETUTCDATE())
              AND (UsageLimit IS NULL OR UsageCount < UsageLimit);
            
            IF @DiscountType = 'PERCENTAGE'
                SET @DiscountAmountUnits = @SubtotalUnits * @DiscountValue / 100;
            ELSE IF @DiscountType = 'FIXED'
                SET @DiscountAmountUnits = CAST(@DiscountValue * 100 AS BIGINT);
        END
        
        -- Calculate tax (simplified: 8% of subtotal)
        SET @TaxAmountUnits = (@SubtotalUnits - @DiscountAmountUnits) * 8 / 100;
        
        -- Generate order number
        SET @OrderNumber = 'ORD-' + FORMAT(GETUTCDATE(), 'yyyyMMdd') + '-' + RIGHT('000000' + CAST(NEXT VALUE FOR OrderNumberSeq AS NVARCHAR), 6);
        
        -- Create order
        INSERT INTO Orders (
            OrderNumber, UserId, Status, PaymentStatus,
            ShippingAddressLine1, ShippingAddressLine2, ShippingCity, ShippingState, ShippingPostalCode, ShippingCountry,
            BillingAddressLine1, BillingAddressLine2, BillingCity, BillingState, BillingPostalCode, BillingCountry,
            SubtotalUnits, TaxAmountUnits, ShippingAmountUnits, DiscountAmountUnits,
            TotalUnits, DiscountCode, Notes
        )
        VALUES (
            @OrderNumber, @UserId, 'PENDING', 'PENDING',
            @ShippingAddressLine1, @ShippingAddressLine2, @ShippingCity, @ShippingState, @ShippingPostalCode, @ShippingCountry,
            @BillingAddressLine1, @BillingAddressLine2, @BillingCity, @BillingState, @BillingPostalCode, @BillingCountry,
            @SubtotalUnits, @TaxAmountUnits, @ShippingAmountUnits, @DiscountAmountUnits,
            @SubtotalUnits + @TaxAmountUnits + @ShippingAmountUnits - @DiscountAmountUnits,
            @DiscountCode, @Notes
        );
        
        SET @OrderId = SCOPE_IDENTITY();
        
        -- Create order items from cart
        INSERT INTO OrderItems (
            OrderId, ProductId, ProductName, ProductSku, Quantity,
            UnitPriceUnits, SubtotalUnits, TaxAmountUnits, TotalUnits
        )
        SELECT 
            @OrderId, ci.ProductId, p.Name, p.Sku, ci.Quantity,
            p.PriceUnits,
            ci.Quantity * p.PriceUnits,
            ci.Quantity * p.PriceUnits * 8 / 100,
            ci.Quantity * p.PriceUnits + ci.Quantity * p.PriceUnits * 8 / 100
        FROM CartItems ci
        INNER JOIN Products p ON ci.ProductId = p.Id
        WHERE ci.CartId = @CartId;
        
        -- Reserve inventory
        UPDATE i
        SET QuantityReserved = i.QuantityReserved + ci.Quantity,
            UpdatedAt = GETUTCDATE()
        FROM Inventory i
        INNER JOIN CartItems ci ON i.ProductId = ci.ProductId
        WHERE ci.CartId = @CartId;
        
        -- Log inventory transactions
        INSERT INTO InventoryTransactions (
            ProductId, TransactionType, Quantity,
            QuantityBefore, QuantityAfter, OrderId, CreatedBy
        )
        SELECT 
            ci.ProductId, 'RESERVED', ci.Quantity,
            i.QuantityOnHand - i.QuantityReserved + ci.Quantity,
            i.QuantityOnHand - i.QuantityReserved,
            @OrderId, @UserId
        FROM CartItems ci
        INNER JOIN Inventory i ON ci.ProductId = i.ProductId
        WHERE ci.CartId = @CartId;
        
        -- Record order status history
        INSERT INTO OrderStatusHistory (OrderId, Status, CreatedBy)
        VALUES (@OrderId, 'PENDING', @UserId);
        
        -- Update discount code usage
        IF @DiscountCode IS NOT NULL
        BEGIN
            UPDATE DiscountCodes
            SET UsageCount = UsageCount + 1
            WHERE Code = @DiscountCode;
        END
        
        -- Clear cart
        DELETE FROM CartItems WHERE CartId = @CartId;
        
        COMMIT TRANSACTION;
        
        -- Return order
        EXEC usp_GetOrderById @OrderId;
    END TRY
    BEGIN CATCH
        ROLLBACK TRANSACTION;
        THROW;
    END CATCH
END
GO

-- Create sequence for order numbers (if not exists)
IF NOT EXISTS (SELECT * FROM sys.sequences WHERE name = 'OrderNumberSeq')
    CREATE SEQUENCE OrderNumberSeq START WITH 1 INCREMENT BY 1;
GO

-- ============================================================================
-- usp_GetOrderById
-- Retrieves an order by ID
-- ============================================================================
CREATE PROCEDURE usp_GetOrderById
    @OrderId BIGINT
AS
BEGIN
    SET NOCOUNT ON;
    
    -- Return order header
    SELECT 
        Id, OrderNumber, UserId, Status, PaymentStatus,
        ShippingAddressLine1, ShippingAddressLine2, ShippingCity, 
        ShippingState, ShippingPostalCode, ShippingCountry,
        BillingAddressLine1, BillingAddressLine2, BillingCity,
        BillingState, BillingPostalCode, BillingCountry,
        SubtotalUnits, SubtotalNanos, TaxAmountUnits, TaxAmountNanos,
        ShippingAmountUnits, ShippingAmountNanos, DiscountAmountUnits, DiscountAmountNanos,
        TotalUnits, TotalNanos, Currency, DiscountCode, Notes,
        TrackingNumber, Carrier, PaymentIntentId,
        CreatedAt, UpdatedAt, ShippedAt, DeliveredAt
    FROM Orders
    WHERE Id = @OrderId;
    
    -- Return order items
    SELECT 
        Id, ProductId, ProductName, ProductSku, Quantity,
        UnitPriceUnits, UnitPriceNanos, SubtotalUnits, SubtotalNanos,
        TaxAmountUnits, TaxAmountNanos, TotalUnits, TotalNanos
    FROM OrderItems
    WHERE OrderId = @OrderId;
END
GO

-- ============================================================================
-- usp_GetOrderByNumber
-- Retrieves an order by order number
-- ============================================================================
CREATE PROCEDURE usp_GetOrderByNumber
    @OrderNumber NVARCHAR(50)
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @OrderId BIGINT;
    SELECT @OrderId = Id FROM Orders WHERE OrderNumber = @OrderNumber;
    
    IF @OrderId IS NOT NULL
        EXEC usp_GetOrderById @OrderId;
END
GO

-- ============================================================================
-- usp_ListOrders
-- Lists orders for a user with filtering and pagination
-- ============================================================================
CREATE PROCEDURE usp_ListOrders
    @UserId BIGINT,
    @Status NVARCHAR(50) = NULL,
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
    FROM Orders
    WHERE UserId = @UserId
      AND (@Status IS NULL OR Status = @Status)
      AND (@FromDate IS NULL OR CreatedAt >= @FromDate)
      AND (@ToDate IS NULL OR CreatedAt <= @ToDate);
    
    -- Get orders
    SELECT 
        Id, OrderNumber, UserId, Status, PaymentStatus,
        SubtotalUnits, TaxAmountUnits, ShippingAmountUnits, 
        DiscountAmountUnits, TotalUnits, Currency,
        TrackingNumber, Carrier,
        CreatedAt, UpdatedAt, ShippedAt, DeliveredAt
    FROM Orders
    WHERE UserId = @UserId
      AND (@Status IS NULL OR Status = @Status)
      AND (@FromDate IS NULL OR CreatedAt >= @FromDate)
      AND (@ToDate IS NULL OR CreatedAt <= @ToDate)
    ORDER BY CreatedAt DESC
    OFFSET @Offset ROWS FETCH NEXT @PageSize ROWS ONLY;
END
GO

-- ============================================================================
-- usp_UpdateOrderStatus
-- Updates the status of an order
-- ============================================================================
CREATE PROCEDURE usp_UpdateOrderStatus
    @OrderId BIGINT,
    @Status NVARCHAR(50),
    @TrackingNumber NVARCHAR(100) = NULL,
    @Carrier NVARCHAR(100) = NULL,
    @Notes NVARCHAR(MAX) = NULL,
    @UpdatedBy BIGINT = NULL
AS
BEGIN
    SET NOCOUNT ON;
    BEGIN TRANSACTION;
    
    BEGIN TRY
        DECLARE @OldStatus NVARCHAR(50);
        SELECT @OldStatus = Status FROM Orders WHERE Id = @OrderId;
        
        -- Update order
        UPDATE Orders
        SET Status = @Status,
            TrackingNumber = COALESCE(@TrackingNumber, TrackingNumber),
            Carrier = COALESCE(@Carrier, Carrier),
            ShippedAt = CASE WHEN @Status = 'SHIPPED' AND ShippedAt IS NULL THEN GETUTCDATE() ELSE ShippedAt END,
            DeliveredAt = CASE WHEN @Status = 'DELIVERED' AND DeliveredAt IS NULL THEN GETUTCDATE() ELSE DeliveredAt END,
            UpdatedAt = GETUTCDATE()
        WHERE Id = @OrderId;
        
        -- If confirmed, convert reserved to sold
        IF @Status = 'CONFIRMED' AND @OldStatus = 'PENDING'
        BEGIN
            UPDATE i
            SET QuantityOnHand = i.QuantityOnHand - oi.Quantity,
                QuantityReserved = i.QuantityReserved - oi.Quantity,
                UpdatedAt = GETUTCDATE()
            FROM Inventory i
            INNER JOIN OrderItems oi ON i.ProductId = oi.ProductId
            WHERE oi.OrderId = @OrderId;
            
            INSERT INTO InventoryTransactions (
                ProductId, TransactionType, Quantity,
                QuantityBefore, QuantityAfter, OrderId, CreatedBy
            )
            SELECT 
                oi.ProductId, 'SOLD', oi.Quantity,
                i.QuantityOnHand + oi.Quantity,
                i.QuantityOnHand,
                @OrderId, @UpdatedBy
            FROM OrderItems oi
            INNER JOIN Inventory i ON oi.ProductId = i.ProductId
            WHERE oi.OrderId = @OrderId;
        END
        
        -- Record status history
        INSERT INTO OrderStatusHistory (OrderId, Status, Notes, CreatedBy)
        VALUES (@OrderId, @Status, @Notes, @UpdatedBy);
        
        COMMIT TRANSACTION;
        
        EXEC usp_GetOrderById @OrderId;
    END TRY
    BEGIN CATCH
        ROLLBACK TRANSACTION;
        THROW;
    END CATCH
END
GO

-- ============================================================================
-- usp_CancelOrder
-- Cancels an order and releases reserved inventory
-- ============================================================================
CREATE PROCEDURE usp_CancelOrder
    @OrderId BIGINT,
    @Reason NVARCHAR(MAX),
    @CancelledBy BIGINT = NULL
AS
BEGIN
    SET NOCOUNT ON;
    BEGIN TRANSACTION;
    
    BEGIN TRY
        DECLARE @Status NVARCHAR(50);
        DECLARE @PaymentStatus NVARCHAR(50);
        
        SELECT @Status = Status, @PaymentStatus = PaymentStatus
        FROM Orders WHERE Id = @OrderId;
        
        IF @Status IN ('SHIPPED', 'DELIVERED', 'CANCELLED', 'REFUNDED')
        BEGIN
            RAISERROR('Order cannot be cancelled in current status', 16, 1);
            RETURN;
        END
        
        -- Release reserved inventory if still pending
        IF @Status = 'PENDING'
        BEGIN
            UPDATE i
            SET QuantityReserved = i.QuantityReserved - oi.Quantity,
                UpdatedAt = GETUTCDATE()
            FROM Inventory i
            INNER JOIN OrderItems oi ON i.ProductId = oi.ProductId
            WHERE oi.OrderId = @OrderId;
            
            INSERT INTO InventoryTransactions (
                ProductId, TransactionType, Quantity,
                QuantityBefore, QuantityAfter, OrderId, CreatedBy
            )
            SELECT 
                oi.ProductId, 'RELEASED', oi.Quantity,
                i.QuantityOnHand - i.QuantityReserved - oi.Quantity,
                i.QuantityOnHand - i.QuantityReserved,
                @OrderId, @CancelledBy
            FROM OrderItems oi
            INNER JOIN Inventory i ON oi.ProductId = i.ProductId
            WHERE oi.OrderId = @OrderId;
        END
        
        -- Update order
        UPDATE Orders
        SET Status = 'CANCELLED',
            CancelledAt = GETUTCDATE(),
            CancellationReason = @Reason,
            UpdatedAt = GETUTCDATE()
        WHERE Id = @OrderId;
        
        -- Record status history
        INSERT INTO OrderStatusHistory (OrderId, Status, Notes, CreatedBy)
        VALUES (@OrderId, 'CANCELLED', @Reason, @CancelledBy);
        
        COMMIT TRANSACTION;
        
        -- Return refund status
        SELECT 
            o.Id, o.Status, 
            CASE WHEN @PaymentStatus = 'CAPTURED' THEN 1 ELSE 0 END AS RefundInitiated
        FROM Orders o
        WHERE o.Id = @OrderId;
    END TRY
    BEGIN CATCH
        ROLLBACK TRANSACTION;
        THROW;
    END CATCH
END
GO

-- ============================================================================
-- usp_ValidateDiscountCode
-- Validates a discount code for use
-- ============================================================================
CREATE PROCEDURE usp_ValidateDiscountCode
    @Code NVARCHAR(50),
    @UserId BIGINT,
    @CartSubtotalUnits BIGINT
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @DiscountId BIGINT;
    DECLARE @DiscountType NVARCHAR(20);
    DECLARE @DiscountValue DECIMAL(10,2);
    DECLARE @MinimumOrderUnits BIGINT;
    DECLARE @MaximumDiscountUnits BIGINT;
    DECLARE @DiscountAmountUnits BIGINT;
    
    SELECT 
        @DiscountId = Id,
        @DiscountType = DiscountType,
        @DiscountValue = DiscountValue,
        @MinimumOrderUnits = MinimumOrderUnits,
        @MaximumDiscountUnits = MaximumDiscountUnits
    FROM DiscountCodes
    WHERE Code = @Code
      AND IsActive = 1
      AND (StartsAt IS NULL OR StartsAt <= GETUTCDATE())
      AND (ExpiresAt IS NULL OR ExpiresAt > GETUTCDATE())
      AND (UsageLimit IS NULL OR UsageCount < UsageLimit);
    
    IF @DiscountId IS NULL
    BEGIN
        SELECT 0 AS IsValid, 'Invalid or expired discount code' AS ErrorMessage;
        RETURN;
    END
    
    IF @MinimumOrderUnits IS NOT NULL AND @CartSubtotalUnits < @MinimumOrderUnits
    BEGIN
        SELECT 0 AS IsValid, 
               'Minimum order of $' + CAST(@MinimumOrderUnits / 100.0 AS NVARCHAR) + ' required' AS ErrorMessage;
        RETURN;
    END
    
    -- Calculate discount
    IF @DiscountType = 'PERCENTAGE'
        SET @DiscountAmountUnits = @CartSubtotalUnits * @DiscountValue / 100;
    ELSE
        SET @DiscountAmountUnits = CAST(@DiscountValue * 100 AS BIGINT);
    
    -- Apply maximum cap
    IF @MaximumDiscountUnits IS NOT NULL AND @DiscountAmountUnits > @MaximumDiscountUnits
        SET @DiscountAmountUnits = @MaximumDiscountUnits;
    
    SELECT 
        1 AS IsValid,
        NULL AS ErrorMessage,
        @DiscountAmountUnits AS DiscountAmountUnits,
        0 AS DiscountAmountNanos,
        @DiscountType AS DiscountType;
END
GO

-- ============================================================================
-- usp_ProcessPayment
-- Records a payment transaction
-- ============================================================================
CREATE PROCEDURE usp_ProcessPayment
    @OrderId BIGINT,
    @PaymentIntentId NVARCHAR(255),
    @Success BIT,
    @ErrorMessage NVARCHAR(MAX) = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @AmountUnits BIGINT;
    SELECT @AmountUnits = TotalUnits FROM Orders WHERE Id = @OrderId;
    
    INSERT INTO PaymentTransactions (
        OrderId, TransactionType, Status, AmountUnits,
        PaymentIntentId, ErrorMessage
    )
    VALUES (
        @OrderId, 'CAPTURE', 
        CASE WHEN @Success = 1 THEN 'SUCCESS' ELSE 'FAILED' END,
        @AmountUnits, @PaymentIntentId, @ErrorMessage
    );
    
    IF @Success = 1
    BEGIN
        UPDATE Orders
        SET PaymentStatus = 'CAPTURED',
            PaymentIntentId = @PaymentIntentId,
            UpdatedAt = GETUTCDATE()
        WHERE Id = @OrderId;
    END
    ELSE
    BEGIN
        UPDATE Orders
        SET PaymentStatus = 'FAILED',
            UpdatedAt = GETUTCDATE()
        WHERE Id = @OrderId;
    END
    
    SELECT @Success AS Success, 
           CASE WHEN @Success = 1 THEN 'CAPTURED' ELSE 'FAILED' END AS Status,
           @ErrorMessage AS ErrorMessage;
END
GO

-- ============================================================================
-- usp_RefundOrder
-- Processes a refund for an order
-- ============================================================================
CREATE PROCEDURE usp_RefundOrder
    @OrderId BIGINT,
    @AmountUnits BIGINT,
    @Reason NVARCHAR(MAX),
    @RefundedBy BIGINT
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @RefundId NVARCHAR(255) = 'REF-' + CAST(NEWID() AS NVARCHAR(36));
    
    INSERT INTO Refunds (
        OrderId, RefundId, AmountUnits, Reason, Status, CreatedBy
    )
    VALUES (
        @OrderId, @RefundId, @AmountUnits, @Reason, 'PENDING', @RefundedBy
    );
    
    INSERT INTO PaymentTransactions (
        OrderId, TransactionType, Status, AmountUnits
    )
    VALUES (
        @OrderId, 'REFUND', 'SUCCESS', @AmountUnits
    );
    
    SELECT 1 AS Success, @RefundId AS RefundId;
END
GO
