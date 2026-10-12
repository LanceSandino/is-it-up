// This file implements MySQL connection and query health checks.
// It depends on database/sql, the MySQL driver, and injectable connection hooks.
package checks

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/LanceSandino/is-it-up/pkg/logger"
	_ "github.com/go-sql-driver/mysql"
)

// sqlOpen is a package-level variable so tests can replace it with a
// mock (sqlmock) implementation. By default it points to sql.Open.
var sqlOpen = sql.Open

// checkMySQL runs a Ping and a SELECT 1 against the configured MySQL
// endpoint. It returns true when a SELECT 1 returns 1. The function builds
// a DSN from either a fully-qualified Address or from Host/Port/User/Password
// fields. To support environments with stricter TLS requirements, callers
// may set CheckConfig.TLSMode to a recognized tls config name (for
// github.com/go-sql-driver/mysql) which will be appended to the DSN.
//
// The function intentionally logs a masked debug line showing password
// length rather than content when debug logging is enabled.
func checkMySQL(ctx context.Context, cfg CheckConfig) (bool, string) {
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
		lg.Debugw("mysql connect attempt", "check", cfg.Name, "user", user, "addr", redactAddress(addr), "db", db, "pwd_len", pwdLen)
	}
	if addr == "" {
		// If Address wasn't provided, allow Host+Port to be used (common in config)
		host := cfg.Host.String()
		port := cfg.Port.String()
		if host != "" {
			if port == "" {
				port = "3306"
			}
			addr = fmt.Sprintf("%s:%s", host, port)
		} else {
			addr = "127.0.0.1:3306"
		}
	}
	var dsn string
	if strings.Contains(addr, "@") {
		dsn = addr
	} else {
		if user == "" {
			user = "root"
		}
		// Build base DSN and optionally append TLS param if requested via
		// The canonical per-check setting is CheckConfig.TLSMode (tls_mode in YAML).
		// e.g. "skip-verify" or "true" which map to go-sql-driver/mysql tls
		// configs.
		base := ""
		if db == "" {
			base = fmt.Sprintf("%s:%s@tcp(%s)/", user, pass, addr)
		} else {
			base = fmt.Sprintf("%s:%s@tcp(%s)/%s", user, pass, addr, db)
		}
		// Allow per-check TLS mode configured via canonical CheckConfig.TLSMode
		tlsMode := strings.TrimSpace(cfg.TLSMode)
		if tlsMode == "" {
			dsn = base
		} else {
			// If DSN already contains a '?', append with '&' otherwise use '?'
			sep := "?"
			if strings.Contains(base, "?") {
				sep = "&"
			}
			dsn = base + sep + "tls=" + tlsMode
		}
	}

	tctx, cancel := contextWithDefaultTimeout(ctx, 4*time.Second)
	defer cancel()
	dbh, err := sqlOpen("mysql", dsn)
	if err != nil {
		checkErr := NewCheckError("mysql", cfg.Name, "connect", err)
		return false, checkErr.Error()
	}
	defer func() { _ = dbh.Close() }()
	if err := dbh.PingContext(tctx); err != nil {
		checkErr := NewCheckError("mysql", cfg.Name, "ping", err)
		return false, checkErr.Error()
	}
	var one int
	row := dbh.QueryRowContext(tctx, "SELECT 1")
	if err := row.Scan(&one); err != nil {
		checkErr := NewCheckError("mysql", cfg.Name, "select", err)
		return false, checkErr.Error()
	}
	if one == 1 {
		return true, "select 1 ok"
	}
	return false, "unexpected select result"
}
