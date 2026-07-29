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
      "args": ["--tools", "gmail calendar tasks", "--tool-tier", "core", "--capability", "edit"]
    }
  }
}
```

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

## Next: agent clarity (most → least impactful)

For small local models (e.g. Qwen 35B). Pick what to implement; order is recommended priority.

1. **Starve the tool catalog on the host** — Prefer `--tools gmail calendar --tool-tier core --capability edit` (~9 tools) over full core (~45). Add drive/tasks only when the persona needs them. Biggest win; mostly config, not code.
2. **Host-side intent recipes** — In ai-gantry / persona `TOOLS.md`, pin 8–10 intents → exact tool + args. Small models route from examples better than from long schemas. Consumer-side.
3. **Split overloaded tools** — e.g. `calendar_list_events` (range list) vs `calendar_get_event` (requires `event_id`). Removes `event_id="primary"` class failures. Same pattern anywhere an optional id branches behavior.
4. **Auth is user setup + silent refresh (not an agent tool)** —
   - **First token (human / CLI):** expose something like `google-mcp auth` (or `login`) so the user runs OAuth once, gets `~/.google_workspace_mcp/credentials/{email}.json`, and can copy that file onto the agent host. Document this in the README (today README still says “ask the assistant to call `auth_start`” — replace that).
   - **Runtime (MCP):** validate/refresh credentials before API calls; do not require the model to call `auth_start`. Drop `auth_start` from lean surfaces (keep only if useful as a rare re-auth escape hatch).
5. **`--preset` for a lean everyday surface** — One flag that selects services + tier + capability (e.g. gmail+calendar+tasks, core, edit). Name the *surface* (`lean` / `everyday` / `minimal`), not “agent” — MCP is always agent-facing. Auth is CLI setup, not part of the preset tool list.
6. **Teach in error messages** — Shorten hot-path descriptions; on bad args return the exact next call (tool name + required params). 35B often skips long description text under tool pressure.
7. **Trim optional params on hot tools** — e.g. drop/hide `detailed` / `include_attachments` noise on everyday calendar list so schemas stay small.
8. **MCP readOnly / destructive annotations** — If the host surfaces them; secondary to count and split tools.

**Deprioritize:** dual/alias old names; longer descriptions; loading all 12 services “just in case.”

## Checklist

- [x] Full old→new map (`scripts/rename_tools.py`)
- [x] Rename MCP `AddTool` names + description cross-links
- [x] Update `tiers.go` / capability sets
- [x] Comment tools use `{service}_*_comment*`
- [x] Handler / mock / tier tests
- [x] README + AGENTS.md
- [x] MCP `ServerName` = `google`
- [ ] Announce breaking change in GitHub Release notes (on next release)
