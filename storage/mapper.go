package storage

import (
	"strings"
)

// ProtoToSQLMapper maps proto service methods to stored procedures.
type ProtoToSQLMapper struct {
	proto      *ProtoParseResult
	procedures []*Procedure
	mappings   map[string]*MethodMapping // "ServiceName.MethodName" -> mapping
}

// MethodMapping represents a mapping between a proto method and a stored procedure.
type MethodMapping struct {
	ServiceName   string
	MethodName    string
	Procedure     *Procedure
	ParamMappings []ParamMapping
	ResultMapping *ResultMapping
	Confidence    float64 // 0.0 - 1.0 confidence score
	MatchReason   string  // Why this match was made
}

// ParamMapping maps a proto request field to a procedure parameter.
type ParamMapping struct {
	ProtoField    string // Field name in request message
	ProtoType     string // Proto type
	ProcParam     string // Parameter name in procedure (without @)
	ProcType      string // SQL type
	GoType        string // Go type to use
	IsOptional    bool   // Proto field is optional
	HasDefault    bool   // Proc param has default
}

// ResultMapping maps procedure results to proto response message.
type ResultMapping struct {
	ResponseType    string // Proto response message name
	ResultSetIndex  int    // Which result set (0-indexed)
	NestedFieldName string // If response wraps a nested message, the field name (e.g., "customer")
	NestedTypeName  string // If response wraps a nested message, the type name (e.g., "Customer")
	FieldMappings   []FieldMapping
}

// FieldMapping maps a result column to a response field.
type FieldMapping struct {
	ColumnName  string // SQL column name
	ColumnIndex int    // Position in result set
	ProtoField  string // Field name in response message
	ProtoType   string // Proto type
	GoType      string // Go type
}

// NewProtoToSQLMapper creates a new mapper.
func NewProtoToSQLMapper(proto *ProtoParseResult, procedures []*Procedure) *ProtoToSQLMapper {
	return &ProtoToSQLMapper{
		proto:      proto,
		procedures: procedures,
		mappings:   make(map[string]*MethodMapping),
	}
}

// MapAll attempts to map all proto methods to stored procedures.
func (m *ProtoToSQLMapper) MapAll() map[string]*MethodMapping {
	for svcName, svc := range m.proto.AllServices {
		for _, method := range svc.Methods {
			key := svcName + "." + method.Name
			if mapping := m.mapMethod(svcName, &method); mapping != nil {
				m.mappings[key] = mapping
			}
		}
	}
	return m.mappings
}

// GetMapping returns the mapping for a specific method.
func (m *ProtoToSQLMapper) GetMapping(serviceName, methodName string) *MethodMapping {
	return m.mappings[serviceName+"."+methodName]
}

// mapMethod attempts to find a stored procedure for a proto method.
func (m *ProtoToSQLMapper) mapMethod(serviceName string, method *ProtoMethodInfo) *MethodMapping {
	// Try different naming conventions to find matching procedure
	candidates := m.generateProcedureCandidates(serviceName, method.Name)

	var bestMatch *Procedure
	var bestScore float64
	var matchReason string

	for _, proc := range m.procedures {
		for _, candidate := range candidates {
			if strings.EqualFold(proc.Name, candidate.name) {
				score := candidate.score
				
				// Boost score based on parameter matching
				paramScore := m.scoreParameterMatch(method, proc)
				score = (score + paramScore) / 2

				if score > bestScore {
					bestScore = score
					bestMatch = proc
					matchReason = candidate.reason
				}
			}
		}
	}

	if bestMatch == nil {
		return nil
	}

	mapping := &MethodMapping{
		ServiceName: serviceName,
		MethodName:  method.Name,
		Procedure:   bestMatch,
		Confidence:  bestScore,
		MatchReason: matchReason,
	}

	// Map parameters
	mapping.ParamMappings = m.mapParameters(method, bestMatch)

	// Map results
	mapping.ResultMapping = m.mapResults(method, bestMatch)

	return mapping
}

type procCandidate struct {
	name   string
	score  float64
	reason string
}

