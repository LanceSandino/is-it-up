package checks

import (
	"context"
	"net"
	"testing"
)

func TestCheckTCP_ConnectionError(t *testing.T) {
	// Test with a non-existent port
	ok, msg := checkTCP(context.Background(), "localhost:99999")
	if ok {
		t.Errorf("checkTCP() = true, expected false for connection error")
	}
	if msg == "" {
		t.Errorf("checkTCP() msg should not be empty on error")
	}
}

func TestCheckTCP_InvalidAddress(t *testing.T) {
	// Test with an invalid address format
	ok, msg := checkTCP(context.Background(), "invalid-address")
	if ok {
		t.Errorf("checkTCP() = true, expected false for invalid address")
	}
	if msg == "" {
		t.Errorf("checkTCP() msg should not be empty on error")
	}
}

func TestCheckTCP_Success(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	done := make(chan struct{})
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr == nil {
			_ = conn.Close()
		}
		close(done)
	}()
	ok, msg := checkTCP(context.Background(), listener.Addr().String())
	<-done
	if !ok || msg != "tcp connect ok" {
		t.Fatalf("got ok=%v msg=%q", ok, msg)
	}
}
