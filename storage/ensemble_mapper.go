package storage

import (
	"regexp"
	"strings"
)

// MatchStrategy represents a single matching strategy.
type MatchStrategy interface {
	Name() string
	Match(method *ProtoMethodInfo, proc *Procedure, context *MatchContext) *StrategyResult
}

// MatchContext provides shared context for all strategies.
type MatchContext struct {
	ServiceName   string
	AllMessages   map[string]*ProtoMessageInfo
	AllProcedures []*Procedure
}

// StrategyResult is the output of a single strategy.
type StrategyResult struct {
	Matched    bool
	Score      float64 // 0.0 to 1.0
	Reason     string
	Confidence float64 // How confident the strategy is in its own result
}

// EnsembleMapper uses multiple strategies to map proto methods to procedures.
type EnsembleMapper struct {
	proto      *ProtoParseResult
	procedures []*Procedure
	strategies []MatchStrategy
	mappings   map[string]*MethodMapping
}

// NewEnsembleMapper creates a mapper with all available strategies.
func NewEnsembleMapper(proto *ProtoParseResult, procedures []*Procedure) *EnsembleMapper {
	return &EnsembleMapper{
		proto:      proto,
		procedures: procedures,
		strategies: []MatchStrategy{
			&NamingConventionStrategy{},
			&DMLTableStrategy{},
			&ParameterSignatureStrategy{},
			&VerbEntityStrategy{},
		},
		mappings: make(map[string]*MethodMapping),
	}
}

// MapAll maps all proto methods using ensemble of strategies.
func (m *EnsembleMapper) MapAll() map[string]*MethodMapping {
	for svcName, svc := range m.proto.AllServices {
		ctx := &MatchContext{
			ServiceName:   svcName,
			AllMessages:   m.proto.AllMessages,
			AllProcedures: m.procedures,
		}

		for _, method := range svc.Methods {
			key := svcName + "." + method.Name
			if mapping := m.mapMethodEnsemble(svcName, &method, ctx); mapping != nil {
				m.mappings[key] = mapping
			}
		}
	}
	return m.mappings
}

func (m *EnsembleMapper) mapMethodEnsemble(serviceName string, method *ProtoMethodInfo, ctx *MatchContext) *MethodMapping {
	type procScore struct {
		proc          *Procedure
		totalScore    float64
		totalWeight   float64
		strategyVotes map[string]*StrategyResult
		agreement     int // How many strategies agree
		hasExactName  bool // Has exact naming match
	}

	scores := make(map[string]*procScore)

	// Run each strategy against each procedure
	for _, proc := range m.procedures {
		ps := &procScore{
			proc:          proc,
			strategyVotes: make(map[string]*StrategyResult),
		}

		for _, strategy := range m.strategies {
			result := strategy.Match(method, proc, ctx)
			if result != nil && result.Matched {
				ps.strategyVotes[strategy.Name()] = result
				
				// Weight by strategy confidence
				ps.totalScore += result.Score * result.Confidence
				ps.totalWeight += result.Confidence
				ps.agreement++
				
				// Track if naming gave a high score (exact/verb match)
				if strategy.Name() == "naming" && result.Score >= 0.85 {
					ps.hasExactName = true
				}
			}
		}

		if ps.agreement > 0 {
			scores[proc.Name] = ps
		}
	}

	// Find best match considering agreement/disagreement
	var bestProc *procScore
	var bestFinalScore float64

	for _, ps := range scores {
		if ps.totalWeight == 0 {
			continue
		}

		// Base score from strategies
		avgScore := ps.totalScore / ps.totalWeight

		// Agreement bonus: multiple strategies agreeing boosts confidence
		agreementBonus := float64(ps.agreement-1) * 0.03 // +3% per additional agreeing strategy
		if agreementBonus > 0.12 {
			agreementBonus = 0.12 // Cap at 12%
		}

		// Strong bonus for exact naming matches - naming should be primary signal
		if ps.hasExactName {
			agreementBonus += 0.10
		}

		// Check for disagreement (strategies that matched OTHER procedures with high scores)
		disagreementPenalty := 0.0
		for _, otherPS := range scores {
			if otherPS.proc.Name == ps.proc.Name {
				continue
			}
			
			// If another procedure has an exact name match, penalize this one
			if otherPS.hasExactName && !ps.hasExactName {
				disagreementPenalty += 0.15
			}
			
			// If same strategy voted for multiple procedures, small penalty
			for stratName := range ps.strategyVotes {
				if _, hasVote := otherPS.strategyVotes[stratName]; hasVote {
					disagreementPenalty += 0.01
				}
			}
		}
		if disagreementPenalty > 0.20 {
			disagreementPenalty = 0.20 // Cap penalty
		}

		finalScore := avgScore + agreementBonus - disagreementPenalty
		if finalScore > 1.0 {
			finalScore = 1.0
		}
		if finalScore < 0 {
			finalScore = 0
		}

		if finalScore > bestFinalScore {
			bestFinalScore = finalScore
			bestProc = ps
		}
	}

	if bestProc == nil {
		return nil
	}

	// Build reason string from all contributing strategies
	var reasons []string
	for stratName, result := range bestProc.strategyVotes {
		if result.Matched {
			reasons = append(reasons, stratName+": "+result.Reason)
		}
	}

	mapping := &MethodMapping{
		ServiceName: serviceName,
		MethodName:  method.Name,
		Procedure:   bestProc.proc,
		Confidence:  bestFinalScore,
		MatchReason: strings.Join(reasons, "; "),
	}

	// Map parameters using the existing logic
	mapping.ParamMappings = mapParametersFromContext(method, bestProc.proc, ctx)
	mapping.ResultMapping = mapResultsFromContext(method, bestProc.proc, ctx)

	return mapping
}

