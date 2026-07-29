# Tool & server naming overhaul

**Status:** implemented (breaking rename; migrate by upgrading the binary)
**Why:** Small local models and stronger agents mis-routed Workspace tools when names were
ambiguous (`get_events`, `send_message`, `list_tasks`) or when the MCP server id was a long
compound (`google-workspace`). Hosts expose `{server}__{tool}`, so both halves matter.

## Shipped end state

| Layer | Value |
| --- | --- |
| MCP server name (`ServerName`) | `google` |
| Binary / CLI | `google-mcp` |
| Tool pattern | `{service}_{verb}_{object}` |
| Host-facing examples | `google__calendar_list_events`, `google__gmail_search_messages` |
| Dual / alias registration | none — one name set per release |
| Python string parity | diverged; agent clarity wins |
| Go module / GitHub repo | `github.com/shotah/google-mcp` |

## Client config

```json
{
  "mcpServers": {
    "google": {
      "command": "google-mcp",
      "args": ["--preset", "everyday"]
    }
  }
}
```

- `everyday` = gmail+calendar+docs+sheets+tasks, core, edit ≈ 20 tools (personal assistant; **Drive usually not needed** for Sheets/Docs).
- `lean` = gmail+calendar only ≈ 10 tools (tiny models).

## Rename map

Canonical old→new map: [`scripts/rename_tools.py`](scripts/rename_tools.py) (`RENAMES`).

Highlights:

| Old | New |
| --- | --- |
| `get_events` | `calendar_list_events` |
| `create_event` / `modify_event` / `delete_event` | `calendar_create_event` / `calendar_update_event` / `calendar_delete_event` |
| `send_message` | `chat_send_message` |
| `get_messages` | `chat_list_messages` |
| `search_gmail_messages` | `gmail_search_messages` |
| `list_tasks` / `get_task` | `tasks_list_tasks` / `tasks_get_task` |
| `start_google_auth` | `auth_start` |
| `read_document_comments` | `docs_read_comments` (same pattern for sheets/slides) |

## Consumer follow-ups (outside this repo)

- Point `mcp.toml` / Cursor config `name = "google"`
- Rewrite persona recipes from `google-workspace__create_event` → `google__calendar_create_event`
- Keep **math** / **strava** / **garmin** as separate short servers

## Done (agent-clarity foundations)

1. [x] Starve catalog via presets (`lean` / `everyday`) + filters
2. ~~Host-side intent recipes~~ — not in this repo (ai-gantry / persona)
3. [x] Split overloaded calendar list vs get
4. [x] Auth = CLI + silent refresh (`google-mcp auth`)
5. [x] Presets + Drive vs Docs/Sheets docs
6–8. [x] Teach-in errors, trim hot schemas, MCP annotations (`newMCPTool`)
9. [x] Lean docs + sheets core for `everyday`
10. [x] Lean tasks core + add `tasks` to `everyday`

## Next: production-ready agent UX (all services)

**Thesis:** Much of this surface looks ported from API coverage, not from watching a real agent fail. Every **core** tool (and hot extended ones) should pass the same bar we applied to calendar/gmail/docs/sheets.

### What `--tool-tier` means (not a mystery)

Defined in `tools/tiers.go`. It is **how deep** each *enabled* service goes — orthogonal to `--tools` (which services) and `--capability` (read/edit/complete).

| Tier | Meaning |
| --- | --- |
| `core` | Everyday path an assistant actually needs for that service |
| `extended` | Discovery / management (list-by-name, folders, filters, …) |
| `complete` | Rare / power-user / destructive-adjacent extras |

Presets always use `core` so small models never see the long tail unless you ask.

### Lean playbook (apply per service)

For each **core** tool:

1. **Short description** — what it returns + 1–2 Use-for phrases + Prefer/Not-for disambiguation (≤ ~400 chars).
2. **Minimal schema** — only params the happy path needs; drop formatting/power flags from schema (handlers may still accept them if sent).
3. **Teach-in errors** — `needArg` / `toolHint` with exact next call (`tool(required=…)`).
4. **Split arity** — if optional id changes list vs get, split tools (calendar pattern).
5. **Required ids called out** — `spreadsheet_id`, `tasklist_id`, `document_id`, …
6. **Destructive confirm** — in description when unclear; annotations already via `newMCPTool`.

Do **not**: load Drive “because Docs”; dual-alias old names; lengthen descriptions; put power tools in `core`.

### Remaining service audit (priority)

Order = personal-assistant impact first, then specialty surfaces.

| Priority | Service | Status | Notes |
| --- | --- | --- | --- |
| — | gmail, calendar, docs, sheets, tasks | **done (core)** | `everyday` includes tasks; use `task_list_id="@default"` |
| 11 | **drive** | **done (core)** | Lean search/get/create/share; not in everyday |
| 12 | **contacts** | **done (core)** | Search/get/list/create lean; batch delete capability-gated |
| 13 | **chat** | **done (core)** | space_id normalize + teach via `chat_list_spaces` |
| 14 | **slides** | **done (core)** | Create/get lean; batch_update stays extended |
| 15 | **forms** | **done (core)** | Create/get lean |
| 16 | **comments** | **done** | Short schemas; resolve destructive + confirm |
| 17 | **search** / **appscript** | **done (core)** | Specialty surfaces leaned; not in presets |

**Deprioritize:** dual/alias old names; longer descriptions; loading all 12 services “just in case.”

## Checklist

- [x] Full old→new map (`scripts/rename_tools.py`)
- [x] Rename MCP `AddTool` names + description cross-links
- [x] Update `tiers.go` / capability sets
- [x] Comment tools use `{service}_*_comment*`
- [x] Handler / mock / tier tests
- [x] README + AGENTS.md
- [x] MCP `ServerName` = `google`
- [x] `--preset lean` / `everyday` (+ tasks) + Drive vs Docs/Sheets guidance
- [x] `google-mcp auth` / `login` CLI + README (auth out of agent path)
- [x] Split `calendar_list_events` / `calendar_get_event`
- [x] Agent-UX lean overhaul — all services’ core tools
- [ ] Announce breaking change in GitHub Release notes (on next release)
