// Package protogen provides proto file parsing and code generation.
package protogen

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ha1tch/tgpiler/storage"
)

// Parser reads and parses .proto files.
type Parser struct {
	// ImportPaths are directories to search for imports
	ImportPaths []string
}

// NewParser creates a new proto parser.
func NewParser(importPaths ...string) *Parser {
	return &Parser{
		ImportPaths: importPaths,
	}
}

// ParseFile parses a single .proto file.
func (p *Parser) ParseFile(path string) (*storage.ProtoFile, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	return p.Parse(f, path)
}

// ParseFiles parses multiple .proto files.
func (p *Parser) ParseFiles(paths ...string) (*storage.ProtoParseResult, error) {
	var files []storage.ProtoFile

	for _, path := range paths {
		pf, err := p.ParseFile(path)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		files = append(files, *pf)
	}

	return storage.NewProtoParseResult(files), nil
}

// ParseDir parses all .proto files in a directory.
func (p *Parser) ParseDir(dir string) (*storage.ProtoParseResult, error) {
	var paths []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".proto") {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk dir: %w", err)
	}

	return p.ParseFiles(paths...)
}

// Parse parses proto content from a reader.
func (p *Parser) Parse(r io.Reader, filename string) (*storage.ProtoFile, error) {
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read content: %w", err)
	}

	return p.parseContent(string(content), filename)
}

// parseContent does the actual parsing.
// This is a hand-rolled parser for proto3 syntax.
func (p *Parser) parseContent(content, filename string) (*storage.ProtoFile, error) {
	pf := &storage.ProtoFile{
		Path: filename,
	}

	lines := strings.Split(content, "\n")
	var currentMessage *storage.ProtoMessageInfo
	var currentService *storage.ProtoServiceInfo
	var currentEnum *storage.ProtoEnumInfo
	var messageDepth, serviceDepth, enumDepth int
	var fieldNumber int

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}

		// Remove inline comments
		if idx := strings.Index(line, "//"); idx != -1 {
			line = strings.TrimSpace(line[:idx])
		}

		// Count braces on this line
		openBraces := strings.Count(line, "{")
		closeBraces := strings.Count(line, "}")

		// Package declaration
		if strings.HasPrefix(line, "package ") {
			pf.Package = extractValue(line, "package ", ";")
			continue
		}

		// Option go_package
		if strings.Contains(line, "option go_package") {
			pf.GoPackage = extractQuoted(line)
			continue
		}

		// Import
		if strings.HasPrefix(line, "import ") {
			imp := extractQuoted(line)
			if imp != "" {
				pf.Imports = append(pf.Imports, imp)
			}
			continue
		}

		// Handle inline empty definitions like "message Request {}"
		if strings.HasPrefix(line, "message ") && strings.Contains(line, "{") && strings.HasSuffix(line, "}") {
			name := extractValue(line, "message ", " {")
			if name == "" {
				name = extractValue(line, "message ", "{")
			}
			pf.Messages = append(pf.Messages, storage.ProtoMessageInfo{
				Name:     strings.TrimSpace(name),
				FullName: pf.Package + "." + strings.TrimSpace(name),
				Package:  pf.Package,
			})
			continue
		}

		// Handle inline empty service (unlikely but possible)
		if strings.HasPrefix(line, "service ") && strings.Contains(line, "{") && strings.HasSuffix(line, "}") {
			name := extractValue(line, "service ", " {")
			if name == "" {
				name = extractValue(line, "service ", "{")
			}
			pf.Services = append(pf.Services, storage.ProtoServiceInfo{
				Name:     strings.TrimSpace(name),
				FullName: pf.Package + "." + strings.TrimSpace(name),
				Package:  pf.Package,
			})
			continue
		}

		// Message start
		if strings.HasPrefix(line, "message ") && strings.Contains(line, "{") {
			name := extractValue(line, "message ", " {")
			if name == "" {
				name = extractValue(line, "message ", "{")
			}
			currentMessage = &storage.ProtoMessageInfo{
				Name:     strings.TrimSpace(name),
				FullName: pf.Package + "." + strings.TrimSpace(name),
				Package:  pf.Package,
			}
			messageDepth = 1
			fieldNumber = 0
			continue
		}

		// Service start
		if strings.HasPrefix(line, "service ") && strings.Contains(line, "{") {
			name := extractValue(line, "service ", " {")
			if name == "" {
				name = extractValue(line, "service ", "{")
			}
			currentService = &storage.ProtoServiceInfo{
				Name:     strings.TrimSpace(name),
				FullName: pf.Package + "." + strings.TrimSpace(name),
				Package:  pf.Package,
			}
			serviceDepth = 1
			continue
		}

		// Enum start
		if strings.HasPrefix(line, "enum ") && strings.Contains(line, "{") {
			name := extractValue(line, "enum ", " {")
			if name == "" {
				name = extractValue(line, "enum ", "{")
			}
			currentEnum = &storage.ProtoEnumInfo{
				Name: strings.TrimSpace(name),
			}
			enumDepth = 1
			continue
		}

		// Track depth changes for messages
		if currentMessage != nil {
			messageDepth += openBraces - closeBraces
			if messageDepth <= 0 {
				pf.Messages = append(pf.Messages, *currentMessage)
				currentMessage = nil
				messageDepth = 0
				continue
			}
			// Parse message fields (only at depth 1 to avoid nested messages)
			if messageDepth == 1 && !strings.HasPrefix(line, "message ") && !strings.HasPrefix(line, "enum ") {
				if field := parseField(line, &fieldNumber); field != nil {
					currentMessage.Fields = append(currentMessage.Fields, *field)
				}
			}
			continue
		}

		// Track depth changes for services
		if currentService != nil {
			serviceDepth += openBraces - closeBraces
			if serviceDepth <= 0 {
				pf.Services = append(pf.Services, *currentService)
				currentService = nil
				serviceDepth = 0
				continue
			}
			// Parse service methods
			if strings.HasPrefix(line, "rpc ") {
				if method := parseMethod(line, currentService.Name); method != nil {
					currentService.Methods = append(currentService.Methods, *method)
				}
			}
			continue
		}

		// Track depth changes for enums
		if currentEnum != nil {
			enumDepth += openBraces - closeBraces
			if enumDepth <= 0 {
				pf.Enums = append(pf.Enums, *currentEnum)
				currentEnum = nil
				enumDepth = 0
				continue
			}
			// Parse enum values
			if ev := parseEnumValue(line); ev != nil {
				currentEnum.Values = append(currentEnum.Values, *ev)
			}
			continue
		}
	}

	return pf, nil
}

