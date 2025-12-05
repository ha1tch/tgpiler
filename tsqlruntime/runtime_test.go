package tsqlruntime

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestValueTypes(t *testing.T) {
	tests := []struct {
		name     string
		value    Value
		asInt    int64
		asFloat  float64
		asString string
		asBool   bool
	}{
		{"int", NewInt(42), 42, 42.0, "42", true},
		{"zero", NewInt(0), 0, 0.0, "0", false},
		{"negative", NewInt(-10), -10, -10.0, "-10", true},
		{"float", NewFloat(3.14), 3, 3.14, "3.14", true},
		{"string", NewVarChar("hello", -1), 0, 0.0, "hello", false},
		{"bool true", NewBit(true), 1, 1.0, "1", true},
		{"bool false", NewBit(false), 0, 0.0, "0", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.value.AsInt(); got != tt.asInt {
				t.Errorf("AsInt() = %v, want %v", got, tt.asInt)
			}
			if got := tt.value.AsFloat(); got != tt.asFloat {
				t.Errorf("AsFloat() = %v, want %v", got, tt.asFloat)
			}
			if got := tt.value.AsString(); got != tt.asString {
				t.Errorf("AsString() = %v, want %v", got, tt.asString)
			}
			if got := tt.value.AsBool(); got != tt.asBool {
				t.Errorf("AsBool() = %v, want %v", got, tt.asBool)
			}
		})
	}
}

func TestValueArithmetic(t *testing.T) {
	tests := []struct {
		name   string
		a, b   Value
		add    int64
		sub    int64
		mul    int64
		div    int64
	}{
		{"ints", NewInt(10), NewInt(3), 13, 7, 30, 3},
		{"mixed", NewInt(10), NewFloat(2.5), 12, 7, 25, 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.a.Add(tt.b).AsInt(); got != tt.add {
				t.Errorf("Add = %v, want %v", got, tt.add)
			}
			if got := tt.a.Sub(tt.b).AsInt(); got != tt.sub {
				t.Errorf("Sub = %v, want %v", got, tt.sub)
			}
			if got := tt.a.Mul(tt.b).AsInt(); got != tt.mul {
				t.Errorf("Mul = %v, want %v", got, tt.mul)
			}
			if got := tt.a.Div(tt.b).AsInt(); got != tt.div {
				t.Errorf("Div = %v, want %v", got, tt.div)
			}
		})
	}
}

func TestValueComparison(t *testing.T) {
	tests := []struct {
		name string
		a, b Value
		eq   bool
		lt   bool
		gt   bool
	}{
		{"equal ints", NewInt(5), NewInt(5), true, false, false},
		{"lt ints", NewInt(3), NewInt(5), false, true, false},
		{"gt ints", NewInt(7), NewInt(5), false, false, true},
		{"equal strings", NewVarChar("abc", -1), NewVarChar("abc", -1), true, false, false},
		{"lt strings", NewVarChar("abc", -1), NewVarChar("xyz", -1), false, true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.a.Equals(tt.b).AsBool(); got != tt.eq {
				t.Errorf("Equals = %v, want %v", got, tt.eq)
			}
			if got := tt.a.LessThan(tt.b).AsBool(); got != tt.lt {
				t.Errorf("LessThan = %v, want %v", got, tt.lt)
			}
			if got := tt.a.GreaterThan(tt.b).AsBool(); got != tt.gt {
				t.Errorf("GreaterThan = %v, want %v", got, tt.gt)
			}
		})
	}
}

func TestNullHandling(t *testing.T) {
	null := Null(TypeInt)
	five := NewInt(5)

	// NULL arithmetic produces NULL
	if !null.Add(five).IsNull {
		t.Error("NULL + 5 should be NULL")
	}
	if !five.Add(null).IsNull {
		t.Error("5 + NULL should be NULL")
	}

	// NULL comparison produces NULL
	result := null.Equals(five)
	if !result.IsNull {
		t.Error("NULL = 5 should be NULL")
	}

	// NULL AND FALSE = FALSE (SQL three-valued logic)
	falseVal := NewBit(false)
	if null.And(falseVal).AsBool() != false {
		t.Error("NULL AND FALSE should be FALSE")
	}

	// NULL OR TRUE = TRUE
	trueVal := NewBit(true)
	if null.Or(trueVal).AsBool() != true {
		t.Error("NULL OR TRUE should be TRUE")
	}
}

