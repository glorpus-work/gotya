# Gotya - Agent Documentation

## Overview
Gotya is a Go package management tool that helps manage Go module dependencies and their metadata. This document provides guidance for LLMs working with the Gotya codebase.

## Key Components

### Core Packages
- `pkg/package/`: Core package management functionality
  - `package.go`: Main package types and interfaces
  - `installed.go`: Handles installed package operations
  - `metadata.go`: Manages package metadata
- `pkg/cache/`: Caching mechanisms
- `pkg/config/`: Configuration management
- `pkg/errors/`: Custom error types and handling

## Common Tasks

### Understanding Package Structure
```go
type Package struct {
    ImportPath string
    Version    string
    // ... other fields
}
```

### Working with Dependencies
- Use `pkg/package` to query and manage Go module dependencies
- The cache system in `pkg/cache` helps optimize dependency resolution

### Error Handling
- All errors are wrapped using the custom error types in `pkg/errors`
- Always check and handle errors appropriately

## Development Guidelines

### Task Runner

The project uses [Task](https://taskfile.dev/) as a task runner to automate common development workflows. Here are the most commonly used tasks:

#### Build Tasks
- `task build` - Build the project
- `task install` - Install the application

#### Testing
- `task test` - Run all tests
- `task test-cover` - Run tests with coverage report
- `task test-cover-html` - Generate HTML coverage report

#### Code Quality
- `task lint` - Run linters
- `task fmt` - Format Go code
- `task tidy` - Tidy Go modules

#### CI/CD
- `task ci` - Run CI pipeline (test, lint, build)

#### Cleanup
- `task clean` - Clean build artifacts

To list all available tasks:
```bash
task --list-all
```

### Adding New Features
1. Add new functionality in the appropriate package
2. Write tests in the corresponding `_test.go` files
3. Update documentation as needed

### Running Tests
You can use either the standard Go command or the task runner:
```bash
task test  # Using task runner
go test ./...  # Directly using Go
```

## Best Practices for LLMs
1. Always check for existing functionality before adding new code
2. Follow Go idioms and the project's coding style
3. Keep functions small and focused
4. Add appropriate documentation for new types and functions
5. Write tests for new functionality

## Common Pitfalls
- Be careful with package version handling
- Ensure proper error handling in all functions
- Be mindful of cross-platform compatibility issues

## Getting Help
- Check the project's README.md for general information
- Review existing issues and pull requests for context
- Consult Go's standard library documentation for reference implementations
