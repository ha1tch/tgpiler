// Structured data tests for tgpiler (JSON/XML with DML mode)
// Tests transpilation, compilation, and execution of JSON/XML procedures
// Run with: go test -v ./tests/... -run TestStructured

package tests

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestCompilationStructuredDML verifies all tsql_structured samples transpile to valid Go with --dml flag
func TestCompilationStructuredDML(t *testing.T) {
	testTranspileDirectoryDML(t, "../tsql_structured")
}

// testTranspileDirectoryDML transpiles all .sql files with --dml flag and verifies syntax
func testTranspileDirectoryDML(t *testing.T, dir string) {
	tgpiler := findTgpiler(t)

	absDir, err := filepath.Abs(dir)
	if err != nil {
		t.Fatalf("Failed to get absolute path for %s: %v", dir, err)
	}

	sqlFiles, err := filepath.Glob(filepath.Join(absDir, "*.sql"))
	if err != nil {
		t.Fatalf("Failed to glob SQL files: %v", err)
	}

	if len(sqlFiles) == 0 {
		t.Fatalf("No SQL files found in %s", absDir)
	}

	t.Logf("Found %d SQL files in %s", len(sqlFiles), dir)

	// Create temp directory for output
	tmpDir, err := os.MkdirTemp("", "tgpiler-structured-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Test each file with --dml flag
	for _, sqlFile := range sqlFiles {
		baseName := filepath.Base(sqlFile)
		goName := strings.TrimSuffix(baseName, ".sql") + ".go"
		goPath := filepath.Join(tmpDir, goName)

		t.Run(baseName, func(t *testing.T) {
			// Transpile with --dml flag
			cmd := exec.Command(tgpiler, "--dml", sqlFile)
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("Transpilation failed: %v\nOutput: %s", err, string(output))
			}

			// Write to temp file
			if err := os.WriteFile(goPath, output, 0644); err != nil {
				t.Fatalf("Failed to write Go file: %v", err)
			}

			// Verify syntax with gofmt
			fmtCmd := exec.Command("gofmt", "-e", goPath)
			if fmtOutput, err := fmtCmd.CombinedOutput(); err != nil {
				t.Errorf("Syntax error in generated Go code:\n%s", string(fmtOutput))
			}
		})
	}
}

// TestStructuredFullBuild transpiles all structured data SQL files and builds as a package
func TestStructuredFullBuild(t *testing.T) {
	tgpiler := findTgpiler(t)
	absDir, err := filepath.Abs("../tsql_structured")
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	sqlFiles, err := filepath.Glob(filepath.Join(absDir, "*.sql"))
	if err != nil {
		t.Fatalf("Failed to glob SQL files: %v", err)
	}

	if len(sqlFiles) == 0 {
		t.Skip("No SQL files found in tsql_structured")
	}

	// Create workspace with go.mod
	workspace, err := os.MkdirTemp("", "tgpiler-structured-build-*")
	if err != nil {
		t.Fatalf("Failed to create workspace: %v", err)
	}
	defer os.RemoveAll(workspace)

	// Create go.mod
	goMod := `module teststructured

go 1.22.2

require (
	github.com/ha1tch/tgpiler v0.0.0
	github.com/shopspring/decimal v1.3.1
)

replace github.com/ha1tch/tgpiler => ` + mustAbs(t, "..") + `
`
	if err := os.WriteFile(filepath.Join(workspace, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	// Transpile all files with --dml
	for _, sqlFile := range sqlFiles {
		baseName := filepath.Base(sqlFile)
		goName := strings.TrimSuffix(baseName, ".sql") + ".go"
		goPath := filepath.Join(workspace, goName)

		cmd := exec.Command(tgpiler, "--dml", sqlFile)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Transpilation of %s failed: %v\nOutput: %s", baseName, err, string(output))
		}

		if err := os.WriteFile(goPath, output, 0644); err != nil {
			t.Fatalf("Failed to write %s: %v", goName, err)
		}
	}

	// Create stubs.go with runtime support
	stubsGo := generateStructuredStubs()
	if err := os.WriteFile(filepath.Join(workspace, "stubs.go"), []byte(stubsGo), 0644); err != nil {
		t.Fatalf("Failed to write stubs.go: %v", err)
	}

	// Create main.go
	mainGo := `package main

func main() {}
`
	if err := os.WriteFile(filepath.Join(workspace, "main.go"), []byte(mainGo), 0644); err != nil {
		t.Fatalf("Failed to write main.go: %v", err)
	}

	// Run go mod tidy
	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = workspace
	tidyCmd.Env = append(os.Environ(), "GONOSUMDB=*", "GOPRIVATE=*")
	if tidyOutput, err := tidyCmd.CombinedOutput(); err != nil {
		t.Fatalf("go mod tidy failed: %v\n%s", err, string(tidyOutput))
	}

	// Build the package
	buildCmd := exec.Command("go", "build", ".")
	buildCmd.Dir = workspace
	buildCmd.Env = append(os.Environ(), "GONOSUMDB=*", "GOPRIVATE=*")
	if buildOutput, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Build failed: %v\n%s", err, string(buildOutput))
	}

	t.Logf("Successfully built %d structured data procedures", len(sqlFiles))
}

