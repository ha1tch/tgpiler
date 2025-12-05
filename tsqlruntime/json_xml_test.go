package tsqlruntime

import (
	"strings"
	"testing"
)

// ============================================================================
// JSON Tests
// ============================================================================

func TestJSONValue_Scalars(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		path     string
		expected string
	}{
		{"simple string", `{"name": "John"}`, "$.name", "John"},
		{"number", `{"id": 123}`, "$.id", "123"},
		{"float", `{"price": 19.99}`, "$.price", "19.99"},
		{"boolean true", `{"active": true}`, "$.active", "true"},
		{"boolean false", `{"active": false}`, "$.active", "false"},
		{"nested string", `{"customer": {"name": "Alice"}}`, "$.customer.name", "Alice"},
		{"nested number", `{"order": {"total": 99}}`, "$.order.total", "99"},
		{"array element", `{"items": ["a", "b", "c"]}`, "$.items[1]", "b"},
		{"array first", `{"items": [10, 20, 30]}`, "$.items[0]", "10"},
		{"nested array", `{"data": {"values": [1, 2, 3]}}`, "$.data.values[2]", "3"},
		{"object in array", `{"orders": [{"id": 1}, {"id": 2}]}`, "$.orders[0].id", "1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := JSONValue(tt.json, tt.path)
			if err != nil {
				t.Fatalf("JSONValue error: %v", err)
			}
			if result.IsNull {
				t.Fatalf("Expected non-null result")
			}
			if result.AsString() != tt.expected {
				t.Errorf("JSONValue(%q, %q) = %q, want %q",
					tt.json, tt.path, result.AsString(), tt.expected)
			}
		})
	}
}

func TestJSONValue_NullCases(t *testing.T) {
	// Missing path returns NULL
	result, _ := JSONValue(`{"name": "John"}`, "$.missing")
	if !result.IsNull {
		t.Error("Missing path should return NULL")
	}

	// Object returns NULL (JSON_VALUE only returns scalars)
	result, _ = JSONValue(`{"customer": {"name": "Alice"}}`, "$.customer")
	if !result.IsNull {
		t.Error("Object should return NULL for JSON_VALUE")
	}

	// Array returns NULL
	result, _ = JSONValue(`{"items": [1, 2, 3]}`, "$.items")
	if !result.IsNull {
		t.Error("Array should return NULL for JSON_VALUE")
	}

	// Invalid JSON returns NULL
	result, _ = JSONValue(`not json`, "$.name")
	if !result.IsNull {
		t.Error("Invalid JSON should return NULL")
	}
}

func TestJSONQuery_Objects(t *testing.T) {
	// Extract object
	result, err := JSONQuery(`{"customer": {"name": "Alice", "id": 123}}`, "$.customer")
	if err != nil {
		t.Fatalf("JSONQuery error: %v", err)
	}
	if result.IsNull {
		t.Fatal("Expected non-null result")
	}
	s := result.AsString()
	if !strings.HasPrefix(s, "{") {
		t.Errorf("Expected JSON object, got %q", s)
	}

	// Extract array
	result, _ = JSONQuery(`{"items": [1, 2, 3]}`, "$.items")
	if result.IsNull {
		t.Fatal("Expected non-null result for array")
	}
	s = result.AsString()
	if !strings.HasPrefix(s, "[") {
		t.Errorf("Expected JSON array, got %q", s)
	}

	// Scalar returns NULL for JSON_QUERY
	result, _ = JSONQuery(`{"name": "John"}`, "$.name")
	if !result.IsNull {
		t.Error("Scalar should return NULL for JSON_QUERY")
	}
}

func TestJSONModify(t *testing.T) {
	// Modify existing value
	result, err := JSONModify(`{"name": "John"}`, "$.name", "Jane")
	if err != nil {
		t.Fatalf("JSONModify error: %v", err)
	}
	check, _ := JSONValue(result.AsString(), "$.name")
	if check.AsString() != "Jane" {
		t.Errorf("Modified value = %q, want 'Jane'", check.AsString())
	}

	// Add new property
	result, _ = JSONModify(`{"name": "John"}`, "$.age", 30)
	check, _ = JSONValue(result.AsString(), "$.age")
	if check.AsString() != "30" {
		t.Errorf("Added value = %q, want '30'", check.AsString())
	}
}

