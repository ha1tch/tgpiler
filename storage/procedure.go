package storage

import (
	"regexp"
	"strings"
)

// Procedure represents a parsed stored procedure.
type Procedure struct {
	Name       string
	Parameters []ProcParameter
	Operations []Operation      // DML operations inside the procedure
	ResultSets []ResultSet      // Expected result sets from SELECT statements
	RawSQL     string           // Original SQL for reference
}

// ProcParameter represents a stored procedure parameter.
type ProcParameter struct {
	Name         string // Without @ prefix
	SQLType      string // BIGINT, NVARCHAR(255), etc.
	GoType       string // Mapped Go type
	IsOutput     bool   // OUTPUT parameter
	HasDefault   bool   // Has default value
	DefaultValue string // Default value if any
	Position     int    // Parameter order (0-indexed)
}

// ResultSet represents columns returned by a SELECT statement.
type ResultSet struct {
	Columns   []ResultColumn
	FromTable string // Primary table if identifiable
}

// ResultColumn represents a column in a result set.
type ResultColumn struct {
	Name    string // Column name or alias
	SQLType string // If determinable
	GoType  string // Mapped Go type
	Source  string // Table.Column if identifiable
}

// ProcedureExtractor extracts procedure metadata from T-SQL.
type ProcedureExtractor struct {
	// SQL type to Go type mapping
	typeMap map[string]string
}

// NewProcedureExtractor creates a new extractor.
func NewProcedureExtractor() *ProcedureExtractor {
	return &ProcedureExtractor{
		typeMap: map[string]string{
			"bigint":         "int64",
			"int":            "int32",
			"smallint":       "int16",
			"tinyint":        "int8",
			"bit":            "bool",
			"decimal":        "float64",
			"numeric":        "float64",
			"money":          "float64",
			"smallmoney":     "float64",
			"float":          "float64",
			"real":           "float32",
			"datetime":       "time.Time",
			"datetime2":      "time.Time",
			"date":           "time.Time",
			"time":           "time.Time",
			"datetimeoffset": "time.Time",
			"char":           "string",
			"varchar":        "string",
			"nchar":          "string",
			"nvarchar":       "string",
			"text":           "string",
			"ntext":          "string",
			"binary":         "[]byte",
			"varbinary":      "[]byte",
			"image":          "[]byte",
			"uniqueidentifier": "string",
			"xml":            "string",
		},
	}
}

// ExtractProcedure parses a CREATE PROCEDURE statement.
func (e *ProcedureExtractor) ExtractProcedure(sql string) (*Procedure, error) {
	proc := &Procedure{
		RawSQL: sql,
	}

	// Extract procedure name
	proc.Name = e.extractProcName(sql)
	if proc.Name == "" {
		return nil, nil // Not a procedure
	}

	// Extract parameters
	proc.Parameters = e.extractParameters(sql)

	// Extract operations (reuse existing detector logic)
	detector := NewSQLDetector(DetectorConfig{})
	ops, _ := detector.DetectFromSQL(sql)
	proc.Operations = ops

	// Extract result sets from SELECT statements
	proc.ResultSets = e.extractResultSets(sql)

	return proc, nil
}

// ExtractAll extracts all procedures from SQL content.
func (e *ProcedureExtractor) ExtractAll(sql string) ([]*Procedure, error) {
	var procedures []*Procedure

	// Split on CREATE PROCEDURE (case insensitive)
	re := regexp.MustCompile(`(?i)CREATE\s+PROCEDURE`)
	parts := re.Split(sql, -1)

	for i, part := range parts {
		if i == 0 && !re.MatchString(sql[:min(len(sql), 50)]) {
			continue // Skip content before first procedure
		}
		if strings.TrimSpace(part) == "" {
			continue
		}

		// Reconstruct the CREATE PROCEDURE statement
		fullSQL := "CREATE PROCEDURE" + part

		proc, err := e.ExtractProcedure(fullSQL)
		if err != nil {
			continue
		}
		if proc != nil {
			procedures = append(procedures, proc)
		}
	}

	return procedures, nil
}

