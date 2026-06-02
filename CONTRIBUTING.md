# Contributing to nettools

Thank you for your interest in contributing to nettools! We welcome contributions from everyone.

## How to Contribute

### Bug Reports

If you find a bug, please open an issue with:

- A clear description of the problem
- Steps to reproduce
- Expected vs. actual behavior
- Your environment (OS, Go version, network setup)

### Feature Requests

Open an issue describing the feature, the use case it serves, and any implementation ideas you have.

### Pull Requests

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/my-feature`)
3. Make your changes
4. Ensure all tests pass: `make test`
5. Run linting: `make check`
6. Commit with a clear message
7. Open a pull request

### Development Setup

```bash
# Clone the repo
git clone https://github.com/baidu/nettools.git
cd nettools

# Install dependencies
make prepare

# Install dev tools (golangci-lint, etc.)
make install-tools

# Run tests
make test

# Run all checks
make check
```

### Code Style

- Follow standard Go conventions and [Effective Go](https://go.dev/doc/effective_go)
- Run `make fmt` before committing
- All exported types and functions must have godoc comments
- Write tests for new functionality

### Commit Messages

- Use clear, descriptive commit messages
- Reference issue numbers when applicable (e.g., "Fix #123: ...")

## License

By contributing to nettools, you agree that your contributions will be licensed under the [MIT License](LICENSE).
