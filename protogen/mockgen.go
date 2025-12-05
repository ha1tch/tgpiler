package protogen

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/ha1tch/tgpiler/storage"
)

// MockServer is a dynamic mock server generated from proto definitions.
type MockServer struct {
	mu       sync.RWMutex
	proto    *storage.ProtoParseResult
	data     map[string][]map[string]interface{} // table -> records
	handlers map[string]MethodHandler            // "Service.Method" -> handler
	nextID   map[string]int64
	hooks    *MockHooks
}

// MethodHandler handles a single RPC method.
type MethodHandler func(ctx context.Context, req map[string]interface{}) (map[string]interface{}, error)

// MockHooks allows customising mock behaviour.
type MockHooks struct {
	// BeforeCall is invoked before each method call
	BeforeCall func(service, method string, req map[string]interface{})

	// AfterCall is invoked after each method call
	AfterCall func(service, method string, req, resp map[string]interface{}, err error)

	// OnNotFound is invoked when a method has no handler
	OnNotFound func(service, method string, req map[string]interface{}) (map[string]interface{}, error)
}

// NewMockServer creates a mock server from proto definitions.
func NewMockServer(proto *storage.ProtoParseResult) *MockServer {
	ms := &MockServer{
		proto:    proto,
		data:     make(map[string][]map[string]interface{}),
		handlers: make(map[string]MethodHandler),
		nextID:   make(map[string]int64),
		hooks:    &MockHooks{},
	}

	// Register default handlers for all methods
	for _, svc := range proto.AllServices {
		for _, method := range svc.Methods {
			ms.registerDefaultHandler(svc, &method)
		}
	}

	return ms
}

// SetHooks sets the mock hooks.
func (s *MockServer) SetHooks(hooks *MockHooks) {
	s.hooks = hooks
}

// RegisterHandler registers a custom handler for a method.
func (s *MockServer) RegisterHandler(service, method string, handler MethodHandler) {
	key := service + "." + method
	s.handlers[key] = handler
}

// Call invokes an RPC method.
func (s *MockServer) Call(ctx context.Context, service, method string, req map[string]interface{}) (map[string]interface{}, error) {
	key := service + "." + method

	// Before hook
	if s.hooks.BeforeCall != nil {
		s.hooks.BeforeCall(service, method, req)
	}

	// Find handler
	handler, ok := s.handlers[key]
	if !ok {
		if s.hooks.OnNotFound != nil {
			resp, err := s.hooks.OnNotFound(service, method, req)
			if s.hooks.AfterCall != nil {
				s.hooks.AfterCall(service, method, req, resp, err)
			}
			return resp, err
		}
		return nil, fmt.Errorf("no handler for %s", key)
	}

	// Call handler
	resp, err := handler(ctx, req)

	// After hook
	if s.hooks.AfterCall != nil {
		s.hooks.AfterCall(service, method, req, resp, err)
	}

	return resp, err
}

// SeedData populates a table with records.
func (s *MockServer) SeedData(table string, records []map[string]interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.data[table]; !exists {
		s.data[table] = make([]map[string]interface{}, 0)
		s.nextID[table] = 1
	}

	for _, rec := range records {
		s.insertRecord(table, rec)
	}
}

// GetData returns all records for a table.
func (s *MockServer) GetData(table string) []map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.findTable(table)
}

// ClearData removes all data.
func (s *MockServer) ClearData() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = make(map[string][]map[string]interface{})
	s.nextID = make(map[string]int64)
}

// registerDefaultHandler creates a default handler based on method name patterns.
func (s *MockServer) registerDefaultHandler(svc *storage.ProtoServiceInfo, method *storage.ProtoMethodInfo) {
	key := svc.Name + "." + method.Name

	// Infer the table name from request/response types
	tableName := inferTableName(method)

	switch method.InferredOp {
	case storage.OpSelect:
		if isListMethod(method.Name) {
			s.handlers[key] = s.makeListHandler(tableName, method)
		} else {
			s.handlers[key] = s.makeGetHandler(tableName, method)
		}
	case storage.OpInsert:
		s.handlers[key] = s.makeCreateHandler(tableName, method)
	case storage.OpUpdate:
		s.handlers[key] = s.makeUpdateHandler(tableName, method)
	case storage.OpDelete:
		s.handlers[key] = s.makeDeleteHandler(tableName, method)
	default:
		s.handlers[key] = s.makeGenericHandler(tableName, method)
	}
}

// makeGetHandler creates a handler for Get* methods.
func (s *MockServer) makeGetHandler(table string, method *storage.ProtoMethodInfo) MethodHandler {
	return func(ctx context.Context, req map[string]interface{}) (map[string]interface{}, error) {
		s.mu.RLock()
		defer s.mu.RUnlock()

		// Try multiple table name variants
		records := s.findTable(table)
		for _, rec := range records {
			if matchesRequest(rec, req) {
				return wrapResponse(method.ResponseType, rec), nil
			}
		}

		return nil, fmt.Errorf("not found")
	}
}

