package storage

import (
	"fmt"
	"sort"
	"strings"
	"testing"
)

// TieBreakerMethod defines different tie-breaking strategies
type TieBreakerMethod int

const (
	TieBreakNone TieBreakerMethod = iota // Current behaviour: arbitrary (first encountered)
	TieBreakLexicographic                // Alphabetical by procedure name
	TieBreakParamCountProximity          // Prefer |proto_fields - proc_params| closest to 0
	TieBreakParamTypeCompatibility       // Score parameter type matches
	TieBreakNameSpecificity              // Prefer longer/more specific names
	TieBreakEntityTableAlignment         // Entity should match primary DML table
	TieBreakResultCardinality            // List methods prefer multi-row results
	TieBreakComposite                    // Combination of above
)

func (t TieBreakerMethod) String() string {
	names := []string{
		"None (arbitrary)",
		"Lexicographic",
		"ParamCountProximity",
		"ParamTypeCompatibility",
		"NameSpecificity",
		"EntityTableAlignment",
		"ResultCardinality",
		"Composite",
	}
	if int(t) < len(names) {
		return names[t]
	}
	return "Unknown"
}

// TieScenario defines a test case where ties are expected
type TieScenario struct {
	Name           string
	Method         ProtoMethodInfo
	Request        *ProtoMessageInfo
	Response       *ProtoMessageInfo
	Procedures     []*Procedure
	ExpectedWinner string // Which procedure SHOULD win with good tie-breaking
	Reason         string // Why that one should win
}