// parseField parses a proto field definition.
func parseField(line string, fieldNumber *int) *storage.ProtoFieldInfo {
	line = strings.TrimSpace(line)
	if line == "" || line == "}" || strings.HasPrefix(line, "//") {
		return nil
	}

	// Remove trailing semicolon
	line = strings.TrimSuffix(line, ";")

	field := &storage.ProtoFieldInfo{}

	// Check for optional
	if strings.HasPrefix(line, "optional ") {
		field.IsOptional = true
		line = strings.TrimPrefix(line, "optional ")
	}

	// Check for repeated
	if strings.HasPrefix(line, "repeated ") {
		field.IsRepeated = true
		line = strings.TrimPrefix(line, "repeated ")
	}

	// Check for map
	if strings.HasPrefix(line, "map<") {
		field.IsMap = true
		// Parse map<KeyType, ValueType>
		mapPart := extractBetween(line, "map<", ">")
		parts := strings.SplitN(mapPart, ",", 2)
		if len(parts) == 2 {
			field.MapKeyType = strings.TrimSpace(parts[0])
			field.MapValType = strings.TrimSpace(parts[1])
		}
		// Rest is name = number
		rest := line[strings.Index(line, ">")+1:]
		parts = strings.Fields(rest)
		if len(parts) >= 3 {
			field.Name = parts[0]
			field.Number = parseFieldNumber(parts[2])
		}
		return field
	}

	// Regular field: type name = number
	parts := strings.Fields(line)
	if len(parts) < 3 {
		return nil
	}

	field.ProtoType = parts[0]
	field.Name = parts[1]

	// Find the field number after "="
	for i, part := range parts {
		if part == "=" && i+1 < len(parts) {
			field.Number = parseFieldNumber(parts[i+1])
			break
		}
	}

	// Determine if it's a message type
	field.IsMessage = isMessageType(field.ProtoType)
	if field.IsMessage {
		field.MessageType = field.ProtoType
	}

	// Determine Go type
	field.GoType = protoToGoType(field.ProtoType, field.IsOptional, field.IsRepeated)

	// Check for enum
	if isEnumType(field.ProtoType) {
		field.IsEnum = true
		field.EnumType = field.ProtoType
	}

	*fieldNumber++
	if field.Number == 0 {
		field.Number = *fieldNumber
	}

	return field
}

