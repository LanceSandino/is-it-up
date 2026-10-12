package checks

import (
	"errors"
	"testing"
	"time"
)

func TestNewCheckError(t *testing.T) {
	baseErr := errors.New("connection failed")
	checkErr := NewCheckError("http", "web-service", "connect", baseErr)

	if checkErr.Type != "http" {
		t.Errorf("Type = %v, expected http", checkErr.Type)
	}
	if checkErr.CheckName != "web-service" {
		t.Errorf("CheckName = %v, expected web-service", checkErr.CheckName)
	}
	if checkErr.Operation != "connect" {
		t.Errorf("Operation = %v, expected connect", checkErr.Operation)
	}
	if checkErr.Err != baseErr {
		t.Errorf("Err = %v, expected %v", checkErr.Err, baseErr)
	}
}

func TestCheckError_Error(t *testing.T) {
	baseErr := errors.New("timeout")
	checkErr := NewCheckError("mysql", "database", "query", baseErr)

	expected := "check 'database' query failed: timeout"
	if got := checkErr.Error(); got != expected {
		t.Errorf("Error() = %v, expected %v", got, expected)
	}

	// Test with duration set directly
	checkErr.Duration = 2 * time.Second
	expectedWithDuration := "check 'database' query failed after 2s: timeout"
	if got := checkErr.Error(); got != expectedWithDuration {
		t.Errorf("Error() with duration = %v, expected %v", got, expectedWithDuration)
	}
}
