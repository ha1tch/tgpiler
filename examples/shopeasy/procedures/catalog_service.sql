-- ============================================================================
-- ShopEasy Catalog Service Stored Procedures
-- ============================================================================

-- ============================================================================
-- CATEGORY PROCEDURES
-- ============================================================================

-- ============================================================================
-- usp_GetCategoryById
-- Retrieves a category by ID
-- ============================================================================
CREATE PROCEDURE usp_GetCategoryById
    @CategoryId BIGINT
AS
BEGIN
    SET NOCOUNT ON;
    
    SELECT 
        Id, Name, Slug, Description, ParentId,
        DisplayOrder, IsActive, ImageUrl, CreatedAt
    FROM Categories
    WHERE Id = @CategoryId;
END
GO

-- ============================================================================
-- usp_GetCategoryBySlug
-- Retrieves a category by slug
-- ============================================================================
CREATE PROCEDURE usp_GetCategoryBySlug
    @Slug NVARCHAR(100)
AS
BEGIN
    SET NOCOUNT ON;
    
    SELECT 
        Id, Name, Slug, Description, ParentId,
        DisplayOrder, IsActive, ImageUrl, CreatedAt
    FROM Categories
    WHERE Slug = @Slug;
END
GO

-- ============================================================================
-- usp_ListCategories
-- Lists categories with optional parent filter
-- ============================================================================
CREATE PROCEDURE usp_ListCategories
    @ParentId BIGINT = NULL,
    @IncludeInactive BIT = 0
AS
BEGIN
    SET NOCOUNT ON;
    
    SELECT 
        Id, Name, Slug, Description, ParentId,
        DisplayOrder, IsActive, ImageUrl, CreatedAt
    FROM Categories
    WHERE (@ParentId IS NULL AND ParentId IS NULL)
       OR (@ParentId IS NOT NULL AND ParentId = @ParentId)
    AND (@IncludeInactive = 1 OR IsActive = 1)
    ORDER BY DisplayOrder, Name;
END
GO

-- ============================================================================
-- usp_CreateCategory
-- Creates a new category
-- ============================================================================
CREATE PROCEDURE usp_CreateCategory
    @Name NVARCHAR(100),
    @Slug NVARCHAR(100),
    @Description NVARCHAR(MAX) = NULL,
    @ParentId BIGINT = NULL,
    @DisplayOrder INT = 0,
    @ImageUrl NVARCHAR(500) = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    -- Check for duplicate slug
    IF EXISTS (SELECT 1 FROM Categories WHERE Slug = @Slug)
    BEGIN
        RAISERROR('Category slug already exists', 16, 1);
        RETURN;
    END
    
    INSERT INTO Categories (Name, Slug, Description, ParentId, DisplayOrder, ImageUrl)
    VALUES (@Name, @Slug, @Description, @ParentId, @DisplayOrder, @ImageUrl);
    
    SELECT 
        Id, Name, Slug, Description, ParentId,
        DisplayOrder, IsActive, ImageUrl, CreatedAt
    FROM Categories
    WHERE Id = SCOPE_IDENTITY();
END
GO

-- ============================================================================
-- usp_UpdateCategory
-- Updates an existing category
-- ============================================================================
CREATE PROCEDURE usp_UpdateCategory
    @CategoryId BIGINT,
    @Name NVARCHAR(100) = NULL,
    @Description NVARCHAR(MAX) = NULL,
    @DisplayOrder INT = NULL,
    @IsActive BIT = NULL,
    @ImageUrl NVARCHAR(500) = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    UPDATE Categories
    SET Name = COALESCE(@Name, Name),
        Description = COALESCE(@Description, Description),
        DisplayOrder = COALESCE(@DisplayOrder, DisplayOrder),
        IsActive = COALESCE(@IsActive, IsActive),
        ImageUrl = COALESCE(@ImageUrl, ImageUrl),
        UpdatedAt = GETUTCDATE()
    WHERE Id = @CategoryId;
    
    SELECT 
        Id, Name, Slug, Description, ParentId,
        DisplayOrder, IsActive, ImageUrl, CreatedAt
    FROM Categories
    WHERE Id = @CategoryId;
