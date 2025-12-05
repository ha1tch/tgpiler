-- Extract scalar values from XML using .value()
CREATE PROCEDURE dbo.ParseCustomerXml
    @XmlData XML,
    @CustomerName NVARCHAR(100) OUTPUT,
    @CustomerId INT OUTPUT,
    @Email NVARCHAR(200) OUTPUT
AS
BEGIN
    SET @CustomerName = @XmlData.value('(/customer/name)[1]', 'NVARCHAR(100)')
    SET @CustomerId = @XmlData.value('(/customer/id)[1]', 'INT')
    SET @Email = @XmlData.value('(/customer/email)[1]', 'NVARCHAR(200)')
END