// makeListHandler creates a handler for List* methods.
func (s *MockServer) makeListHandler(table string, method *storage.ProtoMethodInfo) MethodHandler {
	return func(ctx context.Context, req map[string]interface{}) (map[string]interface{}, error) {
		s.mu.RLock()
		defer s.mu.RUnlock()

		records := s.findTable(table)
		var filtered []map[string]interface{}

		for _, rec := range records {
			if matchesFilters(rec, req) {
				filtered = append(filtered, rec)
			}
		}

		// Apply pagination if present
		totalCount := len(filtered)
		offset := getInt(req, "offset", 0)
		limit := getInt(req, "limit", len(filtered))
		pageSize := getInt(req, "page_size", limit)

		if pageSize > 0 && pageSize < limit {
			limit = pageSize
		}

		if offset > 0 && offset < len(filtered) {
			filtered = filtered[offset:]
		}
		if limit > 0 && limit < len(filtered) {
			filtered = filtered[:limit]
		}

		return map[string]interface{}{
			inferListFieldName(table): filtered,
			"total_count":             totalCount,
		}, nil
	}
}

// makeCreateHandler creates a handler for Create* methods.
func (s *MockServer) makeCreateHandler(table string, method *storage.ProtoMethodInfo) MethodHandler {
	return func(ctx context.Context, req map[string]interface{}) (map[string]interface{}, error) {
		s.mu.Lock()
		defer s.mu.Unlock()

		// Extract the entity from request (might be nested)
		entity := extractEntity(req, table)
		if entity == nil {
			entity = req
		}

		// Normalize table name for storage
		tableName := s.normalizeTableName(table)
		record := s.insertRecord(tableName, entity)
		return wrapResponse(method.ResponseType, record), nil
	}
}

// makeUpdateHandler creates a handler for Update* methods.
func (s *MockServer) makeUpdateHandler(table string, method *storage.ProtoMethodInfo) MethodHandler {
	return func(ctx context.Context, req map[string]interface{}) (map[string]interface{}, error) {
		s.mu.Lock()
		defer s.mu.Unlock()

		tableName, records := s.findTableWithName(table)
		for i, rec := range records {
			if matchesRequest(rec, req) {
				// Apply updates
				updates := extractEntity(req, table)
				if updates == nil {
					updates = req
				}
				for k, v := range updates {
					if k != "id" && k != "ID" {
						s.data[tableName][i][k] = v
					}
				}
				s.data[tableName][i]["updated_at"] = time.Now()
				return wrapResponse(method.ResponseType, s.data[tableName][i]), nil
			}
		}

		return nil, fmt.Errorf("not found")
	}
}

// makeDeleteHandler creates a handler for Delete* methods.
func (s *MockServer) makeDeleteHandler(table string, method *storage.ProtoMethodInfo) MethodHandler {
	return func(ctx context.Context, req map[string]interface{}) (map[string]interface{}, error) {
		s.mu.Lock()
		defer s.mu.Unlock()

		tableName, records := s.findTableWithName(table)
		var remaining []map[string]interface{}
		deleted := false

		for _, rec := range records {
			if matchesRequest(rec, req) {
				deleted = true
			} else {
				remaining = append(remaining, rec)
			}
		}

		if !deleted {
			return nil, fmt.Errorf("not found")
		}

		s.data[tableName] = remaining
		return map[string]interface{}{"success": true}, nil
	}
}

// findTable looks for a table by trying multiple name variants.
func (s *MockServer) findTable(table string) []map[string]interface{} {
	_, records := s.findTableWithName(table)
	return records
}

// findTableWithName returns both the actual table name and its records.
func (s *MockServer) findTableWithName(table string) (string, []map[string]interface{}) {
	// Try exact match
	if records, ok := s.data[table]; ok {
		return table, records
	}

	// Try lowercase
	lower := toLower(table)
	if records, ok := s.data[lower]; ok {
		return lower, records
	}

	// Try singular (remove trailing 's')
	if endsWith(table, "s") {
		singular := table[:len(table)-1]
		if records, ok := s.data[singular]; ok {
			return singular, records
		}
		singularLower := toLower(singular)
		if records, ok := s.data[singularLower]; ok {
			return singularLower, records
		}
	}

	// Try plural (add 's')
	plural := table + "s"
	if records, ok := s.data[plural]; ok {
		return plural, records
	}
	pluralLower := toLower(plural)
	if records, ok := s.data[pluralLower]; ok {
		return pluralLower, records
	}

	return table, nil
}

// normalizeTableName returns a consistent table name for storage.
func (s *MockServer) normalizeTableName(table string) string {
	// If the table already exists, use that name
	name, _ := s.findTableWithName(table)
	if s.data[name] != nil {
		return name
	}
	// Otherwise use the table name as-is
	return table
}

// makeGenericHandler creates a catch-all handler.
func (s *MockServer) makeGenericHandler(table string, method *storage.ProtoMethodInfo) MethodHandler {
	return func(ctx context.Context, req map[string]interface{}) (map[string]interface{}, error) {
		// Return an empty response of the expected type
		return map[string]interface{}{}, nil
	}
}

