// This file anchors remediations for the v1.0.0 security audit findings.
// It uses only bounded loopback fixtures and injectable checker seams.
package checks

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/LanceSandino/is-it-up/v2/pkg/secrets"
	mysqlDriver "github.com/go-sql-driver/mysql"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func TestRunAllRedactsSensitiveDiagnostics(t *testing.T) {
	const secret = "audit-diagnostic-secret"
	original := checkMySQLFn
	checkMySQLFn = func(context.Context, CheckConfig) (bool, string) {
		return false, "peer reflected " + secret
	}
	t.Cleanup(func() { checkMySQLFn = original })

	cfg := &Config{Checks: []CheckConfig{{
		Name:     "database",
		Type:     "mysql",
		Address:  secrets.SecretValue{Raw: "db.example:3306"},
		User:     secrets.SecretValue{Raw: "audit-user"},
		Password: secrets.SecretValue{Raw: secret, Sensitive: true},
	}}}

	results := RunAll(context.Background(), cfg)
	if len(results) != 1 {
		t.Fatalf("expected one result, got %d", len(results))
	}
	combined := results[0].Message + " " + results[0].Details
	if strings.Contains(combined, secret) {
		t.Fatalf("secret reached runtime diagnostics: %q", combined)
	}
	if !strings.Contains(combined, "REDACTED") {
		t.Fatalf("expected explicit redaction marker, got %q", combined)
	}
}

type fixedSecretFetcher string

func (f fixedSecretFetcher) GetSecret(context.Context, *secrets.SecretRef) (string, error) {
	return string(f), nil
}

func TestResolveConfigSecretsPreservesSensitiveProvenance(t *testing.T) {
	cfg := &Config{Checks: []CheckConfig{{
		Name: "web", Type: "http",
		Address: secrets.SecretValue{Ref: &secrets.SecretRef{SecretID: "audit"}},
	}}}
	count, err := ResolveConfigSecrets(context.Background(), cfg, fixedSecretFetcher("https://secret.example/health"))
	if err != nil || count != 1 {
		t.Fatalf("resolution failed: count=%d err=%v", count, err)
	}
	if !cfg.Checks[0].Address.Sensitive || cfg.Checks[0].Address.Ref != nil {
		t.Fatalf("secret provenance was not preserved: %#v", cfg.Checks[0].Address)
	}
	if got := RedactedConfig(cfg, false).Checks[0].Address.String(); got != "REDACTED" {
		t.Fatalf("resolved address was not redacted: %q", got)
	}
}

func TestRedactedConfigMasksAllResolvedSecretFields(t *testing.T) {
	secretValue := func(name string) secrets.SecretValue {
		return secrets.SecretValue{Raw: "secret-" + name, Sensitive: true}
	}
	cfg := &Config{Checks: []CheckConfig{{
		Name:           "all-fields",
		Type:           "http",
		Address:        secretValue("address"),
		Protocol:       secretValue("protocol"),
		Host:           secretValue("host"),
		Port:           secretValue("port"),
		Path:           secretValue("path"),
		Database:       secretValue("database"),
		User:           secretValue("user"),
		Password:       secretValue("password"),
		Vhost:          secretValue("vhost"),
		SMTPEncryption: secretValue("smtp-encryption"),
	}}}

	redacted := RedactedConfig(cfg, false).Checks[0]
	values := []secrets.SecretValue{
		redacted.Address, redacted.Protocol, redacted.Host, redacted.Port,
		redacted.Path, redacted.Database, redacted.User, redacted.Password,
		redacted.Vhost, redacted.SMTPEncryption,
	}
	for i, value := range values {
		if got := value.String(); got != "REDACTED" {
			t.Fatalf("field %d was not redacted: %q", i, got)
		}
	}
}

func TestValidateResolvedSecretsDoesNotExposeValue(t *testing.T) {
	const secret = "invalid-secret-encryption-mode"
	cfg := &Config{Checks: []CheckConfig{{
		Name:           "mail",
		Type:           "smtp",
		Address:        secrets.SecretValue{Raw: "mail.example:587"},
		SMTPEncryption: secrets.SecretValue{Raw: secret, Sensitive: true},
	}}}

	err := ValidateResolvedSecrets(cfg)
	if err == nil {
		t.Fatal("expected validation failure")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("secret-derived validation value leaked: %v", err)
	}
}

