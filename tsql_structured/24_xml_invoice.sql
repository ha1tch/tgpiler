-- Process XML invoice document
CREATE PROCEDURE dbo.ProcessInvoiceXml
    @InvoiceXml XML,
    @InvoiceNumber NVARCHAR(50) OUTPUT,
    @InvoiceDate DATE OUTPUT,
    @CustomerId INT OUTPUT,
    @Subtotal DECIMAL(18,2) OUTPUT,
    @TaxAmount DECIMAL(18,2) OUTPUT,
    @TotalAmount DECIMAL(18,2) OUTPUT,
    @LineItemCount INT OUTPUT
AS
BEGIN
    -- Extract header information
    SET @InvoiceNumber = @InvoiceXml.value('(/invoice/@number)[1]', 'NVARCHAR(50)')
    SET @InvoiceDate = @InvoiceXml.value('(/invoice/header/invoiceDate)[1]', 'DATE')
    SET @CustomerId = @InvoiceXml.value('(/invoice/header/customerId)[1]', 'INT')
    
    -- Create temp table for line items
    CREATE TABLE #LineItems (
        LineNumber INT,
        ProductCode NVARCHAR(50),
        Description NVARCHAR(200),
        Quantity INT,
        UnitPrice DECIMAL(10,2),
        LineTotal DECIMAL(18,2)
    )
    
    -- Shred line items
    INSERT INTO #LineItems
    SELECT
        ROW_NUMBER() OVER (ORDER BY (SELECT NULL)) AS LineNumber,
        Item.value('(productCode)[1]', 'NVARCHAR(50)'),
        Item.value('(description)[1]', 'NVARCHAR(200)'),
        Item.value('(quantity)[1]', 'INT'),
        Item.value('(unitPrice)[1]', 'DECIMAL(10,2)'),
        Item.value('(quantity)[1]', 'INT') * Item.value('(unitPrice)[1]', 'DECIMAL(10,2)')
    FROM @InvoiceXml.nodes('/invoice/lineItems/item') AS T(Item)
    
    -- Calculate totals
    SELECT @Subtotal = SUM(LineTotal), @LineItemCount = COUNT(*)
    FROM #LineItems
    
    -- Get tax rate from XML or default
    DECLARE @TaxRate DECIMAL(5,4)
    SET @TaxRate = ISNULL(
        @InvoiceXml.value('(/invoice/tax/@rate)[1]', 'DECIMAL(5,4)'),
        0.08
    )
    
    SET @TaxAmount = @Subtotal * @TaxRate
    SET @TotalAmount = @Subtotal + @TaxAmount
    
    -- Return line item details
    SELECT 
        LineNumber,
        ProductCode,
        Description,
        Quantity,
        UnitPrice,
        LineTotal
    FROM #LineItems
    ORDER BY LineNumber
    
    DROP TABLE #LineItems
END
