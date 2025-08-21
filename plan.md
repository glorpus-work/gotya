# Code Restructuring Plan

## Objective
Refactor the `internal/cli` package to move reusable components to appropriate packages under `pkg/` for better code organization and reusability.

## Phase 1: Configuration Management
1. **Move Configuration Helpers**
   - Move `createDefaultConfig()` from `internal/cli/config.go` to `pkg/config/config.go`
   - Move `setConfigValue()` and `getConfigValue()` to `pkg/config/helpers.go`
   - Update imports and references

## Phase 2: Package Installation Logic
1. **Create New Package**
   - Create `pkg/installer/installer.go`
   - Move core installation logic from `internal/cli/install.go`
   - Include functions:
     - `InstallPackage()`
     - `UpdatePackage()`
     - `InstallSinglePackage()`
     - `UpdateSinglePackage()`

## Phase 3: Repository Management
1. **Move Repository Logic**
   - Move non-CLI repository management code from `internal/cli/repo.go` to `pkg/repository/`
   - Ensure clean separation between CLI commands and core logic

## Phase 4: Cache Management
1. **Consolidate Cache Code**
   - Move cache-related code from `internal/cli/cache.go` to `pkg/cache/`
   - Update CLI commands to use the consolidated cache package

## Phase 5: Helper Functions
1. **Move Utilities**
   - Create `pkg/app/initializer.go`
   - Move `loadConfigAndManager()` to this package
   - Update references in CLI commands

## Phase 6: Testing
1. **Update Tests**
   - Move and update tests for the moved functions
   - Ensure all tests pass after refactoring

## Phase 7: Cleanup
1. **Remove Duplicate Code**
   - Remove any remaining duplicate code
   - Update documentation
   - Verify all imports are correct

## Dependencies
- Update all import paths to reflect the new structure
- Ensure all moved code maintains its functionality
- Update any dependent code

## Verification
1. Run all tests to ensure nothing is broken
2. Verify CLI commands still work as expected
3. Check for any circular dependencies

## Timeline
1. Phase 1-3: Day 1
2. Phase 4-5: Day 2
3. Phase 6-7: Day 3

## Risks
- Breaking changes to the public API
- Potential circular dependencies
- Need to update documentation and examples