END
GO

-- ============================================================================
-- PRODUCT PROCEDURES
-- ============================================================================

-- ============================================================================
-- usp_GetProductById
-- Retrieves a product by ID with category info
-- ============================================================================
CREATE PROCEDURE usp_GetProductById
    @ProductId BIGINT
AS
BEGIN
    SET NOCOUNT ON;
    
    SELECT 
        p.Id, p.Sku, p.Name, p.Slug, p.Description,
        p.PriceUnits, p.PriceNanos, p.PriceCurrency,
        p.CompareAtPriceUnits, p.CompareAtPriceNanos,
        p.CostPriceUnits, p.CostPriceNanos,
        p.CategoryId, c.Name AS CategoryName,
        p.IsActive, p.TrackInventory,
        COALESCE(i.QuantityOnHand - i.QuantityReserved, 0) AS StockQuantity,
        p.AverageRating, p.ReviewCount,
        p.CreatedAt, p.UpdatedAt
    FROM Products p
    INNER JOIN Categories c ON p.CategoryId = c.Id
    LEFT JOIN Inventory i ON p.Id = i.ProductId
    WHERE p.Id = @ProductId;
    
    -- Return images
    SELECT ImageUrl, DisplayOrder, AltText
    FROM ProductImages
    WHERE ProductId = @ProductId
    ORDER BY DisplayOrder;
    
    -- Return attributes
    SELECT AttributeName, AttributeValue
    FROM ProductAttributes
    WHERE ProductId = @ProductId;
END
GO

-- ============================================================================
-- usp_GetProductBySku
-- Retrieves a product by SKU
-- ============================================================================
CREATE PROCEDURE usp_GetProductBySku
    @Sku NVARCHAR(50)
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @ProductId BIGINT;
    SELECT @ProductId = Id FROM Products WHERE Sku = @Sku;
    
    IF @ProductId IS NOT NULL
        EXEC usp_GetProductById @ProductId;
END
GO

-- ============================================================================
-- usp_GetProductBySlug
-- Retrieves a product by slug
-- ============================================================================
CREATE PROCEDURE usp_GetProductBySlug
    @Slug NVARCHAR(255)
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @ProductId BIGINT;
    SELECT @ProductId = Id FROM Products WHERE Slug = @Slug;
    
    IF @ProductId IS NOT NULL
        EXEC usp_GetProductById @ProductId;
END
GO