func TestIsJSON(t *testing.T) {
	tests := []struct {
		input string
		want  int64
	}{
		{`{"key": "value"}`, 1},
		{`[1, 2, 3]`, 1},
		{`{"nested": {"a": 1}}`, 1},
		{`[]`, 1},
		{`{}`, 1},
		{`not json`, 0},
		{`{invalid}`, 0},
		{``, 0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, _ := IsJSON(tt.input)
			if result.AsInt() != tt.want {
				t.Errorf("IsJSON(%q) = %d, want %d", tt.input, result.AsInt(), tt.want)
			}
		})
	}
}

func TestOpenJSON(t *testing.T) {
	// Array of mixed values
	results, err := OpenJSON(`[1, "two", true, null]`, "$")
	if err != nil {
		t.Fatalf("OpenJSON error: %v", err)
	}
	if len(results) != 4 {
		t.Errorf("Expected 4 rows, got %d", len(results))
	}

	// Object properties
	results, _ = OpenJSON(`{"name": "John", "age": 30}`, "$")
	if len(results) != 2 {
		t.Errorf("Expected 2 rows, got %d", len(results))
	}

	// Nested path
	results, _ = OpenJSON(`{"items": [{"id": 1}, {"id": 2}]}`, "$.items")
	if len(results) != 2 {
		t.Errorf("Expected 2 rows for nested path, got %d", len(results))
	}
}

func TestOpenJSONWithSchema(t *testing.T) {
	json := `[{"id": 1, "name": "Alice"}, {"id": 2, "name": "Bob"}]`
	schema := []OpenJSONColumn{
		{Name: "id", Type: TypeInt, JSONPath: "$.id"},
		{Name: "name", Type: TypeVarChar, JSONPath: "$.name"},
	}

	results, err := OpenJSONWithSchema(json, "$", schema)
	if err != nil {
		t.Fatalf("OpenJSONWithSchema error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("Expected 2 rows, got %d", len(results))
	}

	if results[0]["id"].AsInt() != 1 {
		t.Errorf("First id = %d, want 1", results[0]["id"].AsInt())
	}
	if results[0]["name"].AsString() != "Alice" {
		t.Errorf("First name = %q, want 'Alice'", results[0]["name"].AsString())
	}
}

func TestForJSON(t *testing.T) {
	columns := []string{"id", "name"}
	rows := [][]Value{
		{NewInt(1), NewVarChar("Alice", -1)},
		{NewInt(2), NewVarChar("Bob", -1)},
	}

	// Basic PATH mode
	result, err := ForJSON(columns, rows, ForJSONOptions{Mode: ForJSONPath})
	if err != nil {
		t.Fatalf("ForJSON error: %v", err)
	}
	if !strings.HasPrefix(result, "[") {
		t.Errorf("Expected JSON array, got %q", result)
	}

	// With root
	result, _ = ForJSON(columns, rows, ForJSONOptions{
		Mode:     ForJSONPath,
		RootName: "data",
	})
	if !strings.Contains(result, `"data"`) {
		t.Errorf("Expected root wrapper 'data', got %q", result)
	}

	// Without array wrapper (single row)
	result, _ = ForJSON(columns, rows[:1], ForJSONOptions{
		Mode:                ForJSONPath,
		WithoutArrayWrapper: true,
	})
	if strings.HasPrefix(result, "[") {
		t.Errorf("Expected single object without array, got %q", result)
	}
}

// ============================================================================
// XML Tests
// ============================================================================

func TestXMLValue_Elements(t *testing.T) {
	tests := []struct {
		name     string
		xml      string
		xpath    string
		expected string
	}{
		{"simple element", `<root><item>John</item></root>`, "/root/item", "John"},
		{"nested element", `<root><customer><item>Alice</item></customer></root>`, "/root/customer/item", "Alice"},
		{"deep nesting", `<a><b><c><d>value</d></c></b></a>`, "/a/b/c/d", "value"},
		{"indexed element", `<root><item>A</item><item>B</item><item>C</item></root>`, "/root/item[2]", "B"},
		{"first element", `<root><item>A</item><item>B</item></root>`, "/root/item[1]", "A"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := XMLValue(tt.xml, tt.xpath, TypeVarChar)
			if err != nil {
				t.Fatalf("XMLValue error: %v", err)
			}
			if result.IsNull {
				t.Fatalf("Expected non-null result")
			}
			if result.AsString() != tt.expected {
				t.Errorf("XMLValue(%q, %q) = %q, want %q",
					tt.xml, tt.xpath, result.AsString(), tt.expected)
			}
		})
	}
}

