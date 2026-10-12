package checks

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/LanceSandino/is-it-up/pkg/secrets"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

// fake client implementing mongoClientMinimal for unit tests
type fakeMongoClient struct {
	pingErr error
}

func (f *fakeMongoClient) Ping(ctx context.Context, rp *readpref.ReadPref) error {
	_ = rp
	return f.pingErr
}

func (f *fakeMongoClient) Disconnect(ctx context.Context) error { return nil }

func TestCheckDocumentDB_Success(t *testing.T) {
	orig := mongoConnectFn
	defer func() { mongoConnectFn = orig }()

	mongoConnectFn = func(ctx context.Context, uri string, opts ...interface{}) (mongoClientMinimal, error) {
		return &fakeMongoClient{pingErr: nil}, nil
	}

	cfg := CheckConfig{Name: "doc1"}
	ok, msg := checkDocumentDB(context.Background(), cfg)
	if !ok || msg != "documentdb ping ok" {
		t.Fatalf("unexpected result: ok=%v msg=%s", ok, msg)
	}
}

func TestCheckDocumentDB_ConnectError(t *testing.T) {
	orig := mongoConnectFn
	defer func() { mongoConnectFn = orig }()

	mongoConnectFn = func(ctx context.Context, uri string, opts ...interface{}) (mongoClientMinimal, error) {
		return nil, errors.New("connect failed")
	}

	cfg := CheckConfig{Name: "doc1"}
	ok, msg := checkDocumentDB(context.Background(), cfg)
	if ok || !strings.Contains(msg, "connect failed") {
		t.Fatalf("expected connect error, got: ok=%v msg=%s", ok, msg)
	}
}

func TestCheckDocumentDB_PingError(t *testing.T) {
	orig := mongoConnectFn
	defer func() { mongoConnectFn = orig }()

	mongoConnectFn = func(ctx context.Context, uri string, opts ...interface{}) (mongoClientMinimal, error) {
		return &fakeMongoClient{pingErr: errors.New("ping failed")}, nil
	}

	cfg := CheckConfig{Name: "doc1"}
	ok, msg := checkDocumentDB(context.Background(), cfg)
	if ok || !strings.Contains(msg, "ping failed") {
		t.Fatalf("expected ping error, got: ok=%v msg=%s", ok, msg)
	}
}

func TestCheckDocumentDB_EncodesAndRedactsCredentials(t *testing.T) {
	orig := mongoConnectFn
	defer func() { mongoConnectFn = orig }()
	var gotURI string
	mongoConnectFn = func(_ context.Context, uri string, _ ...interface{}) (mongoClientMinimal, error) {
		gotURI = uri
		return nil, errors.New("connection failed for " + uri)
	}
	cfg := CheckConfig{
		Name: "doc", Type: "documentdb", Address: secrets.SecretValue{Raw: "db.example:27017"},
		User: secrets.SecretValue{Raw: "name@example"}, Password: secrets.SecretValue{Raw: "p@ss:/word"},
	}
	ok, msg := checkDocumentDB(context.Background(), cfg)
	if ok {
		t.Fatal("expected connection failure")
	}
	if gotURI != "mongodb://name%40example:p%40ss%3A%2Fword@db.example:27017/admin" {
		t.Fatalf("unexpected encoded URI: %q", gotURI)
	}
	if strings.Contains(msg, "name@example") || strings.Contains(msg, "p@ss:/word") || strings.Contains(msg, "p%40ss") {
		t.Fatalf("credential leaked in error: %q", msg)
	}
}
