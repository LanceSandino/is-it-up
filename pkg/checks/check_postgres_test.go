package checks

import (
	"context"
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/LanceSandino/is-it-up/v2/pkg/secrets"
)

// TestCheckPostgres_SelectOK tests successful PostgreSQL connection and SELECT 1 query execution.
// Uses sqlmock to simulate database responses without requiring an actual PostgreSQL server.
func TestCheckPostgres_SelectOK(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer func() { _ = db.Close() }()

	// replace sqlOpenPostgres to return our mock DB
	orig := sqlOpenPostgres
	sqlOpenPostgres = func(driverName, dataSourceName string) (*sql.DB, error) {
		return db, nil
	}
	defer func() { sqlOpenPostgres = orig }()

	// expect Ping then QueryRow for SELECT 1
	mock.ExpectPing()
	rows := sqlmock.NewRows([]string{"1"}).AddRow(1)
	mock.ExpectQuery("SELECT 1").WillReturnRows(rows)

	cfg := CheckConfig{
		User:     secrets.SecretValue{Raw: "u"},
		Password: secrets.SecretValue{Raw: "p"},
		Address:  secrets.SecretValue{Raw: "127.0.0.1:5432"},
		Database: secrets.SecretValue{Raw: "testdb"},
	}
	ok, msg := checkPostgres(context.Background(), cfg)
	if !ok {
		t.Fatalf("expected ok got false: %s", msg)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// TestCheckPostgres_DSNFromAddress tests DSN construction when Address contains full DSN
func TestCheckPostgres_DSNFromAddress(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer func() { _ = db.Close() }()

	orig := sqlOpenPostgres
	sqlOpenPostgres = func(driverName, dataSourceName string) (*sql.DB, error) {
		// Verify the DSN passed is exactly what we provided
		expectedDSN := "postgres://user:pass@host:5432/db?connect_timeout=4&sslmode=verify-full"
		if dataSourceName != expectedDSN {
			t.Errorf("expected DSN %q, got %q", expectedDSN, dataSourceName)
		}
		return db, nil
	}
	defer func() { sqlOpenPostgres = orig }()

	mock.ExpectPing()
	rows := sqlmock.NewRows([]string{"1"}).AddRow(1)
	mock.ExpectQuery("SELECT 1").WillReturnRows(rows)

	cfg := CheckConfig{
		Name:    "test-check",
		Address: secrets.SecretValue{Raw: "postgres://user:pass@host:5432/db?sslmode=require"},
	}
	ok, msg := checkPostgres(context.Background(), cfg)
	if !ok {
		t.Fatalf("expected ok got false: %s", msg)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// TestCheckPostgres_HostPortConstruction tests DSN construction from Host/Port fields
func TestCheckPostgres_HostPortConstruction(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer func() { _ = db.Close() }()

	orig := sqlOpenPostgres
	sqlOpenPostgres = func(driverName, dataSourceName string) (*sql.DB, error) {
		// Verify DSN construction from Host/Port
		expectedDSN := "postgres://testuser:testpass@testhost:5433/testdb?connect_timeout=4&sslmode=verify-full"
		if dataSourceName != expectedDSN {
			t.Errorf("expected DSN %q, got %q", expectedDSN, dataSourceName)
		}
		return db, nil
	}
	defer func() { sqlOpenPostgres = orig }()

	mock.ExpectPing()
	rows := sqlmock.NewRows([]string{"1"}).AddRow(1)
	mock.ExpectQuery("SELECT 1").WillReturnRows(rows)

	cfg := CheckConfig{
		Name:     "test-check",
		Host:     secrets.SecretValue{Raw: "testhost"},
		Port:     secrets.SecretValue{Raw: "5433"},
		User:     secrets.SecretValue{Raw: "testuser"},
		Password: secrets.SecretValue{Raw: "testpass"},
		Database: secrets.SecretValue{Raw: "testdb"},
	}
	ok, msg := checkPostgres(context.Background(), cfg)
	if !ok {
		t.Fatalf("expected ok got false: %s", msg)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// TestCheckPostgres_DefaultValues tests default value handling
func TestCheckPostgres_DefaultValues(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer func() { _ = db.Close() }()

	orig := sqlOpenPostgres
	sqlOpenPostgres = func(driverName, dataSourceName string) (*sql.DB, error) {
		// Verify defaults: user=postgres, db=postgres, port=5432, sslmode=require
		expectedDSN := "postgres://postgres:@127.0.0.1:5432/postgres?connect_timeout=4&sslmode=verify-full"
		if dataSourceName != expectedDSN {
			t.Errorf("expected DSN %q, got %q", expectedDSN, dataSourceName)
		}
		return db, nil
	}
	defer func() { sqlOpenPostgres = orig }()

	mock.ExpectPing()
	rows := sqlmock.NewRows([]string{"1"}).AddRow(1)
	mock.ExpectQuery("SELECT 1").WillReturnRows(rows)

	cfg := CheckConfig{
		Name: "test-check",
		// All fields empty to test defaults
	}
	ok, msg := checkPostgres(context.Background(), cfg)
	if !ok {
		t.Fatalf("expected ok got false: %s", msg)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// TestCheckPostgres_SSLModes tests different SSL mode configurations
func TestCheckPostgres_SSLModes(t *testing.T) {
	tests := []struct {
		name     string
		sslMode  string
		expected string
	}{
		{"default verify-full", "", "postgres://user:pass@host:5432/db?connect_timeout=4&sslmode=verify-full"},
		{"explicit require", "require", "postgres://user:pass@host:5432/db?connect_timeout=4&sslmode=require"},
		{"disable", "disable", "postgres://user:pass@host:5432/db?connect_timeout=4&sslmode=disable"},
		{"prefer", "prefer", "postgres://user:pass@host:5432/db?connect_timeout=4&sslmode=prefer"},
		{"verify-ca", "verify-ca", "postgres://user:pass@host:5432/db?connect_timeout=4&sslmode=verify-ca"},
		{"verify-full", "verify-full", "postgres://user:pass@host:5432/db?connect_timeout=4&sslmode=verify-full"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer func() { _ = db.Close() }()

			orig := sqlOpenPostgres
			sqlOpenPostgres = func(driverName, dataSourceName string) (*sql.DB, error) {
				if dataSourceName != tt.expected {
					t.Errorf("expected DSN %q, got %q", tt.expected, dataSourceName)
				}
				return db, nil
			}
			defer func() { sqlOpenPostgres = orig }()

			mock.ExpectPing()
			rows := sqlmock.NewRows([]string{"1"}).AddRow(1)
			mock.ExpectQuery("SELECT 1").WillReturnRows(rows)

			cfg := CheckConfig{
				Name:     "test-check",
				User:     secrets.SecretValue{Raw: "user"},
				Password: secrets.SecretValue{Raw: "pass"},
				Address:  secrets.SecretValue{Raw: "host:5432"},
				Database: secrets.SecretValue{Raw: "db"},
				TLSMode:  tt.sslMode,
			}
			ok, msg := checkPostgres(context.Background(), cfg)
			if !ok {
				t.Fatalf("expected ok got false: %s", msg)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatalf("unmet expectations: %v", err)
			}
		})
	}
}

// TestCheckPostgres_ConnectionError tests connection failure handling
func TestCheckPostgres_ConnectionError(t *testing.T) {
	orig := sqlOpenPostgres
	sqlOpenPostgres = func(driverName, dataSourceName string) (*sql.DB, error) {
		return nil, sql.ErrConnDone
	}
	defer func() { sqlOpenPostgres = orig }()

	cfg := CheckConfig{
		Name:    "test-check",
		Address: secrets.SecretValue{Raw: "127.0.0.1:5432"},
	}
	ok, msg := checkPostgres(context.Background(), cfg)
	if ok {
		t.Fatal("expected failure on connection error")
	}
	expectedMsg := "check 'test-check' connect failed: sql: connection is already closed"
	if msg != expectedMsg {
		t.Fatalf("expected %q, got %q", expectedMsg, msg)
	}
}

// TestCheckPostgres_PingError tests ping failure handling
func TestCheckPostgres_PingError(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer func() { _ = db.Close() }()

	orig := sqlOpenPostgres
	sqlOpenPostgres = func(driverName, dataSourceName string) (*sql.DB, error) {
		return db, nil
	}
	defer func() { sqlOpenPostgres = orig }()

	mock.ExpectPing().WillReturnError(sql.ErrNoRows)

	cfg := CheckConfig{
		Name:    "test-check",
		Address: secrets.SecretValue{Raw: "127.0.0.1:5432"},
	}
	ok, msg := checkPostgres(context.Background(), cfg)
	if ok {
		t.Fatal("expected failure on ping error")
	}
	expectedMsg := "check 'test-check' ping failed: sql: no rows in result set"
	if msg != expectedMsg {
		t.Fatalf("expected %q, got %q", expectedMsg, msg)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// TestCheckPostgres_QueryError tests query failure handling
func TestCheckPostgres_QueryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer func() { _ = db.Close() }()

	orig := sqlOpenPostgres
	sqlOpenPostgres = func(driverName, dataSourceName string) (*sql.DB, error) {
		return db, nil
	}
	defer func() { sqlOpenPostgres = orig }()

	mock.ExpectPing()
	mock.ExpectQuery("SELECT 1").WillReturnError(sql.ErrNoRows)

	cfg := CheckConfig{
		Name:    "test-check",
		Address: secrets.SecretValue{Raw: "127.0.0.1:5432"},
	}
	ok, msg := checkPostgres(context.Background(), cfg)
	if ok {
		t.Fatal("expected failure on query error")
	}
	expectedMsg := "check 'test-check' select failed: sql: no rows in result set"
	if msg != expectedMsg {
		t.Fatalf("expected %q, got %q", expectedMsg, msg)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// TestCheckPostgres_UnexpectedResult tests handling of unexpected SELECT 1 result
func TestCheckPostgres_UnexpectedResult(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer func() { _ = db.Close() }()

	orig := sqlOpenPostgres
	sqlOpenPostgres = func(driverName, dataSourceName string) (*sql.DB, error) {
		return db, nil
	}
	defer func() { sqlOpenPostgres = orig }()

	mock.ExpectPing()
	// Return unexpected result (not 1)
	rows := sqlmock.NewRows([]string{"1"}).AddRow(42)
	mock.ExpectQuery("SELECT 1").WillReturnRows(rows)

	cfg := CheckConfig{
		Name:    "test-check",
		Address: secrets.SecretValue{Raw: "127.0.0.1:5432"},
	}
	ok, msg := checkPostgres(context.Background(), cfg)
	if ok {
		t.Fatal("expected failure on unexpected result")
	}
	expectedMsg := "unexpected select result"
	if msg != expectedMsg {
		t.Fatalf("expected %q, got %q", expectedMsg, msg)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
