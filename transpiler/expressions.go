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
		// Handle special system variables
		upperName := strings.ToUpper(e.Name)
		switch upperName {
		case "@@IDENTITY":
			return t.transpileIdentityFunction()
		case "@@ROWCOUNT":
			// rowsAffected is declared at function start if @@ROWCOUNT is used
			return "rowsAffected", nil
		case "@@ERROR":
			// In Go, errors are returned explicitly
			return "0 /* @@ERROR: check err != nil instead */", nil
		case "@@TRANCOUNT":
			// Transaction count - not directly available in Go
			return "0 /* @@TRANCOUNT: track transaction state in Go */", nil
		}
		// Mark variable as used (read)
		varName := goIdentifier(e.Name)
		t.symbols.markUsed(varName)
		return varName, nil

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
		if t.dmlEnabled {
			return t.transpileSubqueryExpression(e)
		}
		return "", fmt.Errorf("subqueries not supported in procedural transpilation")

	case *ast.ExistsExpression:
		if t.dmlEnabled {
			return t.transpileExistsExpression(e)
		}
		return "", fmt.Errorf("EXISTS not supported in procedural transpilation")

	case *ast.MethodCallExpression:
		return t.transpileMethodCallExpression(e)

	case *ast.NextValueForExpression:
		seqName := ""
		if e.SequenceName != nil {
			for _, part := range e.SequenceName.Parts {
				if seqName != "" {
					seqName += "."
				}
				seqName += part.Value
			}
		}
		return t.transpileNextValueFor(seqName)

	default:
		return "", unsupportedExpressionError(expr)
	}
}