// GetStats returns mapping statistics.
func (m *EnsembleMapper) GetStats() MappingStats {
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

// ============================================================================
// Strategy 1: Naming Convention (existing approach, refined)
// ============================================================================

type NamingConventionStrategy struct{}

func (s *NamingConventionStrategy) Name() string { return "naming" }

func (s *NamingConventionStrategy) Match(method *ProtoMethodInfo, proc *Procedure, ctx *MatchContext) *StrategyResult {
	methodName := method.Name
	procName := proc.Name

	// Normalize procedure name (remove usp_, sp_, etc.)
	normalizedProc := strings.ToLower(procName)
	for _, prefix := range []string{"usp_", "sp_", "proc_", "p_"} {
		normalizedProc = strings.TrimPrefix(normalizedProc, prefix)
	}

	normalizedMethod := strings.ToLower(methodName)

	// Exact match after normalization
	if normalizedProc == normalizedMethod {
		return &StrategyResult{
			Matched:    true,
			Score:      1.0,
			Reason:     "exact match",
			Confidence: 0.9,
		}
	}

	// Check verb-based patterns
	verbPatterns := []struct {
		methodPrefix string
		procPatterns []string
		suffix       string
	}{
		// Standard CRUD
		{"get", []string{"get", "fetch", "retrieve", "find", "load"}, "byid"},
		{"list", []string{"list", "get", "fetch", "find", "search"}, "s"},
		{"create", []string{"create", "insert", "add", "new"}, ""},
		{"update", []string{"update", "modify", "edit", "change", "set"}, ""},
		{"delete", []string{"delete", "remove", "drop"}, ""},
		{"change", []string{"change", "update", "modify", "set"}, ""}, // ChangePassword -> usp_ChangePassword
		
		// DSL-specific verbs
		{"validate", []string{"validate", "check", "verify"}, ""},
		{"isvalid", []string{"isvalid", "validate", "check"}, ""},
		{"check", []string{"check", "validate", "verify", "test"}, ""},
		{"confirm", []string{"confirm", "verify", "validate"}, ""},
		{"convert", []string{"convert", "transform", "translate"}, ""},
		{"authenticate", []string{"authenticate", "auth", "login", "verify"}, ""},
		{"authorize", []string{"authorize", "auth", "checkpermission"}, ""},
		
		// Additional common patterns
		{"search", []string{"search", "find", "query", "lookup"}, ""},
		{"process", []string{"process", "execute", "run", "handle"}, ""},
		{"calculate", []string{"calculate", "calc", "compute"}, ""},
		{"generate", []string{"generate", "create", "produce"}, ""},
		{"send", []string{"send", "transmit", "dispatch", "deliver"}, ""},
		{"receive", []string{"receive", "get", "fetch", "accept"}, ""},
		{"reserve", []string{"reserve", "hold", "lock", "allocate"}, ""},
		{"release", []string{"release", "free", "unlock", "deallocate"}, ""},
		{"cancel", []string{"cancel", "abort", "revoke", "void"}, ""},
		{"approve", []string{"approve", "accept", "authorize"}, ""},
		{"reject", []string{"reject", "deny", "decline"}, ""},
		{"refresh", []string{"refresh", "renew", "update"}, ""},
	}

	for _, vp := range verbPatterns {
		if strings.HasPrefix(normalizedMethod, vp.methodPrefix) {
			entity := strings.TrimPrefix(normalizedMethod, vp.methodPrefix)
			
			for _, procVerb := range vp.procPatterns {
				// Check: procVerb + entity
				if normalizedProc == procVerb+entity {
					return &StrategyResult{
						Matched:    true,
						Score:      0.85,
						Reason:     vp.methodPrefix + " verb match",
						Confidence: 0.8,
					}
				}
				// Check: procVerb + entity + suffix (e.g., GetUser -> GetUserById)
				if vp.suffix != "" && normalizedProc == procVerb+entity+vp.suffix {
					return &StrategyResult{
						Matched:    true,
						Score:      0.9,
						Reason:     vp.methodPrefix + " verb+suffix match",
						Confidence: 0.85,
					}
				}
			}
		}
	}

	// Fuzzy match: significant overlap
	if len(normalizedMethod) > 4 && len(normalizedProc) > 4 {
		if strings.Contains(normalizedProc, normalizedMethod) || strings.Contains(normalizedMethod, normalizedProc) {
			return &StrategyResult{
				Matched:    true,
				Score:      0.6,
				Reason:     "substring match",
				Confidence: 0.5,
			}
		}
	}

	return nil
}

// ============================================================================
// Strategy 2: DML Table Analysis
// ============================================================================

type DMLTableStrategy struct{}

func (s *DMLTableStrategy) Name() string { return "dml_table" }

func (s *DMLTableStrategy) Match(method *ProtoMethodInfo, proc *Procedure, ctx *MatchContext) *StrategyResult {
	// Extract expected entity from method name
	entity := extractEntityFromMethod(method.Name)
	if entity == "" {
		return nil
	}

	// Extract expected operation type
	expectedOp := extractOperationType(method.Name)

	// Check procedure's DML operations
	var tableMatches int
	var opMatches int
	var tables []string

	for _, op := range proc.Operations {
		tables = append(tables, op.Table)
		
		// Check if table name matches entity
		tableLower := strings.ToLower(op.Table)
		entityLower := strings.ToLower(entity)
		
		// Table matches entity (Users matches User, Products matches Product, etc.)
		if tableLower == entityLower || 
		   tableLower == entityLower+"s" || 
		   strings.TrimSuffix(tableLower, "s") == entityLower ||
		   strings.Contains(tableLower, entityLower) {
			tableMatches++
		}

		// Operation type matches
		if expectedOp != "" && strings.EqualFold(op.Type.String(), expectedOp) {
			opMatches++
		}
	}

	if tableMatches == 0 {
		return nil
	}

	score := 0.5
	if tableMatches > 0 {
		score += 0.2
	}
	if opMatches > 0 {
		score += 0.2
	}
	if tableMatches > 1 {
		score += 0.1 // Multiple table hits increase confidence
	}

	return &StrategyResult{
		Matched:    true,
		Score:      score,
		Reason:     "table: " + strings.Join(unique(tables), ","),
		Confidence: 0.7,
	}
}

func extractEntityFromMethod(methodName string) string {
	// Remove common verb prefixes
	verbs := []string{
		"Get", "List", "Create", "Update", "Delete", "Remove", "Add",
		"Find", "Search", "Fetch", "Load", "Save", "Store",
		"Validate", "Check", "Verify", "IsValid", "Is",
		"Process", "Execute", "Handle", "Run",
		"Send", "Receive", "Convert", "Transform",
		"Reserve", "Release", "Cancel", "Approve", "Reject",
		"Authenticate", "Authorize", "Login", "Logout",
		"Refresh", "Generate", "Calculate", "Compute",
	}

	entity := methodName
	for _, verb := range verbs {
		if strings.HasPrefix(methodName, verb) {
			entity = strings.TrimPrefix(methodName, verb)
			break
		}
	}

	// Remove common suffixes
	suffixes := []string{"ById", "ByEmail", "BySlug", "BySku", "ByCode", "ByNumber", "ByName"}
	for _, suffix := range suffixes {
		entity = strings.TrimSuffix(entity, suffix)
	}

	return entity
}

func extractOperationType(methodName string) string {
	methodLower := strings.ToLower(methodName)
	
	if strings.HasPrefix(methodLower, "get") || 
	   strings.HasPrefix(methodLower, "list") || 
	   strings.HasPrefix(methodLower, "find") ||
	   strings.HasPrefix(methodLower, "search") ||
	   strings.HasPrefix(methodLower, "fetch") {
		return "SELECT"
	}
	if strings.HasPrefix(methodLower, "create") || 
	   strings.HasPrefix(methodLower, "add") ||
	   strings.HasPrefix(methodLower, "insert") {
		return "INSERT"
	}
	if strings.HasPrefix(methodLower, "update") || 
	   strings.HasPrefix(methodLower, "modify") ||
	   strings.HasPrefix(methodLower, "change") ||
	   strings.HasPrefix(methodLower, "set") {
		return "UPDATE"
	}
	if strings.HasPrefix(methodLower, "delete") || 
	   strings.HasPrefix(methodLower, "remove") {
		return "DELETE"
	}
	return ""
}

func unique(strs []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, s := range strs {
		if !seen[s] && s != "" {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

// ============================================================================
// Strategy 3: Parameter Signature Matching
// ============================================================================

type ParameterSignatureStrategy struct{}

func (s *ParameterSignatureStrategy) Name() string { return "params" }

func (s *ParameterSignatureStrategy) Match(method *ProtoMethodInfo, proc *Procedure, ctx *MatchContext) *StrategyResult {
	reqMsg := ctx.AllMessages[method.RequestType]
	if reqMsg == nil || len(reqMsg.Fields) == 0 {
		return nil
	}

	if len(proc.Parameters) == 0 {
		return nil
	}

	// Build normalized field names from proto
	protoFields := make(map[string]bool)
	for _, field := range reqMsg.Fields {
		normalized := strings.ToLower(strings.ReplaceAll(field.Name, "_", ""))
		protoFields[normalized] = true
	}

	// Count matching parameters
	requiredParams := 0
	matchedParams := 0

	for _, param := range proc.Parameters {
		if param.IsOutput {
			continue
		}
		if !param.HasDefault {
			requiredParams++
		}

		paramNorm := strings.ToLower(param.Name)
		
		// Direct match
		if protoFields[paramNorm] {
			matchedParams++
			continue
		}

		// Try without common suffixes (UserId -> user)
		for _, suffix := range []string{"id", "code", "name", "key"} {
			if strings.HasSuffix(paramNorm, suffix) {
				trimmed := strings.TrimSuffix(paramNorm, suffix)
				if protoFields[trimmed] || protoFields[trimmed+suffix] {
					matchedParams++
					break
				}
			}
		}
	}

	if matchedParams == 0 {
		return nil
	}

	// Score based on match ratio
	var score float64
	var totalParams int
	if requiredParams > 0 {
		totalParams = requiredParams
	} else {
		totalParams = len(proc.Parameters)
	}
	
	score = float64(matchedParams) / float64(totalParams)

	// Penalty for procedures with very few params - too easy to get 100% match
	// This prevents usp_ClearCart(@UserId) from beating usp_ChangePassword(@UserId, @Hash, ...)
	if totalParams < 2 && matchedParams == totalParams {
		score *= 0.6 // Reduce score for single-param 100% matches
	} else if totalParams < 3 && matchedParams == totalParams {
		score *= 0.8 // Reduce slightly for 2-param matches
	}

	// Bonus for high match count with many params (shows strong correlation)
	if matchedParams >= 3 && totalParams >= 3 {
		score += 0.1
	}
	if score > 1.0 {
		score = 1.0
	}

	return &StrategyResult{
		Matched:    true,
		Score:      score,
		Reason:     "params matched: " + string(rune('0'+matchedParams)) + "/" + string(rune('0'+totalParams)),
		Confidence: 0.75,
	}
}

// ============================================================================
// Strategy 4: Verb + Entity Semantic Analysis
// ============================================================================

type VerbEntityStrategy struct{}

func (s *VerbEntityStrategy) Name() string { return "verb_entity" }

func (s *VerbEntityStrategy) Match(method *ProtoMethodInfo, proc *Procedure, ctx *MatchContext) *StrategyResult {
	// Parse method into verb + entity
	methodVerb, methodEntity := parseVerbEntity(method.Name)
	if methodVerb == "" || methodEntity == "" {
		return nil
	}

	// Parse procedure into verb + entity
	procName := proc.Name
	for _, prefix := range []string{"usp_", "sp_", "proc_", "p_"} {
		procName = strings.TrimPrefix(procName, prefix)
	}
	procVerb, procEntity := parseVerbEntity(procName)
	if procVerb == "" {
		return nil
	}

	// Check verb compatibility
	verbScore := scoreVerbMatch(methodVerb, procVerb)
	if verbScore == 0 {
		return nil
	}

	// Check entity match
	entityScore := scoreEntityMatch(methodEntity, procEntity)

	// Combined score
	totalScore := (verbScore*0.4 + entityScore*0.6)
	if totalScore < 0.3 {
		return nil
	}

	return &StrategyResult{
		Matched:    true,
		Score:      totalScore,
		Reason:     methodVerb + "+" + methodEntity + " ~ " + procVerb + "+" + procEntity,
		Confidence: 0.7,
	}
}

func parseVerbEntity(name string) (verb, entity string) {
	// Known verb patterns (order matters - longer first)
	verbs := []string{
		"Authenticate", "Authorize", "IsValid", "Validate", "Calculate", "Generate",
		"GetActive", "GetValid", "GetAvailable",
		"CheckDatabase", "Check", "Confirm", "Convert", "Transform",
		"CreateTransfer", "Create", "Insert", "Add", "New",
		"UpdateOrder", "Update", "Modify", "Edit", "Change", "Set",
		"Delete", "Remove", "Drop",
		"List", "GetAll", "FindAll", "Search", "Query", "Lookup",
		"Get", "Fetch", "Find", "Load", "Retrieve",
		"Process", "Execute", "Run", "Handle",
		"Send", "Transmit", "Dispatch",
		"Receive", "Accept",
		"Reserve", "Hold", "Lock", "Allocate",
		"Release", "Free", "Unlock",
		"Cancel", "Abort", "Revoke", "Void",
		"Approve", "Accept",
		"Reject", "Deny", "Decline",
		"Refresh", "Renew",
		"Login", "Logout",
	}

	nameLower := strings.ToLower(name)
	for _, v := range verbs {
		vLower := strings.ToLower(v)
		if strings.HasPrefix(nameLower, vLower) {
			verb = v
			entity = name[len(v):]
			break
		}
	}

	// Clean up entity
	if entity != "" {
		// Remove ById, ByEmail, etc suffixes
		re := regexp.MustCompile(`(?i)(By\w+)$`)
		entity = re.ReplaceAllString(entity, "")
	}

	return verb, entity
}

// Verb compatibility groups
var verbGroups = map[string][]string{
	"read":   {"get", "fetch", "find", "load", "retrieve", "list", "search", "query", "lookup", "getactive", "getvalid", "getavailable", "getall", "findall"},
	"create": {"create", "insert", "add", "new", "createtransfer"},
	"update": {"update", "modify", "edit", "change", "set", "updateorder"},
	"delete": {"delete", "remove", "drop"},
	"validate": {"validate", "check", "verify", "isvalid", "checkdatabase", "confirm"},
	"convert": {"convert", "transform", "translate"},
	"auth":   {"authenticate", "authorize", "login"},
	"process": {"process", "execute", "run", "handle"},
	"reserve": {"reserve", "hold", "lock", "allocate"},
	"release": {"release", "free", "unlock", "cancel", "abort", "revoke", "void"},
	"approve": {"approve", "accept"},
	"reject": {"reject", "deny", "decline"},
	"send":   {"send", "transmit", "dispatch"},
	"receive": {"receive", "accept", "get"},
}

func scoreVerbMatch(v1, v2 string) float64 {
	v1Lower := strings.ToLower(v1)
	v2Lower := strings.ToLower(v2)

	if v1Lower == v2Lower {
		return 1.0
	}

	// Check if in same group
	for _, group := range verbGroups {
		hasV1, hasV2 := false, false
		for _, v := range group {
			if v == v1Lower {
				hasV1 = true
			}
			if v == v2Lower {
				hasV2 = true
			}
		}
		if hasV1 && hasV2 {
			return 0.8
		}
	}

	return 0
}

func scoreEntityMatch(e1, e2 string) float64 {
	e1Lower := strings.ToLower(e1)
	e2Lower := strings.ToLower(e2)

	if e1Lower == "" || e2Lower == "" {
		return 0.3 // Partial credit for no entity
	}

	if e1Lower == e2Lower {
		return 1.0
	}

	// Singular/plural
	if e1Lower+"s" == e2Lower || e1Lower == e2Lower+"s" {
		return 0.95
	}

	// Substring
	if strings.Contains(e1Lower, e2Lower) || strings.Contains(e2Lower, e1Lower) {
		return 0.7
	}

	return 0
}

// Helper functions for parameter and result mapping (shared with existing code)
func mapParametersFromContext(method *ProtoMethodInfo, proc *Procedure, ctx *MatchContext) []ParamMapping {
	var mappings []ParamMapping

	reqMsg := ctx.AllMessages[method.RequestType]
	if reqMsg == nil {
		return mappings
	}

	protoFields := make(map[string]*ProtoFieldInfo)
	for i := range reqMsg.Fields {
		field := &reqMsg.Fields[i]
		protoFields[strings.ToLower(field.Name)] = field
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

		if field, ok := protoFields[paramLower]; ok {
			pm.ProtoField = field.Name
			pm.ProtoType = field.ProtoType
			pm.IsOptional = field.IsOptional
		} else if field, ok := protoFields[strings.ReplaceAll(paramLower, "_", "")]; ok {
			pm.ProtoField = field.Name
			pm.ProtoType = field.ProtoType
			pm.IsOptional = field.IsOptional
		} else if strings.HasSuffix(paramLower, "id") {
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

func mapResultsFromContext(method *ProtoMethodInfo, proc *Procedure, ctx *MatchContext) *ResultMapping {
	if len(proc.ResultSets) == 0 {
		return nil
	}

	respMsg := ctx.AllMessages[method.ResponseType]
	if respMsg == nil {
		return nil
	}

	rm := &ResultMapping{
		ResponseType:   method.ResponseType,
		ResultSetIndex: 0,
	}

	resultSet := proc.ResultSets[0]

	protoFields := make(map[string]*ProtoFieldInfo)
	for i := range respMsg.Fields {
		field := &respMsg.Fields[i]
		protoFields[strings.ToLower(field.Name)] = field
		protoFields[strings.ReplaceAll(strings.ToLower(field.Name), "_", "")] = field
	}

	for i, col := range resultSet.Columns {
		if col.Name == "*" {
			continue
		}

		colLower := strings.ToLower(col.Name)
		fm := FieldMapping{
			ColumnName:  col.Name,
			ColumnIndex: i,
		}

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
