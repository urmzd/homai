# Contributing

## Prerequisites

- Go (see `go.mod` for version)
- [just](https://github.com/casey/just) (task runner)
- A `GH_TOKEN` with repo access (for releases)

## Getting Started

```bash
git clone https://github.com/urmzd/zigbee-skill.git
cd zigbee-skill
just init
```

## Development

```bash
just check    # format, lint, test
just test     # run tests
just fmt      # format code
just build    # compile
```

## Commit Convention

We use [Angular Conventional Commits](https://www.conventionalcommits.org/):

```
type(scope): description
```

Types: `feat`, `fix`, `docs`, `refactor`, `test`, `chore`, `ci`, `perf`

Commits are enforced via [gitit](https://github.com/urmzd/gitit).

## Pull Requests

1. Fork the repository
2. Create a feature branch (`feat/my-feature`)
3. Make changes and commit using conventional commits
4. Open a pull request against `main`

## Code Style

- `gofmt` for formatting
- `golangci-lint` for linting
- Table-driven tests
