# gRPC Backend and Proto Generation

tgpiler provides comprehensive gRPC support through the command line, enabling migration from database-centric architectures to horizontally-scalable microservices.

## Why gRPC with tgpiler?

**The Problem**

Enterprise applications often have decades of business logic locked in SQL Server stored procedures. This creates:

- Vertical scaling bottleneck — the database is the constraint
- Expensive licensing — SQL Server licensing costs scale with hardware
- Tight coupling — business logic is inseparable from the data layer
- Difficult testing — stored procedures are hard to unit test

**The Solution**

tgpiler extracts business logic from stored procedures and generates Go code that can:

- Deploy as stateless microservices
- Scale horizontally across containers
- Use any database backend (Postgres, MySQL, SQLite)
- Expose gRPC APIs with protobuf contracts
- Run in Kubernetes with standard observability

The database becomes a dumb persistence layer. You scale by adding pods, not buying bigger iron.

## When to Use Each Feature

| Scenario | Command |
|----------|---------|
| Transpile procedures to gRPC client calls | `--dml --backend=grpc` |
| Generate server stubs from proto files | `--gen-server --proto <file>` |
| Generate repository implementations with procedure mappings | `--gen-impl --proto <file> --sql-dir <path>` |
| Preview procedure-to-method mappings | `--show-mappings --proto <file> --sql-dir <path>` |
| Generate mock server scaffolding | `--gen-mock --proto <file>` |
| Transpile procedures to mock store calls (for testing) | `--dml --backend=mock` |

## CLI Reference

### Backend Selection

When transpiling T-SQL with DML support, select the target backend:

```bash
# SQL backend (default) — generates database/sql calls
tgpiler --dml --backend=sql input.sql

# gRPC backend — generates gRPC client calls
tgpiler --dml --backend=grpc --grpc-client=catalogSvc input.sql

# Mock backend — generates mock store calls (for testing)
tgpiler --dml --backend=mock --mock-store=mockDB input.sql

# Inline backend — embeds SQL strings (for migration/debugging)
tgpiler --dml --backend=inline input.sql
```

### Backend Options

| Flag | Default | Description |
|------|---------|-------------|
| `--backend <type>` | `sql` | Backend type: `sql`, `grpc`, `mock`, `inline` |
| `--grpc-client <var>` | `client` | Variable name for gRPC client |
| `--grpc-package <path>` | (none) | Import path for generated gRPC package |
| `--mock-store <var>` | `store` | Variable name for mock store |

### Proto Generation Commands

Generate code from `.proto` files:

```bash
# Generate gRPC server stubs
tgpiler --gen-server --proto api.proto -o server.go

# Generate server stubs for all protos in a directory
tgpiler --gen-server --proto-dir ./protos -o server.go

# Generate for a specific service only
tgpiler --gen-server --proto api.proto --service CatalogService -o catalog_server.go

# Generate repository implementations with automatic procedure mapping
tgpiler --gen-impl --proto api.proto --sql-dir ./procedures -o repo.go

# Show mappings without generating code
tgpiler --show-mappings --proto api.proto --sql-dir ./procedures

# Generate mock server scaffolding
tgpiler --gen-mock --proto api.proto -o mocks.go
```

### Proto Generation Options

| Flag | Description |
|------|-------------|
| `--proto <file>` | Proto file for gRPC operations |
| `--proto-dir <path>` | Directory containing proto files |
| `--sql-dir <path>` | Directory of SQL procedure files (for mapping) |
| `--service <name>` | Target specific service (default: all) |
| `--gen-server` | Generate gRPC server stubs |
| `--gen-impl` | Generate repository implementations |
| `--gen-mock` | Generate mock server code |
| `--show-mappings` | Display procedure-to-method mappings |

## Complete Workflow Example

### Step 1: Define Proto Service

```protobuf
// protos/catalog.proto
syntax = "proto3";
package catalog.v1;
option go_package = "github.com/example/catalog/v1";

message Product {
  int64 id = 1;
  string sku = 2;
  string name = 3;
  double price = 4;
}

message GetProductRequest { int64 id = 1; }
message GetProductResponse { Product product = 1; }

message ListProductsRequest {
  int32 page_size = 1;
  int32 page_number = 2;
}
message ListProductsResponse {
  repeated Product products = 1;
  int32 total_count = 2;
}

service CatalogService {
  rpc GetProduct(GetProductRequest) returns (GetProductResponse);
  rpc ListProducts(ListProductsRequest) returns (ListProductsResponse);
}
```

### Step 2: Write Corresponding Procedures

