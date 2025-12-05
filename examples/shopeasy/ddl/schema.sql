-- ============================================================================
-- ShopEasy E-Commerce Database Schema
-- SQL Server / T-SQL DDL
-- ============================================================================

-- ============================================================================
-- Users & Authentication
-- ============================================================================

CREATE TABLE Users (
    Id BIGINT IDENTITY(1,1) PRIMARY KEY,
    Email NVARCHAR(255) NOT NULL,
    Username NVARCHAR(100) NOT NULL,
    PasswordHash NVARCHAR(255) NOT NULL,
    Salt NVARCHAR(100) NOT NULL,
    FirstName NVARCHAR(100) NULL,
    LastName NVARCHAR(100) NULL,
    Phone NVARCHAR(50) NULL,
    BillingAddressLine1 NVARCHAR(255) NULL,
    BillingAddressLine2 NVARCHAR(255) NULL,
    BillingCity NVARCHAR(100) NULL,
    BillingState NVARCHAR(100) NULL,
    BillingPostalCode NVARCHAR(20) NULL,
    BillingCountry NVARCHAR(100) NULL,
    ShippingAddressLine1 NVARCHAR(255) NULL,
    ShippingAddressLine2 NVARCHAR(255) NULL,
    ShippingCity NVARCHAR(100) NULL,
    ShippingState NVARCHAR(100) NULL,
    ShippingPostalCode NVARCHAR(20) NULL,
    ShippingCountry NVARCHAR(100) NULL,
    IsActive BIT NOT NULL DEFAULT 1,
    EmailVerified BIT NOT NULL DEFAULT 0,
    EmailVerificationToken NVARCHAR(255) NULL,
    PasswordResetToken NVARCHAR(255) NULL,
    PasswordResetExpires DATETIME2 NULL,
    FailedLoginAttempts INT NOT NULL DEFAULT 0,
    LockoutEnd DATETIME2 NULL,
    LastLoginAt DATETIME2 NULL,
    CreatedAt DATETIME2 NOT NULL DEFAULT GETUTCDATE(),
    UpdatedAt DATETIME2 NOT NULL DEFAULT GETUTCDATE(),
    
    CONSTRAINT UQ_Users_Email UNIQUE (Email),
    CONSTRAINT UQ_Users_Username UNIQUE (Username)
);

CREATE INDEX IX_Users_Email ON Users(Email);
CREATE INDEX IX_Users_Username ON Users(Username);
CREATE INDEX IX_Users_IsActive ON Users(IsActive) WHERE IsActive = 1;

CREATE TABLE RefreshTokens (
    Id BIGINT IDENTITY(1,1) PRIMARY KEY,
    UserId BIGINT NOT NULL,
    Token NVARCHAR(500) NOT NULL,
    ExpiresAt DATETIME2 NOT NULL,
    CreatedAt DATETIME2 NOT NULL DEFAULT GETUTCDATE(),
    RevokedAt DATETIME2 NULL,
    
    CONSTRAINT FK_RefreshTokens_Users FOREIGN KEY (UserId) REFERENCES Users(Id)
);

CREATE INDEX IX_RefreshTokens_Token ON RefreshTokens(Token);
CREATE INDEX IX_RefreshTokens_UserId ON RefreshTokens(UserId);

-- ============================================================================
-- Catalog: Categories & Products
-- ============================================================================

CREATE TABLE Categories (
    Id BIGINT IDENTITY(1,1) PRIMARY KEY,
    Name NVARCHAR(100) NOT NULL,
    Slug NVARCHAR(100) NOT NULL,
    Description NVARCHAR(MAX) NULL,
    ParentId BIGINT NULL,
    DisplayOrder INT NOT NULL DEFAULT 0,
    IsActive BIT NOT NULL DEFAULT 1,
    ImageUrl NVARCHAR(500) NULL,
    CreatedAt DATETIME2 NOT NULL DEFAULT GETUTCDATE(),
    UpdatedAt DATETIME2 NOT NULL DEFAULT GETUTCDATE(),
    
    CONSTRAINT UQ_Categories_Slug UNIQUE (Slug),
    CONSTRAINT FK_Categories_Parent FOREIGN KEY (ParentId) REFERENCES Categories(Id)
);

