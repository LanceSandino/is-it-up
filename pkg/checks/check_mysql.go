// This file implements MySQL connection and query health checks.
// It depends on database/sql, the MySQL driver, and injectable connection hooks.
package checks

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/LanceSandino/is-it-up/v2/pkg/logger"
	mysqlDriver "github.com/go-sql-driver/mysql"
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
	if strings.ContainsAny(user, ":@/[]()") {
		return false, NewCheckError("mysql", cfg.Name, "config", errors.New("MySQL user contains DSN delimiter characters")).Error()
	}
	var mysqlConfig *mysqlDriver.Config
	var err error
	if strings.Contains(addr, "@") {
		mysqlConfig, err = mysqlDriver.ParseDSN(addr)
		if err != nil {
			return false, NewCheckError("mysql", cfg.Name, "config", errors.New(redactText(err.Error(), user, pass))).Error()
		}
	} else {
		if user == "" {
			user = "root"
		}
		mysqlConfig = mysqlDriver.NewConfig()
		mysqlConfig.User = user
		mysqlConfig.Passwd = pass
		mysqlConfig.Net = "tcp"
		mysqlConfig.Addr = addr
		mysqlConfig.DBName = db
	}

	hasPassword := mysqlConfig.Passwd != "" || pass != ""
	tlsSetting, err := mysqlTLSSetting(cfg.TLSMode, hasPassword)
	if err != nil {
		return false, NewCheckError("mysql", cfg.Name, "config", err).Error()
	}
	mysqlConfig.TLSConfig = tlsSetting
	requestTimeout := timeoutFromContext(ctx, 4*time.Second)
	mysqlConfig.Timeout = requestTimeout
	mysqlConfig.ReadTimeout = requestTimeout
	mysqlConfig.WriteTimeout = requestTimeout
	dsn := mysqlConfig.FormatDSN()

	tctx, cancel := contextWithDefaultTimeout(ctx, 4*time.Second)
	defer cancel()
	dbh, err := sqlOpen("mysql", dsn)
	if err != nil {
		checkErr := NewCheckError("mysql", cfg.Name, "connect", fmt.Errorf("%s", redactText(err.Error(), user, pass)))
		return false, checkErr.Error()
	}
	defer func() { _ = dbh.Close() }()
	if err := dbh.PingContext(tctx); err != nil {
		checkErr := NewCheckError("mysql", cfg.Name, "ping", fmt.Errorf("%s", redactText(err.Error(), user, pass)))
		return false, checkErr.Error()
	}
	var one int
	row := dbh.QueryRowContext(tctx, "SELECT 1")
	if err := row.Scan(&one); err != nil {
		checkErr := NewCheckError("mysql", cfg.Name, "select", fmt.Errorf("%s", redactText(err.Error(), user, pass)))
		return false, checkErr.Error()
	}
	if one == 1 {
		return true, "select 1 ok"
	}
	return false, "unexpected select result"
}

func mysqlTLSSetting(mode string, hasPassword bool) (string, error) {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "":
		if hasPassword {
			return "true", nil
		}
		return "", nil
	case "required":
		return "true", nil
	case "skip-verify", "preferred":
		return strings.ToLower(strings.TrimSpace(mode)), nil
	case "disable":
		return "", nil
	default:
		return "", fmt.Errorf("unsupported MySQL tls_mode %q", mode)
	}
}