func TestDecimalArithmetic(t *testing.T) {
	d1 := NewDecimal(decimal.NewFromFloat(10.5), 18, 2)
	d2 := NewDecimal(decimal.NewFromFloat(3.25), 18, 2)

	sum := d1.Add(d2)
	if sum.AsDecimal().String() != "13.75" {
		t.Errorf("10.5 + 3.25 = %s, want 13.75", sum.AsDecimal().String())
	}

	diff := d1.Sub(d2)
	if diff.AsDecimal().String() != "7.25" {
		t.Errorf("10.5 - 3.25 = %s, want 7.25", diff.AsDecimal().String())
	}

	prod := d1.Mul(d2)
	expected := decimal.NewFromFloat(10.5 * 3.25)
	if prod.AsDecimal().Sub(expected).Abs().GreaterThan(decimal.NewFromFloat(0.01)) {
		t.Errorf("10.5 * 3.25 = %s, want ~34.125", prod.AsDecimal().String())
	}
}

func TestCast(t *testing.T) {
	tests := []struct {
		name     string
		value    Value
		toType   DataType
		expected string
	}{
		{"int to varchar", NewInt(123), TypeVarChar, "123"},
		{"varchar to int", NewVarChar("456", -1), TypeInt, "456"},
		{"float to int", NewFloat(3.9), TypeInt, "3"},
		{"int to decimal", NewInt(100), TypeDecimal, "100"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Cast(tt.value, tt.toType, 18, 2, 50)
			if err != nil {
				t.Errorf("Cast error: %v", err)
				return
			}
			if result.AsString() != tt.expected {
				t.Errorf("Cast = %v, want %v", result.AsString(), tt.expected)
			}
		})
	}
}

func TestConvertDateTimeStyles(t *testing.T) {
	dt := NewDateTime(time.Date(2024, 3, 15, 14, 30, 45, 0, time.UTC))

	tests := []struct {
		style    int
		expected string
	}{
		{101, "03/15/2024"},          // USA
		{103, "15/03/2024"},          // British
		{111, "2024/03/15"},          // Japan
		{112, "20240315"},            // ISO
		{120, "2024-03-15 14:30:45"}, // ODBC
	}

	for _, tt := range tests {
		t.Run(string(rune(tt.style)), func(t *testing.T) {
			result, err := Convert(dt, TypeVarChar, 0, 0, 50, tt.style)
			if err != nil {
				t.Errorf("Convert error: %v", err)
				return
			}
			if result.AsString() != tt.expected {
				t.Errorf("Style %d = %v, want %v", tt.style, result.AsString(), tt.expected)
			}
		})
	}
}