// TestE2EExecuteJSON tests JSON function execution
func TestE2EExecuteJSON(t *testing.T) {
	workspace := setupStructuredWorkspace(t)
	defer os.RemoveAll(workspace)

	tgpiler := findTgpiler(t)

	tests := []struct {
		sqlFile  string
		testCode string
		expected string
	}{
		{
			sqlFile: "../tsql_structured/01_json_value_extract.sql",
			testCode: `
				customerName, customerId, email := r.ParseCustomerJson(ctx, ` + "`" + `{"customer":{"id":42,"name":"Alice","email":"alice@example.com"}}` + "`" + `)
				fmt.Printf("%s,%d,%s\n", customerName, customerId, email)
			`,
			expected: "Alice,42,alice@example.com\n",
		},
		{
			sqlFile: "../tsql_structured/05_json_modify.sql",
			testCode: `
				result := r.UpdateCustomerJson(ctx, ` + "`" + `{"customer":{"name":"Alice","email":"old@example.com"},"status":"inactive"}` + "`" + `, "alice@new.com", "555-1234", "active")
				// Check result contains new email and phone
				if strings.Contains(result, "alice@new.com") && strings.Contains(result, "555-1234") {
					fmt.Println("OK")
				} else {
					fmt.Printf("FAIL: %s\n", result)
				}
			`,
			expected: "OK\n",
		},
	}

	for _, tc := range tests {
		name := filepath.Base(tc.sqlFile)
		t.Run(name, func(t *testing.T) {
			result := transpileAndExecuteStructured(t, workspace, tgpiler, tc.sqlFile, tc.testCode)
			if result != tc.expected {
				t.Errorf("Expected %q, got %q", tc.expected, result)
			}
		})
	}
}

// TestE2EExecuteXML tests XML function execution
func TestE2EExecuteXML(t *testing.T) {
	workspace := setupStructuredWorkspace(t)
	defer os.RemoveAll(workspace)

	tgpiler := findTgpiler(t)

	tests := []struct {
		sqlFile  string
		testCode string
		expected string
	}{
		{
			sqlFile: "../tsql_structured/11_xml_value_extract.sql",
			testCode: `
				customerName, customerId, email := r.ParseCustomerXml(ctx, "<customer><id>123</id><name>Bob</name><email>bob@test.com</email></customer>")
				fmt.Printf("%s,%d,%s\n", customerName, customerId, email)
			`,
			expected: "Bob,123,bob@test.com\n",
		},
		{
			sqlFile: "../tsql_structured/13_xml_exist.sql",
			testCode: `
				isValid, hasCustomer, hasItems, hasShipping, msg := r.ValidateOrderXml(ctx, "<order status=\"pending\"><customer>John</customer><items><item>Widget</item></items><shipping>Express</shipping></order>")
				_ = msg
				fmt.Printf("%v,%v,%v,%v\n", isValid, hasCustomer, hasItems, hasShipping)
			`,
			expected: "true,true,true,true\n",
		},
	}

	for _, tc := range tests {
		name := filepath.Base(tc.sqlFile)
		t.Run(name, func(t *testing.T) {
			result := transpileAndExecuteStructured(t, workspace, tgpiler, tc.sqlFile, tc.testCode)
			if result != tc.expected {
				t.Errorf("Expected %q, got %q", tc.expected, result)
			}
		})
	}
}

