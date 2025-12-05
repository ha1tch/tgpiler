-- Extract XML fragments using .query()
CREATE PROCEDURE dbo.ExtractXmlFragments
    @XmlData XML,
    @CustomerFragment XML OUTPUT,
    @ItemsFragment XML OUTPUT,
    @ShippingFragment XML OUTPUT
AS
BEGIN
    -- Extract entire customer section
    SET @CustomerFragment = @XmlData.query('/order/customer')
    
    -- Extract all items
    SET @ItemsFragment = @XmlData.query('/order/items')
    
    -- Extract shipping info
    SET @ShippingFragment = @XmlData.query('/order/shipping')
END