func TestHTTPDoesNotFollowRedirects(t *testing.T) {
	var destinationHits int
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		destinationHits++
		w.WriteHeader(http.StatusOK)
	}))
	defer destination.Close()

	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Redirect(w, &http.Request{}, destination.URL, http.StatusFound)
	}))
	defer source.Close()

	ok, msg := checkHTTP(context.Background(), CheckConfig{Address: secrets.SecretValue{Raw: source.URL}})
	if !ok || msg != "status 302" {
		t.Fatalf("expected redirect response without traversal, ok=%v msg=%q", ok, msg)
	}
	if destinationHits != 0 {
		t.Fatalf("redirect crossed authority boundary %d time(s)", destinationHits)
	}
}

func TestHTTPRequiredTLSRejectsPlaintextBeforeRequest(t *testing.T) {
	var hits int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ok, msg := checkHTTP(context.Background(), CheckConfig{
		Address: secrets.SecretValue{Raw: server.URL},
		TLSMode: "required",
	})
	if ok || !strings.Contains(msg, "requires https") {
		t.Fatalf("expected required-TLS rejection, ok=%v msg=%q", ok, msg)
	}
	if hits != 0 {
		t.Fatalf("plaintext server was contacted %d time(s)", hits)
	}
}

func TestElasticsearchRejectsOversizedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"green","padding":"`))
		_, _ = w.Write([]byte(strings.Repeat("x", maxHTTPResponseBytes+1)))
		_, _ = w.Write([]byte(`"}`))
	}))
	defer server.Close()

	ok, msg := checkElasticsearch(context.Background(), server.URL, "_cluster/health")
	if ok || !strings.Contains(msg, "response exceeds") {
		t.Fatalf("expected bounded response rejection, ok=%v msg=%q", ok, msg)
	}
}

func TestSMTPStartTLSFailsWhenCapabilityMissing(t *testing.T) {
	addr, closeServer := startSMTPFixture(t, func(conn net.Conn) {
		defer func() { _ = conn.Close() }()
		_, _ = fmt.Fprint(conn, "220 fixture ESMTP\r\n")
		buf := make([]byte, 1024)
		_, _ = conn.Read(buf)
		_, _ = fmt.Fprint(conn, "250-fixture\r\n250 AUTH PLAIN\r\n")
	})
	defer closeServer()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	ok, msg := checkSMTP(ctx, CheckConfig{
		Address:        secrets.SecretValue{Raw: addr},
		SMTPEncryption: secrets.SecretValue{Raw: "starttls"},
	})
	if ok || !strings.Contains(strings.ToLower(msg), "starttls") {
		t.Fatalf("expected missing STARTTLS failure, ok=%v msg=%q", ok, msg)
	}
}

func TestSMTPRejectsOversizedReply(t *testing.T) {
	addr, closeServer := startSMTPFixture(t, func(conn net.Conn) {
		defer func() { _ = conn.Close() }()
		_, _ = fmt.Fprint(conn, strings.Repeat("2", int(maxSMTPResponseBytes+1)))
	})
	defer closeServer()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	ok, msg := checkSMTP(ctx, CheckConfig{
		Address:        secrets.SecretValue{Raw: addr},
		SMTPEncryption: secrets.SecretValue{Raw: "plain"},
	})
	if ok || !strings.Contains(strings.ToLower(msg), "response limit") {
		t.Fatalf("expected bounded SMTP response failure, ok=%v msg=%q", ok, msg)
	}
}

func startSMTPFixture(t *testing.T, handler func(net.Conn)) (string, func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := listener.Accept()
		if err == nil {
			handler(conn)
		}
	}()
	return listener.Addr().String(), func() {
		_ = listener.Close()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("SMTP fixture did not stop")
		}
	}
}

