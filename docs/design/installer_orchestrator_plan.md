# Installer/Orchestrator Evolution Plan

Purpose: Track the steps to evolve the installation flow with a Download Manager, an Orchestrator, a planning method on the Index Manager, the removal of downloading from the Artifact Manager, and the CLI adaptation.

Legend for progress marks in this document
- ✓ = completed in current session
- * = in progress in current session
- ! = attempted but failed in current session
- (no mark) = not started in current session

Last updated: 2025-09-23 15:00

---

1. Download Manager foundation ✓
   - Define pkg/download interfaces: Manager, Item, Options ✓
   - Implement ManagerImpl: concurrent downloads, de-dup by URL, atomic writes, secure perms, optional SHA-256 verification ✓
   - Unit tests for basic behaviors (success path, checksum, dedupe) ✓
   - Decide cache directory strategy (absolute dir required for now) ✓

2. Index Manager: planning API
   - Add Resolve(ctx, req) → ResolvedGraph (concrete versions, deps)
   - Add Plan(ctx, graph) → InstallPlan (topologically ordered steps, with SourceURL and Checksum) ✓
   - Data types: InstallRequest, ResolvedGraph, InstallPlan, InstallStep ✓
   - Validation: cycle/conflict detection; deterministic ordering
   - Unit tests: simple graphs, cycle/conflict reporting

3. Orchestrator (thin coordinator) ✓
   - New package (e.g., pkg/installer or pkg/orchestrator) with Orchestrator struct ✓
   - Install(ctx, req, hooks) flow:
     - call Index.Resolve → graph
     - call Index.Plan → plan ✓
     - call DownloadManager.FetchAll for plan items ✓
     - execute ArtifactManager.Install per step in dependency order ✓
     - emit progress events; support dry-run by skipping DL/install ✓
   - Minimal rollback (best-effort uninstall of already-installed steps on failure)
   - Tests using fakes for Index, Download, Artifact

4. Artifact Manager refactor (remove downloading) ✓
   - Change Install signature (if needed) to accept local file path/blob + metadata ✓
   - Remove any direct HTTP/download logic from pkg/artifact ✓
   - Ensure installed DB updates remain atomic and idempotent
   - Tests updated to install from local artifacts ✓

5. CLI adaptation
   - Wire `gotya install` to call Orchestrator.Install instead of Artifact Manager directly ✓
   - Add `--dry-run` to print the InstallPlan ✓
   - Render progress events (resolving/downloading/installing/done) ✓
   - Keep uninstall and other commands working; adjust as needed for new installed DB shape

6. HTTP package migration
   - Replace usages of pkg/http for artifacts and index downloads with pkg/download manager where appropriate
   - Optionally provide adapters to ease transition
   - Mark pkg/http as deprecated (or repurpose strictly for index metadata if kept)

7. Cache and configuration
   - Config option for download/cache directory (default under XDG cache path) ✓
   - Ensure `gotya cache clean` knows how to clean download cache (indexes and artifacts)
   - Document cache layout and GC policy (future: size-based LRU)

8. Observability and reliability
   - Integrate internal/logger for structured logs in Orchestrator and Download Manager
   - Add retry/backoff policy for downloads (future enhancement)
   - Provide cancellation via context and ensure graceful unwind

9. Integration tests
   - End-to-end install using a small test repo index (existing test/repo)
   - Verify install order, installed DB contents, and cache reuse on second run

10. Backwards compatibility and migration
   - Maintain CLI flags where possible; provide clear migration notes
   - Keep artifact installation idempotent; re-running should be safe

11. Documentation
   - Update docs/design to reflect new architecture boundaries
   - Provide developer guide for writing resolvers and orchestrator hooks

12. Deliverables checklist
   - New/updated packages: pkg/download ✓, pkg/index (Resolve/Plan), pkg/installer (Orchestrator), pkg/artifact (refactor), internal/cli (install wiring)
   - Tests: unit for each layer, integration end-to-end
   - Docs: this plan and architecture notes

---

Milestone breakdown and acceptance criteria

M1: Planner and Orchestrator skeleton
- IndexManager.Plan exists and returns deterministic InstallPlan for simple graphs
- Orchestrator.Install executes plan sequentially
- DownloadManager.FetchAll invoked with plan items; artifacts installed from local paths
- CLI dry-run prints plan

M2: Robust install
- Parallel downloads, sequential installs in topo order
- Basic rollback on failure
- Installed DB records full provenance

M3: Migration and cleanup
- Artifact Manager free of networking
- CLI uses Orchestrator end-to-end; pkg/http deprecated or isolated
- Cache commands handle download cache

Notes
- Keep Orchestrator thin to limit complexity, most logic in Index (policy) and Artifact (mechanism).
- Prefer small, testable interfaces and use fakes in tests.
