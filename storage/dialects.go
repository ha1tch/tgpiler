package storage

import (
	"fmt"
)

// PostgresDialect implements SQLDialect for PostgreSQL.
type PostgresDialect struct{}

func (d PostgresDialect) Name() string { return "postgres" }

func (d PostgresDialect) Placeholder(n int) string {
	return fmt.Sprintf("$%d", n)
}

func (d PostgresDialect) QuoteIdentifier(name string) string {
	return `"` + name + `"`
}

func (d PostgresDialect) TableAlias(table, alias string) string {
	return table + " AS " + alias
}

func (d PostgresDialect) SupportsTableAliasAS() bool {
	return true
}

func (d PostgresDialect) LastInsertIDMethod() string {
	return "RETURNING"
}

func (d PostgresDialect) SupportsReturning() bool {
	return true
}

func (d PostgresDialect) SupportsOutputClause() bool {
	return false
}

func (d PostgresDialect) BooleanLiteral(b bool) string {
	if b {
		return "TRUE"
	}
	return "FALSE"
}

func (d PostgresDialect) LimitClause(n int) string {
	return fmt.Sprintf("LIMIT %d", n)
}

func (d PostgresDialect) LimitPosition() string {
	return "end"
}

func (d PostgresDialect) OffsetFetchClause(offset, limit int) string {
	return fmt.Sprintf("OFFSET %d ROWS FETCH FIRST %d ROWS ONLY", offset, limit)
}

func (d PostgresDialect) NullSafeEqual(left, right string) string {
	return fmt.Sprintf("%s IS NOT DISTINCT FROM %s", left, right)
}

func (d PostgresDialect) StringConcat(parts ...string) string {
	if len(parts) == 0 {
		return "''"
	}
	result := parts[0]
	for i := 1; i < len(parts); i++ {
		result += " || " + parts[i]
	}
	return result
}

func (d PostgresDialect) UpdateJoinSyntax() UpdateJoinStyle {
	return UpdateJoinFromWhere
}

func (d PostgresDialect) DeleteJoinSyntax() DeleteJoinStyle {
	return DeleteJoinUsing
}

func (d PostgresDialect) UpsertSyntax() UpsertStyle {
	return UpsertOnConflict
}

func (d PostgresDialect) NeedsFromDual() bool {
	return false
}

func (d PostgresDialect) TypeMapping(goType string) string {
	switch goType {
	case "int32":
		return "INTEGER"
	case "int64":
		return "BIGINT"
	case "float64":
		return "DOUBLE PRECISION"
	case "decimal.Decimal":
		return "NUMERIC"
	case "string":
		return "TEXT"
	case "*string":
		return "TEXT"
	case "bool":
		return "BOOLEAN"
	case "*bool":
		return "BOOLEAN"
	case "time.Time":
		return "TIMESTAMP WITH TIME ZONE"
	case "*time.Time":
		return "TIMESTAMP WITH TIME ZONE"
	case "[]byte":
		return "BYTEA"
	default:
		return "TEXT"
	}
}

func (d PostgresDialect) DriverName() string {
	return "pgx" // or "postgres"
}

func (d PostgresDialect) ConnectionStringFormat() string {
	return "postgres://user:password@host:5432/dbname?sslmode=disable"
}

// MySQLDialect implements SQLDialect for MySQL/MariaDB.
type MySQLDialect struct{}

func (d MySQLDialect) Name() string { return "mysql" }

func (d MySQLDialect) Placeholder(n int) string {
	return "?"
}

func (d MySQLDialect) QuoteIdentifier(name string) string {
	return "`" + name + "`"
}

func (d MySQLDialect) TableAlias(table, alias string) string {
	return table + " AS " + alias
}

func (d MySQLDialect) SupportsTableAliasAS() bool {
	return true
}

func (d MySQLDialect) LastInsertIDMethod() string {
	return "LAST_INSERT_ID()"
}

func (d MySQLDialect) SupportsReturning() bool {
	return false
}

func (d MySQLDialect) SupportsOutputClause() bool {
	return false
}

