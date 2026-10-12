package checks

import (
	"context"
	"net"
	"strings"
	"testing"
)

func TestRedisCommand_UsesBulkStrings(t *testing.T) {
	got := string(redisCommand("AUTH", "line1\r\nPING"))
	if got != "*2\r\n$4\r\nAUTH\r\n$11\r\nline1\r\nPING\r\n" {
		t.Fatalf("unexpected RESP command: %q", got)
	}
	if strings.HasPrefix(got, "AUTH ") {
		t.Fatalf("command used injection-prone inline encoding: %q", got)
	}
}

func TestCheckRedis_ConnectionError(t *testing.T) {
	// Test with a non-existent address
	ok, msg := checkRedis(context.Background(), "localhost:99999", "")
	if ok {
		t.Errorf("checkRedis() = true, expected false for connection error")
	}
	if msg == "" {
		t.Errorf("checkRedis() msg should not be empty on error")
	}
}

func TestCheckRedis_InvalidAddress(t *testing.T) {
	// Test with an invalid address format
	ok, msg := checkRedis(context.Background(), "invalid-address", "")
	if ok {
		t.Errorf("checkRedis() = true, expected false for invalid address")
	}
	if msg == "" {
		t.Errorf("checkRedis() msg should not be empty on error")
	}
}

func TestCheckRedis_ProtocolResponses(t *testing.T) {
	tests := []struct {
		name      string
		password  string
		responses []string
		want      bool
		contains  string
	}{
		{name: "ping", responses: []string{"+PONG\r\n"}, want: true, contains: "pong"},
		{name: "no pong", responses: []string{"+OK\r\n"}, want: false, contains: "no pong"},
		{name: "auth then ping", password: "safe", responses: []string{"+OK\r\n", "+PONG\r\n"}, want: true, contains: "pong"},
		{name: "auth rejected", password: "bad", responses: []string{"-ERR invalid password\r\n"}, want: false, contains: "invalid"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			done := make(chan struct{})
			go func() {
				defer close(done)
				conn, acceptErr := listener.Accept()
				if acceptErr != nil {
					return
				}
				defer conn.Close()
				for _, response := range tt.responses {
					buffer := make([]byte, 512)
					if _, readErr := conn.Read(buffer); readErr != nil {
						return
					}
					_, _ = conn.Write([]byte(response))
				}
			}()
			ok, msg := checkRedis(context.Background(), listener.Addr().String(), tt.password)
			<-done
			if ok != tt.want || !strings.Contains(strings.ToLower(msg), tt.contains) {
				t.Fatalf("got ok=%v msg=%q, want ok=%v containing %q", ok, msg, tt.want, tt.contains)
			}
		})
	}
}
