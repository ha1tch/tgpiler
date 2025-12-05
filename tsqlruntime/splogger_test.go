package tsqlruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"
)

func TestSPErrorToXML(t *testing.T) {
	err := SPError{
		ProcedureName: "TestProc",
		Parameters: map[string]interface{}{
			"id":   123,
			"name": "test",
		},
		ErrorMessage: "Something went wrong",
		ErrorNumber:  50000,
		Severity:     16,
		State:        1,
		Line:         42,
		Timestamp:    time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
	}

	xml := err.ToXML()

	checks := []string{
		"<SPError>",
		"<ProcedureName>TestProc</ProcedureName>",
		"<ErrorMessage>Something went wrong</ErrorMessage>",
		"<ErrorNumber>50000</ErrorNumber>",
		"<Severity>16</Severity>",
		"<State>1</State>",
		"<Line>42</Line>",
		"</SPError>",
	}

	for _, check := range checks {
		if !strings.Contains(xml, check) {
			t.Errorf("Expected XML to contain %q, got: %s", check, xml)
		}
	}
}

func TestSPErrorToJSON(t *testing.T) {
	err := SPError{
		ProcedureName: "TestProc",
		Parameters: map[string]interface{}{
			"id": 123,
		},
		ErrorMessage: "Test error",
		ErrorNumber:  50000,
		Severity:     16,
		State:        1,
		Line:         42,
		Timestamp:    time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
	}

	jsonStr := err.ToJSON()

	var parsed map[string]interface{}
	if e := json.Unmarshal([]byte(jsonStr), &parsed); e != nil {
		t.Fatalf("Failed to parse JSON: %v", e)
	}

	if parsed["procedure_name"] != "TestProc" {
		t.Errorf("Expected procedure_name=TestProc, got %v", parsed["procedure_name"])
	}
	if parsed["error_message"] != "Test error" {
		t.Errorf("Expected error_message=Test error, got %v", parsed["error_message"])
	}
}

func TestCaptureError(t *testing.T) {
	params := map[string]interface{}{"id": 42}
	
	err := CaptureError("MyProc", "panic: something bad", params)

	if err.ProcedureName != "MyProc" {
		t.Errorf("Expected ProcedureName=MyProc, got %s", err.ProcedureName)
	}
	if err.ErrorMessage != "panic: something bad" {
		t.Errorf("Expected ErrorMessage='panic: something bad', got %s", err.ErrorMessage)
	}
	if err.Parameters["id"] != 42 {
		t.Errorf("Expected params[id]=42, got %v", err.Parameters["id"])
	}
	if err.Severity != 16 {
		t.Errorf("Expected Severity=16, got %d", err.Severity)
	}
	if err.StackTrace == "" {
		t.Error("Expected StackTrace to be populated")
	}
	if err.Line == 0 {
		t.Error("Expected Line to be populated")
	}
}

func TestSlogSPLogger(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := NewSlogSPLoggerWithHandler(handler)

	ctx := context.Background()
	err := SPError{
		ProcedureName: "TestProc",
		Parameters:    map[string]interface{}{"id": 1},
		ErrorMessage:  "Test error",
		Timestamp:     time.Now(),
	}

	logger.LogError(ctx, err)

	output := buf.String()
	if !strings.Contains(output, "TestProc") {
		t.Errorf("Expected log to contain 'TestProc', got: %s", output)
	}
	if !strings.Contains(output, "Test error") {
		t.Errorf("Expected log to contain 'Test error', got: %s", output)
	}
	if !strings.Contains(output, "stored procedure error") {
		t.Errorf("Expected log to contain 'stored procedure error', got: %s", output)
	}
}

func TestSlogSPLoggerEntry(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := NewSlogSPLoggerWithHandler(handler)

	ctx := context.Background()
	logger.LogEntry(ctx, "MyProc", map[string]interface{}{"x": 10})

	output := buf.String()
	if !strings.Contains(output, "MyProc") {
		t.Errorf("Expected log to contain 'MyProc', got: %s", output)
	}
	if !strings.Contains(output, "stored procedure entry") {
		t.Errorf("Expected log to contain 'stored procedure entry', got: %s", output)
	}
}

func TestMultiSPLogger(t *testing.T) {
	var buf1, buf2 bytes.Buffer
	handler1 := slog.NewJSONHandler(&buf1, nil)
	handler2 := slog.NewJSONHandler(&buf2, nil)

	logger1 := NewSlogSPLoggerWithHandler(handler1)
	logger2 := NewSlogSPLoggerWithHandler(handler2)
	multi := NewMultiSPLogger(logger1, logger2)

	ctx := context.Background()
	err := SPError{
		ProcedureName: "MultiTest",
		ErrorMessage:  "Multi error",
		Timestamp:     time.Now(),
	}

	multi.LogError(ctx, err)

	// Both loggers should have the error
	if !strings.Contains(buf1.String(), "MultiTest") {
		t.Error("Logger 1 should contain 'MultiTest'")
	}
	if !strings.Contains(buf2.String(), "MultiTest") {
		t.Error("Logger 2 should contain 'MultiTest'")
	}
}

