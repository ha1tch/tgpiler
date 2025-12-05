package protogen

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ha1tch/tgpiler/storage"
)

func TestImplementationGenerator_ShopEasy(t *testing.T) {
	exampleDir := "../examples/shopeasy"
	protoDir := filepath.Join(exampleDir, "protos")
	procDir := filepath.Join(exampleDir, "procedures")
	outputDir := filepath.Join(exampleDir, "generated_impl")

	os.MkdirAll(outputDir, 0755)

	t.Log("=" + strings.Repeat("=", 70))
	t.Log("Proto-to-SQL Mapping and Implementation Generation Test")
	t.Log("=" + strings.Repeat("=", 70))

	// Step 1: Parse protos
	t.Log("\n[STEP 1] Parsing Proto Files")
	parser := NewParser(protoDir)
	protoResult, err := parser.ParseDir(protoDir)
	if err != nil {
		t.Fatalf("Failed to parse protos: %v", err)
	}
	t.Logf("  Parsed %d services with %d methods", 
		len(protoResult.AllServices), len(protoResult.AllMethods))

	// Step 2: Extract stored procedures
	t.Log("\n[STEP 2] Extracting Stored Procedures")
	extractor := storage.NewProcedureExtractor()
	
	files, _ := filepath.Glob(filepath.Join(procDir, "*.sql"))
	var allProcs []*storage.Procedure

	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		procs, err := extractor.ExtractAll(string(content))
		if err != nil {
			t.Logf("  Warning: %s: %v", filepath.Base(file), err)
			continue
		}

		t.Logf("  %s: %d procedures", filepath.Base(file), len(procs))
		allProcs = append(allProcs, procs...)
	}
	t.Logf("  Total: %d stored procedures extracted", len(allProcs))

	// Show some procedure details
	t.Log("\n  Sample procedures with parameters:")
	for i, proc := range allProcs {
		if i >= 5 {
			break
		}
		paramNames := make([]string, len(proc.Parameters))
		for j, p := range proc.Parameters {
			suffix := ""
			if p.HasDefault {
				suffix = "=?"
			}
			if p.IsOutput {
				suffix += " OUT"
			}
			paramNames[j] = "@" + p.Name + " " + p.SQLType + suffix
		}
		t.Logf("    %s(%s)", proc.Name, strings.Join(paramNames, ", "))
		if len(proc.ResultSets) > 0 {
			cols := make([]string, 0)
			for _, c := range proc.ResultSets[0].Columns {
				cols = append(cols, c.Name)
			}
			if len(cols) > 5 {
				cols = append(cols[:5], "...")
			}
			t.Logf("      → returns: %s", strings.Join(cols, ", "))
		}
	}

	// Step 3: Map protos to procedures
	t.Log("\n[STEP 3] Mapping Proto Methods to Stored Procedures")
	gen := NewImplementationGenerator(protoResult, allProcs)
	stats := gen.GetStats()

	t.Logf("  Total methods:     %d", stats.TotalMethods)
	t.Logf("  Mapped methods:    %d (%.0f%%)", stats.MappedMethods, 
		float64(stats.MappedMethods)/float64(stats.TotalMethods)*100)
	t.Logf("  Unmapped methods:  %d", stats.UnmappedMethods)
	t.Logf("  High confidence:   %d (>80%%)", stats.HighConfidence)
	t.Logf("  Medium confidence: %d (50-80%%)", stats.MediumConfidence)
	t.Logf("  Low confidence:    %d (<50%%)", stats.LowConfidence)

	// Show mappings by service
	t.Log("\n  Mappings by service:")
	for svcName, svcStats := range stats.ByService {
		t.Logf("    %s: %d/%d mapped", svcName, svcStats.MappedMethods, svcStats.TotalMethods)
		for _, m := range svcStats.Mappings {
			t.Logf("      ✓ %s → %s (%.0f%% - %s)", 
				m.MethodName, m.Procedure.Name, m.Confidence*100, m.MatchReason)
			
			// Show parameter mappings
			if len(m.ParamMappings) > 0 {
				for _, pm := range m.ParamMappings {
					if pm.ProtoField != "" {
						t.Logf("        param: req.%s → @%s", pm.ProtoField, pm.ProcParam)
					}
				}
			}
		}
	}

	// Step 4: Generate implementation code
	t.Log("\n[STEP 4] Generating Implementation Code")
	
	opts := DefaultServerGenOptions()
	for svcName := range protoResult.AllServices {
		opts.PackageName = strings.ToLower(strings.TrimSuffix(svcName, "Service"))
		
		var buf bytes.Buffer
		err := gen.GenerateServiceImpl(svcName, opts, &buf)
		if err != nil {
			t.Errorf("Failed to generate %s: %v", svcName, err)
			continue
		}

		outFile := filepath.Join(outputDir, strings.ToLower(svcName)+"_impl.go")
		err = os.WriteFile(outFile, buf.Bytes(), 0644)
		if err != nil {
			t.Errorf("Failed to write %s: %v", outFile, err)
			continue
		}

		// Count implemented vs stub methods
		content := buf.String()
		implemented := strings.Count(content, "EXEC ")
		stub := strings.Count(content, "not implemented - no stored procedure mapping")
		
		t.Logf("  %s: %d bytes, %d implemented, %d stubs", 
			filepath.Base(outFile), buf.Len(), implemented, stub)
	}

	// Step 5: Show sample generated code
	t.Log("\n[STEP 5] Sample Generated Code")
	t.Log("-" + strings.Repeat("-", 70))
	
	// Show UserService as example
	userFile := filepath.Join(outputDir, "userservice_impl.go")
	if content, err := os.ReadFile(userFile); err == nil {
		lines := strings.Split(string(content), "\n")
		// Show first 60 lines
		maxLines := 60
		if len(lines) < maxLines {
			maxLines = len(lines)
		}
		for i := 0; i < maxLines; i++ {
			t.Logf("  %s", lines[i])
		}
		if len(lines) > maxLines {
			t.Logf("  ... (%d more lines)", len(lines)-maxLines)
		}
	}

	t.Log("\n" + strings.Repeat("=", 72))
	t.Logf("Output directory: %s", outputDir)
}