func TestDatabaseDSNsUseVerifiedTLSAndEscapeComponents(t *testing.T) {
	t.Run("mysql", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = db.Close() }()
		mock.ExpectPing()
		mock.ExpectQuery("SELECT 1").WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))

		var gotDSN string
		original := sqlOpen
		sqlOpen = func(_ string, dsn string) (*sql.DB, error) {
			gotDSN = dsn
			return db, nil
		}
		t.Cleanup(func() { sqlOpen = original })

		cfg := CheckConfig{
			Name: "mysql", Type: "mysql",
			Address:  secrets.SecretValue{Raw: "db.example:3306"},
			User:     secrets.SecretValue{Raw: "user"},
			Password: secrets.SecretValue{Raw: "p@ss?allowAllFiles=true"},
			Database: secrets.SecretValue{Raw: "app/name"},
		}
		if ok, msg := checkMySQL(context.Background(), cfg); !ok {
			t.Fatalf("check failed: %s", msg)
		}
		parsed, err := mysqlDriver.ParseDSN(gotDSN)
		if err != nil {
			t.Fatalf("parse generated DSN: %v (%q)", err, gotDSN)
		}
		if parsed.User != cfg.User.String() || parsed.Passwd != cfg.Password.String() || parsed.DBName != cfg.Database.String() {
			t.Fatalf("components were not preserved: %#v", parsed)
		}
		if parsed.TLSConfig != "true" || parsed.AllowAllFiles {
			t.Fatalf("expected verified TLS and inert password components: %#v", parsed)
		}
	})

	t.Run("postgres", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = db.Close() }()
		mock.ExpectPing()
		mock.ExpectQuery("SELECT 1").WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))

		var gotDSN string
		original := sqlOpenPostgres
		sqlOpenPostgres = func(_ string, dsn string) (*sql.DB, error) {
			gotDSN = dsn
			return db, nil
		}
		t.Cleanup(func() { sqlOpenPostgres = original })

		cfg := CheckConfig{
			Name: "postgres", Type: "postgres", TimeoutMS: 2500,
			Address:  secrets.SecretValue{Raw: "db.example:5432"},
			User:     secrets.SecretValue{Raw: "user:name"},
			Password: secrets.SecretValue{Raw: "p@ss?sslmode=disable"},
			Database: secrets.SecretValue{Raw: "app/name"},
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2500*time.Millisecond)
		defer cancel()
		if ok, msg := checkPostgres(ctx, cfg); !ok {
			t.Fatalf("check failed: %s", msg)
		}
		parsed, err := url.Parse(gotDSN)
		if err != nil {
			t.Fatalf("parse generated DSN: %v (%q)", err, gotDSN)
		}
		password, _ := parsed.User.Password()
		if parsed.User.Username() != cfg.User.String() || password != cfg.Password.String() || strings.TrimPrefix(parsed.Path, "/") != cfg.Database.String() {
			t.Fatalf("components were not preserved: %s", gotDSN)
		}
		if parsed.Query().Get("sslmode") != "verify-full" || parsed.Query().Get("connect_timeout") != "3" {
			t.Fatalf("expected verified TLS and rounded context timeout: %s", gotDSN)
		}
	})
}

func TestRunAllContainsCheckerPanic(t *testing.T) {
	originalMySQL := checkMySQLFn
	originalTCP := checkTCPFn
	checkMySQLFn = func(context.Context, CheckConfig) (bool, string) { panic("malformed handshake") }
	checkTCPFn = func(context.Context, string) (bool, string) { return true, "tcp ok" }
	t.Cleanup(func() {
		checkMySQLFn = originalMySQL
		checkTCPFn = originalTCP
	})

	results := RunAll(context.Background(), &Config{Checks: []CheckConfig{
		{Name: "database", Type: "mysql", Address: secrets.SecretValue{Raw: "db:3306"}},
		{Name: "later", Type: "tcp", Address: secrets.SecretValue{Raw: "service:443"}},
	}})
	if len(results) != 2 || results[0].OK || !strings.Contains(results[0].Message, "panic") || !results[1].OK {
		t.Fatalf("panic was not contained per check: %#v", results)
	}
}

func TestCredentialChecksDefaultToTLS(t *testing.T) {
	originalRedis := checkRedisFn
	originalRabbit := checkRabbitAMQPFn
	var redisAddress, rabbitAddress string
	checkRedisFn = func(_ context.Context, address, _ string) (bool, string) {
		redisAddress = address
		return true, "pong"
	}
	checkRabbitAMQPFn = func(_ context.Context, address, _, _, _ string) (bool, string) {
		rabbitAddress = address
		return true, "amqp ok"
	}
	t.Cleanup(func() {
		checkRedisFn = originalRedis
		checkRabbitAMQPFn = originalRabbit
	})

	RunAll(context.Background(), &Config{Checks: []CheckConfig{
		{Name: "redis", Type: "redis", Address: secrets.SecretValue{Raw: "redis.example:6379"}, Password: secrets.SecretValue{Raw: "secret"}},
		{Name: "rabbit", Type: "rabbitmq", Address: secrets.SecretValue{Raw: "rabbit.example:5671"}, User: secrets.SecretValue{Raw: "user"}, Password: secrets.SecretValue{Raw: "secret"}},
	}})
	if redisAddress != "rediss://redis.example:6379" {
		t.Fatalf("Redis did not default to verified TLS: %q", redisAddress)
	}
	if rabbitAddress != "amqps://rabbit.example:5671" {
		t.Fatalf("RabbitMQ did not default to verified TLS: %q", rabbitAddress)
	}
}

func TestCredentialChecksDoNotAcceptImplicitSkipVerify(t *testing.T) {
	cfg := CheckConfig{
		TLSMode:  "required",
		Address:  secrets.SecretValue{Raw: "rediss+insecure://redis.example:6379"},
		Password: secrets.SecretValue{Raw: "secret"},
	}
	if got := credentialTransportAddress(cfg, "rediss"); got != "rediss://redis.example:6379" {
		t.Fatalf("required TLS retained an insecure scheme: %q", got)
	}
}

