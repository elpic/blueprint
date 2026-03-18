package testutils

import (
	"strings"
	"testing"

	"github.com/elpic/blueprint/internal/handlers"
	"github.com/elpic/blueprint/internal/platform"
)

// AssertExecuteResult provides fluent assertions for platform.ExecuteResult.
type AssertExecuteResult struct {
	t      *testing.T
	result *platform.ExecuteResult
	name   string
}

// NewAssertExecuteResult creates a new execute result assertion helper.
func NewAssertExecuteResult(t *testing.T, result *platform.ExecuteResult, name string) *AssertExecuteResult {
	return &AssertExecuteResult{
		t:      t,
		result: result,
		name:   name,
	}
}

// IsSuccess asserts that the execute result indicates success.
func (a *AssertExecuteResult) IsSuccess() *AssertExecuteResult {
	if !a.result.Success {
		a.t.Errorf("%s: expected success, got failure (exit code: %d)", a.name, a.result.ExitCode)
	}
	if a.result.ExitCode != 0 {
		a.t.Errorf("%s: expected exit code 0, got %d", a.name, a.result.ExitCode)
	}
	return a
}

// IsError asserts that the execute result indicates an error.
func (a *AssertExecuteResult) IsError() *AssertExecuteResult {
	if a.result.Success {
		a.t.Errorf("%s: expected error, got success", a.name)
	}
	if a.result.ExitCode == 0 {
		a.t.Errorf("%s: expected non-zero exit code, got 0", a.name)
	}
	return a
}

// HasExitCode asserts that the execute result has the specified exit code.
func (a *AssertExecuteResult) HasExitCode(expected int) *AssertExecuteResult {
	if a.result.ExitCode != expected {
		a.t.Errorf("%s: expected exit code %d, got %d", a.name, expected, a.result.ExitCode)
	}
	return a
}

// HasStdout asserts that the stdout contains the expected text.
func (a *AssertExecuteResult) HasStdout(expected string) *AssertExecuteResult {
	if !strings.Contains(a.result.Stdout, expected) {
		a.t.Errorf("%s: expected stdout to contain %q, got %q", a.name, expected, a.result.Stdout)
	}
	return a
}

// HasStderr asserts that the stderr contains the expected text.
func (a *AssertExecuteResult) HasStderr(expected string) *AssertExecuteResult {
	if !strings.Contains(a.result.Stderr, expected) {
		a.t.Errorf("%s: expected stderr to contain %q, got %q", a.name, expected, a.result.Stderr)
	}
	return a
}

// AssertStatus provides fluent assertions for handlers.Status.
type AssertStatus struct {
	t      *testing.T
	status *handlers.Status
	name   string
}

// NewAssertStatus creates a new status assertion helper.
func NewAssertStatus(t *testing.T, status *handlers.Status, name string) *AssertStatus {
	return &AssertStatus{
		t:      t,
		status: status,
		name:   name,
	}
}

// HasPackageCount asserts that the status has the expected number of packages.
func (a *AssertStatus) HasPackageCount(expected int) *AssertStatus {
	actual := len(a.status.Packages)
	if actual != expected {
		a.t.Errorf("%s: expected %d packages, got %d", a.name, expected, actual)
	}
	return a
}

// HasPackage asserts that the status contains a specific package.
func (a *AssertStatus) HasPackage(packageName, blueprint, os string) *AssertStatus {
	for _, pkg := range a.status.Packages {
		if pkg.Name == packageName && pkg.Blueprint == blueprint && pkg.OS == os {
			return a // Found the package
		}
	}
	a.t.Errorf("%s: expected package %q for blueprint %q and OS %q not found",
		a.name, packageName, blueprint, os)
	return a
}

// DoesNotHavePackage asserts that the status does not contain a specific package.
func (a *AssertStatus) DoesNotHavePackage(packageName, blueprint, os string) *AssertStatus {
	for _, pkg := range a.status.Packages {
		if pkg.Name == packageName && pkg.Blueprint == blueprint && pkg.OS == os {
			a.t.Errorf("%s: unexpected package %q for blueprint %q and OS %q found",
				a.name, packageName, blueprint, os)
			return a
		}
	}
	return a // Package not found, which is expected
}

// HasCloneCount asserts that the status has the expected number of clones.
func (a *AssertStatus) HasCloneCount(expected int) *AssertStatus {
	actual := len(a.status.Clones)
	if actual != expected {
		a.t.Errorf("%s: expected %d clones, got %d", a.name, expected, actual)
	}
	return a
}

// HasClone asserts that the status contains a specific clone.
func (a *AssertStatus) HasClone(url, path, blueprint, os string) *AssertStatus {
	for _, clone := range a.status.Clones {
		if clone.URL == url && clone.Path == path && clone.Blueprint == blueprint && clone.OS == os {
			return a // Found the clone
		}
	}
	a.t.Errorf("%s: expected clone %q at %q for blueprint %q and OS %q not found",
		a.name, url, path, blueprint, os)
	return a
}