```sql
-- procedures/catalog.sql
CREATE PROCEDURE usp_GetProductById
    @ProductId BIGINT
AS
BEGIN
    SET NOCOUNT ON
    SELECT ProductID, SKU, Name, Price
    FROM Products
    WHERE ProductID = @ProductId
END
GO

CREATE PROCEDURE usp_ListProducts
    @PageSize INT = 20,
    @PageNumber INT = 1
AS
BEGIN
    SET NOCOUNT ON
    SELECT ProductID, SKU, Name, Price
    FROM Products
    ORDER BY Name
    OFFSET (@PageNumber - 1) * @PageSize ROWS
    FETCH NEXT @PageSize ROWS ONLY
    
    SELECT COUNT(*) AS TotalCount FROM Products
END
```

### Step 3: Preview Mappings

```bash
$ tgpiler --show-mappings --proto protos/catalog.proto --sql-dir procedures/

Procedure-to-Method Mappings
============================

Service: CatalogService
  GetProduct -> usp_GetProductById (92% confidence, naming: get verb+suffix match; verb_entity: Get+Product ~ Get+Product)
  ListProducts -> usp_ListProducts (95% confidence, naming: exact match; params: params matched: 2/2; verb_entity: List+Products ~ List+Products)

Statistics:
  Total methods: 2
  Mapped: 2
  Unmapped: 0
  High confidence (>80%): 2
  Medium confidence (50-80%): 0
  Low confidence (<50%): 0
```

### Step 4: Generate Server Stubs

```bash
$ tgpiler --gen-server --proto protos/catalog.proto -p server -o server/catalog.go
```

Output:
```go
// Code generated by tgpiler. DO NOT EDIT.

package server

import (
    "context"
    "fmt"
)

// CatalogServiceServer implements the CatalogService gRPC service.
type CatalogServiceServer struct {
    repo CatalogServiceRepository
}

// NewCatalogServiceServer creates a new CatalogServiceServer.
func NewCatalogServiceServer(repo CatalogServiceRepository) *CatalogServiceServer {
    return &CatalogServiceServer{repo: repo}
}

// GetProduct handles the GetProduct RPC.
func (s *CatalogServiceServer) GetProduct(ctx context.Context, req *GetProductRequest) (*GetProductResponse, error) {
    return s.repo.GetProduct(ctx, req)
}

// ListProducts handles the ListProducts RPC.
func (s *CatalogServiceServer) ListProducts(ctx context.Context, req *ListProductsRequest) (*ListProductsResponse, error) {
    return s.repo.ListProducts(ctx, req)
}
```

### Step 5: Generate Repository Implementation

```bash
$ tgpiler --gen-impl --proto protos/catalog.proto --sql-dir procedures/ -p repository -o repository/catalog_impl.go
Generated implementations: 2 methods mapped, 0 unmapped
```

Output:
```go
// Code generated by tgpiler. DO NOT EDIT.

package repository

import (
    "context"
    "database/sql"
    "fmt"
)

// CatalogServiceRepository defines the data access interface.
type CatalogServiceRepository interface {
    GetProduct(ctx context.Context, req *GetProductRequest) (*GetProductResponse, error)
    ListProducts(ctx context.Context, req *ListProductsRequest) (*ListProductsResponse, error)
}

// CatalogServiceRepositorySQL implements CatalogServiceRepository.
type CatalogServiceRepositorySQL struct {
    db *sql.DB
}

// NewCatalogServiceRepositorySQL creates a new SQL repository.
func NewCatalogServiceRepositorySQL(db *sql.DB) *CatalogServiceRepositorySQL {
    return &CatalogServiceRepositorySQL{db: db}
}

// GetProduct implements the GetProduct operation.
// Mapped to: usp_GetProductById (confidence: 92%, naming: get verb+suffix match)
func (r *CatalogServiceRepositorySQL) GetProduct(ctx context.Context, req *GetProductRequest) (*GetProductResponse, error) {
    query := "EXEC usp_GetProductById @ProductId"
    row := r.db.QueryRowContext(ctx, query, req.Id)
    
    var result Product
    err := row.Scan(&result.Id, &result.Sku, &result.Name, &result.Price)
    if err != nil {
        if err == sql.ErrNoRows {
            return nil, fmt.Errorf("GetProduct: not found")
        }
        return nil, fmt.Errorf("GetProduct: %w", err)
    }
    
    return &GetProductResponse{Product: &result}, nil
}

// ListProducts implements the ListProducts operation.
// Mapped to: usp_ListProducts (confidence: 95%, naming: exact match)
func (r *CatalogServiceRepositorySQL) ListProducts(ctx context.Context, req *ListProductsRequest) (*ListProductsResponse, error) {
    query := "EXEC usp_ListProducts @PageSize, @PageNumber"
    rows, err := r.db.QueryContext(ctx, query, req.PageSize, req.PageNumber)
    if err != nil {
        return nil, fmt.Errorf("ListProducts: %w", err)
    }
    defer rows.Close()
    
    var products []*Product
    for rows.Next() {
        var p Product
        if err := rows.Scan(&p.Id, &p.Sku, &p.Name, &p.Price); err != nil {
            return nil, fmt.Errorf("ListProducts scan: %w", err)
        }
        products = append(products, &p)
    }
    
    return &ListProductsResponse{Products: products}, nil
}
```

