-- Convert XML data to JSON format
CREATE PROCEDURE dbo.ConvertXmlToJson
    @XmlData XML,
    @JsonResult NVARCHAR(MAX) OUTPUT
AS
BEGIN
    -- Shred XML into temp table
    CREATE TABLE #Data (
        ItemId INT,
        ItemName NVARCHAR(100),
        Quantity INT,
        Price DECIMAL(10,2)
    )
    
    INSERT INTO #Data (ItemId, ItemName, Quantity, Price)
    SELECT
        Item.value('@id', 'INT'),
        Item.value('(name)[1]', 'NVARCHAR(100)'),
        Item.value('(qty)[1]', 'INT'),
        Item.value('(price)[1]', 'DECIMAL(10,2)')
    FROM @XmlData.nodes('/items/item') AS T(Item)
    
    -- Convert to JSON
    SELECT @JsonResult = (
        SELECT 
            ItemId AS 'id',
            ItemName AS 'name',
            Quantity AS 'quantity',
            Price AS 'price',
            Quantity * Price AS 'total'
        FROM #Data
        ORDER BY ItemId
        FOR JSON PATH, ROOT('items')
    )
    
    DROP TABLE #Data
END