CREATE INDEX IX_Categories_ParentId ON Categories(ParentId);
CREATE INDEX IX_Categories_Slug ON Categories(Slug);
CREATE INDEX IX_Categories_IsActive ON Categories(IsActive) WHERE IsActive = 1;

CREATE TABLE Products (
    Id BIGINT IDENTITY(1,1) PRIMARY KEY,
    Sku NVARCHAR(50) NOT NULL,
    Name NVARCHAR(255) NOT NULL,
    Slug NVARCHAR(255) NOT NULL,
    Description NVARCHAR(MAX) NULL,
    PriceUnits BIGINT NOT NULL,
    PriceNanos INT NOT NULL DEFAULT 0,
    PriceCurrency NVARCHAR(3) NOT NULL DEFAULT 'USD',
    CompareAtPriceUnits BIGINT NULL,
    CompareAtPriceNanos INT NULL,
    CostPriceUnits BIGINT NULL,
    CostPriceNanos INT NULL,
    CategoryId BIGINT NOT NULL,
    IsActive BIT NOT NULL DEFAULT 1,
    TrackInventory BIT NOT NULL DEFAULT 1,
    AverageRating DECIMAL(3,2) NOT NULL DEFAULT 0.00,
    ReviewCount INT NOT NULL DEFAULT 0,
    CreatedAt DATETIME2 NOT NULL DEFAULT GETUTCDATE(),
    UpdatedAt DATETIME2 NOT NULL DEFAULT GETUTCDATE(),
    CreatedBy BIGINT NULL,
    UpdatedBy BIGINT NULL,
    
    CONSTRAINT UQ_Products_Sku UNIQUE (Sku),
    CONSTRAINT UQ_Products_Slug UNIQUE (Slug),
    CONSTRAINT FK_Products_Category FOREIGN KEY (CategoryId) REFERENCES Categories(Id),
    CONSTRAINT FK_Products_CreatedBy FOREIGN KEY (CreatedBy) REFERENCES Users(Id),
    CONSTRAINT FK_Products_UpdatedBy FOREIGN KEY (UpdatedBy) REFERENCES Users(Id)
);

CREATE INDEX IX_Products_Sku ON Products(Sku);
CREATE INDEX IX_Products_Slug ON Products(Slug);
CREATE INDEX IX_Products_CategoryId ON Products(CategoryId);
CREATE INDEX IX_Products_IsActive ON Products(IsActive) WHERE IsActive = 1;
CREATE INDEX IX_Products_Price ON Products(PriceUnits, PriceNanos);

CREATE TABLE ProductImages (
    Id BIGINT IDENTITY(1,1) PRIMARY KEY,
    ProductId BIGINT NOT NULL,
    ImageUrl NVARCHAR(500) NOT NULL,
    DisplayOrder INT NOT NULL DEFAULT 0,
    AltText NVARCHAR(255) NULL,
    CreatedAt DATETIME2 NOT NULL DEFAULT GETUTCDATE(),
    
    CONSTRAINT FK_ProductImages_Product FOREIGN KEY (ProductId) REFERENCES Products(Id) ON DELETE CASCADE
);

CREATE INDEX IX_ProductImages_ProductId ON ProductImages(ProductId);

CREATE TABLE ProductAttributes (
    Id BIGINT IDENTITY(1,1) PRIMARY KEY,
    ProductId BIGINT NOT NULL,
    AttributeName NVARCHAR(100) NOT NULL,
    AttributeValue NVARCHAR(500) NOT NULL,
    
    CONSTRAINT FK_ProductAttributes_Product FOREIGN KEY (ProductId) REFERENCES Products(Id) ON DELETE CASCADE,
    CONSTRAINT UQ_ProductAttributes UNIQUE (ProductId, AttributeName)
);

CREATE INDEX IX_ProductAttributes_ProductId ON ProductAttributes(ProductId);

-- ============================================================================
-- Inventory
-- ============================================================================

