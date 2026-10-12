// This file wraps configuration-resolution failures with safe field context.
// It depends only on standard error formatting and never includes resolved values.
package checks

import (
	"fmt"
)

// CheckFieldError represents an error related to a specific field in a check configuration.
type CheckFieldError struct {
	Check string `json:"check"`
	Field string `json:"field"`
	Err   error  `json:"error"`
}

func (e *CheckFieldError) Error() string {
	return fmt.Sprintf("check '%s' field '%s': %v", e.Check, e.Field, e.Err)
}

// Unwrap returns the underlying error.
func (e *CheckFieldError) Unwrap() error {
	return e.Err
}
