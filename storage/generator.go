package storage

import (
	"context"
	"io"
)

// Generator is the main interface for generating storage layer code.
type Generator interface {
	// GenerateInterfaces generates the repository interfaces (common for all backends).
	GenerateInterfaces(ctx context.Context, spec *GenerationSpec, w io.Writer) error
	
	// GenerateModels generates the model structs (common for all backends).
	GenerateModels(ctx context.Context, spec *GenerationSpec, w io.Writer) error
	
	// GenerateBackend generates backend-specific implementation.
	GenerateBackend(ctx context.Context, spec *GenerationSpec, backend BackendGenerator) error
}

// BackendCapabilities describes what a backend supports.
type BackendCapabilities struct {
	// Supports transactions (Begin/Commit/Rollback)
	SupportsTransactions bool
	
	// Supports batch operations
	SupportsBatch bool
	
	// Supports streaming results
	SupportsStreaming bool
	
	// Supports prepared statements / query caching
	SupportsPrepared bool
	
	// Supports schema generation
	SupportsSchemaGen bool
	
	// Supports seed data generation
	SupportsSeedGen bool
	
	// Requires external dependencies (e.g., proto files for gRPC)
	RequiresExternalDeps bool
	ExternalDepTypes     []string // e.g., ["proto"]
}

// SQLDialect describes SQL dialect differences for SQL backends.
type SQLDialect interface {
	// Name returns the dialect name.
	Name() string
	
	// Placeholder returns the parameter placeholder for position n.
	// PostgreSQL: $1, $2, $3...
	// MySQL/SQLite: ?
	// Oracle: :1, :2, :3...
	// SQL Server: @p1, @p2, @p3...
	Placeholder(n int) string
	
	// QuoteIdentifier quotes an identifier (table/column name).
	// PostgreSQL: "name"
	// MySQL: `name`
	// SQL Server: [name]
	QuoteIdentifier(name string) string
	
	// TableAlias returns how to write a table alias.
	// PostgreSQL/MySQL/SQLite/SQL Server: "table AS alias" or "table alias"
	// Oracle: "table alias" (AS is forbidden!)
	TableAlias(table, alias string) string
	
	// SupportsTableAliasAS returns whether AS keyword is allowed for table aliases.
	// All except Oracle: true
	// Oracle: false
	SupportsTableAliasAS() bool
	
	// LastInsertIDMethod returns the method to get last insert ID.
	// PostgreSQL: RETURNING id
	// MySQL: LAST_INSERT_ID()
	// SQLite: last_insert_rowid()
	// SQL Server: SCOPE_IDENTITY() or OUTPUT clause
	// Oracle: RETURNING INTO
	LastInsertIDMethod() string
	
	// SupportsReturning returns whether INSERT ... RETURNING is supported.
	// PostgreSQL, SQLite 3.35+: true
	// MySQL, Oracle (without INTO): false
	// SQL Server: false (uses OUTPUT instead)
	SupportsReturning() bool
	
	// SupportsOutputClause returns whether OUTPUT clause is supported (SQL Server).
	SupportsOutputClause() bool
	
	// BooleanLiteral returns how to represent true/false.
	BooleanLiteral(b bool) string
	
	// LimitClause returns how to limit results.
	// PostgreSQL/MySQL/SQLite: LIMIT n
	// SQL Server: TOP n (in SELECT)
	// Oracle: FETCH FIRST n ROWS ONLY
	LimitClause(n int) string
	
	// LimitPosition returns where LIMIT goes.
	// Most: "end" (after WHERE/ORDER BY)
	// SQL Server: "select" (TOP after SELECT keyword)
	LimitPosition() string
	
	// OffsetFetchClause returns SQL standard OFFSET/FETCH syntax (if supported).
	// Returns empty string if not supported.
	OffsetFetchClause(offset, limit int) string
	
	// NullSafeEqual returns a null-safe equality operator or function.
	// PostgreSQL: IS NOT DISTINCT FROM
	// MySQL: <=>
	// SQLite: IS
	// Others: (a = b OR (a IS NULL AND b IS NULL))
	NullSafeEqual(left, right string) string
	
	// StringConcat returns how to concatenate strings.
	// PostgreSQL/SQLite/Oracle: ||
	// SQL Server: +
	// MySQL: CONCAT() function
	StringConcat(parts ...string) string
	
	// UpdateJoinSyntax returns how UPDATE with JOIN is written.
	// SQL Server: UPDATE t SET ... FROM t JOIN o ON ...
	// PostgreSQL: UPDATE t SET ... FROM o WHERE t.id = o.id
	// MySQL: UPDATE t JOIN o ON ... SET t.col = ...
	// SQLite/Oracle: subquery or not supported
	UpdateJoinSyntax() UpdateJoinStyle
	
	// DeleteJoinSyntax returns how DELETE with JOIN is written.
	// SQL Server: DELETE t FROM t JOIN o ON ...
	// PostgreSQL: DELETE FROM t USING o WHERE ...
	// MySQL: DELETE t FROM t JOIN o ON ...
	// SQLite/Oracle: subquery
	DeleteJoinSyntax() DeleteJoinStyle
	
	// UpsertSyntax returns how UPSERT/MERGE is written.
	UpsertSyntax() UpsertStyle
	
	// NeedsFromDual returns whether SELECT without FROM needs FROM DUAL.
	// Oracle: true
	// Others: false
	NeedsFromDual() bool
	
	// TypeMapping returns the native type for a Go type.
	TypeMapping(goType string) string
	
	// DriverName returns the database/sql driver name.
	DriverName() string
	
	// ConnectionStringFormat returns a sample connection string format.
	ConnectionStringFormat() string
}