func TestRabbitMQTLSConfigVerified(t *testing.T) {
	original := amqpDialFn
	defer func() { amqpDialFn = original }()

	var gotURI string
	var gotConfig amqp.Config
	amqpDialFn = func(uri string, config amqp.Config) (amqpConn, error) {
		gotURI = uri
		gotConfig = config
		return securityAMQPConn{}, nil
	}

	ok, msg := checkRabbitAMQP(context.Background(), "amqps://rabbit.example:5671", "user", "password", "/")
	if !ok {
		t.Fatalf("TLS AMQP check failed: %s", msg)
	}
	if !strings.HasPrefix(gotURI, "amqps://") {
		t.Fatalf("expected amqps URI, got %q", gotURI)
	}
	if gotConfig.TLSClientConfig == nil || gotConfig.TLSClientConfig.ServerName != "rabbit.example" || gotConfig.TLSClientConfig.InsecureSkipVerify {
		t.Fatalf("expected verified RabbitMQ TLS config: %#v", gotConfig.TLSClientConfig)
	}
	if gotConfig.FrameSize <= 0 || gotConfig.FrameSize > 128<<10 {
		t.Fatalf("expected bounded AMQP frame size, got %d", gotConfig.FrameSize)
	}
}

func TestRedisTLSVerifiesServerBeforeAuthentication(t *testing.T) {
	var applicationHits int
	server := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		applicationHits++
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	address := "rediss://" + strings.TrimPrefix(server.URL, "https://")
	ok, msg := checkRedis(ctx, address, "dummy-password")
	if ok || (!strings.Contains(strings.ToLower(msg), "certificate") && !strings.Contains(strings.ToLower(msg), "unknown authority")) {
		t.Fatalf("expected certificate verification failure before AUTH, ok=%v msg=%q", ok, msg)
	}
	if applicationHits != 0 {
		t.Fatalf("untrusted Redis TLS peer received application data %d time(s)", applicationHits)
	}
}

type securityAMQPConn struct{}

func (securityAMQPConn) Channel() (amqpChannel, error) { return securityAMQPChannel{}, nil }
func (securityAMQPConn) Close() error                  { return nil }

type securityAMQPChannel struct{}

func (securityAMQPChannel) Close() error { return nil }

func TestPlaintextCredentialsRequireExplicitDisable(t *testing.T) {
	for _, checkType := range []string{"redis", "rabbitmq"} {
		cfg := &Config{Checks: []CheckConfig{{
			Name: checkType, Type: checkType, TLSMode: "disable",
			Address: secrets.SecretValue{Raw: "127.0.0.1:1"},
			User:    secrets.SecretValue{Raw: "user"}, Password: secrets.SecretValue{Raw: "password"},
		}}}
		if err := ValidateConfig(cfg); err != nil {
			t.Fatalf("%s explicit plaintext compatibility mode rejected: %v", checkType, err)
		}
	}
}

func TestDocumentDBPreservesURIOptionsWithTLSOverride(t *testing.T) {
	original := mongoConnectFn
	defer func() { mongoConnectFn = original }()

	mongoConnectFn = func(_ context.Context, _ string, opts ...interface{}) (mongoClientMinimal, error) {
		if len(opts) != 1 {
			t.Fatalf("expected one client option, got %d", len(opts))
		}
		clientOpts, ok := opts[0].(*options.ClientOptions)
		if !ok {
			t.Fatalf("unexpected options type %T", opts[0])
		}
		if strings.Join(clientOpts.Hosts, ",") != "db-a:27017,db-b:27018" {
			t.Fatalf("URI hosts were replaced: %#v", clientOpts.Hosts)
		}
		if clientOpts.ReplicaSet == nil || *clientOpts.ReplicaSet != "rs0" {
			t.Fatalf("replica set was replaced: %#v", clientOpts.ReplicaSet)
		}
		if clientOpts.TLSConfig == nil || clientOpts.TLSConfig.InsecureSkipVerify {
			t.Fatalf("verified TLS override missing: %#v", clientOpts.TLSConfig)
		}
		return &fakeMongoClient{}, nil
	}

	cfg := CheckConfig{
		Name: "documentdb", Type: "documentdb", TLSMode: "required",
		Address: secrets.SecretValue{Raw: "mongodb://db-a:27017,db-b:27018/admin?replicaSet=rs0"},
	}
	if ok, msg := checkDocumentDB(context.Background(), cfg); !ok {
		t.Fatalf("check failed: %s", msg)
	}
}