func createTieScenarios() []TieScenario {
	return []TieScenario{
		{
			Name: "GetUser with ById vs ByEmail",
			Method: ProtoMethodInfo{
				Name:         "GetUser",
				RequestType:  "GetUserRequest",
				ResponseType: "GetUserResponse",
			},
			Request: &ProtoMessageInfo{
				Name: "GetUserRequest",
				Fields: []ProtoFieldInfo{
					{Name: "id", ProtoType: "int64", Number: 1},
				},
			},
			Response: &ProtoMessageInfo{
				Name:   "GetUserResponse",
				Fields: []ProtoFieldInfo{{Name: "user", ProtoType: "User", Number: 1}},
			},
			Procedures: []*Procedure{
				{
					Name:       "usp_GetUserById",
					Parameters: []ProcParameter{{Name: "UserId", SQLType: "BIGINT", GoType: "int64"}},
					Operations: []Operation{{Type: OpSelect, Table: "Users"}},
					ResultSets: []ResultSet{{FromTable: "Users"}},
				},
				{
					Name:       "usp_GetUserByEmail",
					Parameters: []ProcParameter{{Name: "Email", SQLType: "NVARCHAR(255)", GoType: "string"}},
					Operations: []Operation{{Type: OpSelect, Table: "Users"}},
					ResultSets: []ResultSet{{FromTable: "Users"}},
				},
			},
			ExpectedWinner: "usp_GetUserById",
			Reason:         "Request has 'id' field which matches 'ById' suffix and int64 type",
		},
		{
			Name: "ListProducts with different orderings",
			Method: ProtoMethodInfo{
				Name:         "ListProducts",
				RequestType:  "ListProductsRequest",
				ResponseType: "ListProductsResponse",
			},
			Request: &ProtoMessageInfo{
				Name: "ListProductsRequest",
				Fields: []ProtoFieldInfo{
					{Name: "category_id", ProtoType: "int64", Number: 1},
					{Name: "limit", ProtoType: "int32", Number: 2},
					{Name: "offset", ProtoType: "int32", Number: 3},
				},
			},
			Response: &ProtoMessageInfo{
				Name:   "ListProductsResponse",
				Fields: []ProtoFieldInfo{{Name: "products", ProtoType: "Product", Number: 1, IsRepeated: true}},
			},
			Procedures: []*Procedure{
				{
					Name: "usp_ListProducts",
					Parameters: []ProcParameter{
						{Name: "CategoryId", SQLType: "BIGINT", GoType: "int64"},
						{Name: "PageSize", SQLType: "INT", GoType: "int32"},
						{Name: "PageNum", SQLType: "INT", GoType: "int32"},
					},
					Operations: []Operation{{Type: OpSelect, Table: "Products"}},
					ResultSets: []ResultSet{{FromTable: "Products"}},
				},
				{
					Name: "usp_ListProductsByCategory",
					Parameters: []ProcParameter{
						{Name: "CategoryId", SQLType: "BIGINT", GoType: "int64"},
					},
					Operations: []Operation{{Type: OpSelect, Table: "Products"}},
					ResultSets: []ResultSet{{FromTable: "Products"}},
				},
			},
			ExpectedWinner: "usp_ListProducts",
			Reason:         "Exact name match + parameter count matches (3 vs 3)",
		},
		{
			Name: "CreateOrder with different param counts",
			Method: ProtoMethodInfo{
				Name:         "CreateOrder",
				RequestType:  "CreateOrderRequest",
				ResponseType: "CreateOrderResponse",
			},
			Request: &ProtoMessageInfo{
				Name: "CreateOrderRequest",
				Fields: []ProtoFieldInfo{
					{Name: "user_id", ProtoType: "int64", Number: 1},
					{Name: "items", ProtoType: "OrderItem", Number: 2, IsRepeated: true},
					{Name: "shipping_address", ProtoType: "Address", Number: 3},
				},
			},
			Response: &ProtoMessageInfo{
				Name:   "CreateOrderResponse",
				Fields: []ProtoFieldInfo{{Name: "order_id", ProtoType: "int64", Number: 1}},
			},
			Procedures: []*Procedure{
				{
					Name: "usp_CreateOrder",
					Parameters: []ProcParameter{
						{Name: "UserId", SQLType: "BIGINT", GoType: "int64"},
						{Name: "ItemsJson", SQLType: "NVARCHAR(MAX)", GoType: "string"},
						{Name: "ShippingAddressId", SQLType: "BIGINT", GoType: "int64"},
					},
					Operations: []Operation{{Type: OpInsert, Table: "Orders"}},
				},
				{
					Name: "usp_CreateOrderFromCart",
					Parameters: []ProcParameter{
						{Name: "UserId", SQLType: "BIGINT", GoType: "int64"},
						{Name: "CartId", SQLType: "BIGINT", GoType: "int64"},
					},
					Operations: []Operation{{Type: OpInsert, Table: "Orders"}},
				},
			},
			ExpectedWinner: "usp_CreateOrder",
			Reason:         "Exact name match + parameter count closer (3 vs 3, not 3 vs 2)",
		},
		{
			Name: "UpdateInventory with table mismatch",
			Method: ProtoMethodInfo{
				Name:         "UpdateInventory",
				RequestType:  "UpdateInventoryRequest",
				ResponseType: "UpdateInventoryResponse",
			},
			Request: &ProtoMessageInfo{
				Name: "UpdateInventoryRequest",
				Fields: []ProtoFieldInfo{
					{Name: "product_id", ProtoType: "int64", Number: 1},
					{Name: "quantity", ProtoType: "int32", Number: 2},
				},
			},
			Response: &ProtoMessageInfo{Name: "UpdateInventoryResponse"},
			Procedures: []*Procedure{
				{
					Name: "usp_UpdateInventory",
					Parameters: []ProcParameter{
						{Name: "ProductId", SQLType: "BIGINT", GoType: "int64"},
						{Name: "Quantity", SQLType: "INT", GoType: "int32"},
					},
					Operations: []Operation{{Type: OpUpdate, Table: "Inventory"}},
				},
				{
					Name: "usp_UpdateProductInventory",
					Parameters: []ProcParameter{
						{Name: "ProductId", SQLType: "BIGINT", GoType: "int64"},
						{Name: "Quantity", SQLType: "INT", GoType: "int32"},
					},
					Operations: []Operation{{Type: OpUpdate, Table: "Inventory"}},
				},
			},
			ExpectedWinner: "usp_UpdateInventory",
			Reason:         "Exact name match should win over more specific but longer name",
		},
		{
			Name: "ValidatePayment with verb synonyms",
			Method: ProtoMethodInfo{
				Name:         "ValidatePayment",
				RequestType:  "ValidatePaymentRequest",
				ResponseType: "ValidatePaymentResponse",
			},
			Request: &ProtoMessageInfo{
				Name: "ValidatePaymentRequest",
				Fields: []ProtoFieldInfo{
					{Name: "payment_id", ProtoType: "int64", Number: 1},
					{Name: "amount", ProtoType: "double", Number: 2},
				},
			},
			Response: &ProtoMessageInfo{Name: "ValidatePaymentResponse"},
			Procedures: []*Procedure{
				{
					Name: "usp_ValidatePayment",
					Parameters: []ProcParameter{
						{Name: "PaymentId", SQLType: "BIGINT", GoType: "int64"},
						{Name: "Amount", SQLType: "DECIMAL(18,2)", GoType: "float64"},
					},
					Operations: []Operation{{Type: OpSelect, Table: "Payments"}},
				},
				{
					Name: "usp_CheckPayment",
					Parameters: []ProcParameter{
						{Name: "PaymentId", SQLType: "BIGINT", GoType: "int64"},
						{Name: "Amount", SQLType: "DECIMAL(18,2)", GoType: "float64"},
					},
					Operations: []Operation{{Type: OpSelect, Table: "Payments"}},
				},
			},
			ExpectedWinner: "usp_ValidatePayment",
			Reason:         "Exact verb match (Validate) beats synonym (Check)",
		},
		{
			Name: "GetCustomer with entity mismatch",
			Method: ProtoMethodInfo{
				Name:         "GetCustomer",
				RequestType:  "GetCustomerRequest",
				ResponseType: "GetCustomerResponse",
			},
			Request: &ProtoMessageInfo{
				Name: "GetCustomerRequest",
				Fields: []ProtoFieldInfo{
					{Name: "customer_id", ProtoType: "int64", Number: 1},
				},
			},
			Response: &ProtoMessageInfo{Name: "GetCustomerResponse"},
			Procedures: []*Procedure{
				{
					Name: "usp_GetCustomer",
					Parameters: []ProcParameter{
						{Name: "CustomerId", SQLType: "BIGINT", GoType: "int64"},
					},
					Operations: []Operation{{Type: OpSelect, Table: "Customers"}},
				},
				{
					Name: "usp_GetUser",
					Parameters: []ProcParameter{
						{Name: "UserId", SQLType: "BIGINT", GoType: "int64"},
					},
					Operations: []Operation{{Type: OpSelect, Table: "Users"}},
				},
			},
			ExpectedWinner: "usp_GetCustomer",
			Reason:         "Entity match (Customer) + table match (Customers)",
		},
	}
}

