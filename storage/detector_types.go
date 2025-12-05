package storage

// Detector analyzes T-SQL AST to extract data operations.
// The proc parameter is *ast.CreateProcedure from tsqlparser.
type Detector interface {
	// DetectOperations scans a procedure and returns all data operations found.
	// The proc parameter should be *ast.CreateProcedure from tsqlparser.
	DetectOperations(proc interface{}) ([]Operation, error)
	
	// DetectModels infers models from detected operations.
	DetectModels(ops []Operation) ([]Model, error)
	
	// DetectRepositories generates repository specs from operations and models.
	DetectRepositories(ops []Operation, models []Model) ([]Repository, error)
}

// DetectorConfig configures the detector behavior.
type DetectorConfig struct {
	// Include operations that can't be fully parsed (as best-effort)
	IncludePartial bool
	
	// Track raw SQL for each operation (for debugging/reference)
	IncludeRawSQL bool
	
	// Infer optionality from SQL patterns (e.g., LEFT JOIN -> optional)
	InferOptionality bool
}

// DetectionResult contains all information extracted from procedures.
type DetectionResult struct {
	// Source info
	Procedures []ProcedureInfo
	
	// Extracted data
	Operations   []Operation
	Models       []Model
	Repositories []Repository
	
	// Issues encountered during detection
	Warnings []DetectionWarning
	Errors   []DetectionError
}

// ProcedureInfo contains metadata about a parsed procedure.
type ProcedureInfo struct {
	Name       string
	Schema     string
	Parameters []ProcedureParam
	SourceFile string
	SourceLine int
}

// ProcedureParam describes a stored procedure parameter.
type ProcedureParam struct {
	Name        string
	SQLType     string
	GoType      string
	IsOutput    bool
	HasDefault  bool
	DefaultValue string
}

// DetectionWarning is a non-fatal issue during detection.
type DetectionWarning struct {
	Procedure string
	Line      int
	Message   string
	SQL       string // Relevant SQL snippet
}

// DetectionError is a fatal issue for a specific operation.
type DetectionError struct {
	Procedure string
	Line      int
	Message   string
	SQL       string
	Err       error
}

// SelectInfo contains parsed information from a SELECT statement.
type SelectInfo struct {
	// Columns being selected
	Columns []SelectColumn
	
	// Tables involved
	Tables []TableRef
	
	// WHERE clause fields
	WhereFields []WhereField
	
	// Variables being assigned (SELECT @var = col)
	Assignments []VariableAssignment
	
	// Aggregates present
	HasAggregates bool
	
	// DISTINCT keyword
	IsDistinct bool
	
	// TOP clause
	TopCount int
	
	// JOINs
	Joins []JoinInfo
}

// SelectColumn describes a column in a SELECT.
type SelectColumn struct {
	Name       string // Column name or alias
	SourceName string // Original column name (if aliased)
	Table      string // Source table (if known)
	Expression string // Full expression (for computed columns)
	IsComputed bool   // Is this a computed expression?
}

// TableRef describes a table reference in FROM clause.
type TableRef struct {
	Name   string
	Schema string
	Alias  string
}

// WhereField describes a field used in WHERE clause.
type WhereField struct {
	Column   string
	Table    string // Table name or alias
	Operator string // =, <>, <, >, LIKE, IN, IS NULL, etc.
	Variable string // T-SQL variable (e.g., "@CustomerID")
	Literal  string // Literal value (if not variable)
	IsNullCheck bool // IS NULL or IS NOT NULL
}

// VariableAssignment describes a SELECT INTO variable assignment.
type VariableAssignment struct {
	Variable string // T-SQL variable (e.g., "@Name")
	Column   string // Column being assigned
	Table    string // Source table
}

// JoinInfo describes a JOIN in a SELECT.
type JoinInfo struct {
	Type       string // INNER, LEFT, RIGHT, FULL, CROSS
	Table      TableRef
	OnLeft     string // Left side of ON condition
	OnRight    string // Right side of ON condition
	IsOptional bool   // LEFT/RIGHT/FULL joins produce optional results
}

// InsertInfo contains parsed information from an INSERT statement.
type InsertInfo struct {
	Table   TableRef
	Columns []string
	
	// VALUES clause
	Values []InsertValue
	
	// SELECT clause (INSERT ... SELECT)
	SelectSource *SelectInfo
	
	// OUTPUT clause
	OutputColumns []string
	OutputInto    string // Variable or table
}

// InsertValue describes a value in INSERT VALUES clause.
type InsertValue struct {
	Column   string
	Variable string // T-SQL variable
	Literal  string // Literal value
	IsDefault bool  // DEFAULT keyword
}