// setupStructuredWorkspace creates a workspace for structured data tests
func setupStructuredWorkspace(t *testing.T) string {
	workspace, err := os.MkdirTemp("", "tgpiler-structured-exec-*")
	if err != nil {
		t.Fatalf("Failed to create workspace: %v", err)
	}

	// Create go.mod
	goMod := `module teststructured

go 1.22.2

require (
	github.com/ha1tch/tgpiler v0.0.0
	github.com/shopspring/decimal v1.3.1
)

replace github.com/ha1tch/tgpiler => ` + mustAbs(t, "..") + `
`
	if err := os.WriteFile(filepath.Join(workspace, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	return workspace
}

// transpileAndExecuteStructured transpiles with --dml, compiles, and runs
func transpileAndExecuteStructured(t *testing.T, workspace, tgpiler, sqlFile, testCode string) string {
	// Transpile with --dml
	cmd := exec.Command(tgpiler, "--dml", sqlFile)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Transpilation failed: %v\nOutput: %s", err, string(output))
	}

	generatedCode := string(output)

	// Detect required imports
	usesDecimal := strings.Contains(generatedCode, "decimal.")
	usesTime := strings.Contains(generatedCode, "time.")
	usesStrings := strings.Contains(generatedCode, "strings.")
	usesStrconv := strings.Contains(generatedCode, "strconv.")

	// Build imports
	var imports []string
	imports = append(imports, `"context"`) // Always needed for DML functions
	imports = append(imports, `"fmt"`)
	imports = append(imports, `"strings"`) // Always needed for Contains check in tests
	if usesDecimal {
		imports = append(imports, `"github.com/shopspring/decimal"`)
	}
	if usesTime {
		imports = append(imports, `"time"`)
	}
	if usesStrings && !contains(imports, `"strings"`) {
		imports = append(imports, `"strings"`)
	}
	if usesStrconv {
		imports = append(imports, `"strconv"`)
	}

	// Extract function definitions
	lines := strings.Split(generatedCode, "\n")
	var funcLines []string
	inImports := false
	for _, line := range lines {
		if strings.HasPrefix(line, "package ") {
			continue
		}
		if strings.HasPrefix(line, "import (") {
			inImports = true
			continue
		}
		if inImports {
			if line == ")" {
				inImports = false
			}
			continue
		}
		if strings.HasPrefix(line, "import ") {
			continue
		}
		funcLines = append(funcLines, line)
	}

	// Create main.go with test harness and runtime stubs
	mainCode := fmt.Sprintf(`package main

import (
	%s
)

// Runtime stubs for JSON/XML functions
%s

%s

func main() {
	%s
}
`, strings.Join(imports, "\n\t"), generateRuntimeStubs(), strings.Join(funcLines, "\n"), testCode)

	mainPath := filepath.Join(workspace, "main.go")
	if err := os.WriteFile(mainPath, []byte(mainCode), 0644); err != nil {
		t.Fatalf("Failed to write main.go: %v", err)
	}

	// Run go mod tidy
	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = workspace
	tidyCmd.Env = append(os.Environ(), "GONOSUMDB=*", "GOPRIVATE=*")
	tidyCmd.CombinedOutput() // Ignore errors, build will catch issues

	// Build
	buildCmd := exec.Command("go", "build", "-o", "test_binary", "main.go")
	buildCmd.Dir = workspace
	buildCmd.Env = append(os.Environ(), "GONOSUMDB=*", "GOPRIVATE=*")
	if buildOutput, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Build failed:\n%s\nGenerated code:\n%s", string(buildOutput), mainCode)
	}

	// Execute
	runCmd := exec.Command(filepath.Join(workspace, "test_binary"))
	runCmd.Dir = workspace
	runOutput, err := runCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Execution failed: %v\nOutput: %s", err, string(runOutput))
	}

	return string(runOutput)
}

