package transpiler

import (
	"fmt"
	"strings"

	"github.com/ha1tch/tsqlparser/ast"
)

// mapDataType converts a T-SQL data type to a Go type.
func (t *transpiler) mapDataType(dt *ast.DataType) (string, error) {
	if dt == nil {
		return "", fmt.Errorf("nil data type")
	}

	name := strings.ToUpper(dt.Name)

	switch name {
	// Integer types
	case "TINYINT":
		return "uint8", nil
	case "SMALLINT":
		return "int16", nil
	case "INT", "INTEGER":
		return "int32", nil
	case "BIGINT":
		return "int64", nil

	// Floating point types (all map to float64)
	case "REAL", "FLOAT":
		return "float64", nil

	// Exact numeric types (use shopspring/decimal)
	case "DECIMAL", "NUMERIC", "MONEY", "SMALLMONEY":
		t.imports["github.com/shopspring/decimal"] = true
		return "decimal.Decimal", nil

	// String types
	case "CHAR", "VARCHAR", "TEXT", "NCHAR", "NVARCHAR", "NTEXT", "SYSNAME":
		return "string", nil

	// Date/time types
	case "DATE", "TIME", "DATETIME", "DATETIME2", "SMALLDATETIME", "DATETIMEOFFSET":
		t.imports["time"] = true
		return "time.Time", nil

	// Boolean
	case "BIT":
		return "bool", nil

	// Binary types
	case "BINARY", "VARBINARY", "IMAGE":
		return "[]byte", nil

	// Other types
	case "UNIQUEIDENTIFIER":
		return "string", nil // Could use uuid.UUID with another import
	case "XML":
		return "string", nil
	case "SQL_VARIANT":
		return "interface{}", nil

	default:
		return "", fmt.Errorf("unsupported data type: %s", dt.Name)
	}
}

// goIdentifier converts a T-SQL identifier to a valid Go identifier.
// It removes @ prefix from variables and converts to camelCase if needed.
func goIdentifier(name string) string {
	// Remove @ prefix
	name = strings.TrimPrefix(name, "@")
	name = strings.TrimPrefix(name, "@@")

	// Handle bracketed identifiers
	name = strings.TrimPrefix(name, "[")
	name = strings.TrimSuffix(name, "]")

	// If it starts with a digit, prefix with underscore
	if len(name) > 0 && name[0] >= '0' && name[0] <= '9' {
		name = "_" + name
	}

	// Replace invalid characters with underscore
	var result strings.Builder
	for i, r := range name {
		if r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (i > 0 && r >= '0' && r <= '9') {
			result.WriteRune(r)
		} else {
			result.WriteRune('_')
		}
	}

	return result.String()
}
