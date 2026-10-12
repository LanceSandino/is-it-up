// This file implements Redis connectivity, authentication, and INFO health checks.
// It depends on net.Conn and encodes every command with RESP bulk strings.
package checks

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

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
	d := net.Dialer{Timeout: timeout}
	conn, err := d.DialContext(ctx, "tcp", address)
	if err != nil {
		checkErr := NewCheckError("redis", address, "connect", err)
		return false, checkErr.Error()
	}
	defer func() { _ = conn.Close() }()
	_ = setConnectionDeadline(ctx, conn, 3*time.Second)

	// If password provided, send AUTH first
	if password != "" {
		if _, err := conn.Write(redisCommand("AUTH", password)); err != nil {
			checkErr := NewCheckError("redis", address, "auth", err)
			return false, checkErr.Error()
		}
		buf := make([]byte, 512)
		n, err := conn.Read(buf)
		if err != nil {
			checkErr := NewCheckError("redis", address, "auth_response", err)
			return false, checkErr.Error()
		}
		resp := string(buf[:n])
		if strings.HasPrefix(resp, "-ERR") || strings.Contains(strings.ToLower(resp), "invalid") || strings.Contains(strings.ToLower(resp), "noauth") {
			return false, strings.TrimSpace(resp)
		}
	}

	// Send PING
	if _, err := conn.Write(redisCommand("PING")); err != nil {
		checkErr := NewCheckError("redis", address, "ping", err)
		return false, checkErr.Error()
	}
	buf := make([]byte, 512)
	n, err := conn.Read(buf)
	if err != nil {
		checkErr := NewCheckError("redis", address, "ping_response", err)
		return false, checkErr.Error()
	}
	resp := string(buf[:n])
	if strings.Contains(resp, "PONG") {
		return true, "pong"
	}
	return false, "no pong: " + strings.TrimSpace(resp)
}

// checkRedisInfo sends an INFO command and parses a few useful keys.
func checkRedisInfo(ctx context.Context, address, password string) (bool, string) {
	timeout := timeoutFromContext(ctx, 3*time.Second)
	d := net.Dialer{Timeout: timeout}
	conn, err := d.DialContext(ctx, "tcp", address)
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