func TestNopSPLogger(t *testing.T) {
	logger := NewNopSPLogger()
	ctx := context.Background()

	// Should not panic
	err := logger.LogError(ctx, SPError{ErrorMessage: "test"})
	if err != nil {
		t.Errorf("NopSPLogger.LogError should return nil, got %v", err)
	}

	logger.LogEntry(ctx, "proc", nil)
	logger.LogExit(ctx, "proc", time.Second, nil)
}

func TestFileSPLogger(t *testing.T) {
	// Create temp file
	f, err := os.CreateTemp("", "splogger_test_*.log")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	path := f.Name()
	f.Close()
	defer os.Remove(path)

	// Create logger
	logger, err := NewFileSPLogger(path, "json")
	if err != nil {
		t.Fatalf("Failed to create file logger: %v", err)
	}

	ctx := context.Background()
	spErr := SPError{
		ProcedureName: "FileTest",
		ErrorMessage:  "File error",
		Parameters:    map[string]interface{}{"id": 99},
		Timestamp:     time.Now(),
	}

	if err := logger.LogError(ctx, spErr); err != nil {
		t.Fatalf("LogError failed: %v", err)
	}

	logger.Close()

	// Read and verify
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	if !strings.Contains(string(content), "FileTest") {
		t.Errorf("Log file should contain 'FileTest', got: %s", content)
	}
	if !strings.Contains(string(content), "File error") {
		t.Errorf("Log file should contain 'File error', got: %s", content)
	}
}

func TestFileSPLoggerTextFormat(t *testing.T) {
	f, err := os.CreateTemp("", "splogger_test_*.log")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	path := f.Name()
	f.Close()
	defer os.Remove(path)

	logger, err := NewFileSPLogger(path, "text")
	if err != nil {
		t.Fatalf("Failed to create file logger: %v", err)
	}

	ctx := context.Background()
	spErr := SPError{
		ProcedureName: "TextTest",
		ErrorMessage:  "Text error",
		Timestamp:     time.Now(),
	}

	logger.LogError(ctx, spErr)
	logger.Close()

	content, _ := os.ReadFile(path)
	if !strings.Contains(string(content), "TextTest: Text error") {
		t.Errorf("Expected text format, got: %s", content)
	}
}

func TestDefaultSPLogger(t *testing.T) {
	// Default should be NopSPLogger
	logger := GetDefaultSPLogger()
	if _, ok := logger.(*NopSPLogger); !ok {
		t.Error("Default logger should be NopSPLogger")
	}

	// Set custom logger
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, nil)
	custom := NewSlogSPLoggerWithHandler(handler)
	SetDefaultSPLogger(custom)

	// Use global function
	err := SPError{ProcedureName: "GlobalTest", ErrorMessage: "Global error", Timestamp: time.Now()}
	LogSPError(context.Background(), err)

	if !strings.Contains(buf.String(), "GlobalTest") {
		t.Error("Global log should contain 'GlobalTest'")
	}

	// Reset to nop
	SetDefaultSPLogger(NewNopSPLogger())
}

func TestBufferedSPLogger(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, nil)
	inner := NewSlogSPLoggerWithHandler(handler)

	buffered := NewBufferedSPLogger(inner, 3, time.Hour) // Large interval, small batch
	ctx := context.Background()

	// Log 2 errors (below batch size)
	buffered.LogError(ctx, SPError{ProcedureName: "Buf1", ErrorMessage: "e1", Timestamp: time.Now()})
	buffered.LogError(ctx, SPError{ProcedureName: "Buf2", ErrorMessage: "e2", Timestamp: time.Now()})

	// Should not be flushed yet
	if strings.Contains(buf.String(), "Buf1") {
		t.Error("Should not have flushed yet")
	}

	// Log 3rd error (triggers batch)
	buffered.LogError(ctx, SPError{ProcedureName: "Buf3", ErrorMessage: "e3", Timestamp: time.Now()})

	// Give flush goroutine time to run
	time.Sleep(50 * time.Millisecond)

	// Should be flushed now
	if !strings.Contains(buf.String(), "Buf1") {
		t.Error("Should have flushed Buf1")
	}

	buffered.Close(ctx)
}

func TestDatabaseLoggerColumns(t *testing.T) {
	cols := DefaultDatabaseLoggerColumns()

	if cols.ProcedureName != "StoreProcedure" {
		t.Errorf("Expected ProcedureName='StoreProcedure', got %s", cols.ProcedureName)
	}
	if cols.ErrorMessage != "Message" {
		t.Errorf("Expected ErrorMessage='Message', got %s", cols.ErrorMessage)
	}
	if cols.Parameters != "XmlParameters" {
		t.Errorf("Expected Parameters='XmlParameters', got %s", cols.Parameters)
	}
}

func TestXMLEscaping(t *testing.T) {
	err := SPError{
		ProcedureName: "Test<Proc>",
		ErrorMessage:  "Error with <special> & \"chars\"",
		Parameters:    map[string]interface{}{},
		Timestamp:     time.Now(),
	}

	xmlStr := err.ToXML()

	if strings.Contains(xmlStr, "<Proc>") {
		t.Error("Should have escaped <Proc>")
	}
	if !strings.Contains(xmlStr, "&lt;") {
		t.Error("Should contain escaped < as &lt;")
	}
	if !strings.Contains(xmlStr, "&amp;") {
		t.Error("Should contain escaped & as &amp;")
	}
}
