// This file implements PostgreSQL connection and query health checks.
// It depends on database/sql, the PostgreSQL driver, and injectable connection hooks.
package checks

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/LanceSandino/is-it-up/v2/pkg/logger"
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
	sslMode := strings.TrimSpace(cfg.TLSMode)
	if sslMode == "" {
		sslMode = "verify-full"
	}
	connectTimeout := postgresConnectTimeout(ctx)

	var dsn string
	if !strings.HasPrefix(addr, "postgres://") && !strings.HasPrefix(addr, "postgresql://") && (strings.Contains(addr, "=") || strings.Contains(addr, " ")) {
		// lib/pq applies the last duplicate keyword. Append canonical policy so
		// an embedded or secret-derived component cannot override it.
		dsn = strings.TrimSpace(addr) + " sslmode='" + sslMode + "' connect_timeout='" + strconv.Itoa(connectTimeout) + "'"
	} else {
		if user == "" {
			user = "postgres"
		}
		if db == "" {
			db = "postgres"
		}
		var parsed *url.URL
		var err error
		if strings.HasPrefix(addr, "postgres://") || strings.HasPrefix(addr, "postgresql://") {
			parsed, err = url.Parse(addr)
			if err != nil {
				return false, NewCheckError("postgres", cfg.Name, "config", err).Error()
			}
			if cfg.User.HasValue() || cfg.Password.HasValue() {
				parsed.User = url.UserPassword(user, pass)
			}
			if cfg.Database.HasValue() {
				parsed.Path = "/" + db
			}
		} else {
			parsed = &url.URL{Scheme: "postgres", User: url.UserPassword(user, pass), Host: addr, Path: "/" + db}
		}
		query := parsed.Query()
		query.Set("sslmode", sslMode)
		query.Set("connect_timeout", strconv.Itoa(connectTimeout))
		parsed.RawQuery = query.Encode()
		dsn = parsed.String()
	}

	tctx, cancel := contextWithDefaultTimeout(ctx, 4*time.Second)
	defer cancel()
	dbh, err := sqlOpenPostgres("postgres", dsn)
	if err != nil {
		checkErr := NewCheckError("postgres", cfg.Name, "connect", fmt.Errorf("%s", redactText(err.Error(), user, pass)))
		return false, checkErr.Error()
	}
	defer func() { _ = dbh.Close() }()
	if err := dbh.PingContext(tctx); err != nil {
		checkErr := NewCheckError("postgres", cfg.Name, "ping", fmt.Errorf("%s", redactText(err.Error(), user, pass)))
		return false, checkErr.Error()
	}
	var one int
	row := dbh.QueryRowContext(tctx, "SELECT 1")
	if err := row.Scan(&one); err != nil {
		checkErr := NewCheckError("postgres", cfg.Name, "select", fmt.Errorf("%s", redactText(err.Error(), user, pass)))
		return false, checkErr.Error()
	}
	if one == 1 {
		return true, "select 1 ok"
	}
	return false, "unexpected select result"
}

func postgresConnectTimeout(ctx context.Context) int {
	timeout := timeoutFromContext(ctx, 4*time.Second)
	seconds := int((timeout + time.Second - 1) / time.Second)
	if seconds < 1 {
		return 1
	}
	return seconds
}