func TestProcedureExtractor(t *testing.T) {
	sql := `
CREATE PROCEDURE usp_GetUserById
    @UserId BIGINT
AS
BEGIN
    SET NOCOUNT ON;
    
    SELECT 
        Id, Email, Username, FirstName, LastName, Phone,
        IsActive, EmailVerified, CreatedAt, UpdatedAt
    FROM Users
    WHERE Id = @UserId;
END
GO

CREATE PROCEDURE usp_CreateUser
    @Email NVARCHAR(255),
    @Username NVARCHAR(100),
    @PasswordHash NVARCHAR(255),
    @FirstName NVARCHAR(100) = NULL,
    @LastName NVARCHAR(100) = NULL
AS
BEGIN
    INSERT INTO Users (Email, Username, PasswordHash, FirstName, LastName)
    VALUES (@Email, @Username, @PasswordHash, @FirstName, @LastName);
    
    SELECT SCOPE_IDENTITY() AS Id;
END
GO
`

	extractor := storage.NewProcedureExtractor()
	procs, err := extractor.ExtractAll(sql)
	if err != nil {
		t.Fatalf("Failed to extract: %v", err)
	}

	if len(procs) != 2 {
		t.Errorf("Expected 2 procedures, got %d", len(procs))
	}

	// Check first procedure
	if procs[0].Name != "usp_GetUserById" {
		t.Errorf("Expected usp_GetUserById, got %s", procs[0].Name)
	}
	if len(procs[0].Parameters) != 1 {
		t.Errorf("Expected 1 parameter, got %d", len(procs[0].Parameters))
	}
	if procs[0].Parameters[0].Name != "UserId" {
		t.Errorf("Expected UserId, got %s", procs[0].Parameters[0].Name)
	}
	if procs[0].Parameters[0].GoType != "int64" {
		t.Errorf("Expected int64, got %s", procs[0].Parameters[0].GoType)
	}

	// Check second procedure
	if procs[1].Name != "usp_CreateUser" {
		t.Errorf("Expected usp_CreateUser, got %s", procs[1].Name)
	}
	if len(procs[1].Parameters) != 5 {
		t.Errorf("Expected 5 parameters, got %d", len(procs[1].Parameters))
	}

	// Check optional parameters
	optionalCount := 0
	for _, p := range procs[1].Parameters {
		if p.HasDefault {
			optionalCount++
		}
	}
	if optionalCount != 2 {
		t.Errorf("Expected 2 optional parameters, got %d", optionalCount)
	}

	t.Log("Procedure extraction test passed")
	for _, proc := range procs {
		t.Logf("  %s: %d params, %d result sets", 
			proc.Name, len(proc.Parameters), len(proc.ResultSets))
	}
}

