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
		agreement     int  // How many strategies agree
		hasExactName  bool // Has exact naming match
		tieBreakScore float64 // Secondary score for tie-breaking
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
			// Compute tie-breaker score using composite method
			ps.tieBreakScore = m.computeTieBreakScore(method, proc, ctx)
			scores[proc.Name] = ps
		}
	}

	// Find best match considering agreement/disagreement
	var bestProc *procScore
	var bestFinalScore float64

	// Total number of strategies available
	numStrategies := len(m.strategies)

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

		// Coverage penalty: penalize when few strategies matched
		// If only 1 of 4 strategies matched, that's 25% coverage = 15% penalty
		// If 2 of 4 matched, that's 50% coverage = 10% penalty
		// If 3 of 4 matched, that's 75% coverage = 5% penalty
		// If all 4 matched, no penalty
		coverage := float64(ps.agreement) / float64(numStrategies)
		coveragePenalty := (1.0 - coverage) * 0.20 // Up to 20% penalty for low coverage

		// Single-strategy penalty: if only 1 strategy matched, apply extra penalty
		// This prevents a single perfect score from dominating multiple partial matches
		if ps.agreement == 1 {
			coveragePenalty += 0.10
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
			
			// If another procedure has MORE strategies matching, penalize this one
			if otherPS.agreement > ps.agreement {
				disagreementPenalty += 0.05 * float64(otherPS.agreement - ps.agreement)
			}
			
			// If same strategy voted for multiple procedures, small penalty
			for stratName := range ps.strategyVotes {
				if _, hasVote := otherPS.strategyVotes[stratName]; hasVote {
					disagreementPenalty += 0.01
				}
			}
		}
		if disagreementPenalty > 0.25 {
			disagreementPenalty = 0.25 // Cap penalty
		}

		finalScore := avgScore + agreementBonus - coveragePenalty - disagreementPenalty
		if finalScore > 1.0 {
			finalScore = 1.0
		}
		if finalScore < 0 {
			finalScore = 0
		}

		// Use tie-breaker when scores are very close (within 5%)
		if bestProc == nil {
			bestFinalScore = finalScore
			bestProc = ps
		} else if finalScore > bestFinalScore+0.05 {
			// Clear winner - more than 5% better
			bestFinalScore = finalScore
			bestProc = ps
		} else if finalScore > bestFinalScore-0.05 {
			// Scores are within 5% - use tie-breaker
			if ps.tieBreakScore > bestProc.tieBreakScore {
				bestFinalScore = finalScore
				bestProc = ps
			} else if ps.tieBreakScore == bestProc.tieBreakScore {
				// Final fallback: lexicographic by name for determinism
				if ps.proc.Name < bestProc.proc.Name {
					bestFinalScore = finalScore
					bestProc = ps
				}
			}
		}
		// else: bestProc remains (it's more than 5% better)
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

