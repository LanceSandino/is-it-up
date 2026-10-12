// This file implements RabbitMQ AMQP and management API health checks.
// It depends on safe URI construction, the AMQP client, and shared HTTP behavior.
package checks

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// package-level var so tests can stub the dial behavior
var checkRabbitAMQPFn = checkRabbitAMQP

// amqpConn abstracts the minimal methods we need from *amqp.Connection so
// tests can supply fakes without importing the full implementation.
type amqpConn interface {
	Channel() (amqpChannel, error)
	Close() error
}

// amqpChannel is a tiny abstraction over the real amqp.Channel for testing.
type amqpChannel interface {
	Close() error
}

// amqpDialFn is a package-level variable that performs the actual dial. It is
// a wrapper around amqp.DialConfig by default but can be swapped in tests.
// realConnAdapter wraps the real *amqp.Connection so it conforms to our
// amqpConn interface (returns amqpChannel) while still using the upstream
// library in production.
type realConnAdapter struct {
	c       *amqp.Connection
	netConn net.Conn
}

func (r *realConnAdapter) Channel() (amqpChannel, error) {
	ch, err := r.c.Channel()
	if err != nil {
		return nil, err
	}
	return ch, nil
}

func (r *realConnAdapter) Close() error { return r.c.Close() }

func (r *realConnAdapter) SetDeadline(deadline time.Time) error {
	if r.netConn == nil {
		return nil
	}
	return r.netConn.SetDeadline(deadline)
}

var amqpDialFn = func(uri string, cfg amqp.Config) (amqpConn, error) {
	originalDial := cfg.Dial
	if originalDial == nil {
		originalDial = amqp.DefaultDial(30 * time.Second)
	}
	var netConn net.Conn
	cfg.Dial = func(network, address string) (net.Conn, error) {
		conn, err := originalDial(network, address)
		if err == nil {
			netConn = conn
		}
		return conn, err
	}
	conn, err := amqp.DialConfig(uri, cfg)
	if err != nil {
		return nil, err
	}
	return &realConnAdapter{c: conn, netConn: netConn}, nil
}

// checkRabbitAMQP dials AMQP, opens a channel, and closes it. Non-destructive.
func checkRabbitAMQP(ctx context.Context, address, user, password, vhost string) (bool, string) {
	// Build URI with proper vhost handling
	// Default to "/" if vhost is empty, otherwise use the specified vhost
	if vhost == "" {
		vhost = "/"
	}
	uri := fmt.Sprintf("amqp://%s@%s/%s", url.UserPassword(user, password).String(), address, url.PathEscape(vhost))
	// Use a short dial timeout via amqp's DefaultDial helper
	conn, err := amqpDialFn(uri, amqp.Config{Locale: "en_US", Dial: amqp.DefaultDial(timeoutFromContext(ctx, 4*time.Second))})
	if err != nil {
		return false, redactText(err.Error(), user, password)
	}
	defer func() { _ = conn.Close() }()
	if deadline, ok := ctx.Deadline(); ok {
		if deadlineConn, supported := conn.(interface{ SetDeadline(time.Time) error }); supported {
			if err := deadlineConn.SetDeadline(deadline); err != nil {
				return false, redactText(err.Error(), user, password)
			}
		}
	}
	ch, err := conn.Channel()
	if err != nil {
		return false, redactText(err.Error(), user, password)
	}
	_ = ch.Close()
	return true, "amqp connect ok"
}