func TestProtoToSQLMapper(t *testing.T) {
	// Create a simple proto result
	proto := &storage.ProtoParseResult{
		AllServices: map[string]*storage.ProtoServiceInfo{
			"UserService": {
				Name: "UserService",
				Methods: []storage.ProtoMethodInfo{
					{Name: "GetUser", RequestType: "GetUserRequest", ResponseType: "GetUserResponse"},
					{Name: "CreateUser", RequestType: "CreateUserRequest", ResponseType: "CreateUserResponse"},
				},
			},
		},
		AllMessages: map[string]*storage.ProtoMessageInfo{
			"GetUserRequest": {
				Name: "GetUserRequest",
				Fields: []storage.ProtoFieldInfo{
					{Name: "id", ProtoType: "int64", Number: 1},
				},
			},
			"GetUserResponse": {
				Name: "GetUserResponse",
				Fields: []storage.ProtoFieldInfo{
					{Name: "user", ProtoType: "User", Number: 1},
				},
			},
			"CreateUserRequest": {
				Name: "CreateUserRequest",
				Fields: []storage.ProtoFieldInfo{
					{Name: "email", ProtoType: "string", Number: 1},
					{Name: "username", ProtoType: "string", Number: 2},
					{Name: "password", ProtoType: "string", Number: 3},
				},
			},
			"CreateUserResponse": {
				Name: "CreateUserResponse",
				Fields: []storage.ProtoFieldInfo{
					{Name: "user", ProtoType: "User", Number: 1},
				},
			},
		},
	}

	// Create procedures
	procs := []*storage.Procedure{
		{
			Name: "usp_GetUserById",
			Parameters: []storage.ProcParameter{
				{Name: "UserId", SQLType: "BIGINT", GoType: "int64"},
			},
			ResultSets: []storage.ResultSet{
				{
					FromTable: "Users",
					Columns: []storage.ResultColumn{
						{Name: "Id"},
						{Name: "Email"},
						{Name: "Username"},
					},
				},
			},
		},
		{
			Name: "usp_CreateUser",
			Parameters: []storage.ProcParameter{
				{Name: "Email", SQLType: "NVARCHAR(255)", GoType: "string"},
				{Name: "Username", SQLType: "NVARCHAR(100)", GoType: "string"},
				{Name: "PasswordHash", SQLType: "NVARCHAR(255)", GoType: "string"},
			},
		},
	}

	mapper := storage.NewProtoToSQLMapper(proto, procs)
	mappings := mapper.MapAll()

	if len(mappings) != 2 {
		t.Errorf("Expected 2 mappings, got %d", len(mappings))
	}

	// Check GetUser mapping
	getUserMapping := mappings["UserService.GetUser"]
	if getUserMapping == nil {
		t.Error("Expected GetUser mapping")
	} else {
		if getUserMapping.Procedure.Name != "usp_GetUserById" {
			t.Errorf("Expected usp_GetUserById, got %s", getUserMapping.Procedure.Name)
		}
		t.Logf("GetUser → %s (%.0f%% confidence)", 
			getUserMapping.Procedure.Name, getUserMapping.Confidence*100)
	}

	// Check CreateUser mapping
	createUserMapping := mappings["UserService.CreateUser"]
	if createUserMapping == nil {
		t.Error("Expected CreateUser mapping")
	} else {
		t.Logf("CreateUser → %s (%.0f%% confidence)", 
			createUserMapping.Procedure.Name, createUserMapping.Confidence*100)
	}

	stats := mapper.GetStats()
	t.Logf("Stats: %d/%d mapped, %d high confidence", 
		stats.MappedMethods, stats.TotalMethods, stats.HighConfidence)
}
