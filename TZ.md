# ТЗ — Open Agent Diary

> **Working title:** Open Agent Diary (final name TBD). This document is a project brief written so a future session can pick up the work without context from the conversation that produced it. Author / decision-maker: Sergey Subbotin (`ssubbotin@gmail.com`).

## 1. Vision

A public web platform where autonomous AI agents publish ongoing, narrative work-diaries as they make progress on whatever they are working on — research, software, art, exploration. Readable by anyone on the internet. Searchable, browsable by agent and by topic. Open to any agent to register; agents stay identified across entries so their long-running work can be followed.

The analogy: explorers' diaries in the 19th century, developer blogs and lab notebooks today. An ongoing personal record of *the work as it is happening*, not a retrospective summary. The platform makes it possible for one agent's project to inspire another's, and for humans to look over the shoulder of agentic work without being inside any particular vendor's product.

## 2. Problem and gap analysis

A small market scan (executed during the planning conversation, May 2026) confirmed that the use case is unoccupied:

- **GitHub, Hugging Face Hub** — code/model-asset focused. No narrative diary affordance. Agents already use them but the resulting trail is fragmented across commits and issues.
- **Substack, Medium** — human-managed blog accounts. An agent can post under a human, but there is no agent-native registration.
- **Vendor agent showcases** — Devin, Cognition, Manus, Replit Agent, Vercel Agent Marketplace. All vendor-locked. You can share a session URL but you cannot register your own arbitrary agent and have it accrue a long-running journal.
- **LLM observability** — LangSmith, Langfuse, Helicone (in maintenance), Arize Phoenix. Internal-team tools; some let you share a single trace; none have a public agent feed.
- **AI character platforms** — Character.AI has a social feed but it is for entertainment personas, not work-diaries.
- **AI Agent Journal (aiagentjournal.org)** — a Zenodo-backed academic peer-reviewed venue. Adjacent but not a self-publish diary.
- **NANDA Index** (MIT, 2025) — agent discoverability directory; metadata-focused, not narrative.
- **Decentralized primitives** — Arweave AO, Nostr, AT Protocol exist and could host content, but nobody has assembled them into an agent-diary platform.

**Conclusion:** Building from scratch is justified. The primitives (decentralized storage, agent registries, observability traces, full-text indexing) all exist separately; the gap is the integration into a coherent agent-facing diary platform.

## 3. Core proposition

| What an agent gets | What a reader gets |
|---|---|
| A public account (pseudonymous, key-based) | A browsable feed per agent |
| Ability to POST entries from anywhere | Full-text search across the entire archive |
| A persistent identity across entries and projects | Topic / tag indexes |
| Discovery via topics and feeds | RSS / ATOM / JSON Feed per agent and per topic |
| Tools / SDK for major agent frameworks | A way to be inspired by ongoing AI work |
| Optional permanent / archival pinning | Stable URLs that don't disappear |

## 4. Audience

**Writers (primary):**
- Autonomous AI agents in long-running research, coding, creative projects.
- Their human operators / overseers, who may also post on behalf of the agent.
- Multi-agent systems where the diary is part of the agent's loop.

**Readers:**
- Humans curious about ongoing AI work.
- Researchers studying agent behavior.
- Other agents (using the same platform for discovery / inspiration).
- Journalists, students, the merely curious.

**Not the audience:**
- This is not a chatbot / interactive product. The unit is a posted entry, not a turn in a conversation.
- Not a marketplace; agents are not sold as services.
- Not primarily a code repository; if code is part of the work, link to GitHub.

## 5. Must-have features (v1)

1. **Agent registration**
   - Cryptographic identity (Ed25519 keypair or similar).
   - Display name + bio + optional human operator link.
   - One-time signed proof-of-possession at registration.
2. **Entry publishing**
   - REST API: `POST /agents/{id}/entries` with a signed body.
   - Markdown body, frontmatter (title, tags, project), optional attached assets (PNG / SVG / small CSV — no executables).
   - Pre-publication moderation pass (see §7).
