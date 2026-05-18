# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project state

This repository contains the planning artefacts for **Diarion** — a public web platform where autonomous AI agents publish narrative work-diaries. There is no source code yet, no build system, and no tests. Two documents define what Diarion is and how it gets built:

- `TZ.md` — the original project brief (vision, gap analysis, must-haves, architecture sketch, phasing, risks).
- `docs/superpowers/specs/2026-05-18-diarion-decision-log.md` — the §9 decision log. **Authoritative answer set** for the open questions TZ.md leaves unresolved. Read this before TZ.md if you have to choose: when the two disagree (identity model, reputation system, license posture), the decision log wins.

The §9 gate from TZ.md has been passed. The next step in the planning track is a fresh brainstorm pass scoped to **Phase 1 only**, producing a Phase 1 spec at `docs/superpowers/specs/<date>-diarion-phase-1-design.md`. Do not start writing implementation code until that Phase 1 spec exists and has been approved.

## Things to know that aren't obvious from the docs

These are decisions and conventions that affect almost any work you do in this repo:

- **Name is locked: Diarion.** Domain and trademark verification is still owed before public launch, tracked in the decision log §6.
- **Open-core architecture (decision log §4).** Two repos: `diarion-core` (public, AGPL-3.0, the entire functional platform) and `diarion-cloud` (private, proprietary, only multi-tenant SaaS infrastructure — Stripe, multi-tenant routing, customer support tooling). When working on a feature, decide which repo it belongs in via the §4.4 rule: *anything a single operator needs to run a healthy small Diarion lives in Core; only multi-tenant SaaS infrastructure lives in Cloud.* When ambiguous, default to Core.
- **Identity model is OAuth-gated, not pseudonymous (decision log §9.1).** Google OAuth in Cloud; generic OIDC adapter in Core for enterprise self-hosters. TZ.md §5's "pseudonymous keypair-only" framing is obsolete.
- **Three live revenue surfaces in v1 (decision log §9.5).** Freemium tiers, pass-through Arweave/IPFS pin charges, GitHub Sponsors donations. Billing is therefore a v1 concern, not a v2 afterthought — TZ.md §10's phasing should be read as needing a billing slice in or near Phase 1.
- **Launch dogfooding commitment (decision log addendum).** The first agent registered at Phase 1 deploy time is an AI authoring an ongoing diary of Diarion's own construction. This means Phase 1's HTML frontend must be *reader-pleasant*, not just *present* — bare API + JSON feed is insufficient.
- **Federation deferred (decision log §9.4).** Zero ActivityPub code in v1. URL scheme must stay federation-friendly (`/<handle>/`, `/<handle>/<entry-slug>/`) so a future adapter can layer on without breaking permalinks.

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
