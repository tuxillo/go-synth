# go-synth Documentation Index

Welcome to the go-synth documentation. This index provides an overview of all available documentation organized by purpose.

## Getting Started

- [README](../README.md) - Project overview and introduction
- [Quick Start Guide](../QUICKSTART.md) - Installation and first build
- [Development Guide](../DEVELOPMENT.md) - Contribution workflow and current status
- [Agent Guide](../AGENTS.md) - Guide for AI agents and developers

## Documentation Sections

### 📚 Overview
High-level system documentation and core concepts.

**Location**: [docs/overview/](overview/INDEX.md)

- System architecture
- Core concepts (packages, dependencies, CRC, workers)
- Common workflows and patterns

### 🏗️ Design
Design specifications, architectural decisions, and analyses.

**Location**: [docs/design/INDEX.md](design/INDEX.md)

- **MVP Phase Specs** ([mvp/](design/mvp/)) - Historical phase 1-7 specifications
- **Post-MVP Design** ([post-mvp/](design/post-mvp/)) - Current refactoring and new features

### 📖 Guides
Task-oriented how-to guides and tutorials.

**Location**: [docs/guide/INDEX.md](guide/INDEX.md)

- Building packages
- Environment isolation
- Logging and monitoring
- Fixture capture
- VM testing
- Troubleshooting

### 🔧 API Reference
Command-line interface and API documentation.

**Location**: [docs/api/INDEX.md](api/INDEX.md)

- CLI command reference
- REST API (future)

### ⚙️ Operations
Deployment, operations, and maintenance documentation.

**Location**: [docs/ops/INDEX.md](ops/INDEX.md)

- Deployment procedures
- Monitoring and metrics
- Known issues and workarounds

### 📜 History
Historical context, decisions, and project evolution.

**Location**: [docs/history/INDEX.md](history/INDEX.md)

- Project roadmap and timeline
- Brainstorming and ideas
- Phase summaries
- Architecture Decision Records (ADRs)

## Quick Links

### For New Contributors
1. Read [QUICKSTART.md](../QUICKSTART.md) to get the project running
2. Read [DEVELOPMENT.md](../DEVELOPMENT.md) to understand contribution workflow
3. Browse [docs/overview/](overview/INDEX.md) to understand architecture
4. Check [docs/guide/](guide/INDEX.md) for specific how-tos

### For Developers
1. [AGENTS.md](../AGENTS.md) - Development patterns and workflows
2. [docs/design/post-mvp/](design/post-mvp/) - Current design work
3. [docs/guide/](guide/INDEX.md) - Implementation guides

### For Operators
1. [docs/ops/](ops/INDEX.md) - Operational procedures
2. [docs/api/](api/INDEX.md) - CLI reference
3. [docs/guide/troubleshooting.md](guide/INDEX.md) - Common issues

## Documentation Standards

When adding new documentation:
- Place design specs in `docs/design/post-mvp/`
- Place how-to guides in `docs/guide/`
- Place operational docs in `docs/ops/`
- Update the relevant INDEX.md file
- Add cross-references to related docs
- Use lowercase_with_underscores for filenames

See [DEVELOPMENT.md](../DEVELOPMENT.md) for complete contribution guidelines.

---

**Last Updated**: 2025-12-05  
**Maintained By**: go-synth project