// scoreProcForTieBreaker scores a procedure using a specific tie-breaker method
func scoreProcForTieBreaker(
	method TieBreakerMethod,
	proc *Procedure,
	protoMethod *ProtoMethodInfo,
	request *ProtoMessageInfo,
	response *ProtoMessageInfo,
) float64 {
	switch method {
	case TieBreakNone:
		return 0 // No additional scoring

	case TieBreakLexicographic:
		// Lower lexicographic = higher score (inverted for consistency)
		// Normalize to 0-1 range
		return 1.0 - float64(len(proc.Name))/100.0

	case TieBreakParamCountProximity:
		protoFieldCount := 0
		if request != nil {
			protoFieldCount = len(request.Fields)
		}
		procParamCount := len(proc.Parameters)
		diff := protoFieldCount - procParamCount
		if diff < 0 {
			diff = -diff
		}
		// Score: 1.0 for exact match, decreases with difference
		return 1.0 / (1.0 + float64(diff)*0.25)

	case TieBreakParamTypeCompatibility:
		if request == nil {
			return 0.5
		}
		matches := 0
		total := len(request.Fields)
		if total == 0 {
			return 0.5
		}
		for _, field := range request.Fields {
			fieldLower := strings.ToLower(field.Name)
			fieldLower = strings.ReplaceAll(fieldLower, "_", "")
			for _, param := range proc.Parameters {
				paramLower := strings.ToLower(param.Name)
				if fieldLower == paramLower || strings.Contains(paramLower, fieldLower) || strings.Contains(fieldLower, paramLower) {
					// Check type compatibility
					if isTypeCompatible(field.ProtoType, param.GoType) {
						matches++
						break
					}
				}
			}
		}
		return float64(matches) / float64(total)

	case TieBreakNameSpecificity:
		// Prefer names that are close to the method name length
		methodLen := len(protoMethod.Name)
		// Remove common prefixes for comparison
		procNorm := strings.ToLower(proc.Name)
		for _, prefix := range []string{"usp_", "sp_", "proc_"} {
			procNorm = strings.TrimPrefix(procNorm, prefix)
		}
		lenDiff := len(procNorm) - methodLen
		if lenDiff < 0 {
			lenDiff = -lenDiff
		}
		// Score decreases with length difference
		return 1.0 / (1.0 + float64(lenDiff)*0.1)

	case TieBreakEntityTableAlignment:
		_, methodEntity := parseVerbEntity(protoMethod.Name)
		if methodEntity == "" {
			return 0.5
		}
		methodEntityLower := strings.ToLower(methodEntity)
		// Check if any DML table matches the entity
		for _, op := range proc.Operations {
			tableLower := strings.ToLower(op.Table)
			// Singular/plural match
			if tableLower == methodEntityLower ||
				tableLower == methodEntityLower+"s" ||
				tableLower+"s" == methodEntityLower {
				return 1.0
			}
			if strings.Contains(tableLower, methodEntityLower) || strings.Contains(methodEntityLower, tableLower) {
				return 0.8
			}
		}
		return 0.3

	case TieBreakResultCardinality:
		// Check if method name suggests list/single
		methodLower := strings.ToLower(protoMethod.Name)
		expectsList := strings.HasPrefix(methodLower, "list") ||
			strings.HasPrefix(methodLower, "search") ||
			strings.HasPrefix(methodLower, "find") ||
			strings.HasPrefix(methodLower, "getall")

		// Without IsSingleRow field, we infer from result set presence
		// If the method expects a list and we have result sets, that's a match
		hasResults := len(proc.ResultSets) > 0
		if expectsList && hasResults {
			return 0.9
		}
		if !expectsList && hasResults {
			return 0.8
		}
		if len(proc.ResultSets) == 0 {
			return 0.5
		}
		return 0.5

	case TieBreakComposite:
		// Weighted combination of all methods
		scores := []float64{
			scoreProcForTieBreaker(TieBreakParamCountProximity, proc, protoMethod, request, response) * 0.25,
			scoreProcForTieBreaker(TieBreakParamTypeCompatibility, proc, protoMethod, request, response) * 0.25,
			scoreProcForTieBreaker(TieBreakNameSpecificity, proc, protoMethod, request, response) * 0.20,
			scoreProcForTieBreaker(TieBreakEntityTableAlignment, proc, protoMethod, request, response) * 0.20,
			scoreProcForTieBreaker(TieBreakResultCardinality, proc, protoMethod, request, response) * 0.10,
		}
		sum := 0.0
		for _, s := range scores {
			sum += s
		}
		return sum
	}
	return 0
}

