// Compilation verification tests for tgpiler
// Ensures all SQL samples transpile to syntactically valid Go code
// Run with: go test -v ./tests/... -run TestCompilation

package tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestCompilationBasic verifies all tsql_basic samples transpile to valid Go
func TestCompilationBasic(t *testing.T) {
	testTranspileDirectory(t, "../tsql_basic")
}

// TestCompilationNontrivial verifies all tsql_nontrivial samples transpile to valid Go
func TestCompilationNontrivial(t *testing.T) {
	testTranspileDirectory(t, "../tsql_nontrivial")
}

// TestCompilationFinancial verifies all tsql_financial samples transpile to valid Go
func TestCompilationFinancial(t *testing.T) {
	testTranspileDirectory(t, "../tsql_financial")
}

// testTranspileDirectory transpiles all .sql files in a directory and verifies syntax
func testTranspileDirectory(t *testing.T, dir string) {
	// Find tgpiler binary
	tgpiler := findTgpiler(t)

	// Get absolute path to directory
	absDir, err := filepath.Abs(dir)
	if err != nil {
		t.Fatalf("Failed to get absolute path for %s: %v", dir, err)
	}

	// Find all SQL files
	sqlFiles, err := filepath.Glob(filepath.Join(absDir, "*.sql"))
	if err != nil {
		t.Fatalf("Failed to glob SQL files: %v", err)
	}

	if len(sqlFiles) == 0 {
		t.Fatalf("No SQL files found in %s", absDir)
	}

	t.Logf("Found %d SQL files in %s", len(sqlFiles), dir)

	// Create temp directory for output
	tmpDir, err := os.MkdirTemp("", "tgpiler-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Test each file
	for _, sqlFile := range sqlFiles {
		baseName := filepath.Base(sqlFile)
		goName := strings.TrimSuffix(baseName, ".sql") + ".go"
		goPath := filepath.Join(tmpDir, goName)

		t.Run(baseName, func(t *testing.T) {
			// Transpile
			cmd := exec.Command(tgpiler, sqlFile)
			output, err := cmd.Output()
			if err != nil {
				t.Fatalf("Transpilation failed: %v", err)
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

// findTgpiler locates the tgpiler binary
func findTgpiler(t *testing.T) string {
	// Try relative path first (when running from tests directory)
	candidates := []string{
		"../tgpiler",
		"./tgpiler",
		"tgpiler",
	}

	for _, candidate := range candidates {
		if path, err := filepath.Abs(candidate); err == nil {
			if _, err := os.Stat(path); err == nil {
				return path
			}
		}
	}

	// Try to find in PATH
	if path, err := exec.LookPath("tgpiler"); err == nil {
		return path
	}

	t.Fatal("tgpiler binary not found. Run 'go build -o tgpiler ./cmd/tgpiler' first.")
	return ""
}
