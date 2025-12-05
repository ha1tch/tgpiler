# T-SQL Structured Data Test Battery

Test stored procedures covering JSON and XML processing patterns commonly found in enterprise T-SQL applications.

## JSON Procedures (01-10)

| File | Procedure | Features Tested |
|------|-----------|-----------------|
| 01 | ParseCustomerJson | `JSON_VALUE`, scalar extraction, nested paths |
| 02 | ParseOrderJson | Deep nesting, array indexing `[0]`, `JSON_QUERY` |
| 03 | ShredJsonArray | Basic `OPENJSON` (key, value, type columns) |
| 04 | ParseProductsJson | `OPENJSON WITH` schema, type conversion |
| 05 | UpdateCustomerJson | `JSON_MODIFY` - update, add properties |
| 06 | BuildOrdersJson | `FOR JSON PATH`, temp tables |
| 07 | BuildEmployeeDirectory | `FOR JSON PATH, ROOT()`, nested path aliases |
| 08 | ProcessApiPayload | `ISJSON` validation, conditional processing |
| 09 | CalculateOrderTotals | JSON aggregation, window functions |
| 10 | MergeCustomerData | JSON document merging with `JSON_MODIFY` |

## XML Procedures (11-20)

| File | Procedure | Features Tested |
|------|-----------|-----------------|
| 11 | ParseCustomerXml | `.value()` method, scalar extraction |
| 12 | ParseProductXmlAttributes | XML attributes with `@` syntax |
| 13 | ValidateOrderXml | `.exist()` method, structure validation |
| 14 | ShredOrderItems | `.nodes()` method, row shredding |
| 15 | ImportEmployeesXml | `OPENXML`, `sp_xml_preparedocument` |
| 16 | ExportCustomersXml | `FOR XML RAW`, `ROOT()` |
| 17 | ExportOrdersXml | `FOR XML PATH`, nested elements |
| 18 | ExtractXmlFragments | `.query()` method, fragment extraction |
| 19 | CalculateXmlOrderTotals | XML aggregation, window functions |
| 20 | UpdateOrderXml | `.modify()` DML (insert, replace) |

## Conversion Procedures (21-22)

| File | Procedure | Features Tested |
|------|-----------|-----------------|
| 21 | ConvertXmlToJson | XML → JSON conversion pipeline |
| 22 | ConvertJsonToXml | JSON → XML conversion pipeline |

## Enterprise Patterns (23-25)

| File | Procedure | Features Tested |
|------|-----------|-----------------|
| 23 | ParseAppConfig | Configuration parsing, feature flags, defaults |
| 24 | ProcessInvoiceXml | Invoice processing, header/line item pattern |
| 25 | ProcessApiResponse | REST API response handling, pagination, errors |

## Sample JSON for Testing

### 01 - ParseCustomerJson
```json
{
  "customer": {
    "id": 123,
    "name": "John Doe",
    "email": "john@example.com"
  }
}
```

### 04 - ParseProductsJson
```json
{
  "products": [
    {"id": 1, "name": "Widget A", "price": 19.99, "qty": 5, "category": "Electronics"},
    {"id": 2, "name": "Widget B", "price": 29.99, "qty": 3, "category": "Electronics"}
  ]
}
```

### 23 - ParseAppConfig
```json
{
  "application": {"name": "MyApp", "environment": "production"},
  "database": {"connectionString": "Server=..."},
  "cache": {"enabled": true, "ttlSeconds": 600},
  "logging": {"level": "INFO"},
  "retryPolicy": {"maxRetries": 5},
  "featureFlags": [
    {"name": "newUI", "enabled": true, "description": "New user interface"},
    {"name": "betaFeature", "enabled": false, "description": "Beta testing"}
  ]
}
```

## Sample XML for Testing

### 11 - ParseCustomerXml
```xml
<customer>
  <id>123</id>
  <name>John Doe</name>
  <email>john@example.com</email>
</customer>
```

### 14 - ShredOrderItems
```xml
<order>
  <items>
    <item id="1"><product>Widget A</product><quantity>2</quantity><unitPrice>19.99</unitPrice></item>
    <item id="2"><product>Widget B</product><quantity>1</quantity><unitPrice>29.99</unitPrice></item>
  </items>
</order>
```

