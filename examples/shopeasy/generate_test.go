// +build ignore

// This is a standalone test script to evaluate tgpiler output quality.
// Run with: go run generate_test.go

package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/example/tgpiler/protogen"
	"github.com/example/tgpiler/storage"
)

func main() {
	fmt.Println("=" + strings.Repeat("=", 79))
	fmt.Println("ShopEasy tgpiler Integration Test")
	fmt.Println("=" + strings.Repeat("=", 79))
	fmt.Println()

	// Create output directory
	outputDir := "./generated"
	os.MkdirAll(outputDir, 0755)

	// Step 1: Parse proto files
	fmt.Println("STEP 1: Parsing Proto Files")
	fmt.Println("-" + strings.Repeat("-", 79))

	parser := protogen.NewParser("./protos")
	protoResult, err := parser.ParseDir("./protos")
	if err != nil {
		fmt.Printf("ERROR: Failed to parse protos: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("  Parsed %d proto files\n", len(protoResult.Files))
	fmt.Printf("  Found %d messages\n", len(protoResult.Messages))
	fmt.Printf("  Found %d services\n", len(protoResult.Services))

	totalMethods := 0
	for _, svc := range protoResult.Services {
		totalMethods += len(svc.Methods)
	}
	fmt.Printf("  Found %d methods\n", totalMethods)
	fmt.Println()

	// List services
	fmt.Println("  Services:")
	for _, svc := range protoResult.Services {
		fmt.Printf("    - %s (%d methods)\n", svc.Name, len(svc.Methods))
	}
	fmt.Println()

	// Step 2: Parse SQL procedures
	fmt.Println("STEP 2: Parsing SQL Stored Procedures")
	fmt.Println("-" + strings.Repeat("-", 79))

	detector := storage.NewSQLDetector()
	procDir := "./procedures"
	files, _ := filepath.Glob(filepath.Join(procDir, "*.sql"))

	var allOps []storage.Operation
	procCount := 0

	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			fmt.Printf("  WARNING: Could not read %s: %v\n", file, err)
			continue
		}

		ops, err := detector.DetectFromSQL(string(content))
		if err != nil {
			fmt.Printf("  WARNING: Could not parse %s: %v\n", file, err)
			continue
		}

		// Count procedures (CREATE PROCEDURE statements)
		procCount += strings.Count(strings.ToUpper(string(content)), "CREATE PROCEDURE")
		allOps = append(allOps, ops...)
		fmt.Printf("  Parsed %s: %d operations\n", filepath.Base(file), len(ops))
	}

	fmt.Printf("\n  Total: %d SQL files, ~%d procedures, %d operations detected\n", len(files), procCount, len(allOps))
	fmt.Println()

	// Analyze operations by table
	tableOps := make(map[string]map[string]int)
	for _, op := range allOps {
		if tableOps[op.Table] == nil {
			tableOps[op.Table] = make(map[string]int)
		}
		tableOps[op.Table][op.Type]++
	}

	fmt.Println("  Operations by table (top 10):")
	count := 0
	for table, ops := range tableOps {
		if count >= 10 {
			break
		}
		var opList []string
		for opType, cnt := range ops {
			opList = append(opList, fmt.Sprintf("%s:%d", opType, cnt))
		}
		fmt.Printf("    - %s: %s\n", table, strings.Join(opList, ", "))
		count++
	}
	fmt.Println()

	// Step 3: Generate Go server code for each service
	fmt.Println("STEP 3: Generating Go Server Code")
	fmt.Println("-" + strings.Repeat("-", 79))

	opts := protogen.DefaultServerGenOptions()

	for _, svc := range protoResult.Services {
		opts.PackageName = strings.ToLower(strings.TrimSuffix(svc.Name, "Service"))

		var buf bytes.Buffer
		err := protogen.GenerateFile(protoResult, svc.Name, opts, &buf)
		if err != nil {
			fmt.Printf("  ERROR generating %s: %v\n", svc.Name, err)
			continue
		}

		// Write to file
		outFile := filepath.Join(outputDir, strings.ToLower(svc.Name)+".go")
		err = os.WriteFile(outFile, buf.Bytes(), 0644)
		if err != nil {
			fmt.Printf("  ERROR writing %s: %v\n", outFile, err)
			continue
		}

		fmt.Printf("  Generated %s (%d bytes)\n", outFile, buf.Len())
	}
	fmt.Println()

	// Step 4: Quality assessment
	fmt.Println("STEP 4: Quality Assessment")
	fmt.Println("-" + strings.Repeat("-", 79))

	// Read generated files and analyze
	genFiles, _ := filepath.Glob(filepath.Join(outputDir, "*.go"))
	totalBytes := 0
	totalLines := 0

	for _, f := range genFiles {
		content, _ := os.ReadFile(f)
		totalBytes += len(content)
		totalLines += strings.Count(string(content), "\n")
	}

	fmt.Printf("  Generated %d Go files\n", len(genFiles))
	fmt.Printf("  Total size: %d bytes\n", totalBytes)
	fmt.Printf("  Total lines: %d\n", totalLines)
	fmt.Println()

	// Check for key patterns
	fmt.Println("  Checking generated code patterns:")

	patterns := map[string]string{
		"Server struct":       "type *Server struct",
		"Repository interface": "type *Repository interface",
		"Stub implementation": "RepositoryStub struct",
		"SQL implementation":  "RepositorySQL struct",
		"Context parameter":   "ctx context.Context",
		"Error handling":      "error",
		"TODO comments":       "// TODO:",
	}

	for _, f := range genFiles {
		content, _ := os.ReadFile(f)
		contentStr := string(content)

		fmt.Printf("\n    %s:\n", filepath.Base(f))
		for name, pattern := range patterns {
			// Simple pattern matching (not regex for speed)
			found := false
			if strings.Contains(pattern, "*") {
				// Wildcard pattern
				parts := strings.Split(pattern, "*")
				if len(parts) == 2 {
					found = strings.Contains(contentStr, parts[0]) && strings.Contains(contentStr, parts[1])
				}
			} else {
				found = strings.Contains(contentStr, pattern)
			}

			status := "✗"
			if found {
				status = "✓"
			}
			fmt.Printf("      %s %s\n", status, name)
		}
	}

	fmt.Println()
	fmt.Println("=" + strings.Repeat("=", 79))
	fmt.Println("Generation complete. Review files in ./generated/")
	fmt.Println("=" + strings.Repeat("=", 79))
}
