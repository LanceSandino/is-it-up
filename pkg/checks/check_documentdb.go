// This file implements MongoDB-compatible document database health checks.
// It depends on the MongoDB v2 driver through a replaceable connection seam.
package checks

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

// mongoClientMinimal abstracts the minimal methods we need from mongo.Client
type mongoClientMinimal interface {
	Ping(ctx context.Context, rp *readpref.ReadPref) error
	Disconnect(ctx context.Context) error
}

// Package-level hook so unit tests can replace the connect behavior
var mongoConnectFn = func(ctx context.Context, uri string, opts ...interface{}) (mongoClientMinimal, error) {
	clientOpts := options.Client().ApplyURI(uri)
	if len(opts) > 0 {
		if co, ok := opts[0].(*options.ClientOptions); ok && co != nil {
			// use the provided client options if given
			clientOpts = co
		}
	}
	client, err := mongo.Connect(clientOpts)
	if err != nil {
		return nil, err
	}
	return client, nil
}

// checkDocumentDB performs a ping against a Mongo-compatible endpoint.
func checkDocumentDB(ctx context.Context, cfg CheckConfig) (bool, string) {
	user := cfg.User.String()
	pass := cfg.Password.String()
	addr := cfg.Address.String()
	db := cfg.Database.String()

	// default database and auth DB
	if db == "" {
		db = "admin"
	}

	if addr == "" {
		addr = "127.0.0.1:27017"
	}

	// If Address looks like a full mongo URI, use it directly
	var uri string
	if strings.Contains(addr, "mongodb://") || strings.Contains(addr, "mongodb+srv://") || strings.Contains(addr, "=") {
		uri = addr
	} else {
		// Build a simple mongodb URI
		creds := ""
		if user != "" || pass != "" {
			creds = url.UserPassword(user, pass).String() + "@"
		}
		uri = fmt.Sprintf("mongodb://%s%s/%s", creds, addr, db)
	}

	// Map TLS semantics via canonical CheckConfig.TLSMode.
	// Supported values (informational only): "", "required", "skip-verify", "disable".
	// When a full URI is provided it may already include TLS/auth settings; we only
	// apply a TLS config when the user requests skip-verify or required via TLSMode.
	clientOpts := options.Client()
	tlsMode := strings.TrimSpace(cfg.TLSMode)
	switch tlsMode {
	case "skip-verify":
		// Explicit skip-verify requested by config; mark inline suppression to
		// acknowledge this is an accepted, opt-in insecure mode for diagnostics.
		clientOpts.SetTLSConfig(&tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12}) // #nosec G402 - explicit user opt-in
	case "required":
		// Require at least TLS1.2 by default
		clientOpts.SetTLSConfig(&tls.Config{MinVersion: tls.VersionTLS12})
	case "disable", "":
		// no TLS configuration — driver will follow URI or default behavior
	}

	// Apply the protocol default only when the caller did not configure a deadline.
	tctx, cancel := contextWithDefaultTimeout(ctx, 4*time.Second)
	defer cancel()

	client, err := mongoConnectFn(tctx, uri, clientOpts)
	if err != nil {
		checkErr := NewCheckError("documentdb", cfg.Name, "connect", errors.New(redactText(err.Error(), user, pass)))
		return false, checkErr.Error()
	}
	defer func() { _ = client.Disconnect(tctx) }()

	if err := client.Ping(tctx, readpref.Primary()); err != nil {
		checkErr := NewCheckError("documentdb", cfg.Name, "ping", errors.New(redactText(err.Error(), user, pass)))
		return false, checkErr.Error()
	}

	return true, "documentdb ping ok"
}
