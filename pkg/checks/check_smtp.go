// This file implements plain, STARTTLS, and implicit-TLS SMTP health checks.
// It depends on replaceable network operations so protocol behavior remains testable.
package checks

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"

	"github.com/LanceSandino/is-it-up/v2/pkg/logger"
)

const maxSMTPResponseBytes int64 = 64 << 10

var errSMTPResponseLimit = errors.New("smtp response limit exceeded")

type smtpReadLimitConn struct {
	net.Conn
	remaining int64
}

func newSMTPReadLimitConn(conn net.Conn) net.Conn {
	return &smtpReadLimitConn{Conn: conn, remaining: maxSMTPResponseBytes}
}

func (c *smtpReadLimitConn) Read(p []byte) (int, error) {
	if c.remaining <= 0 {
		return 0, errSMTPResponseLimit
	}
	if int64(len(p)) > c.remaining {
		p = p[:c.remaining]
	}
	n, err := c.Conn.Read(p)
	c.remaining -= int64(n)
	return n, err
}

// checkSMTP performs SMTP connectivity and authentication verification.
// It connects to the SMTP server, negotiates encryption if specified, and attempts
// authentication with the provided credentials. Returns true when the complete
// SMTP handshake and authentication succeed.
func checkSMTP(ctx context.Context, cfg CheckConfig) (bool, string) {
	user := cfg.User.String()
	pass := cfg.Password.String()
	addr := cfg.Address.String()
	encryption := strings.ToLower(cfg.SMTPEncryption.String())

	// Debug log connection attempt (do not include password)
	if lg := logger.FromContext(ctx); lg != nil && lg.IsDebug() {
		pwdLen := 0
		if pass != "" {
			pwdLen = len(pass)
		}
		lg.Debugw("smtp connect attempt", "check", cfg.Name, "user", user, "addr", redactAddress(addr), "encryption", encryption, "pwd_len", pwdLen)
	}

	// Build address from host+port if address wasn't provided
	if addr == "" {
		host := cfg.Host.String()
		port := cfg.Port.String()
		if host != "" {
			if port == "" {
				// Default port based on encryption
				if encryption == "ssl" || encryption == "smtps" {
					port = "465"
				} else {
					port = "587" // STARTTLS or plain
				}
			}
			addr = fmt.Sprintf("%s:%s", host, port)
		} else {
			addr = "localhost:587"
		}
	}

	// Apply the protocol default only when the caller did not configure a deadline.
	tctx, cancel := contextWithDefaultTimeout(ctx, 10*time.Second)
	defer cancel()

	// Different connection methods based on encryption
	switch encryption {
	case "ssl", "smtps":
		return checkSMTPSSL(tctx, addr, user, pass)
	case "tls", "starttls", "":
		return checkSMTPStartTLS(tctx, addr, user, pass)
	case "none", "plain":
		return checkSMTPPlain(tctx, addr, user, pass)
	default:
		return false, fmt.Sprintf("unknown smtp encryption mode: %s", encryption)
	}
}

// checkSMTPSSL handles SMTP over SSL (SMTPS) connections
func checkSMTPSSL(ctx context.Context, addr, user, pass string) (bool, string) {
	// Connect with TLS
	config := &tls.Config{ServerName: strings.Split(addr, ":")[0], MinVersion: tls.VersionTLS12}

	// Use context-aware dialing
	d := &net.Dialer{}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		checkErr := NewCheckError("smtp", "", "ssl_connect", errors.New(redactText(err.Error(), user, pass)))
		return false, checkErr.Error()
	}
	defer func() { _ = conn.Close() }()
	if err := setConnectionDeadline(ctx, conn, 10*time.Second); err != nil {
		checkErr := NewCheckError("smtp", "", "ssl_deadline", errors.New(redactText(err.Error(), user, pass)))
		return false, checkErr.Error()
	}

	tlsConn := tls.Client(conn, config)
	if err := tlsConn.Handshake(); err != nil {
		checkErr := NewCheckError("smtp", "", "ssl_handshake", errors.New(redactText(err.Error(), user, pass)))
		return false, checkErr.Error()
	}

	client, err := smtp.NewClient(newSMTPReadLimitConn(tlsConn), strings.Split(addr, ":")[0])
	if err != nil {
		checkErr := NewCheckError("smtp", "", "ssl_smtp_client", errors.New(redactText(err.Error(), user, pass)))
		return false, checkErr.Error()
	}
	defer func() {
		if err := client.Quit(); err != nil {
			// SMTP Quit errors during cleanup are typically not critical
			// but we acknowledge the error occurred
			_ = err
		}
	}()

	return authenticateAndTest(client, user, pass, "ssl", strings.Split(addr, ":")[0])
}