// parseMethod parses an RPC method definition.
func parseMethod(line string, serviceName string) *storage.ProtoMethodInfo {
	// rpc MethodName(RequestType) returns (ResponseType) {}
	line = strings.TrimPrefix(line, "rpc ")
	line = strings.TrimSuffix(line, "{}")
	line = strings.TrimSuffix(line, ";")
	line = strings.TrimSpace(line)

	method := &storage.ProtoMethodInfo{}

	// Extract method name
	parenIdx := strings.Index(line, "(")
	if parenIdx == -1 {
		return nil
	}
	method.Name = strings.TrimSpace(line[:parenIdx])
	method.FullName = serviceName + "." + method.Name

	// Check for streaming
	if strings.Contains(line, "stream ") {
		// Client streaming: (stream RequestType)
		if strings.Contains(line[:strings.Index(line, "returns")], "stream ") {
			method.ClientStreaming = true
		}
		// Server streaming: returns (stream ResponseType)
		if strings.Contains(line[strings.Index(line, "returns"):], "stream ") {
			method.ServerStreaming = true
		}
	}

	// Extract request type
	reqStart := strings.Index(line, "(")
	reqEnd := strings.Index(line, ")")
	if reqStart != -1 && reqEnd != -1 {
		reqType := line[reqStart+1 : reqEnd]
		reqType = strings.TrimPrefix(reqType, "stream ")
		method.RequestType = strings.TrimSpace(reqType)
	}

	// Extract response type
	retIdx := strings.Index(line, "returns")
	if retIdx != -1 {
		rest := line[retIdx+7:]
		respStart := strings.Index(rest, "(")
		respEnd := strings.Index(rest, ")")
		if respStart != -1 && respEnd != -1 {
			respType := rest[respStart+1 : respEnd]
			respType = strings.TrimPrefix(respType, "stream ")
			method.ResponseType = strings.TrimSpace(respType)
		}
	}

	// Infer operation type
	method.InferredOp = method.InferOperationType()

	return method
}

// parseEnumValue parses an enum value definition.
func parseEnumValue(line string) *storage.ProtoEnumValue {
	line = strings.TrimSpace(line)
	line = strings.TrimSuffix(line, ";")

	if line == "" || line == "}" || strings.HasPrefix(line, "//") {
		return nil
	}

	// NAME = NUMBER
	parts := strings.Split(line, "=")
	if len(parts) != 2 {
		return nil
	}

	return &storage.ProtoEnumValue{
		Name:   strings.TrimSpace(parts[0]),
		Number: parseFieldNumber(strings.TrimSpace(parts[1])),
	}
}

// Helper functions

func extractValue(line, prefix, suffix string) string {
	line = strings.TrimPrefix(line, prefix)
	if idx := strings.Index(line, suffix); idx != -1 {
		return strings.TrimSpace(line[:idx])
	}
	return strings.TrimSpace(line)
}

func extractQuoted(line string) string {
	start := strings.Index(line, `"`)
	if start == -1 {
		return ""
	}
	end := strings.LastIndex(line, `"`)
	if end <= start {
		return ""
	}
	return line[start+1 : end]
}

func extractBetween(line, start, end string) string {
	startIdx := strings.Index(line, start)
	if startIdx == -1 {
		return ""
	}
	startIdx += len(start)
	endIdx := strings.Index(line[startIdx:], end)
	if endIdx == -1 {
		return ""
	}
	return line[startIdx : startIdx+endIdx]
}

func parseFieldNumber(s string) int {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, ";")
	var n int
	fmt.Sscanf(s, "%d", &n)
	return n
}

func isMessageType(t string) bool {
	// Scalar types in proto3
	scalars := map[string]bool{
		"double": true, "float": true,
		"int32": true, "int64": true,
		"uint32": true, "uint64": true,
		"sint32": true, "sint64": true,
		"fixed32": true, "fixed64": true,
		"sfixed32": true, "sfixed64": true,
		"bool": true, "string": true, "bytes": true,
	}
	return !scalars[t]
}

func isEnumType(t string) bool {
	// Heuristic: enums often end with "Status", "Type", "State", etc.
	// or are ALL_CAPS. This is imperfect without full context.
	return false // Conservative default
}

func protoToGoType(protoType string, optional, repeated bool) string {
	baseType := protoTypeToGo(protoType)

	if repeated {
		return "[]" + baseType
	}
	if optional && !isMessageType(protoType) {
		return "*" + baseType
	}
	if isMessageType(protoType) {
		return "*" + baseType
	}
	return baseType
}

func protoTypeToGo(t string) string {
	switch t {
	case "double":
		return "float64"
	case "float":
		return "float32"
	case "int32", "sint32", "sfixed32":
		return "int32"
	case "int64", "sint64", "sfixed64":
		return "int64"
	case "uint32", "fixed32":
		return "uint32"
	case "uint64", "fixed64":
		return "uint64"
	case "bool":
		return "bool"
	case "string":
		return "string"
	case "bytes":
		return "[]byte"
	default:
		// Message type - return as-is (will be pointer)
		return t
	}
}
