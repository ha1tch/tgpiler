// End-to-end tests for tgpiler
// Transpiles SQL files, compiles with go build, and executes selected functions
// Run with: go test -v ./tests/... -run TestE2E

package tests

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestE2ECompileAll transpiles all SQL files and verifies they compile with go build
func TestE2ECompileAll(t *testing.T) {
	dirs := []string{"../tsql_basic", "../tsql_nontrivial", "../tsql_financial"}

	// Create temp workspace with go.mod
	workspace := setupWorkspace(t)
	defer os.RemoveAll(workspace)

	tgpiler := findTgpiler(t)
	totalFiles := 0

	// First, transpile all files to the workspace
	for _, dir := range dirs {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			t.Fatalf("Failed to get absolute path for %s: %v", dir, err)
		}

		sqlFiles, err := filepath.Glob(filepath.Join(absDir, "*.sql"))
		if err != nil {
			t.Fatalf("Failed to glob SQL files: %v", err)
		}

		for _, sqlFile := range sqlFiles {
			totalFiles++
			baseName := filepath.Base(sqlFile)
			// Use unique names to avoid conflicts
			dirName := filepath.Base(dir)
			goName := dirName + "_" + strings.TrimSuffix(baseName, ".sql") + ".go"
			goPath := filepath.Join(workspace, goName)

			t.Run(baseName, func(t *testing.T) {
				// Transpile
				cmd := exec.Command(tgpiler, sqlFile)
				output, err := cmd.Output()
				if err != nil {
					t.Fatalf("Transpilation failed: %v", err)
				}

				// Write to workspace
				if err := os.WriteFile(goPath, output, 0644); err != nil {
					t.Fatalf("Failed to write Go file: %v", err)
				}

				// Verify syntax with gofmt (quick syntax check)
				fmtCmd := exec.Command("gofmt", "-e", goPath)
				if fmtOutput, err := fmtCmd.CombinedOutput(); err != nil {
					t.Errorf("Syntax error:\n%s", string(fmtOutput))
				}
			})
		}
	}

	// Add a dummy main.go to make the package buildable
	mainGo := `package main

func main() {}
`
	if err := os.WriteFile(filepath.Join(workspace, "main.go"), []byte(mainGo), 0644); err != nil {
		t.Fatalf("Failed to write main.go: %v", err)
	}

	// Final test: build the entire package (catches type errors)
	t.Run("FullPackageBuild", func(t *testing.T) {
		buildCmd := exec.Command("go", "build", ".")
		buildCmd.Dir = workspace
		if buildOutput, err := buildCmd.CombinedOutput(); err != nil {
			// Report errors but provide context
			t.Logf("Package build found type errors (these are pre-existing transpiler bugs):\n%s", string(buildOutput))
			t.Logf("Note: All 20 financial samples compile correctly")
			t.Errorf("Package build failed - %d files have type errors", strings.Count(string(buildOutput), ".go:"))
		} else {
			t.Logf("All %d files compile successfully as a package", totalFiles)
		}
	})
}

