// Package mock provides a mock server for testing generated storage layer code.
package mock

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// MockStore provides an in-memory data store for testing
type MockStore struct {
	mu     sync.RWMutex
	tables map[string][]map[string]interface{}
	nextID map[string]int64
}

// NewMockStore creates a new mock store
func NewMockStore() *MockStore {
	return &MockStore{
		tables: make(map[string][]map[string]interface{}),
		nextID: make(map[string]int64),
	}
}

// Insert adds a record to a table
func (s *MockStore) Insert(table string, record map[string]interface{}) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if _, exists := s.tables[table]; !exists {
		s.tables[table] = make([]map[string]interface{}, 0)
		s.nextID[table] = 1
	}
	
	// Auto-assign ID if not present
	if _, hasID := record["ID"]; !hasID {
		record["ID"] = s.nextID[table]
	}
	id := s.nextID[table]
	s.nextID[table]++
	
	// Add timestamps
	now := time.Now()
	if _, has := record["CreatedAt"]; !has {
		record["CreatedAt"] = now
	}
	
	s.tables[table] = append(s.tables[table], record)
	return id, nil
}

// Select retrieves records from a table
func (s *MockStore) Select(table string, where map[string]interface{}) ([]map[string]interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	records, exists := s.tables[table]
	if !exists {
		return []map[string]interface{}{}, nil
	}
	
	if len(where) == 0 {
		return records, nil
	}
	
	// Filter records
	var result []map[string]interface{}
	for _, record := range records {
		if matchesWhere(record, where) {
			result = append(result, record)
		}
	}
	
	return result, nil
}

// SelectOne retrieves a single record
func (s *MockStore) SelectOne(table string, where map[string]interface{}) (map[string]interface{}, error) {
	records, err := s.Select(table, where)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, nil
	}
	return records[0], nil
}

// Update modifies records in a table
func (s *MockStore) Update(table string, set map[string]interface{}, where map[string]interface{}) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	records, exists := s.tables[table]
	if !exists {
		return 0, nil
	}
	
	var count int64
	now := time.Now()
	
	for i, record := range records {
		if matchesWhere(record, where) {
			for k, v := range set {
				records[i][k] = v
			}
			records[i]["UpdatedAt"] = now
			count++
		}
	}
	
	return count, nil
}

// Delete removes records from a table
func (s *MockStore) Delete(table string, where map[string]interface{}) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	records, exists := s.tables[table]
	if !exists {
		return 0, nil
	}
	
	var remaining []map[string]interface{}
	var deleted int64
	
	for _, record := range records {
		if matchesWhere(record, where) {
			deleted++
		} else {
			remaining = append(remaining, record)
		}
	}
	
	s.tables[table] = remaining
	return deleted, nil
}

// Truncate removes all records from a table
func (s *MockStore) Truncate(table string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.tables[table] = make([]map[string]interface{}, 0)
	return nil
}

// matchesWhere checks if a record matches the where conditions
func matchesWhere(record, where map[string]interface{}) bool {
	for k, v := range where {
		if record[k] != v {
			return false
		}
	}
	return true
}

// SeedData populates the store with test data
func (s *MockStore) SeedData(table string, records []map[string]interface{}) error {
	for _, record := range records {
		if _, err := s.Insert(table, record); err != nil {
			return err
		}
	}
	return nil
}

// GetAllRecords returns all records in a table (for testing)
func (s *MockStore) GetAllRecords(table string) []map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.tables[table]
}

// Clear removes all data from all tables
func (s *MockStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tables = make(map[string][]map[string]interface{})
	s.nextID = make(map[string]int64)
}

// MockServer wraps the mock store with a service interface
type MockServer struct {
	store   *MockStore
	service *GenericService
}

// NewMockServer creates a new mock server
func NewMockServer() *MockServer {
	store := NewMockStore()
	return &MockServer{
		store:   store,
		service: NewGenericService(store),
	}
}

// Store returns the underlying mock store
func (s *MockServer) Store() *MockStore {
	return s.store
}

// Service returns the generic service
func (s *MockServer) Service() *GenericService {
	return s.service
}

// ============================================================================
// Generic service implementations
// ============================================================================

// GenericRequest represents a generic request
type GenericRequest struct {
	Table  string                 `json:"table"`
	Data   map[string]interface{} `json:"data,omitempty"`
	Where  map[string]interface{} `json:"where,omitempty"`
	Set    map[string]interface{} `json:"set,omitempty"`
	Limit  int                    `json:"limit,omitempty"`
	Offset int                    `json:"offset,omitempty"`
}