// insertRecord adds a record with auto-generated ID and timestamps.
func (s *MockServer) insertRecord(table string, record map[string]interface{}) map[string]interface{} {
	if _, exists := s.data[table]; !exists {
		s.data[table] = make([]map[string]interface{}, 0)
		s.nextID[table] = 1
	}

	// Clone to avoid mutation
	rec := make(map[string]interface{})
	for k, v := range record {
		rec[k] = v
	}

	// Auto-assign ID
	if _, hasID := rec["id"]; !hasID {
		if _, hasID := rec["ID"]; !hasID {
			rec["id"] = s.nextID[table]
		}
	}
	s.nextID[table]++

	// Add timestamps
	now := time.Now()
	if _, has := rec["created_at"]; !has {
		rec["created_at"] = now
	}

	s.data[table] = append(s.data[table], rec)
	return rec
}

// Helper functions

func inferTableName(method *storage.ProtoMethodInfo) string {
	// Try to extract table name from method name
	name := method.Name

	// Remove common prefixes
	for _, prefix := range []string{"Get", "List", "Create", "Update", "Delete", "Find", "Search"} {
		if len(name) > len(prefix) && name[:len(prefix)] == prefix {
			name = name[len(prefix):]
			break
		}
	}

	// Remove common suffixes
	for _, suffix := range []string{"ById", "ByID", "Request", "Response"} {
		if len(name) > len(suffix) && name[len(name)-len(suffix):] == suffix {
			name = name[:len(name)-len(suffix)]
			break
		}
	}

	// Pluralise for list operations
	if isListMethod(method.Name) && !endsWith(name, "s") {
		name += "s"
	}

	return name
}

func isListMethod(name string) bool {
	return hasPrefix(name, "List") || hasPrefix(name, "Search") || hasPrefix(name, "Find")
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func endsWith(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}

func matchesRequest(record, req map[string]interface{}) bool {
	// Match on id/ID fields
	if id, ok := req["id"]; ok {
		if rid, ok := record["id"]; ok && rid == id {
			return true
		}
		if rid, ok := record["ID"]; ok && rid == id {
			return true
		}
	}
	if id, ok := req["ID"]; ok {
		if rid, ok := record["id"]; ok && rid == id {
			return true
		}
		if rid, ok := record["ID"]; ok && rid == id {
			return true
		}
	}
	return false
}

func matchesFilters(record, req map[string]interface{}) bool {
	// Skip pagination fields
	skip := map[string]bool{"offset": true, "limit": true, "page_size": true, "page_token": true}

	for k, v := range req {
		if skip[k] {
			continue
		}
		if rv, ok := record[k]; ok {
			if !valuesEqual(rv, v) {
				return false
			}
		}
	}
	return true
}

func valuesEqual(a, b interface{}) bool {
	// Handle type mismatches (e.g., int vs int64)
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

func getInt(m map[string]interface{}, key string, defaultVal int) int {
	if v, ok := m[key]; ok {
		switch val := v.(type) {
		case int:
			return val
		case int32:
			return int(val)
		case int64:
			return int(val)
		case float64:
			return int(val)
		}
	}
	return defaultVal
}

func extractEntity(req map[string]interface{}, table string) map[string]interface{} {
	// Look for a nested field matching the table name
	lowerTable := toLower(table)
	for k, v := range req {
		if toLower(k) == lowerTable {
			if m, ok := v.(map[string]interface{}); ok {
				return m
			}
		}
	}
	return nil
}

func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		result[i] = c
	}
	return string(result)
}

func inferListFieldName(table string) string {
	// Convert "User" -> "users", "Product" -> "products"
	name := toLower(table)
	if !endsWith(name, "s") {
		name += "s"
	}
	return name
}

func wrapResponse(responseType string, data map[string]interface{}) map[string]interface{} {
	return data
}

// ============================================================================
// Request/Response helpers for typed access
// ============================================================================

// ToStruct converts a map to a struct using JSON marshaling.
func ToStruct(m map[string]interface{}, target interface{}) error {
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

// FromStruct converts a struct to a map using JSON marshaling.
func FromStruct(src interface{}) (map[string]interface{}, error) {
	data, err := json.Marshal(src)
	if err != nil {
		return nil, err
	}
	var m map[string]interface{}
	err = json.Unmarshal(data, &m)
	return m, err
}

// ============================================================================
// Service descriptor for code generation
// ============================================================================

// ServiceDescriptor provides runtime information about a service.
type ServiceDescriptor struct {
	Name    string
	Methods []MethodDescriptor
}

// MethodDescriptor provides runtime information about a method.
type MethodDescriptor struct {
	Name         string
	RequestType  reflect.Type
	ResponseType reflect.Type
	Handler      MethodHandler
}

// BuildDescriptors builds service descriptors from proto definitions.
func BuildDescriptors(proto *storage.ProtoParseResult) []ServiceDescriptor {
	var descriptors []ServiceDescriptor

	for _, svc := range proto.AllServices {
		sd := ServiceDescriptor{
			Name: svc.Name,
		}
		for _, method := range svc.Methods {
			md := MethodDescriptor{
				Name: method.Name,
			}
			sd.Methods = append(sd.Methods, md)
		}
		descriptors = append(descriptors, sd)
	}

	return descriptors
}
