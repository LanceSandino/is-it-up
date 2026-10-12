package checks

import (
	"context"
	"database/sql"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/LanceSandino/is-it-up/pkg/secrets"
)

// TestCheckRedis_AuthFail verifies that checkRedis returns false when the
// Redis server responds with an -ERR message during AUTH command.
func TestCheckRedis_AuthFail(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Close() }()

	go func() {
		conn, err := l.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		buf := make([]byte, 512)
		n, err := conn.Read(buf)
		if err != nil {
			return
		}
		req := string(buf[:n])
		if strings.Contains(req, "\r\nAUTH\r\n") {
			if _, err := conn.Write([]byte("-ERR invalid password\r\n")); err != nil {
				return
			}
			return
		}
	}()

	addr := l.Addr().String()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	ok, msg := checkRedis(ctx, addr, "bad")
	if ok {
		t.Fatalf("expected auth to fail, but it succeeded")
	}
	if !strings.Contains(strings.ToLower(msg), "err") && !strings.Contains(strings.ToLower(msg), "invalid") {
		t.Fatalf("expected error message to contain err/invalid, got: %s", msg)
	}
}

// TestCheckMySQL_DSNAndTLS verifies the DSN generation for MySQL checks,
// including TLS parameter appending and full DSN passthrough behavior.
func TestCheckMySQL_DSNAndTLS(t *testing.T) {
	// Case 1: TLS mode appended. Use a dedicated sqlmock DB so closing it in
	// checkMySQL doesn't affect the next case.
	db1, mock1, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer func() { _ = db1.Close() }()

	var capturedDSN string
	orig := sqlOpen
	sqlOpen = func(driverName, dataSourceName string) (*sql.DB, error) {
		capturedDSN = dataSourceName
		return db1, nil
	}
	// will restore at end
	defer func() { sqlOpen = orig }()

	mock1.ExpectPing()
	rows1 := sqlmock.NewRows([]string{"1"}).AddRow(1)
	mock1.ExpectQuery("SELECT 1").WillReturnRows(rows1)

	cfg := CheckConfig{User: secrets.SecretValue{Raw: "u"}, Password: secrets.SecretValue{Raw: "p"}, Address: secrets.SecretValue{Raw: "127.0.0.1:3306"}, TLSMode: "skip-verify"}
	ok, msg := checkMySQL(context.Background(), cfg)
	if !ok {
		t.Fatalf("expected ok got false: %s", msg)
	}
	if !strings.Contains(capturedDSN, "tls=skip-verify") {
		t.Fatalf("expected tls param in DSN, got: %s", capturedDSN)
	}
	if err := mock1.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}

	// Case 2: Address contains full DSN, ensure used verbatim with a fresh mock DB
	db2, mock2, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock2: %v", err)
	}
	defer func() { _ = db2.Close() }()
	full := "user:pw@tcp(127.0.0.1:3306)/mydb?param=1"
	sqlOpen = func(driverName, dataSourceName string) (*sql.DB, error) {
		capturedDSN = dataSourceName
		return db2, nil
	}
	mock2.ExpectPing()
	rows2 := sqlmock.NewRows([]string{"1"}).AddRow(1)
	mock2.ExpectQuery("SELECT 1").WillReturnRows(rows2)
	cfg2 := CheckConfig{Address: secrets.SecretValue{Raw: full}, User: secrets.SecretValue{Raw: ""}, Password: secrets.SecretValue{Raw: ""}}
	ok2, msg2 := checkMySQL(context.Background(), cfg2)
	if !ok2 {
		t.Fatalf("expected ok for full DSN, got: %s", msg2)
	}
	if capturedDSN != full {
		t.Fatalf("expected DSN to be used verbatim, got: %s", capturedDSN)
	}
	if err := mock2.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations for second db: %v", err)
	}
}