func TestFunctions(t *testing.T) {
	registry := NewFunctionRegistry()

	tests := []struct {
		name     string
		funcName string
		args     []Value
		expected string
	}{
		// String functions
		{"LEN", "LEN", []Value{NewVarChar("hello", -1)}, "5"},
		{"UPPER", "UPPER", []Value{NewVarChar("hello", -1)}, "HELLO"},
		{"LOWER", "LOWER", []Value{NewVarChar("HELLO", -1)}, "hello"},
		{"LEFT", "LEFT", []Value{NewVarChar("hello", -1), NewInt(3)}, "hel"},
		{"RIGHT", "RIGHT", []Value{NewVarChar("hello", -1), NewInt(3)}, "llo"},
		{"SUBSTRING", "SUBSTRING", []Value{NewVarChar("hello", -1), NewInt(2), NewInt(3)}, "ell"},
		{"LTRIM", "LTRIM", []Value{NewVarChar("  hello", -1)}, "hello"},
		{"RTRIM", "RTRIM", []Value{NewVarChar("hello  ", -1)}, "hello"},
		{"REPLACE", "REPLACE", []Value{NewVarChar("hello world", -1), NewVarChar("world", -1), NewVarChar("there", -1)}, "hello there"},
		{"CHARINDEX", "CHARINDEX", []Value{NewVarChar("l", -1), NewVarChar("hello", -1)}, "3"},
		{"REVERSE", "REVERSE", []Value{NewVarChar("hello", -1)}, "olleh"},
		{"REPLICATE", "REPLICATE", []Value{NewVarChar("ab", -1), NewInt(3)}, "ababab"},
		{"SPACE", "SPACE", []Value{NewInt(5)}, "     "},

		// NULL functions
		{"ISNULL null", "ISNULL", []Value{Null(TypeInt), NewInt(42)}, "42"},
		{"ISNULL not null", "ISNULL", []Value{NewInt(10), NewInt(42)}, "10"},
		{"COALESCE", "COALESCE", []Value{Null(TypeInt), Null(TypeInt), NewInt(99)}, "99"},
		{"NULLIF equal", "NULLIF", []Value{NewInt(5), NewInt(5)}, ""},
		{"NULLIF not equal", "NULLIF", []Value{NewInt(5), NewInt(10)}, "5"},
		{"IIF true", "IIF", []Value{NewBit(true), NewVarChar("yes", -1), NewVarChar("no", -1)}, "yes"},
		{"IIF false", "IIF", []Value{NewBit(false), NewVarChar("yes", -1), NewVarChar("no", -1)}, "no"},

		// Numeric functions
		{"ABS", "ABS", []Value{NewInt(-42)}, "42"},
		{"CEILING", "CEILING", []Value{NewFloat(3.2)}, "4"},
		{"FLOOR", "FLOOR", []Value{NewFloat(3.8)}, "3"},
		{"ROUND", "ROUND", []Value{NewFloat(3.567), NewInt(2)}, "3.57"},
		{"SIGN positive", "SIGN", []Value{NewInt(42)}, "1"},
		{"SIGN negative", "SIGN", []Value{NewInt(-42)}, "-1"},
		{"SIGN zero", "SIGN", []Value{NewInt(0)}, "0"},
		{"POWER", "POWER", []Value{NewInt(2), NewInt(10)}, "1024"},
		{"SQRT", "SQRT", []Value{NewInt(16)}, "4"},

		// Date functions
		{"YEAR", "YEAR", []Value{NewDateTime(time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC))}, "2024"},
		{"MONTH", "MONTH", []Value{NewDateTime(time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC))}, "3"},
		{"DAY", "DAY", []Value{NewDateTime(time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC))}, "15"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := registry.Call(tt.funcName, tt.args)
			if err != nil {
				t.Errorf("%s error: %v", tt.funcName, err)
				return
			}
			// For NULL results, check IsNull
			if tt.expected == "" && result.IsNull {
				return
			}
			if result.AsString() != tt.expected {
				t.Errorf("%s = %v, want %v", tt.funcName, result.AsString(), tt.expected)
			}
		})
	}
}

func TestDateAdd(t *testing.T) {
	registry := NewFunctionRegistry()
	base := NewDateTime(time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC))

	tests := []struct {
		interval string
		number   int64
		expected string
	}{
		{"day", 5, "2024-01-20 10:30:00"},
		{"month", 2, "2024-03-15 10:30:00"},
		{"year", 1, "2025-01-15 10:30:00"},
		{"hour", 3, "2024-01-15 13:30:00"},
		{"minute", 45, "2024-01-15 11:15:00"},
	}

	for _, tt := range tests {
		t.Run(tt.interval, func(t *testing.T) {
			result, err := registry.Call("DATEADD", []Value{
				NewVarChar(tt.interval, -1),
				NewInt(tt.number),
				base,
			})
			if err != nil {
				t.Errorf("DATEADD error: %v", err)
				return
			}
			if result.AsString() != tt.expected {
				t.Errorf("DATEADD(%s, %d) = %v, want %v", tt.interval, tt.number, result.AsString(), tt.expected)
			}
		})
	}
}

