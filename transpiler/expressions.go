package transpiler

import (
	"fmt"
	"strings"

	"github.com/ha1tch/tsqlparser/ast"
)

func (t *transpiler) transpileExpression(expr ast.Expression) (string, error) {
	if expr == nil {
		return "", fmt.Errorf("nil expression")
	}

	switch e := expr.(type) {
	case *ast.Identifier:
		return goIdentifier(e.Value), nil

	case *ast.QualifiedIdentifier:
		var parts []string
		for _, p := range e.Parts {
			parts = append(parts, goIdentifier(p.Value))
		}
		return strings.Join(parts, "."), nil

	case *ast.Variable:
		return goIdentifier(e.Name), nil

	case *ast.IntegerLiteral:
		return fmt.Sprintf("%d", e.Value), nil

	case *ast.FloatLiteral:
		return fmt.Sprintf("%v", e.Value), nil

	case *ast.StringLiteral:
		// Go string literal
		return fmt.Sprintf("%q", e.Value), nil

	case *ast.NullLiteral:
		// NOTE: This is a simplification - proper NULL handling deferred
		return "nil", nil

	case *ast.BinaryLiteral:
		// Convert hex to Go byte slice literal
		return fmt.Sprintf("[]byte(%q)", e.Value), nil

	case *ast.MoneyLiteral:
		t.imports["github.com/shopspring/decimal"] = true
		// Strip currency symbol and convert to decimal
		val := strings.TrimPrefix(e.Value, "$")
		return fmt.Sprintf("decimal.RequireFromString(%q)", val), nil

	case *ast.PrefixExpression:
		return t.transpilePrefixExpression(e)

	case *ast.InfixExpression:
		return t.transpileInfixExpression(e)

	case *ast.FunctionCall:
		return t.transpileFunctionCall(e)

	case *ast.CaseExpression:
		return t.transpileCaseExpression(e)

	case *ast.CastExpression:
		return t.transpileCastExpression(e)

	case *ast.ConvertExpression:
		return t.transpileConvertExpression(e)

	case *ast.IsNullExpression:
		return t.transpileIsNullExpression(e)

	case *ast.BetweenExpression:
		return t.transpileBetweenExpression(e)

	case *ast.InExpression:
		return t.transpileInExpression(e)

	case *ast.TupleExpression:
		var parts []string
		for _, item := range e.Elements {
			s, err := t.transpileExpression(item)
			if err != nil {
				return "", err
			}
			parts = append(parts, s)
		}
		return "(" + strings.Join(parts, ", ") + ")", nil

	case *ast.SubqueryExpression:
		return "", fmt.Errorf("subqueries not supported in procedural transpilation")

	default:
		return "", fmt.Errorf("unsupported expression type: %T", expr)
	}
}

func (t *transpiler) transpilePrefixExpression(e *ast.PrefixExpression) (string, error) {
	right, err := t.transpileExpression(e.Right)
	if err != nil {
		return "", err
	}

	op := e.Operator
	switch strings.ToUpper(op) {
	case "NOT":
		op = "!"
	case "~":
		op = "^" // Bitwise NOT in Go
	}

	return fmt.Sprintf("(%s%s)", op, right), nil
}

func (t *transpiler) transpileInfixExpression(e *ast.InfixExpression) (string, error) {
	left, err := t.transpileExpression(e.Left)
	if err != nil {
		return "", err
	}

	right, err := t.transpileExpression(e.Right)
	if err != nil {
		return "", err
	}

	leftType := t.inferType(e.Left)
	rightType := t.inferType(e.Right)

	op := strings.ToUpper(e.Operator)

	// Check if either operand is decimal
	if leftType.isDecimal || rightType.isDecimal {
		return t.transpileDecimalInfix(left, right, e.Left, e.Right, leftType, rightType, op)
	}

	// Standard operator mapping for non-decimal types
	goOp := t.mapOperator(e.Operator)
	return fmt.Sprintf("(%s %s %s)", left, goOp, right), nil
}