### 24 - ProcessInvoiceXml
```xml
<invoice number="INV-2024-001">
  <header>
    <invoiceDate>2024-01-15</invoiceDate>
    <customerId>100</customerId>
  </header>
  <lineItems>
    <item>
      <productCode>PROD-001</productCode>
      <description>Widget A</description>
      <quantity>5</quantity>
      <unitPrice>19.99</unitPrice>
    </item>
  </lineItems>
  <tax rate="0.0825"/>
</invoice>
```

## Coverage Summary

- **JSON Functions**: JSON_VALUE, JSON_QUERY, JSON_MODIFY, ISJSON, OPENJSON, FOR JSON PATH/ROOT
- **XML Methods**: .value(), .query(), .exist(), .nodes(), .modify()
- **XML Functions**: OPENXML, FOR XML RAW/PATH/ROOT
- **Patterns**: Validation, aggregation, conversion, merging, configuration, API responses

## Transpilation Results

After running `tgpiler` on all 25 procedures:

### Without DML Mode (9/25 = 36%)

Passing procedures use pure scalar operations that don't require database access:
- JSON_VALUE, JSON_QUERY, JSON_MODIFY, ISJSON
- XML .value(), .query(), .exist(), .modify()

### With DML Mode (25/25 = 100%)

Using `tgpiler --dml`, nearly all procedures transpile:

| # | File | Features |
|---|------|----------|
| 01 | ParseCustomerJson | JSON_VALUE |
| 02 | ParseOrderJson | JSON_VALUE nested, JSON_QUERY, ISJSON |
| 03 | ShredJsonArray | OPENJSON, SELECT |
| 04 | ParseProductsJson | OPENJSON WITH schema |
| 05 | UpdateCustomerJson | JSON_MODIFY |
| 06 | BuildOrdersJson | CREATE TABLE #temp, INSERT, FOR JSON |
| 07 | BuildEmployeeDirectory | CREATE TABLE #temp, FOR JSON ROOT |
| 08 | ProcessApiPayload | CREATE TABLE #temp, ISJSON validation |
| 09 | CalculateOrderTotals | CREATE TABLE #temp, aggregation |
| 10 | MergeCustomerData | JSON_VALUE, JSON_QUERY, JSON_MODIFY |
| 11 | ParseCustomerXml | .value() method |
| 12 | ParseProductXmlAttributes | .value() with @attributes |
| 13 | ValidateOrderXml | .exist() method |
| 14 | ShredOrderItems | .nodes() method, SELECT |
| 15 | ImportEmployeesXml | OPENXML WITH schema, sp_xml_preparedocument |
| 16 | ExportCustomersXml | CREATE TABLE #temp, FOR XML RAW |
| 17 | ExportOrdersXml | CREATE TABLE #temp, FOR XML PATH |
| 18 | ExtractXmlFragments | .query() method |
| 19 | CalculateXmlOrderTotals | CREATE TABLE #temp, XML aggregation |
| 20 | UpdateOrderXml | .modify() method, .exist() |
| 21 | ConvertXmlToJson | CREATE TABLE #temp, XML to JSON |
| 22 | ConvertJsonToXml | SET with subquery, FOR XML |
| 23 | ParseAppConfig | RAISERROR, JSON parsing |
| 24 | ProcessInvoiceXml | CREATE TABLE #temp, XML invoice |
| 25 | ProcessApiResponse | CREATE TABLE #temp, OPENJSON |

### Still Failing (0/25)

All 25 procedures now transpile successfully with `--dml` mode!

### DML Mode Features Added

- `--dml` flag enables DML mode
- `--dialect` specifies SQL dialect (postgres, mysql, sqlite, sqlserver)
- `--store` specifies store variable name (default: r.db)
- CREATE TABLE #temp generates tsqlruntime.TempTableManager calls
- DROP TABLE #temp supported
- TRUNCATE TABLE #temp supported
- RAISERROR generates fmt.Errorf()
- THROW generates error returns
- SET @var = (SELECT ...) subqueries supported
