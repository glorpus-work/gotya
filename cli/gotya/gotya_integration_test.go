//go:build integration

package main

// This file has been refactored to split integration tests into focused files:
// - sync_integration_test.go: Repository synchronization tests
// - artifact_integration_test.go: Artifact creation and verification tests
// - index_integration_test.go: Index generation tests
// - cli_integration_test.go: CLI command tests (version, help, config)
// - integration_helpers_test.go: Shared helper functions
//
// All tests use the integration build tag and can be run with:
//   go test -tags integration ./cli/gotya -v
//
// This organization follows Go community best practices for organizing
// integration tests alongside the main binary while maintaining clear
// separation of concerns.
