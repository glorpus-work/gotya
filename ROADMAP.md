# Gotya 1.0 Roadmap

This document outlines the roadmap to reach version 1.0 of gotya, a lightweight personal artifact/package manager.

## Current Status (v0.x)

Gotya has a solid foundation with:
- ✅ Core architecture implemented (orchestrator, artifact management, index handling)
- ✅ Basic CLI commands: sync, install, uninstall, config, cache, version, artifact, index
- ✅ Comprehensive testing (unit and integration tests)
- ✅ Archive operations and structured logging
- ✅ Configuration management and dependency resolution

## 1.0 Release Goals

### High Priority (Must Have)

#### 1. Search Command
**Status**: Not implemented
**Description**: Implement `gotya search <query>` command for package discovery
**Implementation**: Create search functionality in the index package and wire it to CLI
**Effort**: 2-3 days

#### 2. List Command
**Status**: Not implemented
**Description**: Implement `gotya list` command to show installed packages only
**Implementation**: Create list functionality to display currently installed packages from the local database
**Note**: User clarified that list should focus on installed packages only, not available packages
**Effort**: 1-2 days

#### 3. Update Command
**Status**: Code exists but commented out
**Description**: Uncomment and complete the `gotya update` command implementation
**Implementation**: The command structure exists but needs to be enabled and tested
**Effort**: 1-2 days

### Medium Priority (Should Have)

#### 4. Skip Dependencies Feature
**Status**: Partially implemented
**Description**: Complete the `--skip-deps` functionality in install command
**Implementation**: The flag is accepted but has no effect - needs actual implementation
**Effort**: 1-2 days

#### 5. Environment Variables Support
**Status**: Not implemented
**Description**: Add support for `GOTYA_CONFIG_DIR`, `GOTYA_CACHE_DIR`, `GOTYA_INSTALL_DIR`
**Implementation**: These are mentioned as TODO in README and used in tests but not honored by application
**Effort**: 1 day

#### 6. Testing Integration
**Status**: Tests exist but need validation
**Description**: Review and run integration tests for new commands
**Implementation**: Ensure all new CLI commands work with existing integration test framework
**Effort**: 1-2 days

### Low Priority (Nice to Have)

#### 7. Enhanced Hooks System
**Status**: Basic infrastructure exists
**Description**: Expand the hooks system for better extensibility
**Implementation**: The orchestrator has hooks infrastructure that could be exposed via CLI
**Effort**: 2-3 days

#### 8. Documentation Updates
**Status**: README is good but could reference roadmap
**Description**: Add roadmap section to README.md
**Implementation**: Update README to include link to this roadmap document
**Effort**: 0.5 days

## Implementation Order

### Phase 1: Core CLI Commands (Week 1-2)
1. Search command
2. List command
3. Update command

### Phase 2: Feature Completion (Week 2)
4. Skip dependencies
5. Environment variables
6. Testing validation

### Phase 3: Polish (Week 2-3)
7. Enhanced hooks (if time permits)
8. Documentation updates

## Success Criteria for 1.0

- ✅ All core CLI commands implemented (sync, search, list, install, uninstall, update, config, cache, version)
- ✅ All major features from README requirements implemented
- ✅ Integration tests pass
- ✅ No critical bugs in core functionality
- ✅ Documentation is complete and accurate

## Post 1.0 Considerations

- Performance optimizations
- Additional package formats support
- Plugin system expansion
- GUI interface
- Cross-platform testing improvements

## Contributing

This roadmap is a living document. Features may be reprioritized based on:
- User feedback and requirements
- Technical debt discovered during implementation
- Community contributions
- Maintenance and security considerations

Last updated: $(date)
Version: 0.9.x → 1.0.0