-- ============================================================================
-- usp_ListProducts
-- Lists products with filtering, sorting, and pagination
-- ============================================================================
CREATE PROCEDURE usp_ListProducts
    @CategoryId BIGINT = NULL,
    @SearchQuery NVARCHAR(255) = NULL,
    @MinPriceUnits BIGINT = NULL,
    @MaxPriceUnits BIGINT = NULL,
    @InStockOnly BIT = 0,
    @IncludeInactive BIT = 0,
    @SortBy NVARCHAR(50) = 'created_at',
    @SortDescending BIT = 1,
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
    FROM Products p
    LEFT JOIN Inventory i ON p.Id = i.ProductId
    WHERE (@CategoryId IS NULL OR p.CategoryId = @CategoryId)
      AND (@SearchQuery IS NULL OR p.Name LIKE '%' + @SearchQuery + '%' OR p.Description LIKE '%' + @SearchQuery + '%')
      AND (@MinPriceUnits IS NULL OR p.PriceUnits >= @MinPriceUnits)
      AND (@MaxPriceUnits IS NULL OR p.PriceUnits <= @MaxPriceUnits)
      AND (@InStockOnly = 0 OR COALESCE(i.QuantityOnHand - i.QuantityReserved, 0) > 0)
      AND (@IncludeInactive = 1 OR p.IsActive = 1);
    
    -- Get products
    SELECT 
        p.Id, p.Sku, p.Name, p.Slug, p.Description,
        p.PriceUnits, p.PriceNanos, p.PriceCurrency,
        p.CompareAtPriceUnits, p.CompareAtPriceNanos,
        p.CategoryId, c.Name AS CategoryName,
        p.IsActive, p.TrackInventory,
        COALESCE(i.QuantityOnHand - i.QuantityReserved, 0) AS StockQuantity,
        p.AverageRating, p.ReviewCount,
        p.CreatedAt, p.UpdatedAt
    FROM Products p
    INNER JOIN Categories c ON p.CategoryId = c.Id
    LEFT JOIN Inventory i ON p.Id = i.ProductId
    WHERE (@CategoryId IS NULL OR p.CategoryId = @CategoryId)
      AND (@SearchQuery IS NULL OR p.Name LIKE '%' + @SearchQuery + '%' OR p.Description LIKE '%' + @SearchQuery + '%')
      AND (@MinPriceUnits IS NULL OR p.PriceUnits >= @MinPriceUnits)
      AND (@MaxPriceUnits IS NULL OR p.PriceUnits <= @MaxPriceUnits)
      AND (@InStockOnly = 0 OR COALESCE(i.QuantityOnHand - i.QuantityReserved, 0) > 0)
      AND (@IncludeInactive = 1 OR p.IsActive = 1)
    ORDER BY 
        CASE WHEN @SortBy = 'name' AND @SortDescending = 0 THEN p.Name END ASC,
        CASE WHEN @SortBy = 'name' AND @SortDescending = 1 THEN p.Name END DESC,
        CASE WHEN @SortBy = 'price' AND @SortDescending = 0 THEN p.PriceUnits END ASC,
        CASE WHEN @SortBy = 'price' AND @SortDescending = 1 THEN p.PriceUnits END DESC,
        CASE WHEN @SortBy = 'rating' AND @SortDescending = 0 THEN p.AverageRating END ASC,
        CASE WHEN @SortBy = 'rating' AND @SortDescending = 1 THEN p.AverageRating END DESC,
        CASE WHEN @SortBy = 'created_at' AND @SortDescending = 0 THEN p.CreatedAt END ASC,
        CASE WHEN @SortBy = 'created_at' AND @SortDescending = 1 THEN p.CreatedAt END DESC
    OFFSET @Offset ROWS FETCH NEXT @PageSize ROWS ONLY;
END
GO

-- ============================================================================
-- usp_CreateProduct
-- Creates a new product
-- ============================================================================
CREATE PROCEDURE usp_CreateProduct
    @Sku NVARCHAR(50),
    @Name NVARCHAR(255),
    @Slug NVARCHAR(255),
    @Description NVARCHAR(MAX) = NULL,
    @PriceUnits BIGINT,
    @PriceNanos INT = 0,
    @PriceCurrency NVARCHAR(3) = 'USD',
    @CompareAtPriceUnits BIGINT = NULL,
    @CompareAtPriceNanos INT = NULL,
    @CostPriceUnits BIGINT = NULL,
    @CostPriceNanos INT = NULL,
    @CategoryId BIGINT,
    @TrackInventory BIT = 1,
    @InitialStock INT = 0,
    @CreatedBy BIGINT = NULL
