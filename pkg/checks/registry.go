// This file owns the check-type registry shared by validation and RunAll.
// Check packages register runners at init time; no other module edits dispatch.
package checks

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
)

// CheckOutcome is the check-specific part of a Result. RunAll adds the name,
// configured severity, timestamp, and duration, and redacts secret-backed values.
type CheckOutcome struct {
	OK      bool
	Message string
	Details string
	// Severity overrides the configured severity when set.
	Severity string
}

// CheckFunc executes one check. ctx already carries the check's deadline.
type CheckFunc func(ctx context.Context, cfg CheckConfig) CheckOutcome

var checkTypes = struct {
	sync.RWMutex
	byName map[string]CheckFunc
}{byName: make(map[string]CheckFunc)}

// RegisterCheckType makes a check type available to validation and RunAll.
// It is normally called from a package init function. Empty names, nil runners,
// and duplicate names panic so a binary cannot start with ambiguous dispatch.
func RegisterCheckType(name string, run CheckFunc) {
	name = normalizeCheckType(name)
	if name == "" {
		panic("checks: cannot register an empty check type")
	}
	if run == nil {
		panic(fmt.Sprintf("checks: cannot register %q with a nil runner", name))
	}
	checkTypes.Lock()
	defer checkTypes.Unlock()
	if _, exists := checkTypes.byName[name]; exists {
		panic(fmt.Sprintf("checks: check type %q is already registered", name))
	}
	checkTypes.byName[name] = run
}

// SupportedCheckTypes returns the registered check types in sorted order.
func SupportedCheckTypes() []string {
	checkTypes.RLock()
	types := make([]string, 0, len(checkTypes.byName))
	for name := range checkTypes.byName {
		types = append(types, name)
	}
	checkTypes.RUnlock()
	sort.Strings(types)
	return types
}

func lookupCheckType(name string) (CheckFunc, bool) {
	checkTypes.RLock()
	run, ok := checkTypes.byName[normalizeCheckType(name)]
	checkTypes.RUnlock()
	return run, ok
}

func normalizeCheckType(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}