CREATE TABLE Inventory (
    ProductId BIGINT PRIMARY KEY,
    QuantityOnHand INT NOT NULL DEFAULT 0,
    QuantityReserved INT NOT NULL DEFAULT 0,
    ReorderPoint INT NOT NULL DEFAULT 10,
    ReorderQuantity INT NOT NULL DEFAULT 50,
    UpdatedAt DATETIME2 NOT NULL DEFAULT GETUTCDATE(),
    
    CONSTRAINT FK_Inventory_Product FOREIGN KEY (ProductId) REFERENCES Products(Id) ON DELETE CASCADE,
    CONSTRAINT CK_Inventory_QuantityOnHand CHECK (QuantityOnHand >= 0),
    CONSTRAINT CK_Inventory_QuantityReserved CHECK (QuantityReserved >= 0)
);

CREATE TABLE InventoryTransactions (
    Id BIGINT IDENTITY(1,1) PRIMARY KEY,
    ProductId BIGINT NOT NULL,
    TransactionType NVARCHAR(50) NOT NULL,  -- RECEIVED, SOLD, RETURNED, ADJUSTMENT, RESERVED, RELEASED, DAMAGED
    Quantity INT NOT NULL,
    QuantityBefore INT NOT NULL,
    QuantityAfter INT NOT NULL,
    OrderId BIGINT NULL,
    Reference NVARCHAR(100) NULL,
    Notes NVARCHAR(MAX) NULL,
    CreatedBy BIGINT NOT NULL,
    CreatedAt DATETIME2 NOT NULL DEFAULT GETUTCDATE(),
    
    CONSTRAINT FK_InventoryTransactions_Product FOREIGN KEY (ProductId) REFERENCES Products(Id),
    CONSTRAINT FK_InventoryTransactions_CreatedBy FOREIGN KEY (CreatedBy) REFERENCES Users(Id)
);

CREATE INDEX IX_InventoryTransactions_ProductId ON InventoryTransactions(ProductId);
CREATE INDEX IX_InventoryTransactions_OrderId ON InventoryTransactions(OrderId);
CREATE INDEX IX_InventoryTransactions_CreatedAt ON InventoryTransactions(CreatedAt);

-- ============================================================================
-- Shopping Cart
-- ============================================================================

CREATE TABLE Carts (
    Id BIGINT IDENTITY(1,1) PRIMARY KEY,
    UserId BIGINT NOT NULL,
    CreatedAt DATETIME2 NOT NULL DEFAULT GETUTCDATE(),
    UpdatedAt DATETIME2 NOT NULL DEFAULT GETUTCDATE(),
    
    CONSTRAINT FK_Carts_User FOREIGN KEY (UserId) REFERENCES Users(Id),
    CONSTRAINT UQ_Carts_UserId UNIQUE (UserId)
);

CREATE TABLE CartItems (
    Id BIGINT IDENTITY(1,1) PRIMARY KEY,
    CartId BIGINT NOT NULL,
    ProductId BIGINT NOT NULL,
    Quantity INT NOT NULL DEFAULT 1,
    AddedAt DATETIME2 NOT NULL DEFAULT GETUTCDATE(),
    
    CONSTRAINT FK_CartItems_Cart FOREIGN KEY (CartId) REFERENCES Carts(Id) ON DELETE CASCADE,
    CONSTRAINT FK_CartItems_Product FOREIGN KEY (ProductId) REFERENCES Products(Id),
    CONSTRAINT UQ_CartItems_CartProduct UNIQUE (CartId, ProductId),
    CONSTRAINT CK_CartItems_Quantity CHECK (Quantity > 0)
);

CREATE INDEX IX_CartItems_CartId ON CartItems(CartId);
CREATE INDEX IX_CartItems_ProductId ON CartItems(ProductId);

-- ============================================================================
-- Orders
-- ============================================================================

