# Contributing to Reloader

Thank you for your interest in contributing to the Reloader project! This document provides guidelines and information for contributors.

## Code of Conduct

By participating in this project, you agree to abide by our Code of Conduct. Please be respectful and constructive in all interactions.

## How to Contribute

### Reporting Bugs

1. **Check existing issues** - Look through existing issues to see if the bug has already been reported
2. **Create a detailed issue** - Include:
   - Go version
   - Operating system
   - Steps to reproduce
   - Expected vs actual behavior
   - Code examples (if applicable)

### Suggesting Features

1. **Check existing issues** - Look for existing feature requests
2. **Create a feature request** - Include:
   - Clear description of the feature
   - Use cases and motivation
   - Proposed API (if applicable)

### Submitting Code Changes

1. **Fork the repository**
2. **Create a feature branch** from `main`:
   ```bash
   git checkout -b feature/your-feature-name
   ```
3. **Make your changes** following our coding standards
4. **Add tests** for new functionality
5. **Run the test suite**:
   ```bash
   make test
   ```
6. **Run linting**:
   ```bash
   make lint
   ```
7. **Commit your changes** with a clear commit message
8. **Push to your fork** and create a pull request

## Development Setup

### Prerequisites

- Go 1.22 or later
- golangci-lint (install with `make install-tools`)

### Getting Started

1. **Clone the repository**:
   ```bash
   git clone https://github.com/blackorder/reloader.git
   cd reloader
   ```

2. **Install dependencies**:
   ```bash
   make deps
   ```

3. **Install development tools**:
   ```bash
   make install-tools
   ```

4. **Run tests**:
   ```bash
   make test
   ```

## Coding Standards

### Code Style

- Follow standard Go formatting (`go fmt`)
- Use meaningful variable and function names
- Add comments for exported functions and types
- Keep functions focused and reasonably sized

### Testing

- Write tests for all new functionality
- Maintain or improve code coverage
- Use table-driven tests where appropriate
- Include both positive and negative test cases

### Documentation

- Update README.md for new features
- Add inline documentation for exported functions
- Include examples for complex functionality

## Pull Request Guidelines

### Before Submitting

- [ ] Tests pass locally
- [ ] Code is properly formatted
- [ ] Linting passes
- [ ] Documentation is updated
- [ ] Commit messages are clear

### Pull Request Description

Include:
- **What** - Brief description of changes
- **Why** - Motivation for the changes
- **How** - Implementation approach (if complex)
- **Testing** - How you tested the changes

### Review Process

1. **Automated checks** - CI must pass
2. **Code review** - At least one maintainer approval
3. **Testing** - Verify changes work as expected
4. **Merge** - Squash and merge preferred

## Release Process

Releases are handled by maintainers:

1. Version bump following [Semantic Versioning](https://semver.org/)
2. Update CHANGELOG.md
3. Create and push git tag
4. GitHub Actions automatically creates the release

## Getting Help

- **Documentation** - Check the README and inline docs
- **Issues** - Search existing issues or create a new one
- **Discussions** - Use GitHub Discussions for questions

## Recognition

Contributors will be acknowledged in:
- Release notes
- README.md contributors section (if significant contribution)

Thank you for contributing! 🎉
