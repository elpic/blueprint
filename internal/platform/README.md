# Blueprint Platform Abstraction Layer

This package provides the foundational testing infrastructure for Blueprint, enabling fast, reliable unit tests by separating I/O and system dependencies from business logic.

## Architecture Overview

The platform abstraction layer follows a dependency injection pattern with clearly defined interfaces for all external dependencies:

```
                           ┌─────────────────┐
                           │   Handlers      │
                           │ (Business Logic)│
                           └─────────────────┘
                                    │
                           ┌─────────────────┐
                           │    Container    │
                           │ (Dependency     │
                           │  Injection)     │
                           └─────────────────┘
                                    │
                    ┌───────────────┼───────────────┐
                    │               │               │
            ┌───────────────┐ ┌───────────────┐ ┌───────────────┐
            │ SystemProvider│ │  GitProvider  │ │CryptoProvider │
            │               │ │               │ │               │
            └───────────────┘ └───────────────┘ └───────────────┘
                    │
            ┌───────┼───────┐
            │       │       │
    ┌─────────┐ ┌─────────┐ ┌─────────┐
    │OSDetector│ │Process  │ │Filesystem│
    │         │ │Executor │ │Provider │
    └─────────┘ └─────────┘ └─────────┘
```

## Key Components

### Interfaces (`interfaces.go`)

- **SystemProvider**: Main interface combining all platform operations
- **OSDetector**: Operating system detection capabilities  
- **ProcessExecutor**: Command execution and process management
- **FilesystemProvider**: File and directory operations
- **NetworkProvider**: HTTP requests and network operations
- **GitProvider**: Git-specific operations
- **CryptoProvider**: Encryption/decryption operations

### Container (`container.go`)

- **Container**: Dependency injection container interface
- **NewContainer()**: Creates production container with real implementations
- **NewTestContainer()**: Creates test container for mock injection
- **TestContainer**: Fluent interface for test configuration

### Mocks (`mocks/`)

All mocks provide fluent interfaces for easy test configuration:

- **MockSystemProvider**: Complete system mock with fluent API
- **MockOSDetector**: OS detection mocking
- **MockProcessExecutor**: Command execution mocking
- **MockFilesystemProvider**: In-memory filesystem
- **MockGitProvider**: Git operations mocking

Example usage:
```go
systemProvider := mocks.NewMockSystemProvider().
    WithOS("linux").
    WithUser("testuser", "1000", "1000", "/home/testuser").
    WithCommandResult("git clone ...", &platform.ExecuteResult{
        ExitCode: 0,
        Stdout:   "Cloned successfully",
        Success:  true,
    }).
    WithFile("/home/testuser/.gitconfig", []byte("git config"))
```

### Test Utilities (`testutils/`)

#### Builders (`builders.go`)

Fluent builders for common test objects:

- **RuleBuilder**: Creates `parser.Rule` objects
- **StatusBuilder**: Creates `handlers.Status` objects  
- **ExecutionRecordBuilder**: Creates `handlers.ExecutionRecord` objects
- **ExecuteResultBuilder**: Creates `platform.ExecuteResult` objects

Example:
```go
rule := testutils.NewRule().
    WithAction("install").
    WithPackages("git", "curl").
    WithOS("mac").
    Build()

status := testutils.NewStatus().
    WithPackage("git", "/tmp/test.bp", "mac").
    WithClone("https://github.com/user/repo", "~/repo", "abc123", "/tmp/test.bp", "mac").
    Build()
```

#### Assertions (`assertions.go`)

Fluent assertion helpers:

- **AssertExecuteResult**: Assertions for command execution results
- **AssertStatus**: Assertions for status objects  
- **AssertString**: String assertions
- **AssertError**: Error assertions

Example:
```go
testutils.NewAssertExecuteResult(t, result, "git clone").
    IsSuccess().
    HasStdout("Cloned").
    HasExitCode(0)

testutils.NewAssertStatus(t, &status, "status").
    HasPackageCount(2).
    HasPackage("git", "/tmp/test.bp", "mac").
    DoesNotHaveClone("https://other.com/repo", "~/other", "/tmp/test.bp", "mac")
```

