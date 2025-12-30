# Contributing to Orchix 🚀

Thank you for your interest in contributing to Orchix! This document provides guidelines and instructions for contributing to the project.

## Table of Contents
- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Workflow](#development-workflow)
- [Project Structure](#project-structure)
- [Coding Standards](#coding-standards)
- [Commit Guidelines](#commit-guidelines)
- [Pull Request Process](#pull-request-process)
- [Testing](#testing)
- [Documentation](#documentation)
- [Issue Reporting](#issue-reporting)
- [Community](#community)

## Code of Conduct

We are committed to providing a welcoming and inclusive environment for all contributors. Please read our [Code of Conduct](CODE_OF_CONDUCT.md) before participating.

## Getting Started

### Prerequisites
- Go 1.21 or higher
- Docker (for provider development)
- Git
- Make (recommended)

### Setting Up Development Environment

1. **Fork the Repository**
   ```bash
   # Click the "Fork" button on GitHub
   # Clone your fork
   git clone https://github.com/YOUR_USERNAME/Orchix.git
   cd Orchix
   ```

2. **Add Upstream Remote**
   ```bash
   git remote add upstream https://github.com/Bibekbb/Orchix.git
   ```

3. **Install Dependencies**
   ```bash
   make build
   ```

4. **Verify Setup**
   ```bash
   ./orchix --version
   ```

### First Time Contributors
Look for issues labeled:
- `good-first-issue` - Beginner friendly
- `help-wanted` - Need assistance
- `documentation` - Documentation improvements

## Development Workflow

### Branch Strategy
- `main` - Stable production-ready code
- `develop` - Development branch (default)
- `feature/*` - New features
- `fix/*` - Bug fixes
- `docs/*` - Documentation updates
- `test/*` - Test improvements

### Creating a Feature
```bash
# 1. Update develop branch
git checkout develop
git pull upstream develop

# 2. Create feature branch
git checkout -b feature/your-feature-name

# 3. Make your changes
# ... edit files ...

# 4. Test your changes
make test

# 5. Commit with conventional commit message
git commit -m "feat: add docker provider health checks"

# 6. Push to your fork
git push origin feature/your-feature-name

# 7. Create Pull Request on GitHub
```

### Making Changes
1. Make changes in logical units
2. Add/update tests for new functionality
3. Update documentation if needed
4. Ensure code passes linting and tests
5. Follow the coding standards below

## Project Structure

```
Orchix/
├── cmd/                    # CLI entry points
│   ├── orchix/            # Main CLI
│   └── orchix-agent/      # Agent mode
├── internal/              # Private application code
│   ├── cli/              # CLI implementation
│   ├── core/             # Core orchestration logic
│   ├── providers/        # Provider implementations
│   └── utils/            # Utilities
├── pkg/                   # Public packages
│   ├── types/            # Public types
│   ├── config/           # Configuration schemas
│   └── api/              # Public API
├── examples/             # Example configurations
├── test/                 # Test files
├── docs/                 # Documentation
├── scripts/              # Build and utility scripts
└── assets/               # Static assets
```

## Coding Standards

### Go Style Guide
- Follow [Effective Go](https://go.dev/doc/effective_go)
- Use `gofmt` for formatting
- Maximum line length: 120 characters
- Use meaningful variable and function names

### Naming Conventions
- **Packages**: lowercase, single word
- **Interfaces**: `er` suffix (e.g., `Provider`, `Logger`)
- **Constants**: `UPPER_CASE`
- **Exported functions**: `PascalCase`
- **Private functions**: `camelCase`

### Error Handling
```go
// Good
if err := doSomething(); err != nil {
    return fmt.Errorf("failed to do something: %w", err)
}

// Bad
if err != nil {
    log.Fatal(err)
}
```

### Logging
Use structured logging with levels:
```go
logger.Debug("debug message")
logger.Info("info message", "component", "engine")
logger.Warn("warning message", "error", err)
logger.Error("error message", "error", err)
```

## Commit Guidelines

We follow [Conventional Commits](https://www.conventionalcommits.org/):

### Commit Message Format
```
<type>[optional scope]: <description>

[optional body]

[optional footer(s)]
```

### Types
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `style`: Code style changes (formatting, etc.)
- `refactor`: Code refactoring
- `test`: Adding or fixing tests
- `chore`: Maintenance tasks
- `ci`: CI/CD changes
- `build`: Build system changes
- `perf`: Performance improvements
- `revert`: Revert changes

### Examples
```bash
# Feature with scope
git commit -m "feat(providers): add docker compose support"

# Bug fix
git commit -m "fix(engine): handle circular dependencies"

# Documentation
git commit -m "docs: update quick start guide"

# Breaking change (note the !)
git commit -m "feat(cli)!: change deploy command signature"
```

## Pull Request Process

### Before Submitting
1. **Update your branch** with latest changes from `develop`
   ```bash
   git fetch upstream
   git rebase upstream/develop
   ```

2. **Run tests**
   ```bash
   make test
   make lint
   ```

3. **Update documentation** if needed

4. **Add/update tests** for your changes

### PR Checklist
- [ ] Code follows project standards
- [ ] Tests pass
- [ ] Documentation updated
- [ ] Commit messages follow conventions
- [ ] Changes are backward compatible (unless marked as breaking)
- [ ] PR description includes:
  - Purpose of changes
  - Testing performed
  - Screenshots (if UI changes)
  - Related issues

### PR Review Process
1. Automated checks pass (CI/CD)
2. Code review by maintainers
3. Address review comments
4. Maintainer merges when approved

## Testing

### Running Tests
```bash
# Run all tests
make test

# Run specific test
go test ./internal/core/... -v

# Run with coverage
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out

# Integration tests
cd test/integration
go test -v
```

### Writing Tests
- Test files should be named `*_test.go`
- Use table-driven tests when appropriate
- Mock external dependencies
- Test both success and failure cases

Example:
```go
func TestEngine_Deploy(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        wantErr bool
    }{
        {"valid manifest", "test.yaml", false},
        {"invalid manifest", "bad.yaml", true},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test logic
        })
    }
}
```

## Documentation

### Documentation Types
1. **Code Documentation** - GoDoc comments
2. **User Documentation** - Guides and tutorials
3. **API Documentation** - REST API specs
4. **Architecture Docs** - System design

### Adding Documentation
```bash
# User documentation
docs/getting-started.md
docs/providers/docker.md

# API documentation
docs/api/rest-api.md

# Architecture
docs/architecture/overview.md
```

### GoDoc Comments
```go
// Package engine provides the core orchestration engine.
package engine

// Engine orchestrates deployment of application components.
// It handles dependency resolution, execution planning,
// and coordination between providers.
type Engine struct {
    // ... fields
}

// Deploy executes the deployment plan.
// It validates the manifest, resolves dependencies,
// and executes providers in the correct order.
//
// Parameters:
//   ctx: Context for cancellation and timeout
//   dryRun: If true, only shows plan without executing
//
// Returns:
//   error: Any error that occurred during deployment
func (e *Engine) Deploy(ctx context.Context, dryRun bool) error {
    // ...
}
```

## Issue Reporting

### Bug Reports
When reporting bugs, include:
1. **Description** - What happened vs expected
2. **Steps to Reproduce**
3. **Environment** - OS, Go version, Orchix version
4. **Logs/Output** - Error messages and logs
5. **Code/Screenshots** - If applicable

### Feature Requests
For feature requests:
1. **Use Case** - What problem does it solve?
2. **Proposed Solution** - How should it work?
3. **Alternatives Considered**
4. **Additional Context**

### Issue Templates
Use the GitHub issue templates for:
- 🐛 Bug Report
- 🚀 Feature Request
- 📚 Documentation Issue
- 🔧 Improvement Request

## Community

### Communication Channels
- **GitHub Issues** - Bug reports and feature requests
- **GitHub Discussions** - Questions and discussions
- **Pull Requests** - Code contributions

### Getting Help
1. Check the [documentation](docs/)
2. Search existing issues
3. Create a discussion for questions
4. Ask in PR reviews

### Recognition
Contributors will be:
- Listed in CONTRIBUTORS.md
- Thanked in release notes
- Eligible for maintainer status with consistent contributions

## Becoming a Maintainer

After consistent contributions, you may be invited to become a maintainer. Maintainers:
- Have merge permissions
- Review pull requests
- Help triage issues
- Guide new contributors

Requirements:
- Multiple significant contributions
- Understanding of codebase
- Active participation in community
- Following code of conduct

---

## Quick Reference

### Common Commands
```bash
# Build
make build

# Test
make test

# Lint
make lint

# Install
make install

# Run
make run ARGS="--help"

# Docker build
make docker
```

### Development Tips
1. **Start small** - Begin with documentation or simple bug fixes
2. **Ask questions** - Don't hesitate to ask for clarification
3. **Write tests** - Always add tests for new code
4. **Keep PRs focused** - One feature/fix per PR
5. **Be patient** - Reviewers may be in different time zones

---

Thank you for contributing to Orchix! Your help makes this project better for everyone. 🎉

*Last Updated: December 2025*