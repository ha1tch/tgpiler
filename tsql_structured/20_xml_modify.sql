-- Modify XML using .modify() DML
CREATE PROCEDURE dbo.UpdateOrderXml
    @XmlData XML,
    @NewStatus NVARCHAR(20),
    @DiscountPercent DECIMAL(5,2),
    @UpdatedXml XML OUTPUT
AS
BEGIN
    SET @UpdatedXml = @XmlData
    
    -- Update order status attribute
    SET @UpdatedXml.modify('
        replace value of (/order/@status)[1]
        with sql:variable("@NewStatus")
    ')
    
    -- Add discount element if it doesn't exist
    IF @UpdatedXml.exist('/order/discount') = 0
    BEGIN
        SET @UpdatedXml.modify('
            insert <discount percent="0">0.00</discount>
            as last into (/order)[1]
        ')
    END
    
    -- Update discount value
    DECLARE @DiscountAmount DECIMAL(18,2)
    SET @DiscountAmount = @UpdatedXml.value('(/order/total)[1]', 'DECIMAL(18,2)') * @DiscountPercent / 100
    
    SET @UpdatedXml.modify('
        replace value of (/order/discount/@percent)[1]
        with sql:variable("@DiscountPercent")
    ')
    
    -- Add timestamp
    DECLARE @Now NVARCHAR(30) = CONVERT(VARCHAR(30), GETDATE(), 126)
    
    IF @UpdatedXml.exist('/order/lastModified') = 0
    BEGIN
        SET @UpdatedXml.modify('
            insert <lastModified></lastModified>
            as last into (/order)[1]
        ')
    END
    
    SET @UpdatedXml.modify('
        replace value of (/order/lastModified/text())[1]
        with sql:variable("@Now")
    ')
END