// computeTieBreakScore calculates a secondary score for tie-breaking.
// Uses a composite of: parameter proximity, type compatibility, name specificity,
// entity-table alignment, and result cardinality.
func (m *EnsembleMapper) computeTieBreakScore(method *ProtoMethodInfo, proc *Procedure, ctx *MatchContext) float64 {
	var score float64
	
	// 1. Parameter count proximity (weight: 0.25)
	reqMsg := ctx.AllMessages[method.RequestType]
	protoFieldCount := 0
	if reqMsg != nil {
		protoFieldCount = len(reqMsg.Fields)
	}
	procParamCount := len(proc.Parameters)
	diff := protoFieldCount - procParamCount
	if diff < 0 {
		diff = -diff
	}
	paramProximity := 1.0 / (1.0 + float64(diff)*0.25)
	score += paramProximity * 0.25

	// 2. Parameter type compatibility (weight: 0.25)
	if reqMsg != nil && len(reqMsg.Fields) > 0 {
		matches := 0
		for _, field := range reqMsg.Fields {
			fieldLower := strings.ToLower(field.Name)
			fieldLower = strings.ReplaceAll(fieldLower, "_", "")
			for _, param := range proc.Parameters {
				paramLower := strings.ToLower(param.Name)
				if fieldLower == paramLower || 
				   strings.Contains(paramLower, fieldLower) || 
				   strings.Contains(fieldLower, paramLower) {
					if isTieBreakTypeCompatible(field.ProtoType, param.GoType) {
						matches++
						break
					}
				}
			}
		}
		typeCompat := float64(matches) / float64(len(reqMsg.Fields))
		score += typeCompat * 0.25
	} else {
		score += 0.5 * 0.25 // Neutral when no fields
	}

	// 3. Name specificity (weight: 0.20)
	// Prefer names that closely match the method name length
	methodLen := len(method.Name)
	procNorm := strings.ToLower(proc.Name)
	for _, prefix := range []string{"usp_", "sp_", "proc_", "p_"} {
		procNorm = strings.TrimPrefix(procNorm, prefix)
	}
	lenDiff := len(procNorm) - methodLen
	if lenDiff < 0 {
		lenDiff = -lenDiff
	}
	nameSpec := 1.0 / (1.0 + float64(lenDiff)*0.1)
	score += nameSpec * 0.20

	// 4. Entity-table alignment (weight: 0.20)
	_, methodEntity := parseVerbEntity(method.Name)
	if methodEntity != "" {
		methodEntityLower := strings.ToLower(methodEntity)
		entityScore := 0.3 // Default if no match
		for _, op := range proc.Operations {
			tableLower := strings.ToLower(op.Table)
			// Exact or singular/plural match
			if tableLower == methodEntityLower ||
			   tableLower == methodEntityLower+"s" ||
			   tableLower+"s" == methodEntityLower {
				entityScore = 1.0
				break
			}
			if strings.Contains(tableLower, methodEntityLower) || 
			   strings.Contains(methodEntityLower, tableLower) {
				if entityScore < 0.8 {
					entityScore = 0.8
				}
			}
		}
		score += entityScore * 0.20
	} else {
		score += 0.5 * 0.20 // Neutral when no entity
	}

	// 5. Suffix match bonus (weight: 0.10)
	// If method has ById/ByEmail etc and proc matches, boost
	if reqMsg != nil && len(reqMsg.Fields) > 0 {
		suffixScore := 0.5
		procLower := strings.ToLower(proc.Name)
		for _, field := range reqMsg.Fields {
			fieldLower := strings.ToLower(field.Name)
			fieldLower = strings.ReplaceAll(fieldLower, "_", "")
			// Check if proc name has "by" + field name
			byField := "by" + fieldLower
			if strings.Contains(procLower, byField) {
				suffixScore = 1.0
				break
			}
		}
		score += suffixScore * 0.10
	} else {
		score += 0.5 * 0.10
	}

	return score
}

