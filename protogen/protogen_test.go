package protogen

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/ha1tch/tgpiler/storage"
)

// Sample proto content for testing
const testProto = `
syntax = "proto3";

package catalog.v1;

option go_package = "github.com/example/catalog/v1";

import "google/protobuf/timestamp.proto";

// Product represents a product in the catalog.
message Product {
  int64 id = 1;
  string name = 2;
  string description = 3;
  optional double price = 4;
  bool is_active = 5;
  repeated string tags = 6;
  google.protobuf.Timestamp created_at = 7;
}

message GetProductRequest {
  int64 id = 1;
}

message GetProductResponse {
  Product product = 1;
}

message ListProductsRequest {
  int32 page_size = 1;
  string page_token = 2;
  optional string category = 3;
}

message ListProductsResponse {
  repeated Product products = 1;
  string next_page_token = 2;
  int32 total_count = 3;
}

message CreateProductRequest {
  Product product = 1;
}

message CreateProductResponse {
  Product product = 1;
}

message UpdateProductRequest {
  int64 id = 1;
  Product product = 2;
}

message UpdateProductResponse {
  Product product = 1;
}

message DeleteProductRequest {
  int64 id = 1;
}

message DeleteProductResponse {
  bool success = 1;
}

// ProductStatus defines product availability.
enum ProductStatus {
  PRODUCT_STATUS_UNSPECIFIED = 0;
  PRODUCT_STATUS_ACTIVE = 1;
  PRODUCT_STATUS_DISCONTINUED = 2;
}

// CatalogService provides product catalog operations.
service CatalogService {
  // GetProduct retrieves a product by ID.
  rpc GetProduct(GetProductRequest) returns (GetProductResponse) {}
  
  // ListProducts lists products with pagination.
  rpc ListProducts(ListProductsRequest) returns (ListProductsResponse) {}
  
  // CreateProduct creates a new product.
  rpc CreateProduct(CreateProductRequest) returns (CreateProductResponse) {}
  
  // UpdateProduct updates an existing product.
  rpc UpdateProduct(UpdateProductRequest) returns (UpdateProductResponse) {}
  
  // DeleteProduct deletes a product.
  rpc DeleteProduct(DeleteProductRequest) returns (DeleteProductResponse) {}
}
`

func TestParser_ParseContent(t *testing.T) {
	parser := NewParser()
	pf, err := parser.Parse(strings.NewReader(testProto), "test.proto")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Check package
	if pf.Package != "catalog.v1" {
		t.Errorf("Expected package 'catalog.v1', got '%s'", pf.Package)
	}

	// Check go_package
	if pf.GoPackage != "github.com/example/catalog/v1" {
		t.Errorf("Expected go_package 'github.com/example/catalog/v1', got '%s'", pf.GoPackage)
	}

	// Check imports
	if len(pf.Imports) != 1 {
		t.Errorf("Expected 1 import, got %d", len(pf.Imports))
	}

	// Check messages
	if len(pf.Messages) < 7 {
		t.Errorf("Expected at least 7 messages, got %d", len(pf.Messages))
	}

	// Check Product message
	productMsg := pf.GetMessage("Product")
	if productMsg == nil {
		t.Fatal("Product message not found")
	}
	if len(productMsg.Fields) < 6 {
		t.Errorf("Expected at least 6 fields in Product, got %d", len(productMsg.Fields))
	}

	// Check field properties
	idField := productMsg.GetField("id")
	if idField == nil {
		t.Fatal("id field not found")
	}
	if idField.ProtoType != "int64" {
		t.Errorf("Expected id type 'int64', got '%s'", idField.ProtoType)
	}

	priceField := productMsg.GetField("price")
	if priceField == nil {
		t.Fatal("price field not found")
	}
	if !priceField.IsOptional {
		t.Error("price field should be optional")
	}

	tagsField := productMsg.GetField("tags")
	if tagsField == nil {
		t.Fatal("tags field not found")
	}
	if !tagsField.IsRepeated {
		t.Error("tags field should be repeated")
	}

	// Check services
	if len(pf.Services) != 1 {
		t.Errorf("Expected 1 service, got %d", len(pf.Services))
	}

	svc := pf.GetService("CatalogService")
	if svc == nil {
		t.Fatal("CatalogService not found")
	}
	if len(svc.Methods) != 5 {
		t.Errorf("Expected 5 methods, got %d", len(svc.Methods))
	}

	// Check method
	getMethod := svc.GetMethod("GetProduct")
	if getMethod == nil {
		t.Fatal("GetProduct method not found")
	}
	if getMethod.RequestType != "GetProductRequest" {
		t.Errorf("Expected request type 'GetProductRequest', got '%s'", getMethod.RequestType)
	}
	if getMethod.ResponseType != "GetProductResponse" {
		t.Errorf("Expected response type 'GetProductResponse', got '%s'", getMethod.ResponseType)
	}

	// Check inferred operation types
	if getMethod.InferredOp != storage.OpSelect {
		t.Errorf("GetProduct should infer SELECT, got %v", getMethod.InferredOp)
	}

	createMethod := svc.GetMethod("CreateProduct")
	if createMethod.InferredOp != storage.OpInsert {
		t.Errorf("CreateProduct should infer INSERT, got %v", createMethod.InferredOp)
	}

	// Check enums
	if len(pf.Enums) != 1 {
		t.Errorf("Expected 1 enum, got %d", len(pf.Enums))
	}
}