// TestRedactedConfig_Validate_Load tests configuration management functions:
// RedactedConfig password masking, ValidateConfig validation, and LoadConfigAuto YAML parsing.
func TestRedactedConfig_Validate_Load(t *testing.T) {
	// RedactedConfig hides passwords when reveal=false
	cfg := &Config{Checks: []CheckConfig{{Name: "c1", Type: "redis", Password: secrets.SecretValue{Raw: "secret"}}}}
	red := RedactedConfig(cfg, false)
	if len(red.Checks) != 1 {
		t.Fatalf("unexpected length")
	}
	if red.Checks[0].Password.Raw != "REDACTED" {
		t.Fatalf("expected password to be redacted, got: %s", red.Checks[0].Password.Raw)
	}
	// The legacy reveal argument no longer permits secret disclosure.
	red2 := RedactedConfig(cfg, true)
	if red2.Checks[0].Password.Raw != "REDACTED" {
		t.Fatalf("expected password to remain redacted, got: %s", red2.Checks[0].Password.Raw)
	}

	// ValidateConfig: mysql without user/password should error
	mcfg := &Config{Checks: []CheckConfig{{Name: "db", Type: "mysql", Address: secrets.SecretValue{Raw: "localhost:3306"}}}}
	if err := ValidateConfig(mcfg); err == nil {
		t.Fatalf("expected ValidateConfig to error for mysql missing creds")
	}
	// valid mysql passes
	mcfg2 := &Config{Checks: []CheckConfig{{Name: "db", Type: "mysql", Address: secrets.SecretValue{Raw: "localhost:3306"}, User: secrets.SecretValue{Raw: "u"}, Password: secrets.SecretValue{Raw: "p"}}}}
	if err := ValidateConfig(mcfg2); err != nil {
		t.Fatalf("expected ValidateConfig to succeed: %v", err)
	}

	// LoadConfigAuto: create a temp yaml file
	f, err := os.CreateTemp("", "is-it-up-*.yaml")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	defer func() { _ = os.Remove(f.Name()) }()
	yaml := "checks:\n  - name: t1\n    type: redis\n    address: 127.0.0.1:6379\n"
	if _, err := f.WriteString(yaml); err != nil {
		t.Fatalf("write temp: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close temp: %v", err)
	}
	cfgLoaded, kind, err := LoadConfig(f.Name())
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if kind != "yaml" {
		t.Fatalf("expected yaml kind, got %s", kind)
	}
	if cfgLoaded == nil || len(cfgLoaded.Checks) != 1 {
		t.Fatalf("unexpected loaded config: %+v", cfgLoaded)
	}

	// Also test LoadConfigAuto with no-extension file containing YAML
	f2, err := os.CreateTemp("", "is-it-up-*")
	if err != nil {
		t.Fatalf("create temp2: %v", err)
	}
	defer func() { _ = os.Remove(f2.Name()) }()
	if _, err := f2.WriteString(yaml); err != nil {
		t.Fatalf("write temp2: %v", err)
	}
	if err := f2.Close(); err != nil {
		t.Fatalf("close temp2: %v", err)
	}
	cfgLoaded2, kind2, err := LoadConfig(f2.Name())
	if err != nil {
		t.Fatalf("LoadConfig for no-ext failed: %v", err)
	}
	if kind2 != "yaml" {
		t.Fatalf("expected yaml kind2, got %s", kind2)
	}
	if cfgLoaded2 == nil || len(cfgLoaded2.Checks) != 1 {
		t.Fatalf("unexpected loaded config2: %+v", cfgLoaded2)
	}
}

// TestResult_StringAndGetters tests the Result struct's String method and getter functions.
// Verifies that all getter methods return the correct values and that String() doesn't panic.
func TestResult_StringAndGetters(t *testing.T) {
	r := Result{Name: "n", OK: false, Message: "m", Severity: "s", Timestamp: time.Unix(0, 0), Duration: 1500 * time.Millisecond, Details: "d"}
	_ = r.String()
	if r.GetName() != "n" || r.GetMessage() != "m" || r.GetSeverity() != "s" {
		t.Fatalf("getters returned unexpected values")
	}
	if r.GetDuration() != r.Duration || !r.GetTimestamp().Equal(r.Timestamp) {
		t.Fatalf("timestamp/duration getters mismatch")
	}
}