func TestXMLValue_Attributes(t *testing.T) {
	tests := []struct {
		name     string
		xml      string
		xpath    string
		expected string
	}{
		{"root attribute", `<root id="123"/>`, "/root/@id", "123"},
		{"child attribute", `<root><item id="456"/></root>`, "/root/item/@id", "456"},
		{"nested attribute", `<root><a><b id="789"/></a></root>`, "/root/a/b/@id", "789"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := XMLValue(tt.xml, tt.xpath, TypeVarChar)
			if err != nil {
				t.Fatalf("XMLValue error: %v", err)
			}
			if result.IsNull {
				t.Fatalf("Expected non-null result for %s", tt.name)
			}
			if result.AsString() != tt.expected {
				t.Errorf("XMLValue = %q, want %q", result.AsString(), tt.expected)
			}
		})
	}
}

func TestXMLValue_TypeConversion(t *testing.T) {
	// Integer conversion
	result, _ := XMLValue(`<root><id>123</id></root>`, "/root/id", TypeInt)
	if result.AsInt() != 123 {
		t.Errorf("Integer conversion: got %d, want 123", result.AsInt())
	}

	// Float conversion
	result, _ = XMLValue(`<root><price>19.99</price></root>`, "/root/price", TypeFloat)
	if result.AsFloat() != 19.99 {
		t.Errorf("Float conversion: got %f, want 19.99", result.AsFloat())
	}
}

func TestXMLExist(t *testing.T) {
	tests := []struct {
		xml   string
		xpath string
		want  int64
	}{
		{`<root><item/></root>`, "/root/item", 1},
		{`<root><item/></root>`, "/root/missing", 0},
		{`<root id="1"/>`, "/root/@id", 1},
		{`<root id="1"/>`, "/root/@missing", 0},
		{`<root><a><b/></a></root>`, "/root/a/b", 1},
		{`<root><a><b/></a></root>`, "/root/a/c", 0},
	}

	for _, tt := range tests {
		t.Run(tt.xpath, func(t *testing.T) {
			result, _ := XMLExist(tt.xml, tt.xpath)
			if result.AsInt() != tt.want {
				t.Errorf("XMLExist(%q, %q) = %d, want %d",
					tt.xml, tt.xpath, result.AsInt(), tt.want)
			}
		})
	}
}

func TestXMLQuery(t *testing.T) {
	// Extract single element
	result, err := XMLQuery(`<root><item id="1">A</item></root>`, "/root/item")
	if err != nil {
		t.Fatalf("XMLQuery error: %v", err)
	}
	if result.IsNull {
		t.Fatal("Expected non-null result")
	}
	s := result.AsString()
	if !strings.Contains(s, "item") {
		t.Errorf("Expected XML with 'item', got %q", s)
	}
}

