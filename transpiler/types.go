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

// sanitiseIdentifier removes T-SQL-specific prefixes and invalid characters.
func sanitiseIdentifier(name string) string {
	// Remove @ prefix (variables)
	name = strings.TrimPrefix(name, "@")
	name = strings.TrimPrefix(name, "@@")

	// Handle bracketed identifiers
	name = strings.TrimPrefix(name, "[")
	name = strings.TrimSuffix(name, "]")

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

// splitIdentifier splits an identifier into words based on underscores and case transitions.
// Examples:
//   - "calculate_total" -> ["calculate", "total"]
//   - "CalculateTotal"  -> ["Calculate", "Total"]
//   - "CALCULATE_TOTAL" -> ["CALCULATE", "TOTAL"]
//   - "calculateTotal"  -> ["calculate", "Total"]
func splitIdentifier(name string) []string {
	if name == "" {
		return nil
	}

	// First split on underscores
	parts := strings.Split(name, "_")

	var words []string
	for _, part := range parts {
		if part == "" {
			continue
		}
		// Now split on case transitions (lowercase -> uppercase)
		words = append(words, splitOnCaseTransition(part)...)
	}

	return words
}

// splitOnCaseTransition splits a string where lowercase transitions to uppercase.
// "calculateTotal" -> ["calculate", "Total"]
// "HTTPServer"     -> ["HTTP", "Server"]
// "ALLCAPS"        -> ["ALLCAPS"]
func splitOnCaseTransition(s string) []string {
	if s == "" {
		return nil
	}

	var words []string
	var current strings.Builder

	runes := []rune(s)
	for i, r := range runes {
		if i == 0 {
			current.WriteRune(r)
			continue
		}

		prevUpper := isUpper(runes[i-1])
		currUpper := isUpper(r)

		// Transition: lowercase -> uppercase starts a new word
		if !prevUpper && currUpper {
			if current.Len() > 0 {
				words = append(words, current.String())
				current.Reset()
			}
		}

		// Transition: uppercase -> lowercase in a run of uppercase
		// "HTTPServer" -> at 'e', we need to break before 'S'
		if prevUpper && !currUpper && current.Len() > 1 {
			// Move the last character to the new word
			str := current.String()
			words = append(words, str[:len(str)-1])
			current.Reset()
			current.WriteRune(runes[i-1])
		}

		current.WriteRune(r)
	}

	if current.Len() > 0 {
		words = append(words, current.String())
	}

	return words
}

func isUpper(r rune) bool {
	return r >= 'A' && r <= 'Z'
}

func isLower(r rune) bool {
	return r >= 'a' && r <= 'z'
}

func toLowerRune(r rune) rune {
	if isUpper(r) {
		return r + 32
	}
	return r
}

func toUpperRune(r rune) rune {
	if isLower(r) {
		return r - 32
	}
	return r
}

// toPascalCase converts a word to PascalCase (first letter uppercase, rest lowercase).
func wordToPascal(word string) string {
	if word == "" {
		return ""
	}
	runes := []rune(strings.ToLower(word))
	runes[0] = toUpperRune(runes[0])
	return string(runes)
}

// goExportedIdentifier converts a T-SQL identifier to an exported Go identifier (PascalCase).
// Use for procedure names and other public API elements.
// Examples:
//   - "calculate_total" -> "CalculateTotal"
//   - "CALCULATE_TOTAL" -> "CalculateTotal"
//   - "calculateTotal"  -> "CalculateTotal"
func goExportedIdentifier(name string) string {
	name = sanitiseIdentifier(name)
	if name == "" {
		return ""
	}

	words := splitIdentifier(name)
	if len(words) == 0 {
		return name
	}

	var result strings.Builder
	for _, word := range words {
		result.WriteString(wordToPascal(word))
	}

	out := result.String()

	// If it starts with a digit after all processing, prefix with underscore
	if len(out) > 0 && out[0] >= '0' && out[0] <= '9' {
		out = "_" + out
	}

	return out
}

// goUnexportedIdentifier converts a T-SQL identifier to an unexported Go identifier (camelCase).
// Use for parameters, local variables, and other internal elements.
// Examples:
//   - "calculate_total" -> "calculateTotal"
//   - "CALCULATE_TOTAL" -> "calculateTotal"
//   - "CalculateTotal"  -> "calculateTotal"
func goUnexportedIdentifier(name string) string {
	name = sanitiseIdentifier(name)
	if name == "" {
		return ""
	}

	words := splitIdentifier(name)
	if len(words) == 0 {
		return strings.ToLower(name)
	}

	var result strings.Builder
	for i, word := range words {
		if i == 0 {
			result.WriteString(strings.ToLower(word))
		} else {
			result.WriteString(wordToPascal(word))
		}
	}

	out := result.String()

	// If it starts with a digit, prefix with underscore
	if len(out) > 0 && out[0] >= '0' && out[0] <= '9' {
		out = "_" + out
	}

	return out
}

// goIdentifier is a compatibility wrapper - defaults to unexported (camelCase).
// This maintains backward compatibility but call sites should migrate to
// goExportedIdentifier or goUnexportedIdentifier for clarity.
func goIdentifier(name string) string {
	return goUnexportedIdentifier(name)
}
