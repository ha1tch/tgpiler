-- Shred XML into rows using .nodes()
CREATE PROCEDURE dbo.ShredOrderItems
    @XmlData XML
AS
BEGIN
    SELECT
        Item.value('@id', 'INT') AS ItemId,
        Item.value('(product)[1]', 'NVARCHAR(100)') AS ProductName,
        Item.value('(quantity)[1]', 'INT') AS Quantity,
        Item.value('(unitPrice)[1]', 'DECIMAL(10,2)') AS UnitPrice,
        Item.value('(quantity)[1]', 'INT') * 
            Item.value('(unitPrice)[1]', 'DECIMAL(10,2)') AS LineTotal
    FROM @XmlData.nodes('/order/items/item') AS T(Item)
    ORDER BY ItemId
END
