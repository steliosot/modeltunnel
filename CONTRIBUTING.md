# Contributing to Modeltunnel

First off, thank you for considering contributing to Modeltunnel! It's people like you that make Modeltunnel such a great tool.

## Code of Conduct

This project and everyone participating in it is governed by our [Code of Conduct](CODE_OF_CONDUCT.md). By participating, you are expected to uphold this code.

## How Can I Contribute?

### Reporting Bugs

Before creating bug reports, please check the existing issues to see if the problem has already been reported. When you are creating a bug report, please include as many details as possible:

- **Use a clear and descriptive title**
- **Describe the exact steps to reproduce the problem**
- **Provide specific examples to demonstrate the steps**
- **Describe the behavior you observed and what behavior you expected**
- **Include screenshots if applicable**
- **Include your environment details** (OS, Go version, Ollama version)

### Suggesting Enhancements

Enhancement suggestions are tracked as GitHub issues. When creating an enhancement suggestion, please include:

- **Use a clear and descriptive title**
- **Provide a step-by-step description of the suggested enhancement**
- **Provide specific examples to demonstrate the enhancement**
- **Explain why this enhancement would be useful**

### Pull Requests

1. Fork the repository
2. Create a new branch from `main` for your feature or bug fix
3. Make your changes
4. Add or update tests as needed
5. Ensure all tests pass (`go test ./...`)
6. Update documentation if needed
7. Submit a pull request

## Development Setup

### Prerequisites

- Go 1.21 or later
- Ollama (for testing)
- SQLite (usually included with Go)

### Building from Source

```bash
# Clone the repository
git clone https://github.com/steliosot/modeltunnel.git
cd modeltunnel

# Build the binary
go build -o modeltunnel ./cmd/modeltunnel/main.go

# Run tests
go test ./...

# Install locally
go install ./cmd/modeltunnel
```

### Project Structure

```
modeltunnel/
├── cmd/modeltunnel/       # CLI entry point
├── internal/
│   ├── config/           # Configuration management
│   ├── db/               # SQLite database
│   ├── gateway/          # Rate limiting
│   ├── keys/             # API key management
│   ├── server/           # HTTP server and dashboard
│   ├── tunnel/           # Tunnel clients (localtunnel, ngrok)
│   └── upstream/         # LLM provider adapters (Ollama, etc.)
├── pkg/openai/           # OpenAI-compatible types
└── tests/                # Integration tests
```

## Style Guidelines

### Go Code Style

We follow standard Go conventions:

- Use `gofmt` to format your code
- Use `golint` to check for style issues
- Use `go vet` to check for suspicious constructs
- Follow [Effective Go](https://golang.org/doc/effective_go.html) guidelines

### Commit Messages

- Use the present tense ("Add feature" not "Added feature")
- Use the imperative mood ("Move cursor to..." not "Moves cursor to...")
- Limit the first line to 72 characters or less
- Reference issues and pull requests liberally after the first line

Example:
```
Add per-model rate limiting

Implements different rate limits for different models based on
their resource usage. For example:
- mistral: 5/min (expensive)
- phi: 100/min (cheap)

Fixes #42
```

### Documentation

- Update the README.md if you change functionality
- Add comments to exported functions and types
- Update the dashboard documentation if you change UI

## Testing

### Running Tests

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run integration tests
cd tests && python3 run_all_tests.py
```

### Writing Tests

- Add tests for new features
- Ensure existing tests pass before submitting PR
- Include both unit tests and integration tests where appropriate

## Release Process

1. Update CHANGELOG.md with new version
2. Create a new release on GitHub
3. Tag the release with semantic versioning (e.g., `v1.2.3`)
4. GitHub Actions will automatically build and release binaries

## Community

- Join our [Discord/Slack] (coming soon)
- Follow us on [Twitter] (coming soon)

## Questions?

Feel free to open an issue for any questions or concerns. We're happy to help!

Thank you for contributing! 🚀
