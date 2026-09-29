# TODO

## This repo

Multi-account storage and lookup already work. See [docs/multi-account.md](docs/multi-account.md).

- [x] `auth_list_accounts` lists logged-in emails from credential filenames (no token material).
- [x] Stored and requested emails are lowercase, so `Ada@gmail.com` and `ada@gmail.com` hit the same file. Plus-addresses stay distinct.

## Other checkouts

Do not implement these here. The owning checkout should add them to its own todo list (`/home/deck/repos/gantree/docs/todo.md` for the dashboard, and ai-gantry's docs for the harness).

### ai-gantry

- [ ] Keep one persistent credential directory (`DATA_DIR` or `WORKSPACE_MCP_CREDENTIALS_DIR`) across restarts.
- [ ] Allow `/auth google` more than once. Each exchange writes a new `{email}.json` and leaves the others in place.
- [ ] After exchange, show the stored email in chat so the model can pass `user_google_email`.
- [ ] Leave `USER_GOOGLE_EMAIL` unset unless the operator chose a default account. Do not require it.
- [ ] Forward `user_google_email` on tool calls as the model sent it.
- [ ] Give each in-progress login its own PKCE pending file.

### gantree

- [ ] Provision one OAuth client id/secret per pendant, shared by every Google account on that pendant.
- [ ] Replace a single-account email field with a list of linked accounts (filenames only).
- [ ] Add account: run auth URL / exchange again.
- [ ] Remove account: delete that one credential file.
- [ ] Optional default account sets `USER_GOOGLE_EMAIL`. Empty default means every call must pass `user_google_email`.
- [ ] Store the credential directory on the data volume.

### Pendant and Cab

- [ ] Show "Signed in as …" from the credential filenames (the same addresses `auth_list_accounts` returns). Add account runs auth again. Remove account deletes that one file. Do not show token contents.