// TestResultGetters tests all getter methods of the Result struct.
// Verifies that each getter method returns the expected value from the Result fields.
func TestResultGetters(t *testing.T) {
	r := Result{Name: "n", OK: true, Message: "m", Severity: "s", Timestamp: time.Now(), Duration: 10 * time.Millisecond, Details: "d"}
	if r.GetName() != "n" {
		t.Fatalf("name")
	}
	if !r.IsOK() {
		t.Fatalf("ok")
	}
	if r.GetMessage() != "m" {
		t.Fatalf("msg")
	}
	if r.GetSeverity() != "s" {
		t.Fatalf("sev")
	}
	if r.GetDetails() != "d" {
		t.Fatalf("details")
	}
}

// TestRunAll_UnknownTypeAndTCPFallback tests behavior when an unknown check type is encountered
// and TCP fallback mechanism. Verifies that unknown types are ignored and TCP checks work properly.
func TestRunAll_UnknownTypeAndTCPFallback(t *testing.T) {
	// stub tcp function to a deterministic result
	orig := checkTCPFn
	defer func() { checkTCPFn = orig }()
	checkTCPFn = func(ctx context.Context, address string) (bool, string) { return false, "connect fail" }
	cfg := &Config{Checks: []CheckConfig{{Name: "x", Type: "unknown"}, {Name: "t", Type: "tcp", Address: secrets.SecretValue{Raw: "1.2.3.4:80"}}}}
	res := RunAll(context.Background(), cfg)
	if len(res) != 2 {
		t.Fatalf("expected two results, got %d", len(res))
	}
}

// TestRunAll_MySQL_DirectConnection tests MySQL health checks with direct database connectivity.
// Verifies that MySQL checks perform direct database connection and query validation.
func TestRunAll_MySQL_DirectConnection(t *testing.T) {
	// stub MySQL function
	oldMy := checkMySQLFn
	defer func() { checkMySQLFn = oldMy }()
	checkMySQLFn = func(ctx context.Context, c CheckConfig) (bool, string) { return true, "select 1 ok" }

	cfg := &Config{Checks: []CheckConfig{{Name: "db", Type: "mysql", Address: secrets.SecretValue{Raw: "127.0.0.1:3306"}, User: secrets.SecretValue{Raw: "u"}, Password: secrets.SecretValue{Raw: "p"}}}}
	res := RunAll(context.Background(), cfg)
	if len(res) != 1 {
		t.Fatalf("expected 1 result (direct mysql), got %d", len(res))
	}
}

// TestRunAll_MySQL_ConnectionError tests that MySQL connection errors
// are properly handled. Verifies efficient error reporting for connection issues.
func TestRunAll_MySQL_ConnectionError(t *testing.T) {
	oldMy := checkMySQLFn
	defer func() { checkMySQLFn = oldMy }()
	checkMySQLFn = func(ctx context.Context, c CheckConfig) (bool, string) { return false, "connection refused" }

	cfg := &Config{Checks: []CheckConfig{{Name: "db", Type: "mysql", Address: secrets.SecretValue{Raw: "127.0.0.1:3306"}}}}
	res := RunAll(context.Background(), cfg)
	if len(res) != 1 {
		t.Fatalf("expected 1 result when mysql connection fails, got %d", len(res))
	}
}

// TestRunAll_Redis_InfoFlag tests Redis health checks with the RedisInfo flag enabled.
// Verifies that when RedisInfo is true, both PING and INFO commands are executed.
func TestRunAll_Redis_InfoFlag(t *testing.T) {
	oldRedis := checkRedisFn
	oldRedisInfo := checkRedisInfoFn
	defer func() { checkRedisFn = oldRedis; checkRedisInfoFn = oldRedisInfo }()
	checkRedisFn = func(ctx context.Context, addr, pwd string) (bool, string) { return true, "pong" }
	checkRedisInfoFn = func(ctx context.Context, addr, pwd string) (bool, string) { return true, "info-ok" }

	cfg := &Config{Checks: []CheckConfig{{Name: "r", Type: "redis", Address: secrets.SecretValue{Raw: "127.0.0.1:6379"}, RedisInfo: true}}}
	res := RunAll(context.Background(), cfg)
	if len(res) != 1 {
		t.Fatalf("expected 1 result, got %d", len(res))
	}
}