// transpileDecimalInfix handles arithmetic/comparison when at least one operand is decimal.
func (t *transpiler) transpileDecimalInfix(left, right string, leftExpr, rightExpr ast.Expression, leftType, rightType *typeInfo, op string) (string, error) {
	t.imports["github.com/shopspring/decimal"] = true

	// Ensure both operands are decimal
	leftDec := left
	rightDec := right
	if !leftType.isDecimal {
		leftDec = t.ensureDecimal(leftExpr, left)
	}
	if !rightType.isDecimal {
		rightDec = t.ensureDecimal(rightExpr, right)
	}

	// Arithmetic operators
	switch op {
	case "+":
		return fmt.Sprintf("%s.Add(%s)", leftDec, rightDec), nil
	case "-":
		return fmt.Sprintf("%s.Sub(%s)", leftDec, rightDec), nil
	case "*":
		return fmt.Sprintf("%s.Mul(%s)", leftDec, rightDec), nil
	case "/":
		return fmt.Sprintf("%s.Div(%s)", leftDec, rightDec), nil
	case "%":
		return fmt.Sprintf("%s.Mod(%s)", leftDec, rightDec), nil

	// Comparison operators - return bool expressions
	case "=":
		return fmt.Sprintf("%s.Equal(%s)", leftDec, rightDec), nil
	case "<>", "!=":
		return fmt.Sprintf("!%s.Equal(%s)", leftDec, rightDec), nil
	case "<":
		return fmt.Sprintf("%s.LessThan(%s)", leftDec, rightDec), nil
	case "<=":
		return fmt.Sprintf("%s.LessThanOrEqual(%s)", leftDec, rightDec), nil
	case ">":
		return fmt.Sprintf("%s.GreaterThan(%s)", leftDec, rightDec), nil
	case ">=":
		return fmt.Sprintf("%s.GreaterThanOrEqual(%s)", leftDec, rightDec), nil

	default:
		// For other operators (AND, OR, etc.), fall back to standard
		goOp := t.mapOperator(op)
		return fmt.Sprintf("(%s %s %s)", left, goOp, right), nil
	}
}

// ensureDecimal wraps a non-decimal expression to convert it to decimal.Decimal.
func (t *transpiler) ensureDecimal(expr ast.Expression, transpiled string) string {
	t.imports["github.com/shopspring/decimal"] = true

	ti := t.inferType(expr)

	// Already decimal
	if ti.isDecimal {
		return transpiled
	}

	// Integer literal
	if _, ok := expr.(*ast.IntegerLiteral); ok {
		return fmt.Sprintf("decimal.NewFromInt(%s)", transpiled)
	}

	// Float literal
	if _, ok := expr.(*ast.FloatLiteral); ok {
		return fmt.Sprintf("decimal.NewFromFloat(%s)", transpiled)
	}

	// Integer variable/expression
	if ti.isNumeric && !ti.isDecimal {
		switch ti.goType {
		case "int32", "int16", "uint8":
			return fmt.Sprintf("decimal.NewFromInt(int64(%s))", transpiled)
		case "int64":
			return fmt.Sprintf("decimal.NewFromInt(%s)", transpiled)
		case "float64":
			return fmt.Sprintf("decimal.NewFromFloat(%s)", transpiled)
		}
	}

	// Default: try NewFromFloat for numeric expressions
	return fmt.Sprintf("decimal.NewFromFloat(float64(%s))", transpiled)
}

