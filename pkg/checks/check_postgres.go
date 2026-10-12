// This file implements PostgreSQL connection and query health checks.
// It depends on database/sql, the PostgreSQL driver, and injectable connection hooks.
package checks

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/LanceSandino/is-it-up/pkg/logger"
	_ "github.com/lib/pq"
)

// sqlOpen is a package-level variable so tests can replace it with a
// mock (sqlmock) implementation. By default it points to sql.Open.
var sqlOpenPostgres = sql.Open

// checkPostgres runs a Ping and a SELECT 1 against the configured PostgreSQL
// endpoint. It returns true when a SELECT 1 returns 1. The function builds
// a DSN from either a fully-qualified Address or from Host/Port/User/Password
// fields. To support environments with stricter TLS requirements, callers
// may set CheckConfig.TLSMode to control TLS mode (for github.com/lib/pq).
//
// The function intentionally logs a masked debug line showing password
// length rather than content when debug logging is enabled.
func checkPostgres(ctx context.Context, cfg CheckConfig) (bool, string) {
	user := cfg.User.String()
	pass := cfg.Password.String()
	addr := cfg.Address.String()
	db := cfg.Database.String()
	// debug log connection attempt (do not include password)
	if lg := logger.FromContext(ctx); lg != nil && lg.IsDebug() {
		pwdLen := 0
		if pass != "" {
			pwdLen = len(pass)
		}
		lg.Debugw("postgres connect attempt", "check", cfg.Name, "user", user, "addr", redactAddress(addr), "db", db, "pwd_len", pwdLen)
	}
	if addr == "" {
		// If Address wasn't provided, allow Host+Port to be used (common in config)
		host := cfg.Host.String()
		port := cfg.Port.String()
		if host != "" {
			if port == "" {
				port = "5432"
			}
			addr = fmt.Sprintf("%s:%s", host, port)
		} else {
			addr = "127.0.0.1:5432"
		}
	}
	var dsn string
	if strings.Contains(addr, "=") || strings.Contains(addr, " ") {
		// If addr looks like a full DSN, use it as-is
		dsn = addr
	} else {
		if user == "" {
			user = "postgres"
		}
		if db == "" {
			db = "postgres"
		}
		// Build DSN for PostgreSQL
		// Format: postgres://<user>:<credential>@host:port/dbname?sslmode=...
		base := fmt.Sprintf("postgres://%s:%s@%s/%s", user, pass, addr, db)

		// Allow per-check TLS mode configured via canonical CheckConfig.TLSMode
		sslMode := strings.TrimSpace(cfg.TLSMode)
		if sslMode == "" {
			sslMode = "require" // Default to require TLS for security
		}

		dsn = base + "?sslmode=" + sslMode
	}

	tctx, cancel := contextWithDefaultTimeout(ctx, 4*time.Second)
	defer cancel()
	dbh, err := sqlOpenPostgres("postgres", dsn)
	if err != nil {
		checkErr := NewCheckError("postgres", cfg.Name, "connect", err)
		return false, checkErr.Error()
	}
	defer func() { _ = dbh.Close() }()
	if err := dbh.PingContext(tctx); err != nil {
		checkErr := NewCheckError("postgres", cfg.Name, "ping", err)
		return false, checkErr.Error()
	}
	var one int
	row := dbh.QueryRowContext(tctx, "SELECT 1")
	if err := row.Scan(&one); err != nil {
		checkErr := NewCheckError("postgres", cfg.Name, "select", err)
		return false, checkErr.Error()
	}
	if one == 1 {
		return true, "select 1 ok"
	}
	return false, "unexpected select result"
}