// generateRuntimeStubs generates minimal runtime stubs for JSON/XML execution
func generateRuntimeStubs() string {
	return `
import "encoding/json"
import "regexp"

// Repository is the database wrapper used by transpiled code
type Repository struct{}

var ctx = context.Background()
var r = &Repository{}

// JsonValue extracts a scalar value from JSON using a path
func JsonValue(jsonStr, path string) string {
	var data interface{}
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return ""
	}
	// Simple path parser for $.a.b.c format
	parts := strings.Split(strings.TrimPrefix(path, "$."), ".")
	current := data
	for _, part := range parts {
		if m, ok := current.(map[string]interface{}); ok {
			current = m[part]
		} else {
			return ""
		}
	}
	switch v := current.(type) {
	case string:
		return v
	case float64:
		if v == float64(int(v)) {
			return fmt.Sprintf("%d", int(v))
		}
		return fmt.Sprintf("%g", v)
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}

// JsonQuery extracts a JSON fragment
func JsonQuery(jsonStr, path string) string {
	var data interface{}
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return ""
	}
	parts := strings.Split(strings.TrimPrefix(path, "$."), ".")
	current := data
	for _, part := range parts {
		if m, ok := current.(map[string]interface{}); ok {
			current = m[part]
		} else {
			return ""
		}
	}
	result, _ := json.Marshal(current)
	return string(result)
}

// JsonModify modifies a JSON value
func JsonModify(jsonStr, path string, newValue interface{}) string {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return jsonStr
	}
	parts := strings.Split(strings.TrimPrefix(path, "$."), ".")
	setNested(data, parts, newValue)
	result, _ := json.Marshal(data)
	return string(result)
}

func setNested(data map[string]interface{}, path []string, value interface{}) {
	if len(path) == 1 {
		data[path[0]] = value
		return
	}
	if next, ok := data[path[0]].(map[string]interface{}); ok {
		setNested(next, path[1:], value)
	}
}

// Isjson checks if a string is valid JSON
func Isjson(s string) int {
	var js interface{}
	if json.Unmarshal([]byte(s), &js) == nil {
		return 1
	}
	return 0
}

// XmlValueString extracts a string value from XML
func XmlValueString(xml, xpath string) string {
	// Simple regex-based extraction for testing
	// Extract element name from xpath like "(/customer/name)[1]"
	re := regexp.MustCompile(` + "`" + `/([^/\[\]@()]+)` + "`" + `)
	matches := re.FindAllStringSubmatch(xpath, -1)
	if len(matches) == 0 {
		return ""
	}
	lastElem := matches[len(matches)-1][1]
	
	// Try to find <element>value</element>
	elemRe := regexp.MustCompile(` + "`" + `<` + "`" + ` + lastElem + ` + "`" + `[^>]*>([^<]*)</` + "`" + ` + lastElem + ` + "`" + `>` + "`" + `)
	if m := elemRe.FindStringSubmatch(xml); len(m) > 1 {
		return m[1]
	}
	return ""
}

// XmlExist checks if an XPath exists in XML
func XmlExist(xml, xpath string) bool {
	// Simple check: see if the element/attribute exists
	if strings.Contains(xpath, "@") {
		// Attribute check
		re := regexp.MustCompile(` + "`" + `\[@?([^=\]]+)` + "`" + `)
		if m := re.FindStringSubmatch(xpath); len(m) > 1 {
			attrRe := regexp.MustCompile(m[1] + ` + "`" + `\s*=` + "`" + `)
			return attrRe.MatchString(xml)
		}
	}
	// Element check
	re := regexp.MustCompile(` + "`" + `/([^/\[\]@]+)(?:\[\d+\])?$` + "`" + `)
	if m := re.FindStringSubmatch(xpath); len(m) > 1 {
		elemRe := regexp.MustCompile(` + "`" + `<` + "`" + ` + m[1] + ` + "`" + `[\s>]` + "`" + `)
		return elemRe.MatchString(xml)
	}
	return false
}

var _ = regexp.MustCompile // Ensure regexp is used
`
}

// generateStructuredStubs generates full stubs for package-level build
func generateStructuredStubs() string {
	return `package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	
	"github.com/ha1tch/tgpiler/tsqlruntime"
	"github.com/shopspring/decimal"
)

var ctx = context.Background()

// Repository is the database wrapper used by transpiled code
type Repository struct {
	db *sql.DB
}

var r = &Repository{}
var tempTables = tsqlruntime.NewTempTableManager()
var rowcount int32

var _ = strings.ToLower
var _ = strconv.Atoi
var _ decimal.Decimal
var _ = regexp.MustCompile
var _ = json.Marshal

func JsonValue(jsonStr, path string) string {
	v, err := tsqlruntime.JSONValue(jsonStr, path)
	if err != nil || v.IsNull {
		return ""
	}
	return v.AsString()
}

func JsonQuery(jsonStr, path string) string {
	v, err := tsqlruntime.JSONQuery(jsonStr, path)
	if err != nil || v.IsNull {
		return ""
	}
	return v.AsString()
}

func JsonModify(jsonStr, path string, value interface{}) string {
	v, err := tsqlruntime.JSONModify(jsonStr, path, value)
	if err != nil || v.IsNull {
		return jsonStr
	}
	return v.AsString()
}

func Isjson(s string) int {
	v, err := tsqlruntime.IsJSON(s)
	if err != nil || v.IsNull {
		return 0
	}
	return int(v.AsInt())
}

func XmlValueString(xml, xpath string) string {
	v, err := tsqlruntime.XMLValue(xml, xpath, tsqlruntime.TypeNVarChar)
	if err != nil || v.IsNull {
		return ""
	}
	return v.AsString()
}

func XmlExist(xml, xpath string) bool {
	v, err := tsqlruntime.XMLExist(xml, xpath)
	if err != nil || v.IsNull {
		return false
	}
	return v.AsInt() == 1
}

func XmlQuery(xml, xpath string) string {
	v, err := tsqlruntime.XMLQuery(xml, xpath)
	if err != nil || v.IsNull {
		return ""
	}
	return v.AsString()
}

func XmlNodes(xml, xpath string) []map[string]interface{} {
	return nil
}

func XmlModify(xml, dml string) string {
	return xml
}

func XmlPreparedocument(xml string) int32 {
	return 1
}

func XmlRemovedocument(handle int32) {
}
`
}

func mustAbs(t *testing.T, path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatalf("Failed to get absolute path for %s: %v", path, err)
	}
	return abs
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