// checkSMTPStartTLS handles SMTP with STARTTLS encryption
func checkSMTPStartTLS(ctx context.Context, addr, user, pass string) (bool, string) {
	// Use context-aware dialing
	d := &net.Dialer{}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		checkErr := NewCheckError("smtp", "", "starttls_connect", errors.New(redactText(err.Error(), user, pass)))
		return false, checkErr.Error()
	}
	defer func() { _ = conn.Close() }()
	if err := setConnectionDeadline(ctx, conn, 10*time.Second); err != nil {
		checkErr := NewCheckError("smtp", "", "starttls_deadline", errors.New(redactText(err.Error(), user, pass)))
		return false, checkErr.Error()
	}

	client, err := smtp.NewClient(newSMTPReadLimitConn(conn), strings.Split(addr, ":")[0])
	if err != nil {
		checkErr := NewCheckError("smtp", "", "starttls_smtp_client", errors.New(redactText(err.Error(), user, pass)))
		return false, checkErr.Error()
	}
	defer func() {
		if err := client.Quit(); err != nil {
			// SMTP Quit errors during cleanup are typically not critical
			// but we acknowledge the error occurred
			_ = err
		}
	}()

	// Check if STARTTLS is supported
	if ok, _ := client.Extension("STARTTLS"); ok {
		config := &tls.Config{ServerName: strings.Split(addr, ":")[0], MinVersion: tls.VersionTLS12}
		if err := client.StartTLS(config); err != nil {
			checkErr := NewCheckError("smtp", "", "starttls", errors.New(redactText(err.Error(), user, pass)))
			return false, checkErr.Error()
		}
	} else {
		return false, "smtp STARTTLS required but server did not advertise STARTTLS"
	}

	return authenticateAndTest(client, user, pass, "starttls", strings.Split(addr, ":")[0])
}

// checkSMTPPlain handles plain SMTP connections (no encryption)
func checkSMTPPlain(ctx context.Context, addr, user, pass string) (bool, string) {
	// Use context-aware dialing
	d := &net.Dialer{}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		checkErr := NewCheckError("smtp", "", "plain_connect", errors.New(redactText(err.Error(), user, pass)))
		return false, checkErr.Error()
	}
	defer func() { _ = conn.Close() }()
	if err := setConnectionDeadline(ctx, conn, 10*time.Second); err != nil {
		checkErr := NewCheckError("smtp", "", "plain_deadline", errors.New(redactText(err.Error(), user, pass)))
		return false, checkErr.Error()
	}

	client, err := smtp.NewClient(newSMTPReadLimitConn(conn), strings.Split(addr, ":")[0])
	if err != nil {
		checkErr := NewCheckError("smtp", "", "plain_smtp_client", errors.New(redactText(err.Error(), user, pass)))
		return false, checkErr.Error()
	}
	defer func() {
		if err := client.Quit(); err != nil {
			// SMTP Quit errors during cleanup are typically not critical
			// but we acknowledge the error occurred
			_ = err
		}
	}()

	return authenticateAndTest(client, user, pass, "plain", strings.Split(addr, ":")[0])
}

// authenticateAndTest performs SMTP authentication and basic protocol test
func authenticateAndTest(client *smtp.Client, user, pass, encType, hostname string) (bool, string) {
	// If credentials provided, attempt authentication
	if user != "" && pass != "" {
		// Try different auth mechanisms
		auth := smtp.PlainAuth("", user, pass, hostname)
		if err := client.Auth(auth); err != nil {
			checkErr := NewCheckError("smtp", "", "auth", errors.New(redactText(err.Error(), user, pass)))
			return false, checkErr.Error()
		}
		return true, fmt.Sprintf("smtp auth ok (%s)", encType)
	}

	// No credentials - just verify SMTP protocol works
	return true, fmt.Sprintf("smtp connect ok (%s)", encType)
}