AS
BEGIN
    SET NOCOUNT ON;
    BEGIN TRANSACTION;
    
    BEGIN TRY
        -- Check for duplicate SKU
        IF EXISTS (SELECT 1 FROM Products WHERE Sku = @Sku)
        BEGIN
            RAISERROR('Product SKU already exists', 16, 1);
            RETURN;
        END
        
        -- Check for duplicate slug
        IF EXISTS (SELECT 1 FROM Products WHERE Slug = @Slug)
        BEGIN
            RAISERROR('Product slug already exists', 16, 1);
            RETURN;
        END
        
        DECLARE @ProductId BIGINT;
        
        INSERT INTO Products (
            Sku, Name, Slug, Description,
            PriceUnits, PriceNanos, PriceCurrency,
            CompareAtPriceUnits, CompareAtPriceNanos,
            CostPriceUnits, CostPriceNanos,
            CategoryId, TrackInventory, CreatedBy
        )
        VALUES (
            @Sku, @Name, @Slug, @Description,
            @PriceUnits, @PriceNanos, @PriceCurrency,
            @CompareAtPriceUnits, @CompareAtPriceNanos,
            @CostPriceUnits, @CostPriceNanos,
            @CategoryId, @TrackInventory, @CreatedBy
        );
        
        SET @ProductId = SCOPE_IDENTITY();
        
        -- Create inventory record
        IF @TrackInventory = 1
        BEGIN
            INSERT INTO Inventory (ProductId, QuantityOnHand)
            VALUES (@ProductId, @InitialStock);
            
            IF @InitialStock > 0
            BEGIN
                INSERT INTO InventoryTransactions (
                    ProductId, TransactionType, Quantity,
                    QuantityBefore, QuantityAfter, Reference, CreatedBy
                )
                VALUES (
                    @ProductId, 'RECEIVED', @InitialStock,
                    0, @InitialStock, 'Initial stock', COALESCE(@CreatedBy, 0)
                );
            END
        END
        
        COMMIT TRANSACTION;
        
        EXEC usp_GetProductById @ProductId;
    END TRY
    BEGIN CATCH
        ROLLBACK TRANSACTION;
        THROW;
    END CATCH
END
GO

-- ============================================================================
-- usp_UpdateProduct
-- Updates an existing product
-- ============================================================================
CREATE PROCEDURE usp_UpdateProduct
    @ProductId BIGINT,
    @Name NVARCHAR(255) = NULL,
    @Description NVARCHAR(MAX) = NULL,
    @PriceUnits BIGINT = NULL,
    @PriceNanos INT = NULL,
    @CompareAtPriceUnits BIGINT = NULL,
    @CompareAtPriceNanos INT = NULL,
    @CostPriceUnits BIGINT = NULL,
    @CostPriceNanos INT = NULL,
    @CategoryId BIGINT = NULL,
    @IsActive BIT = NULL,
    @UpdatedBy BIGINT = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    UPDATE Products
    SET Name = COALESCE(@Name, Name),
        Description = COALESCE(@Description, Description),
        PriceUnits = COALESCE(@PriceUnits, PriceUnits),
        PriceNanos = COALESCE(@PriceNanos, PriceNanos),
        CompareAtPriceUnits = COALESCE(@CompareAtPriceUnits, CompareAtPriceUnits),
        CompareAtPriceNanos = COALESCE(@CompareAtPriceNanos, CompareAtPriceNanos),
        CostPriceUnits = COALESCE(@CostPriceUnits, CostPriceUnits),
        CostPriceNanos = COALESCE(@CostPriceNanos, CostPriceNanos),
        CategoryId = COALESCE(@CategoryId, CategoryId),
        IsActive = COALESCE(@IsActive, IsActive),
        UpdatedBy = @UpdatedBy,
        UpdatedAt = GETUTCDATE()
    WHERE Id = @ProductId;
    
    EXEC usp_GetProductById @ProductId;
END
GO

-- ============================================================================
-- usp_DeleteProduct
-- Soft-deletes a product (sets IsActive = 0)
-- ============================================================================
CREATE PROCEDURE usp_DeleteProduct
    @ProductId BIGINT,
    @DeletedBy BIGINT = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    UPDATE Products
    SET IsActive = 0,
        UpdatedBy = @DeletedBy,
        UpdatedAt = GETUTCDATE()
    WHERE Id = @ProductId;
    
    -- Log deletion
    INSERT INTO AuditLog (EntityType, EntityId, Action, PerformedBy)
    VALUES ('Product', @ProductId, 'DELETE', @DeletedBy);
    
    SELECT 1 AS Success;
END
GO

-- ============================================================================
-- usp_AddProductImage
-- Adds an image to a product
-- ============================================================================
CREATE PROCEDURE usp_AddProductImage
    @ProductId BIGINT,
    @ImageUrl NVARCHAR(500),
    @DisplayOrder INT = 0,
    @AltText NVARCHAR(255) = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    INSERT INTO ProductImages (ProductId, ImageUrl, DisplayOrder, AltText)
    VALUES (@ProductId, @ImageUrl, @DisplayOrder, @AltText);
    
    SELECT SCOPE_IDENTITY() AS Id;
