// Package testutils provides utilities for testing Blueprint handlers and components.
// It includes builders for common test objects, fixture loading, and assertion helpers.
package testutils

import (
	"time"

	"github.com/elpic/blueprint/internal/handlers"
	"github.com/elpic/blueprint/internal/parser"
	"github.com/elpic/blueprint/internal/platform"
)

// RuleBuilder provides a fluent interface for building parser.Rule objects in tests.
type RuleBuilder struct {
	rule parser.Rule
}

// NewRule creates a new rule builder with default values.
func NewRule() *RuleBuilder {
	return &RuleBuilder{
		rule: parser.Rule{
			Action: "install", // Default action
			OSList: []string{"mac", "linux", "windows"},
		},
	}
}

// WithAction sets the action and returns the builder for chaining.
func (b *RuleBuilder) WithAction(action string) *RuleBuilder {
	b.rule.Action = action
	return b
}

// WithID sets the rule ID and returns the builder for chaining.
func (b *RuleBuilder) WithID(id string) *RuleBuilder {
	b.rule.ID = id
	return b
}

// WithOS sets the OS list and returns the builder for chaining.
func (b *RuleBuilder) WithOS(osNames ...string) *RuleBuilder {
	b.rule.OSList = osNames
	return b
}

// WithPackage adds a package and returns the builder for chaining.
func (b *RuleBuilder) WithPackage(name string) *RuleBuilder {
	b.rule.Packages = append(b.rule.Packages, parser.Package{
		Name:    name,
		Version: "latest",
	})
	return b
}

// WithPackages adds multiple packages and returns the builder for chaining.
func (b *RuleBuilder) WithPackages(names ...string) *RuleBuilder {
	for _, name := range names {
		b.WithPackage(name)
	}
	return b
}

// WithPackageManager sets the package manager for the last added package.
func (b *RuleBuilder) WithPackageManager(manager string) *RuleBuilder {
	if len(b.rule.Packages) > 0 {
		b.rule.Packages[len(b.rule.Packages)-1].PackageManager = manager
	}
	return b
}

// WithClone sets clone-related fields and returns the builder for chaining.
func (b *RuleBuilder) WithClone(url, path string) *RuleBuilder {
	b.rule.Action = "clone"
	b.rule.CloneURL = url
	b.rule.ClonePath = path
	return b
}

// WithBranch sets the git branch and returns the builder for chaining.
func (b *RuleBuilder) WithBranch(branch string) *RuleBuilder {
	b.rule.Branch = branch
	return b
}

// WithDecrypt sets decrypt-related fields and returns the builder for chaining.
func (b *RuleBuilder) WithDecrypt(sourceFile, destPath string) *RuleBuilder {
	b.rule.Action = "decrypt"
	b.rule.DecryptFile = sourceFile
	b.rule.DecryptPath = destPath
	return b
}

// WithMkdir sets the mkdir path and returns the builder for chaining.
func (b *RuleBuilder) WithMkdir(path string) *RuleBuilder {
	b.rule.Action = "mkdir"
	b.rule.Mkdir = path
	return b
}

// WithRun sets the run command and returns the builder for chaining.
func (b *RuleBuilder) WithRun(command string) *RuleBuilder {
	b.rule.Action = "run"
	b.rule.RunCommand = command
	return b
}

// WithRunSh sets the run-sh URL and returns the builder for chaining.
func (b *RuleBuilder) WithRunSh(url string) *RuleBuilder {
	b.rule.Action = "run-sh"
	b.rule.RunShURL = url
	return b
}

// Build returns the constructed Rule.
func (b *RuleBuilder) Build() parser.Rule {
	return b.rule
}

// StatusBuilder provides a fluent interface for building handlers.Status objects in tests.
type StatusBuilder struct {
	status handlers.Status
}

// NewStatus creates a new status builder.
func NewStatus() *StatusBuilder {
	return &StatusBuilder{
		status: handlers.Status{},
	}
}

// WithPackage adds a package status and returns the builder for chaining.
func (b *StatusBuilder) WithPackage(name, blueprint, os string) *StatusBuilder {
	b.status.Packages = append(b.status.Packages, handlers.PackageStatus{
		Name:        name,
		InstalledAt: time.Now().Format(time.RFC3339),
		Blueprint:   blueprint,
		OS:          os,
	})
	return b
}

// WithClone adds a clone status and returns the builder for chaining.
func (b *StatusBuilder) WithClone(url, path, sha, blueprint, os string) *StatusBuilder {
	b.status.Clones = append(b.status.Clones, handlers.CloneStatus{
		URL:       url,
		Path:      path,
		SHA:       sha,
		ClonedAt:  time.Now().Format(time.RFC3339),
		Blueprint: blueprint,
		OS:        os,
	})
	return b
}

// WithDecrypt adds a decrypt status and returns the builder for chaining.
func (b *StatusBuilder) WithDecrypt(sourceFile, destPath, blueprint, os string) *StatusBuilder {
	b.status.Decrypts = append(b.status.Decrypts, handlers.DecryptStatus{
		SourceFile:  sourceFile,
		DestPath:    destPath,
		DecryptedAt: time.Now().Format(time.RFC3339),
		Blueprint:   blueprint,
		OS:          os,
	})
	return b
}

// WithMkdir adds a mkdir status and returns the builder for chaining.
func (b *StatusBuilder) WithMkdir(path, blueprint, os string) *StatusBuilder {
	b.status.Mkdirs = append(b.status.Mkdirs, handlers.MkdirStatus{
		Path:      path,
		CreatedAt: time.Now().Format(time.RFC3339),
		Blueprint: blueprint,
		OS:        os,
	})
	return b
}

