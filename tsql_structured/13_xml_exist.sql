-- Check XML structure using .exist()
CREATE PROCEDURE dbo.ValidateOrderXml
    @XmlData XML,
    @IsValid BIT OUTPUT,
    @HasCustomer BIT OUTPUT,
    @HasItems BIT OUTPUT,
    @HasShipping BIT OUTPUT,
    @ValidationMessage NVARCHAR(500) OUTPUT
AS
BEGIN
    SET @IsValid = 0
    SET @ValidationMessage = ''
    
    -- Check for required elements
    SET @HasCustomer = @XmlData.exist('/order/customer')
    SET @HasItems = @XmlData.exist('/order/items/item')
    SET @HasShipping = @XmlData.exist('/order/shipping')
    
    -- Build validation message
    IF @HasCustomer = 0
        SET @ValidationMessage = @ValidationMessage + 'Missing customer element. '
    
    IF @HasItems = 0
        SET @ValidationMessage = @ValidationMessage + 'Missing items. '
    
    IF @HasShipping = 0
        SET @ValidationMessage = @ValidationMessage + 'Missing shipping information. '
    
    -- Check for valid status attribute
    IF @XmlData.exist('/order[@status]') = 0
        SET @ValidationMessage = @ValidationMessage + 'Missing order status. '
    
    -- Set validity
    IF @HasCustomer = 1 AND @HasItems = 1 AND @HasShipping = 1 
       AND @XmlData.exist('/order[@status]') = 1
        SET @IsValid = 1
    
    IF @IsValid = 1
        SET @ValidationMessage = 'Valid order XML'
END