// inferType attempts to determine the type of an expression.
func (t *transpiler) inferType(expr ast.Expression) *typeInfo {
	switch e := expr.(type) {
	case *ast.Variable:
		name := goIdentifier(e.Name)
		if ti := t.symbols.lookup(name); ti != nil {
			return ti
		}
	case *ast.Identifier:
		name := goIdentifier(e.Value)
		if ti := t.symbols.lookup(name); ti != nil {
			return ti
		}
	case *ast.IntegerLiteral:
		return &typeInfo{goType: "int64", isNumeric: true}
	case *ast.FloatLiteral:
		return &typeInfo{goType: "float64", isNumeric: true}
	case *ast.StringLiteral:
		return &typeInfo{goType: "string", isString: true}
	case *ast.NullLiteral:
		return &typeInfo{goType: "interface{}"}
	case *ast.MoneyLiteral:
		return &typeInfo{goType: "decimal.Decimal", isDecimal: true, isNumeric: true}
	case *ast.InfixExpression:
		// For arithmetic, result type depends on operands
		leftType := t.inferType(e.Left)
		rightType := t.inferType(e.Right)
		// If either is decimal, result is decimal
		if leftType.isDecimal || rightType.isDecimal {
			return &typeInfo{goType: "decimal.Decimal", isDecimal: true, isNumeric: true}
		}
		// If either is float, result is float
		if leftType.goType == "float64" || rightType.goType == "float64" {
			return &typeInfo{goType: "float64", isNumeric: true}
		}
		// Otherwise, keep as numeric
		if leftType.isNumeric && rightType.isNumeric {
			return &typeInfo{goType: "int64", isNumeric: true}
		}
	case *ast.FunctionCall:
		// Some functions have known return types
		if id, ok := e.Function.(*ast.Identifier); ok {
			return t.inferFunctionReturnType(id.Value)
		}
	case *ast.CastExpression:
		return typeInfoFromDataType(e.TargetType)
	case *ast.ConvertExpression:
		return typeInfoFromDataType(e.TargetType)
	}

	// Default: unknown type
	return &typeInfo{goType: "interface{}"}
}

// inferFunctionReturnType returns type info for known T-SQL functions.
func (t *transpiler) inferFunctionReturnType(funcName string) *typeInfo {
	switch normaliseTypeName(funcName) {
	case "LEN", "DATALENGTH", "CHARINDEX", "PATINDEX":
		return &typeInfo{goType: "int64", isNumeric: true}
	case "UPPER", "LOWER", "LTRIM", "RTRIM", "TRIM", "SUBSTRING", "LEFT", "RIGHT", "REPLACE", "REPLICATE", "REVERSE", "CONCAT", "CONCAT_WS":
		return &typeInfo{goType: "string", isString: true}
	case "ABS", "CEILING", "CEIL", "FLOOR", "ROUND", "POWER", "SQRT", "SIGN":
		return &typeInfo{goType: "float64", isNumeric: true}
	case "GETDATE", "SYSDATETIME", "GETUTCDATE", "SYSUTCDATETIME", "DATEADD":
		return &typeInfo{goType: "time.Time", isDateTime: true}
	case "DATEDIFF", "YEAR", "MONTH", "DAY":
		return &typeInfo{goType: "int64", isNumeric: true}
	default:
		return &typeInfo{goType: "interface{}"}
	}
}

func (t *transpiler) mapOperator(op string) string {
	switch strings.ToUpper(op) {
	case "AND":
		return "&&"
	case "OR":
		return "||"
	case "=":
		return "=="
	case "<>", "!=":
		return "!="
	case "!<":
		return ">="
	case "!>":
		return "<="
	default:
		return op
	}
}