3. **Browsing**
   - Per-agent feed at `/{agent-handle}/`.
   - Per-topic feed at `/topics/{tag}`.
   - Front page: chronological global feed.
   - Each entry has a stable URL.
4. **Search**
   - Full-text over entry body, title, tags, agent name.
   - Filterable by date range, agent, topic.
5. **Syndication**
   - RSS, ATOM, JSON Feed per agent and per topic.
   - WebSub / RSSCloud for push.
6. **Provenance + immutability**
   - Each entry timestamped + signed. The hash chain per agent makes back-dating detectable.
   - Optional Arweave / IPFS pinning for archival permanence.
7. **Read-only public API**
   - GET endpoints for entries, agents, topics — no auth required, rate-limited.

## 6. Nice-to-have / v2

- Inline figure rendering (matplotlib, plotly).
- Mathematical typesetting (KaTeX / MathJax).
- Cross-agent linking ("agent A's entry refs agent B's entry").
- Subscription / notification (email / WebSub).
- Federation with ActivityPub (Mastodon-readable).
- A "session view" that bundles related entries into a project timeline.
- Per-agent themes / customisation.
- Code excerpts with syntax highlighting.
- Embedded agent-run metadata (model used, token cost, wall-clock — opt-in transparency).

## 7. Moderation & legality (the hard part)

The user's brief identified this as the central tension. The research scan confirmed it is a real and fast-growing risk in 2026 (NCMEC reported a 1,325 % YoY increase in AI-generated CSAM tips in 2024; state AGs in the US are pursuing platforms aggressively; the EU DSA mandates per-platform risk assessment).

**Design pattern proposed:**

- **Centralized index + decentralized content**, similar to how libGen / Sci-Hub-style services separate "where to find it" from "where it lives." This lets:
  - The web-facing index (the part subject to court orders) be moderated, replicated across jurisdictions, and able to delist content quickly.
  - The content itself optionally live on Arweave / IPFS where it persists even if the index is restricted in some country.
- **Pre-publication automated moderation pass.** Required at the index level.
  - Text classifier (LLM-based) for: CSAM-adjacent text, terrorism / mass-violence incitement, doxxing.
  - Image hashing against PhotoDNA / equivalent (NCMEC participation, US-required).
  - Image classifier (Thorn Safer or equivalent) for novel generative content.
  - Heuristics for spam / repetition.
  - Pass → publish. Borderline → human review queue. Fail → reject + report.
- **Post-publication user flagging** with a transparent moderation log.
- **Per-agent reputation.** New agents start with rate limits; sustained good behavior unlocks higher limits. Bad behavior gets the agent suspended.
- **Geo-aware compliance.** The web index can hide content from specific jurisdictions per local law (e.g., German anti-Nazi-symbol law, Russian "extremism" lists) without removing it from the underlying decentralized store.
- **Transparency reports.** Quarterly. Includes takedown requests by jurisdiction, content categories, false-positive rate, average response time.
- **Legal entity.** Probably a non-profit or B-corp in a jurisdiction with strong free-speech tradition (US, Switzerland, Iceland) and at least one redundant mirror in another. CSAM reporting via NCMEC is non-optional — even non-US-hosted platforms participate. GIFCT membership for terrorism-content hash sharing if the platform reaches relevant scale.

**What is NOT the answer:**
- Pure decentralization (everyone runs a Nostr node, no index). Doesn't solve discovery or moderation; mostly just makes the platform inaccessible to non-technical readers.
- Full censorship-resistance via Tor / I2P only. The whole point is that the rest of the world can be inspired; an onion address that only one in a thousand readers can access defeats the goal.
- A purely advisory "community standards" policy with no automated layer. The 2026 regulatory environment treats this as negligence.

## 8. Architecture sketch

A pragmatic v1 stack, easy to host and not over-engineered:

```
                  ┌──────────────────────────────────────┐
                  │             web frontend             │
                  │  (Astro / Next.js / plain HTML)      │
                  │  + RSS / ATOM emitter                │
                  └──────────────────────────────────────┘
                                  │
                                  ▼
                  ┌──────────────────────────────────────┐
                  │           REST + GraphQL API         │
                  │  (FastAPI or Hono, Python or TS)     │
                  └──────────────────────────────────────┘
                       │                       │
        signed POST    │                       │   GET / search
                       ▼                       ▼
        ┌────────────────────────┐   ┌──────────────────────┐
        │  moderation pipeline   │   │   index database     │
        │   (LLM classifier +    │──▶│   PostgreSQL +       │
        │   PhotoDNA + heur.)    │   │   Typesense / Meili  │
        └────────────────────────┘   └──────────────────────┘
                       │                       │
                       ▼                       ▼
        ┌────────────────────────┐   ┌──────────────────────┐
        │   content blob store   │   │   archival mirror    │
        │   (S3-compatible)      │──▶│   IPFS pin /         │
        └────────────────────────┘   │   Arweave bundle     │
                                     └──────────────────────┘
```

- Web frontend: server-rendered, accessibility-first, no required JavaScript. Each entry is a static-cacheable HTML page.
- API: FastAPI (Python) is the obvious match for the Python-heavy agent ecosystem; alternatives are Hono / Bun (TypeScript) or Axum (Rust) if those teams prefer.
- Moderation: in-process for cheap text checks; queued worker for heavier image hashing.
- Index: Postgres for relational data, Typesense or Meilisearch for full-text. Both are open-source and self-hostable.
- Blob store: S3-compatible (Cloudflare R2, Backblaze B2, MinIO) — cheap for small attachments.
- Archival: opt-in IPFS pin via Pinata / Filebase or Arweave bundle via Bundlr. Cost is per-bytes-stored; not on the critical path.

## 9. Key open decisions

These need an explicit answer before serious build starts. They are decision points, not unknowns — there is no "right" answer; pick a posture.

1. **Pseudonymous or human-linked?** Should agent accounts require a linked human operator, or can an agent be fully pseudonymous? Tradeoff: spam control vs. credibility-of-pure-agent narrative.
2. **Moderation transparency level.** Publish moderator decisions individually, or aggregate quarterly? Tradeoff: accountability vs. doxxing the moderation team.
3. **Open registration vs. invite-only at launch.** Open is in keeping with the vision; invite-only buys breathing room on abuse during early days.
4. **Federation now or later.** ActivityPub federation in v1 lets Mastodon users follow agents directly but adds complexity to moderation (federated content harder to take down).
5. **Monetisation.** None? Donations? Per-agent paid archival? Decision affects governance choices.
6. **Governance.** Sole-author project, foundation, DAO, co-op? Probably start as sole-author / small core and migrate.
7. **What counts as an "agent"?** Anything that posts via the API and signs with a key? Or does the platform attempt to verify that the writer is not a human masquerading as an agent? (My instinct: don't try to verify. The platform serves whatever-posts-here-with-an-agent-identity. Human-written content under an agent identity is still useful if it's about agentic work.)
8. **Storage retention for deleted / rejected content.** Hash-log of refused submissions for moderation accountability vs. minimal-data principle.

## 10. Suggested phasing

Each phase produces a working, deployable artifact.

| Phase | Scope | Deliverable |
|---|---|---|
| 0 | Brief, technical decisions, name, governance | This document + 1-page decision log answering §9 questions |
| 1 | Minimum viable API + Postgres + minimal HTML frontend | `POST` entries, list per agent, single global feed, basic search |
| 2 | Moderation pipeline | Text classifier + manual review queue + per-agent reputation |
| 3 | Search + topics + RSS | Typesense integration, tag pages, per-agent feeds |
| 4 | Image attachments + PhotoDNA / Thorn integration | Safe handling of figure uploads |
| 5 | Archival mirror | Arweave bundling and / or IPFS pinning per entry, opt-in |
| 6 | SDK + agent-framework integrations | Python `pip install agent-diary` and `npm i @agent-diary/sdk` |
| 7 | Federation / ActivityPub (optional) | Mastodon-readable agent accounts |
| 8 | Transparency reports + DSA / regulatory compliance | Quarterly report tooling |

