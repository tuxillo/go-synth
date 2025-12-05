# Documentation Refactoring Plan

**Date**: 2025-12-05  
**Status**: Proposed  
**Goal**: Reduce documentation scatter, clearly separate MVP from post-MVP work, improve navigation and maintainability

## Current State Assessment

### Documentation Locations

**Root Directory** (11 files):
- `README.md` - Main project overview
- `QUICKSTART.md` - Getting started guide
- `DEVELOPMENT.md` - Contribution workflow and phase tracking
- `AGENTS.md` - Developer guide for AI agents and contributors
- `PHASE_1_DEVELOPER_GUIDE.md` - Phase 1 specific guide
- `TESTING.md` - General testing guide
- `TESTING_PHASE6.md` - Phase 6 testing specifics
- `FIXTURE_CAPTURE_GUIDE.md` - Fixture capture workflow
- `VM_TEST_WORKER_HELPER.md` - VM testing documentation
- `STUB_COMMANDS.md` - Incomplete command documentation
- `INCONSISTENCIES.md` - Known issues and inconsistencies

**docs/design/** (20 files):
- Phase specifications: `PHASE_1_LIBRARY.md` through `PHASE_7_INTEGRATION.md`
- Phase TODOs: `PHASE_X_TODO.md` (various phases)
- Analysis docs: `PHASE_1_ANALYSIS_SUMMARY.md`, `PHASE_1.5_FIDELITY_ANALYSIS.md`
- Planning docs: `phase_1.5_part_b_plan.md`
- Brainstorming: `IDEAS.md`, `IDEAS_MVP.md`, `FUTURE_BACKLOG.md`
- Post-MVP: `POST_MVP_ANALYSIS.md` (just added)

**docs/issues/** (5 files):
- Implementation issue tracking for specific features

**docs/refactoring/** (1 file):
- `REFACTOR_ISSUE_FIXES.md`

**docs/testing/** (1 file):
- `VM_TESTING.md`

### Problems Identified

1. **Scattered Documentation**
   - 11 markdown files in root directory
   - Unclear which docs are authoritative vs historical
   - Testing docs split between root, docs/design, and docs/testing

2. **Mixed Timeframes**
   - Pre-MVP planning (IDEAS, phase specs) mixed with current status
   - No clear indicator of what's historical vs current
   - Phase TODO files mixed with phase specifications

3. **Unclear Hierarchy**
   - No clear entry point beyond README
   - No index or table of contents for docs/ directory
   - Related content scattered (testing in 3+ places)

4. **Inconsistent Naming**
   - Mix of UPPERCASE and lowercase filenames
   - No consistent prefix/suffix for doc types
   - Phase docs use different naming patterns

5. **Duplication Risk**
   - Multiple testing guides
   - Fixture capture documented in multiple places
   - Development workflow spread across DEVELOPMENT.md, AGENTS.md, and phase guides

## Target Information Architecture

### Proposed Directory Structure

```
/
├── README.md                   # Product overview, quick start link, key features
├── QUICKSTART.md              # Installation, init, first build (keep in root)
├── DEVELOPMENT.md             # Contribution workflow, phase status (keep in root)
├── CHANGELOG.md               # Version history (create)
│
└── docs/
    ├── INDEX.md               # Master documentation index
    │
    ├── overview/              # High-level system documentation
    │   ├── INDEX.md
    │   ├── architecture.md    # System architecture overview
    │   ├── concepts.md        # Core concepts (packages, deps, CRC, workers)
    │   └── workflows.md       # Common workflows and patterns
    │
    ├── design/                # Design specifications and analyses
    │   ├── INDEX.md
    │   │
    │   ├── mvp/               # MVP phase specifications (historical)
    │   │   ├── phase1_library.md
    │   │   ├── phase2_builddb.md
    │   │   ├── phase3_builder.md
    │   │   ├── phase4_environment.md
    │   │   ├── phase5_min_api.md
    │   │   ├── phase6_testing.md
    │   │   └── phase7_integration.md
    │   │
    │   └── post-mvp/          # Post-MVP refactoring and analysis
    │       ├── build_architecture_analysis.md
    │       ├── environment_backends.md
    │       └── api_design.md
    │
    ├── guide/                 # How-to guides and tutorials
    │   ├── INDEX.md
    │   ├── building_packages.md
    │   ├── environment_isolation.md
    │   ├── logging_system.md
    │   ├── stats_monitoring.md
    │   ├── fixture_capture.md
    │   ├── vm_testing.md
    │   └── troubleshooting.md
    │
    ├── api/                   # API reference (CLI and future REST)
    │   ├── INDEX.md
    │   ├── cli_reference.md   # Command reference (consolidate STUB_COMMANDS)
    │   └── rest_api.md        # Future REST API (placeholder)
    │
    ├── ops/                   # Operations and maintenance
    │   ├── INDEX.md
    │   ├── deployment.md
    │   ├── monitoring.md
    │   └── known_issues.md    # Consolidate INCONSISTENCIES.md
    │
    └── history/               # Historical context and decisions
        ├── INDEX.md
        ├── roadmap.md         # Project timeline and milestones
        ├── brainstorming.md   # Consolidate IDEAS*.md, FUTURE_BACKLOG.md
        ├── phase_summaries.md # Summary of what each phase achieved
        └── adr/               # Architecture Decision Records
            └── README.md
```

### Design Principles

1. **Clear Separation**
   - MVP vs Post-MVP clearly separated
   - Design vs Guide vs Operations clearly separated
   - Historical vs Current marked with status tags

2. **Consistent Navigation**
   - Every directory has an INDEX.md
   - Master index at docs/INDEX.md
   - Cross-references use consistent patterns

3. **Single Source of Truth**
   - Each topic has ONE authoritative document
   - Historical docs clearly marked as such
   - Cross-links instead of duplication

4. **Progressive Disclosure**
   - README → QUICKSTART → docs/INDEX.md → specific guides
   - Overviews link to detailed guides
   - Guides link to design rationale

5. **Consistent Naming**
   - Lowercase with underscores: `building_packages.md`
   - Phase docs: `phaseN_topic.md`
   - Descriptive names, not abbreviations

## File Migration Plan

### Phase 1: Create New Structure (No Deletions)

**Create Index Files:**
```
docs/INDEX.md
docs/overview/INDEX.md
docs/design/INDEX.md
docs/guide/INDEX.md
docs/api/INDEX.md
docs/ops/INDEX.md
docs/history/INDEX.md
```

**Create Directories:**
```
mkdir -p docs/overview
mkdir -p docs/design/mvp
mkdir -p docs/design/post-mvp
mkdir -p docs/guide
mkdir -p docs/api
mkdir -p docs/ops
mkdir -p docs/history/adr
```

### Phase 2: Move and Rename Files

**Design Documents (MVP):**
```
docs/design/PHASE_1_LIBRARY.md → docs/design/mvp/phase1_library.md
docs/design/PHASE_2_BUILDDB.md → docs/design/mvp/phase2_builddb.md
docs/design/PHASE_3_BUILDER.md → docs/design/mvp/phase3_builder.md
docs/design/PHASE_4_ENVIRONMENT.md → docs/design/mvp/phase4_environment.md
docs/design/PHASE_5_MIN_API.md → docs/design/mvp/phase5_min_api.md
docs/design/PHASE_6_TESTING.md → docs/design/mvp/phase6_testing.md
docs/design/PHASE_7_INTEGRATION.md → docs/design/mvp/phase7_integration.md
```

**Design Documents (Post-MVP):**
```
docs/design/POST_MVP_ANALYSIS.md → docs/design/post-mvp/build_architecture_analysis.md
```

**Historical/Brainstorming:**
```
docs/design/IDEAS.md → docs/history/brainstorming.md (consolidate)
docs/design/IDEAS_MVP.md → docs/history/brainstorming.md (merge)
docs/design/FUTURE_BACKLOG.md → docs/history/brainstorming.md (merge)
```

**Analysis Documents (Archive):**
```
docs/design/PHASE_1_ANALYSIS_SUMMARY.md → docs/history/phase_summaries.md (merge)
docs/design/PHASE_1.5_FIDELITY_ANALYSIS.md → docs/history/phase_summaries.md (merge)
docs/design/phase_1.5_part_b_plan.md → docs/history/phase_summaries.md (merge)
```

**TODO Documents (Archive or Delete):**
```
docs/design/PHASE_*_TODO.md → Remove (outdated, captured in DEVELOPMENT.md)
```

**Guide Documents:**
```
FIXTURE_CAPTURE_GUIDE.md → docs/guide/fixture_capture.md
docs/testing/VM_TESTING.md → docs/guide/vm_testing.md
VM_TEST_WORKER_HELPER.md → docs/guide/vm_testing.md (merge)
TESTING.md → docs/guide/testing.md
TESTING_PHASE6.md → docs/history/phase_summaries.md (merge)
PHASE_1_DEVELOPER_GUIDE.md → docs/history/phase_summaries.md (merge)
```

**Operations Documents:**
```
INCONSISTENCIES.md → docs/ops/known_issues.md
docs/refactoring/REFACTOR_ISSUE_FIXES.md → docs/ops/known_issues.md (merge)
```

**API Documents:**
```
STUB_COMMANDS.md → docs/api/cli_reference.md
```

**Issues (Convert to Tickets or Archive):**
```
docs/issues/*.md → GitHub issues or docs/ops/known_issues.md
```

**Keep in Root:**
```
README.md (update with links to docs/INDEX.md)
QUICKSTART.md (no change)
DEVELOPMENT.md (no change, update links)
AGENTS.md (no change, update links)
CHANGELOG.md (create)
```

### Phase 3: Update Cross-References

**Update README.md:**
- Add link to docs/INDEX.md
- Add link to QUICKSTART.md
- Add link to DEVELOPMENT.md

**Update DEVELOPMENT.md:**
- Link to docs/design/mvp/ for phase specs
- Link to docs/design/post-mvp/ for refactoring plans
- Link to docs/guide/ for how-tos

**Update AGENTS.md:**
- Link to docs/overview/architecture.md
- Link to docs/guide/ for specific workflows

**Fix Internal Links:**
- Update all relative links in moved files
- Update links in code comments (if any)
- Update links in DEVELOPMENT.md phase tracking

### Phase 4: Add Metadata Tags

Add front matter to all design docs:

```markdown
---
status: MVP | Post-MVP | Historical | Deprecated
phase: 1-7 (for MVP docs)
updated: YYYY-MM-DD
---
```

Example:
```markdown
---
status: Historical
phase: 1
updated: 2024-XX-XX
---

# Phase 1: Library Extraction

This document describes the original Phase 1 specification...
```

### Phase 5: Create Content

**docs/INDEX.md** - Master index with:
- Link to overview/
- Link to design/ (MVP and Post-MVP)
- Link to guide/
- Link to api/
- Link to ops/
- Link to history/

**docs/overview/architecture.md** - High-level architecture:
- System components diagram
- Package relationships
- Build workflow overview
- Links to detailed design docs

**docs/overview/concepts.md** - Core concepts:
- Package metadata
- Dependency resolution
- CRC-based incremental builds
- Worker pool
- Environment isolation

**docs/guide/troubleshooting.md** - Common issues:
- Build failures
- Mount errors
- Permission problems
- Known issues link

**docs/history/roadmap.md** - Project timeline:
- MVP phases completed
- Current state
- Future plans
- Links to phase summaries

**docs/history/phase_summaries.md** - What each phase achieved:
- Phase 1: Library extraction (pkg package)
- Phase 2: Build database (builddb)
- Phase 3: Builder orchestration
- Phase 4: Environment abstraction
- Phase 5: Minimal API (deferred)
- Phase 6: Testing strategy
- Phase 7: Integration and completion

## Migration Execution Steps

### Step 1: Create Structure (Low Risk)

```bash
# Create new directory structure
mkdir -p docs/overview
mkdir -p docs/design/mvp
mkdir -p docs/design/post-mvp
mkdir -p docs/guide
mkdir -p docs/api
mkdir -p docs/ops
mkdir -p docs/history/adr

# Create placeholder INDEX.md files
touch docs/INDEX.md
touch docs/overview/INDEX.md
touch docs/design/INDEX.md
touch docs/guide/INDEX.md
touch docs/api/INDEX.md
touch docs/ops/INDEX.md
touch docs/history/INDEX.md
touch docs/history/adr/README.md
```

### Step 2: Move MVP Phase Docs (Low Risk)

```bash
# Move phase specifications
git mv docs/design/PHASE_1_LIBRARY.md docs/design/mvp/phase1_library.md
git mv docs/design/PHASE_2_BUILDDB.md docs/design/mvp/phase2_builddb.md
git mv docs/design/PHASE_3_BUILDER.md docs/design/mvp/phase3_builder.md
git mv docs/design/PHASE_4_ENVIRONMENT.md docs/design/mvp/phase4_environment.md
git mv docs/design/PHASE_5_MIN_API.md docs/design/mvp/phase5_min_api.md
git mv docs/design/PHASE_6_TESTING.md docs/design/mvp/phase6_testing.md
git mv docs/design/PHASE_7_INTEGRATION.md docs/design/mvp/phase7_integration.md

# Move post-MVP analysis
git mv docs/design/POST_MVP_ANALYSIS.md docs/design/post-mvp/build_architecture_analysis.md
```

### Step 3: Move Guides (Medium Risk - Update Links)

```bash
# Move fixture capture guide
git mv FIXTURE_CAPTURE_GUIDE.md docs/guide/fixture_capture.md

# Move VM testing docs
git mv docs/testing/VM_TESTING.md docs/guide/vm_testing.md

# Move general testing guide
git mv TESTING.md docs/guide/testing.md
```

### Step 4: Consolidate Historical Docs (High Effort)

Manually merge these into consolidated documents:
- IDEAS.md, IDEAS_MVP.md, FUTURE_BACKLOG.md → docs/history/brainstorming.md
- PHASE_1_ANALYSIS_SUMMARY.md, PHASE_1.5_FIDELITY_ANALYSIS.md → docs/history/phase_summaries.md
- TESTING_PHASE6.md, PHASE_1_DEVELOPER_GUIDE.md → docs/history/phase_summaries.md

### Step 5: Create Index Pages (High Value)

Write comprehensive INDEX.md for each section with:
- Purpose of the section
- Links to all documents
- Recommended reading order
- Cross-references to related sections

### Step 6: Update Cross-References (Critical)

Systematically update all links:
1. Generate list of moved files
2. Search for references in all .md and .go files
3. Update relative links
4. Verify all links work

### Step 7: Add Status Tags (Low Effort)

Add front matter to all design docs indicating status.

### Step 8: Clean Up (Final Step)

Remove:
- Outdated TODO files (PHASE_*_TODO.md)
- Duplicate content after consolidation
- Empty directories

## Document Status Taxonomy

### Status Tags

- **Current** - Actively maintained, reflects current state
- **MVP** - Historical specification from MVP phase
- **Post-MVP** - Current design work beyond MVP
- **Historical** - Archived for reference, not current
- **Deprecated** - Superseded by newer documents
- **Draft** - Work in progress, not finalized

### Usage Guidelines

**MVP Phase Docs (docs/design/mvp/):**
```markdown
---
status: MVP
phase: N
completed: YYYY-MM-DD
---
```

**Post-MVP Design (docs/design/post-mvp/):**
```markdown
---
status: Post-MVP
updated: YYYY-MM-DD
---
```

**Guides (docs/guide/):**
```markdown
---
status: Current
updated: YYYY-MM-DD
applies-to: version X.Y.Z
---
```

**Historical (docs/history/):**
```markdown
---
status: Historical
archived: YYYY-MM-DD
superseded-by: path/to/current/doc.md
---
```

## Documentation Contribution Guidelines

### Where to Add New Docs

| Type | Location | Example |
|------|----------|---------|
| Design spec (new feature) | `docs/design/post-mvp/` | `distributed_builds.md` |
| How-to guide | `docs/guide/` | `custom_builders.md` |
| Troubleshooting | `docs/guide/troubleshooting.md` | Add section |
| CLI command | `docs/api/cli_reference.md` | Add command |
| Known issue | `docs/ops/known_issues.md` | Add issue |
| Architecture decision | `docs/history/adr/` | `NNN-decision-title.md` |
| Historical context | `docs/history/` | `phase_summaries.md` |

### Naming Conventions

- Use lowercase with underscores: `build_system.md`
- Be descriptive: `fixture_capture.md` not `fixtures.md`
- Phase docs: `phaseN_topic.md` (e.g., `phase4_environment.md`)
- ADRs: `NNN-short-title.md` (e.g., `001-use-bbolt-database.md`)

### Required Elements

Every document should have:
1. **Front matter** with status and date
2. **Title** (H1) matching filename
3. **Purpose/Context** section
4. **See Also** section with cross-references
5. **Update index** - add link to relevant INDEX.md

### Cross-Linking Best Practices

- Use relative links from docs/: `../guide/testing.md`
- Link to sections with anchors: `architecture.md#worker-pool`
- Add "See also" sections at the end
- Update INDEX.md when adding new docs

### Commit Guidelines

When adding/updating docs:
- Update relevant INDEX.md files
- Fix broken links in related docs
- Add status tag if missing
- Mention doc changes in commit message

## Benefits of Reorganization

### For New Contributors

- Clear entry point: `docs/INDEX.md`
- Guides separated from design specs
- Current vs historical clearly marked
- Reduced confusion from scattered docs

### For Maintainers

- Single source of truth for each topic
- Easier to find and update docs
- Clear structure for new content
- Reduced duplication

### For Users

- Quick start remains easy (QUICKSTART.md in root)
- Guides organized by task
- API reference centralized
- Troubleshooting consolidated

## Quick Wins (Low-Hanging Fruit)

### 1. Create docs/INDEX.md (1 hour)

Create master index linking to all major sections. Immediate improvement to
discoverability.

### 2. Move Phase Docs to mvp/ (30 minutes)

```bash
mkdir -p docs/design/mvp
git mv docs/design/PHASE_*.md docs/design/mvp/
# Rename to lowercase
# Fix links in DEVELOPMENT.md
```

Low risk, immediate organization improvement.

### 3. Consolidate Testing Docs (2 hours)

Merge TESTING.md, TESTING_PHASE6.md, VM_TESTING.md into:
- `docs/guide/testing.md` (general)
- `docs/guide/vm_testing.md` (VM-specific)
- `docs/history/phase_summaries.md` (phase 6 notes)

Reduces scatter, single source of truth.

### 4. Add Status Tags (1 hour)

Add front matter to all phase docs marking them as "MVP" status. Immediate
clarity on doc age/relevance.

## Open Questions

1. **Timing**: Should we complete this before or after next major feature work?
   - **Recommendation**: Do structure and moves now (Phase 1-3), consolidation
     later

2. **Breaking Links**: How to handle external links to old paths?
   - **Recommendation**: Add redirects in README if docs are referenced
     externally
   - Keep old structure for 1 release cycle with deprecation notices

3. **TODO Files**: Delete or archive phase TODO files?
   - **Recommendation**: Delete - they're captured in DEVELOPMENT.md and git
     history

4. **Issue Docs**: Convert to GitHub issues or keep as markdown?
   - **Recommendation**: Merge resolved items into known_issues.md, create
     GitHub issues for unresolved items

5. **Environment README**: Keep or consolidate?
   - **Recommendation**: Keep as package-level documentation, add link from
     docs/guide/environment_isolation.md

## Success Criteria

Reorganization is successful when:

1. ✅ All docs accessible from docs/INDEX.md
2. ✅ No documentation in root except README, QUICKSTART, DEVELOPMENT, AGENTS,
   CHANGELOG
3. ✅ MVP and Post-MVP clearly separated
4. ✅ Each topic has ONE authoritative document
5. ✅ All design docs have status tags
6. ✅ No broken internal links
7. ✅ New contributors can navigate docs easily
8. ✅ Contribution guidelines clearly document where to add new docs

## Timeline Estimate

| Phase | Effort | Risk | Dependencies |
|-------|--------|------|--------------|
| 1. Create structure | 1 hour | Low | None |
| 2. Move phase docs | 1 hour | Low | Phase 1 |
| 3. Move guides | 2 hours | Medium | Phase 1 |
| 4. Consolidate historical | 4 hours | High | Phase 2-3 |
| 5. Create indexes | 3 hours | Medium | Phase 2-3 |
| 6. Update cross-refs | 3 hours | High | Phase 2-5 |
| 7. Add status tags | 1 hour | Low | Phase 2-3 |
| 8. Clean up | 1 hour | Low | Phase 4-7 |
| **Total** | **16 hours** | | |

**Recommended Approach**: Execute phases 1-3 immediately (4 hours, low risk),
then tackle consolidation and cross-references in a dedicated session.

## Next Steps

1. **Review this plan** - discuss with team/maintainers
2. **Approve structure** - agree on target IA
3. **Execute Phase 1** - create directory structure and indexes
4. **Execute Phase 2** - move phase docs (low risk)
5. **Test navigation** - verify links work from docs/INDEX.md
6. **Continue incrementally** - tackle guides and consolidation
7. **Final verification** - check all links, update README
8. **Document new structure** - update DEVELOPMENT.md contribution guidelines

---

**Status**: Draft for review  
**Next Review**: After initial feedback  
**Implementation Target**: TBD