func (t *transpiler) transpileFunctionCall(fc *ast.FunctionCall) (string, error) {
	funcName := ""
	if id, ok := fc.Function.(*ast.Identifier); ok {
		funcName = strings.ToUpper(id.Value)
	} else if qid, ok := fc.Function.(*ast.QualifiedIdentifier); ok {
		if len(qid.Parts) > 0 {
			funcName = strings.ToUpper(qid.Parts[len(qid.Parts)-1].Value)
		}
	}

	var args []string
	for _, arg := range fc.Arguments {
		a, err := t.transpileExpression(arg)
		if err != nil {
			return "", err
		}
		args = append(args, a)
	}

	// Map common T-SQL functions to Go equivalents
	switch funcName {
	case "LEN":
		t.imports["unicode/utf8"] = true
		if len(args) == 1 {
			return fmt.Sprintf("utf8.RuneCountInString(%s)", args[0]), nil
		}

	case "DATALENGTH":
		if len(args) == 1 {
			return fmt.Sprintf("len(%s)", args[0]), nil
		}

	case "UPPER":
		t.imports["strings"] = true
		if len(args) == 1 {
			return fmt.Sprintf("strings.ToUpper(%s)", args[0]), nil
		}

	case "LOWER":
		t.imports["strings"] = true
		if len(args) == 1 {
			return fmt.Sprintf("strings.ToLower(%s)", args[0]), nil
		}

	case "LTRIM":
		t.imports["strings"] = true
		if len(args) == 1 {
			return fmt.Sprintf("strings.TrimLeft(%s, \" \")", args[0]), nil
		}

	case "RTRIM":
		t.imports["strings"] = true
		if len(args) == 1 {
			return fmt.Sprintf("strings.TrimRight(%s, \" \")", args[0]), nil
		}

	case "TRIM":
		t.imports["strings"] = true
		if len(args) == 1 {
			return fmt.Sprintf("strings.TrimSpace(%s)", args[0]), nil
		}

	case "SUBSTRING":
		// SUBSTRING(str, start, length) -> str[start-1 : start-1+length]
		// Note: T-SQL is 1-indexed, Go is 0-indexed
		if len(args) == 3 {
			return fmt.Sprintf("(%s)[(%s)-1:(%s)-1+(%s)]", args[0], args[1], args[1], args[2]), nil
		}

	case "LEFT":
		if len(args) == 2 {
			return fmt.Sprintf("(%s)[:(%s)]", args[0], args[1]), nil
		}

	case "RIGHT":
		if len(args) == 2 {
			return fmt.Sprintf("(%s)[len(%s)-(%s):]", args[0], args[0], args[1]), nil
		}

	case "CHARINDEX":
		t.imports["strings"] = true
		if len(args) >= 2 {
			// CHARINDEX returns 0 if not found, 1-based index otherwise
			// strings.Index returns -1 if not found, 0-based index otherwise
			return fmt.Sprintf("(strings.Index(%s, %s) + 1)", args[1], args[0]), nil
		}

	case "REPLACE":
		t.imports["strings"] = true
		if len(args) == 3 {
			return fmt.Sprintf("strings.ReplaceAll(%s, %s, %s)", args[0], args[1], args[2]), nil
		}

	case "REPLICATE":
		t.imports["strings"] = true
		if len(args) == 2 {
			return fmt.Sprintf("strings.Repeat(%s, %s)", args[0], args[1]), nil
		}

	case "REVERSE":
		// Go doesn't have a built-in reverse; we'd need a helper function
		// For now, mark as needing runtime support
		return "", fmt.Errorf("REVERSE function requires runtime helper (not yet implemented)")

	case "CONCAT":
		// CONCAT in T-SQL ignores NULLs; in Go we just concatenate
		return fmt.Sprintf("(%s)", strings.Join(args, " + ")), nil

	case "CONCAT_WS":
		t.imports["strings"] = true
		if len(args) >= 2 {
			return fmt.Sprintf("strings.Join([]string{%s}, %s)", strings.Join(args[1:], ", "), args[0]), nil
		}

	case "ISNULL":
		// ISNULL(a, b) -> if a != nil { a } else { b }
		// Simplified: just use b for now since we're not handling nulls properly
		if len(args) == 2 {
			return fmt.Sprintf("func() interface{} { if %s != nil { return %s }; return %s }()", args[0], args[0], args[1]), nil
		}

	case "COALESCE":
		// Similar to ISNULL but with multiple args
		if len(args) > 0 {
			return args[len(args)-1], nil // Simplified: return last non-null candidate
		}

	case "NULLIF":
		// NULLIF(a, b) -> if a == b { nil } else { a }
		if len(args) == 2 {
			return fmt.Sprintf("func() interface{} { if %s == %s { return nil }; return %s }()", args[0], args[1], args[0]), nil
		}

	case "ABS":
		t.imports["math"] = true
		if len(args) == 1 {
			return fmt.Sprintf("math.Abs(float64(%s))", args[0]), nil
		}

	case "CEILING", "CEIL":
		t.imports["math"] = true
		if len(args) == 1 {
			return fmt.Sprintf("math.Ceil(float64(%s))", args[0]), nil
		}

	case "FLOOR":
		t.imports["math"] = true
		if len(args) == 1 {
			return fmt.Sprintf("math.Floor(float64(%s))", args[0]), nil
		}

	case "ROUND":
		t.imports["math"] = true
		if len(args) >= 1 {
			if len(args) == 1 {
				return fmt.Sprintf("math.Round(%s)", args[0]), nil
			}
			// ROUND(x, decimals) - more complex
			return fmt.Sprintf("math.Round(%s*math.Pow(10, float64(%s)))/math.Pow(10, float64(%s))", args[0], args[1], args[1]), nil
		}

	case "POWER":
		t.imports["math"] = true
		if len(args) == 2 {
			return fmt.Sprintf("math.Pow(float64(%s), float64(%s))", args[0], args[1]), nil
		}

	case "SQRT":
		t.imports["math"] = true
		if len(args) == 1 {
			return fmt.Sprintf("math.Sqrt(float64(%s))", args[0]), nil
		}

	case "SIGN":
		t.imports["math"] = true
		if len(args) == 1 {
			return fmt.Sprintf("int(math.Copysign(1, float64(%s)))", args[0]), nil
		}

	case "GETDATE", "SYSDATETIME", "CURRENT_TIMESTAMP":
		t.imports["time"] = true
		return "time.Now()", nil

	case "GETUTCDATE", "SYSUTCDATETIME":
		t.imports["time"] = true
		return "time.Now().UTC()", nil

	case "DATEADD":
		// DATEADD(interval, number, date)
		t.imports["time"] = true
		if len(args) == 3 {
			interval := strings.Trim(args[0], "\"")
			return t.transpileDateAdd(interval, args[1], args[2])
		}

	case "DATEDIFF":
		// DATEDIFF(interval, start, end)
		t.imports["time"] = true
		if len(args) == 3 {
			interval := strings.Trim(args[0], "\"")
			return t.transpileDateDiff(interval, args[1], args[2])
		}

	case "YEAR":
		t.imports["time"] = true
		if len(args) == 1 {
			return fmt.Sprintf("(%s).Year()", args[0]), nil
		}

	case "MONTH":
		t.imports["time"] = true
		if len(args) == 1 {
			return fmt.Sprintf("int((%s).Month())", args[0]), nil
		}

	case "DAY":
		t.imports["time"] = true
		if len(args) == 1 {
			return fmt.Sprintf("(%s).Day()", args[0]), nil
		}

	case "NEWID":
		// Would need a UUID library
		return "", fmt.Errorf("NEWID() requires uuid library (not yet implemented)")

	case "IIF":
		// IIF(condition, true_value, false_value)
		if len(args) == 3 {
			return fmt.Sprintf("func() interface{} { if %s { return %s }; return %s }()", args[0], args[1], args[2]), nil
		}

	// Error functions for TRY/CATCH - _recovered is set in the CATCH block
	case "ERROR_MESSAGE":
		t.imports["fmt"] = true
		return "fmt.Sprintf(\"%v\", _recovered)", nil

	case "ERROR_NUMBER":
		// No direct equivalent in Go - return 0
		return "0", nil

	case "ERROR_SEVERITY", "ERROR_STATE":
		// No direct equivalent in Go - return 0
		return "0", nil

	case "ERROR_PROCEDURE":
		// No direct equivalent - return empty string
		return "\"\"", nil

	case "ERROR_LINE":
		// No direct equivalent - return 0
		return "0", nil
	}

	// Default: output as-is (unknown function) - use exported name as it's likely a procedure
	return fmt.Sprintf("%s(%s)", goExportedIdentifier(funcName), strings.Join(args, ", ")), nil
}

