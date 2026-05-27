# Contributing

Thanks for considering a contribution to `green-api-mcp-gateway`.

## Quick start

```bash
git clone https://github.com/green-api/green-api-mcp-gateway.git
cd green-api-mcp-gateway
make lint test build
```

Requires Go 1.25+ and `golangci-lint`.

## Pull requests

1. Open an issue first if the change is non-trivial — it's easier to align on direction before code is written.
2. Use [Conventional Commits](https://www.conventionalcommits.org/) for commit messages (`feat:`, `fix:`, `docs:`, `chore:`, `refactor:`, `test:`).
3. Keep PRs focused — one concern per PR.
4. Include tests for new behaviour. Run `make test` locally before pushing.
5. CI must be green before merge.

## Reporting issues

Please include:

- What you expected to happen
- What actually happened
- Steps to reproduce (minimal config / commands)
- Output of `./green-api-mcp-gateway --version` (or commit SHA if built from source)
- Relevant logs with secrets redacted

See `.github/ISSUE_TEMPLATE/` for bug-report and feature-request templates.
