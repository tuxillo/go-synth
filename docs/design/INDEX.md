# Design Documentation Index

This section contains design specifications, architectural analyses, and planning documents for go-synth.

## Structure

### MVP Phase Specifications (Historical)
**Location**: [mvp/](mvp/)

Historical specifications from the MVP development phases (1-7). These documents capture the original design decisions and implementation plans.

- [Phase 1: Library Extraction](mvp/phase1_library.md) - Extraction of pkg package
- [Phase 2: Build Database](mvp/phase2_builddb.md) - bbolt-based persistence
- [Phase 3: Builder Orchestration](mvp/phase3_builder.md) - Worker pool and scheduling
- [Phase 4: Environment Abstraction](mvp/phase4_environment.md) - Isolation layer
- [Phase 5: Minimal API](mvp/phase5_min_api.md) - REST API (deferred)
- [Phase 6: Testing Strategy](mvp/phase6_testing.md) - Test infrastructure
- [Phase 7: Integration](mvp/phase7_integration.md) - Final integration

**Status**: These are historical documents. They capture the original plans but may not reflect current implementation details.

### Post-MVP Design (Current)
**Location**: [post-mvp/](post-mvp/)

Current design work, refactoring plans, and architectural analyses beyond the MVP.

- [Build Architecture Analysis](post-mvp/build_architecture_analysis.md) - Proposed refactoring for better separation of concerns

## Related Documentation

- [Overview](../overview/INDEX.md) - High-level architecture and concepts
- [History](../history/INDEX.md) - Project evolution and decisions
- [Guides](../guide/INDEX.md) - Implementation how-tos

## Adding Design Documentation

When adding new design docs:

1. **Post-MVP designs** go in `post-mvp/`
2. **Use descriptive names**: `feature_name_design.md` not `design.md`
3. **Add status front matter**:
   ```markdown
   ---
   status: Draft | Current | Implemented | Superseded
   updated: YYYY-MM-DD
   ---
   ```
4. **Update this INDEX.md** with a link to your new document
5. **Cross-reference** related docs (overview, guides, history)

## Document Lifecycle

- **Draft** - Work in progress, under discussion
- **Current** - Approved design, being implemented
- **Implemented** - Feature complete, move to guides for usage
- **MVP** - Historical MVP specification (for reference only)
- **Superseded** - Replaced by newer design, kept for context

---

**Last Updated**: 2025-12-05