### Step 6: Transpile Existing Procedures to gRPC Calls

If you have procedures that call other procedures, transpile them to use gRPC:

```bash
$ tgpiler --dml --backend=grpc --grpc-client=catalogSvc order_processing.sql
```

Input:
```sql
CREATE PROCEDURE usp_ProcessOrder
    @OrderId BIGINT
AS
BEGIN
    DECLARE @ProductId BIGINT
    
    -- Get product from catalog service
    EXEC usp_GetProductById @ProductId = @ProductId
    
    -- Process order...
END
```

Output:
```go
func ProcessOrder(orderId int64) error {
    var productId int64
    
    // gRPC call: GetProductById
    resp, err := catalogSvc.GetProduct(ctx, &GetProductRequest{
        Id: productId,
    })
    if err != nil {
        return err
    }
    _ = resp
    
    // Process order...
    return nil
}
```

## Intelligent Procedure Mapping

tgpiler uses an ensemble of four strategies to map proto methods to stored procedures:

### 1. Naming Convention Strategy

Matches by naming patterns:

| Proto Method | Procedure Patterns Matched |
|--------------|---------------------------|
| `GetProduct` | `usp_GetProduct`, `usp_GetProductById`, `GetProduct` |
| `ListProducts` | `usp_ListProducts`, `GetProducts`, `usp_GetAllProducts` |
| `CreateProduct` | `usp_CreateProduct`, `InsertProduct`, `usp_AddProduct` |
| `UpdateProduct` | `usp_UpdateProduct`, `ModifyProduct`, `usp_EditProduct` |
| `DeleteProduct` | `usp_DeleteProduct`, `RemoveProduct`, `usp_DropProduct` |

### 2. DML Table Strategy

Matches by analysing which tables procedures operate on:

- `CatalogService.GetProduct` → procedure that SELECTs from `Products` table
- `OrderService.CreateOrder` → procedure that INSERTs into `Orders` table

### 3. Parameter Signature Strategy

Matches by comparing parameter types and names:

```protobuf
message GetProductRequest { int64 id = 1; }
```

Matches:
```sql
CREATE PROCEDURE usp_GetProductById @ProductId BIGINT
```

### 4. Verb-Entity Strategy

Matches HTTP verb patterns:

| Proto Verb | SQL Verb Patterns |
|------------|-------------------|
| `Get`, `Read`, `Fetch` | SELECT single row |
| `List`, `Search`, `Find` | SELECT multiple rows |
| `Create`, `Add`, `Insert` | INSERT |
| `Update`, `Modify`, `Edit` | UPDATE |
| `Delete`, `Remove`, `Drop` | DELETE |

### Confidence Scoring

Each strategy contributes to a weighted confidence score:

- **90-100%**: Exact naming match + parameter match + table match
- **75-89%**: Strong naming match + partial parameter match
- **50-74%**: Verb pattern match or table match only
- **Below 50%**: Weak match, manual review recommended

## Best Practices

### 1. Use Consistent Naming Conventions

For high-confidence automatic mapping:

```sql
-- Good: follows usp_VerbEntity pattern
CREATE PROCEDURE usp_GetProductById ...
CREATE PROCEDURE usp_ListProducts ...
CREATE PROCEDURE usp_CreateProduct ...

-- Poor: inconsistent naming
CREATE PROCEDURE spGetProd ...
CREATE PROCEDURE ProductList ...
CREATE PROCEDURE AddNewProduct ...
```

### 2. Match Parameter Names to Proto Fields

```protobuf
message CreateProductRequest {
  string sku = 1;
  string name = 2;
  double price = 3;
}
```

