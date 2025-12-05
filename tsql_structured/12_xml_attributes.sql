-- Extract values from XML attributes
CREATE PROCEDURE dbo.ParseProductXmlAttributes
    @XmlData XML,
    @ProductId INT OUTPUT,
    @ProductSku NVARCHAR(50) OUTPUT,
    @IsActive BIT OUTPUT,
    @Price DECIMAL(10,2) OUTPUT
AS
BEGIN
    -- Extract attribute values using @ syntax
    SET @ProductId = @XmlData.value('(/product/@id)[1]', 'INT')
    SET @ProductSku = @XmlData.value('(/product/@sku)[1]', 'NVARCHAR(50)')
    SET @IsActive = @XmlData.value('(/product/@active)[1]', 'BIT')
    
    -- Mix of attribute and element
    SET @Price = @XmlData.value('(/product/pricing/@current)[1]', 'DECIMAL(10,2)')
END
