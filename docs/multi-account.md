# Multiple Google accounts

This server already stores one OAuth credential file per Google account and loads that file when a tool call names the address. No extra flag turns this on.

Login twice and you get two files. A later call for `ada@work.com` uses only that file. The other account's token stays on disk.

## Where tokens live

Directory, first match wins:

1. `WORKSPACE_MCP_CREDENTIALS_DIR`
2. `GOOGLE_MCP_CREDENTIALS_DIR`
3. `$DATA_DIR/.google_workspace_mcp/credentials` when `DATA_DIR` is set (gantry volume)
4. `~/.google_workspace_mcp/credentials`

Each account is `{email}.json` in that directory. Mode `0600` on the file, `0700` on the directory. The filename is the lookup key. `GetCredential(email)` reads it. `StoreCredential` writes it. A second login does not delete the first file.

The HTTP client cache is also keyed by email, so two accounts do not share an access token in memory.

## How a tool call picks an account

`resolveEmail` in `tools/registry.go`:

1. `user_google_email` on that call, when it is non-empty.
2. Otherwise `USER_GOOGLE_EMAIL`.
3. Otherwise the only `*.json` in the credential directory, when there is exactly one.
4. Otherwise the call errors and asks for an email.

Every Workspace tool takes `user_google_email`. An explicit argument overrides the env var.

With two or more files, step 3 stops. If `USER_GOOGLE_EMAIL` is set, every call that omits the argument uses that one account. For real multi-account use, leave `USER_GOOGLE_EMAIL` unset so a missing argument is an error instead of a silent default. Or set it as the everyday default and pass `user_google_email` only when the user names the other account.

The same Google Cloud OAuth client (`GOOGLE_OAUTH_CLIENT_ID` / `GOOGLE_OAUTH_CLIENT_SECRET`) is used for every account. Each person consents separately. Do not mint a second client per mailbox.

## Logging in another account

Laptop:

```bash
google-mcp auth --email ada@work.com
google-mcp auth --email ada@gmail.com
```

`--email` is optional. When it is omitted, the address comes from the Google userinfo response after consent, then from `USER_GOOGLE_EMAIL` if userinfo fails. The file is named with the address that was stored, not with a guess.

Headless (`auth url` then `auth exchange <code>`) does the same write: one new `{email}.json` per successful exchange.

Repeat auth for an address replaces that address's file only.

## Choosing an account from the model

`auth_list_accounts` returns the signed-in addresses and nothing else (no tokens). It is on the core tier, including read-only, lean, and everyday. Call it when the account is unclear, then pass one address as `user_google_email`.

Addresses are stored and compared in lowercase. `Ada@gmail.com` and `ada@gmail.com` are the same file. Plus-addresses stay distinct (`ada+news@gmail.com`). An older mixed-case filename is still found, and the next save rewrites it to the lowercase name.

## What this server does not do

- It does not infer "work calendar" or similar. The model must pass the email it was given, or call `auth_list_accounts` first.
- It is not multi-tenant. There is no session per remote caller. Whoever can invoke the process can name any email that has a file.

## What ai-gantry must do

ai-gantry launches this binary and runs `/auth google`. This repo does not change that host. The host already has the right credential shape if it keeps the following:

1. **One persistent credential directory** for the process, preferably via `DATA_DIR` or `WORKSPACE_MCP_CREDENTIALS_DIR`, so every `{email}.json` survives recreate. A fresh empty dir on each boot drops every account.
2. **Repeat `/auth google` per account.** Each `auth url` / `auth exchange` must leave existing `*.json` files in place. A new exchange writes only the new address. Do not wipe the directory, and do not reuse one fixed filename.
3. **Return the stored email to the chat** after exchange (the address in the filename). The model needs that string for `user_google_email`.
4. **Treat `USER_GOOGLE_EMAIL` as optional.** The host manifest already lists it under `optional_env_keys`. Setting it pins the default account for every call that omits `user_google_email`. With more than one account, leave it unset, or set it only when the operator picked a default. Do not require it at provision time.
5. **Pass tool arguments through unchanged.** `user_google_email` is a normal tool parameter. The harness must not strip it or overwrite it with the env default after the model set it.
6. **A second login needs its own PKCE pending file.** Starting auth for account B must not delete account A's credential file or mix the verifiers.

`auth_list_accounts` tells the model which emails exist. The host still has to show that same list to the person after login (step 3).

## What gantree must do

gantree provisions the host. It does not speak to Google on the tool path. It needs to stop assuming one mailbox:

1. **One OAuth client** (client id and secret) for the pendant. Accounts are consents, not extra clients.
2. **Account list, not a single email field.** Show linked accounts as the `{email}.json` filenames. Do not display file contents (access token, refresh token, client secret).
3. **Add account** runs the existing auth URL / exchange flow again and expects a new file beside the old ones.
4. **Remove account** deletes that one file (and drops that email from any default). Other files stay.
5. **Optional default account** maps to `USER_GOOGLE_EMAIL` on the MCP process. Empty means the model must pass `user_google_email` on each call.
6. **The credential directory is on the data volume** the pendant already bind-mounts (`DATA_DIR`), not on a container overlay that disappears on recreate.

Those dashboard and harness changes belong in the gantree and ai-gantry checkouts. They are not implemented by editing this repo.

## Pendant and Cab

The phone and pendant screens should show the same addresses `auth_list_accounts` returns, as "Signed in as …". That is for the person. The model already gets the list from the tool.

Add account and remove account on those screens are the same operations as gantree: another auth exchange, or delete that one `{email}.json`. Do not show token file contents. Those UIs live in the Pendant and Cab checkouts.
