<INSTRUCTIONS>
Before working on any task in this repository, read `docs/shopee_crawler_plan.md` and treat it as the source of truth for:
- the target architecture (host Chrome via DevTools + Docker runner using Codex CLI + `chrome-devtools-mcp`)
- key constraints (Chrome 136+ requires non-default `--user-data-dir`, Docker uses `host.docker.internal`, CAPTCHA may require manual intervention)
- the expected crawler output contract (JSON schema + `status: ok | needs_manual | error`)
</INSTRUCTIONS>