func TestDateDiff(t *testing.T) {
	registry := NewFunctionRegistry()
	d1 := NewDateTime(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	d2 := NewDateTime(time.Date(2024, 3, 15, 12, 30, 0, 0, time.UTC))

	tests := []struct {
		interval string
		expected int64
	}{
		{"day", 74},
		{"month", 2},
		{"hour", 74*24 + 12},
	}

	for _, tt := range tests {
		t.Run(tt.interval, func(t *testing.T) {
			result, err := registry.Call("DATEDIFF", []Value{
				NewVarChar(tt.interval, -1),
				d1,
				d2,
			})
			if err != nil {
				t.Errorf("DATEDIFF error: %v", err)
				return
			}
			if result.AsInt() != tt.expected {
				t.Errorf("DATEDIFF(%s) = %v, want %v", tt.interval, result.AsInt(), tt.expected)
			}
		})
	}
}

func TestExpressionEvaluator(t *testing.T) {
	eval := NewExpressionEvaluator()
	eval.SetVariable("x", NewInt(10))
	eval.SetVariable("y", NewInt(5))
	eval.SetVariable("name", NewVarChar("hello", -1))

	// We can't easily test full expression parsing without going through the parser
	// But we can test the evaluator's variable handling
	if v, ok := eval.GetVariable("x"); !ok || v.AsInt() != 10 {
		t.Error("Failed to get variable x")
	}

	if v, ok := eval.GetVariable("y"); !ok || v.AsInt() != 5 {
		t.Error("Failed to get variable y")
	}

	if v, ok := eval.GetVariable("name"); !ok || v.AsString() != "hello" {
		t.Error("Failed to get variable name")
	}

	if _, ok := eval.GetVariable("undefined"); ok {
		t.Error("Should not find undefined variable")
	}
}

func TestToValueFromValue(t *testing.T) {
	tests := []struct {
		input    interface{}
		expected interface{}
	}{
		{42, int32(42)},
		{int64(100), int64(100)},
		{3.14, 3.14},
		{"hello", "hello"},
		{true, true},
		{false, false},
		{nil, nil},
	}

	for _, tt := range tests {
		val := ToValue(tt.input)
		result := FromValue(val)

		if tt.expected == nil {
			if result != nil {
				t.Errorf("ToValue(%v) -> FromValue = %v, want nil", tt.input, result)
			}
			continue
		}

		// Compare based on type
		switch exp := tt.expected.(type) {
		case int32:
			if r, ok := result.(int32); !ok || r != exp {
				t.Errorf("ToValue(%v) -> FromValue = %v, want %v", tt.input, result, tt.expected)
			}
		case int64:
			if r, ok := result.(int64); !ok || r != exp {
				t.Errorf("ToValue(%v) -> FromValue = %v, want %v", tt.input, result, tt.expected)
			}
		case float64:
			if r, ok := result.(float64); !ok || r != exp {
				t.Errorf("ToValue(%v) -> FromValue = %v, want %v", tt.input, result, tt.expected)
			}
		case string:
			if r, ok := result.(string); !ok || r != exp {
				t.Errorf("ToValue(%v) -> FromValue = %v, want %v", tt.input, result, tt.expected)
			}
		case bool:
			if r, ok := result.(bool); !ok || r != exp {
				t.Errorf("ToValue(%v) -> FromValue = %v, want %v", tt.input, result, tt.expected)
			}
		}
	}
}

func TestLikePattern(t *testing.T) {
	tests := []struct {
		str     string
		pattern string
		matches bool
	}{
		{"hello", "hello", true},
		{"hello", "h%", true},
		{"hello", "%o", true},
		{"hello", "%ll%", true},
		{"hello", "h_llo", true},
		{"hello", "h___o", true},
		{"hello", "x%", false},
		{"hello", "%x", false},
		{"hello world", "hello%", true},
		{"hello world", "%world", true},
		{"hello world", "hello%world", true},
	}

	for _, tt := range tests {
		t.Run(tt.str+"/"+tt.pattern, func(t *testing.T) {
			if got := matchLikePattern(tt.str, tt.pattern); got != tt.matches {
				t.Errorf("matchLikePattern(%q, %q) = %v, want %v", tt.str, tt.pattern, got, tt.matches)
			}
		})
	}
}

func TestParseDataType(t *testing.T) {
	tests := []struct {
		input     string
		dt        DataType
		precision int
		scale     int
		maxLen    int
	}{
		{"int", TypeInt, 0, 0, 0},
		{"varchar(50)", TypeVarChar, 50, 0, 50},
		{"decimal(18,2)", TypeDecimal, 18, 2, 18},
		{"nvarchar(max)", TypeNVarChar, 0, 0, -1},
		{"datetime", TypeDateTime, 0, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			dt, prec, scale, maxLen := ParseDataType(tt.input)
			if dt != tt.dt {
				t.Errorf("DataType = %v, want %v", dt, tt.dt)
			}
			if prec != tt.precision {
				t.Errorf("Precision = %v, want %v", prec, tt.precision)
			}
			if scale != tt.scale {
				t.Errorf("Scale = %v, want %v", scale, tt.scale)
			}
			if maxLen != tt.maxLen {
				t.Errorf("MaxLen = %v, want %v", maxLen, tt.maxLen)
			}
		})
	}
}