func TestParser_ProtoParseResult(t *testing.T) {
	parser := NewParser()
	pf, _ := parser.Parse(strings.NewReader(testProto), "test.proto")

	result := storage.NewProtoParseResult([]storage.ProtoFile{*pf})

	// Check indexes
	if len(result.AllServices) != 1 {
		t.Errorf("Expected 1 service in index, got %d", len(result.AllServices))
	}

	if len(result.AllMessages) < 7 {
		t.Errorf("Expected at least 7 messages in index, got %d", len(result.AllMessages))
	}

	if len(result.AllMethods) != 5 {
		t.Errorf("Expected 5 methods in index, got %d", len(result.AllMethods))
	}

	// Check method lookup
	method := result.AllMethods["CatalogService.GetProduct"]
	if method == nil {
		t.Fatal("CatalogService.GetProduct not found in index")
	}

	// Check FindMethodsForTable
	methods := result.FindMethodsForTable("Product", storage.OpSelect)
	if len(methods) == 0 {
		t.Error("Expected to find SELECT methods for Product")
	}
}

func TestMockServer_CRUD(t *testing.T) {
	parser := NewParser()
	pf, _ := parser.Parse(strings.NewReader(testProto), "test.proto")
	result := storage.NewProtoParseResult([]storage.ProtoFile{*pf})

	server := NewMockServer(result)

	// Seed data
	server.SeedData("Product", []map[string]interface{}{
		{"id": int64(1), "name": "Widget A", "price": 29.99, "is_active": true},
		{"id": int64(2), "name": "Widget B", "price": 49.99, "is_active": true},
		{"id": int64(3), "name": "Gadget X", "price": 99.99, "is_active": false},
	})

	ctx := context.Background()

	// Test GetProduct
	resp, err := server.Call(ctx, "CatalogService", "GetProduct", map[string]interface{}{"id": int64(1)})
	if err != nil {
		t.Fatalf("GetProduct failed: %v", err)
	}
	if resp["name"] != "Widget A" {
		t.Errorf("Expected 'Widget A', got '%v'", resp["name"])
	}

	// Test ListProducts
	resp, err = server.Call(ctx, "CatalogService", "ListProducts", map[string]interface{}{})
	if err != nil {
		t.Fatalf("ListProducts failed: %v", err)
	}
	// The mock returns products under the inferred field name
	var products []map[string]interface{}
	for k, v := range resp {
		if slice, ok := v.([]map[string]interface{}); ok {
			products = slice
			t.Logf("Found products under field '%s'", k)
			break
		}
	}
	if len(products) != 3 {
		t.Errorf("Expected 3 products, got %d (resp: %v)", len(products), resp)
	}

	// Test CreateProduct
	resp, err = server.Call(ctx, "CatalogService", "CreateProduct", map[string]interface{}{
		"name":      "New Product",
		"price":     19.99,
		"is_active": true,
	})
	if err != nil {
		t.Fatalf("CreateProduct failed: %v", err)
	}
	if resp["name"] != "New Product" {
		t.Errorf("Expected 'New Product', got '%v'", resp["name"])
	}

	// Verify it was added
	data := server.GetData("Product")
	if len(data) != 4 {
		t.Errorf("Expected 4 products after create, got %d", len(data))
	}

	// Test UpdateProduct
	resp, err = server.Call(ctx, "CatalogService", "UpdateProduct", map[string]interface{}{
		"id":   int64(1),
		"name": "Updated Widget",
	})
	if err != nil {
		t.Fatalf("UpdateProduct failed: %v", err)
	}

	// Verify update
	resp, _ = server.Call(ctx, "CatalogService", "GetProduct", map[string]interface{}{"id": int64(1)})
	if resp["name"] != "Updated Widget" {
		t.Errorf("Expected 'Updated Widget', got '%v'", resp["name"])
	}

	// Test DeleteProduct
	resp, err = server.Call(ctx, "CatalogService", "DeleteProduct", map[string]interface{}{"id": int64(1)})
	if err != nil {
		t.Fatalf("DeleteProduct failed: %v", err)
	}

	// Verify deletion
	data = server.GetData("Product")
	if len(data) != 3 {
		t.Errorf("Expected 3 products after delete, got %d", len(data))
	}
}

