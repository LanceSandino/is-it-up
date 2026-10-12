package checks

import (
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

// TestCheckRedisInfo_Parse verifies that checkRedisInfo correctly handles
// AUTH (when a password is provided) and parses an INFO-style response for
// keys like role and used_memory. It uses a tiny TCP listener that speaks a
// minimal RESP-like subset (responds +OK for AUTH and a bulk-like reply for INFO).
func TestCheckRedisInfo_Parse(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Close() }()

	// sample INFO payload (lines separated by \n)
	infoPayload := "# Server\r\nrole:master\nused_memory:12345\n"

	go func() {
		conn, err := l.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		buf := make([]byte, 1024)
		for {
			n, err := conn.Read(buf)
			if err != nil {
				return
			}
			req := string(buf[:n])
			if len(req) == 0 {
				continue
			}
			if strings.Contains(req, "\r\nAUTH\r\n") {
				// respond with +OK\r\n
				if _, err := conn.Write([]byte("+OK\r\n")); err != nil {
					return
				}
				continue
			}
			if strings.Contains(req, "\r\nINFO\r\n") {
				// send a simple bulk-like response (length not strictly required for our parser)
				if _, err := conn.Write([]byte("$")); err != nil {
					return
				}
				if _, err := fmt.Fprintf(conn, "%d", len(infoPayload)); err != nil {
					return
				}
				if _, err := conn.Write([]byte("\r\n")); err != nil {
					return
				}
				if _, err := conn.Write([]byte(infoPayload)); err != nil {
					return
				}
				return
			}
		}
	}()

	addr := l.Addr().String()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ok, msg := checkRedisInfo(ctx, addr, "password")
	if !ok {
		t.Fatalf("expected ok got false: %s", msg)
	}
	if msg == "" || !strings.Contains(msg, "role=") || !strings.Contains(msg, "used_memory=") {
		t.Fatalf("unexpected info message: %q", msg)
	}
}
