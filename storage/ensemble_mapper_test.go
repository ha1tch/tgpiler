package storage

import (
	"strings"
	"testing"
)

func TestEnsembleMapper_BasicMatching(t *testing.T) {
	t.Log("=" + strings.Repeat("=", 70))
	t.Log("Ensemble Mapper Test - Strategy Agreement/Disagreement")
	t.Log("=" + strings.Repeat("=", 70))

	// Create test proto result
	proto := &ProtoParseResult{
		AllServices: map[string]*ProtoServiceInfo{
			"UserService": {
				Name: "UserService",
				Methods: []ProtoMethodInfo{
					{Name: "GetUser", RequestType: "GetUserRequest", ResponseType: "GetUserResponse"},
					{Name: "CreateUser", RequestType: "CreateUserRequest", ResponseType: "CreateUserResponse"},
					{Name: "ValidateUser", RequestType: "ValidateUserRequest", ResponseType: "ValidateUserResponse"},
				},
			},
			"OrderService": {
				Name: "OrderService",
				Methods: []ProtoMethodInfo{
					{Name: "CreateOrder", RequestType: "CreateOrderRequest", ResponseType: "CreateOrderResponse"},
					{Name: "ListOrders", RequestType: "ListOrdersRequest", ResponseType: "ListOrdersResponse"},
				},
			},
		},
		AllMessages: map[string]*ProtoMessageInfo{
			"GetUserRequest": {
				Name: "GetUserRequest",
				Fields: []ProtoFieldInfo{
					{Name: "id", ProtoType: "int64", Number: 1},
				},
			},
			"GetUserResponse": {Name: "GetUserResponse"},
			"CreateUserRequest": {
				Name: "CreateUserRequest",
				Fields: []ProtoFieldInfo{
					{Name: "email", ProtoType: "string", Number: 1},
					{Name: "username", ProtoType: "string", Number: 2},
				},
			},
			"CreateUserResponse": {Name: "CreateUserResponse"},
			"ValidateUserRequest": {
				Name: "ValidateUserRequest",
				Fields: []ProtoFieldInfo{
					{Name: "user_id", ProtoType: "int64", Number: 1},
				},
			},
			"ValidateUserResponse": {Name: "ValidateUserResponse"},
			"CreateOrderRequest": {
				Name: "CreateOrderRequest",
				Fields: []ProtoFieldInfo{
					{Name: "user_id", ProtoType: "int64", Number: 1},
					{Name: "items", ProtoType: "OrderItem", Number: 2},
				},
			},
			"CreateOrderResponse": {Name: "CreateOrderResponse"},
			"ListOrdersRequest": {
				Name: "ListOrdersRequest",
				Fields: []ProtoFieldInfo{
					{Name: "user_id", ProtoType: "int64", Number: 1},
				},
			},
			"ListOrdersResponse": {Name: "ListOrdersResponse"},
		},
		AllMethods: make(map[string]*ProtoMethodInfo),
	}

	// Create test procedures
	procs := []*Procedure{
		{
			Name: "usp_GetUserById",
			Parameters: []ProcParameter{
				{Name: "UserId", SQLType: "BIGINT", GoType: "int64"},
			},
			Operations: []Operation{
				{Type: OpSelect, Table: "Users"},
			},
			ResultSets: []ResultSet{
				{FromTable: "Users", Columns: []ResultColumn{{Name: "Id"}, {Name: "Email"}}},
			},
		},
		{
			Name: "usp_CreateUser",
			Parameters: []ProcParameter{
				{Name: "Email", SQLType: "NVARCHAR(255)", GoType: "string"},
				{Name: "Username", SQLType: "NVARCHAR(100)", GoType: "string"},
			},
			Operations: []Operation{
				{Type: OpInsert, Table: "Users"},
			},
		},
		{
			Name: "usp_ValidateUser",
			Parameters: []ProcParameter{
				{Name: "UserId", SQLType: "BIGINT", GoType: "int64"},
			},
			Operations: []Operation{
				{Type: OpSelect, Table: "Users"},
			},
		},
		{
			Name: "usp_CreateOrder",
			Parameters: []ProcParameter{
				{Name: "UserId", SQLType: "BIGINT", GoType: "int64"},
			},
			Operations: []Operation{
				{Type: OpInsert, Table: "Orders"},
			},
		},
		{
			Name: "usp_ListOrders",
			Parameters: []ProcParameter{
				{Name: "UserId", SQLType: "BIGINT", GoType: "int64"},
			},
			Operations: []Operation{
				{Type: OpSelect, Table: "Orders"},
			},
		},
	}

	t.Logf("Test data: %d services, %d procedures\n",
		len(proto.AllServices), len(procs))

	// Run original mapper
	t.Log("\n[ORIGINAL MAPPER]")
	origMapper := NewProtoToSQLMapper(proto, procs)
	origMappings := origMapper.MapAll()
	origStats := origMapper.GetStats()

	t.Logf("  Mapped: %d/%d (%.0f%%)", origStats.MappedMethods, origStats.TotalMethods,
		float64(origStats.MappedMethods)/float64(origStats.TotalMethods)*100)

	// Run ensemble mapper
	t.Log("\n[ENSEMBLE MAPPER]")
	ensMapper := NewEnsembleMapper(proto, procs)
	ensMappings := ensMapper.MapAll()
	ensStats := ensMapper.GetStats()

	t.Logf("  Mapped: %d/%d (%.0f%%)", ensStats.MappedMethods, ensStats.TotalMethods,
		float64(ensStats.MappedMethods)/float64(ensStats.TotalMethods)*100)
	t.Logf("  High confidence: %d, Medium: %d, Low: %d",
		ensStats.HighConfidence, ensStats.MediumConfidence, ensStats.LowConfidence)

	// Show all mappings with strategy breakdown
	t.Log("\n[MAPPINGS WITH STRATEGY BREAKDOWN]")
	for svcName, svc := range proto.AllServices {
		for _, method := range svc.Methods {
			key := svcName + "." + method.Name
			
			origMap := origMappings[key]
			ensMap := ensMappings[key]

			t.Logf("\n  %s:", key)
			if origMap != nil {
				t.Logf("    Original:  %s (%.0f%%) - %s", 
					origMap.Procedure.Name, origMap.Confidence*100, origMap.MatchReason)
			} else {
				t.Logf("    Original:  NO MAPPING")
			}
			if ensMap != nil {
				t.Logf("    Ensemble:  %s (%.0f%%)", 
					ensMap.Procedure.Name, ensMap.Confidence*100)
				t.Logf("    Strategies: %s", ensMap.MatchReason)
			} else {
				t.Logf("    Ensemble:  NO MAPPING")
			}
		}
	}

	// Verify all methods got mapped
	if ensStats.MappedMethods != ensStats.TotalMethods {
		t.Errorf("Expected all %d methods to be mapped, got %d", 
			ensStats.TotalMethods, ensStats.MappedMethods)
	}
}