func TestMockServer_Hooks(t *testing.T) {
	parser := NewParser()
	pf, _ := parser.Parse(strings.NewReader(testProto), "test.proto")
	result := storage.NewProtoParseResult([]storage.ProtoFile{*pf})

	server := NewMockServer(result)
	server.SeedData("Product", []map[string]interface{}{
		{"id": int64(1), "name": "Widget"},
	})

	var callLog []string

	server.SetHooks(&MockHooks{
		BeforeCall: func(service, method string, req map[string]interface{}) {
			callLog = append(callLog, "before:"+method)
		},
		AfterCall: func(service, method string, req, resp map[string]interface{}, err error) {
			callLog = append(callLog, "after:"+method)
		},
	})

	ctx := context.Background()
	server.Call(ctx, "CatalogService", "GetProduct", map[string]interface{}{"id": int64(1)})

	if len(callLog) != 2 {
		t.Errorf("Expected 2 hook calls, got %d", len(callLog))
	}
	if callLog[0] != "before:GetProduct" {
		t.Errorf("Expected 'before:GetProduct', got '%s'", callLog[0])
	}
	if callLog[1] != "after:GetProduct" {
		t.Errorf("Expected 'after:GetProduct', got '%s'", callLog[1])
	}
}

func TestMockServer_CustomHandler(t *testing.T) {
	parser := NewParser()
	pf, _ := parser.Parse(strings.NewReader(testProto), "test.proto")
	result := storage.NewProtoParseResult([]storage.ProtoFile{*pf})

	server := NewMockServer(result)

	// Register custom handler
	server.RegisterHandler("CatalogService", "GetProduct", func(ctx context.Context, req map[string]interface{}) (map[string]interface{}, error) {
		return map[string]interface{}{
			"id":     req["id"],
			"name":   "Custom Response",
			"custom": true,
		}, nil
	})

	ctx := context.Background()
	resp, err := server.Call(ctx, "CatalogService", "GetProduct", map[string]interface{}{"id": int64(42)})
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}

	if resp["name"] != "Custom Response" {
		t.Errorf("Expected custom response")
	}
	if resp["custom"] != true {
		t.Errorf("Expected custom field")
	}
}

func TestServerGenerator_GenerateService(t *testing.T) {
	parser := NewParser()
	pf, _ := parser.Parse(strings.NewReader(testProto), "test.proto")
	result := storage.NewProtoParseResult([]storage.ProtoFile{*pf})

	opts := DefaultServerGenOptions()
	opts.PackageName = "catalog"

	var buf bytes.Buffer
	err := GenerateFile(result, "CatalogService", opts, &buf)
	if err != nil {
		t.Fatalf("GenerateFile failed: %v", err)
	}

	output := buf.String()

	// Check package
	if !strings.Contains(output, "package catalog") {
		t.Error("Expected 'package catalog' in output")
	}

	// Check generated code marker
	if !strings.Contains(output, "Code generated by tgpiler") {
		t.Error("Expected generated code marker")
	}

	// Check service struct
	if !strings.Contains(output, "type CatalogServiceServer struct") {
		t.Error("Expected CatalogServiceServer struct")
	}

	// Check constructor
	if !strings.Contains(output, "func NewCatalogServiceServer") {
		t.Error("Expected NewCatalogServiceServer constructor")
	}

	// Check methods
	if !strings.Contains(output, "func (s *CatalogServiceServer) GetProduct") {
		t.Error("Expected GetProduct method")
	}

	// Check repository interface
	if !strings.Contains(output, "type CatalogServiceRepository interface") {
		t.Error("Expected CatalogServiceRepository interface")
	}

	// Check stub implementation
	if !strings.Contains(output, "type CatalogServiceRepositoryStub struct") {
		t.Error("Expected CatalogServiceRepositoryStub struct")
	}

	// Check SQL implementation
	if !strings.Contains(output, "type CatalogServiceRepositorySQL struct") {
		t.Error("Expected CatalogServiceRepositorySQL struct")
	}
}

