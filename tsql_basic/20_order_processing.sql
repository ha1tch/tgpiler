-- Order processing with validation and pricing
-- Demonstrates complex business logic with multiple validation steps

CREATE PROCEDURE dbo.ProcessOrder
    @CustomerId INT,
    @ProductCode VARCHAR(20),
    @Quantity INT,
    @UnitPrice DECIMAL(10,2),
    @ShippingMethod INT,
    @PromoCode VARCHAR(20),
    @TotalAmount DECIMAL(10,2) OUTPUT,
    @ShippingCost DECIMAL(10,2) OUTPUT,
    @Discount DECIMAL(10,2) OUTPUT,
    @FinalTotal DECIMAL(10,2) OUTPUT,
    @StatusCode INT OUTPUT,
    @StatusMessage VARCHAR(200) OUTPUT
AS
BEGIN
    DECLARE @Subtotal DECIMAL(10,2)
    DECLARE @DiscountPercent DECIMAL(5,2) = 0
    DECLARE @FreeShippingThreshold DECIMAL(10,2) = 100.00
    
    -- Initialize outputs
    SET @TotalAmount = 0
    SET @ShippingCost = 0
    SET @Discount = 0
    SET @FinalTotal = 0
    SET @StatusCode = 0
    SET @StatusMessage = ''
    
    -- Validate customer ID
    IF @CustomerId <= 0
    BEGIN
        SET @StatusCode = 1
        SET @StatusMessage = 'Invalid customer ID'
        RETURN 1
    END
    
    -- Validate product code
    IF @ProductCode IS NULL OR LEN(LTRIM(RTRIM(@ProductCode))) = 0
    BEGIN
        SET @StatusCode = 2
        SET @StatusMessage = 'Invalid product code'
        RETURN 2
    END
    
    -- Validate quantity
    IF @Quantity <= 0
    BEGIN
        SET @StatusCode = 3
        SET @StatusMessage = 'Quantity must be positive'
        RETURN 3
    END
    
    IF @Quantity > 1000
    BEGIN
        SET @StatusCode = 4
        SET @StatusMessage = 'Quantity exceeds maximum allowed (1000)'
        RETURN 4
    END
    
    -- Validate unit price
    IF @UnitPrice <= 0
    BEGIN
        SET @StatusCode = 5
        SET @StatusMessage = 'Invalid unit price'
        RETURN 5
    END
    
    -- Calculate subtotal
    SET @Subtotal = @UnitPrice * @Quantity
    SET @TotalAmount = @Subtotal
    
    -- Apply promo code discount
    IF @PromoCode IS NOT NULL AND LEN(@PromoCode) > 0
    BEGIN
        IF UPPER(@PromoCode) = 'SAVE10'
            SET @DiscountPercent = 10
        ELSE IF UPPER(@PromoCode) = 'SAVE20'
            SET @DiscountPercent = 20
        ELSE IF UPPER(@PromoCode) = 'HALFOFF'
            SET @DiscountPercent = 50
        ELSE
        BEGIN
            -- Invalid promo code - continue without discount
            SET @StatusMessage = 'Note: Invalid promo code ignored. '
        END
    END
    
    -- Apply volume discount (stacks with promo)
    IF @Quantity >= 100
        SET @DiscountPercent = @DiscountPercent + 15
    ELSE IF @Quantity >= 50
        SET @DiscountPercent = @DiscountPercent + 10
    ELSE IF @Quantity >= 25
        SET @DiscountPercent = @DiscountPercent + 5
    
    -- Cap discount at 60%
    IF @DiscountPercent > 60
        SET @DiscountPercent = 60
    
    SET @Discount = @Subtotal * (@DiscountPercent / 100)
    
    -- Calculate shipping
    IF @ShippingMethod = 1
    BEGIN
        -- Standard shipping
        SET @ShippingCost = 5.99
    END
    ELSE IF @ShippingMethod = 2
    BEGIN
        -- Express shipping
        SET @ShippingCost = 12.99
    END
    ELSE IF @ShippingMethod = 3
    BEGIN
        -- Overnight shipping
        SET @ShippingCost = 24.99
    END
    ELSE
    BEGIN
        SET @StatusCode = 6
        SET @StatusMessage = 'Invalid shipping method'
        RETURN 6
    END
    
    -- Free shipping for orders over threshold
    IF (@Subtotal - @Discount) >= @FreeShippingThreshold
    BEGIN
        SET @ShippingCost = 0
        SET @StatusMessage = @StatusMessage + 'Free shipping applied! '
    END
    
    -- Calculate final total
    SET @FinalTotal = @Subtotal - @Discount + @ShippingCost
    
    -- Success
    SET @StatusCode = 0
    SET @StatusMessage = @StatusMessage + 'Order processed successfully'
    RETURN 0
END