func TestEnsembleMapper_StrategyAgreement(t *testing.T) {
	// Test case where strategies agree
	proto := &ProtoParseResult{
		AllServices: map[string]*ProtoServiceInfo{
			"UserService": {
				Name: "UserService",
				Methods: []ProtoMethodInfo{
					{Name: "GetUser", RequestType: "GetUserRequest", ResponseType: "GetUserResponse"},
				},
			},
		},
		AllMessages: map[string]*ProtoMessageInfo{
			"GetUserRequest": {
				Name: "GetUserRequest",
				Fields: []ProtoFieldInfo{
					{Name: "id", ProtoType: "int64", Number: 1},
				},
			},
			"GetUserResponse": {
				Name: "GetUserResponse",
				Fields: []ProtoFieldInfo{
					{Name: "user", ProtoType: "User", Number: 1},
				},
			},
		},
		AllMethods: make(map[string]*ProtoMethodInfo),
	}

	procs := []*Procedure{
		{
			Name: "usp_GetUserById",
			Parameters: []ProcParameter{
				{Name: "UserId", SQLType: "BIGINT", GoType: "int64"},
			},
			Operations: []Operation{
				{Type: OpSelect, Table: "Users"},
			},
			ResultSets: []ResultSet{
				{FromTable: "Users", Columns: []ResultColumn{{Name: "Id"}, {Name: "Email"}}},
			},
		},
		{
			Name: "usp_UpdateUser", // Should NOT match
			Parameters: []ProcParameter{
				{Name: "UserId", SQLType: "BIGINT", GoType: "int64"},
			},
			Operations: []Operation{
				{Type: OpUpdate, Table: "Users"},
			},
		},
	}

	mapper := NewEnsembleMapper(proto, procs)
	mappings := mapper.MapAll()

	mapping := mappings["UserService.GetUser"]
	if mapping == nil {
		t.Fatal("Expected mapping for GetUser")
	}

	t.Logf("GetUser mapped to: %s", mapping.Procedure.Name)
	t.Logf("Confidence: %.0f%%", mapping.Confidence*100)
	t.Logf("Reasons: %s", mapping.MatchReason)

	if mapping.Procedure.Name != "usp_GetUserById" {
		t.Errorf("Expected usp_GetUserById, got %s", mapping.Procedure.Name)
	}

	// With multiple strategies agreeing, confidence should be higher
	if mapping.Confidence < 0.85 {
		t.Errorf("Expected confidence > 85%% with strategy agreement, got %.0f%%", mapping.Confidence*100)
	}

	// Should have multiple strategies in reason
	strategyCount := strings.Count(mapping.MatchReason, ";") + 1
	t.Logf("Number of agreeing strategies: %d", strategyCount)
	if strategyCount < 2 {
		t.Errorf("Expected multiple strategies to agree, got %d", strategyCount)
	}
}