func TestParser_StreamingMethods(t *testing.T) {
	streamingProto := `
syntax = "proto3";
package streaming.v1;

message Request {}
message Response {}

service StreamService {
  rpc Unary(Request) returns (Response) {}
  rpc ServerStream(Request) returns (stream Response) {}
  rpc ClientStream(stream Request) returns (Response) {}
  rpc BidiStream(stream Request) returns (stream Response) {}
}
`
	parser := NewParser()
	pf, err := parser.Parse(strings.NewReader(streamingProto), "streaming.proto")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	svc := pf.GetService("StreamService")
	if svc == nil {
		t.Fatal("StreamService not found")
	}

	unary := svc.GetMethod("Unary")
	if unary.ClientStreaming || unary.ServerStreaming {
		t.Error("Unary should not be streaming")
	}

	serverStream := svc.GetMethod("ServerStream")
	if !serverStream.ServerStreaming {
		t.Error("ServerStream should be server streaming")
	}
	if serverStream.ClientStreaming {
		t.Error("ServerStream should not be client streaming")
	}

	clientStream := svc.GetMethod("ClientStream")
	if !clientStream.ClientStreaming {
		t.Error("ClientStream should be client streaming")
	}
	if clientStream.ServerStreaming {
		t.Error("ClientStream should not be server streaming")
	}

	bidi := svc.GetMethod("BidiStream")
	if !bidi.ClientStreaming || !bidi.ServerStreaming {
		t.Error("BidiStream should be bidirectional streaming")
	}
}

func TestParser_MapField(t *testing.T) {
	mapProto := `
syntax = "proto3";
package maps.v1;

message Config {
  map<string, string> labels = 1;
  map<int64, Value> values = 2;
}

message Value {
  string data = 1;
}
`
	parser := NewParser()
	pf, err := parser.Parse(strings.NewReader(mapProto), "maps.proto")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	config := pf.GetMessage("Config")
	if config == nil {
		t.Fatal("Config message not found")
	}

	labels := config.GetField("labels")
	if labels == nil {
		t.Fatal("labels field not found")
	}
	if !labels.IsMap {
		t.Error("labels should be a map")
	}
	if labels.MapKeyType != "string" {
		t.Errorf("Expected map key type 'string', got '%s'", labels.MapKeyType)
	}
	if labels.MapValType != "string" {
		t.Errorf("Expected map value type 'string', got '%s'", labels.MapValType)
	}
}

func TestMockServer_Pagination(t *testing.T) {
	parser := NewParser()
	pf, _ := parser.Parse(strings.NewReader(testProto), "test.proto")
	result := storage.NewProtoParseResult([]storage.ProtoFile{*pf})

	server := NewMockServer(result)

	// Seed 10 products
	var seedData []map[string]interface{}
	for i := 1; i <= 10; i++ {
		seedData = append(seedData, map[string]interface{}{
			"name":  fmt.Sprintf("Product %d", i),
			"price": float64(i * 10),
		})
	}
	server.SeedData("Product", seedData)

	ctx := context.Background()

	// Helper to extract products from response
	getProducts := func(resp map[string]interface{}) []map[string]interface{} {
		for _, v := range resp {
			if slice, ok := v.([]map[string]interface{}); ok {
				return slice
			}
		}
		return nil
	}

	// Test with limit
	resp, _ := server.Call(ctx, "CatalogService", "ListProducts", map[string]interface{}{
		"page_size": 3,
	})
	products := getProducts(resp)
	if len(products) != 3 {
		t.Errorf("Expected 3 products with page_size=3, got %d", len(products))
	}

	// Test with offset
	resp, _ = server.Call(ctx, "CatalogService", "ListProducts", map[string]interface{}{
		"offset": 5,
		"limit":  3,
	})
	products = getProducts(resp)
	if len(products) != 3 {
		t.Errorf("Expected 3 products with offset=5, limit=3, got %d", len(products))
	}
}

func BenchmarkMockServer_GetProduct(b *testing.B) {
	parser := NewParser()
	pf, _ := parser.Parse(strings.NewReader(testProto), "test.proto")
	result := storage.NewProtoParseResult([]storage.ProtoFile{*pf})

	server := NewMockServer(result)
	server.SeedData("Product", []map[string]interface{}{
		{"id": int64(1), "name": "Widget"},
	})

	ctx := context.Background()
	req := map[string]interface{}{"id": int64(1)}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		server.Call(ctx, "CatalogService", "GetProduct", req)
	}
}

func BenchmarkParser_Parse(b *testing.B) {
	parser := NewParser()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parser.Parse(strings.NewReader(testProto), "test.proto")
	}
}