func isTypeCompatible(protoType, goType string) bool {
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

func TestTieBreaker_DetectTies(t *testing.T) {
	scenarios := createTieScenarios()

	t.Log("=" + strings.Repeat("=", 70))
	t.Log("TIE-BREAKER ANALYSIS")
	t.Log("=" + strings.Repeat("=", 70))

	for _, scenario := range scenarios {
		t.Logf("\n### Scenario: %s", scenario.Name)
		t.Logf("    Method: %s", scenario.Method.Name)
		t.Logf("    Expected winner: %s", scenario.ExpectedWinner)
		t.Logf("    Reason: %s", scenario.Reason)

		// Build proto result for this scenario
		proto := &ProtoParseResult{
			AllServices: map[string]*ProtoServiceInfo{
				"TestService": {
					Name:    "TestService",
					Methods: []ProtoMethodInfo{scenario.Method},
				},
			},
			AllMessages: map[string]*ProtoMessageInfo{
				scenario.Request.Name:  scenario.Request,
				scenario.Response.Name: scenario.Response,
			},
			AllMethods: make(map[string]*ProtoMethodInfo),
		}

		// Run current mapper
		mapper := NewEnsembleMapper(proto, scenario.Procedures)
		mappings := mapper.MapAll()

		key := "TestService." + scenario.Method.Name
		mapping := mappings[key]

		if mapping == nil {
			t.Logf("    ✗ NO MAPPING FOUND")
			continue
		}

		t.Logf("    Current result: %s (%.0f%% confidence)", mapping.Procedure.Name, mapping.Confidence*100)

		if mapping.Procedure.Name == scenario.ExpectedWinner {
			t.Logf("    ✓ CORRECT")
		} else {
			t.Logf("    ✗ INCORRECT - expected %s", scenario.ExpectedWinner)
		}
	}
}

func TestTieBreaker_CompareMethods(t *testing.T) {
	scenarios := createTieScenarios()
	methods := []TieBreakerMethod{
		TieBreakLexicographic,
		TieBreakParamCountProximity,
		TieBreakParamTypeCompatibility,
		TieBreakNameSpecificity,
		TieBreakEntityTableAlignment,
		TieBreakResultCardinality,
		TieBreakComposite,
	}

	t.Log("=" + strings.Repeat("=", 80))
	t.Log("TIE-BREAKER METHOD COMPARISON")
	t.Log("=" + strings.Repeat("=", 80))

	// Track success rate per method
	successCount := make(map[TieBreakerMethod]int)

	for _, scenario := range scenarios {
		t.Logf("\n### %s", scenario.Name)
		t.Logf("    Expected: %s", scenario.ExpectedWinner)

		for _, method := range methods {
			// Score each procedure with this tie-breaker
			type procScore struct {
				proc  *Procedure
				score float64
			}
			var scores []procScore

			for _, proc := range scenario.Procedures {
				score := scoreProcForTieBreaker(method, proc, &scenario.Method, scenario.Request, scenario.Response)
				scores = append(scores, procScore{proc: proc, score: score})
			}

			// Sort by score descending
			sort.Slice(scores, func(i, j int) bool {
				if scores[i].score == scores[j].score {
					// Secondary sort by name for determinism
					return scores[i].proc.Name < scores[j].proc.Name
				}
				return scores[i].score > scores[j].score
			})

			winner := scores[0].proc.Name
			correct := winner == scenario.ExpectedWinner
			if correct {
				successCount[method]++
			}

			marker := "✗"
			if correct {
				marker = "✓"
			}
			t.Logf("    %s %-25s -> %s (score: %.3f)", marker, method, winner, scores[0].score)
		}
	}

	// Summary
	t.Log("\n" + strings.Repeat("=", 80))
	t.Log("SUMMARY")
	t.Log(strings.Repeat("=", 80))

	type methodResult struct {
		method  TieBreakerMethod
		correct int
	}
	var results []methodResult
	for method, count := range successCount {
		results = append(results, methodResult{method: method, correct: count})
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].correct > results[j].correct
	})

	for _, r := range results {
		pct := float64(r.correct) / float64(len(scenarios)) * 100
		t.Logf("    %s: %d/%d correct (%.0f%%)", r.method, r.correct, len(scenarios), pct)
	}

	// Recommend best method
	if len(results) > 0 {
		t.Logf("\n    RECOMMENDED: %s", results[0].method)
	}
}