func (e *ProcedureExtractor) extractProcName(sql string) string {
	// Match: CREATE PROCEDURE [dbo.]usp_Name or CREATE PROCEDURE usp_Name
	re := regexp.MustCompile(`(?i)CREATE\s+PROCEDURE\s+(?:\[?dbo\]?\.)?\[?(\w+)\]?`)
	matches := re.FindStringSubmatch(sql)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

func (e *ProcedureExtractor) extractParameters(sql string) []ProcParameter {
	var params []ProcParameter

	// Find the parameter section between procedure name and AS
	// This handles both inline and multi-line parameter declarations
	
	// First, find where AS BEGIN or AS\n starts
	reAS := regexp.MustCompile(`(?i)\bAS\s*\n|\bAS\s+BEGIN`)
	asLoc := reAS.FindStringIndex(sql)
	if asLoc == nil {
		return params
	}
	
	// Find procedure name end
	reProcName := regexp.MustCompile(`(?i)CREATE\s+PROCEDURE\s+(?:\[?dbo\]?\.)?\[?(\w+)\]?`)
	procMatch := reProcName.FindStringIndex(sql)
	if procMatch == nil {
		return params
	}
	
	// Parameter block is between procedure name and AS
	paramBlock := sql[procMatch[1]:asLoc[0]]
	
	// Match individual parameters
	// @Name TYPE[(size)] [= default] [OUTPUT]
	reParam := regexp.MustCompile(`(?i)@(\w+)\s+(\w+(?:\s*\([^)]+\))?)\s*(?:=\s*([^,\n@]+))?\s*(OUTPUT)?`)
	matches := reParam.FindAllStringSubmatch(paramBlock, -1)

	for i, match := range matches {
		if len(match) < 3 {
			continue
		}

		name := match[1]
		sqlType := strings.TrimSpace(match[2])
		hasDefault := len(match) > 3 && strings.TrimSpace(match[3]) != ""
		isOutput := len(match) > 4 && strings.TrimSpace(match[4]) != ""

		defaultVal := ""
		if hasDefault && len(match) > 3 {
			defaultVal = strings.TrimSpace(match[3])
		}

		param := ProcParameter{
			Name:         name,
			SQLType:      sqlType,
			GoType:       e.sqlTypeToGo(sqlType),
			IsOutput:     isOutput,
			HasDefault:   hasDefault,
			DefaultValue: defaultVal,
			Position:     i,
		}
		params = append(params, param)
	}

	return params
}

func (e *ProcedureExtractor) extractResultSets(sql string) []ResultSet {
	var results []ResultSet

	// Pattern 1: SELECT ... FROM table (with FROM clause)
	// Skip EXISTS/NOT EXISTS subqueries
	reSelectFrom := regexp.MustCompile(`(?is)SELECT\s+(.*?)\s+FROM\s+(\w+)`)
	
	// Find all EXISTS positions to skip
	reExists := regexp.MustCompile(`(?is)EXISTS\s*\(\s*SELECT`)
	existsMatches := reExists.FindAllStringIndex(sql, -1)
	isInExists := func(pos int) bool {
		for _, em := range existsMatches {
			// Check if pos is within 200 chars after EXISTS (
			if pos > em[0] && pos < em[1]+200 {
				return true
			}
		}
		return false
	}

	matches := reSelectFrom.FindAllStringSubmatchIndex(sql, -1)
	for _, matchIdx := range matches {
		if len(matchIdx) < 6 {
			continue
		}
		
		// Skip if this SELECT is inside an EXISTS clause
		if isInExists(matchIdx[0]) {
			continue
		}

		columnList := sql[matchIdx[2]:matchIdx[3]]
		fromTable := sql[matchIdx[4]:matchIdx[5]]

		// Skip if it's a subquery marker or variable assignment
		if strings.Contains(strings.ToUpper(columnList), "INTO") {
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(columnList), "@") {
			continue
		}
		// Skip simple "1" columns (typically from EXISTS checks)
		if strings.TrimSpace(columnList) == "1" {
			continue
		}

		rs := ResultSet{
			FromTable: fromTable,
			Columns:   e.parseColumnList(columnList),
		}

		if len(rs.Columns) > 0 {
			results = append(results, rs)
		}
	}

	// Pattern 2: SELECT without FROM (e.g., SELECT 1 AS Success, @Id AS Id)
	// These are typically the return values we care about
	reSelectNoFrom := regexp.MustCompile(`(?is)SELECT\s+(\d+\s+AS\s+\w+[^;]*?)\s*;`)
	matchesNoFrom := reSelectNoFrom.FindAllStringSubmatch(sql, -1)

	for _, match := range matchesNoFrom {
		if len(match) < 2 {
			continue
		}

		columnList := strings.TrimSpace(match[1])

		// Skip if this has a FROM clause (already handled above)
		if strings.Contains(strings.ToUpper(columnList), " FROM ") {
			continue
		}
		// Skip if it's INTO (variable assignment)
		if strings.Contains(strings.ToUpper(columnList), "INTO") {
			continue
		}

		rs := ResultSet{
			FromTable: "", // No FROM clause
			Columns:   e.parseColumnList(columnList),
		}

		if len(rs.Columns) > 0 {
			results = append(results, rs)
		}
	}

	return results
}

func (e *ProcedureExtractor) parseColumnList(columnList string) []ResultColumn {
	var columns []ResultColumn

	// Handle SELECT *
	if strings.TrimSpace(columnList) == "*" {
		return []ResultColumn{{Name: "*"}}
	}

	// Split by comma (simplified - doesn't handle nested functions well)
	parts := strings.Split(columnList, ",")

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		col := ResultColumn{}

		// Check for alias: expr AS alias or expr alias
		reAlias := regexp.MustCompile(`(?i)(.+?)\s+AS\s+(\w+)\s*$`)
		if match := reAlias.FindStringSubmatch(part); len(match) > 2 {
			col.Name = match[2]
			col.Source = strings.TrimSpace(match[1])
		} else {
			// Check for simple alias without AS: expr alias
			words := strings.Fields(part)
			if len(words) >= 2 && !strings.Contains(words[len(words)-1], "(") && !strings.Contains(words[len(words)-1], ".") {
				col.Name = words[len(words)-1]
				col.Source = strings.Join(words[:len(words)-1], " ")
			} else {
				// No alias - use column name
				// Handle Table.Column format
				if strings.Contains(part, ".") {
					parts := strings.Split(part, ".")
					col.Name = parts[len(parts)-1]
					col.Source = part
				} else {
					col.Name = part
				}
			}
		}

		// Clean up column name
		col.Name = strings.Trim(col.Name, "[]")

		columns = append(columns, col)
	}

	return columns
}

func (e *ProcedureExtractor) sqlTypeToGo(sqlType string) string {
	// Normalize: remove size specifiers, lowercase
	normalized := strings.ToLower(sqlType)
	if idx := strings.Index(normalized, "("); idx > 0 {
		normalized = normalized[:idx]
	}
	normalized = strings.TrimSpace(normalized)

	if goType, ok := e.typeMap[normalized]; ok {
		return goType
	}
	return "interface{}" // Unknown type
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