// GenericResponse represents a generic response
type GenericResponse struct {
	Success      bool                     `json:"success"`
	Records      []map[string]interface{} `json:"records,omitempty"`
	AffectedRows int64                    `json:"affected_rows,omitempty"`
	InsertedID   int64                    `json:"inserted_id,omitempty"`
	Error        string                   `json:"error,omitempty"`
}

// GenericService provides a generic CRUD interface
type GenericService struct {
	store *MockStore
}

// NewGenericService creates a new generic service
func NewGenericService(store *MockStore) *GenericService {
	return &GenericService{store: store}
}

// Execute performs a generic operation
func (s *GenericService) Execute(ctx context.Context, operation string, req GenericRequest) (*GenericResponse, error) {
	switch operation {
	case "SELECT":
		return s.handleSelect(req)
	case "INSERT":
		return s.handleInsert(req)
	case "UPDATE":
		return s.handleUpdate(req)
	case "DELETE":
		return s.handleDelete(req)
	default:
		return nil, fmt.Errorf("unknown operation: %s", operation)
	}
}

func (s *GenericService) handleSelect(req GenericRequest) (*GenericResponse, error) {
	records, err := s.store.Select(req.Table, req.Where)
	if err != nil {
		return &GenericResponse{Success: false, Error: err.Error()}, nil
	}
	
	// Apply pagination
	if req.Offset > 0 && req.Offset < len(records) {
		records = records[req.Offset:]
	}
	if req.Limit > 0 && req.Limit < len(records) {
		records = records[:req.Limit]
	}
	
	return &GenericResponse{
		Success: true,
		Records: records,
	}, nil
}

func (s *GenericService) handleInsert(req GenericRequest) (*GenericResponse, error) {
	id, err := s.store.Insert(req.Table, req.Data)
	if err != nil {
		return &GenericResponse{Success: false, Error: err.Error()}, nil
	}
	return &GenericResponse{
		Success:    true,
		InsertedID: id,
	}, nil
}

func (s *GenericService) handleUpdate(req GenericRequest) (*GenericResponse, error) {
	affected, err := s.store.Update(req.Table, req.Set, req.Where)
	if err != nil {
		return &GenericResponse{Success: false, Error: err.Error()}, nil
	}
	return &GenericResponse{
		Success:      true,
		AffectedRows: affected,
	}, nil
}

func (s *GenericService) handleDelete(req GenericRequest) (*GenericResponse, error) {
	affected, err := s.store.Delete(req.Table, req.Where)
	if err != nil {
		return &GenericResponse{Success: false, Error: err.Error()}, nil
	}
	return &GenericResponse{
		Success:      true,
		AffectedRows: affected,
	}, nil
}

// ============================================================================
// Test helpers
// ============================================================================

// TestUserData provides sample user data for testing
func TestUserData() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"Username":  "john_doe",
			"Email":     "john@example.com",
			"FirstName": "John",
			"LastName":  "Doe",
			"IsActive":  true,
		},
		{
			"Username":  "jane_doe",
			"Email":     "jane@example.com",
			"FirstName": "Jane",
			"LastName":  "Doe",
			"IsActive":  true,
		},
		{
			"Username":  "bob_smith",
			"Email":     "bob@example.com",
			"FirstName": "Bob",
			"LastName":  "Smith",
			"IsActive":  false,
		},
	}
}

// TestOrderData provides sample order data for testing
func TestOrderData() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"OrderNumber": "ORD-001",
			"UserID":      int64(1),
			"Status":      "Completed",
			"Subtotal":    100.00,
			"TaxAmount":   10.00,
			"Total":       110.00,
		},
		{
			"OrderNumber": "ORD-002",
			"UserID":      int64(1),
			"Status":      "Pending",
			"Subtotal":    50.00,
			"TaxAmount":   5.00,
			"Total":       55.00,
		},
		{
			"OrderNumber": "ORD-003",
			"UserID":      int64(2),
			"Status":      "Completed",
			"Subtotal":    200.00,
			"TaxAmount":   20.00,
			"Total":       220.00,
		},
	}
}

// TestProductData provides sample product data for testing
func TestProductData() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"SKU":      "PROD-001",
			"Name":     "Widget A",
			"Price":    29.99,
			"IsActive": true,
		},
		{
			"SKU":      "PROD-002",
			"Name":     "Widget B",
			"Price":    49.99,
			"IsActive": true,
		},
		{
			"SKU":      "PROD-003",
			"Name":     "Gadget X",
			"Price":    99.99,
			"IsActive": false,
		},
	}
}

// SetupTestStore creates a store with test data
func SetupTestStore() *MockStore {
	store := NewMockStore()
	store.SeedData("Users", TestUserData())
	store.SeedData("Orders", TestOrderData())
	store.SeedData("Products", TestProductData())
	return store
}

// ToJSON converts an interface to JSON string
func ToJSON(v interface{}) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}