func TestTieBreaker_EdgeCases(t *testing.T) {
	// Test edge cases where tie-breaking is critical

	edgeCases := []TieScenario{
		{
			Name: "Three-way tie with similar names",
			Method: ProtoMethodInfo{
				Name:         "GetProduct",
				RequestType:  "GetProductRequest",
				ResponseType: "GetProductResponse",
			},
			Request: &ProtoMessageInfo{
				Name: "GetProductRequest",
				Fields: []ProtoFieldInfo{
					{Name: "product_id", ProtoType: "int64", Number: 1},
				},
			},
			Response: &ProtoMessageInfo{Name: "GetProductResponse"},
			Procedures: []*Procedure{
				{
					Name: "usp_GetProduct",
					Parameters: []ProcParameter{
						{Name: "ProductId", SQLType: "BIGINT", GoType: "int64"},
					},
					Operations: []Operation{{Type: OpSelect, Table: "Products"}},
				},
				{
					Name: "usp_GetProductById",
					Parameters: []ProcParameter{
						{Name: "ProductId", SQLType: "BIGINT", GoType: "int64"},
					},
					Operations: []Operation{{Type: OpSelect, Table: "Products"}},
				},
				{
					Name: "usp_GetProductDetails",
					Parameters: []ProcParameter{
						{Name: "ProductId", SQLType: "BIGINT", GoType: "int64"},
					},
					Operations: []Operation{{Type: OpSelect, Table: "Products"}},
				},
			},
			ExpectedWinner: "usp_GetProduct",
			Reason:         "Exact name match should beat suffixed variants",
		},
		{
			Name: "Type mismatch should break tie",
			Method: ProtoMethodInfo{
				Name:         "FindUser",
				RequestType:  "FindUserRequest",
				ResponseType: "FindUserResponse",
			},
			Request: &ProtoMessageInfo{
				Name: "FindUserRequest",
				Fields: []ProtoFieldInfo{
					{Name: "email", ProtoType: "string", Number: 1},
				},
			},
			Response: &ProtoMessageInfo{Name: "FindUserResponse"},
			Procedures: []*Procedure{
				{
					Name: "usp_FindUserById",
					Parameters: []ProcParameter{
						{Name: "UserId", SQLType: "BIGINT", GoType: "int64"},
					},
					Operations: []Operation{{Type: OpSelect, Table: "Users"}},
				},
				{
					Name: "usp_FindUserByEmail",
					Parameters: []ProcParameter{
						{Name: "Email", SQLType: "NVARCHAR(255)", GoType: "string"},
					},
					Operations: []Operation{{Type: OpSelect, Table: "Users"}},
				},
			},
			ExpectedWinner: "usp_FindUserByEmail",
			Reason:         "Request has 'email' (string) field which matches ByEmail",
		},
	}

	for _, scenario := range edgeCases {
		t.Run(scenario.Name, func(t *testing.T) {
			proto := &ProtoParseResult{
				AllServices: map[string]*ProtoServiceInfo{
					"TestService": {
						Name:    "TestService",
						Methods: []ProtoMethodInfo{scenario.Method},
					},
				},
				AllMessages: map[string]*ProtoMessageInfo{
					scenario.Request.Name:  scenario.Request,
					scenario.Response.Name: scenario.Response,
				},
				AllMethods: make(map[string]*ProtoMethodInfo),
			}

			mapper := NewEnsembleMapper(proto, scenario.Procedures)
			mappings := mapper.MapAll()

			key := "TestService." + scenario.Method.Name
			mapping := mappings[key]

			if mapping == nil {
				t.Fatal("Expected a mapping")
			}

			t.Logf("Result: %s (%.0f%%)", mapping.Procedure.Name, mapping.Confidence*100)
			t.Logf("Expected: %s", scenario.ExpectedWinner)
			t.Logf("Reason: %s", scenario.Reason)

			// Score with composite method
			t.Log("\nComposite tie-breaker scores:")
			for _, proc := range scenario.Procedures {
				score := scoreProcForTieBreaker(TieBreakComposite, proc, &scenario.Method, scenario.Request, scenario.Response)
				t.Logf("  %s: %.3f", proc.Name, score)
			}
		})
	}
}

// BenchmarkTieBreakers measures overhead of different tie-breaking methods
func BenchmarkTieBreakers(b *testing.B) {
	scenario := createTieScenarios()[0]
	methods := []TieBreakerMethod{
		TieBreakLexicographic,
		TieBreakParamCountProximity,
		TieBreakComposite,
	}

	for _, method := range methods {
		b.Run(fmt.Sprintf("%s", method), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				for _, proc := range scenario.Procedures {
					scoreProcForTieBreaker(method, proc, &scenario.Method, scenario.Request, scenario.Response)
				}
			}
		})
	}
}