// Build returns the constructed Status.
func (b *StatusBuilder) Build() handlers.Status {
	return b.status
}

// ExecutionRecordBuilder provides a fluent interface for building ExecutionRecord objects in tests.
type ExecutionRecordBuilder struct {
	record handlers.ExecutionRecord
}

// NewExecutionRecord creates a new execution record builder with default values.
func NewExecutionRecord() *ExecutionRecordBuilder {
	return &ExecutionRecordBuilder{
		record: handlers.ExecutionRecord{
			Timestamp: time.Now().Format(time.RFC3339),
			Status:    "success",
		},
	}
}

// WithCommand sets the command and returns the builder for chaining.
func (b *ExecutionRecordBuilder) WithCommand(command string) *ExecutionRecordBuilder {
	b.record.Command = command
	return b
}

// WithOutput sets the output and returns the builder for chaining.
func (b *ExecutionRecordBuilder) WithOutput(output string) *ExecutionRecordBuilder {
	b.record.Output = output
	return b
}

// WithStatus sets the status and returns the builder for chaining.
func (b *ExecutionRecordBuilder) WithStatus(status string) *ExecutionRecordBuilder {
	b.record.Status = status
	return b
}

// WithError sets the error and returns the builder for chaining.
func (b *ExecutionRecordBuilder) WithError(errorMsg string) *ExecutionRecordBuilder {
	b.record.Error = errorMsg
	return b
}

// WithBlueprint sets the blueprint and returns the builder for chaining.
func (b *ExecutionRecordBuilder) WithBlueprint(blueprint string) *ExecutionRecordBuilder {
	b.record.Blueprint = blueprint
	return b
}

// WithOS sets the OS and returns the builder for chaining.
func (b *ExecutionRecordBuilder) WithOS(os string) *ExecutionRecordBuilder {
	b.record.OS = os
	return b
}

// AsSuccess configures the record as successful and returns the builder for chaining.
func (b *ExecutionRecordBuilder) AsSuccess() *ExecutionRecordBuilder {
	b.record.Status = "success"
	return b
}

// AsError configures the record as an error and returns the builder for chaining.
func (b *ExecutionRecordBuilder) AsError(errorMsg string) *ExecutionRecordBuilder {
	b.record.Status = "error"
	b.record.Error = errorMsg
	return b
}

// Build returns the constructed ExecutionRecord.
func (b *ExecutionRecordBuilder) Build() handlers.ExecutionRecord {
	return b.record
}

// ExecuteResultBuilder provides a fluent interface for building platform.ExecuteResult objects in tests.
type ExecuteResultBuilder struct {
	result platform.ExecuteResult
}

// NewExecuteResult creates a new execute result builder with default values.
func NewExecuteResult() *ExecuteResultBuilder {
	return &ExecuteResultBuilder{
		result: platform.ExecuteResult{
			ExitCode: 0,
			Stdout:   "",
			Stderr:   "",
			Duration: time.Millisecond,
			Success:  true,
		},
	}
}

// WithExitCode sets the exit code and returns the builder for chaining.
func (b *ExecuteResultBuilder) WithExitCode(code int) *ExecuteResultBuilder {
	b.result.ExitCode = code
	b.result.Success = code == 0
	return b
}

// WithStdout sets the stdout and returns the builder for chaining.
func (b *ExecuteResultBuilder) WithStdout(stdout string) *ExecuteResultBuilder {
	b.result.Stdout = stdout
	return b
}

// WithStderr sets the stderr and returns the builder for chaining.
func (b *ExecuteResultBuilder) WithStderr(stderr string) *ExecuteResultBuilder {
	b.result.Stderr = stderr
	return b
}

// WithDuration sets the duration and returns the builder for chaining.
func (b *ExecuteResultBuilder) WithDuration(duration time.Duration) *ExecuteResultBuilder {
	b.result.Duration = duration
	return b
}

// AsSuccess configures the result as successful and returns the builder for chaining.
func (b *ExecuteResultBuilder) AsSuccess() *ExecuteResultBuilder {
	b.result.ExitCode = 0
	b.result.Success = true
	return b
}

// AsError configures the result as an error and returns the builder for chaining.
func (b *ExecuteResultBuilder) AsError(exitCode int, stderr string) *ExecuteResultBuilder {
	b.result.ExitCode = exitCode
	b.result.Stderr = stderr
	b.result.Success = false
	return b
}

// Build returns the constructed ExecuteResult.
func (b *ExecuteResultBuilder) Build() *platform.ExecuteResult {
	return &b.result
}

// Common test scenarios as convenience functions

// SuccessfulInstall creates a successful package install execution record.
func SuccessfulInstall(packageName, packageManager, os string) handlers.ExecutionRecord {
	var command string
	switch packageManager {
	case "brew":
		command = "brew install " + packageName
	case "apt":
		command = "sudo apt-get install -y " + packageName
	case "snap":
		command = "sudo snap install " + packageName
	default:
		command = packageManager + " install " + packageName
	}

	return NewExecutionRecord().
		WithCommand(command).
		WithOutput("Successfully installed " + packageName).
		WithOS(os).
		AsSuccess().
		Build()
}

// SuccessfulClone creates a successful clone execution record.
func SuccessfulClone(url, path, sha string) handlers.ExecutionRecord {
	command := "git clone " + url + " " + path
	output := "Cloned (SHA: " + sha + ")"

	return NewExecutionRecord().
		WithCommand(command).
		WithOutput(output).
		AsSuccess().
		Build()
}

// FailedCommand creates a failed command execution record.
func FailedCommand(command, errorMsg string) handlers.ExecutionRecord {
	return NewExecutionRecord().
		WithCommand(command).
		WithError(errorMsg).
		AsError(errorMsg).
		Build()
}
