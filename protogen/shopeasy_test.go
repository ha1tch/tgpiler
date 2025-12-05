package protogen

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ha1tch/tgpiler/storage"
)

// TestShopEasyIntegration runs tgpiler against the ShopEasy example
// to assess quality of generated code.
func TestShopEasyIntegration(t *testing.T) {
	exampleDir := "../examples/shopeasy"
	protoDir := filepath.Join(exampleDir, "protos")
	procDir := filepath.Join(exampleDir, "procedures")
	outputDir := filepath.Join(exampleDir, "generated")

	// Create output directory
	os.MkdirAll(outputDir, 0755)

	t.Log("=" + strings.Repeat("=", 60))
	t.Log("ShopEasy tgpiler Integration Test")
	t.Log("=" + strings.Repeat("=", 60))

	// =========================================================================
	// STEP 1: Parse Proto Files
	// =========================================================================
	t.Log("\nSTEP 1: Parsing Proto Files")
	t.Log("-" + strings.Repeat("-", 60))

	parser := NewParser(protoDir)
	protoResult, err := parser.ParseDir(protoDir)
	if err != nil {
		t.Fatalf("Failed to parse protos: %v", err)
	}

	t.Logf("  Parsed %d proto files", len(protoResult.Files))
	t.Logf("  Found %d messages", len(protoResult.AllMessages))
	t.Logf("  Found %d services", len(protoResult.AllServices))

	totalMethods := len(protoResult.AllMethods)
	t.Logf("  Found %d methods", totalMethods)

	// Verify expected services exist
	expectedServices := []string{
		"UserService",
		"CatalogService",
		"CartService",
		"OrderService",
		"InventoryService",
		"ReviewService",
	}

	for _, expected := range expectedServices {
		if _, ok := protoResult.AllServices[expected]; !ok {
			t.Errorf("Missing expected service: %s", expected)
		}
	}

	for svcName, svc := range protoResult.AllServices {
		t.Logf("    - %s (%d methods)", svcName, len(svc.Methods))
	}

	// =========================================================================
	// STEP 2: Parse SQL Procedures
	// =========================================================================
	t.Log("\nSTEP 2: Parsing SQL Stored Procedures")
	t.Log("-" + strings.Repeat("-", 60))

	detector := storage.NewSQLDetector(storage.DetectorConfig{})
	files, err := filepath.Glob(filepath.Join(procDir, "*.sql"))
	if err != nil {
		t.Fatalf("Failed to glob SQL files: %v", err)
	}

	var allOps []storage.Operation
	procCount := 0

	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			t.Logf("  WARNING: Could not read %s: %v", file, err)
			continue
		}

		ops, err := detector.DetectFromSQL(string(content))
		if err != nil {
			t.Logf("  WARNING: Could not parse %s: %v", file, err)
			continue
		}

		procCount += strings.Count(strings.ToUpper(string(content)), "CREATE PROCEDURE")
		allOps = append(allOps, ops...)
		t.Logf("  %s: %d operations", filepath.Base(file), len(ops))
	}

	t.Logf("\n  Total: %d SQL files, ~%d procedures, %d operations", len(files), procCount, len(allOps))

	// Analyze by table
	tableOps := make(map[string]int)
	for _, op := range allOps {
		tableOps[op.Table]++
	}
	t.Logf("  Unique tables referenced: %d", len(tableOps))

	// =========================================================================
	// STEP 3: Generate Go Server Code
	// =========================================================================
	t.Log("\nSTEP 3: Generating Go Server Code")
	t.Log("-" + strings.Repeat("-", 60))

	opts := DefaultServerGenOptions()
	generatedFiles := make(map[string][]byte)

	for svcName := range protoResult.AllServices {
		opts.PackageName = strings.ToLower(strings.TrimSuffix(svcName, "Service"))

		var buf bytes.Buffer
		err := GenerateFile(protoResult, svcName, opts, &buf)
		if err != nil {
			t.Errorf("Failed to generate %s: %v", svcName, err)
			continue
		}

		outFile := filepath.Join(outputDir, strings.ToLower(svcName)+".go")
		err = os.WriteFile(outFile, buf.Bytes(), 0644)
		if err != nil {
			t.Errorf("Failed to write %s: %v", outFile, err)
			continue
		}

		generatedFiles[svcName] = buf.Bytes()
		t.Logf("  Generated %s (%d bytes, %d lines)",
			filepath.Base(outFile), buf.Len(), strings.Count(buf.String(), "\n"))
	}

	// =========================================================================
	// STEP 4: Quality Assessment
	// =========================================================================
	t.Log("\nSTEP 4: Quality Assessment")
	t.Log("-" + strings.Repeat("-", 60))

	totalBytes := 0
	totalLines := 0
	for _, content := range generatedFiles {
		totalBytes += len(content)
		totalLines += strings.Count(string(content), "\n")
	}

	t.Logf("  Generated %d Go files", len(generatedFiles))
	t.Logf("  Total size: %d bytes", totalBytes)
	t.Logf("  Total lines: %d", totalLines)

	// Check each generated file
	for svcName, content := range generatedFiles {
		contentStr := string(content)
		t.Logf("\n  %s:", svcName)

		// Check required patterns
		checks := []struct {
			name    string
			pattern string
		}{
			{"Server struct", "type " + svcName + "Server struct"},
			{"Repository interface", "type " + svcName + "Repository interface"},
			{"Stub struct", svcName + "RepositoryStub struct"},
			{"SQL struct", svcName + "RepositorySQL struct"},
			{"Context import", `"context"`},
			{"Database import", `"database/sql"`},
			{"Fmt import", `"fmt"`},
		}

		for _, check := range checks {
			if strings.Contains(contentStr, check.pattern) {
				t.Logf("    ✓ %s", check.name)
			} else {
				t.Logf("    ✗ %s (missing: %s)", check.name, check.pattern)
			}
		}

		// Count methods in stub
		svc := protoResult.AllServices[svcName]
		if svc != nil {
			expectedMethods := len(svc.Methods)
			actualMethods := strings.Count(contentStr, "func (r *"+svcName+"RepositoryStub)")
			if actualMethods == expectedMethods {
				t.Logf("    ✓ All %d methods implemented in stub", expectedMethods)
			} else {
				t.Logf("    ✗ Method count mismatch: expected %d, got %d", expectedMethods, actualMethods)
			}
		}
	}

	// =========================================================================
	// STEP 5: Proto-to-SQL Mapping Analysis
	// =========================================================================
	t.Log("\n\nSTEP 5: Proto-to-SQL Mapping Analysis")
	t.Log("-" + strings.Repeat("-", 60))

	// Try to match proto methods to SQL tables
	mappings := make(map[string][]string)
	unmapped := []string{}

	for svcName, svc := range protoResult.AllServices {
		for _, method := range svc.Methods {
			methodName := method.Name
			
			// Extract table name from method name
			tableName := ""
			for _, prefix := range []string{"Get", "List", "Create", "Update", "Delete", "Add", "Remove", "Validate", "Process", "Search", "Moderate", "Vote", "Clear", "Release", "Reserve", "Adjust", "Set", "Refresh", "Change", "Deactivate", "Verify", "Register", "Login"} {
				if strings.HasPrefix(methodName, prefix) {
					candidate := strings.TrimPrefix(methodName, prefix)
					// Handle common suffixes
					candidate = strings.TrimSuffix(candidate, "s")
					candidate = strings.TrimSuffix(candidate, "ById")
					candidate = strings.TrimSuffix(candidate, "ByEmail")
					candidate = strings.TrimSuffix(candidate, "BySlug")
					candidate = strings.TrimSuffix(candidate, "BySku")
					candidate = strings.TrimSuffix(candidate, "ByNumber")
					candidate = strings.TrimSuffix(candidate, "Item")
					candidate = strings.TrimSuffix(candidate, "Level")
					candidate = strings.TrimSuffix(candidate, "Status")
					candidate = strings.TrimSuffix(candidate, "Token")
					candidate = strings.TrimSuffix(candidate, "Code")
					candidate = strings.TrimSuffix(candidate, "Password")
					candidate = strings.TrimSuffix(candidate, "Email")
					candidate = strings.TrimSuffix(candidate, "Summary")
					candidate = strings.TrimSuffix(candidate, "Point")
					candidate = strings.TrimSuffix(candidate, "Pending")
					candidate = strings.TrimSuffix(candidate, "Stock")
					candidate = strings.TrimSuffix(candidate, "Low")
					candidate = strings.TrimSuffix(candidate, "ToCart")
					candidate = strings.TrimSuffix(candidate, "FromCart")
					candidate = strings.TrimSuffix(candidate, "Cart")
					
					if len(candidate) > 0 {
						tableName = candidate
						break
					}
				}
			}

			fullMethod := svcName + "." + methodName
			if tableName != "" {
				// Check if table exists in SQL operations
				found := false
				for table := range tableOps {
					if strings.EqualFold(table, tableName) || 
					   strings.EqualFold(table, tableName+"s") ||
					   strings.EqualFold(tableName, table+"s") {
						mappings[fullMethod] = append(mappings[fullMethod], table)
						found = true
					}
				}
				if !found {
					unmapped = append(unmapped, fullMethod+" (inferred: "+tableName+")")
				}
			} else {
				unmapped = append(unmapped, fullMethod)
			}
		}
	}

	mapped := 0
	for _, tables := range mappings {
		if len(tables) > 0 {
			mapped++
		}
	}
	t.Logf("  Mapped methods: %d", mapped)
	t.Logf("  Unmapped methods: %d", len(unmapped))

	if len(unmapped) > 0 && len(unmapped) <= 15 {
		t.Log("  Unmapped:")
		for _, m := range unmapped {
			t.Logf("    - %s", m)
		}
	}

	// =========================================================================
	// Summary
	// =========================================================================
	t.Log("\n" + strings.Repeat("=", 62))
	t.Log("SUMMARY")
	t.Log(strings.Repeat("=", 62))
	t.Logf("  Proto files:      %d", len(protoResult.Files))
	t.Logf("  Services:         %d", len(protoResult.AllServices))
	t.Logf("  Methods:          %d", totalMethods)
	t.Logf("  Messages:         %d", len(protoResult.AllMessages))
	t.Logf("  SQL files:        %d", len(files))
	t.Logf("  SQL operations:   %d", len(allOps))
	t.Logf("  Generated files:  %d", len(generatedFiles))
	t.Logf("  Generated lines:  %d", totalLines)
	t.Log(strings.Repeat("=", 62))
	t.Logf("  Output directory: %s", outputDir)
}
