package tests

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ha1tch/tgpiler/transpiler"
)

// TestCTETranspilation tests that all CTE sample files transpile correctly
func TestCTETranspilation(t *testing.T) {
	files, err := filepath.Glob("../tsql_cte/*.sql")
	if err != nil {
		t.Fatalf("Failed to glob CTE files: %v", err)
	}

	if len(files) == 0 {
		t.Skip("No CTE sample files found")
	}

	t.Logf("Found %d CTE SQL files in ../tsql_cte", len(files))

	for _, file := range files {
		name := filepath.Base(file)
		t.Run(name, func(t *testing.T) {
			source, err := os.ReadFile(file)
			if err != nil {
				t.Fatalf("Failed to read %s: %v", file, err)
			}

			config := transpiler.DefaultDMLConfig()
			config.SQLDialect = "postgres"
			
			result, err := transpiler.TranspileWithDML(string(source), "main", config)
			if err != nil {
				t.Fatalf("Transpilation failed for %s: %v", name, err)
			}

			// Basic validation
			if result == "" {
				t.Error("Transpilation produced empty result")
			}
			if !strings.Contains(result, "func ") {
				t.Error("Expected generated code to contain a function")
			}
		})
	}
}

// TestCTESimple tests a simple CTE with SELECT
func TestCTESimple(t *testing.T) {
	sql := `
CREATE PROCEDURE GetTopCustomers
    @MinSales DECIMAL(18,2)
AS
BEGIN
    WITH SalesCTE AS (
        SELECT CustomerID, SUM(Amount) AS TotalSales
        FROM Orders
        GROUP BY CustomerID
    )
    SELECT c.Name, s.TotalSales
    FROM Customers c
    JOIN SalesCTE s ON c.ID = s.CustomerID
    WHERE s.TotalSales > @MinSales
    ORDER BY s.TotalSales DESC
END
`
	config := transpiler.DefaultDMLConfig()
	config.SQLDialect = "postgres"

	result, err := transpiler.TranspileWithDML(sql, "main", config)
	if err != nil {
		t.Fatalf("Transpilation failed: %v", err)
	}

	t.Logf("Generated code:\n%s", result)

	// Verify CTE is in the query
	if !strings.Contains(result, "WITH SalesCTE AS") {
		t.Error("Expected CTE in generated query")
	}

	// Verify parameter placeholder
	if !strings.Contains(result, "$1") {
		t.Error("Expected PostgreSQL parameter placeholder $1")
	}

	// Verify variable is used
	if !strings.Contains(result, "minSales") {
		t.Error("Expected minSales variable in generated code")
	}
}

// TestCTEMultiple tests multiple CTEs in a single statement
func TestCTEMultiple(t *testing.T) {
	sql := `
CREATE PROCEDURE GetMetrics
AS
BEGIN
    WITH 
        CTE1 AS (SELECT ID FROM Table1),
        CTE2 AS (SELECT ID FROM Table2)
    SELECT * FROM CTE1 JOIN CTE2 ON CTE1.ID = CTE2.ID
END
`
	config := transpiler.DefaultDMLConfig()
	config.SQLDialect = "postgres"

	result, err := transpiler.TranspileWithDML(sql, "main", config)
	if err != nil {
		t.Fatalf("Transpilation failed: %v", err)
	}

	t.Logf("Generated code:\n%s", result)

	// Verify both CTEs are in the query
	if !strings.Contains(result, "CTE1 AS") || !strings.Contains(result, "CTE2 AS") {
		t.Error("Expected both CTEs in generated query")
	}

	// Verify comment mentions both CTEs
	if !strings.Contains(result, "CTE1, CTE2") {
		t.Error("Expected comment to mention both CTEs")
	}
}

// TestCTERecursive tests a recursive CTE
func TestCTERecursive(t *testing.T) {
	sql := `
CREATE PROCEDURE GetHierarchy
    @RootID INT
AS
BEGIN
    WITH Hierarchy AS (
        SELECT ID, ParentID, Name, 1 AS Level
        FROM Items
        WHERE ID = @RootID
        
        UNION ALL
        
        SELECT i.ID, i.ParentID, i.Name, h.Level + 1
        FROM Items i
        INNER JOIN Hierarchy h ON i.ParentID = h.ID
    )
    SELECT * FROM Hierarchy ORDER BY Level
END
`
	config := transpiler.DefaultDMLConfig()
	config.SQLDialect = "postgres"

	result, err := transpiler.TranspileWithDML(sql, "main", config)
	if err != nil {
		t.Fatalf("Transpilation failed: %v", err)
	}

	t.Logf("Generated code:\n%s", result)

	// Verify recursive CTE structure
	if !strings.Contains(result, "UNION ALL") {
		t.Error("Expected UNION ALL in recursive CTE")
	}

	// Verify parameter placeholder for @RootID
	if !strings.Contains(result, "$1") {
		t.Error("Expected PostgreSQL parameter placeholder $1")
	}
}