// DoesNotHaveClone asserts that the status does not contain a specific clone.
func (a *AssertStatus) DoesNotHaveClone(url, path, blueprint, os string) *AssertStatus {
	for _, clone := range a.status.Clones {
		if clone.URL == url && clone.Path == path && clone.Blueprint == blueprint && clone.OS == os {
			a.t.Errorf("%s: unexpected clone %q at %q for blueprint %q and OS %q found",
				a.name, url, path, blueprint, os)
			return a
		}
	}
	return a // Clone not found, which is expected
}

// AssertString provides fluent assertions for string values.
type AssertString struct {
	t     *testing.T
	value string
	name  string
}

// NewAssertString creates a new string assertion helper.
func NewAssertString(t *testing.T, value, name string) *AssertString {
	return &AssertString{
		t:     t,
		value: value,
		name:  name,
	}
}

// Equals asserts that the string equals the expected value.
func (a *AssertString) Equals(expected string) *AssertString {
	if a.value != expected {
		a.t.Errorf("%s: expected %q, got %q", a.name, expected, a.value)
	}
	return a
}

// Contains asserts that the string contains the expected substring.
func (a *AssertString) Contains(expected string) *AssertString {
	if !strings.Contains(a.value, expected) {
		a.t.Errorf("%s: expected to contain %q, got %q", a.name, expected, a.value)
	}
	return a
}

// DoesNotContain asserts that the string does not contain the expected substring.
func (a *AssertString) DoesNotContain(unexpected string) *AssertString {
	if strings.Contains(a.value, unexpected) {
		a.t.Errorf("%s: expected not to contain %q, but got %q", a.name, unexpected, a.value)
	}
	return a
}

// IsEmpty asserts that the string is empty.
func (a *AssertString) IsEmpty() *AssertString {
	if a.value != "" {
		a.t.Errorf("%s: expected empty string, got %q", a.name, a.value)
	}
	return a
}

// IsNotEmpty asserts that the string is not empty.
func (a *AssertString) IsNotEmpty() *AssertString {
	if a.value == "" {
		a.t.Errorf("%s: expected non-empty string, got empty", a.name)
	}
	return a
}

// AssertError provides fluent assertions for error values.
type AssertError struct {
	t    *testing.T
	err  error
	name string
}

// NewAssertError creates a new error assertion helper.
func NewAssertError(t *testing.T, err error, name string) *AssertError {
	return &AssertError{
		t:    t,
		err:  err,
		name: name,
	}
}

// IsNil asserts that the error is nil.
func (a *AssertError) IsNil() *AssertError {
	if a.err != nil {
		a.t.Errorf("%s: expected no error, got %v", a.name, a.err)
	}
	return a
}

// IsNotNil asserts that the error is not nil.
func (a *AssertError) IsNotNil() *AssertError {
	if a.err == nil {
		a.t.Errorf("%s: expected error, got nil", a.name)
	}
	return a
}

// HasMessage asserts that the error message contains the expected text.
func (a *AssertError) HasMessage(expected string) *AssertError {
	if a.err == nil {
		a.t.Errorf("%s: expected error with message %q, got nil", a.name, expected)
		return a
	}
	if !strings.Contains(a.err.Error(), expected) {
		a.t.Errorf("%s: expected error message to contain %q, got %q", a.name, expected, a.err.Error())
	}
	return a
}

// Common assertion functions

// AssertExecuteResultSuccess is a convenience function for asserting successful execute results.
func AssertExecuteResultSuccess(t *testing.T, result *platform.ExecuteResult, name string) {
	NewAssertExecuteResult(t, result, name).IsSuccess()
}

// AssertExecuteResultError is a convenience function for asserting failed execute results.
func AssertExecuteResultError(t *testing.T, result *platform.ExecuteResult, name string) {
	NewAssertExecuteResult(t, result, name).IsError()
}

// AssertNoError is a convenience function for asserting no error.
func AssertNoError(t *testing.T, err error, name string) {
	NewAssertError(t, err, name).IsNil()
}

// AssertHasError is a convenience function for asserting an error exists.
func AssertHasError(t *testing.T, err error, name string) {
	NewAssertError(t, err, name).IsNotNil()
}

// AssertStringEquals is a convenience function for asserting string equality.
func AssertStringEquals(t *testing.T, actual, expected, name string) {
	NewAssertString(t, actual, name).Equals(expected)
}

// AssertStringContains is a convenience function for asserting string containment.
func AssertStringContains(t *testing.T, actual, expected, name string) {
	NewAssertString(t, actual, name).Contains(expected)
}