func (d MySQLDialect) BooleanLiteral(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

func (d MySQLDialect) LimitClause(n int) string {
	return fmt.Sprintf("LIMIT %d", n)
}

func (d MySQLDialect) LimitPosition() string {
	return "end"
}

func (d MySQLDialect) OffsetFetchClause(offset, limit int) string {
	return fmt.Sprintf("LIMIT %d OFFSET %d", limit, offset)
}

func (d MySQLDialect) NullSafeEqual(left, right string) string {
	return fmt.Sprintf("%s <=> %s", left, right)
}

func (d MySQLDialect) StringConcat(parts ...string) string {
	if len(parts) == 0 {
		return "''"
	}
	result := "CONCAT(" + parts[0]
	for i := 1; i < len(parts); i++ {
		result += ", " + parts[i]
	}
	return result + ")"
}

func (d MySQLDialect) UpdateJoinSyntax() UpdateJoinStyle {
	return UpdateJoinDirect
}

func (d MySQLDialect) DeleteJoinSyntax() DeleteJoinStyle {
	return DeleteJoinFromClause
}

func (d MySQLDialect) UpsertSyntax() UpsertStyle {
	return UpsertOnDuplicateKey
}

func (d MySQLDialect) NeedsFromDual() bool {
	return false // MySQL doesn't require DUAL, but supports it
}

func (d MySQLDialect) TypeMapping(goType string) string {
	switch goType {
	case "int32":
		return "INT"
	case "int64":
		return "BIGINT"
	case "float64":
		return "DOUBLE"
	case "decimal.Decimal":
		return "DECIMAL(18,4)"
	case "string":
		return "VARCHAR(255)"
	case "*string":
		return "VARCHAR(255)"
	case "bool":
		return "TINYINT(1)"
	case "*bool":
		return "TINYINT(1)"
	case "time.Time":
		return "DATETIME"
	case "*time.Time":
		return "DATETIME"
	case "[]byte":
		return "BLOB"
	default:
		return "TEXT"
	}
}

func (d MySQLDialect) DriverName() string {
	return "mysql"
}

func (d MySQLDialect) ConnectionStringFormat() string {
	return "user:password@tcp(host:3306)/dbname?parseTime=true"
}

// SQLiteDialect implements SQLDialect for SQLite.
type SQLiteDialect struct{}

func (d SQLiteDialect) Name() string { return "sqlite" }

func (d SQLiteDialect) Placeholder(n int) string {
	return "?"
}

func (d SQLiteDialect) QuoteIdentifier(name string) string {
	return `"` + name + `"`
}

func (d SQLiteDialect) TableAlias(table, alias string) string {
	return table + " AS " + alias
}

func (d SQLiteDialect) SupportsTableAliasAS() bool {
	return true
}

func (d SQLiteDialect) LastInsertIDMethod() string {
	return "last_insert_rowid()"
}

func (d SQLiteDialect) SupportsReturning() bool {
	return true // SQLite 3.35+
}

func (d SQLiteDialect) SupportsOutputClause() bool {
	return false
}

func (d SQLiteDialect) BooleanLiteral(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

func (d SQLiteDialect) LimitClause(n int) string {
	return fmt.Sprintf("LIMIT %d", n)
}

func (d SQLiteDialect) LimitPosition() string {
	return "end"
}

func (d SQLiteDialect) OffsetFetchClause(offset, limit int) string {
	return fmt.Sprintf("LIMIT %d OFFSET %d", limit, offset)
}

func (d SQLiteDialect) NullSafeEqual(left, right string) string {
	return fmt.Sprintf("%s IS %s", left, right)
}

func (d SQLiteDialect) StringConcat(parts ...string) string {
	if len(parts) == 0 {
		return "''"
	}
	result := parts[0]
	for i := 1; i < len(parts); i++ {
		result += " || " + parts[i]
	}
	return result
}

func (d SQLiteDialect) UpdateJoinSyntax() UpdateJoinStyle {
	return UpdateJoinFromWhere // SQLite 3.33+ supports UPDATE FROM
}

func (d SQLiteDialect) DeleteJoinSyntax() DeleteJoinStyle {
	return DeleteJoinNotSupported // Must use subquery
}

func (d SQLiteDialect) UpsertSyntax() UpsertStyle {
	return UpsertOnConflict // SQLite 3.24+
}

func (d SQLiteDialect) NeedsFromDual() bool {
	return false
}

func (d SQLiteDialect) TypeMapping(goType string) string {
	switch goType {
	case "int32", "int64":
		return "INTEGER"
	case "*int32", "*int64":
		return "INTEGER"
	case "float64", "decimal.Decimal":
		return "REAL"
	case "*float64":
		return "REAL"
	case "string":
		return "TEXT"
	case "*string":
		return "TEXT"
	case "bool":
		return "INTEGER"
	case "*bool":
		return "INTEGER"
	case "time.Time":
		return "TEXT" // ISO8601 string
	case "*time.Time":
		return "TEXT"
	case "[]byte":
		return "BLOB"
	default:
		return "TEXT"
	}
}

func (d SQLiteDialect) DriverName() string {
	return "sqlite3"
}

func (d SQLiteDialect) ConnectionStringFormat() string {
	return "file:database.db?cache=shared&mode=rwc"
}

// SQLServerDialect implements SQLDialect for Microsoft SQL Server.
type SQLServerDialect struct{}

func (d SQLServerDialect) Name() string { return "sqlserver" }

func (d SQLServerDialect) Placeholder(n int) string {
	return fmt.Sprintf("@p%d", n)
}

func (d SQLServerDialect) QuoteIdentifier(name string) string {
	return "[" + name + "]"
}

func (d SQLServerDialect) TableAlias(table, alias string) string {
	return table + " AS " + alias
}

func (d SQLServerDialect) SupportsTableAliasAS() bool {
	return true
}

func (d SQLServerDialect) LastInsertIDMethod() string {
	return "SCOPE_IDENTITY()"
}

func (d SQLServerDialect) SupportsReturning() bool {
	return false // Uses OUTPUT clause instead
}

func (d SQLServerDialect) SupportsOutputClause() bool {
	return true
}

func (d SQLServerDialect) BooleanLiteral(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

func (d SQLServerDialect) LimitClause(n int) string {
	// SQL Server uses TOP in SELECT, not LIMIT
	return fmt.Sprintf("TOP %d", n)
}

func (d SQLServerDialect) LimitPosition() string {
	return "select" // TOP goes after SELECT keyword
}

func (d SQLServerDialect) OffsetFetchClause(offset, limit int) string {
	return fmt.Sprintf("OFFSET %d ROWS FETCH NEXT %d ROWS ONLY", offset, limit)
}

func (d SQLServerDialect) NullSafeEqual(left, right string) string {
	// SQL Server doesn't have a null-safe equal operator
	return fmt.Sprintf("((%s = %s) OR (%s IS NULL AND %s IS NULL))", left, right, left, right)
}

func (d SQLServerDialect) StringConcat(parts ...string) string {
	if len(parts) == 0 {
		return "''"
	}
	result := parts[0]
	for i := 1; i < len(parts); i++ {
		result += " + " + parts[i]
	}
	return result
}

func (d SQLServerDialect) UpdateJoinSyntax() UpdateJoinStyle {
	return UpdateJoinFromClause
}

func (d SQLServerDialect) DeleteJoinSyntax() DeleteJoinStyle {
	return DeleteJoinFromClause
}

func (d SQLServerDialect) UpsertSyntax() UpsertStyle {
	return UpsertMerge
}

func (d SQLServerDialect) NeedsFromDual() bool {
	return false
}

func (d SQLServerDialect) TypeMapping(goType string) string {
	switch goType {
	case "int32":
		return "INT"
	case "*int32":
		return "INT"
	case "int64":
		return "BIGINT"
	case "*int64":
		return "BIGINT"
	case "float64":
		return "FLOAT"
	case "*float64":
		return "FLOAT"
	case "decimal.Decimal":
		return "DECIMAL(18,4)"
	case "string":
		return "NVARCHAR(MAX)"
	case "*string":
		return "NVARCHAR(MAX)"
	case "bool":
		return "BIT"
	case "*bool":
		return "BIT"
	case "time.Time":
		return "DATETIME2"
	case "*time.Time":
		return "DATETIME2"
	case "[]byte":
		return "VARBINARY(MAX)"
	default:
		return "NVARCHAR(MAX)"
	}
}

func (d SQLServerDialect) DriverName() string {
	return "sqlserver"
}

func (d SQLServerDialect) ConnectionStringFormat() string {
	return "sqlserver://user:password@host:1433?database=dbname"
}

// OracleDialect implements SQLDialect for Oracle Database.
type OracleDialect struct{}

func (d OracleDialect) Name() string { return "oracle" }

func (d OracleDialect) Placeholder(n int) string {
	return fmt.Sprintf(":p%d", n)
}

func (d OracleDialect) QuoteIdentifier(name string) string {
	return `"` + name + `"`
}

func (d OracleDialect) TableAlias(table, alias string) string {
	// Oracle does NOT allow AS keyword for table aliases!
	return table + " " + alias
}

func (d OracleDialect) SupportsTableAliasAS() bool {
	return false // AS is forbidden for table aliases in Oracle!
}

func (d OracleDialect) LastInsertIDMethod() string {
	return "RETURNING INTO"
}

func (d OracleDialect) SupportsReturning() bool {
	return false // Oracle RETURNING requires INTO variable (PL/SQL only)
}

func (d OracleDialect) SupportsOutputClause() bool {
	return false
}

func (d OracleDialect) BooleanLiteral(b bool) string {
	// Oracle has no boolean type in SQL
	if b {
		return "1"
	}
	return "0"
}

func (d OracleDialect) LimitClause(n int) string {
	return fmt.Sprintf("FETCH FIRST %d ROWS ONLY", n)
}

func (d OracleDialect) LimitPosition() string {
	return "end"
}

func (d OracleDialect) OffsetFetchClause(offset, limit int) string {
	return fmt.Sprintf("OFFSET %d ROWS FETCH NEXT %d ROWS ONLY", offset, limit)
}

func (d OracleDialect) NullSafeEqual(left, right string) string {
	// Oracle: use DECODE or NVL trick
	return fmt.Sprintf("DECODE(%s, %s, 1, 0) = 1", left, right)
}

func (d OracleDialect) StringConcat(parts ...string) string {
	if len(parts) == 0 {
		return "''"
	}
	result := parts[0]
	for i := 1; i < len(parts); i++ {
		result += " || " + parts[i]
	}
	return result
}

func (d OracleDialect) UpdateJoinSyntax() UpdateJoinStyle {
	return UpdateJoinNotSupported // Must use subquery or MERGE
}

func (d OracleDialect) DeleteJoinSyntax() DeleteJoinStyle {
	return DeleteJoinNotSupported // Must use subquery
}

func (d OracleDialect) UpsertSyntax() UpsertStyle {
	return UpsertMerge
}

func (d OracleDialect) NeedsFromDual() bool {
	return true // SELECT 1 FROM DUAL
}

func (d OracleDialect) TypeMapping(goType string) string {
	switch goType {
	case "int32":
		return "NUMBER(10)"
	case "*int32":
		return "NUMBER(10)"
	case "int64":
		return "NUMBER(19)"
	case "*int64":
		return "NUMBER(19)"
	case "float64":
		return "BINARY_DOUBLE"
	case "*float64":
		return "BINARY_DOUBLE"
	case "decimal.Decimal":
		return "NUMBER(18,4)"
	case "string":
		return "VARCHAR2(4000)"
	case "*string":
		return "VARCHAR2(4000)"
	case "bool":
		return "NUMBER(1)"
	case "*bool":
		return "NUMBER(1)"
	case "time.Time":
		return "TIMESTAMP WITH TIME ZONE"
	case "*time.Time":
		return "TIMESTAMP WITH TIME ZONE"
	case "[]byte":
		return "BLOB"
	default:
		return "CLOB"
	}
}

func (d OracleDialect) DriverName() string {
	return "godror"
}

func (d OracleDialect) ConnectionStringFormat() string {
	return "user/password@host:1521/service_name"
}

// GetDialect returns the SQLDialect for a backend type.
func GetDialect(backend BackendType) SQLDialect {
	switch backend {
	case BackendPostgres:
		return PostgresDialect{}
	case BackendMySQL:
		return MySQLDialect{}
	case BackendSQLite:
		return SQLiteDialect{}
	case BackendSQLServer:
		return SQLServerDialect{}
	case BackendOracle:
		return OracleDialect{}
	default:
		return nil
	}
}