// isTieBreakTypeCompatible checks if proto and Go types are compatible
func isTieBreakTypeCompatible(protoType, goType string) bool {
	compatMap := map[string][]string{
		"int32":  {"int", "int32", "int64"},
		"int64":  {"int", "int32", "int64"},
		"string": {"string"},
		"bool":   {"bool"},
		"double": {"float64", "float32"},
		"float":  {"float64", "float32"},
		"bytes":  {"[]byte", "string"},
	}
	if goTypes, ok := compatMap[protoType]; ok {
		for _, gt := range goTypes {
			if strings.Contains(goType, gt) {
				return true
			}
		}
	}
	return false
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
		{"create", []string{"create", "insert", "add", "new", "register", "enroll"}, ""},
		{"update", []string{"update", "modify", "edit", "change", "set", "patch", "amend"}, ""},
		{"delete", []string{"delete", "remove", "drop", "purge"}, ""},
		{"change", []string{"change", "update", "modify", "set"}, ""},

		// Validation & verification
		{"validate", []string{"validate", "check", "verify"}, ""},
		{"isvalid", []string{"isvalid", "validate", "check"}, ""},
		{"check", []string{"check", "validate", "verify", "test"}, ""},
		{"confirm", []string{"confirm", "verify", "validate"}, ""},
		{"verify", []string{"verify", "validate", "check", "confirm"}, ""},

		// Transformation
		{"convert", []string{"convert", "transform", "translate"}, ""},
		{"transform", []string{"transform", "convert", "translate", "normalize"}, ""},
		{"calculate", []string{"calculate", "calc", "compute", "estimate"}, ""},
		{"compute", []string{"compute", "calculate", "calc"}, ""},
		{"generate", []string{"generate", "create", "produce", "build"}, ""},
		{"aggregate", []string{"aggregate", "summarize", "consolidate"}, ""},

		// Authentication
		{"authenticate", []string{"authenticate", "auth", "login", "verify"}, ""},
		{"authorize", []string{"authorize", "auth", "checkpermission", "grant"}, ""},
		{"login", []string{"login", "signin", "authenticate"}, ""},
		{"logout", []string{"logout", "signout"}, ""},

		// Search & query
		{"search", []string{"search", "find", "query", "lookup"}, ""},
		{"query", []string{"query", "search", "find", "lookup"}, ""},

		// Process & execution
		{"process", []string{"process", "execute", "run", "handle", "perform"}, ""},
		{"execute", []string{"execute", "process", "run", "perform"}, ""},

		// Communication
		{"send", []string{"send", "transmit", "dispatch", "deliver", "forward"}, ""},
		{"receive", []string{"receive", "get", "fetch", "accept", "collect"}, ""},
		{"notify", []string{"notify", "alert", "inform", "remind", "broadcast"}, ""},
		{"publish", []string{"publish", "broadcast", "send", "emit"}, ""},

		// Resource management
		{"reserve", []string{"reserve", "hold", "lock", "allocate", "claim"}, ""},
		{"release", []string{"release", "free", "unlock", "deallocate"}, ""},
		{"acquire", []string{"acquire", "get", "obtain", "claim"}, ""},

		// Lifecycle & state transitions
		{"cancel", []string{"cancel", "abort", "revoke", "void", "annul"}, ""},
		{"suspend", []string{"suspend", "pause", "freeze", "hold", "deactivate"}, ""},
		{"resume", []string{"resume", "reactivate", "unfreeze", "unpause"}, ""},
		{"activate", []string{"activate", "enable", "start"}, ""},
		{"deactivate", []string{"deactivate", "disable", "suspend"}, ""},
		{"complete", []string{"complete", "finish", "finalize", "close"}, ""},
		{"finalize", []string{"finalize", "complete", "close", "conclude"}, ""},
		{"initiate", []string{"initiate", "start", "begin", "open", "launch"}, ""},
		{"start", []string{"start", "begin", "initiate", "launch"}, ""},
		{"stop", []string{"stop", "end", "terminate", "halt"}, ""},
		{"terminate", []string{"terminate", "end", "stop", "cancel"}, ""},

		// Approval workflow
		{"approve", []string{"approve", "accept", "authorize", "grant", "permit"}, ""},
		{"reject", []string{"reject", "deny", "decline", "refuse"}, ""},
		{"certify", []string{"certify", "attest", "endorse", "validate", "accredit"}, ""},
		{"recertify", []string{"recertify", "certify", "renew"}, ""},
		{"review", []string{"review", "assess", "evaluate", "inspect", "examine"}, ""},
		{"assess", []string{"assess", "evaluate", "review", "appraise"}, ""},
		{"audit", []string{"audit", "review", "inspect", "examine"}, ""},
		{"escalate", []string{"escalate", "elevate", "refer", "delegate"}, ""},
		{"delegate", []string{"delegate", "assign", "transfer", "refer"}, ""},
		{"reassign", []string{"reassign", "transfer", "delegate", "assign"}, ""},

		// Acknowledgment & signing
		{"acknowledge", []string{"acknowledge", "confirm", "accept", "receipt"}, ""},
		{"sign", []string{"sign", "execute", "approve"}, ""},
		{"countersign", []string{"countersign", "sign", "approve"}, ""},

		// Synchronization
		{"sync", []string{"sync", "synchronize", "replicate", "refresh"}, ""},
		{"refresh", []string{"refresh", "renew", "update", "sync"}, ""},
		{"archive", []string{"archive", "backup", "store", "preserve"}, ""},

		// Registration
		{"register", []string{"register", "enroll", "signup", "create"}, ""},
		{"unregister", []string{"unregister", "deregister", "remove"}, ""},
		{"subscribe", []string{"subscribe", "register", "enroll", "add"}, ""},
		{"unsubscribe", []string{"unsubscribe", "remove", "deregister"}, ""},
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
	// Known verb patterns (order matters - longer first for proper matching)
	verbs := []string{
		// Multi-word patterns first (longer matches before shorter)
		"Authenticate", "Authorize", "IsValid", "Validate", "Calculate", "Generate",
		"GetActive", "GetValid", "GetAvailable",
		"CheckDatabase", "Healthcheck",
		"CreateTransfer", "UpdateOrder",
		"Regenerate", "Revalidate", "Reinitialize", "Reconfigure",

		// CRUD operations
		"Create", "Insert", "Add", "New", "Register", "Enroll", "Subscribe",
		"Update", "Modify", "Edit", "Change", "Set", "Patch", "Amend",
		"Delete", "Remove", "Drop", "Unsubscribe", "Deregister", "Purge",
		"List", "GetAll", "FindAll", "Search", "Query", "Lookup",
		"Get", "Fetch", "Find", "Load", "Retrieve", "Read", "Select",

		// Validation & verification
		"Check", "Confirm", "Verify", "Test", "Validate",

		// Transformation
		"Convert", "Transform", "Translate", "Parse", "Serialize", "Deserialize",
		"Encode", "Decode", "Encrypt", "Decrypt", "Compress", "Decompress",
		"Normalize", "Format", "Sanitize", "Cleanse",

		// Token/session operations
		"Issue", "Reissue", "Rotate", "Refresh", "Renew", "Extend", "Prolong",

		// Generation & calculation
		"Compute", "Estimate", "Forecast", "Project", "Calculate", "Calc",
		"Generate", "Produce", "Build", "Render", "Compile",
		"Aggregate", "Summarize", "Consolidate", "Merge", "Combine",
		"Count", "Sum", "Average",

		// Process & execution
		"Process", "Execute", "Run", "Handle", "Perform", "Invoke",
		"Batch", "Bulk", "Retry", "Rerun", "Reprocess", "Replay",

		// Communication
		"Send", "Transmit", "Dispatch", "Deliver", "Forward", "Route",
		"Receive", "Accept", "Collect",
		"Notify", "Alert", "Warn", "Inform", "Remind", "Broadcast", "Publish",
		"Email", "Print",

		// Resource management
		"Reserve", "Hold", "Lock", "Allocate", "Claim", "Acquire",
		"Release", "Free", "Unlock", "Deallocate", "Relinquish",
		"Unclaim", "Take", "Pickup",

		// Lifecycle & state transitions
		"Cancel", "Abort", "Revoke", "Void", "Annul", "Terminate",
		"Suspend", "Pause", "Freeze", "Deactivate", "Disable", "Ban",
		"Resume", "Reactivate", "Unfreeze", "Unpause", "Enable", "Activate", "Unban",
		"Complete", "Finish", "Finalize", "Close", "Conclude", "End",
		"Initiate", "Start", "Begin", "Open", "Launch", "Trigger",
		"Expire", "Invalidate",
		"Reset", "Initialize", "Configure", "Setup",

		// Approval workflow
		"Approve", "Grant", "Allow", "Permit", "Sanction",
		"Reject", "Deny", "Decline", "Refuse", "Disallow", "Veto",
		"Certify", "Recertify", "Decertify", "Attest", "Endorse", "Accredit", "License",
		"Review", "Assess", "Evaluate", "Inspect", "Audit", "Examine", "Appraise", "Moderate",
		"Escalate", "Elevate", "Refer", "Delegate", "Reassign", "Transfer",
		"Submit", "Propose", "Request",

		// Acknowledgment & signing
		"Acknowledge", "Receipt",
		"Sign", "Countersign", "Cosign", "Seal", "Notarize",

		// Synchronization & data movement
		"Sync", "Synchronize", "Replicate", "Mirror",
		"Archive", "Backup", "Snapshot", "Preserve", "Store",
		"Import", "Export", "Upload", "Download", "Copy", "Clone", "Duplicate",
		"Restore", "Recover", "Rollback",
		"Migrate", "Move",

		// Authentication
		"Login", "Logout", "Signin", "Signout",

		// Financial
		"Bill", "Invoice", "Charge", "Credit", "Debit", "Refund", "Reimburse",
		"Pay", "Settle", "Post", "Reconcile",
		"Capture", "Withdraw", "Payout", "Disburse", "Deposit",

		// E-commerce
		"Checkout", "Purchase", "Buy", "Order", "Return", "Exchange",
		"Ship", "Fulfill", "Pack",

		// Scheduling & assignment
		"Book", "Schedule", "Reschedule", "Assign", "Unassign",
		"Enqueue", "Dequeue", "Queue",

		// Linking & relationships
		"Attach", "Detach", "Link", "Unlink", "Associate", "Dissociate",
		"Tag", "Untag", "Label", "Categorize", "Classify",
		"Flag", "Unflag", "Mark", "Unmark", "Pin", "Unpin",

		// Social actions
		"Share", "Unshare", "Invite",
		"Follow", "Unfollow", "Block", "Unblock", "Mute", "Unmute",
		"Vote", "Rate", "Score", "Rank", "Like", "Upvote", "Downvote",

		// Content & publishing
		"Publish", "Unpublish", "Draft", "Compose", "Write",
		"Stage", "Deploy", "Undeploy",
		"Promote", "Demote", "Upgrade", "Downgrade",
		"Feature", "Unfeature", "Highlight", "Spotlight",

		// Monitoring
		"Monitor", "Track", "Observe", "Measure", "Log", "Ping", "Poll", "Probe",

		// Display
		"Preview", "Display", "Show", "Hide", "Mask", "Reveal",

		// Navigation
		"Navigate", "Browse", "Explore",
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

// Verb compatibility groups - verbs in the same group are considered synonymous
var verbGroups = map[string][]string{
	// CRUD operations
	"read":   {"get", "fetch", "find", "load", "retrieve", "list", "search", "query", "lookup", "read", "select", "getactive", "getvalid", "getavailable", "getall", "findall", "browse", "explore", "navigate"},
	"create": {"create", "insert", "add", "new", "register", "enroll", "subscribe", "generate", "produce", "build", "initialize", "setup", "createtransfer", "issue"},
	"update": {"update", "modify", "edit", "change", "set", "patch", "amend", "updateorder", "reconfigure", "configure"},
	"delete": {"delete", "remove", "drop", "unsubscribe", "deregister", "purge", "teardown"},

	// Validation & verification
	"validate": {"validate", "check", "verify", "isvalid", "checkdatabase", "confirm", "test", "healthcheck", "probe", "ping", "poll"},

	// Token/session operations
	"refresh": {"refresh", "renew", "extend", "prolong", "revalidate", "reissue", "regenerate", "rotate", "validate"},

	// Transformation & parsing
	"convert":   {"convert", "transform", "translate", "normalize", "format", "parse", "serialize", "deserialize", "encode", "decode"},
	"encrypt":   {"encrypt", "decrypt", "encode", "decode", "compress", "decompress"},
	"sanitize":  {"sanitize", "cleanse", "scrub", "normalize", "format"},

	// Generation & calculation
	"calculate": {"calculate", "calc", "compute", "estimate", "forecast", "project", "count", "sum", "average"},
	"generate":  {"generate", "produce", "build", "render", "compile", "create", "issue", "regenerate"},
	"aggregate": {"aggregate", "summarize", "consolidate", "merge", "combine", "collect"},

	// Authentication & authorization
	"auth": {"authenticate", "authorize", "login", "logout", "signin", "signout", "refresh", "renew", "rotate"},

	// Process & execution
	"process": {"process", "execute", "run", "handle", "perform", "invoke", "batch", "bulk"},
	"retry":   {"retry", "rerun", "reprocess", "replay", "repeat"},

	// Communication
	"send":    {"send", "transmit", "dispatch", "deliver", "forward", "route", "email", "print", "publish", "broadcast"},
	"receive": {"receive", "accept", "get", "collect", "retrieve"},
	"notify":  {"notify", "alert", "warn", "inform", "remind", "broadcast", "publish"},

	// Resource management
	"reserve": {"reserve", "hold", "lock", "allocate", "claim", "acquire", "book"},
	"release": {"release", "free", "unlock", "deallocate", "relinquish", "unbook"},

	// Lifecycle & state transitions
	"cancel":   {"cancel", "abort", "revoke", "void", "annul", "terminate", "stop"},
	"suspend":  {"suspend", "pause", "freeze", "hold", "deactivate", "disable", "mute", "block", "ban"},
	"resume":   {"resume", "reactivate", "unfreeze", "unpause", "enable", "activate", "unmute", "unblock", "unban"},
	"complete": {"complete", "finish", "finalize", "close", "conclude", "end", "fulfill"},
	"initiate": {"initiate", "start", "begin", "open", "launch", "trigger", "bootstrap"},
	"expire":   {"expire", "invalidate", "timeout"},
	"reset":    {"reset", "reinitialize", "clear", "wipe"},

	// Approval workflow
	"approve":  {"approve", "accept", "grant", "allow", "permit", "sanction", "authorize"},
	"reject":   {"reject", "deny", "decline", "refuse", "disallow", "veto"},
	"certify":  {"certify", "recertify", "decertify", "attest", "endorse", "accredit", "license"},
	"review":   {"review", "assess", "evaluate", "inspect", "audit", "examine", "appraise", "moderate"},
	"escalate": {"escalate", "elevate", "refer", "delegate", "reassign", "transfer"},
	"submit":   {"submit", "propose", "request"},
	"claim":    {"claim", "unclaim", "take", "pickup"},

	// Acknowledgment & signing
	"acknowledge": {"acknowledge", "confirm", "receipt", "accept"},
	"sign":        {"sign", "countersign", "cosign", "execute", "seal", "notarize"},

	// Synchronization & data movement
	"sync":    {"sync", "synchronize", "replicate", "mirror", "refresh"},
	"archive": {"archive", "backup", "snapshot", "preserve", "store"},
	"import":  {"import", "upload", "ingest", "load"},
	"export":  {"export", "download", "extract"},
	"copy":    {"copy", "clone", "duplicate", "replicate"},
	"restore": {"restore", "recover", "rollback", "undo"},
	"migrate": {"migrate", "transfer", "move"},

	// Financial operations
	"charge":   {"charge", "bill", "invoice", "debit"},
	"credit":   {"credit", "refund", "reimburse"},
	"pay":      {"pay", "settle", "clear", "post", "reconcile"},
	"capture":  {"capture", "collect", "receive"},
	"withdraw": {"withdraw", "payout", "disburse"},
	"deposit":  {"deposit", "receive", "credit"},

	// E-commerce
	"checkout": {"checkout", "purchase", "buy", "order"},
	"return":   {"return", "exchange", "rma"},
	"ship":     {"ship", "deliver", "dispatch", "send", "fulfill", "pack"},

	// Scheduling & assignment
	"schedule": {"schedule", "book", "reserve", "reschedule"},
	"assign":   {"assign", "allocate", "delegate", "reassign"},
	"unassign": {"unassign", "deallocate", "release"},
	"queue":    {"queue", "enqueue", "push"},
	"dequeue":  {"dequeue", "pop", "pull"},

	// Linking & relationships
	"attach":     {"attach", "link", "associate", "bind", "connect", "couple"},
	"detach":     {"detach", "unlink", "dissociate", "unbind", "disconnect", "decouple"},
	"tag":        {"tag", "label", "categorize", "classify", "mark", "flag", "pin"},
	"untag":      {"untag", "unlabel", "unmark", "unflag", "unpin"},

	// Social actions
	"share":    {"share", "publish", "distribute"},
	"follow":   {"follow", "subscribe", "watch"},
	"unfollow": {"unfollow", "unsubscribe", "unwatch"},
	"vote":     {"vote", "rate", "score", "rank", "like", "upvote", "downvote"},

	// Content & publishing
	"publish":   {"publish", "release", "deploy", "stage", "promote"},
	"unpublish": {"unpublish", "undeploy", "demote", "retract"},
	"draft":     {"draft", "compose", "write"},
	"feature":   {"feature", "highlight", "spotlight"},
	"unfeature": {"unfeature", "unhighlight"},

	// Version & deployment
	"upgrade":   {"upgrade", "promote", "update"},
	"downgrade": {"downgrade", "demote", "rollback"},

	// Fulfillment
	"fulfill": {"fulfill", "complete", "finish", "satisfy"},

	// Monitoring & logging
	"monitor": {"monitor", "track", "observe", "watch", "measure"},
	"log":     {"log", "record", "audit", "trace"},

	// Display & visibility
	"show": {"show", "display", "reveal", "preview", "render"},
	"hide": {"hide", "mask", "conceal", "obscure"},
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