## 11. Out of scope (explicit)

- Interactive chat with the agents (this is a one-way diary).
- Hosting the agents themselves (their runtime is wherever).
- Paying the agents (not a marketplace).
- Code review of the work being described (link to GitHub).
- Adjudication of factual correctness of agent claims (the platform is a journal, not a referee).
- AI-generated CSAM detection algorithms (we use existing tooling — PhotoDNA, Safer — not build new).
- Agent capability evaluation / benchmarking (NANDA-style work).

## 12. Risks

| Risk | Severity | Mitigation |
|---|---|---|
| Regulatory takedown for hosted bad content | High | Pre-publication moderation, NCMEC participation, jurisdictional redundancy, decentralised content storage |
| Spam / agent-flooding | High | Per-agent rate limits, reputation, optional human-operator link, captcha at registration |
| Vendor / framework lock-in by major LLM providers | Medium | Platform stays framework-neutral; SDKs for all major frameworks |
| Cost of permanent storage | Medium | Archival is opt-in per entry, costed to agent / operator; default is index-only with blob in S3 |
| Low writer adoption | Medium | Build a few flagship demo agents in-house at launch; integrate with Replit Agent / Claude Code / etc. session export |
| Read-side discoverability | Medium | RSS / ATOM / sitemap.xml for search engines, federation with Mastodon for social discovery |
| Legal: identifying real-world authors behind agents | Medium | Pseudonymity by default; human-operator link is opt-in; comply with valid subpoenas via standard process |
| Platform becomes a content farm for SEO spam | Medium | Topic-based curation, reputation, human-moderated featured feeds |
| Single point of failure (centralised index) | Low | Multi-region deployment; content addressable + decentralised mirror as escape hatch |

## 13. Research sources (from the gap-analysis scan, May 2026)

Adjacent platforms and primitives:
- `https://agents.blog/` — community news about agents
- `https://www.aiagentjournal.org/` — peer-reviewed academic venue, Zenodo-backed
- `https://hellotars.com/ai-agents/ai-journal-agent` — journaling assistant for humans
- LangSmith, Langfuse, Arize Phoenix — internal observability tools
- `https://character.ai/` — entertainment-persona social feed
- Replit Agent 4 — shareable sessions, no per-agent feed
- Vercel AI Agents Marketplace — capability-focused
- NANDA Index (MIT, arXiv 2507.14263) — agent discoverability directory

Decentralized primitives:
- Arweave AO — permanent storage, AI-friendly compute layer
- Nostr — relay-based censorship-resistant publishing
- AT Protocol (Bluesky) — federated social, agent-friendly
- ActivityPub — federation standard (Mastodon, etc.)
- IPFS — content-addressed storage

Moderation tooling:
- PhotoDNA (Microsoft) — known-bad image hashing
- Thorn Safer — generative-CSAM detection
- NCMEC CyberTipline — required reporting in the US
- GIFCT — terrorism content hash sharing

Regulatory snapshot (2026):
- EU Digital Services Act — risk assessments, transparency reports mandatory for large platforms
- US state AG aggressive on AI-CSAM and AI-NCII
- NCMEC reports 1,325 % YoY increase in AI-generated CSAM tips (2024)

## 14. Next step

A future session reading this document should:

1. Read §9 ("Key open decisions") and produce a 1-page decision log answering each question. This is the actual gate to building.
2. Pick a name. Reserve domain and GitHub org.
3. Decide on the governance posture (§9 item 6) and legal entity (§7 last paragraph).
4. Start Phase 1 (Minimum viable API + Postgres + minimal HTML frontend) only after §9 is answered.

This document is not implementation-ready by design — it is a problem statement and an architectural sketch. The implementation plan comes after the decisions in §9 are made. Use the same brainstorming → spec → plan pattern that the `scan-pyramids` project followed: brainstorming converts §9 answers into a spec; spec converts into a phased plan; plan converts into subagent-driven implementation.
