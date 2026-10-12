// This file implements basic TCP connection health checks.
// It depends on a bounded network dial and returns sanitized diagnostics.
package checks

import (
	"context"
	"fmt"
	"net"
	"time"
)

// checkTCP performs a simple TCP dial to the provided address. It does not
// exchange any data; the goal is purely to verify the TCP socket opens.
func checkTCP(ctx context.Context, address string) (bool, string) {
	d := net.Dialer{Timeout: timeoutFromContext(ctx, 3*time.Second)}
	conn, err := d.DialContext(ctx, "tcp", address)
	if err != nil {
		// Return structured error information for better debugging
		checkErr := NewCheckError("tcp", address, "connect", err)
		return false, checkErr.Error()
	}
	if err := conn.Close(); err != nil {
		// Log the close error but still consider the connection test successful
		// since we were able to connect
		return true, fmt.Sprintf("tcp connect ok (close error: %v)", err)
	}
	return true, "tcp connect ok"
}
