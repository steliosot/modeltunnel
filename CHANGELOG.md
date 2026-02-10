# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- **Docker Support** - Multi-stage Dockerfile for easy containerized deployment
- Alpine-based minimal runtime image (~45MB)
- Non-root user in Docker for security
- config.docker.yaml for containerized deployments
- Docker deployment documentation in installation.md
- Docker documentation in dashboard web UI
- **Intent Routing YAML Configuration** - Configure intent priorities in `config.yaml` with hot-reload support
- Custom intent definitions - Add your own intents beyond plan/code/chat
- Intent priority lists - Define model fallback order
- Per-intent parameters - Set temperature and max_tokens per intent
- Per-model rate limiting - Different rate limits for different models (e.g., mistral=5/min, phi=100/min)
- CLI/Server key synchronization - Keys created via CLI are available immediately (no restart needed)
- Automatic key refresh from database every 5 seconds
- SQLite database persistence for API keys
- Model size and modified date in API responses
- Config file backup - Keys preserved in config as backup
- Comprehensive test suite with 25+ tests
- Complete API documentation with examples
- Full CLI reference documentation
- Installation and configuration guides

### Fixed
- Message content parsing - Handles OpenAI-style content arrays (text/image parts)
- Invalid request body errors - Now supports both simple string and array content formats

### Documentation
- **New: docs/CONNECTING_APPS.md** - Step-by-step guide for connecting OpenCode, Cursor, and other apps
- Added comprehensive API reference (docs/api.md) with full endpoint documentation
- **Enhanced Async Jobs docs (docs/ASYNC_JOBS.md)** with 5 complete examples:
  - Document summarization with polling
  - Batch code review system
  - Research report generation
  - JavaScript/Node.js client implementation
  - Bash script with polling
- **Enhanced Intent Routing docs (docs/INTENT_ROUTING.md)** with 5 complete examples:
  - Smart application router with auto-detection
  - IDE assistant with different intents
  - Customer support bot
  - JavaScript web app implementation
  - Content generation pipeline
- Added CLI reference documentation (docs/cli.md)
- Added installation guide (docs/installation.md)
- Added configuration guide (docs/configuration.md)
- Updated README with quick examples
- Added "Combining Features" section to API docs

### Security
- Added proper error logging for database operations
- Graceful shutdown with database cleanup

### Fixed
- Database connection resource leak on shutdown
- Silent database failures now logged properly
- Permanent key loss on migration (config now kept as backup)

## [1.0.0] - 2026-02-10

### Added
- Initial release of Modeltunnel
- OpenAI-compatible API for local Ollama models
- Built-in web dashboard for key management
- CLI for key creation, listing, and revocation
- Rate limiting with configurable policies
- Public tunnel support via LocalTunnel
- Hot-reload configuration
- Request logging via WebSocket
- Code samples in dashboard (Python, cURL, JavaScript)
- API key-based authentication
- Per-key upstream restrictions
- Model wildcard matching (e.g., `tinyllama:*`)

### Features
- **Dashboard**: Overview, API Keys, Configuration, Code Samples, Request Logs
- **CLI Commands**: `up`, `key create`, `key list`, `key revoke`, `init`
- **API Endpoints**:
  - `GET /v1/models` - List available models
  - `POST /v1/chat/completions` - Chat completion (streaming supported)
  - `GET /health` - Health check
- **Configuration**: YAML-based with hot-reload
- **Persistence**: SQLite database for keys

### Security
- API key authentication required for all endpoints
- Configurable rate limiting per key
- Upstream access restrictions per key
- Localhost-only binding by default

## Release Notes Template

When creating a new release, use this format:

```markdown
## [X.Y.Z] - YYYY-MM-DD

### Added
- New features

### Changed
- Changes in existing functionality

### Deprecated
- Soon-to-be removed features

### Removed
- Now removed features

### Fixed
- Bug fixes

### Security
- Security improvements
```

---

**Legend:**
- **Added** - New features
- **Changed** - Changes to existing functionality
- **Deprecated** - Soon-to-be removed features
- **Removed** - Removed features
- **Fixed** - Bug fixes
- **Security** - Security improvements

[Unreleased]: https://github.com/steliosot/modeltunnel/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/steliosot/modeltunnel/releases/tag/v1.0.0