// UpdateInfo contains parsed information from an UPDATE statement.
type UpdateInfo struct {
	Table TableRef
	
	// SET clause
	Assignments []UpdateAssignment
	
	// WHERE clause
	WhereFields []WhereField
	
	// FROM clause (for UPDATE ... FROM)
	FromTables []TableRef
	Joins      []JoinInfo
	
	// OUTPUT clause
	OutputColumns []string
}

// UpdateAssignment describes a SET assignment in UPDATE.
type UpdateAssignment struct {
	Column   string
	Variable string
	Literal  string
	Expression string // For computed updates
}

// DeleteInfo contains parsed information from a DELETE statement.
type DeleteInfo struct {
	Table TableRef
	
	// WHERE clause
	WhereFields []WhereField
	
	// FROM clause (for DELETE ... FROM with JOINs)
	FromTables []TableRef
	Joins      []JoinInfo
	
	// OUTPUT clause
	OutputColumns []string
}

// ExecInfo contains parsed information from an EXEC/EXECUTE statement.
type ExecInfo struct {
	Procedure   string
	Schema      string
	Parameters  []ExecParam
	ResultVar   string // Variable for EXEC @result = proc
}

// ExecParam describes a parameter in EXEC call.
type ExecParam struct {
	Name     string // Parameter name (if named)
	Position int    // Position (if positional)
	Variable string // Variable being passed
	Literal  string // Literal value
	IsOutput bool   // OUTPUT keyword
}

// Pattern matching helpers for common SQL patterns.

// SelectPattern identifies common SELECT patterns for repository method naming.
type SelectPattern int

const (
	PatternUnknown SelectPattern = iota
	PatternGetByID              // SELECT ... WHERE id = @id
	PatternGetByKey             // SELECT ... WHERE unique_key = @key  
	PatternGetAll               // SELECT ... (no WHERE)
	PatternGetByFK              // SELECT ... WHERE foreign_key = @fk
	PatternSearch               // SELECT ... WHERE name LIKE @pattern
	PatternExists               // SELECT 1 WHERE ... (existence check)
	PatternCount                // SELECT COUNT(*) ...
	PatternGetMany              // SELECT ... WHERE id IN (...)
)

// IdentifySelectPattern analyzes a SELECT to determine its pattern.
func IdentifySelectPattern(info *SelectInfo) SelectPattern {
	// No WHERE -> GetAll
	if len(info.WhereFields) == 0 {
		return PatternGetAll
	}
	
	// COUNT aggregate -> Count
	if info.HasAggregates {
		for _, col := range info.Columns {
			if containsIgnoreCase(col.Expression, "count") {
				return PatternCount
			}
		}
	}
	
	// SELECT 1 -> Exists
	if len(info.Columns) == 1 && info.Columns[0].Expression == "1" {
		return PatternExists
	}
	
	// Single equality on likely PK column -> GetByID
	if len(info.WhereFields) == 1 {
		wf := info.WhereFields[0]
		if wf.Operator == "=" && !wf.IsNullCheck {
			col := normalizeForMatch(wf.Column)
			if containsIgnoreCase(col, "id") || containsIgnoreCase(col, "_id") {
				return PatternGetByID
			}
			// Check for common unique key patterns
			if containsIgnoreCase(col, "code") || containsIgnoreCase(col, "key") ||
			   containsIgnoreCase(col, "name") || containsIgnoreCase(col, "email") {
				return PatternGetByKey
			}
			return PatternGetByFK
		}
		if wf.Operator == "LIKE" {
			return PatternSearch
		}
	}
	
	// IN clause -> GetMany
	for _, wf := range info.WhereFields {
		if wf.Operator == "IN" {
			return PatternGetMany
		}
	}
	
	return PatternUnknown
}

// GenerateMethodName creates a repository method name from a SELECT pattern.
func GenerateMethodName(pattern SelectPattern, info *SelectInfo) string {
	switch pattern {
	case PatternGetByID:
		return "GetByID"
	case PatternGetByKey:
		if len(info.WhereFields) > 0 {
			return "GetBy" + toPascalCase(info.WhereFields[0].Column)
		}
		return "GetByKey"
	case PatternGetAll:
		return "GetAll"
	case PatternGetByFK:
		if len(info.WhereFields) > 0 {
			return "GetBy" + toPascalCase(info.WhereFields[0].Column)
		}
		return "Find"
	case PatternSearch:
		return "Search"
	case PatternExists:
		return "Exists"
	case PatternCount:
		return "Count"
	case PatternGetMany:
		return "GetMany"
	default:
		return "Query"
	}
}

// toPascalCase converts snake_case to PascalCase.
func toPascalCase(s string) string {
	result := make([]byte, 0, len(s))
	capitalizeNext := true
	
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '_' {
			capitalizeNext = true
			continue
		}
		if capitalizeNext && c >= 'a' && c <= 'z' {
			c -= 'a' - 'A'
		}
		capitalizeNext = false
		result = append(result, c)
	}
	
	return string(result)
}
