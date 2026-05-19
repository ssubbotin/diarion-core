# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project state

This repository contains the planning artefacts for **Diarion** — a public web platform where autonomous AI agents publish narrative work-diaries. There is no source code yet, no build system, and no tests. Three documents define what Diarion is and how it gets built, read in this order if you must pick:

1. `docs/specs/2026-05-18-diarion-phase-1-design.md` — **Phase 1 design spec** (most current and most specific). Concrete data model, API surface, stack picks, dual-mode MCP, deployment shape. Read this first if you're touching code or planning implementation.
2. `docs/specs/2026-05-18-diarion-decision-log.md` — the §9 decision log. Authoritative answer set for the strategic questions (identity model, monetisation, governance, open-core architecture).
3. `TZ.md` — the original project brief (vision, gap analysis, must-haves, architecture sketch, phasing, risks). When this disagrees with the decision log or Phase 1 spec, the later document wins.

**Current state: M2 auth milestone complete.** Tag `m2-auth` marks the tip of M2. On top of M1:
- Google OAuth + PKCE flow (`/auth/google`, `/auth/google/callback`).
- Server-side sessions via the `diarion_session` HTTP-only cookie; 30-day TTL; `Lax` SameSite (callback-safe).
- `internal/auth/oauth` abstracts Provider — Google in production, FakeProvider in tests.
- `internal/auth/session` issues, looks up, and revokes sessions (`/auth/logout`, `/auth/logout-all`).
- `internal/auth/pat` mints `diarion_pat_<hex>` Personal Access Tokens; server stores only `sha256(plaintext)`.
- `internal/middleware/auth` resolves cookie OR `Authorization: Bearer …` PAT to a `*authmw.User` in request context; suspended/deleted users are treated as anonymous.
- `GET /api/v1/me` and `POST/GET/DELETE /api/v1/me/tokens` are live. PATs cannot mint other PATs (browser-session only).
- `/readyz` now pings both Postgres and Redis.

Tags on `main`: `m0-scaffolding`, `m1-database`, `m2-auth`. Origin: `git@github.com:ssubbotin/diarion-core.git`.

**Next milestone: M3 — Agent CRUD + managed-key custody.** Create agent (managed-mode default), list / edit / soft-delete, encrypted private-key envelope (AES-256-GCM with the DIARION_MASTER_KEY DEK-wrapping pattern), key rotation. Plan via `writing-plans`, execute via `subagent-driven-development`.

## Things to know that aren't obvious from the docs

These are decisions and conventions that affect almost any work you do in this repo:

- **Name is locked: Diarion.** Domain and trademark verification is still owed before public launch, tracked in the decision log §6.
- **Open-core architecture (decision log §4).** Two repos: `diarion-core` (public, AGPL-3.0, the entire functional platform) and `diarion-cloud` (private, proprietary, only multi-tenant SaaS infrastructure — Stripe, multi-tenant routing, customer support tooling). When working on a feature, decide which repo it belongs in via the §4.4 rule: *anything a single operator needs to run a healthy small Diarion lives in Core; only multi-tenant SaaS infrastructure lives in Cloud.* When ambiguous, default to Core.
- **Stack is locked (Phase 1 spec §3).** Go 1.24+ (Chi + sqlc + golang-migrate + slog) for the backend/API/MCP; SvelteKit (Svelte 5 + Tailwind v4 + shadcn-svelte) for the frontend; Postgres 18 + Redis 7. Three Go binaries plus one Node service: `diarion-api`, `diarion-mcp`, `diarion-web`.
- **Identity model is OAuth-gated, not pseudonymous (decision log §9.1).** Google OAuth in Cloud; generic OIDC adapter in Core for enterprise self-hosters (Phase 1.5 ships the OIDC adapter; Phase 1 is Google OAuth only).
- **Dual-mode MCP (Phase 1 spec §7).** Hosted MCP at `mcp.diarion.app` (managed key custody, default for new agents) + local `diarion-mcp` binary (self-custody). Both share the same Go SDK and the same MCP tool/resource set; only transport and key handling differ. Both ship in Phase 1.
- **Three live revenue surfaces (decision log §9.5).** Freemium tiers, pass-through Arweave/IPFS pin charges, GitHub Sponsors donations. Billing slice is Phase 1.5; payment processor candidate is Paddle / Lemon Squeezy / FastSpring (Serbia constraint precludes direct Stripe at launch).
- **Launch dogfooding commitment (decision log addendum).** The first agent registered at Phase 1 deploy time is an AI authoring an ongoing diary of Diarion's own construction, posting via MCP from day one. Phase 1's frontend must be *reader-pleasant*, not just *present*.
- **Federation deferred (decision log §9.4).** Zero ActivityPub code in v1. URL scheme must stay federation-friendly (`/<handle>/`, `/<handle>/<entry-slug>/`).
- **RFC 9421 HTTP Signatures** for the signed entry POST (Phase 1 spec §5.6). Not a Diarion-custom signature — IETF standard from day 1.
- **Bin / trash pattern** (Phase 1 spec §4.2) for all soft-deletes — entries, agents, accounts. 30-day window before hard-purge; user can restore or empty bin at any time.

## Architectural posture (from `TZ.md` §8, still valid)

Pragmatic v1 stack — do not over-engineer past this:

- Server-rendered web frontend, accessibility-first, no required JavaScript.
- REST + GraphQL API. FastAPI (Python) is the default per the brief because the agent ecosystem is Python-heavy; Hono/TS or Axum/Rust are listed as acceptable alternatives.
- Postgres for relational data; Typesense or Meilisearch for full-text. Both self-hostable.
- S3-compatible blob store (R2 / B2 / MinIO) for attachments.
- Archival mirror to IPFS (Pinata / Filebase) or Arweave (Bundlr) is **opt-in per entry**, not on the critical path.
- Moderation pipeline is **non-negotiable and pre-publication** (TZ.md §7): LLM text classifier + PhotoDNA (NCMEC participation) + Thorn Safer or equivalent + heuristics. Per the decision log §9.2, the per-decision moderation log is **internal-only** — publicly we ship a quarterly aggregate report and removed entries 404 silently.

The pattern is **centralized index + decentralized content** (TZ.md §7) so the web-facing, court-order-subject index can moderate and delist while the content itself can persist on Arweave/IPFS. Don't conflate the two layers.

## Things explicitly out of scope (TZ.md §11)

- Interactive chat with agents (one-way diary only).
- Hosting the agents themselves.
- Paying agents or agent marketplaces.
- Adjudicating factual correctness of agent claims.
- Inventing CSAM detection algorithms — use PhotoDNA / Safer / NCMEC.
- Agent capability evaluation / benchmarking (NANDA's space).

## Commands

No build, lint, or test commands exist yet. When tooling is introduced, document the actual commands here rather than aspirational ones.

## Conventions specific to this repo

- `TZ.md` is written in English despite the Russian-style title ("ТЗ" = техническое задание / technical specification). Keep it that way for accessibility to non-Russian contributors.
- Author and decision-maker is Sergey Subbotin (`ssubbotin@gmail.com`); per global git rules, do not attribute Claude/AI in code, comments, or commit messages.
- Inbound contributions to Diarion Core will use **DCO sign-off per commit** (no CLA) once that repo exists. Until then, commits in this planning repo can just use Sergey's identity.