func TestEnsembleMapper_StrategyDisagreement(t *testing.T) {
	// Test case where naming matches but DML doesn't
	proto := &ProtoParseResult{
		AllServices: map[string]*ProtoServiceInfo{
			"UserService": {
				Name: "UserService",
				Methods: []ProtoMethodInfo{
					{Name: "GetUser", RequestType: "GetUserRequest", ResponseType: "GetUserResponse"},
				},
			},
		},
		AllMessages: map[string]*ProtoMessageInfo{
			"GetUserRequest": {
				Name: "GetUserRequest",
				Fields: []ProtoFieldInfo{
					{Name: "id", ProtoType: "int64", Number: 1},
				},
			},
			"GetUserResponse": {
				Name: "GetUserResponse",
				Fields: []ProtoFieldInfo{
					{Name: "user", ProtoType: "User", Number: 1},
				},
			},
		},
		AllMethods: make(map[string]*ProtoMethodInfo),
	}

	procs := []*Procedure{
		{
			Name: "usp_GetUserById", // Name matches GetUser
			Parameters: []ProcParameter{
				{Name: "UserId", SQLType: "BIGINT", GoType: "int64"},
			},
			Operations: []Operation{
				{Type: OpSelect, Table: "Products"}, // But it selects from Products!
			},
		},
		{
			Name: "usp_FetchUserData", // Name doesn't match as well
			Parameters: []ProcParameter{
				{Name: "UserId", SQLType: "BIGINT", GoType: "int64"},
			},
			Operations: []Operation{
				{Type: OpSelect, Table: "Users"}, // But DML is correct
			},
		},
	}

	mapper := NewEnsembleMapper(proto, procs)
	mappings := mapper.MapAll()

	mapping := mappings["UserService.GetUser"]
	if mapping == nil {
		t.Fatal("Expected mapping for GetUser")
	}

	t.Logf("GetUser mapped to: %s", mapping.Procedure.Name)
	t.Logf("Confidence: %.0f%%", mapping.Confidence*100)
	t.Logf("Reasons: %s", mapping.MatchReason)

	// The mapper should handle the conflicting signals
	// Either it picks the name match with lower confidence
	// Or it picks the DML match
	// Either way, confidence should reflect the disagreement
}

func TestVerbEntityParsing(t *testing.T) {
	testCases := []struct {
		name         string
		expectedVerb string
		expectedEntity string
	}{
		{"GetUser", "Get", "User"},
		{"GetUserById", "Get", "User"},
		{"ListOrders", "List", "Orders"},
		{"CreateProduct", "Create", "Product"},
		{"ValidateFxRate", "Validate", "FxRate"},
		{"IsValidCurrency", "IsValid", "Currency"},
		{"Authenticate", "Authenticate", ""},
		{"CheckDatabaseConnection", "CheckDatabase", "Connection"},
		{"GetActiveTransmitter", "GetActive", "Transmitter"},
		{"ConvertCurrency", "Convert", "Currency"},
	}

	for _, tc := range testCases {
		verb, entity := parseVerbEntity(tc.name)
		if verb != tc.expectedVerb || entity != tc.expectedEntity {
			t.Errorf("%s: expected (%s, %s), got (%s, %s)",
				tc.name, tc.expectedVerb, tc.expectedEntity, verb, entity)
		} else {
			t.Logf("✓ %s -> verb=%s, entity=%s", tc.name, verb, entity)
		}
	}
}

