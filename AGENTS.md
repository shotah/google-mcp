# Agent / contributor notes

## Tool naming (required)

Every MCP tool name is **service-first**:

```text
{service}_{verb}_{object}[_{qualifier}]
```

Examples: `calendar_list_events`, `gmail_search_messages`, `chat_send_message`, `tasks_list_tasks`.

Rules:

1. **Service first** — one of: `gmail`, `calendar`, `drive`, `docs`, `sheets`, `slides`, `tasks`, `contacts`, `chat`, `forms`, `appscript`, `search` (auth is `auth_start`).
2. **No bare verbs** — never `get_events`, `send_message`, `list_tasks`.
3. **Server id** — MCP `ServerName` is `google`. Hosts expose `google__{tool}`; do not put `google__` inside the tool name.
4. **Agent clarity > Python parity** — do not rename tools back to match the Python server.

The old→new map used for the rename lives in `scripts/rename_tools.py`.

## LLM-friendly tool descriptions (MCP)

Each tool’s `mcp.WithDescription(...)` string is what models see when choosing tools. Write descriptions for routing user intent, not API docs.

1. **What it returns** — one short phrase on payload shape (ids, titles, body text, URLs, etc.).
2. **`Use for …`** — natural phrases users say (quoted or paraphrased).
3. **Disambiguation** — when tools overlap, add **Prefer X** / **Not for Y — use Z** with the other tool’s **current** MCP name (service-first).
4. **Required ids** — call out critical params in the Long text when models often forget them (`spreadsheet_id`, `tasklist_id`, `document_id`, `event_id`, …).
5. **Destructive / send actions** — note **confirm when unclear** (delete, share, send message, ownership transfer).

Keep descriptions under ~400 characters when practical. Put argument details in parameter `mcp.Description`, not duplicated Args:/Returns: blocks in the tool Long string.

Reference examples: `tools/calendar.go`, `tools/gmail.go`.
