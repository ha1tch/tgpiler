package tsqlruntime

import (
	"strings"
	"testing"

	"github.com/ha1tch/tsqlparser/lexer"
	"github.com/ha1tch/tsqlparser/parser"
	"github.com/ha1tch/tsqlparser/ast"
)

// TestCTEQueryBuilding tests that CTE queries are built correctly
func TestCTEQueryBuilding(t *testing.T) {
	tests := []struct {
		name     string
		sql      string
		vars     map[string]interface{}
		expected []string // substrings that should appear in the query
	}{
		{
			name: "Simple CTE",
			sql: `
				WITH CustomerSales AS (
					SELECT CustomerID, SUM(Amount) AS TotalSales
					FROM Orders
					GROUP BY CustomerID
				)
				SELECT c.Name, cs.TotalSales
				FROM Customers c
				INNER JOIN CustomerSales cs ON c.ID = cs.CustomerID
				WHERE cs.TotalSales >= @MinSales
			`,
			vars: map[string]interface{}{"MinSales": 200},
			expected: []string{
				"WITH CustomerSales AS",
				"SELECT c.Name, cs.TotalSales",
				"INNER JOIN CustomerSales",
			},
		},
		{
			name: "Multiple CTEs",
			sql: `
				WITH 
					CTE1 AS (SELECT ID FROM Table1),
					CTE2 AS (SELECT ID FROM Table2)
				SELECT * FROM CTE1 JOIN CTE2 ON CTE1.ID = CTE2.ID
			`,
			vars: nil,
			expected: []string{
				"WITH",
				"CTE1 AS",
				"CTE2 AS",
				"JOIN CTE2",
			},
		},
		{
			name: "Recursive CTE",
			sql: `
				WITH Hierarchy AS (
					SELECT ID, ParentID, Name, 1 AS Level
					FROM Items
					WHERE ID = @RootID
					
					UNION ALL
					
					SELECT i.ID, i.ParentID, i.Name, h.Level + 1
					FROM Items i
					INNER JOIN Hierarchy h ON i.ParentID = h.ID
				)
				SELECT * FROM Hierarchy
			`,
			vars: map[string]interface{}{"RootID": 1},
			expected: []string{
				"WITH Hierarchy AS",
				"UNION ALL",
				"JOIN Hierarchy",
			},
		},
		{
			name: "CTE with INSERT",
			sql: `
				WITH OldOrders AS (
					SELECT ID, CustomerID, Amount
					FROM Orders
					WHERE Status = 'old'
				)
				INSERT INTO ArchivedOrders (ID, CustomerID, Amount)
				SELECT ID, CustomerID, Amount FROM OldOrders
			`,
			vars: nil,
			expected: []string{
				"WITH OldOrders AS",
				"INSERT INTO ArchivedOrders",
				"SELECT ID, CustomerID, Amount FROM OldOrders",
			},
		},
		{
			name: "CTE with window function",
			sql: `
				WITH RankedSales AS (
					SELECT 
						ProductID,
						Amount,
						ROW_NUMBER() OVER (PARTITION BY ProductID ORDER BY Amount DESC) AS Rank
					FROM Sales
				)
				SELECT ProductID, Amount, Rank
				FROM RankedSales
				WHERE Rank = 1
			`,
			vars: nil,
			expected: []string{
				"WITH RankedSales AS",
				"ROW_NUMBER()",
				"OVER",
				"PARTITION BY ProductID",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse the SQL
			l := lexer.New(tt.sql)
			p := parser.New(l)
			prog := p.ParseProgram()

			if len(p.Errors()) > 0 {
				t.Fatalf("Parse errors: %v", p.Errors())
			}

			if len(prog.Statements) != 1 {
				t.Fatalf("Expected 1 statement, got %d", len(prog.Statements))
			}

			ws, ok := prog.Statements[0].(*ast.WithStatement)
			if !ok {
				t.Fatalf("Expected WithStatement, got %T", prog.Statements[0])
			}

			// Create interpreter with variables
			interp := NewInterpreter(nil, DialectPostgres)
			for name, val := range tt.vars {
				interp.SetVariable(name, val)
			}

			// Build the query
			query, args, err := interp.buildWithQuery(ws)
			if err != nil {
				t.Fatalf("buildWithQuery failed: %v", err)
			}

			t.Logf("Built query: %s", query)
			t.Logf("Args: %v", args)

			// Check expected substrings
			for _, exp := range tt.expected {
				if !strings.Contains(query, exp) {
					t.Errorf("Expected query to contain %q", exp)
				}
			}

			// Check variable substitution
			if len(tt.vars) > 0 {
				if len(args) == 0 {
					t.Error("Expected args to be populated with variable values")
				}
				// Should have placeholders instead of @variable
				if strings.Contains(query, "@MinSales") || strings.Contains(query, "@RootID") {
					t.Error("Variables should be substituted with placeholders")
				}
			}
		})
	}
}

