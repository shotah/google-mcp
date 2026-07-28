# Agent / contributor notes

## LLM-friendly tool descriptions (MCP)

Each tool’s `mcp.WithDescription(...)` string is what models see when choosing tools. Write descriptions for routing user intent, not API docs.

1. **What it returns** — one short phrase on payload shape (ids, titles, body text, URLs, etc.).
2. **`Use for …`** — natural phrases users say (quoted or paraphrased).
3. **Disambiguation** — when tools overlap, add **Prefer X** / **Not for Y — use Z** with the other tool’s MCP name.
4. **Required ids** — call out critical params in the Long text when models often forget them (`spreadsheet_id`, `tasklist_id`, `document_id`, `event_id`, …).
5. **Destructive / send actions** — note **confirm when unclear** (delete, share, send message, ownership transfer).

Keep descriptions under ~400 characters when practical. Put argument details in parameter `mcp.Description`, not duplicated Args:/Returns: blocks in the tool Long string.

Reference examples: `tools/calendar.go`, `tools/gmail.go`.
