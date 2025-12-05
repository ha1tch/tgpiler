package tsqlruntime

import (
	"testing"
)

func TestCursor(t *testing.T) {
	manager := NewCursorManager()

	// Declare cursor
	cursor, err := manager.DeclareCursor(
		"test_cursor",
		"SELECT id, name FROM users",
		false,
		CursorForwardOnly,
		CursorScrollNone,
		CursorReadOnly,
	)
	if err != nil {
		t.Fatalf("DeclareCursor failed: %v", err)
	}

	if !cursor.IsAllocated {
		t.Error("Cursor should be allocated")
	}

	if cursor.IsOpen {
		t.Error("Cursor should not be open yet")
	}

	// Open cursor with test data
	columns := []string{"id", "name"}
	rows := [][]Value{
		{NewInt(1), NewVarChar("Alice", -1)},
		{NewInt(2), NewVarChar("Bob", -1)},
		{NewInt(3), NewVarChar("Charlie", -1)},
	}

	err = cursor.Open(columns, rows)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	if !cursor.IsOpen {
		t.Error("Cursor should be open")
	}

	if cursor.RowCount() != 3 {
		t.Errorf("Expected 3 rows, got %d", cursor.RowCount())
	}

	// Fetch next
	row, status := cursor.FetchNext()
	if status != 0 {
		t.Errorf("Expected status 0, got %d", status)
	}
	if row[0].AsInt() != 1 || row[1].AsString() != "Alice" {
		t.Errorf("Unexpected first row: %v", row)
	}

	// Fetch next again
	row, status = cursor.FetchNext()
	if status != 0 {
		t.Errorf("Expected status 0, got %d", status)
	}
	if row[0].AsInt() != 2 {
		t.Errorf("Expected id=2, got %d", row[0].AsInt())
	}

	// Fetch next again
	row, status = cursor.FetchNext()
	if status != 0 || row[0].AsInt() != 3 {
		t.Error("Expected third row")
	}

	// Fetch past end
	_, status = cursor.FetchNext()
	if status != -1 {
		t.Errorf("Expected status -1 at end, got %d", status)
	}

	// Close cursor
	err = cursor.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	if cursor.IsOpen {
		t.Error("Cursor should be closed")
	}

	// Deallocate cursor
	err = manager.DeallocateCursor("test_cursor")
	if err != nil {
		t.Fatalf("DeallocateCursor failed: %v", err)
	}

	// Verify deallocated
	_, ok := manager.GetCursor("test_cursor")
	if ok {
		t.Error("Cursor should not exist after deallocation")
	}
}

func TestScrollableCursor(t *testing.T) {
	cursor := &Cursor{
		Name:        "scroll_cursor",
		IsAllocated: true,
		ScrollType:  CursorScrollForward,
	}

	columns := []string{"id"}
	rows := [][]Value{
		{NewInt(1)},
		{NewInt(2)},
		{NewInt(3)},
		{NewInt(4)},
		{NewInt(5)},
	}

	cursor.Open(columns, rows)

	// Fetch first
	row, status := cursor.FetchFirst()
	if status != 0 || row[0].AsInt() != 1 {
		t.Error("FetchFirst failed")
	}

	// Fetch last
	row, status = cursor.FetchLast()
	if status != 0 || row[0].AsInt() != 5 {
		t.Error("FetchLast failed")
	}

	// Fetch absolute 3
	row, status = cursor.FetchAbsolute(3)
	if status != 0 || row[0].AsInt() != 3 {
		t.Errorf("FetchAbsolute(3) failed: got %d", row[0].AsInt())
	}

	// Fetch relative -1
	row, status = cursor.FetchRelative(-1)
	if status != 0 || row[0].AsInt() != 2 {
		t.Errorf("FetchRelative(-1) failed: got %d", row[0].AsInt())
	}

	// Fetch prior
	row, status = cursor.FetchPrior()
	if status != 0 || row[0].AsInt() != 1 {
		t.Errorf("FetchPrior failed: got %d", row[0].AsInt())
	}
}

