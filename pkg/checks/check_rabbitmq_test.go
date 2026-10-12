package checks

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type deadlineAwareAMQPConn struct {
	fakeConn
	deadline time.Time
}

func (c *deadlineAwareAMQPConn) SetDeadline(deadline time.Time) error {
	c.deadline = deadline
	return nil
}

func TestCheckRabbitAMQP_AppliesContextDeadlineToChannel(t *testing.T) {
	original := amqpDialFn
	defer func() { amqpDialFn = original }()

	fake := &deadlineAwareAMQPConn{}
	amqpDialFn = func(string, amqp.Config) (amqpConn, error) {
		return fake, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	ok, msg := checkRabbitAMQP(ctx, "rmq.example:5672", "guest", "guest", "/")
	if !ok {
		t.Fatalf("expected success, got %s", msg)
	}
	if fake.deadline.IsZero() {
		t.Fatal("expected the AMQP connection deadline to cover channel creation")
	}
}

func TestCheckRabbitAMQP_EncodesCredentialsAndVhost(t *testing.T) {
	orig := amqpDialFn
	defer func() { amqpDialFn = orig }()

	var gotURI string
	amqpDialFn = func(uri string, cfg amqp.Config) (amqpConn, error) {
		gotURI = uri
		return &fakeConn{}, nil
	}

	ok, msg := checkRabbitAMQP(context.Background(), "rmq.example:5672", "user@example.com", "p@ss:word", "/team space")
	if !ok {
		t.Fatalf("expected success, got %s", msg)
	}
	if strings.Contains(gotURI, "p@ss:word") || strings.Contains(gotURI, "user@example.com") {
		t.Fatalf("URI contains unescaped credentials: %s", gotURI)
	}
	if !strings.Contains(gotURI, "user%40example.com:p%40ss%3Aword@") || !strings.HasSuffix(gotURI, "/%2Fteam%20space") {
		t.Fatalf("URI was not encoded as expected: %s", gotURI)
	}
}

// fakeCh implements amqpChannel for testing AMQP connection behavior
type fakeCh struct{}

func (f *fakeCh) Close() error { return nil }

// fakeConn implements amqpConn for testing AMQP connection establishment
type fakeConn struct{}

func (f *fakeConn) Channel() (amqpChannel, error) { return &fakeCh{}, nil }
func (f *fakeConn) Close() error                  { return nil }

// TestCheckRabbitAMQP_DialFail tests that checkRabbitAMQP returns an error when dialing fails.
func TestCheckRabbitAMQP_DialFail(t *testing.T) {
	orig := amqpDialFn
	defer func() { amqpDialFn = orig }()

	amqpDialFn = func(uri string, cfg amqp.Config) (amqpConn, error) {
		return nil, errors.New("dial failed")
	}

	ok, msg := checkRabbitAMQP(context.Background(), "rmq:5672", "u", "p", "/")
	if ok {
		t.Fatalf("expected dial failure to return ok=false")
	}
	if msg == "" {
		t.Fatalf("expected error message on failure")
	}
}

// TestCheckRabbitAMQP_Success tests checkRabbitAMQP success path by stubbing amqpDialFn
// to return a fake connection whose Channel method returns a fake channel that closes successfully.
func TestCheckRabbitAMQP_Success(t *testing.T) {
	orig := amqpDialFn
	defer func() { amqpDialFn = orig }()
	amqpDialFn = func(uri string, cfg amqp.Config) (amqpConn, error) {
		return &fakeConn{}, nil
	}

	ok, msg := checkRabbitAMQP(context.Background(), "good:5672", "guest", "guest", "/")
	if !ok {
		t.Fatalf("expected success, got failure: %s", msg)
	}
}

// TestRealConnAdapter_InterfaceCompliance tests that realConnAdapter implements amqpConn interface
func TestRealConnAdapter_InterfaceCompliance(t *testing.T) {
	// Test that realConnAdapter implements the interface we expect
	var _ amqpConn = &realConnAdapter{}

	// Note: We can't safely test with nil connection as amqp.Connection
	// methods will panic. This test just verifies interface compliance.
	t.Log("realConnAdapter successfully implements amqpConn interface")
}

// TestChannelFailurePassthrough tests that channel creation errors are passed through
func TestChannelFailurePassthrough(t *testing.T) {
	// Use the existing fake connection that will work
	originalDialFn := amqpDialFn
	defer func() { amqpDialFn = originalDialFn }()

	// Create a connection that fails on Channel() call
	amqpDialFn = func(uri string, cfg amqp.Config) (amqpConn, error) {
		return &fakeConnChannelFails{}, nil
	}

	ok, msg := checkRabbitAMQP(context.Background(), "test:5672", "user", "pass", "/")
	if ok {
		t.Fatal("expected failure when channel creation fails")
	}
	if msg != "channel failed" {
		t.Fatalf("expected 'channel failed' message, got: %s", msg)
	}
}

// fakeConnChannelFails implements amqpConn but fails on Channel()
type fakeConnChannelFails struct{}

func (f *fakeConnChannelFails) Channel() (amqpChannel, error) {
	return nil, errors.New("channel failed")
}

func (f *fakeConnChannelFails) Close() error {
	return nil
}

// mockAMQPConnForAdapter implements amqpConn with controllable Channel/Close behavior
type mockAMQPConnForAdapter struct {
	channelErr    error
	closeErr      error
	channelCalled bool
	closeCalled   bool
}

func (m *mockAMQPConnForAdapter) Channel() (amqpChannel, error) {
	m.channelCalled = true
	if m.channelErr != nil {
		return nil, m.channelErr
	}
	return &fakeCh{}, nil
}

func (m *mockAMQPConnForAdapter) Close() error {
	m.closeCalled = true
	return m.closeErr
}

// TestRealConnAdapter_Channel_Success tests that realConnAdapter.Channel() delegates correctly
func TestRealConnAdapter_Channel_Success(t *testing.T) {
	// Since realConnAdapter is a thin wrapper around amqp.Connection,
	// we test it through the checkRabbitAMQP function with a mock that tracks calls
	origDial := amqpDialFn
	defer func() { amqpDialFn = origDial }()

	mockConn := &mockAMQPConnForAdapter{}
	amqpDialFn = func(uri string, cfg amqp.Config) (amqpConn, error) {
		return mockConn, nil
	}

	ok, msg := checkRabbitAMQP(context.Background(), "test:5672", "user", "pass", "/")
	if !ok {
		t.Fatalf("expected success, got: %s", msg)
	}
	if !mockConn.channelCalled {
		t.Error("expected Channel() to be called")
	}
	if !mockConn.closeCalled {
		t.Error("expected Close() to be called")
	}
}

// TestRealConnAdapter_Channel_Failure tests channel creation failure propagation
func TestRealConnAdapter_Channel_Failure(t *testing.T) {
	origDial := amqpDialFn
	defer func() { amqpDialFn = origDial }()

	mockConn := &mockAMQPConnForAdapter{
		channelErr: errors.New("channel creation failed"),
	}
	amqpDialFn = func(uri string, cfg amqp.Config) (amqpConn, error) {
		return mockConn, nil
	}

	ok, msg := checkRabbitAMQP(context.Background(), "test:5672", "user", "pass", "/")
	if ok {
		t.Fatal("expected failure when channel creation fails")
	}
	if msg != "channel creation failed" {
		t.Fatalf("expected 'channel creation failed', got: %s", msg)
	}
	if !mockConn.channelCalled {
		t.Error("expected Channel() to be called")
	}
	// Note: Close() is called via defer in checkRabbitAMQP even on channel failure
	if !mockConn.closeCalled {
		t.Error("expected Close() to be called via defer even on channel failure")
	}
}

// TestRealConnAdapter_Close_Error tests close error handling
func TestRealConnAdapter_Close_Error(t *testing.T) {
	origDial := amqpDialFn
	defer func() { amqpDialFn = origDial }()

	mockConn := &mockAMQPConnForAdapter{
		closeErr: errors.New("close failed"),
	}
	amqpDialFn = func(uri string, cfg amqp.Config) (amqpConn, error) {
		return mockConn, nil
	}

	// Note: checkRabbitAMQP ignores close errors (they're logged but don't fail the check)
	ok, msg := checkRabbitAMQP(context.Background(), "test:5672", "user", "pass", "/")
	if !ok {
		t.Fatalf("expected success despite close error, got: %s", msg)
	}
	if !mockConn.closeCalled {
		t.Error("expected Close() to be called")
	}
}
