# Legal Disclaimer

This document describes the limits of what kvelmo does and does not do, the responsibilities you take on by running it, and the categories of use it is and is not designed for.

The code license is [BSD 3-Clause](LICENSE). This document does not modify the license; it explains operational context that the license alone does not cover.

## Project Nature

kvelmo is a local development orchestrator. It runs entirely on your machine. There is no hosted kvelmo service, no kvelmo cloud, and no account system. kvelmo's maintainers do not have access to your tasks, your code, your recordings, or your API keys.

The software is provided **as is**, without warranty of any kind, as stated in the LICENSE. Running an AI agent against your codebase carries risks that even careful software cannot eliminate. The sections below describe those risks so you can decide whether and how to use kvelmo.

## AI Model Risks

kvelmo orchestrates AI agents. AI agents fabricate. They produce plausible-looking code that does not compile, plausible-looking commits that misstate intent, and plausible-looking reasoning that does not match what the code actually does.

kvelmo's workflow exists specifically to make these failures catchable:

- The `plan` phase produces a written specification before code changes begin.
- The `implement` phase records what the agent did via the event log and checkpoint system.
- The optional `simplify` and `optimize` phases give the agent another pass with fresh context.
- The `review` phase is a human checkpoint. It is not optional in spirit, even when skippable in flags.
- The `submit` phase is the last point at which work is still local.

**You are responsible for reviewing agent output before it leaves your machine.** Treat kvelmo like a junior engineer with infinite confidence: useful, but every PR needs a human in the loop. The maintainers explicitly disclaim liability for AI-generated content that you ship.

## Third-Party Compliance

kvelmo integrates with services it does not control. Using kvelmo with these services does not grant you any rights you would not otherwise have:

- **Task providers** — GitHub, GitLab, Linear, Wrike, Jira, Azure DevOps. You are responsible for complying with each provider's terms of service, rate limits, and acceptable-use policies. kvelmo passes through the tokens you give it; it does not negotiate or interpret your contracts with these providers.
- **AI model providers** — Anthropic, OpenAI, Ollama, and any custom endpoint you configure. You are responsible for each provider's usage policies, billing terms, content policies, and rate limits.
- **Agent CLIs** — Claude CLI, Codex CLI, and any custom CLI you configure. kvelmo subprocesses these binaries through their documented interfaces. You are responsible for installing, authenticating, and updating them, and for honoring their respective terms.
- **Browser automation** — `internal/browser/` and Playwright-driven flows automate sites you direct them to. You are responsible for ensuring the targets permit automation and that you have the right to access them.

If a provider changes its terms or rate limits in a way that breaks kvelmo, that is between you and the provider. kvelmo will adapt where it can.

## Acceptable Use

kvelmo is designed for the following kinds of work:

- Assisted development on repositories you own or have contributor access to
- Automated quality gates, security scans, and CI checks on your own projects
- Local experimentation with AI-assisted refactors, planning, and code review
- Generating PRs that a human reviews before merge
- Orchestrating agent work across multiple repositories that you control

## Unacceptable Use

kvelmo is not designed for, and the maintainers do not endorse:

- **Auto-merging unreviewed agent output** into production repositories or any branch protected by a review policy. The `review` phase exists for a reason.
- **Bypassing branch protection, CODEOWNERS, or required-review settings** through any combination of kvelmo features, agent automation, or provider tokens.
- **Exfiltrating secrets, source code, or proprietary data** by aiming kvelmo at repositories you do not have permission to access, or by using `agent/recorder/` outputs as a channel for sensitive material.
- **Automating against sites or APIs that prohibit automation**, including but not limited to scraping rate-limited public sites at speed, automating logged-in flows on platforms whose ToS forbid bots, or using browser automation to circumvent paywalls or access controls.
- **Submitting unreviewed agent-generated content** to bug bounty programs, open-source projects, customer-facing systems, or anywhere a human would reasonably expect a human had reviewed the work.
- **Producing or processing content that violates law** in your jurisdiction or the jurisdictions of the services involved.

The maintainers reserve the right to refuse support, decline contributions, and decline to fix bugs for usage that falls in this category.

## Financial Responsibility

AI model usage costs money. Provider API calls cost money. Browser automation against rate-limited services can cost money in throttling or bans.

**You are responsible for all costs incurred by kvelmo on your behalf.** kvelmo does not enforce spending caps. The `metrics/` package tracks token consumption per agent so you can monitor usage, and provider-specific commands expose their own rate-limit awareness, but there is no built-in budget enforcer.

If you are deploying kvelmo in a context where API costs are charged to a third party — a client, an employer, a team budget — make sure they understand and approve the usage pattern before you start.

## Warranty and Liability

The BSD 3-Clause License governs warranty and liability. In summary: there is no warranty, and the copyright holders are not liable for damages arising from use of the software.

This document does not alter, expand, or constrain those terms. It exists to provide context for the categories of risk that operating an AI orchestrator entails, so that "as is" is not a surprise.

## Related

- [`LICENSE`](LICENSE) — BSD-3-Clause warranty and liability terms
- [`DATA_CONTRACT.md`](DATA_CONTRACT.md) — what kvelmo collects, where it lives, what is transmitted
- [`SECURITY.md`](SECURITY.md) — vulnerability disclosure
- [`TRADEMARK.md`](TRADEMARK.md) — brand usage
- [`CODE_OF_CONDUCT.md`](CODE_OF_CONDUCT.md) — community conduct