CREATE TABLE Orders (
    Id BIGINT IDENTITY(1,1) PRIMARY KEY,
    OrderNumber NVARCHAR(50) NOT NULL,
    UserId BIGINT NOT NULL,
    Status NVARCHAR(50) NOT NULL DEFAULT 'PENDING',  -- PENDING, CONFIRMED, PROCESSING, SHIPPED, DELIVERED, CANCELLED, REFUNDED
    PaymentStatus NVARCHAR(50) NOT NULL DEFAULT 'PENDING',  -- PENDING, AUTHORIZED, CAPTURED, FAILED, REFUNDED
    ShippingAddressLine1 NVARCHAR(255) NOT NULL,
    ShippingAddressLine2 NVARCHAR(255) NULL,
    ShippingCity NVARCHAR(100) NOT NULL,
    ShippingState NVARCHAR(100) NOT NULL,
    ShippingPostalCode NVARCHAR(20) NOT NULL,
    ShippingCountry NVARCHAR(100) NOT NULL,
    BillingAddressLine1 NVARCHAR(255) NOT NULL,
    BillingAddressLine2 NVARCHAR(255) NULL,
    BillingCity NVARCHAR(100) NOT NULL,
    BillingState NVARCHAR(100) NOT NULL,
    BillingPostalCode NVARCHAR(20) NOT NULL,
    BillingCountry NVARCHAR(100) NOT NULL,
    SubtotalUnits BIGINT NOT NULL,
    SubtotalNanos INT NOT NULL DEFAULT 0,
    TaxAmountUnits BIGINT NOT NULL DEFAULT 0,
    TaxAmountNanos INT NOT NULL DEFAULT 0,
    ShippingAmountUnits BIGINT NOT NULL DEFAULT 0,
    ShippingAmountNanos INT NOT NULL DEFAULT 0,
    DiscountAmountUnits BIGINT NOT NULL DEFAULT 0,
    DiscountAmountNanos INT NOT NULL DEFAULT 0,
    TotalUnits BIGINT NOT NULL,
    TotalNanos INT NOT NULL DEFAULT 0,
    Currency NVARCHAR(3) NOT NULL DEFAULT 'USD',
    DiscountCode NVARCHAR(50) NULL,
    Notes NVARCHAR(MAX) NULL,
    TrackingNumber NVARCHAR(100) NULL,
    Carrier NVARCHAR(100) NULL,
    PaymentIntentId NVARCHAR(255) NULL,
    CreatedAt DATETIME2 NOT NULL DEFAULT GETUTCDATE(),
    UpdatedAt DATETIME2 NOT NULL DEFAULT GETUTCDATE(),
    ShippedAt DATETIME2 NULL,
    DeliveredAt DATETIME2 NULL,
    CancelledAt DATETIME2 NULL,
    CancellationReason NVARCHAR(MAX) NULL,
    
    CONSTRAINT UQ_Orders_OrderNumber UNIQUE (OrderNumber),
    CONSTRAINT FK_Orders_User FOREIGN KEY (UserId) REFERENCES Users(Id)
);

CREATE INDEX IX_Orders_UserId ON Orders(UserId);
CREATE INDEX IX_Orders_OrderNumber ON Orders(OrderNumber);
CREATE INDEX IX_Orders_Status ON Orders(Status);
CREATE INDEX IX_Orders_CreatedAt ON Orders(CreatedAt);

CREATE TABLE OrderItems (
    Id BIGINT IDENTITY(1,1) PRIMARY KEY,
    OrderId BIGINT NOT NULL,
    ProductId BIGINT NOT NULL,
    ProductName NVARCHAR(255) NOT NULL,
    ProductSku NVARCHAR(50) NOT NULL,
    Quantity INT NOT NULL,
    UnitPriceUnits BIGINT NOT NULL,
    UnitPriceNanos INT NOT NULL DEFAULT 0,
    SubtotalUnits BIGINT NOT NULL,
    SubtotalNanos INT NOT NULL DEFAULT 0,
    TaxAmountUnits BIGINT NOT NULL DEFAULT 0,
    TaxAmountNanos INT NOT NULL DEFAULT 0,
    TotalUnits BIGINT NOT NULL,
    TotalNanos INT NOT NULL DEFAULT 0,
    CreatedAt DATETIME2 NOT NULL DEFAULT GETUTCDATE(),
    
    CONSTRAINT FK_OrderItems_Order FOREIGN KEY (OrderId) REFERENCES Orders(Id),
    CONSTRAINT FK_OrderItems_Product FOREIGN KEY (ProductId) REFERENCES Products(Id)
);