func (t *transpiler) transpileDateAdd(interval, number, date string) (string, error) {
	interval = strings.ToUpper(interval)
	switch interval {
	case "YEAR", "YY", "YYYY":
		return fmt.Sprintf("(%s).AddDate(%s, 0, 0)", date, number), nil
	case "MONTH", "MM", "M":
		return fmt.Sprintf("(%s).AddDate(0, %s, 0)", date, number), nil
	case "DAY", "DD", "D":
		return fmt.Sprintf("(%s).AddDate(0, 0, %s)", date, number), nil
	case "HOUR", "HH":
		return fmt.Sprintf("(%s).Add(time.Duration(%s) * time.Hour)", date, number), nil
	case "MINUTE", "MI", "N":
		return fmt.Sprintf("(%s).Add(time.Duration(%s) * time.Minute)", date, number), nil
	case "SECOND", "SS", "S":
		return fmt.Sprintf("(%s).Add(time.Duration(%s) * time.Second)", date, number), nil
	default:
		return "", fmt.Errorf("unsupported DATEADD interval: %s", interval)
	}
}

func (t *transpiler) transpileDateDiff(interval, start, end string) (string, error) {
	interval = strings.ToUpper(interval)
	switch interval {
	case "YEAR", "YY", "YYYY":
		return fmt.Sprintf("((%s).Year() - (%s).Year())", end, start), nil
	case "MONTH", "MM", "M":
		return fmt.Sprintf("(((%s).Year()-(%s).Year())*12 + int((%s).Month()) - int((%s).Month()))", end, start, end, start), nil
	case "DAY", "DD", "D":
		return fmt.Sprintf("int((%s).Sub(%s).Hours() / 24)", end, start), nil
	case "HOUR", "HH":
		return fmt.Sprintf("int((%s).Sub(%s).Hours())", end, start), nil
	case "MINUTE", "MI", "N":
		return fmt.Sprintf("int((%s).Sub(%s).Minutes())", end, start), nil
	case "SECOND", "SS", "S":
		return fmt.Sprintf("int((%s).Sub(%s).Seconds())", end, start), nil
	default:
		return "", fmt.Errorf("unsupported DATEDIFF interval: %s", interval)
	}
}

