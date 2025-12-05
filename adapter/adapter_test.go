package adapter

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestAdapter_Interface ensures all adapters implement the Adapter interface
func TestAdapter_Interface(t *testing.T) {
	var _ Adapter = (*SQLiteAdapter)(nil)
	var _ Adapter = (*PostgresAdapter)(nil)
	var _ Adapter = (*MySQLAdapter)(nil)
}

// TestConfig_Defaults tests default configuration
func TestConfig_Defaults(t *testing.T) {
	config := DefaultConfig()

	if config.Host != "localhost" {
		t.Errorf("Expected host 'localhost', got '%s'", config.Host)
	}
	if config.Port != 5432 {
		t.Errorf("Expected port 5432, got %d", config.Port)
	}
	if config.MaxOpenConns != 25 {
		t.Errorf("Expected MaxOpenConns 25, got %d", config.MaxOpenConns)
	}
	if config.ConnMaxLifetime != 5*time.Minute {
		t.Errorf("Expected ConnMaxLifetime 5m, got %v", config.ConnMaxLifetime)
	}
}

// TestSQLiteAdapter_Stub tests that SQLite stub returns appropriate errors
func TestSQLiteAdapter_Stub(t *testing.T) {
	ctx := context.Background()
	adapter := NewSQLiteMemory()

	err := adapter.Open(ctx)
	// In stub mode, this should return ErrSQLiteNotAvailable
	// In real mode, it should succeed
	if err == ErrSQLiteNotAvailable {
		t.Log("SQLite adapter is in stub mode (no CGO)")
	} else if err != nil {
		t.Fatalf("Failed to open SQLite: %v", err)
	} else {
		t.Log("SQLite adapter is available")
		defer adapter.Close()

		// Run actual tests
		if err := adapter.Ping(ctx); err != nil {
			t.Fatalf("Ping failed: %v", err)
		}

		if adapter.DialectName() != "sqlite" {
			t.Errorf("Expected dialect 'sqlite', got '%s'", adapter.DialectName())
		}
	}
}

// TestPostgresAdapter_Stub tests that PostgreSQL stub returns appropriate errors
func TestPostgresAdapter_Stub(t *testing.T) {
	if os.Getenv("TEST_POSTGRES") == "" {
		t.Skip("Skipping PostgreSQL test. Set TEST_POSTGRES=1 to run.")
	}

	ctx := context.Background()
	config := Config{
		Host:     "localhost",
		Port:     5432,
		Database: "tgpiler_test",
		Username: "tgpiler",
		Password: "tgpiler_test",
		SSLMode:  "disable",
	}

	adapter := NewPostgresAdapter(config)

	err := adapter.Open(ctx)
	if err == ErrPostgresNotAvailable {
		t.Log("PostgreSQL adapter is in stub mode")
	} else if err != nil {
		t.Fatalf("Failed to open PostgreSQL: %v", err)
	} else {
		t.Log("PostgreSQL adapter is available")
		defer adapter.Close()

		if err := adapter.Ping(ctx); err != nil {
			t.Fatalf("Ping failed: %v", err)
		}

		if adapter.DialectName() != "postgres" {
			t.Errorf("Expected dialect 'postgres', got '%s'", adapter.DialectName())
		}

		if err := adapter.HealthCheck(ctx); err != nil {
			t.Errorf("Health check failed: %v", err)
		}

		exists, err := adapter.TableExists(ctx, "users")
		if err != nil {
			t.Fatalf("TableExists failed: %v", err)
		}
		if !exists {
			t.Error("users table should exist")
		}

		columns, err := adapter.GetTableColumns(ctx, "users")
		if err != nil {
			t.Fatalf("GetTableColumns failed: %v", err)
		}
		t.Logf("users table has %d columns", len(columns))
	}
}

// TestMySQLAdapter_Stub tests that MySQL stub returns appropriate errors
func TestMySQLAdapter_Stub(t *testing.T) {
	if os.Getenv("TEST_MYSQL") == "" {
		t.Skip("Skipping MySQL test. Set TEST_MYSQL=1 to run.")
	}

	ctx := context.Background()
	config := Config{
		Host:     "localhost",
		Port:     3306,
		Database: "tgpiler_test",
		Username: "tgpiler",
		Password: "tgpiler_test",
	}

	adapter := NewMySQLAdapter(config)

	err := adapter.Open(ctx)
	if err == ErrMySQLNotAvailable {
		t.Log("MySQL adapter is in stub mode")
	} else if err != nil {
		t.Fatalf("Failed to open MySQL: %v", err)
	} else {
		t.Log("MySQL adapter is available")
		defer adapter.Close()

		if err := adapter.Ping(ctx); err != nil {
			t.Fatalf("Ping failed: %v", err)
		}

		if adapter.DialectName() != "mysql" {
			t.Errorf("Expected dialect 'mysql', got '%s'", adapter.DialectName())
		}

		if err := adapter.HealthCheck(ctx); err != nil {
			t.Errorf("Health check failed: %v", err)
		}

		version, err := adapter.GetVersion(ctx)
		if err != nil {
			t.Fatalf("GetVersion failed: %v", err)
		}
		t.Logf("MySQL version: %s", version)

		exists, err := adapter.TableExists(ctx, "users")
		if err != nil {
			t.Fatalf("TableExists failed: %v", err)
		}
		if !exists {
			t.Error("users table should exist")
		}

		columns, err := adapter.GetTableColumns(ctx, "users")
		if err != nil {
			t.Fatalf("GetTableColumns failed: %v", err)
		}
		t.Logf("users table has %d columns", len(columns))
	}
}
