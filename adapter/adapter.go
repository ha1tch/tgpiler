// Package adapter provides database adapter implementations for different SQL dialects.
package adapter

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Adapter defines the interface for database operations.
type Adapter interface {
	// Connection management
	Open(ctx context.Context) error
	Close() error
	Ping(ctx context.Context) error

	// Query execution
	Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row
	Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error)

	// Transaction support
	Begin(ctx context.Context) (Tx, error)
	BeginTx(ctx context.Context, opts *sql.TxOptions) (Tx, error)

	// Metadata
	DialectName() string
	DriverName() string

	// Database-specific operations
	LastInsertID(ctx context.Context, table, idColumn string) (int64, error)

	// Health check
	HealthCheck(ctx context.Context) error
}

// Tx represents a database transaction.
type Tx interface {
	Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row
	Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	Commit() error
	Rollback() error
}

// Config holds common adapter configuration.
type Config struct {
	Host            string
	Port            int
	Database        string
	Username        string
	Password        string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration

	// SQLite specific
	FilePath string
	InMemory bool

	// Additional options as key-value pairs
	Options map[string]string
}

// DefaultConfig returns a config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		Host:            "localhost",
		Port:            5432,
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
		ConnMaxIdleTime: 1 * time.Minute,
		SSLMode:         "disable",
		Options:         make(map[string]string),
	}
}

// BaseAdapter provides common functionality for all adapters.
type BaseAdapter struct {
	db     *sql.DB
	config Config
}

// DB returns the underlying database connection.
func (a *BaseAdapter) DB() *sql.DB {
	return a.db
}

// Close closes the database connection.
func (a *BaseAdapter) Close() error {
	if a.db != nil {
		return a.db.Close()
	}
	return nil
}

// Ping verifies the database connection.
func (a *BaseAdapter) Ping(ctx context.Context) error {
	return a.db.PingContext(ctx)
}

// Query executes a query that returns rows.
func (a *BaseAdapter) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return a.db.QueryContext(ctx, query, args...)
}

// QueryRow executes a query that returns a single row.
func (a *BaseAdapter) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return a.db.QueryRowContext(ctx, query, args...)
}

// Exec executes a query that doesn't return rows.
func (a *BaseAdapter) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return a.db.ExecContext(ctx, query, args...)
}

// Begin starts a transaction with default options.
func (a *BaseAdapter) Begin(ctx context.Context) (Tx, error) {
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &txWrapper{tx: tx}, nil
}

// BeginTx starts a transaction with the given options.
func (a *BaseAdapter) BeginTx(ctx context.Context, opts *sql.TxOptions) (Tx, error) {
	tx, err := a.db.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &txWrapper{tx: tx}, nil
}

// HealthCheck performs a basic health check.
func (a *BaseAdapter) HealthCheck(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := a.db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping failed: %w", err)
	}

	// Try a simple query
	var result int
	err := a.db.QueryRowContext(ctx, "SELECT 1").Scan(&result)
	if err != nil {
		return fmt.Errorf("test query failed: %w", err)
	}

	return nil
}

// configurePool sets connection pool parameters.
func (a *BaseAdapter) configurePool() {
	if a.config.MaxOpenConns > 0 {
		a.db.SetMaxOpenConns(a.config.MaxOpenConns)
	}
	if a.config.MaxIdleConns > 0 {
		a.db.SetMaxIdleConns(a.config.MaxIdleConns)
	}
	if a.config.ConnMaxLifetime > 0 {
		a.db.SetConnMaxLifetime(a.config.ConnMaxLifetime)
	}
	if a.config.ConnMaxIdleTime > 0 {
		a.db.SetConnMaxIdleTime(a.config.ConnMaxIdleTime)
	}
}

// txWrapper wraps sql.Tx to implement the Tx interface.
type txWrapper struct {
	tx *sql.Tx
}

func (t *txWrapper) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return t.tx.QueryContext(ctx, query, args...)
}

func (t *txWrapper) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return t.tx.QueryRowContext(ctx, query, args...)
}

func (t *txWrapper) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return t.tx.ExecContext(ctx, query, args...)
}

func (t *txWrapper) Commit() error {
	return t.tx.Commit()
}

func (t *txWrapper) Rollback() error {
	return t.tx.Rollback()
}

// ScanRow is a helper to scan a single row into a map.
func ScanRow(rows *sql.Rows) (map[string]interface{}, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	if err := rows.Scan(valuePtrs...); err != nil {
		return nil, err
	}

	result := make(map[string]interface{})
	for i, col := range columns {
		result[col] = values[i]
	}

	return result, nil
}

// ScanRows is a helper to scan all rows into a slice of maps.
func ScanRows(rows *sql.Rows) ([]map[string]interface{}, error) {
	var results []map[string]interface{}

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}

		row := make(map[string]interface{})
		for i, col := range columns {
			row[col] = values[i]
		}
		results = append(results, row)
	}

	return results, rows.Err()
}