```sql
-- Good: parameters match proto fields
CREATE PROCEDURE usp_CreateProduct
    @SKU VARCHAR(50),
    @Name NVARCHAR(200),
    @Price DECIMAL(18,2)

-- Poor: different naming
CREATE PROCEDURE usp_CreateProduct
    @ProductCode VARCHAR(50),
    @ProductName NVARCHAR(200),
    @UnitPrice DECIMAL(18,2)
```

### 3. Review Low-Confidence Mappings

Always check mappings below 75% confidence:

```bash
$ tgpiler --show-mappings --proto api.proto --sql-dir procedures/ | grep -E "[0-6][0-9]%"
```

### 4. Use Mock Backend for Testing

Develop and test without a database:

```bash
# Generate with mock backend
tgpiler --dml --backend=mock --mock-store=testStore input.sql -o handler_test.go
```

## Complete Example: ShopEasy

The `examples/shopeasy` directory contains a complete e-commerce example:

```
examples/shopeasy/
├── protos/
│   ├── catalog.proto      # Product and category service
│   ├── cart.proto         # Shopping cart service
│   ├── order.proto        # Order management service
│   ├── user.proto         # User account service
│   ├── inventory.proto    # Inventory tracking service
│   ├── review.proto       # Product review service
│   └── common.proto       # Shared types
├── procedures/
│   ├── catalog_service.sql
│   ├── cart_service.sql
│   ├── order_service.sql
│   ├── user_service.sql
│   ├── inventory_service.sql
│   └── review_service.sql
├── ddl/
│   └── schema.sql         # Database schema
├── generated/             # Generated server stubs
└── generated_impl/        # Generated implementations
```

Try it:

```bash
cd examples/shopeasy

# View all mappings across all services
tgpiler --show-mappings --proto-dir protos/ --sql-dir procedures/

# Generate CartService implementation
tgpiler --gen-impl --proto protos/cart.proto --sql-dir procedures/ -p cart

# Generate all server stubs
tgpiler --gen-server --proto-dir protos/ -p server -o all_servers.go
```

## Supported Proto Features

| Feature | Support |
|---------|---------|
| Messages | Full |
| Services | Full |
| Enums | Full |
| Nested messages | Full |
| Imports | Full |
| Options | Partial |
| Maps | Full |
| Repeated fields | Full |
| Optional fields | Full |
| Oneof | Partial |
| Streaming RPCs | Detection only |

## Type Mapping

### Proto to Go Types

| Proto Type | Go Type |
|------------|---------|
| `int32` | `int32` |
| `int64` | `int64` |
| `uint32` | `uint32` |
| `uint64` | `uint64` |
| `float` | `float32` |
| `double` | `float64` |
| `bool` | `bool` |
| `string` | `string` |
| `bytes` | `[]byte` |
| `google.protobuf.Timestamp` | `time.Time` |
| `repeated T` | `[]T` |
| `map<K, V>` | `map[K]V` |

### Proto to SQL Parameter Types

| Proto Type | PostgreSQL | MySQL | SQL Server |
|------------|------------|-------|------------|
| `int32` | `$n` (INTEGER) | `?` (INT) | `@param` (INT) |
| `int64` | `$n` (BIGINT) | `?` (BIGINT) | `@param` (BIGINT) |
| `string` | `$n` (TEXT) | `?` (VARCHAR) | `@param` (NVARCHAR) |
| `bool` | `$n` (BOOLEAN) | `?` (TINYINT) | `@param` (BIT) |
| `Timestamp` | `$n` (TIMESTAMP) | `?` (DATETIME) | `@param` (DATETIME2) |

## Limitations

1. **Streaming RPCs** — Detected in proto but not fully generated
2. **Complex joins** — May require manual implementation
3. **Dynamic SQL** — Cannot be statically analysed
4. **Output cursors** — Not supported for gRPC mapping
5. **XML/JSON returns** — May need custom mapping

For unsupported patterns, the generator marks methods for manual implementation:

```go
// ComplexSearch implements the ComplexSearch operation.
// Mapped to: (no automatic mapping, requires manual implementation)
func (r *CatalogServiceRepositorySQL) ComplexSearch(ctx context.Context, req *ComplexSearchRequest) (*ComplexSearchResponse, error) {
    // TODO: Manual implementation required
    return nil, fmt.Errorf("ComplexSearch: not implemented")
}
```

## See Also

- [DML.md](DML.md) — DML mode and SQL dialect support
- [MANUAL.md](MANUAL.md) — Complete tgpiler documentation
- [storage/DESIGN.md](../storage/DESIGN.md) — Storage layer architecture