## Testing Strategy

### Pure Function Tests

Test business logic without any I/O or external dependencies:

```go
func TestHandler_GetCommand_Pure(t *testing.T) {
    rule := testutils.NewRule().WithPackage("curl").Build()
    handler := handlers.NewInstallHandler(rule, "")
    
    cmd := handler.GetCommand()
    
    testutils.AssertStringEquals(t, cmd, "brew install curl", "command")
}
```

**Performance**: <100ns per test

### Mocked I/O Tests

Test business logic with controlled external dependencies:

```go
func TestHandler_Up_WithMocks(t *testing.T) {
    system := mocks.NewMockSystemProvider().
        WithOS("linux").
        WithCommandResult("apt-get install curl", successResult)
    
    container := platform.NewTestContainer().
        WithSystemProvider(system).
        Build()
    
    handler := handlers.NewInstallHandlerWithContainer(rule, "", container)
    
    result, err := handler.Up()
    
    testutils.AssertNoError(t, err, "Up")
    testutils.AssertStringContains(t, result, "installed", "result")
}
```

**Performance**: <1ms per test

### Integration Tests

Test actual system interactions (kept in `internal/handlers/integration/`):

```go
func TestHandler_Up_Integration(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
    }
    
    // Test with real system dependencies
    handler := handlers.NewInstallHandler(rule, "")
    result, err := handler.Up()
    
    // Assertions...
}
```

## Current Status

### ✅ Completed (Phase 1)

- [x] Platform abstraction interfaces
- [x] Dependency injection container
- [x] Mock infrastructure with fluent APIs
- [x] Test package structure
- [x] Test utilities and builders
- [x] Example unit tests demonstrating new patterns
- [x] Performance optimization (<1ms per unit test)

### 🚧 Next Steps (Phase 2)

- [ ] Refactor handlers to use platform abstractions
- [ ] Implement real platform providers
- [ ] Convert existing integration tests to unit tests
- [ ] Add dependency injection to handler constructors
- [ ] Update engine to use container

### 📊 Performance Metrics

- **Pure function tests**: ~93ns per operation
- **Mocked I/O tests**: <1ms per test
- **Zero external dependencies** in unit tests
- **Fast feedback** for developers

## Usage Examples

### Setting Up Tests

```go
func TestMyHandler(t *testing.T) {
    // Create test data
    rule := testutils.NewRule().WithPackage("git").Build()
    
    // Create mocked dependencies
    system := mocks.NewMockSystemProvider().
        WithOS("mac").
        WithCommandResult("brew install git", successResult)
    
    // Create container with mocks
    container := platform.NewTestContainer().
        WithSystemProvider(system).
        Build()
    
    // Create handler with dependencies
    handler := NewMyHandlerWithContainer(rule, container)
    
    // Test business logic
    result, err := handler.Up()
    
    // Assertions
    testutils.AssertNoError(t, err, "Up")
}
```

### Common Test Patterns

1. **Pure Business Logic**: Test functions that don't require I/O
2. **Mocked Dependencies**: Test with controlled external dependencies  
3. **Integration**: Test actual system interactions (sparingly)

### Fluent Mocking

```go
// Git operations
git := mocks.NewMockGitProvider().
    WithSuccessfulClone(url, path, branch, sha).
    WithUpToDateRepository(path, branch, sha).
    WithUnreachableRemote(url, branch)

// Filesystem operations  
fs := mocks.NewMockFilesystemProvider().
    WithFile("/tmp/test.txt", []byte("content")).
    WithDirectory("/tmp").
    WithPermissions("/tmp/test.txt", 0644)

// Command execution
proc := mocks.NewMockProcessExecutor().
    WithCommandResult("git clone ...", successResult).
    WithCommandError("invalid-command", errors.New("not found"))
```

This foundation enables fast, reliable testing while maintaining clear separation between business logic and external dependencies.