func TestVerbGroupMatching(t *testing.T) {
	testCases := []struct {
		v1, v2   string
		expected float64
	}{
		{"Get", "Fetch", 0.8},
		{"Get", "Get", 1.0},
		{"Create", "Insert", 0.8},
		{"Validate", "Check", 0.8},
		{"Get", "Delete", 0.0},
		{"List", "Search", 0.8},
	}

	for _, tc := range testCases {
		score := scoreVerbMatch(tc.v1, tc.v2)
		if score != tc.expected {
			t.Errorf("%s vs %s: expected %.1f, got %.1f", tc.v1, tc.v2, tc.expected, score)
		} else {
			t.Logf("✓ %s vs %s = %.1f", tc.v1, tc.v2, score)
		}
	}
}

func TestEnsembleMapper_ChangePassword(t *testing.T) {
	// Specific test for ChangePassword mapping issue
	proto := &ProtoParseResult{
		AllServices: map[string]*ProtoServiceInfo{
			"UserService": {
				Name: "UserService",
				Methods: []ProtoMethodInfo{
					{Name: "ChangePassword", RequestType: "ChangePasswordRequest", ResponseType: "ChangePasswordResponse"},
				},
			},
		},
		AllMessages: map[string]*ProtoMessageInfo{
			"ChangePasswordRequest": {
				Name: "ChangePasswordRequest",
				Fields: []ProtoFieldInfo{
					{Name: "user_id", ProtoType: "int64", Number: 1},
					{Name: "current_password", ProtoType: "string", Number: 2},
					{Name: "new_password", ProtoType: "string", Number: 3},
				},
			},
			"ChangePasswordResponse": {Name: "ChangePasswordResponse"},
		},
		AllMethods: make(map[string]*ProtoMethodInfo),
	}

	procs := []*Procedure{
		{
			Name: "usp_ChangePassword",
			Parameters: []ProcParameter{
				{Name: "UserId", SQLType: "BIGINT", GoType: "int64"},
				{Name: "CurrentPasswordHash", SQLType: "NVARCHAR(255)", GoType: "string"},
				{Name: "NewPasswordHash", SQLType: "NVARCHAR(255)", GoType: "string"},
				{Name: "NewSalt", SQLType: "NVARCHAR(100)", GoType: "string"},
			},
			Operations: []Operation{
				{Type: OpUpdate, Table: "Users"},
			},
		},
		{
			Name: "usp_ClearCart",
			Parameters: []ProcParameter{
				{Name: "UserId", SQLType: "BIGINT", GoType: "int64"},
			},
			Operations: []Operation{
				{Type: OpDelete, Table: "CartItems"},
			},
		},
	}

	mapper := NewEnsembleMapper(proto, procs)
	mappings := mapper.MapAll()

	mapping := mappings["UserService.ChangePassword"]
	if mapping == nil {
		t.Fatal("Expected mapping for ChangePassword")
	}

	t.Logf("ChangePassword mapped to: %s", mapping.Procedure.Name)
	t.Logf("Confidence: %.0f%%", mapping.Confidence*100)
	t.Logf("Reasons: %s", mapping.MatchReason)

	if mapping.Procedure.Name != "usp_ChangePassword" {
		t.Errorf("Expected usp_ChangePassword, got %s", mapping.Procedure.Name)
	}
}