END
GO

-- ============================================================================
-- usp_SetProductAttribute
-- Sets a product attribute (creates or updates)
-- ============================================================================
CREATE PROCEDURE usp_SetProductAttribute
    @ProductId BIGINT,
    @AttributeName NVARCHAR(100),
    @AttributeValue NVARCHAR(500)
AS
BEGIN
    SET NOCOUNT ON;
    
    MERGE ProductAttributes AS target
    USING (SELECT @ProductId, @AttributeName, @AttributeValue) AS source (ProductId, AttributeName, AttributeValue)
    ON target.ProductId = source.ProductId AND target.AttributeName = source.AttributeName
    WHEN MATCHED THEN
        UPDATE SET AttributeValue = source.AttributeValue
    WHEN NOT MATCHED THEN
        INSERT (ProductId, AttributeName, AttributeValue)
        VALUES (source.ProductId, source.AttributeName, source.AttributeValue);
END
GO

-- ============================================================================
-- usp_SearchProducts
-- Full-text search for products
-- ============================================================================
CREATE PROCEDURE usp_SearchProducts
    @Query NVARCHAR(255),
    @CategoryIds NVARCHAR(MAX) = NULL,
    @MinPriceUnits BIGINT = NULL,
    @MaxPriceUnits BIGINT = NULL,
    @PageSize INT = 20,
    @PageToken NVARCHAR(100) = NULL
AS
BEGIN
    SET NOCOUNT ON;
    
    DECLARE @Offset INT = 0;
    IF @PageToken IS NOT NULL
        SET @Offset = TRY_CAST(@PageToken AS INT);
    
    -- Parse category IDs if provided
    DECLARE @CategoryTable TABLE (CategoryId BIGINT);
    IF @CategoryIds IS NOT NULL
    BEGIN
        INSERT INTO @CategoryTable
        SELECT value FROM STRING_SPLIT(@CategoryIds, ',');
    END
    
    -- Get total count
    SELECT COUNT(*) AS TotalCount
    FROM Products p
    WHERE p.IsActive = 1
      AND (p.Name LIKE '%' + @Query + '%' OR p.Description LIKE '%' + @Query + '%' OR p.Sku LIKE '%' + @Query + '%')
      AND (@CategoryIds IS NULL OR p.CategoryId IN (SELECT CategoryId FROM @CategoryTable))
      AND (@MinPriceUnits IS NULL OR p.PriceUnits >= @MinPriceUnits)
      AND (@MaxPriceUnits IS NULL OR p.PriceUnits <= @MaxPriceUnits);
    
    -- Get products
    SELECT 
        p.Id, p.Sku, p.Name, p.Slug, p.Description,
        p.PriceUnits, p.PriceNanos, p.PriceCurrency,
        p.CategoryId, c.Name AS CategoryName,
        p.IsActive, p.AverageRating, p.ReviewCount,
        p.CreatedAt
    FROM Products p
    INNER JOIN Categories c ON p.CategoryId = c.Id
    WHERE p.IsActive = 1
      AND (p.Name LIKE '%' + @Query + '%' OR p.Description LIKE '%' + @Query + '%' OR p.Sku LIKE '%' + @Query + '%')
      AND (@CategoryIds IS NULL OR p.CategoryId IN (SELECT CategoryId FROM @CategoryTable))
      AND (@MinPriceUnits IS NULL OR p.PriceUnits >= @MinPriceUnits)
      AND (@MaxPriceUnits IS NULL OR p.PriceUnits <= @MaxPriceUnits)
    ORDER BY 
        CASE WHEN p.Name LIKE @Query + '%' THEN 0 ELSE 1 END,
        p.AverageRating DESC, p.ReviewCount DESC
    OFFSET @Offset ROWS FETCH NEXT @PageSize ROWS ONLY;
END
GO