// TestCTEWithInsert tests a CTE followed by INSERT
func TestCTEWithInsert(t *testing.T) {
	sql := `
CREATE PROCEDURE ArchiveData
    @CutoffDate DATE
AS
BEGIN
    WITH OldData AS (
        SELECT ID, Data FROM Records WHERE CreatedAt < @CutoffDate
    )
    INSERT INTO Archive (ID, Data, ArchivedAt)
    SELECT ID, Data, GETDATE() FROM OldData
END
`
	config := transpiler.DefaultDMLConfig()
	config.SQLDialect = "postgres"

	result, err := transpiler.TranspileWithDML(sql, "main", config)
	if err != nil {
		t.Fatalf("Transpilation failed: %v", err)
	}

	t.Logf("Generated code:\n%s", result)

	// Verify it uses ExecContext for INSERT
	if !strings.Contains(result, "ExecContext") {
		t.Error("Expected ExecContext for CTE INSERT")
	}

	// Verify CTE is in the query
	if !strings.Contains(result, "WITH OldData AS") {
		t.Error("Expected CTE in generated query")
	}

	// Verify INSERT is in the query
	if !strings.Contains(result, "INSERT INTO Archive") {
		t.Error("Expected INSERT in generated query")
	}
}

// TestCTEWithUpdate tests a CTE followed by UPDATE
func TestCTEWithUpdate(t *testing.T) {
	sql := `
CREATE PROCEDURE UpdateTiers
AS
BEGIN
    WITH TierCalc AS (
        SELECT CustomerID, SUM(Amount) AS Total FROM Orders GROUP BY CustomerID
    )
    UPDATE c
    SET c.Tier = CASE WHEN t.Total > 1000 THEN 'Gold' ELSE 'Silver' END
    FROM Customers c
    INNER JOIN TierCalc t ON c.ID = t.CustomerID
END
`
	config := transpiler.DefaultDMLConfig()
	config.SQLDialect = "postgres"

	result, err := transpiler.TranspileWithDML(sql, "main", config)
	if err != nil {
		t.Fatalf("Transpilation failed: %v", err)
	}

	t.Logf("Generated code:\n%s", result)

	// Verify it uses ExecContext for UPDATE
	if !strings.Contains(result, "ExecContext") {
		t.Error("Expected ExecContext for CTE UPDATE")
	}

	// Verify comment indicates CTE UPDATE
	if !strings.Contains(result, "CTE UPDATE") {
		t.Error("Expected 'CTE UPDATE' comment")
	}
}

// TestCTEWithDelete tests a CTE followed by DELETE
func TestCTEWithDelete(t *testing.T) {
	sql := `
CREATE PROCEDURE RemoveDuplicates
AS
BEGIN
    WITH Duplicates AS (
        SELECT ID, ROW_NUMBER() OVER (PARTITION BY Email ORDER BY ID) AS RN
        FROM Users
    )
    DELETE FROM Duplicates WHERE RN > 1
END
`
	config := transpiler.DefaultDMLConfig()
	config.SQLDialect = "postgres"

	result, err := transpiler.TranspileWithDML(sql, "main", config)
	if err != nil {
		t.Fatalf("Transpilation failed: %v", err)
	}

	t.Logf("Generated code:\n%s", result)

	// Verify it uses ExecContext for DELETE
	if !strings.Contains(result, "ExecContext") {
		t.Error("Expected ExecContext for CTE DELETE")
	}

	// Verify comment indicates CTE DELETE
	if !strings.Contains(result, "CTE DELETE") {
		t.Error("Expected 'CTE DELETE' comment")
	}
}

// TestWindowFunctions tests window functions without CTEs
func TestWindowFunctions(t *testing.T) {
	sql := `
CREATE PROCEDURE GetRankings
    @CategoryID INT
AS
BEGIN
    SELECT 
        ProductID,
        Price,
        ROW_NUMBER() OVER (ORDER BY Price DESC) AS RowNum,
        RANK() OVER (ORDER BY Price DESC) AS PriceRank,
        SUM(Price) OVER (ORDER BY Price ROWS UNBOUNDED PRECEDING) AS RunningTotal
    FROM Products
    WHERE CategoryID = @CategoryID
END
`
	config := transpiler.DefaultDMLConfig()
	config.SQLDialect = "postgres"

	result, err := transpiler.TranspileWithDML(sql, "main", config)
	if err != nil {
		t.Fatalf("Transpilation failed: %v", err)
	}

	t.Logf("Generated code:\n%s", result)

	// Verify window functions are in the query
	if !strings.Contains(result, "ROW_NUMBER()") {
		t.Error("Expected ROW_NUMBER() in generated query")
	}
	if !strings.Contains(result, "RANK()") {
		t.Error("Expected RANK() in generated query")
	}
	if !strings.Contains(result, "OVER") {
		t.Error("Expected OVER clause in generated query")
	}
	if !strings.Contains(result, "ROWS UNBOUNDED PRECEDING") {
		t.Error("Expected window frame in generated query")
	}
}

