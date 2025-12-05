//go:build !mysql

package adapter

import (
	"context"
	"database/sql"
	"errors"
)

// MySQLAdapter implements Adapter for MySQL databases.
// This is a stub implementation when the mysql build tag is not set.
type MySQLAdapter struct {
	BaseAdapter
}

// ErrMySQLNotAvailable is returned when MySQL driver is not compiled in.
var ErrMySQLNotAvailable = errors.New("MySQL adapter not available: build with mysql tag")

// NewMySQLAdapter creates a new MySQL adapter.
func NewMySQLAdapter(config Config) *MySQLAdapter {
	if config.Port == 0 {
		config.Port = 3306
	}
	return &MySQLAdapter{
		BaseAdapter: BaseAdapter{config: config},
	}
}

// Open returns an error as MySQL is not available.
func (a *MySQLAdapter) Open(ctx context.Context) error {
	return ErrMySQLNotAvailable
}

// DialectName returns the dialect name.
func (a *MySQLAdapter) DialectName() string {
	return "mysql"
}

// DriverName returns the driver name.
func (a *MySQLAdapter) DriverName() string {
	return "mysql"
}

// LastInsertID returns an error as MySQL is not available.
func (a *MySQLAdapter) LastInsertID(ctx context.Context, table, idColumn string) (int64, error) {
	return 0, ErrMySQLNotAvailable
}

// TableExists returns an error as MySQL is not available.
func (a *MySQLAdapter) TableExists(ctx context.Context, table string) (bool, error) {
	return false, ErrMySQLNotAvailable
}

// GetTableColumns returns an error as MySQL is not available.
func (a *MySQLAdapter) GetTableColumns(ctx context.Context, table string) ([]ColumnInfo, error) {
	return nil, ErrMySQLNotAvailable
}

// GetVersion returns an error as MySQL is not available.
func (a *MySQLAdapter) GetVersion(ctx context.Context) (string, error) {
	return "", ErrMySQLNotAvailable
}

// GetDatabaseSize returns an error as MySQL is not available.
func (a *MySQLAdapter) GetDatabaseSize(ctx context.Context) (int64, error) {
	return 0, ErrMySQLNotAvailable
}

// GetTableSize returns an error as MySQL is not available.
func (a *MySQLAdapter) GetTableSize(ctx context.Context, table string) (int64, error) {
	return 0, ErrMySQLNotAvailable
}

// OptimizeTable returns an error as MySQL is not available.
func (a *MySQLAdapter) OptimizeTable(ctx context.Context, table string) error {
	return ErrMySQLNotAvailable
}

// AnalyzeTable returns an error as MySQL is not available.
func (a *MySQLAdapter) AnalyzeTable(ctx context.Context, table string) error {
	return ErrMySQLNotAvailable
}

// GetProcessList returns an error as MySQL is not available.
func (a *MySQLAdapter) GetProcessList(ctx context.Context) ([]map[string]interface{}, error) {
	return nil, ErrMySQLNotAvailable
}

// KillProcess returns an error as MySQL is not available.
func (a *MySQLAdapter) KillProcess(ctx context.Context, processID int64) error {
	return ErrMySQLNotAvailable
}

// GetVariables returns an error as MySQL is not available.
func (a *MySQLAdapter) GetVariables(ctx context.Context, like string) (map[string]string, error) {
	return nil, ErrMySQLNotAvailable
}

// SetSessionVariable returns an error as MySQL is not available.
func (a *MySQLAdapter) SetSessionVariable(ctx context.Context, name, value string) error {
	return ErrMySQLNotAvailable
}

// LoadDataInfile returns an error as MySQL is not available.
func (a *MySQLAdapter) LoadDataInfile(ctx context.Context, table, filepath string, columns []string) error {
	return ErrMySQLNotAvailable
}

// Ensure MySQLAdapter implements Adapter interface
var _ Adapter = (*MySQLAdapter)(nil)

// Query stub
func (a *MySQLAdapter) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return nil, ErrMySQLNotAvailable
}

// QueryRow stub
func (a *MySQLAdapter) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return nil
}

// Exec stub
func (a *MySQLAdapter) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return nil, ErrMySQLNotAvailable
}