func (m *ProtoToSQLMapper) generateProcedureCandidates(serviceName, methodName string) []procCandidate {
	var candidates []procCandidate

	// Extract entity from service name (UserService -> User)
	entity := strings.TrimSuffix(serviceName, "Service")

	// Common stored procedure naming patterns:
	
	// 1. usp_MethodName (exact match)
	candidates = append(candidates, procCandidate{
		name:   "usp_" + methodName,
		score:  1.0,
		reason: "exact match: usp_" + methodName,
	})

	// 2. usp_EntityMethodName (e.g., usp_UserGetById for UserService.GetUser)
	candidates = append(candidates, procCandidate{
		name:   "usp_" + entity + methodName,
		score:  0.95,
		reason: "entity prefix: usp_" + entity + methodName,
	})

	// 3. Transform GetUser -> usp_GetUserById
	if strings.HasPrefix(methodName, "Get") && !strings.HasSuffix(methodName, "ById") {
		candidates = append(candidates, procCandidate{
			name:   "usp_" + methodName + "ById",
			score:  0.9,
			reason: "get by id pattern",
		})
	}

	// 4. Transform GetUserByEmail -> usp_GetUserByEmail
	candidates = append(candidates, procCandidate{
		name:   "usp_" + methodName,
		score:  0.9,
		reason: "direct method name",
	})

	// 5. List* -> usp_List* or usp_Get*s
	if strings.HasPrefix(methodName, "List") {
		entity := strings.TrimPrefix(methodName, "List")
		candidates = append(candidates, procCandidate{
			name:   "usp_List" + entity,
			score:  0.85,
			reason: "list pattern",
		})
		candidates = append(candidates, procCandidate{
			name:   "usp_Get" + entity,
			score:  0.8,
			reason: "list as get pattern",
		})
	}

	// 6. Create* -> usp_Create* or usp_Insert*
	if strings.HasPrefix(methodName, "Create") {
		entity := strings.TrimPrefix(methodName, "Create")
		candidates = append(candidates, procCandidate{
			name:   "usp_Create" + entity,
			score:  0.9,
			reason: "create pattern",
		})
		candidates = append(candidates, procCandidate{
			name:   "usp_Insert" + entity,
			score:  0.85,
			reason: "insert pattern",
		})
		candidates = append(candidates, procCandidate{
			name:   "usp_Add" + entity,
			score:  0.8,
			reason: "add pattern",
		})
	}

	// 7. Update* -> usp_Update*
	if strings.HasPrefix(methodName, "Update") {
		candidates = append(candidates, procCandidate{
			name:   "usp_" + methodName,
			score:  0.9,
			reason: "update pattern",
		})
	}

	// 8. Delete* -> usp_Delete* or usp_Remove*
	if strings.HasPrefix(methodName, "Delete") {
		entity := strings.TrimPrefix(methodName, "Delete")
		candidates = append(candidates, procCandidate{
			name:   "usp_Delete" + entity,
			score:  0.9,
			reason: "delete pattern",
		})
		candidates = append(candidates, procCandidate{
			name:   "usp_Remove" + entity,
			score:  0.85,
			reason: "remove pattern",
		})
	}

	// 9. Validate* -> usp_Validate*
	if strings.HasPrefix(methodName, "Validate") {
		candidates = append(candidates, procCandidate{
			name:   "usp_" + methodName,
			score:  0.9,
			reason: "validate pattern",
		})
	}

	// 10. Process* -> usp_Process*
	if strings.HasPrefix(methodName, "Process") {
		candidates = append(candidates, procCandidate{
			name:   "usp_" + methodName,
			score:  0.9,
			reason: "process pattern",
		})
	}

	return candidates
}

