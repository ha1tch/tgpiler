-- Calculate discount based on business rules
-- Applies tiered discounts based on quantity and customer tier
CREATE PROCEDURE dbo.CalculateDiscount
    @UnitPrice DECIMAL(10,2),
    @Quantity INT,
    @CustomerTier INT,
    @FinalPrice DECIMAL(10,2) OUTPUT,
    @DiscountApplied DECIMAL(10,2) OUTPUT
AS
BEGIN
    DECLARE @Subtotal DECIMAL(10,2)
    DECLARE @DiscountPercent DECIMAL(5,2) = 0
    
    -- Calculate subtotal
    SET @Subtotal = @UnitPrice * @Quantity
    
    -- Volume discount
    IF @Quantity >= 100
        SET @DiscountPercent = @DiscountPercent + 15
    ELSE IF @Quantity >= 50
        SET @DiscountPercent = @DiscountPercent + 10
    ELSE IF @Quantity >= 20
        SET @DiscountPercent = @DiscountPercent + 5
    
    -- Customer tier discount
    IF @CustomerTier = 3
        SET @DiscountPercent = @DiscountPercent + 10
    ELSE IF @CustomerTier = 2
        SET @DiscountPercent = @DiscountPercent + 5
    
    -- Cap total discount at 25%
    IF @DiscountPercent > 25
        SET @DiscountPercent = 25
    
    -- Calculate final values
    SET @DiscountApplied = @Subtotal * (@DiscountPercent / 100)
    SET @FinalPrice = @Subtotal - @DiscountApplied
END
