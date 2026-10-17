# Repository Guidelines

## Project Structure

The Go executable starts in `cmd/runwisp-manager`; application packages live in `internal/` (API, auth, config editing, Git, UI serving, and related features). The SvelteKit frontend is in `web/src`, with static assets in `web/static`. Its production build is copied into `internal/ui/dist` for embedding. Deployment examples are in `deploy/`, operational documentation is in `docs/`, and phase notes at the repository root describe implementation history.

## Build, Test, and Development

- `make build` builds the Go manager binary as `runwisp-manager`.
- `make run` runs the manager locally with `go run`.
- `make test` runs Go tests and the frontend Svelte check (`npm run check`).
- `make lint` runs `go vet` and, when installed, `golangci-lint`.
- `make web` installs frontend dependencies, checks and builds the frontend, then refreshes the embedded UI files.
- `make parity` runs the frontend compatibility parity script.
- For browser integration, use `make compose-test-up`, `make e2e`, and `make compose-test-down` in that order. The compose commands require Docker.

## Coding Style and Naming

Format Go changes with `gofmt`; keep packages focused under `internal/` and follow existing Go naming conventions. Frontend code uses Svelte 5, JavaScript/TypeScript modules, and the existing project formatting and component patterns. Use descriptive lower-case filenames consistent with neighboring files. Keep generated frontend output in sync through `make web` when changing the UI.

## Testing

Go tests are colocated with packages in `*_test.go` files and run through `make test` (or `go test ./cmd/... ./internal/...`). Frontend static/type checks run through `npm run check` in `web/`; browser tests use Playwright via `make e2e`. Add or update focused tests alongside behavioral changes. No repository-wide coverage threshold is configured.

## Commits and Pull Requests

Recent commits use Conventional Commit-style subjects, commonly `feat: ...` and scoped phase descriptions. Use a concise imperative summary (for example, `fix: handle missing daemon socket`). Pull requests should explain the behavior change, note relevant configuration or deployment impacts, link related issues, and include screenshots for visible UI changes. Mention checks performed and any follow-up work.

## Configuration and Security

Do not commit credentials or production configuration. Use `deploy/compose/runwisp.env.example` as a template and follow `SECURITY.md` and `docs/deployment.md` for authentication, permissions, and secret handling.