func (m *ProtoToSQLMapper) scoreParameterMatch(method *ProtoMethodInfo, proc *Procedure) float64 {
	if len(proc.Parameters) == 0 {
		return 0.5 // No parameters to match
	}

	// Get request message fields
	reqMsg := m.proto.AllMessages[method.RequestType]
	if reqMsg == nil {
		return 0.3
	}

	matched := 0
	total := 0

	// For each procedure parameter (excluding OUTPUT params)
	for _, param := range proc.Parameters {
		if param.IsOutput {
			continue
		}
		if param.HasDefault {
			continue // Optional params don't need to match
		}
		total++

		// Try to find matching proto field
		paramLower := strings.ToLower(param.Name)
		for _, field := range reqMsg.Fields {
			fieldLower := strings.ToLower(field.Name)
			
			// Exact match
			if paramLower == fieldLower {
				matched++
				break
			}
			
			// Common variations: UserId -> user_id, Id -> id
			if paramLower == strings.ReplaceAll(fieldLower, "_", "") {
				matched++
				break
			}
			
			// Parameter without entity prefix: UserId matches id field
			if strings.HasSuffix(paramLower, "id") && fieldLower == "id" {
				matched++
				break
			}
		}
	}

	if total == 0 {
		return 0.5
	}
	return float64(matched) / float64(total)
}

func (m *ProtoToSQLMapper) mapParameters(method *ProtoMethodInfo, proc *Procedure) []ParamMapping {
	var mappings []ParamMapping

	reqMsg := m.proto.AllMessages[method.RequestType]
	if reqMsg == nil {
		return mappings
	}

	// Build lookup of proto fields
	protoFields := make(map[string]*ProtoFieldInfo)
	for i := range reqMsg.Fields {
		field := &reqMsg.Fields[i]
		protoFields[strings.ToLower(field.Name)] = field
		// Also index without underscores
		protoFields[strings.ReplaceAll(strings.ToLower(field.Name), "_", "")] = field
	}

	for _, param := range proc.Parameters {
		if param.IsOutput {
			continue
		}

		pm := ParamMapping{
			ProcParam:  param.Name,
			ProcType:   param.SQLType,
			GoType:     param.GoType,
			HasDefault: param.HasDefault,
		}

		paramLower := strings.ToLower(param.Name)

		// Try to find matching proto field
		if field, ok := protoFields[paramLower]; ok {
			pm.ProtoField = field.Name
			pm.ProtoType = field.ProtoType
			pm.IsOptional = field.IsOptional
		} else if field, ok := protoFields[strings.ReplaceAll(paramLower, "_", "")]; ok {
			pm.ProtoField = field.Name
			pm.ProtoType = field.ProtoType
			pm.IsOptional = field.IsOptional
		} else if strings.HasSuffix(paramLower, "id") {
			// Try matching XxxId to id field
			if field, ok := protoFields["id"]; ok {
				pm.ProtoField = field.Name
				pm.ProtoType = field.ProtoType
				pm.IsOptional = field.IsOptional
			}
		}

		mappings = append(mappings, pm)
	}

	return mappings
}