CREATE INDEX IX_OrderItems_OrderId ON OrderItems(OrderId);
CREATE INDEX IX_OrderItems_ProductId ON OrderItems(ProductId);

CREATE TABLE OrderStatusHistory (
    Id BIGINT IDENTITY(1,1) PRIMARY KEY,
    OrderId BIGINT NOT NULL,
    Status NVARCHAR(50) NOT NULL,
    Notes NVARCHAR(MAX) NULL,
    CreatedBy BIGINT NULL,
    CreatedAt DATETIME2 NOT NULL DEFAULT GETUTCDATE(),
    
    CONSTRAINT FK_OrderStatusHistory_Order FOREIGN KEY (OrderId) REFERENCES Orders(Id)
);

CREATE INDEX IX_OrderStatusHistory_OrderId ON OrderStatusHistory(OrderId);

-- ============================================================================
-- Payments & Refunds
-- ============================================================================

CREATE TABLE PaymentTransactions (
    Id BIGINT IDENTITY(1,1) PRIMARY KEY,
    OrderId BIGINT NOT NULL,
    TransactionType NVARCHAR(50) NOT NULL,  -- AUTHORIZATION, CAPTURE, REFUND
    Status NVARCHAR(50) NOT NULL,  -- PENDING, SUCCESS, FAILED
    AmountUnits BIGINT NOT NULL,
    AmountNanos INT NOT NULL DEFAULT 0,
    Currency NVARCHAR(3) NOT NULL DEFAULT 'USD',
    PaymentIntentId NVARCHAR(255) NULL,
    PaymentMethodId NVARCHAR(255) NULL,
    ErrorMessage NVARCHAR(MAX) NULL,
    CreatedAt DATETIME2 NOT NULL DEFAULT GETUTCDATE(),
    
    CONSTRAINT FK_PaymentTransactions_Order FOREIGN KEY (OrderId) REFERENCES Orders(Id)
);

CREATE INDEX IX_PaymentTransactions_OrderId ON PaymentTransactions(OrderId);
CREATE INDEX IX_PaymentTransactions_PaymentIntentId ON PaymentTransactions(PaymentIntentId);

CREATE TABLE Refunds (
    Id BIGINT IDENTITY(1,1) PRIMARY KEY,
    OrderId BIGINT NOT NULL,
    RefundId NVARCHAR(255) NOT NULL,
    AmountUnits BIGINT NOT NULL,
    AmountNanos INT NOT NULL DEFAULT 0,
    Currency NVARCHAR(3) NOT NULL DEFAULT 'USD',
    Reason NVARCHAR(MAX) NULL,
    Status NVARCHAR(50) NOT NULL DEFAULT 'PENDING',
    CreatedBy BIGINT NOT NULL,
    CreatedAt DATETIME2 NOT NULL DEFAULT GETUTCDATE(),
    ProcessedAt DATETIME2 NULL,
    
    CONSTRAINT FK_Refunds_Order FOREIGN KEY (OrderId) REFERENCES Orders(Id),
    CONSTRAINT FK_Refunds_CreatedBy FOREIGN KEY (CreatedBy) REFERENCES Users(Id)
);

CREATE INDEX IX_Refunds_OrderId ON Refunds(OrderId);

-- ============================================================================
-- Discount Codes
-- ============================================================================

CREATE TABLE DiscountCodes (
    Id BIGINT IDENTITY(1,1) PRIMARY KEY,
    Code NVARCHAR(50) NOT NULL,
    Description NVARCHAR(255) NULL,
    DiscountType NVARCHAR(20) NOT NULL,  -- PERCENTAGE, FIXED
    DiscountValue DECIMAL(10,2) NOT NULL,
    MinimumOrderUnits BIGINT NULL,
    MaximumDiscountUnits BIGINT NULL,
    UsageLimit INT NULL,
    UsageCount INT NOT NULL DEFAULT 0,
    StartsAt DATETIME2 NULL,
    ExpiresAt DATETIME2 NULL,
    IsActive BIT NOT NULL DEFAULT 1,
    CreatedAt DATETIME2 NOT NULL DEFAULT GETUTCDATE(),
    
    CONSTRAINT UQ_DiscountCodes_Code UNIQUE (Code)
);