// UpdateJoinStyle describes how a dialect handles UPDATE with JOIN.
type UpdateJoinStyle int

const (
	UpdateJoinNotSupported UpdateJoinStyle = iota // Must use subquery
	UpdateJoinFromClause                          // UPDATE t SET ... FROM t JOIN o (SQL Server)
	UpdateJoinFromWhere                           // UPDATE t SET ... FROM o WHERE t.id = o.id (PostgreSQL)
	UpdateJoinDirect                              // UPDATE t JOIN o SET ... (MySQL)
)

// DeleteJoinStyle describes how a dialect handles DELETE with JOIN.
type DeleteJoinStyle int

const (
	DeleteJoinNotSupported DeleteJoinStyle = iota // Must use subquery
	DeleteJoinFromClause                          // DELETE t FROM t JOIN o (SQL Server, MySQL)
	DeleteJoinUsing                               // DELETE FROM t USING o WHERE ... (PostgreSQL)
)

// UpsertStyle describes how a dialect handles UPSERT operations.
type UpsertStyle int

const (
	UpsertNotSupported   UpsertStyle = iota // No native support
	UpsertMerge                             // MERGE statement (SQL Server, Oracle, PostgreSQL 15+)
	UpsertOnConflict                        // INSERT ... ON CONFLICT (PostgreSQL 9.5+, SQLite 3.24+)
	UpsertOnDuplicateKey                    // INSERT ... ON DUPLICATE KEY UPDATE (MySQL)
)

// BaseBackendGenerator provides common functionality for backend generators.
type BaseBackendGenerator struct {
	Config BackendConfig
}

// GenerateFileHeader generates common file header.
func (g *BaseBackendGenerator) GenerateFileHeader(w io.Writer, pkg string) error {
	header := "// Code generated by tgpiler. DO NOT EDIT.\n\n"
	header += "package " + pkg + "\n\n"
	_, err := io.WriteString(w, header)
	return err
}

// OutputFile describes a file to be generated.
type OutputFile struct {
	Path     string
	Content  []byte
	Mode     int // File mode (e.g., 0644)
}

// GenerationOutput is the result of a generation run.
type GenerationOutput struct {
	Files []OutputFile
	
	// For post-generation steps
	Commands []string // e.g., "go mod tidy", "go generate"
}

// TemplateData is passed to code generation templates.
type TemplateData struct {
	// Package info
	Package     string
	Imports     []string
	
	// Generation info
	GeneratedBy string
	SourceFiles []string
	
	// Content
	Models       []Model
	Repositories []Repository
	Operations   []Operation
	
	// Backend specific
	Backend     BackendType
	Dialect     SQLDialect
	
	// Proto info (for gRPC)
	ProtoPackage  string
	ProtoServices []ProtoServiceInfo
	ProtoMappings []SQLToProtoMapping
}

// RepositoryTemplate is data for generating a single repository implementation.
type RepositoryTemplate struct {
	// Interface info
	InterfaceName string
	EntityName    string
	TableName     string
	
	// Methods to generate
	Methods []MethodTemplate
	
	// Backend specific
	Dialect SQLDialect
	
	// Proto info (for gRPC backend)
	GRPCClient   string // Client field name
	GRPCService  string // Service name in proto
}

// MethodTemplate is data for generating a single method.
type MethodTemplate struct {
	// Method signature
	Name       string
	Receiver   string
	Params     []ParamTemplate
	Returns    []string
	
	// Implementation details
	Operation   Operation
	Pattern     SelectPattern
	
	// SQL backend
	SQL         string // Generated SQL
	ScanFields  []string // Fields to scan into
	
	// gRPC backend
	GRPCMethod  string // Proto method name
	RequestMapping map[string]string
	ResponseMapping map[string]string
}

// ParamTemplate describes a method parameter.
type ParamTemplate struct {
	Name string
	Type string
}