func (m *ProtoToSQLMapper) mapResults(method *ProtoMethodInfo, proc *Procedure) *ResultMapping {
	if len(proc.ResultSets) == 0 {
		return nil
	}

	respMsg := m.proto.AllMessages[method.ResponseType]
	if respMsg == nil {
		return nil
	}

	// Use the last result set - typically the success path in T-SQL procedures
	// Error paths usually RETURN early, so the final SELECT is the success case
	resultSetIndex := len(proc.ResultSets) - 1
	resultSet := proc.ResultSets[resultSetIndex]

	rm := &ResultMapping{
		ResponseType:   method.ResponseType,
		ResultSetIndex: resultSetIndex,
	}

	// Build lookup of proto fields
	// First check if response has a single nested message field (common pattern)
	// e.g., GetCustomerResponse { Customer customer = 1; }
	protoFields := make(map[string]*ProtoFieldInfo)
	nestedMsgName := ""
	
	if len(respMsg.Fields) == 1 && !isScalarType(respMsg.Fields[0].ProtoType) {
		// Single nested message field - look up that message's fields
		nestedType := respMsg.Fields[0].ProtoType
		nestedMsgName = respMsg.Fields[0].Name
		if nestedMsg, ok := m.proto.AllMessages[nestedType]; ok {
			for i := range nestedMsg.Fields {
				field := &nestedMsg.Fields[i]
				protoFields[strings.ToLower(field.Name)] = field
				protoFields[strings.ReplaceAll(strings.ToLower(field.Name), "_", "")] = field
			}
			rm.NestedFieldName = nestedMsgName
			rm.NestedTypeName = nestedType
		}
	}
	
	// If no nested message found, use direct fields
	if len(protoFields) == 0 {
		for i := range respMsg.Fields {
			field := &respMsg.Fields[i]
			protoFields[strings.ToLower(field.Name)] = field
			protoFields[strings.ReplaceAll(strings.ToLower(field.Name), "_", "")] = field
		}
	}

	for i, col := range resultSet.Columns {
		if col.Name == "*" {
			continue // Can't map SELECT *
		}

		colLower := strings.ToLower(col.Name)
		fm := FieldMapping{
			ColumnName:  col.Name,
			ColumnIndex: i,
		}

		// Try to find matching proto field
		if field, ok := protoFields[colLower]; ok {
			fm.ProtoField = field.Name
			fm.ProtoType = field.ProtoType
			fm.GoType = protoTypeToGo(field.ProtoType)
		} else if field, ok := protoFields[strings.ReplaceAll(colLower, "_", "")]; ok {
			fm.ProtoField = field.Name
			fm.ProtoType = field.ProtoType
			fm.GoType = protoTypeToGo(field.ProtoType)
		}

		rm.FieldMappings = append(rm.FieldMappings, fm)
	}

	return rm
}

// isScalarType returns true if the type is a protobuf scalar type
func isScalarType(t string) bool {
	switch t {
	case "double", "float", "int32", "int64", "uint32", "uint64",
		"sint32", "sint64", "fixed32", "fixed64", "sfixed32", "sfixed64",
		"bool", "string", "bytes":
		return true
	}
	return false
}

func protoTypeToGo(protoType string) string {
	switch protoType {
	case "int32", "sint32", "sfixed32":
		return "int32"
	case "int64", "sint64", "sfixed64":
		return "int64"
	case "uint32", "fixed32":
		return "uint32"
	case "uint64", "fixed64":
		return "uint64"
	case "float":
		return "float32"
	case "double":
		return "float64"
	case "bool":
		return "bool"
	case "string":
		return "string"
	case "bytes":
		return "[]byte"
	default:
		if strings.HasPrefix(protoType, "google.protobuf.Timestamp") {
			return "time.Time"
		}
		return "*" + protoType // Message type
	}
}

// MappingStats returns statistics about the mapping results.
type MappingStats struct {
	TotalMethods    int
	MappedMethods   int
	UnmappedMethods int
	HighConfidence  int // > 0.8
	MediumConfidence int // 0.5 - 0.8
	LowConfidence   int // < 0.5
	ByService       map[string]ServiceMappingStats
}

type ServiceMappingStats struct {
	ServiceName   string
	TotalMethods  int
	MappedMethods int
	Mappings      []*MethodMapping
}

// GetStats returns mapping statistics.
func (m *ProtoToSQLMapper) GetStats() MappingStats {
	stats := MappingStats{
		ByService: make(map[string]ServiceMappingStats),
	}

	for svcName, svc := range m.proto.AllServices {
		svcStats := ServiceMappingStats{
			ServiceName:  svcName,
			TotalMethods: len(svc.Methods),
		}

		for _, method := range svc.Methods {
			stats.TotalMethods++
			key := svcName + "." + method.Name

			if mapping, ok := m.mappings[key]; ok {
				stats.MappedMethods++
				svcStats.MappedMethods++
				svcStats.Mappings = append(svcStats.Mappings, mapping)

				if mapping.Confidence > 0.8 {
					stats.HighConfidence++
				} else if mapping.Confidence >= 0.5 {
					stats.MediumConfidence++
				} else {
					stats.LowConfidence++
				}
			} else {
				stats.UnmappedMethods++
			}
		}

		stats.ByService[svcName] = svcStats
	}

	return stats
}
