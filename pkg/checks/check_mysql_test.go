package checks

import (
	"context"
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/LanceSandino/is-it-up/pkg/secrets"
)

// TestCheckMySQL_SelectOK tests successful MySQL connection and SELECT 1 query execution.
// Uses sqlmock to simulate database responses without requiring an actual MySQL server.
func TestCheckMySQL_SelectOK(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer func() { _ = db.Close() }()

	// replace sqlOpen to return our mock DB
	orig := sqlOpen
	sqlOpen = func(driverName, dataSourceName string) (*sql.DB, error) {
		return db, nil
	}
	defer func() { sqlOpen = orig }()

	// expect Ping then QueryRow for SELECT 1
	mock.ExpectPing()
	rows := sqlmock.NewRows([]string{"1"}).AddRow(1)
	mock.ExpectQuery("SELECT 1").WillReturnRows(rows)

	cfg := CheckConfig{User: secrets.SecretValue{Raw: "u"}, Password: secrets.SecretValue{Raw: "p"}, Address: secrets.SecretValue{Raw: "127.0.0.1:3306"}}
	ok, msg := checkMySQL(context.Background(), cfg)
	if !ok {
		t.Fatalf("expected ok got false: %s", msg)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