CREATE INDEX IX_DiscountCodes_Code ON DiscountCodes(Code);
CREATE INDEX IX_DiscountCodes_IsActive ON DiscountCodes(IsActive) WHERE IsActive = 1;

-- ============================================================================
-- Reviews
-- ============================================================================

CREATE TABLE Reviews (
    Id BIGINT IDENTITY(1,1) PRIMARY KEY,
    ProductId BIGINT NOT NULL,
    UserId BIGINT NOT NULL,
    Rating INT NOT NULL,
    Title NVARCHAR(255) NULL,
    Content NVARCHAR(MAX) NULL,
    Status NVARCHAR(50) NOT NULL DEFAULT 'PENDING',  -- PENDING, APPROVED, REJECTED
    VerifiedPurchase BIT NOT NULL DEFAULT 0,
    HelpfulVotes INT NOT NULL DEFAULT 0,
    UnhelpfulVotes INT NOT NULL DEFAULT 0,
    RejectionReason NVARCHAR(MAX) NULL,
    ModeratedBy BIGINT NULL,
    ModeratedAt DATETIME2 NULL,
    CreatedAt DATETIME2 NOT NULL DEFAULT GETUTCDATE(),
    UpdatedAt DATETIME2 NOT NULL DEFAULT GETUTCDATE(),
    
    CONSTRAINT FK_Reviews_Product FOREIGN KEY (ProductId) REFERENCES Products(Id),
    CONSTRAINT FK_Reviews_User FOREIGN KEY (UserId) REFERENCES Users(Id),
    CONSTRAINT FK_Reviews_ModeratedBy FOREIGN KEY (ModeratedBy) REFERENCES Users(Id),
    CONSTRAINT CK_Reviews_Rating CHECK (Rating >= 1 AND Rating <= 5),
    CONSTRAINT UQ_Reviews_UserProduct UNIQUE (UserId, ProductId)
);

CREATE INDEX IX_Reviews_ProductId ON Reviews(ProductId);
CREATE INDEX IX_Reviews_UserId ON Reviews(UserId);
CREATE INDEX IX_Reviews_Status ON Reviews(Status);
CREATE INDEX IX_Reviews_Rating ON Reviews(Rating);

CREATE TABLE ReviewVotes (
    Id BIGINT IDENTITY(1,1) PRIMARY KEY,
    ReviewId BIGINT NOT NULL,
    UserId BIGINT NOT NULL,
    IsHelpful BIT NOT NULL,
    CreatedAt DATETIME2 NOT NULL DEFAULT GETUTCDATE(),
    
    CONSTRAINT FK_ReviewVotes_Review FOREIGN KEY (ReviewId) REFERENCES Reviews(Id) ON DELETE CASCADE,
    CONSTRAINT FK_ReviewVotes_User FOREIGN KEY (UserId) REFERENCES Users(Id),
    CONSTRAINT UQ_ReviewVotes_UserReview UNIQUE (UserId, ReviewId)
);

CREATE INDEX IX_ReviewVotes_ReviewId ON ReviewVotes(ReviewId);

-- ============================================================================
-- Audit Log
-- ============================================================================

CREATE TABLE AuditLog (
    Id BIGINT IDENTITY(1,1) PRIMARY KEY,
    EntityType NVARCHAR(100) NOT NULL,
    EntityId BIGINT NOT NULL,
    Action NVARCHAR(50) NOT NULL,  -- CREATE, UPDATE, DELETE
    OldValues NVARCHAR(MAX) NULL,
    NewValues NVARCHAR(MAX) NULL,
    PerformedBy BIGINT NULL,
    PerformedAt DATETIME2 NOT NULL DEFAULT GETUTCDATE(),
    IpAddress NVARCHAR(50) NULL,
    UserAgent NVARCHAR(500) NULL
);

CREATE INDEX IX_AuditLog_EntityType_EntityId ON AuditLog(EntityType, EntityId);
CREATE INDEX IX_AuditLog_PerformedAt ON AuditLog(PerformedAt);
CREATE INDEX IX_AuditLog_PerformedBy ON AuditLog(PerformedBy);
