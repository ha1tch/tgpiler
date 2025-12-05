package storage

// ProtoFieldInfo describes a field in a protobuf message.
type ProtoFieldInfo struct {
	Name       string // Field name in proto (e.g., "currency_code")
	Number     int    // Field number
	ProtoType  string // Proto type (e.g., "string", "int32", "message")
	GoType     string // Generated Go type (e.g., "string", "*string", "int32", "*int32")
	
	// Optionality - critical for NULL handling
	IsOptional bool // Has 'optional' keyword in proto3
	IsRepeated bool // Is repeated field (generates slice)
	
	// For message types
	IsMessage   bool   // Is this field a message type?
	MessageType string // Full message type name (e.g., "originations.v1.dsl.Currency")
	
	// For map types
	IsMap      bool
	MapKeyType string
	MapValType string
	
	// For enums
	IsEnum   bool
	EnumType string
	
	// Metadata
	Comment string // Field comment from proto
}

// IsNullable returns true if the field can represent NULL.
// In proto3: optional scalar fields, message fields, and repeated fields can be "null".
func (f *ProtoFieldInfo) IsNullable() bool {
	// Optional scalars are nullable (generates pointer in Go)
	if f.IsOptional {
		return true
	}
	// Message fields are always nullable (nil pointer)
	if f.IsMessage {
		return true
	}
	// Repeated fields can be nil/empty
	if f.IsRepeated {
		return true
	}
	// Map fields can be nil/empty
	if f.IsMap {
		return true
	}
	// Regular proto3 scalars have zero values, not null
	return false
}

// ProtoMessageInfo describes a protobuf message.
type ProtoMessageInfo struct {
	Name       string           // Message name (e.g., "GetCurrencyByCodeRequest")
	FullName   string           // Full name with package (e.g., "originations.v1.dsl.GetCurrencyByCodeRequest")
	Package    string           // Package name
	Fields     []ProtoFieldInfo
	Comment    string           // Message comment
	
	// Nested types
	NestedMessages []ProtoMessageInfo
	NestedEnums    []ProtoEnumInfo
}

// GetField returns a field by name, or nil if not found.
func (m *ProtoMessageInfo) GetField(name string) *ProtoFieldInfo {
	for i := range m.Fields {
		if m.Fields[i].Name == name {
			return &m.Fields[i]
		}
	}
	return nil
}

// GetRequiredFields returns only required (non-optional) fields.
func (m *ProtoMessageInfo) GetRequiredFields() []ProtoFieldInfo {
	var required []ProtoFieldInfo
	for _, f := range m.Fields {
		if !f.IsNullable() {
			required = append(required, f)
		}
	}
	return required
}

// GetOptionalFields returns only optional fields.
func (m *ProtoMessageInfo) GetOptionalFields() []ProtoFieldInfo {
	var optional []ProtoFieldInfo
	for _, f := range m.Fields {
		if f.IsNullable() {
			optional = append(optional, f)
		}
	}
	return optional
}

// ProtoEnumInfo describes a protobuf enum.
type ProtoEnumInfo struct {
	Name    string
	Values  []ProtoEnumValue
	Comment string
}

// ProtoEnumValue describes an enum value.
type ProtoEnumValue struct {
	Name    string
	Number  int
	Comment string
}

// ProtoMethodInfo describes an RPC method in a service.
type ProtoMethodInfo struct {
	Name         string // Method name (e.g., "GetCurrencyByCode")
	FullName     string // Full name with service (e.g., "CatalogService.GetCurrencyByCode")
	
	// Request/Response
	RequestType  string           // Request message type name
	ResponseType string           // Response message type name
	Request      *ProtoMessageInfo // Parsed request message
	Response     *ProtoMessageInfo // Parsed response message
	
	// Streaming
	ClientStreaming bool
	ServerStreaming bool
	
	// Metadata
	Comment string
	
	// Inferred operation type
	InferredOp OperationType // Inferred from method name (Get* -> SELECT, Create* -> INSERT, etc.)
}

// InferOperationType guesses the operation type from method name.
func (m *ProtoMethodInfo) InferOperationType() OperationType {
	name := m.Name
	switch {
	case hasPrefix(name, "Get"), hasPrefix(name, "List"), hasPrefix(name, "Find"), hasPrefix(name, "Search"):
		return OpSelect
	case hasPrefix(name, "Create"), hasPrefix(name, "Add"), hasPrefix(name, "Insert"):
		return OpInsert
	case hasPrefix(name, "Update"), hasPrefix(name, "Set"), hasPrefix(name, "Modify"):
		return OpUpdate
	case hasPrefix(name, "Delete"), hasPrefix(name, "Remove"):
		return OpDelete
	default:
		return OpExec
	}
}

// hasPrefix is a simple prefix check (avoiding strings import).
func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