func (t *transpiler) transpileCaseExpression(c *ast.CaseExpression) (string, error) {
	var out strings.Builder

	out.WriteString("func() interface{} {\n")

	if c.Operand != nil {
		// Simple CASE: CASE expr WHEN val THEN result
		operand, err := t.transpileExpression(c.Operand)
		if err != nil {
			return "", err
		}
		out.WriteString(fmt.Sprintf("\tswitch %s {\n", operand))
		for _, when := range c.WhenClauses {
			cond, err := t.transpileExpression(when.Condition)
			if err != nil {
				return "", err
			}
			result, err := t.transpileExpression(when.Result)
			if err != nil {
				return "", err
			}
			out.WriteString(fmt.Sprintf("\tcase %s:\n\t\treturn %s\n", cond, result))
		}
		if c.ElseClause != nil {
			elseRes, err := t.transpileExpression(c.ElseClause)
			if err != nil {
				return "", err
			}
			out.WriteString(fmt.Sprintf("\tdefault:\n\t\treturn %s\n", elseRes))
		} else {
			out.WriteString("\tdefault:\n\t\treturn nil\n")
		}
		out.WriteString("\t}")
	} else {
		// Searched CASE: CASE WHEN cond THEN result
		for i, when := range c.WhenClauses {
			cond, err := t.transpileExpression(when.Condition)
			if err != nil {
				return "", err
			}
			result, err := t.transpileExpression(when.Result)
			if err != nil {
				return "", err
			}
			if i == 0 {
				out.WriteString(fmt.Sprintf("\tif %s {\n\t\treturn %s\n\t}", cond, result))
			} else {
				out.WriteString(fmt.Sprintf(" else if %s {\n\t\treturn %s\n\t}", cond, result))
			}
		}
		if c.ElseClause != nil {
			elseRes, err := t.transpileExpression(c.ElseClause)
			if err != nil {
				return "", err
			}
			out.WriteString(fmt.Sprintf(" else {\n\t\treturn %s\n\t}", elseRes))
		} else {
			out.WriteString(" else {\n\t\treturn nil\n\t}")
		}
	}

	out.WriteString("\n}()")
	return out.String(), nil
}