// TestCTEStatementTypeDetection tests that the correct inner statement type is detected
func TestCTEStatementTypeDetection(t *testing.T) {
	tests := []struct {
		name        string
		sql         string
		innerType   string
	}{
		{
			name:      "CTE with SELECT",
			sql:       "WITH CTE AS (SELECT 1) SELECT * FROM CTE",
			innerType: "*ast.SelectStatement",
		},
		{
			name:      "CTE with INSERT",
			sql:       "WITH CTE AS (SELECT 1 AS Val) INSERT INTO T SELECT Val FROM CTE",
			innerType: "*ast.InsertStatement",
		},
		{
			name:      "CTE with UPDATE",
			sql:       "WITH CTE AS (SELECT 1 AS ID) UPDATE T SET X = 1 FROM T INNER JOIN CTE ON T.ID = CTE.ID",
			innerType: "*ast.UpdateStatement",
		},
		{
			name:      "CTE with DELETE",
			sql:       "WITH CTE AS (SELECT 1 AS ID) DELETE FROM T WHERE ID IN (SELECT ID FROM CTE)",
			innerType: "*ast.DeleteStatement",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := lexer.New(tt.sql)
			p := parser.New(l)
			prog := p.ParseProgram()

			if len(p.Errors()) > 0 {
				t.Fatalf("Parse errors: %v", p.Errors())
			}

			ws, ok := prog.Statements[0].(*ast.WithStatement)
			if !ok {
				t.Fatalf("Expected WithStatement, got %T", prog.Statements[0])
			}

			innerTypeName := ""
			switch ws.Query.(type) {
			case *ast.SelectStatement:
				innerTypeName = "*ast.SelectStatement"
			case *ast.InsertStatement:
				innerTypeName = "*ast.InsertStatement"
			case *ast.UpdateStatement:
				innerTypeName = "*ast.UpdateStatement"
			case *ast.DeleteStatement:
				innerTypeName = "*ast.DeleteStatement"
			default:
				innerTypeName = "unknown"
			}

			if innerTypeName != tt.innerType {
				t.Errorf("Expected inner type %s, got %s", tt.innerType, innerTypeName)
			}
		})
	}
}

// TestCTEVariableSubstitution tests that variables are correctly substituted
func TestCTEVariableSubstitution(t *testing.T) {
	sql := `
		WITH FilteredData AS (
			SELECT ID, Name, Amount
			FROM Data
			WHERE Amount >= @MinAmount AND CategoryID = @CategoryID
		)
		SELECT * FROM FilteredData WHERE Name LIKE @Pattern
	`

	l := lexer.New(sql)
	p := parser.New(l)
	prog := p.ParseProgram()

	if len(p.Errors()) > 0 {
		t.Fatalf("Parse errors: %v", p.Errors())
	}

	ws := prog.Statements[0].(*ast.WithStatement)

	// Test with different dialects
	dialects := []struct {
		name        Dialect
		placeholder string
	}{
		{DialectPostgres, "$"},
		{DialectMySQL, "?"},
		{DialectSQLite, "?"},
	}

	for _, d := range dialects {
		t.Run(d.name.String(), func(t *testing.T) {
			interp := NewInterpreter(nil, d.name)
			interp.SetVariable("MinAmount", 100)
			interp.SetVariable("CategoryID", 5)
			interp.SetVariable("Pattern", "%test%")

			query, args, err := interp.buildWithQuery(ws)
			if err != nil {
				t.Fatalf("buildWithQuery failed: %v", err)
			}

			t.Logf("Query: %s", query)
			t.Logf("Args: %v", args)

			// Should have 3 args for 3 variables
			if len(args) != 3 {
				t.Errorf("Expected 3 args, got %d", len(args))
			}

			// Should have placeholders
			if !strings.Contains(query, d.placeholder) {
				t.Errorf("Expected %s placeholders in query", d.placeholder)
			}

			// Should not have @variable names
			if strings.Contains(query, "@MinAmount") ||
				strings.Contains(query, "@CategoryID") ||
				strings.Contains(query, "@Pattern") {
				t.Error("Variables should be replaced with placeholders")
			}
		})
	}
}

// Helper function
func (d Dialect) String() string {
	switch d {
	case DialectPostgres:
		return "postgres"
	case DialectMySQL:
		return "mysql"
	case DialectSQLite:
		return "sqlite"
	case DialectSQLServer:
		return "sqlserver"
	default:
		return "generic"
	}
}

