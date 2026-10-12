// This file defines consistent diagnostic messages returned by individual checks.
// It depends only on standard formatting and keeps user-facing errors uniform.
package checks

import (
	"fmt"
	"time"
)

// CheckError represents errors that can occur during health checks.
// It includes structured information for better error handling and logging.
type CheckError struct {
	Type      string        `json:"type"`
	CheckName string        `json:"check_name"`
	Operation string        `json:"operation"`
	Err       error         `json:"error"`
	Duration  time.Duration `json:"duration,omitempty"`
	Details   string        `json:"details,omitempty"`
}

func (e *CheckError) Error() string {
	if e.Duration > 0 {
		return fmt.Sprintf("check '%s' %s failed after %v: %v", e.CheckName, e.Operation, e.Duration, e.Err)
	}
	return fmt.Sprintf("check '%s' %s failed: %v", e.CheckName, e.Operation, e.Err)
}

// NewCheckError creates a new structured check error.
func NewCheckError(checkType, checkName, operation string, err error) *CheckError {
	return &CheckError{
		Type:      checkType,
		CheckName: checkName,
		Operation: operation,
		Err:       err,
	}
}
