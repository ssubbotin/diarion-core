# Contributing to Diarion Core

Thank you for considering a contribution. Diarion Core is licensed under
AGPL-3.0. Diarion Cloud (the proprietary multi-tenant overlay) lives in a
separate, private repository; contributions in this public repo target Core.

## Developer Certificate of Origin (DCO)

All commits must be signed off with the Developer Certificate of Origin.
By signing off, you confirm that you wrote the contribution or otherwise
have the right to submit it under AGPL-3.0.

Add the trailer to every commit:

    Signed-off-by: Your Name <you@example.com>

Or simply use `git commit -s`. CI rejects pull requests with unsigned commits.

The full DCO text: <https://developercertificate.org/>.

## Workflow

1. Fork the repository.
2. Create a topic branch from `main`.
3. Make focused commits with DCO sign-off.
4. Open a pull request describing the change.
5. CI must pass: `golangci-lint`, `go test`, `govulncheck`, `npm audit`,
   `vitest`, and a `docker-compose`-driven smoke test.
6. Squash or rebase is fine — the maintainer will pick.

## Code conventions

- Go: formatted with `gofmt`; linted with `golangci-lint`; tested with
  `go test ./...`.
- SvelteKit (`web/`): formatted with `prettier`; linted with `eslint`;
  unit-tested with `vitest`; e2e-tested with `playwright`.
- Security-sensitive code (signing, sanitisation, auth) uses audited
  off-the-shelf libraries. No hand-rolled crypto, no hand-rolled HTML
  sanitisation.

## Reporting security issues

Do not open public issues for security vulnerabilities. Email
ssubbotin@gmail.com with details. We aim to acknowledge within 48 hours.

## Running tests

- `make test` — unit tests only, no Docker required.
- `make test-integration` — integration tests; spins up ephemeral Postgres
  containers via testcontainers-go (Docker must be running).
- CI runs both on every PR.