// ProtoServiceInfo describes a gRPC service.
type ProtoServiceInfo struct {
	Name     string            // Service name (e.g., "CatalogService")
	FullName string            // Full name with package
	Package  string            // Package name
	Methods  []ProtoMethodInfo
	Comment  string
}

// GetMethod returns a method by name, or nil if not found.
func (s *ProtoServiceInfo) GetMethod(name string) *ProtoMethodInfo {
	for i := range s.Methods {
		if s.Methods[i].Name == name {
			return &s.Methods[i]
		}
	}
	return nil
}

// GetMethodsByOperation returns methods matching an operation type.
func (s *ProtoServiceInfo) GetMethodsByOperation(op OperationType) []ProtoMethodInfo {
	var methods []ProtoMethodInfo
	for _, m := range s.Methods {
		if m.InferOperationType() == op {
			methods = append(methods, m)
		}
	}
	return methods
}

// ProtoFile represents a parsed .proto file.
type ProtoFile struct {
	Path     string              // File path
	Package  string              // Package declaration
	GoPackage string             // go_package option
	
	Imports  []string            // Import statements
	
	Services []ProtoServiceInfo
	Messages []ProtoMessageInfo
	Enums    []ProtoEnumInfo
}

// GetService returns a service by name, or nil if not found.
func (f *ProtoFile) GetService(name string) *ProtoServiceInfo {
	for i := range f.Services {
		if f.Services[i].Name == name {
			return &f.Services[i]
		}
	}
	return nil
}

// GetMessage returns a message by name, or nil if not found.
func (f *ProtoFile) GetMessage(name string) *ProtoMessageInfo {
	for i := range f.Messages {
		if f.Messages[i].Name == name {
			return &f.Messages[i]
		}
	}
	return nil
}

// SQLToProtoMapping describes how a SQL operation maps to a proto method.
type SQLToProtoMapping struct {
	// Source SQL
	Operation Operation
	
	// Target Proto
	Service     string // Service name
	Method      string // Method name
	ProtoMethod *ProtoMethodInfo
	
	// Field mappings: SQL column/variable -> Proto field
	RequestMapping  map[string]string // SQL field -> Request proto field
	ResponseMapping map[string]string // Response proto field -> SQL variable
	
	// Confidence score for automatic matching (0.0 - 1.0)
	Confidence float64
	
	// If confidence < 1.0, reasons for uncertainty
	MatchNotes []string
}

// IsHighConfidence returns true if the mapping is reliable.
func (m *SQLToProtoMapping) IsHighConfidence() bool {
	return m.Confidence >= 0.8
}

// ProtoParseResult contains all parsed proto information.
type ProtoParseResult struct {
	Files []ProtoFile
	
	// Flattened indexes for quick lookup
	AllServices map[string]*ProtoServiceInfo  // service name -> service
	AllMessages map[string]*ProtoMessageInfo  // message name -> message
	AllMethods  map[string]*ProtoMethodInfo   // "Service.Method" -> method
}

// NewProtoParseResult creates an indexed parse result.
func NewProtoParseResult(files []ProtoFile) *ProtoParseResult {
	r := &ProtoParseResult{
		Files:       files,
		AllServices: make(map[string]*ProtoServiceInfo),
		AllMessages: make(map[string]*ProtoMessageInfo),
		AllMethods:  make(map[string]*ProtoMethodInfo),
	}
	
	for i := range files {
		f := &files[i]
		for j := range f.Services {
			s := &f.Services[j]
			r.AllServices[s.Name] = s
			for k := range s.Methods {
				m := &s.Methods[k]
				key := s.Name + "." + m.Name
				r.AllMethods[key] = m
			}
		}
		for j := range f.Messages {
			m := &f.Messages[j]
			r.AllMessages[m.Name] = m
		}
	}
	
	return r
}

// FindMethodsForTable finds proto methods that might correspond to operations on a table.
func (r *ProtoParseResult) FindMethodsForTable(tableName string, opType OperationType) []*ProtoMethodInfo {
	var matches []*ProtoMethodInfo
	
	// Normalize table name for comparison
	tableNorm := normalizeForMatch(tableName)
	
	for _, method := range r.AllMethods {
		// Check if method name contains table name
		methodNorm := normalizeForMatch(method.Name)
		if containsIgnoreCase(methodNorm, tableNorm) {
			// Check operation type matches
			if method.InferOperationType() == opType {
				matches = append(matches, method)
			}
		}
	}
	
	return matches
}

// normalizeForMatch removes common prefixes/suffixes and lowercases.
func normalizeForMatch(s string) string {
	// Simple lowercase conversion without strings package
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

// containsIgnoreCase checks if s contains substr (case-insensitive).
func containsIgnoreCase(s, substr string) bool {
	s = normalizeForMatch(s)
	substr = normalizeForMatch(substr)
	
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