func TestXMLNodes(t *testing.T) {
	xml := `<root><item id="1">A</item><item id="2">B</item><item id="3">C</item></root>`

	results, err := XMLNodes(xml, "/root/item")
	if err != nil {
		t.Fatalf("XMLNodes error: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("Expected 3 nodes, got %d", len(results))
	}

	// Check attributes
	if results[0].Node.Attributes["id"] != "1" {
		t.Errorf("First node id = %q, want '1'", results[0].Node.Attributes["id"])
	}
	if results[1].Node.Attributes["id"] != "2" {
		t.Errorf("Second node id = %q, want '2'", results[1].Node.Attributes["id"])
	}

	// Check values
	if results[0].Node.Value != "A" {
		t.Errorf("First node value = %q, want 'A'", results[0].Node.Value)
	}
}

func TestForXML_Raw(t *testing.T) {
	columns := []string{"id", "name"}
	rows := [][]Value{
		{NewInt(1), NewVarChar("Alice", -1)},
		{NewInt(2), NewVarChar("Bob", -1)},
	}

	// RAW mode (attributes)
	result, err := ForXML(columns, rows, ForXMLOptions{
		Mode:        ForXMLRaw,
		ElementName: "row",
	})
	if err != nil {
		t.Fatalf("ForXML error: %v", err)
	}
	if !strings.Contains(result, `id="1"`) {
		t.Errorf("Expected attribute id='1', got %q", result)
	}
	if !strings.Contains(result, `name="Alice"`) {
		t.Errorf("Expected attribute name='Alice', got %q", result)
	}
}

func TestForXML_Elements(t *testing.T) {
	columns := []string{"id", "name"}
	rows := [][]Value{
		{NewInt(1), NewVarChar("Alice", -1)},
	}

	// ELEMENTS mode
	result, err := ForXML(columns, rows, ForXMLOptions{
		Mode:        ForXMLPath,
		ElementName: "row",
		Elements:    true,
	})
	if err != nil {
		t.Fatalf("ForXML error: %v", err)
	}
	if !strings.Contains(result, "<id>1</id>") {
		t.Errorf("Expected element <id>1</id>, got %q", result)
	}
	if !strings.Contains(result, "<name>Alice</name>") {
		t.Errorf("Expected element <name>Alice</name>, got %q", result)
	}
}

func TestForXML_Root(t *testing.T) {
	columns := []string{"id"}
	rows := [][]Value{{NewInt(1)}}

	result, _ := ForXML(columns, rows, ForXMLOptions{
		Mode:        ForXMLPath,
		ElementName: "row",
		RootName:    "data",
	})

	if !strings.HasPrefix(result, "<data>") {
		t.Errorf("Expected root element <data>, got %q", result)
	}
	if !strings.HasSuffix(result, "</data>") {
		t.Errorf("Expected closing </data>, got %q", result)
	}
}

func TestOpenXML(t *testing.T) {
	xml := `<root><row id="1" name="Alice"/><row id="2" name="Bob"/></root>`

	schema := []OpenXMLColumn{
		{Name: "id", Type: TypeInt, XPath: "@id"},
		{Name: "name", Type: TypeVarChar, MaxLen: 50, XPath: "@name"},
	}

	result, err := OpenXML(xml, "/root/row", 0, schema)
	if err != nil {
		t.Fatalf("OpenXML error: %v", err)
	}
	if len(result.Rows) != 2 {
		t.Fatalf("Expected 2 rows, got %d", len(result.Rows))
	}

	if result.Rows[0]["id"].AsInt() != 1 {
		t.Errorf("First row id = %d, want 1", result.Rows[0]["id"].AsInt())
	}
	if result.Rows[0]["name"].AsString() != "Alice" {
		t.Errorf("First row name = %q, want 'Alice'", result.Rows[0]["name"].AsString())
	}
	if result.Rows[1]["id"].AsInt() != 2 {
		t.Errorf("Second row id = %d, want 2", result.Rows[1]["id"].AsInt())
	}
}

func TestOpenXML_ElementValues(t *testing.T) {
	xml := `<root>
		<row><id>1</id><name>Alice</name></row>
		<row><id>2</id><name>Bob</name></row>
	</root>`

	schema := []OpenXMLColumn{
		{Name: "id", Type: TypeInt, XPath: "id"},
		{Name: "name", Type: TypeVarChar, MaxLen: 50, XPath: "name"},
	}

	result, err := OpenXML(xml, "/root/row", 0, schema)
	if err != nil {
		t.Fatalf("OpenXML error: %v", err)
	}
	if len(result.Rows) != 2 {
		t.Fatalf("Expected 2 rows, got %d", len(result.Rows))
	}

	if result.Rows[0]["id"].AsInt() != 1 {
		t.Errorf("First row id = %d, want 1", result.Rows[0]["id"].AsInt())
	}
}