// unsupportedExpressionError returns a helpful error message for unsupported expressions.
func unsupportedExpressionError(expr ast.Expression) error {
	typeName := fmt.Sprintf("%T", expr)
	
	// Provide specific hints based on type name
	switch {
	case strings.Contains(typeName, "NextValueFor"):
		return fmt.Errorf("unsupported expression type: %s\n"+
			"      Hint: NEXT VALUE FOR sequences are not yet supported.\n"+
			"      Workaround: Replace with a placeholder and implement sequence\n"+
			"      logic in Go using result.LastInsertId() or uuid.New().", typeName)
	
	case strings.Contains(typeName, "Over"):
		return fmt.Errorf("unsupported expression type: %s\n"+
			"      Hint: Window functions (OVER clause) are not yet supported.\n"+
			"      Workaround: Compute aggregations in Go after fetching results,\n"+
			"      or keep window function queries in the database.", typeName)
	
	case strings.Contains(typeName, "Pivot") || strings.Contains(typeName, "Unpivot"):
		return fmt.Errorf("unsupported expression type: %s\n"+
			"      Hint: PIVOT/UNPIVOT are not yet supported.\n"+
			"      Workaround: Transform the data in Go after fetching,\n"+
			"      or use a view in the database.", typeName)
	
	case strings.Contains(typeName, "XML"):
		return fmt.Errorf("unsupported expression type: %s\n"+
			"      Hint: XML expressions are partially supported.\n"+
			"      Use --dml mode for FOR XML queries.\n"+
			"      Complex XML operations may need manual conversion.", typeName)
	
	default:
		return fmt.Errorf("unsupported expression type: %s\n"+
			"      Hint: This expression type is not yet implemented.\n"+
			"      Please file an issue at github.com/ha1tch/tgpiler if you need it.", typeName)
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

	// Handle unary minus on decimal types
	if op == "-" {
		rightType := t.inferType(e.Right)
		if rightType.isDecimal {
			return fmt.Sprintf("%s.Neg()", right), nil
		}
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

	// Handle BIT/bool comparisons with 0 or 1
	// @Flag = 1 -> flag, @Flag = 0 -> !flag
	// @Flag <> 1 -> !flag, @Flag <> 0 -> flag
	if op == "=" || op == "<>" || op == "!=" {
		if leftType.isBool {
			if lit, ok := e.Right.(*ast.IntegerLiteral); ok {
				if lit.Value == 1 {
					if op == "=" {
						return left, nil
					}
					return fmt.Sprintf("!%s", left), nil
				}
				if lit.Value == 0 {
					if op == "=" {
						return fmt.Sprintf("!%s", left), nil
					}
					return left, nil
				}
			}
		}
		if rightType.isBool {
			if lit, ok := e.Left.(*ast.IntegerLiteral); ok {
				if lit.Value == 1 {
					if op == "=" {
						return right, nil
					}
					return fmt.Sprintf("!%s", right), nil
				}
				if lit.Value == 0 {
					if op == "=" {
						return fmt.Sprintf("!%s", right), nil
					}
					return right, nil
				}
			}
		}

		// Handle string comparison with NULL - use empty string instead
		if leftType.isString {
			if _, ok := e.Right.(*ast.NullLiteral); ok {
				if op == "=" {
					return fmt.Sprintf("(%s == \"\")", left), nil
				}
				return fmt.Sprintf("(%s != \"\")", left), nil
			}
		}
		if rightType.isString {
			if _, ok := e.Left.(*ast.NullLiteral); ok {
				if op == "=" {
					return fmt.Sprintf("(%s == \"\")", right), nil
				}
				return fmt.Sprintf("(%s != \"\")", right), nil
			}
		}
	}

	// Check if either operand is decimal
	if leftType.isDecimal || rightType.isDecimal {
		return t.transpileDecimalInfix(left, right, e.Left, e.Right, leftType, rightType, op)
	}

	// Handle T-SQL BIT toggle pattern: 1 - @BitVar becomes !bitVar
	// This is a common idiom for toggling a BIT value
	if op == "-" {
		if leftLit, ok := e.Left.(*ast.IntegerLiteral); ok && leftLit.Value == 1 && rightType.isBool {
			return fmt.Sprintf("!%s", right), nil
		}
	}

	// Handle mixed integer types in arithmetic operations
	// Go requires explicit conversion; T-SQL does implicit promotion
	// But Go's untyped integer literals adapt automatically, so only cast
	// when both operands are typed expressions with different types
	isArithmetic := op == "+" || op == "-" || op == "*" || op == "/" || op == "%"
	leftIsLiteral := isIntegerLiteral(e.Left)
	rightIsLiteral := isIntegerLiteral(e.Right)

	if isArithmetic && leftType.isNumeric && rightType.isNumeric && !leftIsLiteral && !rightIsLiteral {
		// Both are typed expressions - promote to the larger type
		if leftType.goType != rightType.goType {
			targetType := t.promoteNumericType(leftType.goType, rightType.goType)
			if targetType != leftType.goType {
				left = fmt.Sprintf("%s(%s)", targetType, left)
			}
			if targetType != rightType.goType {
				right = fmt.Sprintf("%s(%s)", targetType, right)
			}
		}
	}

	// Handle time.Time comparisons - Go doesn't support comparison operators on structs
	if leftType.isDateTime || rightType.isDateTime {
		switch op {
		case "=":
			return fmt.Sprintf("%s.Equal(%s)", left, right), nil
		case "<>", "!=":
			return fmt.Sprintf("!%s.Equal(%s)", left, right), nil
		case "<":
			return fmt.Sprintf("%s.Before(%s)", left, right), nil
		case ">":
			return fmt.Sprintf("%s.After(%s)", left, right), nil
		case "<=":
			return fmt.Sprintf("!%s.After(%s)", left, right), nil
		case ">=":
			return fmt.Sprintf("!%s.Before(%s)", left, right), nil
		}
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

// ensureBool converts T-SQL BIT semantics (0/1) to Go bool (false/true).
func (t *transpiler) ensureBool(expr ast.Expression, transpiled string) string {
	// Integer literal 0 -> false, 1 -> true
	if lit, ok := expr.(*ast.IntegerLiteral); ok {
		if lit.Value == 0 {
			return "false"
		}
		if lit.Value == 1 {
			return "true"
		}
		// Other integer values: treat non-zero as true (T-SQL behaviour)
		return fmt.Sprintf("(%s != 0)", transpiled)
	}

	// Already a bool variable/expression
	ti := t.inferType(expr)
	if ti.isBool {
		return transpiled
	}

	// Numeric expression: convert to bool comparison
	if ti.isNumeric {
		return fmt.Sprintf("(%s != 0)", transpiled)
	}

	// Default: assume it's usable as-is (may be a comparison result)
	return transpiled
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
	case *ast.PrefixExpression:
		// Unary operators preserve the type of their operand
		return t.inferType(e.Right)
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
		// For integers, promote to the larger type
		if leftType.isNumeric && rightType.isNumeric {
			// Integer literals adapt to the other operand's type
			_, leftIsLiteral := e.Left.(*ast.IntegerLiteral)
			_, rightIsLiteral := e.Right.(*ast.IntegerLiteral)
			if leftIsLiteral && !rightIsLiteral {
				return rightType
			}
			if rightIsLiteral && !leftIsLiteral {
				return leftType
			}
			// Both typed - use promoted type
			promoted := t.promoteNumericType(leftType.goType, rightType.goType)
			return &typeInfo{goType: promoted, isNumeric: true}
		}
	case *ast.FunctionCall:
		// Some functions have known return types
		if id, ok := e.Function.(*ast.Identifier); ok {
			funcName := normaliseTypeName(id.Value)
			
			// Check if this is a window function (has OVER clause)
			if e.Over != nil {
				return t.inferWindowFunctionType(funcName, e)
			}
			
			// For math functions, return type matches argument type
			switch funcName {
			case "ABS", "CEILING", "CEIL", "FLOOR", "ROUND", "POWER", "SQRT":
				if len(e.Arguments) > 0 {
					argType := t.inferType(e.Arguments[0])
					if argType.isDecimal {
						return &typeInfo{goType: "decimal.Decimal", isDecimal: true, isNumeric: true}
					}
				}
			case "ISNULL", "COALESCE":
				// Return type is the type of the first argument
				if len(e.Arguments) > 0 {
					return t.inferType(e.Arguments[0])
				}
			}
			return t.inferFunctionReturnType(id.Value)
		}
	case *ast.CastExpression:
		return typeInfoFromDataType(e.TargetType)
	case *ast.ConvertExpression:
		return typeInfoFromDataType(e.TargetType)
	case *ast.MethodCallExpression:
		// XML method return types
		switch strings.ToLower(e.MethodName) {
		case "value":
			// Return type depends on the type argument
			if len(e.Arguments) >= 2 {
				// Get the type argument (second argument)
				if str, ok := e.Arguments[1].(*ast.StringLiteral); ok {
					typeUpper := strings.ToUpper(str.Value)
					switch {
					case strings.HasPrefix(typeUpper, "INT"):
						return &typeInfo{goType: "int32", isNumeric: true}
					case strings.HasPrefix(typeUpper, "BIGINT"):
						return &typeInfo{goType: "int64", isNumeric: true}
					case strings.HasPrefix(typeUpper, "BIT"):
						return &typeInfo{goType: "bool", isBool: true}
					case strings.HasPrefix(typeUpper, "DECIMAL"), strings.HasPrefix(typeUpper, "NUMERIC"), strings.HasPrefix(typeUpper, "MONEY"):
						return &typeInfo{goType: "decimal.Decimal", isDecimal: true, isNumeric: true}
					case strings.HasPrefix(typeUpper, "FLOAT"), strings.HasPrefix(typeUpper, "REAL"):
						return &typeInfo{goType: "float64", isNumeric: true}
					default:
						return &typeInfo{goType: "string", isString: true}
					}
				}
			}
			return &typeInfo{goType: "string", isString: true}
		case "query":
			return &typeInfo{goType: "string", isString: true}
		case "exist":
			return &typeInfo{goType: "bool", isBool: true}
		case "nodes":
			return &typeInfo{goType: "[]map[string]interface{}"}
		case "modify":
			return &typeInfo{goType: "string", isString: true}
		default:
			return &typeInfo{goType: "interface{}"}
		}
	case *ast.CaseExpression:
		// CASE expression type is determined by the result expressions
		goType := t.inferCaseResultType(e)
		switch goType {
		case "int64", "int32":
			return &typeInfo{goType: goType, isNumeric: true}
		case "float64":
			return &typeInfo{goType: "float64", isNumeric: true}
		case "decimal.Decimal":
			return &typeInfo{goType: "decimal.Decimal", isDecimal: true, isNumeric: true}
		case "string":
			return &typeInfo{goType: "string", isString: true}
		case "bool":
			return &typeInfo{goType: "bool", isBool: true}
		default:
			return &typeInfo{goType: goType}
		}
	}

	// Default: unknown type
	return &typeInfo{goType: "interface{}"}
}

// inferFunctionReturnType returns type info for known T-SQL functions.
func (t *transpiler) inferFunctionReturnType(funcName string) *typeInfo {
	switch normaliseTypeName(funcName) {
	// String length/position functions
	case "LEN", "DATALENGTH", "CHARINDEX", "PATINDEX", "ASCII", "UNICODE":
		return &typeInfo{goType: "int32", isNumeric: true}
	// String manipulation functions
	case "UPPER", "LOWER", "LTRIM", "RTRIM", "TRIM", "SUBSTRING", "LEFT", "RIGHT", "REPLACE", "REPLICATE", "REVERSE", "CONCAT", "CONCAT_WS", "NCHAR", "CHAR":
		return &typeInfo{goType: "string", isString: true}
	// Math functions
	case "ABS", "CEILING", "CEIL", "FLOOR", "ROUND", "POWER", "SQRT", "SIGN":
		return &typeInfo{goType: "float64", isNumeric: true}
	// Date/time functions
	case "GETDATE", "SYSDATETIME", "GETUTCDATE", "SYSUTCDATETIME", "DATEADD":
		return &typeInfo{goType: "time.Time", isDateTime: true}
	case "DATEDIFF", "YEAR", "MONTH", "DAY", "DATEPART":
		return &typeInfo{goType: "int32", isNumeric: true}
	// JSON functions
	case "JSON_VALUE", "JSON_QUERY", "JSON_MODIFY":
		return &typeInfo{goType: "string", isString: true}
	case "ISJSON":
		return &typeInfo{goType: "int32", isNumeric: true}
	// XML functions
	case "XML_VALUE", "XMLVALUE":
		return &typeInfo{goType: "interface{}"}
	case "XML_QUERY", "XMLQUERY":
		return &typeInfo{goType: "string", isString: true}
	case "XML_EXIST", "XMLEXIST":
		return &typeInfo{goType: "int", isNumeric: true}
	// Ranking window functions - always return int64
	case "ROW_NUMBER", "RANK", "DENSE_RANK", "NTILE":
		return &typeInfo{goType: "int64", isNumeric: true}
	// Percentage window functions - return float64
	case "PERCENT_RANK", "CUME_DIST":
		return &typeInfo{goType: "float64", isNumeric: true}
	// Aggregate functions (when used as window functions, type depends on argument)
	case "COUNT":
		return &typeInfo{goType: "int64", isNumeric: true}
	case "SUM", "AVG", "MIN", "MAX":
		// These need argument type - handled specially in inferType
		return &typeInfo{goType: "decimal.Decimal", isDecimal: true, isNumeric: true}
	default:
		return &typeInfo{goType: "interface{}"}
	}
}

// inferWindowFunctionType returns type info for window functions (functions with OVER clause).
func (t *transpiler) inferWindowFunctionType(funcName string, fc *ast.FunctionCall) *typeInfo {
	switch funcName {
	// Ranking functions - always return int64
	case "ROW_NUMBER", "RANK", "DENSE_RANK", "NTILE":
		return &typeInfo{goType: "int64", isNumeric: true}
	
	// Percentage functions - return float64
	case "PERCENT_RANK", "CUME_DIST":
		return &typeInfo{goType: "float64", isNumeric: true}
	
	// Navigation functions - return type matches first argument
	case "LEAD", "LAG", "FIRST_VALUE", "LAST_VALUE", "NTH_VALUE":
		if len(fc.Arguments) > 0 {
			argType := t.inferType(fc.Arguments[0])
			if argType != nil && argType.goType != "" {
				return argType
			}
		}
		return &typeInfo{goType: "interface{}"}
	
	// Aggregate functions with OVER - COUNT always returns int64
	case "COUNT":
		return &typeInfo{goType: "int64", isNumeric: true}
	
	// SUM, AVG, MIN, MAX - return type matches argument
	case "SUM", "AVG":
		if len(fc.Arguments) > 0 {
			argType := t.inferType(fc.Arguments[0])
			if argType != nil {
				// SUM/AVG of integers typically returns the same or larger type
				if argType.isDecimal {
					return &typeInfo{goType: "decimal.Decimal", isDecimal: true, isNumeric: true}
				}
				if argType.isNumeric {
					return &typeInfo{goType: "decimal.Decimal", isDecimal: true, isNumeric: true}
				}
			}
		}
		return &typeInfo{goType: "decimal.Decimal", isDecimal: true, isNumeric: true}
	
	case "MIN", "MAX":
		if len(fc.Arguments) > 0 {
			argType := t.inferType(fc.Arguments[0])
			if argType != nil && argType.goType != "" {
				return argType
			}
		}
		return &typeInfo{goType: "interface{}"}
	
	default:
		// Fall back to regular function type inference
		return t.inferFunctionReturnType(funcName)
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

// promoteNumericType returns the wider of two numeric types for arithmetic.
// Follows T-SQL implicit conversion rules: smaller types promote to larger.
func (t *transpiler) promoteNumericType(type1, type2 string) string {
	// Priority order: float64 > int64 > int32 > int16 > uint8
	priority := map[string]int{
		"uint8":   1,
		"int16":   2,
		"int32":   3,
		"int64":   4,
		"float64": 5,
	}

	p1 := priority[type1]
	p2 := priority[type2]

	if p1 >= p2 {
		return type1
	}
	return type2
}

// isIntegerLiteral checks if an expression is an integer literal,
// including negative literals like -1 which are PrefixExpressions.
func isIntegerLiteral(expr ast.Expression) bool {
	if _, ok := expr.(*ast.IntegerLiteral); ok {
		return true
	}
	// Check for unary minus on an integer literal (e.g., -1)
	if prefix, ok := expr.(*ast.PrefixExpression); ok {
		if prefix.Operator == "-" {
			if _, ok := prefix.Right.(*ast.IntegerLiteral); ok {
				return true
			}
		}
	}
	return false
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

	// Check for user-defined functions first
	if udf, ok := t.userFunctions[strings.ToLower(funcName)]; ok {
		return fmt.Sprintf("%s(%s)", udf.goName, strings.Join(args, ", ")), nil
	}

	// Map common T-SQL functions to Go equivalents
	switch funcName {
	case "LEN":
		t.imports["unicode/utf8"] = true
		if len(args) == 1 {
			return fmt.Sprintf("int32(utf8.RuneCountInString(%s))", args[0]), nil
		}

	case "DATALENGTH":
		if len(args) == 1 {
			return fmt.Sprintf("int32(len(%s))", args[0]), nil
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
			return fmt.Sprintf("int32(strings.Index(%s, %s) + 1)", args[1], args[0]), nil
		}

	case "ASCII":
		// ASCII(char) returns the ASCII code of the first character
		if len(args) == 1 {
			return fmt.Sprintf("int32((%s)[0])", args[0]), nil
		}

	case "UNICODE":
		// UNICODE(char) returns the Unicode code point of the first character
		if len(args) == 1 {
			return fmt.Sprintf("int32([]rune(%s)[0])", args[0]), nil
		}

	case "NCHAR", "CHAR":
		// NCHAR(n) / CHAR(n) returns the character for the given code point
		if len(args) == 1 {
			return fmt.Sprintf("string(rune(%s))", args[0]), nil
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
		// ISNULL(a, b) -> returns a if not null, else b
		// For strings: check if empty
		// For value types: use the value (Go doesn't have null for value types)
		if len(args) == 2 {
			argType := t.inferType(fc.Arguments[0])
			if argType.isString {
				return fmt.Sprintf("func() string { if %s != \"\" { return %s }; return %s }()", args[0], args[0], args[1]), nil
			}
			// For value types, just return the first value
			// In a real scenario, you'd use sql.Null* types
			return args[0], nil
		}

	case "COALESCE":
		// COALESCE returns first non-null value
		// For strings: return first non-empty, or last value as default
		if len(args) > 0 {
			argType := t.inferType(fc.Arguments[0])
			if argType.isString && len(args) == 2 {
				return fmt.Sprintf("func() string { if %s != \"\" { return %s }; return %s }()", args[0], args[0], args[1]), nil
			}
			// For other types or >2 args, return first value (simplified)
			return args[0], nil
		}

	case "NULLIF":
		// NULLIF(a, b) -> if a == b { nil } else { a }
		if len(args) == 2 {
			return fmt.Sprintf("func() interface{} { if %s == %s { return nil }; return %s }()", args[0], args[1], args[0]), nil
		}

	case "ABS":
		if len(args) == 1 {
			argType := t.inferType(fc.Arguments[0])
			if argType.isDecimal {
				return fmt.Sprintf("%s.Abs()", args[0]), nil
			}
			t.imports["math"] = true
			return fmt.Sprintf("math.Abs(float64(%s))", args[0]), nil
		}

	case "CEILING", "CEIL":
		if len(args) == 1 {
			argType := t.inferType(fc.Arguments[0])
			if argType.isDecimal {
				return fmt.Sprintf("%s.Ceil()", args[0]), nil
			}
			t.imports["math"] = true
			return fmt.Sprintf("math.Ceil(float64(%s))", args[0]), nil
		}

	case "FLOOR":
		if len(args) == 1 {
			argType := t.inferType(fc.Arguments[0])
			if argType.isDecimal {
				return fmt.Sprintf("%s.Floor()", args[0]), nil
			}
			t.imports["math"] = true
			return fmt.Sprintf("math.Floor(float64(%s))", args[0]), nil
		}

	case "ROUND":
		if len(args) >= 1 {
			argType := t.inferType(fc.Arguments[0])
			if argType.isDecimal {
				if len(args) == 1 {
					return fmt.Sprintf("%s.Round(0)", args[0]), nil
				}
				return fmt.Sprintf("%s.Round(%s)", args[0], args[1]), nil
			}
			t.imports["math"] = true
			if len(args) == 1 {
				return fmt.Sprintf("math.Round(%s)", args[0]), nil
			}
			// ROUND(x, decimals) - more complex
			return fmt.Sprintf("math.Round(%s*math.Pow(10, float64(%s)))/math.Pow(10, float64(%s))", args[0], args[1], args[1]), nil
		}

	case "POWER":
		if len(args) == 2 {
			argType := t.inferType(fc.Arguments[0])
			if argType.isDecimal {
				return fmt.Sprintf("%s.Pow(decimal.NewFromInt(int64(%s)))", args[0], args[1]), nil
			}
			t.imports["math"] = true
			return fmt.Sprintf("math.Pow(float64(%s), float64(%s))", args[0], args[1]), nil
		}

	case "SQRT":
		if len(args) == 1 {
			argType := t.inferType(fc.Arguments[0])
			if argType.isDecimal {
				// decimal doesn't have Sqrt, convert to float and back
				t.imports["math"] = true
				t.imports["github.com/shopspring/decimal"] = true
				return fmt.Sprintf("decimal.NewFromFloat(math.Sqrt(%s.InexactFloat64()))", args[0]), nil
			}
			t.imports["math"] = true
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

	case "DATEPART":
		// DATEPART(interval, date)
		t.imports["time"] = true
		if len(args) == 2 {
			interval := strings.ToLower(strings.Trim(args[0], "\""))
			return t.transpileDatePart(interval, args[1])
		}

	case "NEWID":
		return t.transpileNewid()

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
		// Could potentially parse error codes from specific error types
		return "0", nil

	case "ERROR_SEVERITY", "ERROR_STATE":
		// No direct equivalent in Go - return 0
		return "0", nil

	case "ERROR_PROCEDURE":
		// Return the current procedure name
		if t.currentProcName != "" {
			return fmt.Sprintf("%q", t.currentProcName), nil
		}
		return "\"\"", nil

	case "ERROR_LINE":
		// Use runtime.Caller to get approximate line info
		t.imports["runtime"] = true
		return "func() int { _, _, line, _ := runtime.Caller(0); return line }()", nil

	// Identity/Sequence functions
	case "SCOPE_IDENTITY", "@@IDENTITY":
		return t.transpileIdentityFunction()

	case "IDENT_CURRENT":
		// IDENT_CURRENT('tablename') - not directly translatable
		// Generate a placeholder
		if len(args) == 1 {
			return fmt.Sprintf("0 /* TODO: IDENT_CURRENT(%s) - implement table-specific identity retrieval */", args[0]), nil
		}

	case "OBJECT_ID":
		// OBJECT_ID('name') checks if database object exists
		// For temp tables: OBJECT_ID('tempdb..#tableName') checks temp table existence
		if len(args) == 1 {
			objName := strings.Trim(args[0], "\"")
			// Check for temp table pattern
			if strings.Contains(objName, "#") {
				// Extract temp table name
				parts := strings.Split(objName, "#")
				if len(parts) >= 2 {
					tableName := "#" + parts[len(parts)-1]
					return fmt.Sprintf("tempTables.Exists(%q) /* OBJECT_ID check for temp table */", tableName), nil
				}
			}
			// For other objects, generate a comment
			return fmt.Sprintf("nil /* TODO: OBJECT_ID(%s) - check if object exists in database */", args[0]), nil
		}
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

func (t *transpiler) transpileDatePart(interval, date string) (string, error) {
	switch strings.ToUpper(interval) {
	case "YEAR", "YY", "YYYY":
		return fmt.Sprintf("int32((%s).Year())", date), nil
	case "MONTH", "MM", "M":
		return fmt.Sprintf("int32((%s).Month())", date), nil
	case "DAY", "DD", "D":
		return fmt.Sprintf("int32((%s).Day())", date), nil
	case "HOUR", "HH":
		return fmt.Sprintf("int32((%s).Hour())", date), nil
	case "MINUTE", "MI", "N":
		return fmt.Sprintf("int32((%s).Minute())", date), nil
	case "SECOND", "SS", "S":
		return fmt.Sprintf("int32((%s).Second())", date), nil
	case "WEEKDAY", "DW", "W":
		// T-SQL: Sunday=1, Monday=2, ... Saturday=7
		// Go: Sunday=0, Monday=1, ... Saturday=6
		return fmt.Sprintf("int32((%s).Weekday() + 1)", date), nil
	case "DAYOFYEAR", "DY", "Y":
		return fmt.Sprintf("int32((%s).YearDay())", date), nil
	case "QUARTER", "QQ", "Q":
		return fmt.Sprintf("int32(((%s).Month()-1)/3 + 1)", date), nil
	default:
		return "", fmt.Errorf("unsupported DATEPART interval: %s", interval)
	}
}

func (t *transpiler) transpileCaseExpression(c *ast.CaseExpression) (string, error) {
	var out strings.Builder

	// Infer the result type from the first WHEN clause
	resultType := t.inferCaseResultType(c)
	
	out.WriteString(fmt.Sprintf("func() %s {\n", resultType))

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
			out.WriteString(fmt.Sprintf("\tdefault:\n\t\treturn %s\n", t.zeroValueFor(resultType)))
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
			out.WriteString(fmt.Sprintf(" else {\n\t\treturn %s\n\t}", t.zeroValueFor(resultType)))
		}
	}

	out.WriteString("\n}()")
	return out.String(), nil
}

// inferCaseResultType looks at CASE WHEN results to determine the return type
func (t *transpiler) inferCaseResultType(c *ast.CaseExpression) string {
	// Look at the first WHEN clause result
	if len(c.WhenClauses) > 0 {
		firstResult := c.WhenClauses[0].Result
		return t.inferExpressionType(firstResult)
	}
	
	// Look at ELSE clause
	if c.ElseClause != nil {
		return t.inferExpressionType(c.ElseClause)
	}
	
	return "interface{}"
}

// inferExpressionType returns a Go type string for an expression
func (t *transpiler) inferExpressionType(expr ast.Expression) string {
	if expr == nil {
		return "interface{}"
	}
	
	switch e := expr.(type) {
	case *ast.IntegerLiteral:
		return "int64"
	case *ast.FloatLiteral:
		return "float64"
	case *ast.StringLiteral:
		return "string"
	case *ast.Variable:
		// Look up variable type from symbols
		varName := strings.TrimPrefix(e.Name, "@")
		if sym := t.symbols.lookup(varName); sym != nil {
			return sym.goType
		}
		// Infer from name patterns
		upperName := strings.ToUpper(varName)
		if strings.Contains(upperName, "DECIMAL") || strings.Contains(upperName, "AMOUNT") ||
			strings.Contains(upperName, "PRICE") || strings.Contains(upperName, "TOTAL") ||
			strings.Contains(upperName, "COST") {
			t.imports["github.com/shopspring/decimal"] = true
			return "decimal.Decimal"
		}
		return "interface{}"
	case *ast.InfixExpression:
		// For arithmetic, infer from operands
		leftType := t.inferExpressionType(e.Left)
		rightType := t.inferExpressionType(e.Right)
		
		// If either is decimal, result is decimal
		if leftType == "decimal.Decimal" || rightType == "decimal.Decimal" {
			t.imports["github.com/shopspring/decimal"] = true
			return "decimal.Decimal"
		}
		// If either is float, result is float
		if leftType == "float64" || rightType == "float64" {
			return "float64"
		}
		// If both are int, result is int
		if leftType == "int64" && rightType == "int64" {
			return "int64"
		}
		return "float64" // Default for arithmetic
	case *ast.FunctionCall:
		// Certain functions have known return types
		funcName := strings.ToUpper(e.Function.String())
		switch funcName {
		case "COUNT", "LEN", "DATALENGTH":
			return "int64"
		case "SUM", "AVG", "MIN", "MAX":
			// Aggregate functions return same type as input, default to decimal
			if len(e.Arguments) > 0 {
				return t.inferExpressionType(e.Arguments[0])
			}
			return "float64"
		case "ROUND", "FLOOR", "CEILING", "ABS":
			return "float64"
		}
	}
	
	// Use existing inferType for declared variables
	typeInfo := t.inferType(expr)
	if typeInfo.isDecimal {
		t.imports["github.com/shopspring/decimal"] = true
		return "decimal.Decimal"
	}
	if typeInfo.isNumeric {
		// Check if it's a float type by looking at goType
		if typeInfo.goType == "float32" || typeInfo.goType == "float64" {
			return "float64"
		}
		return "int64"
	}
	if typeInfo.isString {
		return "string"
	}
	if typeInfo.isBool {
		return "bool"
	}
	
	return "interface{}"
}

// zeroValueFor returns the zero value for a Go type
func (t *transpiler) zeroValueFor(goType string) string {
	switch goType {
	case "int32", "int64", "int":
		return "0"
	case "float32", "float64":
		return "0.0"
	case "string":
		return `""`
	case "bool":
		return "false"
	case "decimal.Decimal":
		t.imports["github.com/shopspring/decimal"] = true
		return "decimal.Zero"
	default:
		return "nil"
	}
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

	// Get source type to handle string-to-numeric conversions
	sourceType := t.inferType(c.Expression)

	// Handle string-to-numeric conversions (need strconv)
	if sourceType.isString {
		switch goType {
		case "int32":
			t.imports["strconv"] = true
			// strconv.Atoi returns (int, error) - we wrap in a helper expression
			// Using ParseInt for explicit int32 control
			return fmt.Sprintf("func() int32 { v, _ := strconv.ParseInt(%s, 10, 32); return int32(v) }()", expr), nil
		case "int64":
			t.imports["strconv"] = true
			return fmt.Sprintf("func() int64 { v, _ := strconv.ParseInt(%s, 10, 64); return v }()", expr), nil
		case "float64":
			t.imports["strconv"] = true
			return fmt.Sprintf("func() float64 { v, _ := strconv.ParseFloat(%s, 64); return v }()", expr), nil
		case "decimal.Decimal":
			t.imports["github.com/shopspring/decimal"] = true
			return fmt.Sprintf("decimal.RequireFromString(%s)", expr), nil
		case "bool":
			t.imports["strings"] = true
			// Handle "true", "false", "1", "0" string values
			return fmt.Sprintf("(strings.ToLower(%s) == \"true\" || %s == \"1\")", expr, expr), nil
		case "time.Time":
			t.imports["time"] = true
			// Try common date formats
			return fmt.Sprintf("func() time.Time { t, _ := time.Parse(\"2006-01-02\", %s); return t }()", expr), nil
		}
	}

	// Handle decimal-to-numeric conversions
	if sourceType.isDecimal {
		switch goType {
		case "int32":
			return fmt.Sprintf("int32(%s.IntPart())", expr), nil
		case "int64":
			return fmt.Sprintf("%s.IntPart()", expr), nil
		case "float64":
			return fmt.Sprintf("%s.InexactFloat64()", expr), nil
		case "string":
			return fmt.Sprintf("%s.String()", expr), nil
		}
	}

	// Simple type conversion for non-string sources
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

	// Get source type to handle string-to-numeric conversions
	sourceType := t.inferType(c.Expression)

	// Handle string-to-numeric conversions (need strconv)
	if sourceType.isString {
		switch goType {
		case "int32":
			t.imports["strconv"] = true
			return fmt.Sprintf("func() int32 { v, _ := strconv.ParseInt(%s, 10, 32); return int32(v) }()", expr), nil
		case "int64":
			t.imports["strconv"] = true
			return fmt.Sprintf("func() int64 { v, _ := strconv.ParseInt(%s, 10, 64); return v }()", expr), nil
		case "float64":
			t.imports["strconv"] = true
			return fmt.Sprintf("func() float64 { v, _ := strconv.ParseFloat(%s, 64); return v }()", expr), nil
		case "decimal.Decimal":
			t.imports["github.com/shopspring/decimal"] = true
			return fmt.Sprintf("decimal.RequireFromString(%s)", expr), nil
		}
	}

	// Handle decimal-to-numeric conversions
	if sourceType.isDecimal {
		switch goType {
		case "int32":
			return fmt.Sprintf("int32(%s.IntPart())", expr), nil
		case "int64":
			return fmt.Sprintf("%s.IntPart()", expr), nil
		case "float64":
			return fmt.Sprintf("%s.InexactFloat64()", expr), nil
		case "string":
			return fmt.Sprintf("%s.String()", expr), nil
		}
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

	// For string types, NULL check becomes empty string check
	exprType := t.inferType(e.Expr)
	if exprType.isString {
		if e.Not {
			return fmt.Sprintf("(%s != \"\")", expr), nil
		}
		return fmt.Sprintf("(%s == \"\")", expr), nil
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

// transpileMethodCallExpression handles XML method calls like @xml.value('/xpath', 'type')
// and also user-defined function calls like dbo.fn_GenerateTransferNumber()
func (t *transpiler) transpileMethodCallExpression(e *ast.MethodCallExpression) (string, error) {
	// Check if this is a user-defined function call (e.g., dbo.fn_MyFunction())
	// The "Object" would be a schema name like "dbo" and "MethodName" is the function name
	if id, ok := e.Object.(*ast.Identifier); ok {
		schemaName := strings.ToLower(id.Value)
		if schemaName == "dbo" || schemaName == "schema" {
			// Check if this is a user-defined function
			funcNameLower := strings.ToLower(e.MethodName)
			if udf, ok := t.userFunctions[funcNameLower]; ok {
				var args []string
				for _, arg := range e.Arguments {
					a, err := t.transpileExpression(arg)
					if err != nil {
						return "", err
					}
					args = append(args, a)
				}
				return fmt.Sprintf("%s(%s)", udf.goName, strings.Join(args, ", ")), nil
			}
		}
	}

	// Get the object (variable) being called on
	obj, err := t.transpileExpression(e.Object)
	if err != nil {
		return "", err
	}

	// Handle XML methods
	switch strings.ToLower(e.MethodName) {
	case "value":
		// @xml.value('/xpath', 'type') -> XmlValueType(xml, "/xpath")
		if len(e.Arguments) < 2 {
			return "", fmt.Errorf("XML .value() requires 2 arguments (xpath, type)")
		}
		xpath, err := t.transpileExpression(e.Arguments[0])
		if err != nil {
			return "", err
		}
		typeName, err := t.transpileExpression(e.Arguments[1])
		if err != nil {
			return "", err
		}
		
		// Generate type-specific wrapper based on target type
		typeUpper := strings.ToUpper(strings.Trim(typeName, "\"'"))
		switch {
		case strings.HasPrefix(typeUpper, "INT"):
			t.imports["strconv"] = true
			return fmt.Sprintf("func() int32 { s := XmlValueString(%s, %s); if s == \"\" { return 0 }; v, _ := strconv.ParseInt(s, 10, 32); return int32(v) }()", obj, xpath), nil
		case strings.HasPrefix(typeUpper, "BIGINT"):
			t.imports["strconv"] = true
			return fmt.Sprintf("func() int64 { s := XmlValueString(%s, %s); if s == \"\" { return 0 }; v, _ := strconv.ParseInt(s, 10, 64); return v }()", obj, xpath), nil
		case strings.HasPrefix(typeUpper, "BIT"):
			t.imports["strings"] = true
			return fmt.Sprintf("(XmlValueString(%s, %s) == \"1\" || strings.ToLower(XmlValueString(%s, %s)) == \"true\")", obj, xpath, obj, xpath), nil
		case strings.HasPrefix(typeUpper, "DECIMAL") || strings.HasPrefix(typeUpper, "NUMERIC") || strings.HasPrefix(typeUpper, "MONEY"):
			t.imports["github.com/shopspring/decimal"] = true
			return fmt.Sprintf("func() decimal.Decimal { s := XmlValueString(%s, %s); if s == \"\" { return decimal.Zero }; v, _ := decimal.NewFromString(s); return v }()", obj, xpath), nil
		case strings.HasPrefix(typeUpper, "FLOAT") || strings.HasPrefix(typeUpper, "REAL"):
			t.imports["strconv"] = true
			return fmt.Sprintf("func() float64 { s := XmlValueString(%s, %s); if s == \"\" { return 0 }; v, _ := strconv.ParseFloat(s, 64); return v }()", obj, xpath), nil
		case strings.HasPrefix(typeUpper, "DATE") || strings.HasPrefix(typeUpper, "DATETIME") || strings.HasPrefix(typeUpper, "DATETIME2"):
			t.imports["time"] = true
			return fmt.Sprintf("func() time.Time { s := XmlValueString(%s, %s); if s == \"\" { return time.Time{} }; t, _ := time.Parse(\"2006-01-02\", s); return t }()", obj, xpath), nil
		default:
			// String types (NVARCHAR, VARCHAR, CHAR, etc.)
			return fmt.Sprintf("XmlValueString(%s, %s)", obj, xpath), nil
		}

	case "query":
		// @xml.query('/xpath') -> XmlQuery(xml, "/xpath")
		if len(e.Arguments) < 1 {
			return "", fmt.Errorf("XML .query() requires 1 argument (xpath)")
		}
		xpath, err := t.transpileExpression(e.Arguments[0])
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("XmlQuery(%s, %s)", obj, xpath), nil

	case "exist":
		// @xml.exist('/xpath') -> XmlExist(xml, "/xpath")
		if len(e.Arguments) < 1 {
			return "", fmt.Errorf("XML .exist() requires 1 argument (xpath)")
		}
		xpath, err := t.transpileExpression(e.Arguments[0])
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("XmlExist(%s, %s)", obj, xpath), nil

	case "nodes":
		// @xml.nodes('/xpath') -> XmlNodes(xml, "/xpath")
		if len(e.Arguments) < 1 {
			return "", fmt.Errorf("XML .nodes() requires 1 argument (xpath)")
		}
		xpath, err := t.transpileExpression(e.Arguments[0])
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("XmlNodes(%s, %s)", obj, xpath), nil

	case "modify":
		// @xml.modify('...') -> XmlModify(xml, "...")
		if len(e.Arguments) < 1 {
			return "", fmt.Errorf("XML .modify() requires 1 argument (dml)")
		}
		dml, err := t.transpileExpression(e.Arguments[0])
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("XmlModify(%s, %s)", obj, dml), nil

	default:
		return "", fmt.Errorf("unsupported method: %s", e.MethodName)
	}
}

// transpileIdentityFunction handles SCOPE_IDENTITY() and @@IDENTITY
func (t *transpiler) transpileIdentityFunction() (string, error) {
	if !t.dmlEnabled {
		// Non-DML mode: generate a placeholder function call
		return "ScopeIdentity() /* TODO: implement or use DML mode */", nil
	}

	switch t.dmlConfig.SequenceMode {
	case "uuid":
		// UUID mode doesn't use identity - this is likely an error in the source
		return "0 /* WARNING: SCOPE_IDENTITY() called but using UUID mode */", nil
	case "stub":
		return "0 /* TODO: implement SCOPE_IDENTITY() - capture LastInsertId() after INSERT */", nil
	case "db", "":
		// In database mode, SCOPE_IDENTITY() should use the result from the previous INSERT.
		// The developer needs to capture result.LastInsertId() after their INSERT.
		// We generate a variable reference that they need to set up.
		return "lastInsertId /* set this from result.LastInsertId() after INSERT */", nil
	default:
		return "lastInsertId /* set this from result.LastInsertId() after INSERT */", nil
	}
}

// transpileNextValueFor handles NEXT VALUE FOR <sequence> expressions
func (t *transpiler) transpileNextValueFor(seqName string) (string, error) {
	if !t.dmlEnabled {
		return "", fmt.Errorf("NEXT VALUE FOR requires DML mode")
	}

	switch t.dmlConfig.SequenceMode {
	case "uuid":
		t.imports["github.com/google/uuid"] = true
		return "uuid.New().String()", nil
	case "stub":
		return fmt.Sprintf("0 /* TODO: implement NEXT VALUE FOR %s */", seqName), nil
	case "db", "":
		// Database-specific sequence handling
		// Generate an inline query to fetch the next sequence value
		switch t.dmlConfig.SQLDialect {
		case "postgres":
			// Postgres: SELECT nextval('sequence_name')
			seqLower := strings.ToLower(seqName)
			return fmt.Sprintf("func() int64 { var id int64; %s.QueryRowContext(ctx, \"SELECT nextval('%s')\").Scan(&id); return id }()",
				t.dmlConfig.StoreVar, seqLower), nil
		case "mysql":
			// MySQL doesn't have sequences - use AUTO_INCREMENT
			return fmt.Sprintf("0 /* MySQL: no sequences - use AUTO_INCREMENT and LastInsertId() after INSERT */"), nil
		case "sqlserver":
			// SQL Server: keep the expression for passthrough mode
			return fmt.Sprintf("func() int64 { var id int64; %s.QueryRowContext(ctx, \"SELECT NEXT VALUE FOR %s\").Scan(&id); return id }()",
				t.dmlConfig.StoreVar, seqName), nil
		default:
			return fmt.Sprintf("0 /* TODO: NEXT VALUE FOR %s - implement for dialect %s */", seqName, t.dmlConfig.SQLDialect), nil
		}
	default:
		return fmt.Sprintf("0 /* TODO: NEXT VALUE FOR %s */", seqName), nil
	}
}

// transpileNewid handles NEWID() based on the configured mode
func (t *transpiler) transpileNewid() (string, error) {
	if !t.dmlEnabled {
		return "", fmt.Errorf("NEWID() requires DML mode (--dml)")
	}

	mode := t.dmlConfig.NewidMode
	if mode == "" {
		mode = "app" // Default to app-side UUID
	}

	switch mode {
	case "app":
		// Generate UUID application-side using google/uuid
		t.imports["github.com/google/uuid"] = true
		return "uuid.New().String()", nil

	case "db":
		// Use database-specific UUID function
		switch t.dmlConfig.SQLDialect {
		case "postgres":
			return fmt.Sprintf("func() string { var id string; %s.QueryRowContext(ctx, \"SELECT gen_random_uuid()::text\").Scan(&id); return id }()",
				t.dmlConfig.StoreVar), nil
		case "mysql":
			return fmt.Sprintf("func() string { var id string; %s.QueryRowContext(ctx, \"SELECT UUID()\").Scan(&id); return id }()",
				t.dmlConfig.StoreVar), nil
		case "sqlite":
			// SQLite lacks native UUID - fall back to app-side
			t.imports["github.com/google/uuid"] = true
			return "uuid.New().String() /* SQLite: no native UUID, using app-side */", nil
		case "sqlserver":
			return fmt.Sprintf("func() string { var id string; %s.QueryRowContext(ctx, \"SELECT NEWID()\").Scan(&id); return id }()",
				t.dmlConfig.StoreVar), nil
		default:
			// Unknown dialect - fall back to app-side
			t.imports["github.com/google/uuid"] = true
			return "uuid.New().String()", nil
		}

	case "grpc":
		// Call gRPC ID service
		if t.dmlConfig.IDServiceVar == "" {
			return "", fmt.Errorf("NEWID() with --newid=grpc requires --id-service=<client>")
		}
		return fmt.Sprintf("%s.GenerateUUID(ctx)", t.dmlConfig.IDServiceVar), nil

	case "mock":
		// Generate predictable sequential UUIDs for testing
		t.imports["github.com/ha1tch/tgpiler/tsqlruntime"] = true
		return "tsqlruntime.NextMockUUID()", nil

	case "stub":
		// Generate TODO placeholder
		return "\"\" /* TODO: implement NEWID() */", nil

	default:
		return "", fmt.Errorf("unknown --newid mode: %s (valid: app, db, grpc, mock, stub)", mode)
	}
}