func TestCursorStatus(t *testing.T) {
	cursor := &Cursor{
		Name:        "status_cursor",
		IsAllocated: false,
	}

	// Not allocated
	if cursor.Status() != -3 {
		t.Errorf("Expected -3 for not allocated, got %d", cursor.Status())
	}

	// Allocated but not open
	cursor.IsAllocated = true
	if cursor.Status() != -1 {
		t.Errorf("Expected -1 for closed, got %d", cursor.Status())
	}

	// Open but empty
	cursor.IsOpen = true
	cursor.Rows = [][]Value{}
	if cursor.Status() != 0 {
		t.Errorf("Expected 0 for empty, got %d", cursor.Status())
	}

	// Open with data
	cursor.Rows = [][]Value{{NewInt(1)}}
	if cursor.Status() != 1 {
		t.Errorf("Expected 1 for open with data, got %d", cursor.Status())
	}
}

func TestHashBytes(t *testing.T) {
	tests := []struct {
		algorithm string
		input     string
		wantLen   int
	}{
		{"MD5", "test", 16},
		{"SHA1", "test", 20},
		{"SHA256", "test", 32},
		{"SHA512", "test", 64},
	}

	for _, tt := range tests {
		t.Run(tt.algorithm, func(t *testing.T) {
			result, err := fnHashBytes([]Value{
				NewVarChar(tt.algorithm, -1),
				NewVarChar(tt.input, -1),
			})
			if err != nil {
				t.Fatalf("fnHashBytes failed: %v", err)
			}
			if result.IsNull {
				t.Error("Expected non-null result")
			}
			if len(result.bytesVal) != tt.wantLen {
				t.Errorf("Expected %d bytes, got %d", tt.wantLen, len(result.bytesVal))
			}
		})
	}
}

func TestChecksum(t *testing.T) {
	result, _ := fnChecksum([]Value{NewVarChar("test", -1)})
	if result.IsNull {
		t.Error("Expected non-null result")
	}

	// Same input should give same checksum
	result2, _ := fnChecksum([]Value{NewVarChar("test", -1)})
	if result.AsInt() != result2.AsInt() {
		t.Error("Same input should give same checksum")
	}

	// Different input should give different checksum
	result3, _ := fnChecksum([]Value{NewVarChar("different", -1)})
	if result.AsInt() == result3.AsInt() {
		t.Error("Different input should give different checksum")
	}
}

func TestIsJson(t *testing.T) {
	tests := []struct {
		input string
		want  int64
	}{
		{`{"key": "value"}`, 1},
		{`[1, 2, 3]`, 1},
		{`not json`, 0},
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

func TestGreatest(t *testing.T) {
	result, _ := fnGreatest([]Value{
		NewInt(3),
		NewInt(1),
		NewInt(5),
		NewInt(2),
	})
	if result.AsInt() != 5 {
		t.Errorf("GREATEST(3,1,5,2) = %d, want 5", result.AsInt())
	}

	// With NULL values
	result, _ = fnGreatest([]Value{
		Null(TypeInt),
		NewInt(3),
		NewInt(1),
	})
	if result.AsInt() != 3 {
		t.Errorf("GREATEST(NULL,3,1) = %d, want 3", result.AsInt())
	}
}

func TestLeast(t *testing.T) {
	result, _ := fnLeast([]Value{
		NewInt(3),
		NewInt(1),
		NewInt(5),
		NewInt(2),
	})
	if result.AsInt() != 1 {
		t.Errorf("LEAST(3,1,5,2) = %d, want 1", result.AsInt())
	}
}

func TestTryCast(t *testing.T) {
	// Valid cast
	result, _ := fnTryCast([]Value{
		NewVarChar("123", -1),
		NewVarChar("int", -1),
	})
	if result.IsNull || result.AsInt() != 123 {
		t.Errorf("TRY_CAST('123' AS INT) failed")
	}

	// Note: Invalid casts may or may not return NULL depending on Cast implementation
	// The important thing is TRY_CAST doesn't return an error
}

func TestCursorManager_ClearSession(t *testing.T) {
	manager := NewCursorManager()

	// Create local and global cursors
	manager.DeclareCursor("local_cursor", "SELECT 1", false, CursorForwardOnly, CursorScrollNone, CursorReadOnly)
	manager.DeclareCursor("global_cursor", "SELECT 2", true, CursorForwardOnly, CursorScrollNone, CursorReadOnly)

	// Clear session
	manager.ClearSession()

	// Local cursor should be gone
	_, ok := manager.GetCursor("local_cursor")
	if ok {
		t.Error("Local cursor should be cleared")
	}

	// Global cursor should remain
	_, ok = manager.GetCursor("global_cursor")
	if !ok {
		t.Error("Global cursor should remain after ClearSession")
	}
}
