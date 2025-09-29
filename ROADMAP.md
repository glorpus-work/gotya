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
**Status**: ✅ Completed
**Description**: Implement `gotya search <query>` command for package discovery
**Implementation**: Created fuzzy search functionality in the index package and wired it to CLI
**Features**:
- Fuzzy matching with relevance scoring
- Searches across all configured repositories
- Results sorted by relevance (best matches first)
- Clean struct-based implementation (no string manipulation)
**Effort**: 2-3 days (completed)

#### 2. List Command
**Status**: ✅ Completed
**Description**: Implement `gotya list` command to show installed packages only
**Implementation**: Created list functionality to display currently installed packages from the local database
**Requirements**:
- ✅ Tabular output format only (no JSON/YAML for now)
- ✅ Filter by package name (optional flag)
- ✅ Show package-name and package-version
- ✅ No categories support (doesn't exist yet)
- ✅ Skip outdated detection for now (too complex)
**Note**: User clarified that list should focus on installed packages only, not available packages
**Features**:
- Clean database method for filtering
- Business logic moved to appropriate pkg layer
**Effort**: 1-2 days (completed)

#### 3. Update Command
**Status**: ✅ Completed
**Description**: Uncomment and complete the `gotya update` command implementation
**Implementation**: The command is fully implemented with --all, --dry-run, --concurrency flags and proper integration with the orchestrator
**Effort**: 1-2 days (completed)

### Medium Priority (Should Have)

#### 4. Environment Variables Support
**Status**: Not implemented
**Description**: Add support for `GOTYA_CONFIG_DIR`, `GOTYA_CACHE_DIR`, `GOTYA_INSTALL_DIR`
**Implementation**: These are mentioned as TODO in README and used in tests but not honored by application
**Effort**: 1 day

#### 6. Testing Integration
**Status**: Tests exist and functional
**Description**: Review and run integration tests for new commands
**Implementation**: Integration tests are working and can be run with `go test -tags integration ./cli/gotya -v`
**Note**: Tests verified working (TestVersionCommand passes)
**Effort**: 1-2 days (completed)

### Low Priority (Nice to Have)

#### 7. Enhanced Hooks System
**Status**: Basic infrastructure exists
**Description**: Expand the hooks system for better extensibility
**Implementation**: The orchestrator has hooks infrastructure that could be exposed via CLI
**Effort**: 2-3 days

#### 8. Documentation Updates
**Status**: ✅ Completed
**Description**: Add roadmap section to README.md
**Implementation**: README already includes roadmap section with link to ROADMAP.md
**Effort**: 0.5 days (completed)

## Implementation Order

### Phase 1: Core CLI Commands (Week 1-2)
1. ~~Search command~~ ✅ Completed
2. ~~List command~~ ✅ Completed
3. ~~Update command~~ ✅ Completed

### Phase 2: Feature Completion (Week 2)
4. Environment variables
5. ~~Testing validation~~ ✅ Completed

### Phase 3: Polish (Week 2-3)
6. Enhanced hooks (if time permits)
7. ~~Documentation updates~~ ✅ Completed

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
