package mocks

import (
	"fmt"
	"strings"
)

// MockCommandExecutor provides a mock implementation for testing command execution
// without actually running commands on the system
type MockCommandExecutor struct {
	// CommandResults maps commands to their expected results
	CommandResults map[string]CommandResult

	// DefaultResult is returned when a command is not found in CommandResults
	DefaultResult CommandResult

	// ExecutedCommands records all commands that were executed (for verification)
	ExecutedCommands []string
}

// CommandResult represents the result of executing a command
type CommandResult struct {
	Output string
	Error  error
}

// NewMockCommandExecutor creates a new mock command executor with sensible defaults
func NewMockCommandExecutor() *MockCommandExecutor {
	return &MockCommandExecutor{
		CommandResults: make(map[string]CommandResult),
		DefaultResult: CommandResult{
			Output: "mock success",
			Error:  nil,
		},
		ExecutedCommands: []string{},
	}
}

// Execute executes a mock command and returns the configured result
func (m *MockCommandExecutor) Execute(cmd string) (string, error) {
	// Record the command execution
	m.ExecutedCommands = append(m.ExecutedCommands, cmd)

	// Check for exact command match first
	if result, exists := m.CommandResults[cmd]; exists {
		return result.Output, result.Error
	}

	// Check for partial matches (useful for commands with dynamic parts)
	for pattern, result := range m.CommandResults {
		if strings.Contains(cmd, pattern) {
			return result.Output, result.Error
		}
	}

	// Return default result
	return m.DefaultResult.Output, m.DefaultResult.Error
}

// Fluent configuration methods for easy test setup

// WithCommandResult configures a specific command result and returns the mock for chaining
func (m *MockCommandExecutor) WithCommandResult(cmd string, output string, err error) *MockCommandExecutor {
	m.CommandResults[cmd] = CommandResult{Output: output, Error: err}
	return m
}

// WithCommandSuccess configures a command to succeed with given output
func (m *MockCommandExecutor) WithCommandSuccess(cmd string, output string) *MockCommandExecutor {
	return m.WithCommandResult(cmd, output, nil)
}

// WithCommandError configures a command to fail with given error
func (m *MockCommandExecutor) WithCommandError(cmd string, errorMsg string) *MockCommandExecutor {
	return m.WithCommandResult(cmd, "", fmt.Errorf("%s", errorMsg))
}

// WithDefaultSuccess configures the default result to be successful
func (m *MockCommandExecutor) WithDefaultSuccess(output string) *MockCommandExecutor {
	m.DefaultResult = CommandResult{Output: output, Error: nil}
	return m
}

// WithDefaultError configures the default result to be an error
func (m *MockCommandExecutor) WithDefaultError(errorMsg string) *MockCommandExecutor {
	m.DefaultResult = CommandResult{Output: "", Error: fmt.Errorf("%s", errorMsg)}
	return m
}

// Clear resets the mock to initial state
func (m *MockCommandExecutor) Clear() *MockCommandExecutor {
	m.CommandResults = make(map[string]CommandResult)
	m.ExecutedCommands = []string{}
	m.DefaultResult = CommandResult{Output: "mock success", Error: nil}
	return m
}

// GetExecutedCommands returns a copy of all executed commands (for test verification)
func (m *MockCommandExecutor) GetExecutedCommands() []string {
	result := make([]string, len(m.ExecutedCommands))
	copy(result, m.ExecutedCommands)
	return result
}

// WasCommandExecuted checks if a specific command was executed
func (m *MockCommandExecutor) WasCommandExecuted(cmd string) bool {
	for _, executed := range m.ExecutedCommands {
		if executed == cmd {
			return true
		}
	}
	return false
}

// WasCommandExecutedMatching checks if a command containing the pattern was executed
func (m *MockCommandExecutor) WasCommandExecutedMatching(pattern string) bool {
	for _, executed := range m.ExecutedCommands {
		if strings.Contains(executed, pattern) {
			return true
		}
	}
	return false
}