func TestEnsembleMapper_BusinessProcessVerbs(t *testing.T) {
	// Test that business process verbs (approve, reject, certify, etc.) are matched correctly
	proto := &ProtoParseResult{
		AllServices: map[string]*ProtoServiceInfo{
			"WorkflowService": {
				Name: "WorkflowService",
				Methods: []ProtoMethodInfo{
					{Name: "ApproveRequest", RequestType: "ApproveRequestRequest", ResponseType: "ApproveRequestResponse"},
					{Name: "RejectRequest", RequestType: "RejectRequestRequest", ResponseType: "RejectRequestResponse"},
					{Name: "CertifyDocument", RequestType: "CertifyDocumentRequest", ResponseType: "CertifyDocumentResponse"},
					{Name: "EscalateTicket", RequestType: "EscalateTicketRequest", ResponseType: "EscalateTicketResponse"},
					{Name: "SuspendAccount", RequestType: "SuspendAccountRequest", ResponseType: "SuspendAccountResponse"},
				},
			},
		},
		AllMessages: map[string]*ProtoMessageInfo{
			"ApproveRequestRequest":    {Name: "ApproveRequestRequest", Fields: []ProtoFieldInfo{{Name: "request_id", ProtoType: "int64", Number: 1}}},
			"ApproveRequestResponse":   {Name: "ApproveRequestResponse"},
			"RejectRequestRequest":     {Name: "RejectRequestRequest", Fields: []ProtoFieldInfo{{Name: "request_id", ProtoType: "int64", Number: 1}}},
			"RejectRequestResponse":    {Name: "RejectRequestResponse"},
			"CertifyDocumentRequest":   {Name: "CertifyDocumentRequest", Fields: []ProtoFieldInfo{{Name: "document_id", ProtoType: "int64", Number: 1}}},
			"CertifyDocumentResponse":  {Name: "CertifyDocumentResponse"},
			"EscalateTicketRequest":    {Name: "EscalateTicketRequest", Fields: []ProtoFieldInfo{{Name: "ticket_id", ProtoType: "int64", Number: 1}}},
			"EscalateTicketResponse":   {Name: "EscalateTicketResponse"},
			"SuspendAccountRequest":    {Name: "SuspendAccountRequest", Fields: []ProtoFieldInfo{{Name: "account_id", ProtoType: "int64", Number: 1}}},
			"SuspendAccountResponse":   {Name: "SuspendAccountResponse"},
		},
		AllMethods: make(map[string]*ProtoMethodInfo),
	}

	procs := []*Procedure{
		{
			Name:       "usp_ApproveRequest",
			Parameters: []ProcParameter{{Name: "RequestId", SQLType: "BIGINT", GoType: "int64"}},
			Operations: []Operation{{Type: OpUpdate, Table: "Requests"}},
		},
		{
			Name:       "usp_RejectRequest",
			Parameters: []ProcParameter{{Name: "RequestId", SQLType: "BIGINT", GoType: "int64"}},
			Operations: []Operation{{Type: OpUpdate, Table: "Requests"}},
		},
		{
			Name:       "usp_CertifyDocument",
			Parameters: []ProcParameter{{Name: "DocumentId", SQLType: "BIGINT", GoType: "int64"}},
			Operations: []Operation{{Type: OpUpdate, Table: "Documents"}},
		},
		{
			Name:       "usp_EscalateTicket",
			Parameters: []ProcParameter{{Name: "TicketId", SQLType: "BIGINT", GoType: "int64"}},
			Operations: []Operation{{Type: OpUpdate, Table: "Tickets"}},
		},
		{
			Name:       "usp_SuspendAccount",
			Parameters: []ProcParameter{{Name: "AccountId", SQLType: "BIGINT", GoType: "int64"}},
			Operations: []Operation{{Type: OpUpdate, Table: "Accounts"}},
		},
	}

	mapper := NewEnsembleMapper(proto, procs)
	results := mapper.MapAll()

	// Check each business process verb is matched correctly
	testCases := []struct {
		method   string
		expected string
	}{
		{"ApproveRequest", "usp_ApproveRequest"},
		{"RejectRequest", "usp_RejectRequest"},
		{"CertifyDocument", "usp_CertifyDocument"},
		{"EscalateTicket", "usp_EscalateTicket"},
		{"SuspendAccount", "usp_SuspendAccount"},
	}

	for _, tc := range testCases {
		key := "WorkflowService." + tc.method
		mapping, ok := results[key]
		if !ok {
			t.Errorf("No mapping found for %s", tc.method)
			continue
		}

		if mapping.Procedure == nil {
			t.Errorf("%s: no procedure matched", tc.method)
			continue
		}

		if mapping.Procedure.Name != tc.expected {
			t.Errorf("%s: expected %s, got %s", tc.method, tc.expected, mapping.Procedure.Name)
		}

		t.Logf("%s -> %s (%.0f%% confidence)", tc.method, mapping.Procedure.Name, mapping.Confidence*100)
	}
}
