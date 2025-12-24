package transpiler

import (
	"github.com/ha1tch/tsqlparser/ast"
)

// typeInfo represents type information for a variable or expression.
type typeInfo struct {
	goType     string // The Go type string (e.g., "int32", "decimal.Decimal")
	isDecimal  bool   // Shorthand for decimal types
	isNumeric  bool   // True for any numeric type (int, float, decimal)
	isString   bool
	isDateTime bool
	isBool     bool
}

// symbolTable tracks variable declarations and their types.
type symbolTable struct {
	variables   map[string]*typeInfo
	parent      *symbolTable // For nested scopes (future use)
	
	// Track Go variable declarations (to use = vs :=)
	declaredVars map[string]bool
	
	// Track variable reads to identify unused variables
	usedVars map[string]bool
}

func newSymbolTable() *symbolTable {
	return &symbolTable{
		variables:    make(map[string]*typeInfo),
		declaredVars: make(map[string]bool),
		usedVars:     make(map[string]bool),
	}
}

func (st *symbolTable) define(name string, ti *typeInfo) {
	st.variables[name] = ti
}

func (st *symbolTable) lookup(name string) *typeInfo {
	if ti, ok := st.variables[name]; ok {
		return ti
	}
	if st.parent != nil {
		return st.parent.lookup(name)
	}
	return nil
}

// markDeclared marks a Go variable as having been declared with :=
func (st *symbolTable) markDeclared(name string) {
	st.declaredVars[name] = true
}

// isDeclared checks if a Go variable has been declared
func (st *symbolTable) isDeclared(name string) bool {
	if st.declaredVars[name] {
		return true
	}
	if st.parent != nil {
		return st.parent.isDeclared(name)
	}
	return false
}

// markUsed marks a variable as having been read/used
func (st *symbolTable) markUsed(name string) {
	st.usedVars[name] = true
	if st.parent != nil {
		st.parent.markUsed(name)
	}
}

// isUsed checks if a variable has been read/used
func (st *symbolTable) isUsed(name string) bool {
	if st.usedVars[name] {
		return true
	}
	if st.parent != nil {
		return st.parent.isUsed(name)
	}
	return false
}

// getUnusedVars returns variables that were declared but never read
func (st *symbolTable) getUnusedVars() []string {
	var unused []string
	for name := range st.declaredVars {
		if !st.usedVars[name] {
			unused = append(unused, name)
		}
	}
	return unused
}

// typeInfoFromDataType creates typeInfo from a T-SQL DataType.
func typeInfoFromDataType(dt *ast.DataType) *typeInfo {
	if dt == nil {
		return &typeInfo{goType: "interface{}"}
	}

	goType, isDecimal, isNumeric, isString, isDateTime, isBool := classifyDataType(dt)
	return &typeInfo{
		goType:     goType,
		isDecimal:  isDecimal,
		isNumeric:  isNumeric,
		isString:   isString,
		isDateTime: isDateTime,
		isBool:     isBool,
	}
}

// classifyDataType returns type classification for a T-SQL data type.
func classifyDataType(dt *ast.DataType) (goType string, isDecimal, isNumeric, isString, isDateTime, isBool bool) {
	switch normaliseTypeName(dt.Name) {
	case "TINYINT":
		return "uint8", false, true, false, false, false
	case "SMALLINT":
		return "int16", false, true, false, false, false
	case "INT", "INTEGER":
		return "int32", false, true, false, false, false
	case "BIGINT":
		return "int64", false, true, false, false, false
	case "REAL", "FLOAT":
		return "float64", false, true, false, false, false
	case "DECIMAL", "NUMERIC", "MONEY", "SMALLMONEY":
		return "decimal.Decimal", true, true, false, false, false
	case "CHAR", "VARCHAR", "TEXT", "NCHAR", "NVARCHAR", "NTEXT", "SYSNAME":
		return "string", false, false, true, false, false
	case "DATE", "TIME", "DATETIME", "DATETIME2", "SMALLDATETIME", "DATETIMEOFFSET":
		return "time.Time", false, false, false, true, false
	case "BIT":
		return "bool", false, false, false, false, true
	case "BINARY", "VARBINARY", "IMAGE":
		return "[]byte", false, false, false, false, false
	case "UNIQUEIDENTIFIER", "XML":
		return "string", false, false, true, false, false
	default:
		return "interface{}", false, false, false, false, false
	}
}

// normaliseTypeName converts a type name to uppercase for comparison.
func normaliseTypeName(name string) string {
	// Simple uppercase - could use strings.ToUpper but avoiding import
	result := make([]byte, len(name))
	for i := 0; i < len(name); i++ {
		c := name[i]
		if c >= 'a' && c <= 'z' {
			c -= 32
		}
		result[i] = c
	}
	return string(result)
}
