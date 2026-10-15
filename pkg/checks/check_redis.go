// This file implements Redis connectivity, authentication, and INFO health checks.
// It depends on net.Conn and encodes every command with RESP bulk strings.
package checks

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func dialRedis(ctx context.Context, address string, timeout time.Duration) (net.Conn, error) {
	secure := false
	skipVerify := false
	endpoint := address
	if strings.Contains(address, "://") {
		parsed, err := url.Parse(address)
		if err != nil {
			return nil, err
		}
		switch parsed.Scheme {
		case "rediss":
			secure = true
		case "rediss+insecure":
			secure = true
			skipVerify = true
		case "redis":
		default:
			return nil, fmt.Errorf("unsupported Redis scheme %q", parsed.Scheme)
		}
		endpoint = parsed.Host
	}

	dialer := &net.Dialer{Timeout: timeout}
	if !secure {
		return dialer.DialContext(ctx, "tcp", endpoint)
	}
	host, _, err := net.SplitHostPort(endpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid Redis TLS address: %w", err)
	}
	tlsDialer := &tls.Dialer{NetDialer: dialer, Config: &tls.Config{
		ServerName: host, MinVersion: tls.VersionTLS12, InsecureSkipVerify: skipVerify, // #nosec G402 - explicit skip-verify mode
	}}
	return tlsDialer.DialContext(ctx, "tcp", endpoint)
}

// redisCommand encodes arguments as RESP bulk strings so values cannot inject commands.
func redisCommand(args ...string) []byte {
	var builder strings.Builder
	builder.WriteString("*")
	builder.WriteString(strconv.Itoa(len(args)))
	builder.WriteString("\r\n")
	for _, arg := range args {
		builder.WriteString("$")
		builder.WriteString(strconv.Itoa(len(arg)))
		builder.WriteString("\r\n")
		builder.WriteString(arg)
		builder.WriteString("\r\n")
	}
	return []byte(builder.String())
}

// checkRedis performs a simple RESP PING and supports optional AUTH (password).
// It's non-destructive by default and returns (ok, message). The implementation
// uses a raw TCP dial and sends simple RESP commands so tests can provide a
// lightweight fake server.
func checkRedis(ctx context.Context, address, password string) (bool, string) {
	timeout := timeoutFromContext(ctx, 3*time.Second)
	conn, err := dialRedis(ctx, address, timeout)
	if err != nil {
		checkErr := NewCheckError("redis", address, "connect", fmt.Errorf("%s", redactText(err.Error(), password)))
		return false, checkErr.Error()
	}
	defer func() { _ = conn.Close() }()
	_ = setConnectionDeadline(ctx, conn, 3*time.Second)

	// If password provided, send AUTH first
	if password != "" {
		if _, err := conn.Write(redisCommand("AUTH", password)); err != nil {
			checkErr := NewCheckError("redis", address, "auth", fmt.Errorf("%s", redactText(err.Error(), password)))
			return false, checkErr.Error()
		}
		buf := make([]byte, 512)
		n, err := conn.Read(buf)
		if err != nil {
			checkErr := NewCheckError("redis", address, "auth_response", fmt.Errorf("%s", redactText(err.Error(), password)))
			return false, checkErr.Error()
		}
		resp := string(buf[:n])
		if strings.HasPrefix(resp, "-ERR") || strings.Contains(strings.ToLower(resp), "invalid") || strings.Contains(strings.ToLower(resp), "noauth") {
			return false, redactText(strings.TrimSpace(resp), password)
		}
	}

	// Send PING
	if _, err := conn.Write(redisCommand("PING")); err != nil {
		checkErr := NewCheckError("redis", address, "ping", fmt.Errorf("%s", redactText(err.Error(), password)))
		return false, checkErr.Error()
	}
	buf := make([]byte, 512)
	n, err := conn.Read(buf)
	if err != nil {
		checkErr := NewCheckError("redis", address, "ping_response", fmt.Errorf("%s", redactText(err.Error(), password)))
		return false, checkErr.Error()
	}
	resp := string(buf[:n])
	if strings.Contains(resp, "PONG") {
		return true, "pong"
	}
	return false, "no pong: " + redactText(strings.TrimSpace(resp), password)
}

// checkRedisInfo sends an INFO command and parses a few useful keys.
func checkRedisInfo(ctx context.Context, address, password string) (bool, string) {
	timeout := timeoutFromContext(ctx, 3*time.Second)
	conn, err := dialRedis(ctx, address, timeout)
	if err != nil {
		return false, err.Error()
	}
	defer func() { _ = conn.Close() }()
	_ = setConnectionDeadline(ctx, conn, 3*time.Second)

	if password != "" {
		if _, err := conn.Write(redisCommand("AUTH", password)); err != nil {
			return false, err.Error()
		}
		buf := make([]byte, 512)
		n, err := conn.Read(buf)
		if err != nil {
			return false, err.Error()
		}
		resp := string(buf[:n])
		if strings.HasPrefix(resp, "-ERR") {
			return false, strings.TrimSpace(resp)
		}
	}

	if _, err := conn.Write(redisCommand("INFO")); err != nil {
		return false, err.Error()
	}
	b := make([]byte, 8192)
	n, err := conn.Read(b)
	if err != nil {
		return false, err.Error()
	}
	info := string(b[:n])
	// parse a couple keys
	var role string
	var usedMem string
	for _, ln := range strings.Split(info, "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" || strings.HasPrefix(ln, "#") {
			continue
		}
		parts := strings.SplitN(ln, ":", 2)
		if len(parts) != 2 {
			continue
		}
		k := parts[0]
		v := parts[1]
		switch k {
		case "role":
			role = v
		case "used_memory":
			usedMem = v
		}
	}
	msg := fmt.Sprintf("role=%s used_memory=%s", role, usedMem)
	return true, msg
}