// TestWindowFunctionTypeInference tests that window functions get correct types
func TestWindowFunctionTypeInference(t *testing.T) {
	tests := []struct {
		name     string
		sql      string
		expected map[string]string // variable name -> expected type
	}{
		{
			name: "Ranking functions",
			sql: `
				CREATE PROCEDURE Test
				AS
				BEGIN
					SELECT 
						ROW_NUMBER() OVER (ORDER BY ID) AS RowNum,
						RANK() OVER (ORDER BY ID) AS Rank,
						DENSE_RANK() OVER (ORDER BY ID) AS DenseRank,
						NTILE(4) OVER (ORDER BY ID) AS Quartile
					FROM Items
				END
			`,
			expected: map[string]string{
				"rowNum":    "int64",
				"rank":      "int64",
				"denseRank": "int64",
				"quartile":  "int64",
			},
		},
		{
			name: "Percentage functions",
			sql: `
				CREATE PROCEDURE Test
				AS
				BEGIN
					SELECT 
						PERCENT_RANK() OVER (ORDER BY Value) AS PercentRank,
						CUME_DIST() OVER (ORDER BY Value) AS CumeDist
					FROM Items
				END
			`,
			expected: map[string]string{
				"percentRank": "float64",
				"cumeDist":    "float64",
			},
		},
		{
			name: "Aggregate window functions",
			sql: `
				CREATE PROCEDURE Test
				AS
				BEGIN
					SELECT 
						COUNT(*) OVER () AS TotalCount,
						SUM(Amount) OVER (ORDER BY ID) AS RunningSum
					FROM Items
				END
			`,
			expected: map[string]string{
				"totalCount": "int64",
				"runningSum": "decimal.Decimal",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := transpiler.DefaultDMLConfig()
			config.SQLDialect = "postgres"

			result, err := transpiler.TranspileWithDML(tt.sql, "main", config)
			if err != nil {
				t.Fatalf("Transpilation failed: %v", err)
			}

			t.Logf("Generated code:\n%s", result)

			// Check expected types
			for varName, expectedType := range tt.expected {
				pattern := fmt.Sprintf("var %s %s", varName, expectedType)
				if !strings.Contains(result, pattern) {
					t.Errorf("Expected %q to be declared as %s", varName, expectedType)
				}
			}
		})
	}
}

// TestNavigationFunctionTypeInference tests that LEAD/LAG/FIRST_VALUE/LAST_VALUE inherit types
func TestNavigationFunctionTypeInference(t *testing.T) {
	tests := []struct {
		name     string
		sql      string
		expected map[string]string
	}{
		{
			name: "LEAD/LAG with decimal",
			sql: `
				CREATE PROCEDURE Test
				AS
				BEGIN
					SELECT 
						Amount,
						LAG(Amount, 1) OVER (ORDER BY ID) AS PrevAmount,
						LEAD(Amount, 1) OVER (ORDER BY ID) AS NextAmount
					FROM Orders
				END
			`,
			expected: map[string]string{
				"prevAmount": "decimal.Decimal",
				"nextAmount": "decimal.Decimal",
			},
		},
		{
			name: "FIRST_VALUE/LAST_VALUE with time",
			sql: `
				CREATE PROCEDURE Test
				AS
				BEGIN
					SELECT 
						OrderDate,
						FIRST_VALUE(OrderDate) OVER (ORDER BY OrderDate) AS FirstDate,
						LAST_VALUE(OrderDate) OVER (ORDER BY OrderDate) AS LastDate
					FROM Orders
				END
			`,
			expected: map[string]string{
				"firstDate": "time.Time",
				"lastDate":  "time.Time",
			},
		},
		{
			name: "MIN/MAX window preserve type",
			sql: `
				CREATE PROCEDURE Test
				AS
				BEGIN
					SELECT 
						Price,
						MIN(Price) OVER () AS MinPrice,
						MAX(Price) OVER () AS MaxPrice
					FROM Products
				END
			`,
			expected: map[string]string{
				"minPrice": "decimal.Decimal",
				"maxPrice": "decimal.Decimal",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := transpiler.DefaultDMLConfig()
			config.SQLDialect = "postgres"

			result, err := transpiler.TranspileWithDML(tt.sql, "main", config)
			if err != nil {
				t.Fatalf("Transpilation failed: %v", err)
			}

			t.Logf("Generated code:\n%s", result)

			for varName, expectedType := range tt.expected {
				pattern := fmt.Sprintf("var %s %s", varName, expectedType)
				if !strings.Contains(result, pattern) {
					t.Errorf("Expected %q to be declared as %s", varName, expectedType)
				}
			}
		})
	}
}
