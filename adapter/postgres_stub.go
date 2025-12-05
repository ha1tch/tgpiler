//go:build !postgres

package adapter

import (
	"context"
	"database/sql"
	"errors"
)

// PostgresAdapter implements Adapter for PostgreSQL databases.
// This is a stub implementation when the postgres build tag is not set.
type PostgresAdapter struct {
	BaseAdapter
}

// ErrPostgresNotAvailable is returned when PostgreSQL driver is not compiled in.
var ErrPostgresNotAvailable = errors.New("PostgreSQL adapter not available: build with postgres tag")

// NewPostgresAdapter creates a new PostgreSQL adapter.
func NewPostgresAdapter(config Config) *PostgresAdapter {
	if config.Port == 0 {
		config.Port = 5432
	}
	return &PostgresAdapter{
		BaseAdapter: BaseAdapter{config: config},
	}
}

// Open returns an error as PostgreSQL is not available.
func (a *PostgresAdapter) Open(ctx context.Context) error {
	return ErrPostgresNotAvailable
}

// DialectName returns the dialect name.
func (a *PostgresAdapter) DialectName() string {
	return "postgres"
}

// DriverName returns the driver name.
func (a *PostgresAdapter) DriverName() string {
	return "pgx"
}

// LastInsertID returns an error as PostgreSQL is not available.
func (a *PostgresAdapter) LastInsertID(ctx context.Context, table, idColumn string) (int64, error) {
	return 0, ErrPostgresNotAvailable
}

// TableExists returns an error as PostgreSQL is not available.
func (a *PostgresAdapter) TableExists(ctx context.Context, table string) (bool, error) {
	return false, ErrPostgresNotAvailable
}

// GetTableColumns returns an error as PostgreSQL is not available.
func (a *PostgresAdapter) GetTableColumns(ctx context.Context, table string) ([]ColumnInfo, error) {
	return nil, ErrPostgresNotAvailable
}

// CreateSchema returns an error as PostgreSQL is not available.
func (a *PostgresAdapter) CreateSchema(ctx context.Context, schema string) error {
	return ErrPostgresNotAvailable
}

// Vacuum returns an error as PostgreSQL is not available.
func (a *PostgresAdapter) Vacuum(ctx context.Context, table string) error {
	return ErrPostgresNotAvailable
}

// GetDatabaseSize returns an error as PostgreSQL is not available.
func (a *PostgresAdapter) GetDatabaseSize(ctx context.Context) (int64, error) {
	return 0, ErrPostgresNotAvailable
}

// GetTableSize returns an error as PostgreSQL is not available.
func (a *PostgresAdapter) GetTableSize(ctx context.Context, table string) (int64, error) {
	return 0, ErrPostgresNotAvailable
}

// GetActiveConnections returns an error as PostgreSQL is not available.
func (a *PostgresAdapter) GetActiveConnections(ctx context.Context) (int, error) {
	return 0, ErrPostgresNotAvailable
}

// Listen returns an error as PostgreSQL is not available.
func (a *PostgresAdapter) Listen(ctx context.Context, channel string) error {
	return ErrPostgresNotAvailable
}

// Notify returns an error as PostgreSQL is not available.
func (a *PostgresAdapter) Notify(ctx context.Context, channel, payload string) error {
	return ErrPostgresNotAvailable
}

// CopyFrom returns an error as PostgreSQL is not available.
func (a *PostgresAdapter) CopyFrom(ctx context.Context, table string, columns []string, values [][]interface{}) (int64, error) {
	return 0, ErrPostgresNotAvailable
}

// Ensure PostgresAdapter implements Adapter interface
var _ Adapter = (*PostgresAdapter)(nil)

// Query stub
func (a *PostgresAdapter) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return nil, ErrPostgresNotAvailable
}

// QueryRow stub
func (a *PostgresAdapter) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return nil
}

// Exec stub
func (a *PostgresAdapter) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return nil, ErrPostgresNotAvailable
}
