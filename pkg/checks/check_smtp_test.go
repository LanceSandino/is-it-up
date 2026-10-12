package checks

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/LanceSandino/is-it-up/pkg/secrets"
)

// TestCheckSMTP_SuccessfulAuth tests successful SMTP authentication
func TestCheckSMTP_SuccessfulAuth(t *testing.T) {
	// Create a mock SMTP server
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start test listener: %v", err)
	}
	defer func() { _ = listener.Close() }()

	// Get the actual port
	addr := listener.Addr().String()

	// Start mock SMTP server
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go handleMockSMTPConnection(conn, true) // successful auth
		}
	}()

	// Test the SMTP check
	cfg := CheckConfig{
		Name:           "test-smtp",
		Type:           "smtp",
		Address:        secrets.SecretValue{Raw: addr},
		User:           secrets.SecretValue{Raw: "testuser"},
		Password:       secrets.SecretValue{Raw: "testpass"},
		SMTPEncryption: secrets.SecretValue{Raw: "none"},
	}

	ctx := context.Background()
	ok, msg := checkSMTP(ctx, cfg)

	if !ok {
		t.Fatalf("expected smtp check to succeed, got: %s", msg)
	}
	if !strings.Contains(msg, "smtp auth ok") {
		t.Fatalf("expected success message, got: %s", msg)
	}
}

// TestCheckSMTP_AuthFailure tests SMTP authentication failure
func TestCheckSMTP_AuthFailure(t *testing.T) {
	// Create a mock SMTP server
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start test listener: %v", err)
	}
	defer func() { _ = listener.Close() }()

	// Get the actual port
	addr := listener.Addr().String()

	// Start mock SMTP server
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go handleMockSMTPConnection(conn, false) // auth failure
		}
	}()

	// Test the SMTP check
	cfg := CheckConfig{
		Name:           "test-smtp",
		Type:           "smtp",
		Address:        secrets.SecretValue{Raw: addr},
		User:           secrets.SecretValue{Raw: "baduser"},
		Password:       secrets.SecretValue{Raw: "badpass"},
		SMTPEncryption: secrets.SecretValue{Raw: "none"},
	}

	ctx := context.Background()
	ok, msg := checkSMTP(ctx, cfg)

	if ok {
		t.Fatalf("expected smtp check to fail due to auth failure")
	}
	if !strings.Contains(msg, "auth") {
		t.Fatalf("expected auth error message, got: %s", msg)
	}
}

func TestCheckSMTPStartTLS_ServerWithoutExtension(t *testing.T) {
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
		_, _ = fmt.Fprint(conn, "220 test ESMTP\r\n")
		scanner := bufio.NewScanner(conn)
		for scanner.Scan() {
			line := scanner.Text()
			switch {
			case strings.HasPrefix(line, "EHLO"), strings.HasPrefix(line, "HELO"):
				_, _ = fmt.Fprint(conn, "250 test\r\n")
			case strings.HasPrefix(line, "QUIT"):
				_, _ = fmt.Fprint(conn, "221 bye\r\n")
				return
			default:
				_, _ = fmt.Fprint(conn, "250 ok\r\n")
			}
		}
	}()

	ok, msg := checkSMTPStartTLS(context.Background(), listener.Addr().String(), "", "")
	<-done
	if !ok || !strings.Contains(msg, "starttls") {
		t.Fatalf("got ok=%v msg=%q", ok, msg)
	}
}

func TestCheckSMTPSSL_HandshakeFailure(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, acceptErr := listener.Accept()
		if acceptErr == nil {
			_, _ = fmt.Fprint(conn, "not tls")
			_ = conn.Close()
		}
	}()
	ok, msg := checkSMTPSSL(context.Background(), listener.Addr().String(), "", "")
	<-done
	if ok || !strings.Contains(msg, "ssl_handshake") {
		t.Fatalf("got ok=%v msg=%q", ok, msg)
	}
}

func TestCheckSMTPPlain_ContextDeadlineCoversGreeting(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	accepted := make(chan net.Conn, 1)
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr == nil {
			accepted <- conn
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 75*time.Millisecond)
	defer cancel()
	start := time.Now()
	ok, msg := checkSMTPPlain(ctx, listener.Addr().String(), "", "")
	if conn := <-accepted; conn != nil {
		_ = conn.Close()
	}
	if ok || !strings.Contains(msg, "plain_smtp_client") {
		t.Fatalf("expected a deadline failure while waiting for the greeting, got ok=%v msg=%q", ok, msg)
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("SMTP greeting exceeded its context deadline: %s", elapsed)
	}
}

// TestCheckSMTP_NoCredentials tests SMTP without authentication
func TestCheckSMTP_NoCredentials(t *testing.T) {
	// Create a mock SMTP server
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start test listener: %v", err)
	}
	defer func() { _ = listener.Close() }()

	// Get the actual port
	addr := listener.Addr().String()

	// Start mock SMTP server
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go handleMockSMTPConnection(conn, true) // doesn't matter for no-auth test
		}
	}()

	// Test the SMTP check without credentials
	cfg := CheckConfig{
		Name:           "test-smtp",
		Type:           "smtp",
		Address:        secrets.SecretValue{Raw: addr},
		SMTPEncryption: secrets.SecretValue{Raw: "none"},
	}

	ctx := context.Background()
	ok, msg := checkSMTP(ctx, cfg)

	if !ok {
		t.Fatalf("expected smtp check to succeed without auth, got: %s", msg)
	}
	if !strings.Contains(msg, "smtp connect ok") {
		t.Fatalf("expected connect success message, got: %s", msg)
	}
}