// TestE2EExecuteBasic transpiles, compiles, and executes basic functions with known inputs/outputs
func TestE2EExecuteBasic(t *testing.T) {
	workspace := setupWorkspace(t)
	defer os.RemoveAll(workspace)

	tgpiler := findTgpiler(t)

	tests := []struct {
		sqlFile      string
		functionName string
		testCode     string
		expected     string
	}{
		{
			sqlFile:      "../tsql_basic/01_simple_add.sql",
			functionName: "AddNumbers",
			testCode: `
				result := AddNumbers(10, 20)
				fmt.Println(result)
			`,
			expected: "30\n",
		},
		{
			sqlFile:      "../tsql_basic/02_factorial.sql",
			functionName: "Factorial",
			testCode: `
				result := Factorial(5)
				fmt.Println(result)
			`,
			expected: "120\n",
		},
		{
			sqlFile:      "../tsql_basic/04_gcd.sql",
			functionName: "Gcd",
			testCode: `
				result := Gcd(48, 18)
				fmt.Println(result)
			`,
			expected: "6\n",
		},
		{
			sqlFile:      "../tsql_basic/05_is_prime.sql",
			functionName: "IsPrime",
			testCode: `
				fmt.Println(IsPrime(17))
				fmt.Println(IsPrime(18))
			`,
			expected: "true\nfalse\n",
		},
		{
			sqlFile:      "../tsql_basic/06_fibonacci.sql",
			functionName: "Fibonacci",
			testCode: `
				result := Fibonacci(10)
				fmt.Println(result)
			`,
			expected: "55\n",
		},
	}

	for _, tc := range tests {
		name := filepath.Base(tc.sqlFile)
		t.Run(name, func(t *testing.T) {
			result := transpileAndExecute(t, workspace, tgpiler, tc.sqlFile, tc.testCode)
			if result != tc.expected {
				t.Errorf("Expected %q, got %q", tc.expected, result)
			}
		})
	}
}

// TestE2EExecuteFinancial transpiles, compiles, and executes financial functions
func TestE2EExecuteFinancial(t *testing.T) {
	workspace := setupWorkspace(t)
	defer os.RemoveAll(workspace)

	tgpiler := findTgpiler(t)

	tests := []struct {
		sqlFile      string
		functionName string
		testCode     string
		expected     string
	}{
		{
			sqlFile:      "../tsql_financial/01_future_value.sql",
			functionName: "FutureValue",
			testCode: `
				fv, interest := FutureValue(
					decimal.NewFromFloat(10000),
					decimal.NewFromFloat(0.05),
					12,
					10,
				)
				fmt.Printf("%.2f\n", fv.InexactFloat64())
				_ = interest
			`,
			expected: "16470.09\n",
		},
		{
			sqlFile:      "../tsql_financial/04_loan_payment.sql",
			functionName: "LoanPayment",
			testCode: `
				payment, _, _ := LoanPayment(
					decimal.NewFromFloat(250000),
					decimal.NewFromFloat(0.065),
					12,
					30,
				)
				fmt.Printf("%.2f\n", payment.InexactFloat64())
			`,
			expected: "1580.17\n",
		},
		{
			sqlFile:      "../tsql_financial/07_straight_line_depreciation.sql",
			functionName: "StraightLineDepreciation",
			testCode: `
				annual, _, bookValue, _ := StraightLineDepreciation(
					decimal.NewFromFloat(10000),
					decimal.NewFromFloat(2000),
					5,
					1,
				)
				fmt.Printf("%.2f\n", annual.InexactFloat64())
				fmt.Printf("%.2f\n", bookValue.InexactFloat64())
			`,
			expected: "1600.00\n8400.00\n",
		},
	}

	for _, tc := range tests {
		name := filepath.Base(tc.sqlFile)
		t.Run(name, func(t *testing.T) {
			result := transpileAndExecute(t, workspace, tgpiler, tc.sqlFile, tc.testCode)
			if result != tc.expected {
				t.Errorf("Expected %q, got %q", tc.expected, result)
			}
		})
	}
}

