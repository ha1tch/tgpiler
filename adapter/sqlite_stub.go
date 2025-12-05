//go:build !cgo || !sqlite

package adapter

import (
	"context"
	"database/sql"
	"errors"
)

// SQLiteAdapter implements Adapter for SQLite databases.
// This is a stub implementation when CGO is not available.
type SQLiteAdapter struct {
	BaseAdapter
}

// ErrSQLiteNotAvailable is returned when SQLite is not compiled in.
var ErrSQLiteNotAvailable = errors.New("SQLite adapter not available: build with CGO and sqlite tag")

// NewSQLiteAdapter creates a new SQLite adapter.
func NewSQLiteAdapter(config Config) *SQLiteAdapter {
	return &SQLiteAdapter{
		BaseAdapter: BaseAdapter{config: config},
	}
}

// NewSQLiteMemory creates an in-memory SQLite adapter for testing.
func NewSQLiteMemory() *SQLiteAdapter {
	return &SQLiteAdapter{
		BaseAdapter: BaseAdapter{
			config: Config{InMemory: true},
		},
	}
}

// NewSQLiteFile creates a SQLite adapter for a file database.
func NewSQLiteFile(path string) *SQLiteAdapter {
	return &SQLiteAdapter{
		BaseAdapter: BaseAdapter{
			config: Config{FilePath: path},
		},
	}
}

// Open returns an error as SQLite is not available.
func (a *SQLiteAdapter) Open(ctx context.Context) error {
	return ErrSQLiteNotAvailable
}

// DialectName returns the dialect name.
func (a *SQLiteAdapter) DialectName() string {
	return "sqlite"
}

// DriverName returns the driver name.
func (a *SQLiteAdapter) DriverName() string {
	return "sqlite3"
}

// LastInsertID returns an error as SQLite is not available.
func (a *SQLiteAdapter) LastInsertID(ctx context.Context, table, idColumn string) (int64, error) {
	return 0, ErrSQLiteNotAvailable
}

// CreateTable returns an error as SQLite is not available.
func (a *SQLiteAdapter) CreateTable(ctx context.Context, ddl string) error {
	return ErrSQLiteNotAvailable
}

// TableExists returns an error as SQLite is not available.
func (a *SQLiteAdapter) TableExists(ctx context.Context, table string) (bool, error) {
	return false, ErrSQLiteNotAvailable
}

// GetTableColumns returns an error as SQLite is not available.
func (a *SQLiteAdapter) GetTableColumns(ctx context.Context, table string) ([]ColumnInfo, error) {
	return nil, ErrSQLiteNotAvailable
}

// Vacuum returns an error as SQLite is not available.
func (a *SQLiteAdapter) Vacuum(ctx context.Context) error {
	return ErrSQLiteNotAvailable
}

// Analyze returns an error as SQLite is not available.
func (a *SQLiteAdapter) Analyze(ctx context.Context) error {
	return ErrSQLiteNotAvailable
}

// ColumnInfo holds metadata about a database column.
type ColumnInfo struct {
	Name         string
	DataType     string
	IsNullable   bool
	IsPrimaryKey bool
	DefaultValue string
}

// Ensure SQLiteAdapter implements Adapter interface
var _ Adapter = (*SQLiteAdapter)(nil)

// Query stub
func (a *SQLiteAdapter) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return nil, ErrSQLiteNotAvailable
}

// QueryRow stub
func (a *SQLiteAdapter) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return nil
}

// Exec stub
func (a *SQLiteAdapter) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return nil, ErrSQLiteNotAvailable
}