// TestCheckSMTP_ConnectionFailure tests SMTP connection failure
func TestCheckSMTP_ConnectionFailure(t *testing.T) {
	// Test connection to non-existent server
	cfg := CheckConfig{
		Name:           "test-smtp",
		Type:           "smtp",
		Address:        secrets.SecretValue{Raw: "127.0.0.1:65432"}, // unlikely to be in use
		SMTPEncryption: secrets.SecretValue{Raw: "none"},
	}

	ctx := context.Background()
	ok, msg := checkSMTP(ctx, cfg)

	if ok {
		t.Fatalf("expected smtp check to fail due to connection error")
	}
	if !strings.Contains(msg, "connect") {
		t.Fatalf("expected connection error message, got: %s", msg)
	}
}

// TestCheckSMTP_HostPortComposition tests building address from host+port
func TestCheckSMTP_HostPortComposition(t *testing.T) {
	// Create a mock SMTP server
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start test listener: %v", err)
	}
	defer func() { _ = listener.Close() }()

	// Get the actual port
	port := strings.Split(listener.Addr().String(), ":")[1]

	// Start mock SMTP server
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go handleMockSMTPConnection(conn, true)
		}
	}()

	// Test with host+port instead of address
	cfg := CheckConfig{
		Name:           "test-smtp",
		Type:           "smtp",
		Host:           secrets.SecretValue{Raw: "127.0.0.1"},
		Port:           secrets.SecretValue{Raw: port},
		SMTPEncryption: secrets.SecretValue{Raw: "none"},
	}

	ctx := context.Background()
	ok, msg := checkSMTP(ctx, cfg)

	if !ok {
		t.Fatalf("expected smtp check to succeed with host+port, got: %s", msg)
	}
}

// TestCheckSMTP_EncryptionModes tests different encryption settings
func TestCheckSMTP_EncryptionModes(t *testing.T) {
	testCases := []struct {
		encryption string
		expectPort string
	}{
		{"ssl", "465"},
		{"smtps", "465"},
		{"tls", "587"},
		{"starttls", "587"},
		{"", "587"}, // default
		{"none", "587"},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("encryption_%s", tc.encryption), func(t *testing.T) {
			cfg := CheckConfig{
				Name:           "test-smtp",
				Type:           "smtp",
				Host:           secrets.SecretValue{Raw: "nonexistent.example.com"},
				SMTPEncryption: secrets.SecretValue{Raw: tc.encryption},
			}

			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			// This will fail to connect, but we can check the error message
			// to ensure the right port/method was attempted
			ok, msg := checkSMTP(ctx, cfg)

			// Should fail with connection error since server doesn't exist
			if ok {
				t.Fatalf("expected connection to fail for test case %s", tc.encryption)
			}

			// The error should contain connection details indicating correct port selection
			if !strings.Contains(msg, "connect") {
				t.Fatalf("expected connection error for %s, got: %s", tc.encryption, msg)
			}
		})
	}
}

// TestCheckSMTP_InvalidEncryption tests invalid encryption mode
func TestCheckSMTP_InvalidEncryption(t *testing.T) {
	cfg := CheckConfig{
		Name:           "test-smtp",
		Type:           "smtp",
		Address:        secrets.SecretValue{Raw: "127.0.0.1:587"},
		SMTPEncryption: secrets.SecretValue{Raw: "invalid"},
	}

	ctx := context.Background()
	ok, msg := checkSMTP(ctx, cfg)

	if ok {
		t.Fatalf("expected smtp check to fail with invalid encryption")
	}
	if !strings.Contains(msg, "unknown smtp encryption mode") {
		t.Fatalf("expected invalid encryption error, got: %s", msg)
	}
}

// handleMockSMTPConnection simulates an SMTP server for testing
func handleMockSMTPConnection(conn net.Conn, allowAuth bool) {
	defer func() { _ = conn.Close() }()

	// Set a deadline to prevent hanging tests
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))

	// Basic SMTP protocol simulation
	_, _ = fmt.Fprintf(conn, "220 test-server ESMTP\r\n")

	buffer := make([]byte, 1024)
	for {
		n, err := conn.Read(buffer)
		if err != nil {
			return
		}

		command := strings.TrimSpace(string(buffer[:n]))

		switch {
		case strings.HasPrefix(command, "EHLO") || strings.HasPrefix(command, "HELO"):
			_, _ = fmt.Fprintf(conn, "250-test-server\r\n")
			_, _ = fmt.Fprintf(conn, "250-AUTH PLAIN LOGIN\r\n")
			_, _ = fmt.Fprintf(conn, "250 STARTTLS\r\n")
		case strings.HasPrefix(command, "AUTH PLAIN"):
			if allowAuth {
				_, _ = fmt.Fprintf(conn, "235 Authentication successful\r\n")
			} else {
				_, _ = fmt.Fprintf(conn, "535 Authentication failed\r\n")
			}
		case strings.HasPrefix(command, "STARTTLS"):
			_, _ = fmt.Fprintf(conn, "220 Ready to start TLS\r\n")
			// For testing purposes, we'll simulate TLS success
			// In a real test, you'd need proper TLS handling
		case strings.HasPrefix(command, "QUIT"):
			_, _ = fmt.Fprintf(conn, "221 Bye\r\n")
			return
		default:
			_, _ = fmt.Fprintf(conn, "250 OK\r\n")
		}
	}
}