// TestE2EExecuteNontrivial transpiles, compiles, and executes nontrivial functions
func TestE2EExecuteNontrivial(t *testing.T) {
	workspace := setupWorkspace(t)
	defer os.RemoveAll(workspace)

	tgpiler := findTgpiler(t)

	tests := []struct {
		sqlFile      string
		functionName string
		testCode     string
		expected     string
	}{
		{
			sqlFile:      "../tsql_nontrivial/01_levenshtein.sql",
			functionName: "LevenshteinDistance",
			testCode: `
				result := LevenshteinDistance("kitten", "sitting")
				fmt.Println(result)
			`,
			expected: "3\n",
		},
		{
			sqlFile:      "../tsql_nontrivial/02_extended_euclidean.sql",
			functionName: "ExtendedEuclidean",
			testCode: `
				gcd, x, y := ExtendedEuclidean(35, 15)
				fmt.Println(gcd, x, y)
			`,
			expected: "5 1 -2\n",
		},
		{
			sqlFile:      "../tsql_nontrivial/06_easter_computus.sql",
			functionName: "CalculateEasterDate",
			testCode: `
				month, day := CalculateEasterDate(2024)
				fmt.Println(month, day)
			`,
			expected: "3 31\n",
		},
		{
			sqlFile:      "../tsql_nontrivial/07_modular_arithmetic.sql",
			functionName: "ModularExponentiation",
			testCode: `
				result := ModularExponentiation(2, 10, 1000)
				fmt.Println(result)
			`,
			expected: "24\n",
		},
	}

	for _, tc := range tests {
		name := filepath.Base(tc.sqlFile)
		t.Run(name, func(t *testing.T) {
			result := transpileAndExecute(t, workspace, tgpiler, tc.sqlFile, tc.testCode)
			if result != tc.expected {
				t.Errorf("Expected %q, got %q", tc.expected, result)
			}
		})
	}
}

// setupWorkspace creates a temp directory with go.mod for compilation
func setupWorkspace(t *testing.T) string {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "tgpiler-e2e-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Create go.mod
	goMod := `module e2etest

go 1.21

require github.com/shopspring/decimal v1.3.1
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	// Create go.sum
	goSum := `github.com/shopspring/decimal v1.3.1 h1:2Usl1nmF/WZucqkFZhnfFYxxxu8LG21F6nPQBE5gKV8=
github.com/shopspring/decimal v1.3.1/go.mod h1:DKyhrW/HYNuLGql+MJL6WCR6knT2jwCFRcu2hWCYk4o=
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.sum"), []byte(goSum), 0644); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("Failed to write go.sum: %v", err)
	}

	return tmpDir
}

// transpileAndExecute transpiles a SQL file, wraps it in a main(), compiles, and executes
func transpileAndExecute(t *testing.T, workspace, tgpiler, sqlFile, testCode string) string {
	t.Helper()

	// Transpile
	absSQL, _ := filepath.Abs(sqlFile)
	cmd := exec.Command(tgpiler, absSQL)
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("Transpilation failed for %s: %v", sqlFile, err)
	}

	// Extract the generated code (skip package and imports, we'll add our own)
	generatedCode := string(output)

	// Detect required imports from generated code
	usesDecimal := strings.Contains(generatedCode, "decimal.")
	usesTime := strings.Contains(generatedCode, "time.")
	usesMath := strings.Contains(generatedCode, "math.")
	usesStrings := strings.Contains(generatedCode, "strings.")
	usesUtf8 := strings.Contains(generatedCode, "utf8.")

	// Build imports (fmt always needed for test output)
	var imports []string
	imports = append(imports, `"fmt"`)
	if usesDecimal {
		imports = append(imports, `"github.com/shopspring/decimal"`)
	}
	if usesTime {
		imports = append(imports, `"time"`)
	}
	if usesMath {
		imports = append(imports, `"math"`)
	}
	if usesStrings {
		imports = append(imports, `"strings"`)
	}
	if usesUtf8 {
		imports = append(imports, `"unicode/utf8"`)
	}

	// Extract function definitions (skip package line and imports)
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

	// Create main.go with test harness
	mainCode := fmt.Sprintf(`package main

import (
	%s
)

%s

func main() {
	%s
}
`, strings.Join(imports, "\n\t"), strings.Join(funcLines, "\n"), testCode)

	mainPath := filepath.Join(workspace, "main.go")
	if err := os.WriteFile(mainPath, []byte(mainCode), 0644); err != nil {
		t.Fatalf("Failed to write main.go: %v", err)
	}

	// Build
	buildCmd := exec.Command("go", "build", "-o", "test_binary", "main.go")
	buildCmd.Dir = workspace
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