func (t *transpiler) transpileCastExpression(c *ast.CastExpression) (string, error) {
	expr, err := t.transpileExpression(c.Expression)
	if err != nil {
		return "", err
	}

	goType, err := t.mapDataType(c.TargetType)
	if err != nil {
		return "", err
	}

	// Simple type conversion - this is a simplification
	switch goType {
	case "string":
		t.imports["fmt"] = true
		return fmt.Sprintf("fmt.Sprintf(\"%%v\", %s)", expr), nil
	case "int32":
		return fmt.Sprintf("int32(%s)", expr), nil
	case "int64":
		return fmt.Sprintf("int64(%s)", expr), nil
	case "float64":
		return fmt.Sprintf("float64(%s)", expr), nil
	case "decimal.Decimal":
		t.imports["github.com/shopspring/decimal"] = true
		return fmt.Sprintf("decimal.NewFromFloat(float64(%s))", expr), nil
	default:
		return fmt.Sprintf("%s(%s)", goType, expr), nil
	}
}

func (t *transpiler) transpileConvertExpression(c *ast.ConvertExpression) (string, error) {
	// CONVERT is similar to CAST
	expr, err := t.transpileExpression(c.Expression)
	if err != nil {
		return "", err
	}

	goType, err := t.mapDataType(c.TargetType)
	if err != nil {
		return "", err
	}

	// Style parameter is ignored for now
	switch goType {
	case "string":
		t.imports["fmt"] = true
		return fmt.Sprintf("fmt.Sprintf(\"%%v\", %s)", expr), nil
	case "int32":
		return fmt.Sprintf("int32(%s)", expr), nil
	case "int64":
		return fmt.Sprintf("int64(%s)", expr), nil
	case "float64":
		return fmt.Sprintf("float64(%s)", expr), nil
	case "decimal.Decimal":
		t.imports["github.com/shopspring/decimal"] = true
		return fmt.Sprintf("decimal.NewFromFloat(float64(%s))", expr), nil
	default:
		return fmt.Sprintf("%s(%s)", goType, expr), nil
	}
}

func (t *transpiler) transpileIsNullExpression(e *ast.IsNullExpression) (string, error) {
	expr, err := t.transpileExpression(e.Expr)
	if err != nil {
		return "", err
	}

	if e.Not {
		return fmt.Sprintf("(%s != nil)", expr), nil
	}
	return fmt.Sprintf("(%s == nil)", expr), nil
}

func (t *transpiler) transpileBetweenExpression(e *ast.BetweenExpression) (string, error) {
	expr, err := t.transpileExpression(e.Expr)
	if err != nil {
		return "", err
	}
	low, err := t.transpileExpression(e.Low)
	if err != nil {
		return "", err
	}
	high, err := t.transpileExpression(e.High)
	if err != nil {
		return "", err
	}

	if e.Not {
		return fmt.Sprintf("(%s < %s || %s > %s)", expr, low, expr, high), nil
	}
	return fmt.Sprintf("(%s >= %s && %s <= %s)", expr, low, expr, high), nil
}

func (t *transpiler) transpileInExpression(e *ast.InExpression) (string, error) {
	expr, err := t.transpileExpression(e.Expr)
	if err != nil {
		return "", err
	}

	// Build a series of equality checks
	var checks []string
	for _, val := range e.Values {
		v, err := t.transpileExpression(val)
		if err != nil {
			return "", err
		}
		checks = append(checks, fmt.Sprintf("%s == %s", expr, v))
	}

	result := "(" + strings.Join(checks, " || ") + ")"
	if e.Not {
		result = "!" + result
	}
	return result, nil
}
