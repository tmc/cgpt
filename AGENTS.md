# Repository Guidelines

## Project Structure & Module Organization
CLI entry points live in `cmd/cgpt`, while reusable libraries stay in the module root (for example `history_*.go`, `options.go`, `config.go`). Provider adapters split between `backends/` and `providers/`; keep new integrations narrow and package-local. Fixtures sit in `testdata/`, `cgpt-test/`, and `cgpt-profiling/`, documentation belongs in `docs/`, and automation scripts stay in `scripts/`. Prefer adding new leaf packages rather than growing broad ones.

## Build, Test, and Development Commands
The fast path during development is `go test ./...` and `go build ./cmd/cgpt`. `make build` produces the `cgpt` binary, while `make build-race` enables the race detector. `make test`, `make test-short`, and `make test-race` wrap the matching `go test` invocations. `make lint` runs `go fmt` followed by `go vet`, and `make dev` chains lint, short tests, and build. Regenerate coverage artifacts with `make coverage`.

## Coding Style & Naming Conventions
Format with `gofmt` (`make fmt`) before sending a change. Follow the standard Go style guide: mixedCase identifiers, short receiver names, and `err` for error values. Keep packages focused and avoid cyclical imports; a file named `history_*` or `retry_*` should cover one concern. Tests belong in `_test.go` next to the code they cover, and exported symbols need doc comments that start with the identifier.

## Testing Guidelines
Unit tests use the stock `testing` package; integration flows live in `integration_test.go` under the `TestIntegration_` prefix. For quick feedback run `make test-short`; before publishing, run `make test` or `make test-integration` if you touched history, backends, or provider wiring. Benchmarks (`go test -bench .`) reside alongside the code and assume deterministic fixtures in `testdata/`; prefer table-driven tests.

## Commit & Pull Request Guidelines
Recent history shows concise, scope-prefixed commit summaries (`history: implement atomic file writes`). Match that format, write in the imperative, and keep unrelated work in separate commits. Every pull request should include the problem statement, validation notes (for example `make test-race`), and relevant issue links. Attach screenshots only for UI-facing changes and re-run `make lint` plus the right test target before requesting review.

## Security & Configuration Tips
Sensitive keys load from environment variables or `config.yaml`; never hard-code secrets or embed provider tokens in fixtures. Before sharing diagnostic logs, redact request and response bodies under `providers/`. When adding configuration fields, update `CONFIGURATION.md`, `example.config.yaml`, and the checks in `config.go` in the same change.
